package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"orbit/internal/agentui"
	"orbit/internal/billing"
	"orbit/internal/debug"
	"orbit/internal/llm"
	"orbit/internal/memorys"
	"orbit/internal/oauth"
	"orbit/internal/tools"
	"orbit/internal/utils"
	"os"
	"strings"
	"time"
)

type RunAgentValue struct {
	Provider      string
	ApiKey        string
	Auth          string
	AccessToken   string
	AccountID     string
	BaseUrl       string
	Model         string
	SystemPt      string
	ThinkLevel    string
	UserInput     string
	ResumeSession bool
}

var contextLength float64 = 0

const (
	maxAgentIterations    = 70
	maxRepeatedToolRounds = 3
)

// RunAgent 加载会话上下文并执行 Agent 循环，直至产生最终回复或遇到不可恢复的错误。
func RunAgent(ctx context.Context, agentvalue RunAgentValue, ui *agentui.AgentUI) error {
	var memory []llm.MemoryMessage
	req := llm.GenReqJsonPvRequest{
		Provider:     agentvalue.Provider,
		Model:        agentvalue.Model,
		UserInput:    agentvalue.UserInput,
		SystemPrompt: agentvalue.SystemPt,
		ThinkLevel:   agentvalue.ThinkLevel,
		Memory:       memory,
	}

	if agentvalue.Provider == "" {
		return errors.New("no provider")
	}
	credential := agentvalue.ApiKey
	if agentvalue.Auth == "codex" {
		credential = agentvalue.AccessToken
	}
	if credential == "" {
		return errors.New("no credential")
	}
	if agentvalue.Auth == "codex" {
		usable, err := oauth.ParseAccessToken(credential)
		if err != nil {
			return fmt.Errorf("check codex access token: %w", err)
		}
		if !usable {
			refreshErr, accessToken := oauth.RefreshTokens()
			if refreshErr != nil {
				return fmt.Errorf("refresh codex access token: %w", refreshErr)
			}
			credential = accessToken
			agentvalue.AccountID = utils.AccountID
		}
	}

	if agentvalue.BaseUrl == "" {
		return errors.New("no BaseUrl")
	}

	if req.Model == "" {
		return errors.New("no model")
	}
	if ui == nil {
		return errors.New("no agent ui")
	}
	var AgentiterNum int = 0
	var RetryNum int = 5
	var previousToolRoundSignature string
	var repeatedToolRoundCount int
	SessionId := utils.SessionId
	debug.Record("agent_start", map[string]any{
		"provider":       agentvalue.Provider,
		"model":          agentvalue.Model,
		"base_url":       agentvalue.BaseUrl,
		"auth":           agentvalue.Auth,
		"system_prompt":  agentvalue.SystemPt,
		"user_input":     agentvalue.UserInput,
		"think_level":    agentvalue.ThinkLevel,
		"resume_session": agentvalue.ResumeSession,
	})
	// 加载记忆
	loadedMemory, err := loadAgentMemory(SessionId, agentvalue.ResumeSession)
	if err != nil {
		return err
	}
	req.Memory = loadedMemory
	debug.Record("loaded_memory", loadedMemory)

	// 计算上下文长度
	contextLength = estimateMemoryTokens(loadedMemory)
	usageStats := agentui.UsageStats{
		ContextUsed:  contextLength,
		ContextTotal: utils.MaxContextLength,
	}
	if dailyUsage, err := billing.QueryTodayUsage(); err == nil {
		usageStats.TodayTokens = dailyUsage.TotalTokens
		usageStats.CacheHitRate = dailyUsage.CacheHitRate()
	}
	_ = ui.DisplayUsage(usageStats)

	LiftContextLength := contextLength / utils.MaxContextLength
	compressCred := CompressCredential{
		Auth:       agentvalue.Auth,
		Credential: credential,
		AccountID:  agentvalue.AccountID,
		BaseUrl:    agentvalue.BaseUrl,
		Provider:   agentvalue.Provider,
		Model:      agentvalue.Model,
	}
	if LiftContextLength > 0.7 {
		var err error
		req.Memory, err = CompressContext(ctx, req.Memory, compressCred)
		if err != nil {
			return err
		}
		contextLength = estimateMemoryTokens(req.Memory)
	}
	// 压缩上下文
	//主循环
	//GetAllTool→GenReqJsonPv→RequestProvider→ParseResponse→PublishDailyUsage→SaveMemory→DisplayResult
	for AgentiterNum < maxAgentIterations {
		AgentiterNum++
		// GetAllTool 内部按 mcpEnabled/disabledTools 自动过滤，agent 层不感知开关。
		allTools := tools.GetAllTool(agentvalue.Provider)
		inputMemory, data, err := llm.GenReqJsonPv(req, allTools)
		// ui.DisplayThinking(data)
		if err != nil {
			return fmt.Errorf("gen req json pv err: %w", err)
		}
		debug.Record("llm_request", map[string]any{"iteration": AgentiterNum, "body": data})
		req.Memory = inputMemory
		// 清空SystemPrompt和UserInput
		req.SystemPrompt = ""
		req.UserInput = ""
		var responseJSON string
		var streamedText strings.Builder
		var streamedReasoning strings.Builder
		var lastStreamTextUpdate time.Time
		var lastStreamReasoningUpdate time.Time
		streamedToolCalls := make(map[string]struct{})
		for attempt := 1; attempt <= RetryNum; attempt++ {
			streamedText.Reset()
			streamedReasoning.Reset()
			lastStreamTextUpdate = time.Time{}
			lastStreamReasoningUpdate = time.Time{}
			streamedToolCalls = make(map[string]struct{})
			var requestErr error
			if agentvalue.Auth == "codex" {
				requestErr, responseJSON = llm.RequOuthStream(ctx, credential, agentvalue.AccountID, agentvalue.Provider, data, func(event llm.CodexStreamEvent) error {
					switch event.Type {
					case "response.output_text.delta":
						streamedText.WriteString(event.Delta)
						if !shouldPublishStreamUpdate(&lastStreamTextUpdate, time.Now()) {
							return nil
						}
						return ui.DisplayStreamResult(streamedText.String())
					case "response.reasoning_summary_text.delta":
						streamedReasoning.WriteString(event.Delta)
						if !shouldPublishStreamUpdate(&lastStreamReasoningUpdate, time.Now()) {
							return nil
						}
						return ui.DisplayThinking(streamedReasoning.String())
					case "response.output_item.done":
						if event.Item["type"] != "function_call" {
							return nil
						}
						callID, _ := event.Item["call_id"].(string)
						if _, shown := streamedToolCalls[callID]; shown {
							return nil
						}
						streamedToolCalls[callID] = struct{}{}
						return ui.DisplayActivity(agentui.ActivityTool, toolActivityFromResponseItem(event.Item))
					}
					return nil
				})
			} else {
				requestErr, responseJSON = llm.RequProvider(ctx, credential, agentvalue.BaseUrl, agentvalue.Provider, data)
			}
			if requestErr == nil {
				debug.Record("llm_response", map[string]any{"iteration": AgentiterNum, "attempt": attempt, "body": responseJSON})
				break
			}
			debug.Record("llm_request_error", map[string]any{"iteration": AgentiterNum, "attempt": attempt, "error": requestErr.Error()})
			if errors.Is(requestErr, context.Canceled) {
				return requestErr
			}
			if attempt == RetryNum {
				return fmt.Errorf("request provider failed after %d attempts: %w", RetryNum, requestErr)
			}
			ui.DisplayThinking(fmt.Sprintf("try %d   %s", attempt, requestErr.Error()))
			timer := time.NewTimer(10 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		// 解析响应
		toolCalls, assistantMemory, usage, responseState, err := ParseResponseDetails(req.Provider, responseJSON)
		if err != nil {
			return err
		}
		if err := responseState.validate(toolCalls); err != nil {
			return err
		}
		debug.Record("parsed_response", map[string]any{"iteration": AgentiterNum, "tool_calls": toolCalls, "assistant_memory": assistantMemory, "usage": usage, "finish_reason": responseState.FinishReason, "status": responseState.Status, "incomplete": responseState.Incomplete})
		if len(assistantMemory) > 0 && strings.TrimSpace(assistantMemory[0].ReasoningContent) != "" {
			if err := ui.DisplayThinking(assistantMemory[0].ReasoningContent); err != nil {
				return err
			}
		}
		// 存储每日token消耗
		dailyUsage, usageErr := billing.InsertCostData(SessionId, usage)
		//上下文长度
		contextLength = GetConLength(req.Provider, usage)
		if err := publishDailyUsage(ui, dailyUsage, usageErr); err != nil {
			return err
		}
		memory = inputMemory
		memory = memorys.AppendMemoryMessages(memory, assistantMemory)

		if len(toolCalls) == 0 {
			//结束循环
			if len(assistantMemory) == 0 {
				return errors.New("final assistant message is empty")
			}
			finalContent := assistantMemory[0].Content
			if strings.TrimSpace(finalContent) == "" {
				finalContent = streamedText.String()
				assistantMemory[0].Content = finalContent
				memory[len(memory)-1].Content = finalContent
			}
			jsonMemory, err := json.Marshal(memory)
			if err != nil {
				return fmt.Errorf("marshal session %q: %w", SessionId, err)
			}
			// 保存记忆
			if err := memorys.SaveChatHistory(jsonMemory, SessionId); err != nil {
				memorys.CreateSessionFolder()
			}
			debug.Record("saved_memory", memory)
			if streamedText.Len() > 0 {
				if err := ui.FinishStreamResult(finalContent); err != nil {
					return err
				}
			}
			return ui.DisplayFinalResult(finalContent)
		}
		if streamedText.Len() > 0 {
			if err := ui.FinishStreamResult(assistantMemory[0].Content); err != nil {
				return err
			}
		} else if content := strings.TrimSpace(assistantMemory[0].Content); content != "" {
			if err := ui.DisplayResult(assistantMemory[0].Content); err != nil {
				return err
			}
		}
		// 显示未通过流式事件发布的工具调用
		for _, toolCall := range toolCalls {
			callID := toolCallID(toolCall)
			if _, shown := streamedToolCalls[callID]; shown {
				continue
			}
			target := toolActivityFromCall(toolCall)
			if err := ui.DisplayActivity(agentui.ActivityTool, target); err != nil {
				return err
			}
		}
		toolResults, err := tools.RunTools(ctx, toolCalls)
		if err != nil {
			return fmt.Errorf("run tools err: %w", err)
		}
		toolRoundSignature, err := makeToolRoundSignature(toolCalls, toolResults)
		if err != nil {
			return fmt.Errorf("build tool round signature: %w", err)
		}
		if toolRoundSignature == previousToolRoundSignature {
			repeatedToolRoundCount++
		} else {
			previousToolRoundSignature = toolRoundSignature
			repeatedToolRoundCount = 1
		}
		if repeatedToolRoundCount >= maxRepeatedToolRounds {
			return fmt.Errorf("agent stopped after repeating the same tool calls and results for %d rounds", repeatedToolRoundCount)
		}
		debug.Record("tool_results", map[string]any{"iteration": AgentiterNum, "calls": toolCalls, "results": toolResults})
		toolMemory := llm.GettoolCallMemory(toolResults)
		memory = memorys.AppendMemoryMessages(memory, toolMemory)
		req.Memory = memory

		// 中间保存：每轮工具调用后保存会话历史，防止崩溃导致数据丢失
		if jsonMem, err := json.Marshal(memory); err == nil {
			_ = memorys.SaveChatHistory(jsonMem, SessionId)
		}
		LiftContextLength := contextLength / utils.MaxContextLength
		if LiftContextLength > 0.7 {
			var err error
			req.Memory, err = CompressContext(ctx, req.Memory, compressCred)
			if err != nil {
				return err
			}
			contextLength = estimateMemoryTokens(req.Memory)
			memory = req.Memory
		}
	}
	return fmt.Errorf("agent stopped after reaching the maximum of %d iterations", maxAgentIterations)
}

func makeToolRoundSignature(toolCalls []any, toolResults []tools.ToolResult) (string, error) {
	if len(toolCalls) != len(toolResults) {
		return "", errors.New("tool calls and results length mismatch")
	}
	type signatureItem struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
		Result    string `json:"result"`
	}
	items := make([]signatureItem, 0, len(toolCalls))
	for index, value := range toolCalls {
		call, ok := value.(map[string]any)
		if !ok {
			return "", errors.New("tool call format is invalid")
		}
		function, ok := call["function"].(map[string]any)
		if !ok {
			return "", errors.New("tool function format is invalid")
		}
		name, _ := function["name"].(string)
		arguments, _ := function["arguments"].(string)
		items = append(items, signatureItem{Name: name, Arguments: arguments, Result: toolResults[index].Content})
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

const streamUIUpdateInterval = 200 * time.Millisecond

func shouldPublishStreamUpdate(lastUpdate *time.Time, now time.Time) bool {
	if !lastUpdate.IsZero() && now.Sub(*lastUpdate) < streamUIUpdateInterval {
		return false
	}
	*lastUpdate = now
	return true
}

// loadAgentMemory 加载会话历史；仅新会话允许历史文件尚不存在，恢复会话会传播该错误。
func loadAgentMemory(sessionID string, resumeSession bool) ([]llm.MemoryMessage, error) {
	messages, err := memorys.LoadMemory(sessionID)
	if err == nil {
		return messages, nil
	}
	if errors.Is(err, os.ErrNotExist) && !resumeSession {
		return nil, nil
	}
	return nil, err
}

func publishDailyUsage(ui *agentui.AgentUI, usage billing.DailyUsage, usageErr error) error {
	if usageErr != nil {
		if errors.Is(usageErr, billing.ErrDailyUsageQuery) {
			return nil
		}
		return fmt.Errorf("insert cost data err: %w", usageErr)
	}
	return ui.DisplayUsage(agentui.UsageStats{
		TodayTokens:  usage.TotalTokens,
		CacheHitRate: usage.CacheHitRate(),
		ContextUsed:  contextLength,
		ContextTotal: utils.MaxContextLength,
	})
}

func toolActivityFromResponseItem(item map[string]any) string {
	return toolActivityTarget(item["name"], item["arguments"])
}

func toolCallID(call any) string {
	value, ok := call.(map[string]any)
	if !ok {
		return ""
	}
	callID, _ := value["id"].(string)
	return callID
}

// 显示工具
func toolActivityFromCall(call any) string {
	value, ok := call.(map[string]any)
	if !ok {
		return "unknown"
	}
	function, ok := value["function"].(map[string]any)
	if !ok {
		return "unknown"
	}
	return toolActivityTarget(function["name"], function["arguments"])
}

func toolActivityTarget(nameValue, argumentsValue any) string {
	name, _ := nameValue.(string)
	rawArguments, _ := argumentsValue.(string)
	var args map[string]any
	if rawArguments != "" {
		_ = json.Unmarshal([]byte(rawArguments), &args)
	}

	if len(args) == 0 {
		return name
	}

	var parts []string
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s: %v", k, v))
	}
	return name + " | " + strings.Join(parts, " | ")
}

// estimateMemoryTokens 根据消息内容粗略估算 token 数量，用于首次 LLM 调用前的上下文显示。
func estimateMemoryTokens(messages []llm.MemoryMessage) float64 {
	var total int
	for _, msg := range messages {
		total += len(msg.Content)
		for _, tc := range msg.ToolCalls {
			if m, ok := tc.(map[string]any); ok {
				if fn, ok := m["function"].(map[string]any); ok {
					if args, ok := fn["arguments"].(string); ok {
						total += len(args)
					}
				}
			}
		}
	}
	return float64(total) / 4
}
