package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"orbit/internal/llm"
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

// CompressCredential 包含压缩请求所需的认证信息。
type CompressCredential struct {
	Auth       string // "codex" 或其他
	Credential string // API Key 或 Access Token
	AccountID  string
	BaseUrl    string
	Provider   string
	Model      string
}

func CompressContext(ctx context.Context, messages []llm.MemoryMessage, cred CompressCredential) ([]llm.MemoryMessage, error) {
	var newContext []llm.MemoryMessage
	var toCompress []llm.MemoryMessage
	var tmpContext []llm.MemoryMessage

	for _, v := range messages {
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

			// 没有工具调用，认为这是这一轮的最终回答，收集待压缩消息。
			toCompress = append(toCompress, tmpContext...)
			tmpContext = nil
		}
	}

	// 将所有已完成的轮次合并为一次摘要请求。
	if len(toCompress) > 0 {
		comcon, err := requestSummary(ctx, toCompress, cred)
		if err != nil {
			return nil, err
		}
		// 使用 user + assistant 消息对，保证角色交替规范。
		newContext = append(newContext,
			llm.MemoryMessage{
				Role:    "user",
				Content: "[Context Checkpoint] The following is a compressed summary of our previous conversation.",
			},
			llm.MemoryMessage{
				Role:    "assistant",
				Content: comcon,
			},
		)
	}

	// 保留没有完成的最后一轮对话。
	if len(tmpContext) > 0 {
		newContext = append(newContext, tmpContext...)
	}

	return newContext, nil
}

// requestSummary 序列化旧消息并请求 LLM 生成摘要。
func requestSummary(ctx context.Context, old []llm.MemoryMessage, cred CompressCredential) (string, error) {
	input, err := json.Marshal(old)
	if err != nil {
		return "", fmt.Errorf("marshal old messages: %w", err)
	}
	req := llm.GenReqJsonPvRequest{
		Provider: cred.Provider,
		Model:    cred.Model,
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

	var reqErr error
	var respJSON string
	if cred.Auth == "codex" {
		reqErr, respJSON = llm.RequOuth(cred.Credential, cred.AccountID, cred.Provider, data)
	} else {
		reqErr, respJSON = llm.RequProvider(ctx, cred.Credential, cred.BaseUrl, cred.Provider, data)
	}
	if reqErr != nil {
		return "", reqErr
	}

	_, assistantMemory, _, err := ParseResponse(cred.Provider, respJSON)
	if err != nil {
		return "", err
	}
	if len(assistantMemory) == 0 || strings.TrimSpace(assistantMemory[0].Content) == "" {
		return "", errors.New("compress summary is empty")
	}
	return assistantMemory[0].Content, nil
}
