package agent

import (
	"orbit/internal/tools"
	"strings"
	"testing"
	"time"
)

func TestShouldPublishStreamUpdateThrottlesRapidEvents(t *testing.T) {
	start := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
	var lastUpdate time.Time

	if !shouldPublishStreamUpdate(&lastUpdate, start) {
		t.Fatal("first stream update was throttled")
	}
	if shouldPublishStreamUpdate(&lastUpdate, start.Add(streamUIUpdateInterval-time.Millisecond)) {
		t.Fatal("rapid stream update was published")
	}
	if !shouldPublishStreamUpdate(&lastUpdate, start.Add(streamUIUpdateInterval)) {
		t.Fatal("stream update at interval boundary was throttled")
	}
}

func TestMaxAgentIterations(t *testing.T) {
	if maxAgentIterations != 70 {
		t.Fatalf("maxAgentIterations = %d, want 70", maxAgentIterations)
	}
}

func TestParseResponseDetailsReadsCompletionFinishReason(t *testing.T) {
	response := `{"usage":{},"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"done"}}]}`
	toolCalls, memory, _, state, err := ParseResponseDetails("openai:completions", response)
	if err != nil {
		t.Fatal(err)
	}
	if len(toolCalls) != 0 || len(memory) != 1 || memory[0].Content != "done" || state.FinishReason != "stop" {
		t.Fatalf("parsed response = calls %#v, memory %#v, state %#v", toolCalls, memory, state)
	}
	if err := state.validate(toolCalls); err != nil {
		t.Fatalf("completed response rejected: %v", err)
	}
}

func TestResponseStateRejectsIncompleteResponse(t *testing.T) {
	for _, state := range []ResponseState{{FinishReason: "length"}, {Status: "incomplete"}, {Status: "completed", Incomplete: true}} {
		if err := state.validate(nil); err == nil {
			t.Fatalf("state %#v was accepted", state)
		}
	}
}

func TestParseResponseDetailsReadsResponsesStatus(t *testing.T) {
	response := `{"status":"completed","incomplete_details":null,"usage":{},"output":[{"type":"message","content":[{"type":"output_text","text":"done"}]}]}`
	toolCalls, memory, _, state, err := ParseResponseDetails("openai:responses", response)
	if err != nil {
		t.Fatal(err)
	}
	if len(toolCalls) != 0 || len(memory) != 1 || memory[0].Content != "done" || state.Status != "completed" || state.Incomplete {
		t.Fatalf("parsed response = calls %#v, memory %#v, state %#v", toolCalls, memory, state)
	}
}

func TestMakeToolRoundSignatureIgnoresCallID(t *testing.T) {
	makeCall := func(id string) []any {
		return []any{map[string]any{
			"id":       id,
			"function": map[string]any{"name": "read_file", "arguments": `{"path":"a.go"}`},
		}}
	}
	first, err := makeToolRoundSignature(makeCall("call-1"), []tools.ToolResult{{ToolCallID: "call-1", Content: "same"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := makeToolRoundSignature(makeCall("call-2"), []tools.ToolResult{{ToolCallID: "call-2", Content: "same"}})
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("signatures differ: %q != %q", first, second)
	}
	if strings.Contains(first, "call-1") {
		t.Fatalf("signature contains call id: %q", first)
	}
}
