package tools

import (
	"errors"
	"fmt"
	"looporbit/internal/utils"
)

func RunTools(toolsValue []any) ([]ToolResult, error) {
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
		//查看注册的工具
		tool, ok := RegToolFuncs[name]
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
		go runtooljob(job.index, job.tool, job.toolCallID, job.function, toolJobChannel)
	}

	results := make([]ToolResult, len(jobs))
	for range jobs {
		result := <-toolJobChannel
		results[result.index] = result
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
func runtooljob(index int, tool ToolFunc, toolCallID string, function map[string]any, toolJobChannel chan<- ToolResult) {
	result := ToolResult{
		index:      index,
		Role:       "tool",
		ToolCallID: toolCallID,
	}
	output, err := tool.Function([]any{function})
	if err != nil {
		result.Content = fmt.Errorf("run tool %q for call %q: %w", tool.Name, toolCallID, err).Error()
	} else {
		result.Content = output
	}
	toolJobChannel <- result
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
