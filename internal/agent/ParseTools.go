package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"orbit/internal/llm"
	"strings"
)

type ToolResultJson struct {
	Role       string `json:"role"`
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
}

type ResponseState struct {
	FinishReason string
	Status       string
	Incomplete   bool
}

func (state ResponseState) validate(toolCalls []any) error {
	switch state.FinishReason {
	case "length":
		return errors.New("assistant response stopped because the output token limit was reached")
	case "content_filter":
		return errors.New("assistant response was stopped by the content filter")
	case "tool_calls", "function_call":
		if len(toolCalls) == 0 {
			return errors.New("assistant response ended for tool calls but contained no tool calls")
		}
	}
	if state.Incomplete || state.Status == "incomplete" {
		return errors.New("assistant response is incomplete")
	}
	switch state.Status {
	case "failed", "cancelled", "canceled":
		return fmt.Errorf("assistant response ended with status %q", state.Status)
	}
	return nil
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

// ParseResponse 保留原有调用方式；需要判断响应结束状态时使用 ParseResponseDetails。
func ParseResponse(req string, jsonstr string) ([]any, []llm.MemoryMessage, map[string]any, error) {
	toolCalls, memory, usage, _, err := ParseResponseDetails(req, jsonstr)
	return toolCalls, memory, usage, err
}

// ParseResponseDetails 解析模型响应及供应商提供的结束状态。
func ParseResponseDetails(req string, jsonstr string) ([]any, []llm.MemoryMessage, map[string]any, ResponseState, error) {
	var state ResponseState
	switch req {
	case "openai:completions":
		response, err := ParseResponseJSON(jsonstr)
		if err != nil {
			return nil, nil, nil, state, err
		}
		usage, ok := response["usage"].(map[string]any)
		if !ok {
			return nil, nil, usage, state, errors.New("response usage is empty")
		}
		choices, ok := response["choices"].([]any)
		if !ok || len(choices) == 0 {
			return nil, nil, usage, state, errors.New("response choices is empty")
		}
		choice, ok := choices[0].(map[string]any)
		if !ok {
			return nil, nil, usage, state, errors.New("response choice format is invalid")
		}
		state.FinishReason, _ = choice["finish_reason"].(string)
		message, ok := choice["message"].(map[string]any)
		if !ok {
			return nil, nil, usage, state, errors.New("response message is empty")
		}
		assistantJSON, err := json.Marshal(message)
		if err != nil {
			return nil, nil, usage, state, errors.New("response message is empty")
		}
		mem, err := ParassistantJson(assistantJSON)
		if err != nil {
			return nil, nil, usage, state, err
		}
		toolCallsValue, exists := message["tool_calls"]
		if !exists || toolCallsValue == nil {
			return []any{}, mem, usage, state, nil
		}
		toolCalls, ok := toolCallsValue.([]any)
		if !ok {
			return nil, nil, usage, state, errors.New("response tool_calls format is invalid")
		}
		return toolCalls, mem, usage, state, nil
	case "openai:response", "openai:responses", "oauth:codex":
		response, err := ParseResponseJSON(jsonstr)
		if err != nil {
			return nil, nil, nil, state, err
		}
		state.Status, _ = response["status"].(string)
		state.Incomplete = response["incomplete_details"] != nil
		usage, ok := response["usage"].(map[string]any)
		if !ok {
			return nil, nil, nil, state, errors.New("response usage is empty")
		}
		output, ok := response["output"].([]any)
		if !ok {
			return nil, nil, usage, state, errors.New("response output is empty")
		}

		toolCalls := make([]any, 0)
		textParts := make([]string, 0)
		reasoningParts := make([]string, 0)
		responseItems := make([]any, 0)
		for _, value := range output {
			item, ok := value.(map[string]any)
			if !ok {
				return nil, nil, usage, state, errors.New("response output item format is invalid")
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
			case "reasoning":
				responseItems = append(responseItems, item)
				summary, _ := item["summary"].([]any)
				for _, summaryValue := range summary {
					summaryItem, ok := summaryValue.(map[string]any)
					if !ok {
						continue
					}
					if text, ok := summaryItem["text"].(string); ok {
						reasoningParts = append(reasoningParts, text)
					}
				}
			case "function_call":
				callID, ok := item["call_id"].(string)
				if !ok || callID == "" {
					return nil, nil, usage, state, errors.New("response function call id is invalid")
				}
				name, ok := item["name"].(string)
				if !ok || name == "" {
					return nil, nil, usage, state, errors.New("response function call name is invalid")
				}
				arguments, ok := item["arguments"].(string)
				if !ok {
					return nil, nil, usage, state, errors.New("response function call arguments are invalid")
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
			Role:             "assistant",
			Content:          strings.Join(textParts, ""),
			ReasoningContent: strings.Join(reasoningParts, "\n"),
			ToolCalls:        toolCalls,
			ResponseItems:    responseItems,
		}
		return toolCalls, []llm.MemoryMessage{message}, usage, state, nil
	case "anthropic:messages":
		return nil, nil, nil, state, nil
	default:
		return nil, nil, nil, state, errors.New("response provider is not support")
	}
}
