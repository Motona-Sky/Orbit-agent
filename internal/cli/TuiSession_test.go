package cli

import (
	"strings"
	"testing"

	"looporbit/internal/memorys"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func sessionFixtures(count int) []memorys.SessionSummary {
	items := make([]memorys.SessionSummary, count)
	for i := range items {
		items[i] = memorys.SessionSummary{ID: string(rune('a'+i)) + "-id", FirstUserMessage: "first question"}
	}
	return items
}

func TestSessionSelectorMovesAndConfirms(t *testing.T) {
	m := initialSessionModel("zh-CN", sessionFixtures(3), 1)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(SessionModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(SessionModel)
	if !m.Confirmed || m.SelectedSession.ID != "b-id" || cmd == nil {
		t.Fatalf("model=%#v cmd=%v", m, cmd)
	}
}

func TestSessionSelectorSupportsJKAndCancelKeys(t *testing.T) {
	m := initialSessionModel("en", sessionFixtures(2), 0)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(SessionModel)
	if m.cursor != 1 {
		t.Fatalf("j cursor=%d", m.cursor)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = updated.(SessionModel)
	if m.cursor != 0 {
		t.Fatalf("k cursor=%d", m.cursor)
	}

	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("q")},
		{Type: tea.KeyEsc},
		{Type: tea.KeyCtrlC},
	} {
		m := initialSessionModel("en", sessionFixtures(2), 0)
		updated, cmd := m.Update(key)
		result := updated.(SessionModel)
		if result.Confirmed || cmd == nil {
			t.Fatalf("key=%q model=%#v cmd=%v", key.String(), result, cmd)
		}
	}
}

func TestSessionSelectorEmptyStateCannotConfirm(t *testing.T) {
	m := initialSessionModel("zh-CN", nil, 2)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := updated.(SessionModel)
	if result.Confirmed || cmd != nil || !strings.Contains(result.View(), "没有可恢复的历史会话") {
		t.Fatalf("model=%#v cmd=%v view=%q", result, cmd, result.View())
	}
}

func TestSessionSelectorShowsSkippedCountAndFitsWidth(t *testing.T) {
	m := initialSessionModel("zh-CN", []memorys.SessionSummary{{
		ID: "session-id", FirstUserMessage: strings.Repeat("很长的摘要", 20),
	}}, 3)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	view := updated.(SessionModel).View()
	if !strings.Contains(view, "已跳过 3 个无效会话") {
		t.Fatalf("view=%q", view)
	}
	for line := range strings.SplitSeq(view, "\n") {
		if lipgloss.Width(line) > 40 {
			t.Fatalf("line width=%d line=%q", lipgloss.Width(line), line)
		}
	}
}

func TestSessionSelectorWindowKeepsCursorReachable(t *testing.T) {
	m := initialSessionModel("en", sessionFixtures(12), 0)
	m.terminalHeight = 12
	for range 11 {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(SessionModel)
	}
	if m.cursor != 11 || !strings.Contains(m.View(), "l-id") {
		t.Fatalf("cursor=%d view=%q", m.cursor, m.View())
	}
}

func TestSessionSelectorWindowKeepsActiveCursorVisible(t *testing.T) {
	m := initialSessionModel("en", sessionFixtures(12), 0)
	for range 7 {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(SessionModel)
	}
	view := ansi.Strip(m.View())
	if !strings.Contains(view, ">  h-id") {
		t.Fatalf("cursor=%d active option is not visible:\n%s", m.cursor, view)
	}
}

func TestSessionSelectorWindowUsesAbsoluteSequenceNumbers(t *testing.T) {
	m := initialSessionModel("en", sessionFixtures(12), 0)
	for range 7 {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(SessionModel)
	}
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "6   f-id") || strings.Contains(view, "1   f-id") {
		t.Fatalf("cursor=%d sequence numbers restarted after scrolling:\n%s", m.cursor, view)
	}
}
