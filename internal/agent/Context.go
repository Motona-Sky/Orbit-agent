package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"orbit/internal/llm"
	"orbit/internal/utils"
	"strings"
)

func GetConLength(provider string, usage map[string]any) float64 {
	switch provider {
	case "openai:completions":
		conlen := usage["prompt_tokens"].(float64)
		return conlen
	case "openai:response", "openai:responses", "oauth:codex":
		conlen := usage["input_tokens"].(float64)
		return conlen
	}
	return 0
}
func CompressContext(context []llm.MemoryMessage) ([]llm.MemoryMessage, error) {
	var newContext []llm.MemoryMessage
	var tmpContext []llm.MemoryMessage

	for _, v := range context {
		switch v.Role {
		case "system":
			newContext = append(newContext, v)

		case "user", "tool":
			tmpContext = append(tmpContext, v)

		case "assistant":
			tmpContext = append(tmpContext, v)

			// 有工具调用，说明当前对话还没结束。
			if len(v.ToolCalls) > 0 {
				continue
			}

			// 没有工具调用，认为这是这一轮的最终回答。
			comcon, err := requestSummary(tmpContext)
			if err != nil {
				return nil, err
			}

			newContext = append(newContext, llm.MemoryMessage{
				Role:    "assistant",
				Content: comcon,
			})

			// 当前轮次已经压缩完毕。
			tmpContext = nil
		}
	}

	// 保留没有完成的最后一轮对话。
	if len(tmpContext) > 0 {
		newContext = append(newContext, tmpContext...)
	}

	return newContext, nil
}

// requestSummary 序列化旧消息并请求 LLM 生成摘要。
func requestSummary(old []llm.MemoryMessage) (string, error) {
	input, err := json.Marshal(old)
	if err != nil {
		return "", fmt.Errorf("marshal old messages: %w", err)
	}
	req := llm.GenReqJsonPvRequest{
		Provider: utils.Provider,
		Model:    utils.Model,
		SystemPrompt: `You are performing a context checkpoint compression.

Create a concise handoff summary for the next LLM that will continue the task. Include:

* Current progress and key decisions
* Important context, constraints, and user preferences
* Unfinished work and clear next steps
* Essential data, examples, and references needed to continue

Structure the summary clearly and focus only on information required for a seamless handoff.

Submit the summary as role: assistant. The original input remains role: user.
`,
		UserInput:  string(input),
		ThinkLevel: "low",
	}
	// 传 nil 工具列表，压缩请求不需要工具调用。
	_, data, err := llm.GenReqJsonPv(req, nil)
	if err != nil {
		return "", fmt.Errorf("gen compress req err: %w", err)
	}
	err, respJSON := llm.RequProvider(utils.ApiKey, utils.BaseUrl, utils.Provider, data)
	if err != nil {
		return "", err
	}
	_, assistantMemory, _, err := ParseResponse(utils.Provider, respJSON)
	if err != nil {
		return "", err
	}
	if len(assistantMemory) == 0 || strings.TrimSpace(assistantMemory[0].Content) == "" {
		return "", errors.New("compress summary is empty")
	}
	return assistantMemory[0].Content, nil
}

// func llmcompressContext(context []llm.MemoryMessage) ([]llm.MemoryMessage, error) {
// 	var newContext []llm.MemoryMessage

// 	return CompressContext(context)
// }
