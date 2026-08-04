package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"orbit/internal/agentui"
	"orbit/internal/billing"
	"orbit/internal/i18n"
	"orbit/internal/llm"
	"orbit/internal/memorys"
	"orbit/internal/tools"
	"orbit/internal/utils"
	"os"
	"strconv"
	"strings"
	"time"
)

type RunAgentValue struct {
	Provider      string
	ApiKey        string
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
	if agentvalue.ApiKey == "" {
		return errors.New("no apikey")
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
	// 加载记忆
	loadedMemory, err := loadAgentMemory(SessionId, agentvalue.ResumeSession)
	if err != nil {
		return err
	}
	req.Memory = loadedMemory

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
		req.Memory = inputMemory
		// 清空SystemPrompt和UserInput
		req.SystemPrompt = ""
		req.UserInput = ""
		responseJSON, err := requestProviderWithRetry(
			RetryNum,
			func() (error, string) {
				return llm.RequProvider(agentvalue.ApiKey, agentvalue.BaseUrl, agentvalue.Provider, data)
			},
			time.Sleep,
			func(requestErr error, attempt int, maxAttempts int) {
				_ = ui.DisplayThinking(
					i18n.For(i18n.DefaultLanguage).Messages.OtherActivity.RetryLabel +
						requestErr.Error() + strconv.Itoa(attempt) + "/" + strconv.Itoa(maxAttempts),
				)
			},
		)
		if err != nil {
			return err
		}
		// 解析响应
		toolCalls, assistantMemory, usage, err := ParseResponse(req.Provider, responseJSON)
		if err != nil {
			return fmt.Errorf("get tool calls json err: %w", err)
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
			jsonMemory, err := json.Marshal(memory)
			if err != nil {
				return fmt.Errorf("marshal session %q: %w", SessionId, err)
			}
			// 保存记忆
			if err := memorys.SaveChatHistory(jsonMemory, SessionId); err != nil {
				memorys.CreateSessionFolder()
			}
			return ui.DisplayFinalResult(assistantMemory[0].Content)
		}
		if content := strings.TrimSpace(assistantMemory[0].Content); content != "" {
			if err := ui.DisplayResult(assistantMemory[0].Content); err != nil {
				return err
			}
		}
		// 显示工具调用
		for _, toolCall := range toolCalls {
			target := toolActivityFromCall(toolCall)
			if err := ui.DisplayActivity(agentui.ActivityTool, target); err != nil {
				return err
			}
		}
		toolResults, err := tools.RunTools(ctx, toolCalls)
		if err != nil {
			return fmt.Errorf("run tools err: %w", err)
		}
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
	name, _ := function["name"].(string)

	rawArguments, _ := function["arguments"].(string)
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
