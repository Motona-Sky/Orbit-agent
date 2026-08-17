package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"orbit/internal/tools"
	"time"

	"github.com/imroc/req/v3"
)

type HTTPStatusError struct {
	StatusCode int
	Status     string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("request failed: %s", e.Status)
}

// RequProvider 根据供应商类型发送 HTTP 请求。
func RequProvider(apikey string, baseurl string, provider string, data string) (error, string) {
	switch {
	case provider == "openai:completions":
		// OpenAI Completion
		client := req.C()
		resp, err := client.SetTimeout(30*time.Second).R().
			SetHeader("Content-Type", "application/json").
			SetHeaders(map[string]string{ // 一次设置多个请求头。

				"Authorization": "Bearer " + apikey,
			}).
			SetBody(data).
			Post(baseurl + "/chat/completions")
		if err != nil {
			return err, ""
		}
		if !resp.IsSuccessState() {
			return &HTTPStatusError{StatusCode: resp.StatusCode, Status: resp.Status}, ""
		}
		return nil, resp.String()
	case provider == "openai:responses":
		client := req.C()
		resp, err := client.SetTimeout(30*time.Second).R().
			SetHeader("Content-Type", "application/json").
			SetHeader("Authorization", "Bearer "+apikey).
			SetBody(data).
			Post(baseurl + "/responses")
		if err != nil {
			return err, ""
		}
		if !resp.IsSuccessState() {
			return &HTTPStatusError{StatusCode: resp.StatusCode, Status: resp.Status}, ""
		}
		return nil, resp.String()
	case provider == "anthropic:messages":
		// Anthropic Messages 暂未实现。
		return errors.New("anthropic provider is not implemented"), ""
	}

	return errors.New("unsupported provider"), ""
}

// MemoryMessage 表示请求中的单条对话消息。
type MemoryMessage struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
	ToolCalls        []any  `json:"tool_calls,omitempty"`
	ToolCallID       string `json:"tool_call_id,omitempty"`
	ResponseItems    []any  `json:"response_items,omitempty"`
}

type openAIThinking struct {
	Type string `json:"type"`
}

type openAIResponseFormat struct {
	Type string `json:"type"`
}

type openAICompletionsRequest struct {
	Model           string               `json:"model"`
	Messages        []MemoryMessage      `json:"messages"`
	Thinking        openAIThinking       `json:"thinking"`
	ReasoningEffort string               `json:"reasoning_effort"`
	MaxTokens       int                  `json:"max_tokens"`
	ResponseFormat  openAIResponseFormat `json:"response_format"`
	Stop            any                  `json:"stop"`
	Stream          bool                 `json:"stream"`
	StreamOptions   any                  `json:"stream_options"`
	Temperature     float64              `json:"temperature"`
	TopP            float64              `json:"top_p"`
	Tools           json.RawMessage      `json:"tools,omitempty"`
	ToolChoice      string               `json:"tool_choice,omitempty"`
	Logprobs        bool                 `json:"logprobs"`
	TopLogprobs     *int                 `json:"top_logprobs"`
}

// buildOpenAICompletionsMessages 构建 system、历史记录和当前 user 消息。
func buildOpenAICompletionsMessages(systemPrompt, userInput string, memory []MemoryMessage) []MemoryMessage {
	messages := make([]MemoryMessage, 0, len(memory)+2)
	messages = append(messages, memory...)
	if len(memory) == 0 && systemPrompt != "" {
		messages = append(messages, MemoryMessage{Role: "system", Content: systemPrompt})
	}
	if userInput != "" {
		messages = append(messages, MemoryMessage{Role: "user", Content: userInput})
	}
	return messages
}

func buildOpenAICompletionsRequest(req GenReqJsonPvRequest, registeredTools []tools.ToolReg) (openAICompletionsRequest, error) {
	toolsJSON, err := json.Marshal(registeredTools)
	if err != nil {
		return openAICompletionsRequest{}, fmt.Errorf("marshal tools JSON: %w", err)
	}

	thinkingType := "disabled"
	if req.ThinkLevel != "" {
		thinkingType = "enabled"
	}

	request := openAICompletionsRequest{
		Model:           req.Model,
		Messages:        buildOpenAICompletionsMessages(req.SystemPrompt, req.UserInput, req.Memory),
		Thinking:        openAIThinking{Type: thinkingType},
		ReasoningEffort: req.ThinkLevel,
		MaxTokens:       4096,
		ResponseFormat:  openAIResponseFormat{Type: "text"},
		Temperature:     1,
		TopP:            1,
	}
	if len(registeredTools) > 0 {
		request.Tools = toolsJSON
		request.ToolChoice = "auto"
	}
	return request, nil
}

type GenReqJsonPvRequest struct {
	Provider     string
	Model        string
	SystemPrompt string
	UserInput    string
	ThinkLevel   string
	Memory       []MemoryMessage
}

// GenReqJsonPv 生成供应商请求所需的 JSON，工具列表由外部注册机制提供。
func GenReqJsonPv(req GenReqJsonPvRequest, tools []tools.ToolReg) ([]MemoryMessage, string, error) {
	switch req.Provider {
	case "openai:completions":
		request, err := buildOpenAICompletionsRequest(req, tools)
		if err != nil {
			return nil, "", err
		}
		data, err := json.Marshal(request)
		if err != nil {
			return nil, "", fmt.Errorf("marshal request JSON: %w", err)
		}
		return request.Messages, string(data), nil
	case "anthropic:messages":
		return nil, "anthropic", errors.New("no anthropic")
	case "openai:responses", "oauth:codex":
		request, memory := buildOpenAIResponseRequest(req, tools)
		data, err := json.Marshal(request)
		if err != nil {
			return nil, "", fmt.Errorf("marshal request JSON: %w", err)
		}
		return memory, string(data), nil
	}
	return nil, "", fmt.Errorf("unsupported provider: %s", req.Provider)
}

func GettoolCallMemory(toolsValue []tools.ToolResult) []MemoryMessage {
	memory := make([]MemoryMessage, 0, len(toolsValue))
	for _, tool := range toolsValue {
		memory = append(memory, MemoryMessage{
			Role:       tool.Role,
			Content:    tool.Content,
			ToolCallID: tool.ToolCallID,
		})
	}
	return memory
}
