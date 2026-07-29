package llm

import "testing"

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
