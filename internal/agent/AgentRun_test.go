package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"orbit/internal/agentui"
	"orbit/internal/tools"
	"orbit/internal/utils"
	"strings"
	"testing"
	"time"
)

func TestRunAgentReturnsTerminalResponseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"usage":{},"output":[]}`))
	}))
	defer server.Close()

	oldSessionID, oldHistoryFolder := utils.SessionId, utils.ChatHistoryFolder
	utils.SessionId = "test-session"
	utils.ChatHistoryFolder = t.TempDir()
	t.Cleanup(func() {
		utils.SessionId = oldSessionID
		utils.ChatHistoryFolder = oldHistoryFolder
	})

	ui := agentui.New()
	defer ui.Close()
	err := RunAgent(context.Background(), RunAgentValue{
		Provider: "openai:responses",
		ApiKey:   "test-key",
		BaseUrl:  server.URL,
		Model:    "test-model",
	}, ui)
	if err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("RunAgent() error = %v, want terminal response error", err)
	}

	eventCh := make(chan agentui.Event, 4)
	go func() {
		for {
			event, eventErr := ui.Next()
			if eventErr != nil {
				return
			}
			eventCh <- event
		}
	}()
	timer := time.NewTimer(50 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case event := <-eventCh:
			if _, ok := event.(agentui.ResultEvent); ok {
				t.Fatalf("terminal error was emitted as ResultEvent: %#v", event)
			}
		case <-timer.C:
			return
		}
	}
}

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

func TestParseResponseDetailsReadsCompletionReasoningFallbacks(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{name: "reasoning content wins", message: `"reasoning_content":"primary","reasoning":"fallback"`, expected: "primary"},
		{name: "reasoning string", message: `"reasoning":"fallback"`, expected: "fallback"},
		{name: "reasoning details objects", message: `"reasoning_details":[{"text":"first"},{"content":"second"},{"summary":"third"}]`, expected: "first\nsecond\nthird"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := `{"usage":{},"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"done",` + test.message + `}}]}`
			_, memory, _, _, err := ParseResponseDetails("openai:completions", response)
			if err != nil {
				t.Fatal(err)
			}
			if len(memory) != 1 || memory[0].ReasoningContent != test.expected {
				t.Fatalf("reasoning content = %#v, want %q", memory, test.expected)
			}
		})
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

func TestParseResponseDetailsReadsResponsesReasoningVariants(t *testing.T) {
	response := `{"status":"completed","incomplete_details":null,"usage":{},"output":[{"type":"reasoning","summary":[{"text":"shared"},{"content":"summary content"}],"content":[{"text":"shared"},{"summary":"content summary"}],"text":"item text"},{"type":"reasoning","summary":"plain summary","content":"plain content"}]}`
	_, memory, _, _, err := ParseResponseDetails("openai:responses", response)
	if err != nil {
		t.Fatal(err)
	}
	want := "shared\nsummary content\ncontent summary\nitem text\nplain summary\nplain content"
	if len(memory) != 1 || memory[0].ReasoningContent != want {
		t.Fatalf("reasoning content = %#v, want %q", memory, want)
	}
	if len(memory[0].ResponseItems) != 2 {
		t.Fatalf("response items = %#v", memory[0].ResponseItems)
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
