package agent

import (
	"encoding/json"
	"errors"
	"orbit/internal/llm"
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
func ParseResponse(jsonstr string) ([]any, []llm.MemoryMessage, map[string]any, error) {
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
}

// 输入map格式。类似[map[arguments:{"command": "ls"} name:exec_command] map[arguments:{"filepath": "F:/tmp.txt"} name:read_file]]
