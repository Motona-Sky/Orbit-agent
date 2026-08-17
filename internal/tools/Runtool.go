package tools

import (
	"context"
	"errors"
	"fmt"
	"orbit/internal/utils"
)

// RunTools 并发执行模型返回的 tool_calls。
// 内部通过 GetEnabledTools() 拿到当前可执行的工具集合（按 mcpEnabled / disabledTools 过滤），
// agent 主循环无需传开关参数 —— MCP 开关和工具禁用全部封装在 tools 包内。
func RunTools(ctx context.Context, toolsValue []any) ([]ToolResult, error) {
	enabled := GetEnabledTools()

	type toolJob struct {
		index      int
		tool       ToolFunc
		toolCallID string
		function   map[string]any
	}

	jobs := make([]toolJob, 0, len(toolsValue))
	for index, value := range toolsValue {
		toolCall, ok := value.(map[string]any)
		if !ok {
			return nil, errors.New("tool call format is invalid")
		}
		toolCallID, ok := toolCall["id"].(string)
		if !ok || toolCallID == "" {
			return nil, errors.New("tool call id is invalid")
		}
		//example tool function = map[arguments:{"command": "ls"} name:exec_command] 示例↓↓↓
		function, ok := toolCall["function"].(map[string]any)
		if !ok {
			return nil, errors.New("tool function format is invalid")
		}
		name, ok := function["name"].(string)
		if !ok || name == "" {
			return nil, errors.New("tool name is invalid")
		}
		//只在 enabled 集合中查找：本地工具 + （可选）MCP 工具统一查这张表
		tool, ok := enabled[name]
		if !ok {
			return nil, fmt.Errorf("tool %q is not registered", name)
		}
		jobs = append(jobs, toolJob{
			index:      index,
			tool:       tool,
			toolCallID: toolCallID,
			function:   function,
		})
	}

	toolJobChannel := make(chan ToolResult, len(jobs))
	for _, job := range jobs {
		go runtooljob(ctx, job.index, job.tool, job.toolCallID, job.function, toolJobChannel)
	}

	results := make([]ToolResult, len(jobs))
	for range jobs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case result := <-toolJobChannel:
			results[result.index] = result
		}
	}
	return results, nil
}

type ToolResult struct {
	index      int
	Role       string `json:"role"`
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
}

// 内部函数，多线程调用
func runtooljob(ctx context.Context, index int, tool ToolFunc, toolCallID string, function map[string]any, toolJobChannel chan<- ToolResult) {
	result := ToolResult{
		index:      index,
		Role:       "tool",
		ToolCallID: toolCallID,
	}
	output, err := tool.Function(ctx, []any{function})
	if err != nil {
		result.Content = fmt.Errorf("run tool %q for call %q: %w", tool.Name, toolCallID, err).Error()
	} else {
		result.Content = output
	}
	select {
	case toolJobChannel <- result:
	case <-ctx.Done():
	}
}

// Tool结果解析为json字符串
func GetToolResultsJson(toolsValue []ToolResult) ([]ToolResult, error) {
	switch utils.ProviderConfig {
	case "openai:completions":
		return toolsValue, nil
	case "anthropic:messages":
		return toolsValue, nil
	case "openai:response":
		return toolsValue, nil
	default:
		return nil, errors.New("provider config is invalid")
	}
}
