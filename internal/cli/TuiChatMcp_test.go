package cli

import (
	"strings"
	"testing"

	"orbit/internal/mcp"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMCPCommandOpensEmbeddedPageAndEscReturns(t *testing.T) {
	m := NewModelForLanguage("en")
	updated, cmd := m.handleSlashMessageSubmit(mcpCommand)
	if updated.mcpPage == nil || cmd == nil {
		t.Fatal("/mcp did not open embedded page")
	}
	updated.setupScreenReady = true
	modelValue, exitCmd := updated.updateMcpPage(tea.KeyMsg{Type: tea.KeyEsc})
	closed := modelValue.(model)
	if closed.mcpPage != nil || exitCmd == nil {
		t.Fatal("Esc did not return to chat")
	}
}

func TestMCPPageRendersServiceDescriptionStatusAndError(t *testing.T) {
	m := NewModelForLanguage("en")
	m.width = 100
	m.mcpPage = &mcpPageState{statuses: []mcp.ServiceStatus{{
		Name: "broken", Description: "Configured description", Enabled: true,
		State: mcp.StateError, Error: "recent stderr",
	}}}
	view := m.renderMcpPage()
	for _, expected := range []string{"broken", "Configured description", "error", "recent stderr"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("MCP page missing %q: %q", expected, view)
		}
	}
}

func TestMainViewShowsMCPErrorSummary(t *testing.T) {
	m := NewModelForLanguage("en")
	m.mcpStatuses = []mcp.ServiceStatus{{Name: "broken", State: mcp.StateError}}
	if got := strings.Join(m.workPromptLines(), "\n"); !strings.Contains(got, "broken") || !strings.Contains(got, "/mcp") {
		t.Fatalf("summary = %q", got)
	}
}
