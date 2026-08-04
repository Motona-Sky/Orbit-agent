package llm

import (
	"encoding/json"
	"testing"
)

func TestGenReqJsonPvBuildsInitialSystemAndUserMessages(t *testing.T) {
	req := GenReqJsonPvRequest{
		Provider:     "openai:completions",
		Model:        "test-model",
		SystemPrompt: "system prompt",
		UserInput:    "first question",
		Memory:       []MemoryMessage{},
	}

	messages, _, err := GenReqJsonPv(req, nil)
	if err != nil {
		t.Fatalf("GenReqJsonPv() error = %v", err)
	}
	want := []MemoryMessage{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "first question"},
	}
	assertMemoryMessages(t, messages, want)
}

func TestGenReqJsonPvAppendsCurrentUserToExistingHistory(t *testing.T) {
	history := []MemoryMessage{
		{Role: "system", Content: "original system"},
		{Role: "user", Content: "earlier question"},
		{Role: "assistant", Content: "earlier answer"},
	}
	req := GenReqJsonPvRequest{
		Provider:     "openai:completions",
		Model:        "test-model",
		SystemPrompt: "replacement system must not be appended",
		UserInput:    "current question",
		Memory:       history,
	}

	messages, _, err := GenReqJsonPv(req, nil)
	if err != nil {
		t.Fatalf("GenReqJsonPv() error = %v", err)
	}
	want := append(append([]MemoryMessage{}, history...), MemoryMessage{
		Role: "user", Content: "current question",
	})
	assertMemoryMessages(t, messages, want)
}

func TestGenReqJsonPvDoesNotMutateHistoryBackingArray(t *testing.T) {
	history := make([]MemoryMessage, 1, 2)
	history[0] = MemoryMessage{Role: "system", Content: "original system"}

	_, _, err := GenReqJsonPv(GenReqJsonPvRequest{
		Provider:  "openai:completions",
		Model:     "test-model",
		UserInput: "current question",
		Memory:    history,
	}, nil)
	if err != nil {
		t.Fatalf("GenReqJsonPv() error = %v", err)
	}

	expanded := history[:2]
	if expanded[1].Role != "" || expanded[1].Content != "" {
		t.Fatalf("history backing array was mutated: %#v", expanded[1])
	}
}

func TestGenReqJsonPvBuildsOpenAIResponsesRequest(t *testing.T) {
	history := []MemoryMessage{{Role: "assistant", Content: "earlier answer"}}
	messages, data, err := GenReqJsonPv(GenReqJsonPvRequest{
		Provider:     "openai:responses",
		Model:        "test-model",
		SystemPrompt: "system prompt",
		UserInput:    "current question",
		ThinkLevel:   "low",
		Memory:       history,
	}, nil)
	if err != nil {
		t.Fatalf("GenReqJsonPv() error = %v", err)
	}

	wantMessages := []MemoryMessage{
		{Role: "assistant", Content: "earlier answer"},
		{Role: "user", Content: "current question"},
	}
	assertMemoryMessages(t, messages, wantMessages)

	var request OpenaiResponse
	if err := json.Unmarshal([]byte(data), &request); err != nil {
		t.Fatalf("unmarshal request JSON: %v", err)
	}
	if request.Model != "test-model" || request.Instructions != "system prompt" {
		t.Fatalf("unexpected responses request: %#v", request)
	}
	if request.MaxOutputTokens != 4096 || request.Text.Format.Type != "text" {
		t.Fatalf("unexpected responses defaults: %#v", request)
	}
	if request.Reasoning == nil || request.Reasoning.Effort != "low" {
		t.Fatalf("unexpected reasoning: %#v", request.Reasoning)
	}
}

func TestOpenAIResponsesDoesNotMutateHistoryBackingArray(t *testing.T) {
	history := make([]MemoryMessage, 1, 2)
	history[0] = MemoryMessage{Role: "assistant", Content: "earlier answer"}

	_, _, err := GenReqJsonPv(GenReqJsonPvRequest{
		Provider:  "openai:responses",
		Model:     "test-model",
		UserInput: "current question",
		Memory:    history,
	}, nil)
	if err != nil {
		t.Fatalf("GenReqJsonPv() error = %v", err)
	}

	if expanded := history[:2]; expanded[1].Role != "" || expanded[1].Content != "" {
		t.Fatalf("history backing array was mutated: %#v", expanded[1])
	}
}

func TestOpenAIResponsesConvertsToolMemoryAtRequestBoundary(t *testing.T) {
	history := []MemoryMessage{
		{
			Role: "assistant",
			ToolCalls: []any{map[string]any{
				"id": "call-1",
				"function": map[string]any{
					"name":      "read_file",
					"arguments": `{"path":"a.txt"}`,
				},
			}},
		},
		{Role: "tool", ToolCallID: "call-1", Content: "file content"},
	}

	memory, data, err := GenReqJsonPv(GenReqJsonPvRequest{
		Provider: "openai:responses",
		Model:    "test-model",
		Memory:   history,
	}, nil)
	if err != nil {
		t.Fatalf("GenReqJsonPv() error = %v", err)
	}
	assertMemoryMessages(t, memory, history)

	var request map[string]any
	if err := json.Unmarshal([]byte(data), &request); err != nil {
		t.Fatalf("unmarshal request JSON: %v", err)
	}
	input, ok := request["input"].([]any)
	if !ok || len(input) != 2 {
		t.Fatalf("input = %#v, want function call and output", request["input"])
	}
	call := input[0].(map[string]any)
	if call["type"] != "function_call" || call["call_id"] != "call-1" || call["id"] != nil {
		t.Fatalf("function call = %#v", call)
	}
	output := input[1].(map[string]any)
	if output["type"] != "function_call_output" || output["call_id"] != "call-1" || output["output"] != "file content" {
		t.Fatalf("function output = %#v", output)
	}
}

func assertMemoryMessages(t *testing.T, got, want []MemoryMessage) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("message count = %d, want %d: %#v", len(got), len(want), got)
	}
	for index := range want {
		if got[index].Role != want[index].Role || got[index].Content != want[index].Content {
			t.Fatalf("message[%d] = %#v, want %#v", index, got[index], want[index])
		}
	}
}
