package cli

import (
	"strings"
	"testing"

	"looporbit/internal/llm"
	"looporbit/internal/memorys"
	"looporbit/internal/utils"
)

func TestNewModelForSessionRestoresVisibleConversation(t *testing.T) {
	session := memorys.SessionSummary{ID: "existing-id", Messages: []llm.MemoryMessage{
		{Role: "system", Content: "system"},
		{Role: "user", Content: "first"},
		{Role: "assistant", ToolCalls: []any{map[string]any{"id": "call"}}},
		{Role: "tool", Content: "result"},
		{Role: "assistant", Content: "answer"},
	}}
	m := NewModelForSession("zh-CN", session)
	if m.sessionID != "existing-id" || len(m.transcript) != 2 {
		t.Fatalf("session=%q transcript=%#v", m.sessionID, m.transcript)
	}
	if m.transcript[0].role != "user" || m.transcript[0].content != "first" {
		t.Fatalf("first=%#v", m.transcript[0])
	}
	if m.transcript[1].role != "assistant" || m.transcript[1].content != "answer" {
		t.Fatalf("second=%#v", m.transcript[1])
	}
}

func TestRestoredModelInitKeepsSessionID(t *testing.T) {
	oldID := utils.SessionId
	t.Cleanup(func() { utils.SessionId = oldID })
	m := NewModelForSession("en", memorys.SessionSummary{ID: "existing-id"})
	_ = m.Init()
	if utils.SessionId != "existing-id" {
		t.Fatalf("session id=%q", utils.SessionId)
	}
}

func TestRestoredPromptCarriesSessionID(t *testing.T) {
	m := NewModelForSession("en", memorys.SessionSummary{ID: "existing-id"})
	event := m.eventForPrompt("continue")
	if event.SessionId != "existing-id" || !event.ResumeSession {
		t.Fatalf("event=%#v", event)
	}
}

func TestRestoredTranscriptStartsPendingTerminalOutput(t *testing.T) {
	m := NewModelForSession("en", memorys.SessionSummary{ID: "existing-id", Messages: []llm.MemoryMessage{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "answer"},
	}})
	output, end := m.pendingTerminalTranscriptOutput(80)
	if end != 2 || !strings.Contains(output, "first") || !strings.Contains(output, "answer") {
		t.Fatalf("end=%d output=%q", end, output)
	}
}
