package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"looporbit/internal/tools"

	"github.com/imroc/req/v3"
)

// RequProvider 根据供应商类型发送 HTTP 请求。
func RequProvider(apikey string, baseurl string, provider string, data string) (error, string) {
	switch {
	case provider == "openai:completions":
		// OpenAI Completion
		client := req.C()
		resp, err := client.R().
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
			return errors.New("request failed: " + resp.Status), ""
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
}

// openAICompletionsMessages 封装请求的消息列表。
type openAICompletionsMessages struct {
	Messages []MemoryMessage `json:"messages"`
}

// openAICompletionsModel 封装请求使用的模型名称。
type openAICompletionsModel struct {
	Model string `json:"model"`
}

// openAIThinking 描述服务端思考模式的开关状态。
type openAIThinking struct {
	Type string `json:"type"`
}

// openAICompletionsThinking 封装思考模式及推理强度。
type openAICompletionsThinking struct {
	Thinking        openAIThinking `json:"thinking"`
	ReasoningEffort string         `json:"reasoning_effort"`
}

// openAIResponseFormat 描述模型响应的输出格式。
type openAIResponseFormat struct {
	Type string `json:"type"`
}

// openAICompletionsOptions 保存请求中始终输出的固定选项。
type openAICompletionsOptions struct {
	MaxTokens      int                  `json:"max_tokens"`
	ResponseFormat openAIResponseFormat `json:"response_format"`
	Stop           any                  `json:"stop"` // nil 会序列化为 null。
	Stream         bool                 `json:"stream"`
	StreamOptions  any                  `json:"stream_options"` // nil 会序列化为 null。
	Temperature    float64              `json:"temperature"`
	TopP           float64              `json:"top_p"`
	Tools          json.RawMessage      `json:"tools,omitempty"` // 保留原始 JSON 顺序
	ToolChoice     string               `json:"tool_choice,omitempty"`
	Logprobs       bool                 `json:"logprobs"`
	TopLogprobs    *int                 `json:"top_logprobs"` // nil 会序列化为 null。
}

// openAICompletionsRequest 组合各请求模块，统一生成最终 JSON。
type openAICompletionsRequest struct {
	openAICompletionsMessages
	openAICompletionsModel
	openAICompletionsThinking
	openAICompletionsOptions
}

// buildOpenAICompletionsMessages 构建 system 和 user 消息+memory。
func buildOpenAICompletionsMessages(systemPrompt, userInput string, memory []MemoryMessage) openAICompletionsMessages {
	// 合并 memory 到 messages 中
	messages := make([]MemoryMessage, 0, len(memory)+2)
	messages = append(messages, memory...)
	if len(messages) == 0 && systemPrompt != "" {
		messages = append(messages, MemoryMessage{Role: "system", Content: systemPrompt})
	}
	if userInput != "" {
		messages = append(messages, MemoryMessage{Role: "user", Content: userInput})
	}
	return openAICompletionsMessages{Messages: messages}
}

// buildOpenAICompletionsModel 构建模型模块。
func buildOpenAICompletionsModel(model string) openAICompletionsModel {
	return openAICompletionsModel{Model: model}
}

// buildOpenAICompletionsThinking 根据推理强度构建思考模块。
func buildOpenAICompletionsThinking(thinkLevel string) openAICompletionsThinking {
	thinkingType := "disabled"
	// 只要指定了推理强度，就启用思考模式。
	if thinkLevel != "" {
		thinkingType = "enabled"
	}

	return openAICompletionsThinking{
		Thinking:        openAIThinking{Type: thinkingType},
		ReasoningEffort: thinkLevel,
	}
}

// buildOpenAICompletionsOptions 构建协议要求始终输出的默认选项和工具列表。
func buildOpenAICompletionsOptions(tools json.RawMessage) openAICompletionsOptions {
	opts := openAICompletionsOptions{
		MaxTokens:      4096,
		ResponseFormat: openAIResponseFormat{Type: "text"},
		Temperature:    1,
		TopP:           1,
	}
	// 只有工具列表非空时才设置 tools 和 tool_choice，
	// 避免发送 tools:[] + tool_choice:"none" 导致部分 API 返回 400。
	if len(tools) > 2 { // 非空数组至少是 "[]" 两个字符
		opts.Tools = tools
		opts.ToolChoice = "auto"
	}
	return opts
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
	switch {
	case req.Provider == "openai:completions":
		// 记忆结构尚未确定；nil 和空 map 均作为无历史消息处理。
		toolsJson, err := json.Marshal(tools)
		if err != nil {
			return nil, "", fmt.Errorf("marshal tools JSON: %w", err)
		}
		Messages := buildOpenAICompletionsMessages(req.SystemPrompt, req.UserInput, req.Memory)
		request := openAICompletionsRequest{
			openAICompletionsMessages: Messages,
			openAICompletionsModel:    buildOpenAICompletionsModel(req.Model),
			openAICompletionsThinking: buildOpenAICompletionsThinking(req.ThinkLevel),
			openAICompletionsOptions:  buildOpenAICompletionsOptions(json.RawMessage(toolsJson)),
		}

		data, err := json.Marshal(request)
		if err != nil {
			return nil, "", fmt.Errorf("marshal request JSON: %w", err)
		}
		mem := request.openAICompletionsMessages.Messages
		if err != nil {
			return nil, "", fmt.Errorf("marshal messages JSON: %w", err)
		}
		return mem, string(data), nil
	case req.Provider == "anthropic:messages":
		return nil, "anthropic", errors.New("no anthropic")
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
