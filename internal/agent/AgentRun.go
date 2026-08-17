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
	if LiftContextLength > 0.7 {
		var err error
		req.Memory, err = CompressContext(req.Memory)
		if err != nil {
			return err
		}
	}
	// 压缩上下文
	//主循环
	//GetAllTool→GenReqJsonPv→RequestProvider→ParseResponse→PublishDailyUsage→SaveMemory→DisplayResult
	for {
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
		streamedText := ""
		streamedReasoning := ""
		streamedToolCalls := make(map[string]struct{})
		for attempt := 1; attempt <= RetryNum; attempt++ {
			var requestErr error
			if agentvalue.Auth == "codex" {
				requestErr, responseJSON = llm.RequOuthStream(ctx, credential, agentvalue.AccountID, agentvalue.Provider, data, func(event llm.CodexStreamEvent) error {
					switch event.Type {
					case "response.output_text.delta":
						streamedText += event.Delta
						return ui.DisplayStreamResult(streamedText)
					case "response.reasoning_summary_text.delta":
						streamedReasoning += event.Delta
						return ui.DisplayThinking(streamedReasoning)
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
				requestErr, responseJSON = llm.RequProvider(credential, agentvalue.BaseUrl, agentvalue.Provider, data)
			}
			if requestErr == nil {
				debug.Record("llm_response", map[string]any{"iteration": AgentiterNum, "attempt": attempt, "body": responseJSON})
				break
			}
			debug.Record("llm_request_error", map[string]any{"iteration": AgentiterNum, "attempt": attempt, "error": requestErr.Error()})
			if errors.Is(requestErr, context.Canceled) {
				return requestErr
			}
			ui.DisplayResult(data)
			var statusErr *llm.HTTPStatusError
			if errors.As(requestErr, &statusErr) {
				return ui.DisplayFinalResult(fmt.Sprintf("HTTP %d: %s", statusErr.StatusCode, statusErr.Status))
			}

			if attempt == RetryNum {
				return ui.DisplayResult(fmt.Sprintf("request provider failed after %d attempts: %s", RetryNum, requestErr.Error()))
			}
			ui.DisplayThinking(fmt.Sprintf("try %d   %s", attempt, requestErr.Error()))
			time.Sleep(10 * time.Second)
		}
		// 解析响应
		toolCalls, assistantMemory, usage, err := ParseResponse(req.Provider, responseJSON)
		if err != nil {
			return ui.DisplayResult(err.Error())
		}
		debug.Record("parsed_response", map[string]any{"iteration": AgentiterNum, "tool_calls": toolCalls, "assistant_memory": assistantMemory, "usage": usage})
		if len(assistantMemory) > 0 && strings.TrimSpace(assistantMemory[0].ReasoningContent) != "" {
			if err := ui.DisplayThinking(assistantMemory[0].ReasoningContent); err != nil {
				return ui.DisplayResult(err.Error())
			}
		}
		// 存储每日token消耗
		dailyUsage, usageErr := billing.InsertCostData(SessionId, usage)
		//上下文长度
		contextLength = GetConLength(req.Provider, usage)
		if err := publishDailyUsage(ui, dailyUsage, usageErr); err != nil {
			return ui.DisplayResult(err.Error())
		}
		memory = inputMemory
		memory = memorys.AppendMemoryMessages(memory, assistantMemory)

		if len(toolCalls) == 0 {
			//结束循环
			if len(assistantMemory) == 0 {
				return ui.DisplayResult("final assistant message is empty")
			}
			finalContent := assistantMemory[0].Content
			if strings.TrimSpace(finalContent) == "" {
				finalContent = streamedText
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
			if streamedText != "" {
				if err := ui.FinishStreamResult(finalContent); err != nil {
					return err
				}
			}
			return ui.DisplayFinalResult(finalContent)
		}
		if streamedText != "" {
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
		debug.Record("tool_results", map[string]any{"iteration": AgentiterNum, "calls": toolCalls, "results": toolResults})
		toolMemory := llm.GettoolCallMemory(toolResults)
		memory = memorys.AppendMemoryMessages(memory, toolMemory)
		req.Memory = memory
		LiftContextLength := contextLength / utils.MaxContextLength
		if LiftContextLength > 0.7 {
			var err error
			req.Memory, err = CompressContext(req.Memory)
			if err != nil {
				return err
			}
		}
	}
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
