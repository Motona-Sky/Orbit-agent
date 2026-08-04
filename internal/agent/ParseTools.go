package agent

import (
	"encoding/json"
	"errors"
	"orbit/internal/llm"
	"strings"
)

type ToolResultJson struct {
	Role       string `json:"role"`
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
}

// 解析模型返回的json字符串
func ParseResponseJSON(reqJSON string) (map[string]any, error) {
	var response map[string]any

	if err := json.Unmarshal([]byte(reqJSON), &response); err != nil {
		return nil, err
	}

	return response, nil
}

func ParassistantJson(reqJSON []byte) ([]llm.MemoryMessage, error) {
	var message llm.MemoryMessage

	if err := json.Unmarshal(reqJSON, &message); err != nil {
		return []llm.MemoryMessage{}, err
	}

	return []llm.MemoryMessage{message}, nil
}

// 模型工具调用部分json,→RunTools
func ParseResponse(req string, jsonstr string) ([]any, []llm.MemoryMessage, map[string]any, error) {
	switch req {
	case "openai:completions":
		response, err := ParseResponseJSON(jsonstr)
		if err != nil {
			return nil, nil, nil, err
		}
		//解析token消耗
		usage, ok := response["usage"].(map[string]any)
		if !ok {
			return nil, nil, usage, errors.New("response usage is empty")
		}
		choices, ok := response["choices"].([]any)
		if !ok || len(choices) == 0 {
			return nil, nil, usage, errors.New("response choices is empty")
		}
		choice, ok := choices[0].(map[string]any)
		if !ok {
			return nil, nil, usage, errors.New("response choice format is invalid")
		}
		message, ok := choice["message"].(map[string]any)
		if !ok {
			return nil, nil, usage, errors.New("response message is empty")
		}
		assistantJson, err := json.Marshal(message)
		if err != nil {
			return nil, nil, usage, errors.New("response message is empty")
		}
		mem, err := ParassistantJson(assistantJson)
		if err != nil {
			return nil, nil, usage, err
		}
		toolCallsValue, exists := message["tool_calls"]
		if !exists || toolCallsValue == nil {
			return []any{}, mem, usage, nil
		}
		// 工具调用部分json
		toolCalls, ok := toolCallsValue.([]any)
		if !ok {
			return nil, nil, usage, errors.New("response tool_calls format is invalid")
		}
		return toolCalls, mem, usage, nil
	case "openai:response", "openai:responses":
		response, err := ParseResponseJSON(jsonstr)
		if err != nil {
			return nil, nil, nil, err
		}
		usage, ok := response["usage"].(map[string]any)
		if !ok {
			return nil, nil, nil, errors.New("response usage is empty")
		}
		output, ok := response["output"].([]any)
		if !ok {
			return nil, nil, usage, errors.New("response output is empty")
		}

		toolCalls := make([]any, 0)
		textParts := make([]string, 0)
		for _, value := range output {
			item, ok := value.(map[string]any)
			if !ok {
				return nil, nil, usage, errors.New("response output item format is invalid")
			}
			itemType, _ := item["type"].(string)
			switch itemType {
			case "message":
				content, ok := item["content"].([]any)
				if !ok {
					continue
				}
				for _, contentValue := range content {
					contentItem, ok := contentValue.(map[string]any)
					if !ok {
						continue
					}
					if contentItem["type"] == "output_text" {
						if text, ok := contentItem["text"].(string); ok {
							textParts = append(textParts, text)
						}
					}
				}
			case "function_call":
				callID, ok := item["call_id"].(string)
				if !ok || callID == "" {
					return nil, nil, usage, errors.New("response function call id is invalid")
				}
				name, ok := item["name"].(string)
				if !ok || name == "" {
					return nil, nil, usage, errors.New("response function call name is invalid")
				}
				arguments, ok := item["arguments"].(string)
				if !ok {
					return nil, nil, usage, errors.New("response function call arguments are invalid")
				}
				toolCalls = append(toolCalls, map[string]any{
					"id": callID,
					"function": map[string]any{
						"name":      name,
						"arguments": arguments,
					},
				})
			}
		}

		message := llm.MemoryMessage{
			Role:      "assistant",
			Content:   strings.Join(textParts, ""),
			ToolCalls: toolCalls,
		}
		return toolCalls, []llm.MemoryMessage{message}, usage, nil
	case "anthropic:messages":
		return nil, nil, nil, nil
	default:
		return nil, nil, nil, errors.New("response provider is not support")
	}
}

// 输入map格式。类似[map[arguments:{"command": "ls"} name:exec_command] map[arguments:{"filepath": "F:/tmp.txt"} name:read_file]]
