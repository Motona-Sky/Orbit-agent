package cli

import (
	"errors"
	"strings"
	"testing"
	"time"

	"looporbit/internal/agentui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestActivityEventsAggregateWithoutEnteringTranscript(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.running = true

	first, _ := m.handleAgentUIEvent(agentui.ActivityEvent{Kind: agentui.ActivityFile, Target: "config.go"})
	m = first.(model)
	m.activitiesExpanded = true
	second, _ := m.handleAgentUIEvent(agentui.ActivityEvent{Kind: agentui.ActivityTool, Target: "go test ./internal/..."})
	m = second.(model)

	if len(m.activities) != 2 || len(m.transcript) != 0 {
		t.Fatalf("activities = %d, transcript = %d; want 2 dynamic and 0 stable", len(m.activities), len(m.transcript))
	}
	if m.activitiesExpanded {
		t.Fatal("new activity did not automatically collapse the group")
	}
	line := ansi.Strip(m.renderActivityGroup(100))
	for _, want := range []string{"2 次工具调用", "执行 go test ./internal/...", "Ctrl+O"} {
		if !strings.Contains(line, want) {
			t.Fatalf("collapsed activity = %q, want %q", line, want)
		}
	}
}

func TestLatestActivityControlsRunningText(t *testing.T) {
	tests := []struct {
		kind agentui.ActivityKind
		want string
	}{
		{kind: agentui.ActivityFile, want: "读取中"},
		{kind: agentui.ActivityTool, want: "执行中"},
	}
	for _, test := range tests {
		m := NewModelForLanguage("zh-CN")
		m.running = true
		m.recordActivity(agentui.ActivityEvent{Kind: test.kind, Target: "target"})
		if status := ansi.Strip(m.renderRunningStatus()); !strings.Contains(status, test.want) {
			t.Fatalf("kind %q status = %q, want %q", test.kind, status, test.want)
		}
	}
}

func TestCtrlOTogglesAllActivityDetails(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.recordActivity(agentui.ActivityEvent{Kind: agentui.ActivityFile, Target: "a.go"})
	m.recordActivity(agentui.ActivityEvent{Kind: agentui.ActivityTool, Target: "go test"})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = updated.(model)
	expanded := ansi.Strip(m.renderActivityGroup(100))
	for _, want := range []string{"a.go", "go test", "Ctrl+O 折叠"} {
		if !strings.Contains(expanded, want) {
			t.Fatalf("expanded group = %q, want %q", expanded, want)
		}
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	m = updated.(model)
	if m.activitiesExpanded {
		t.Fatal("second Ctrl+O did not collapse activities")
	}
}

func TestQuestionTakesPriorityOverActivityGroup(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.running = true
	m.recordActivity(agentui.ActivityEvent{Kind: agentui.ActivityTool, Target: "go test"})
	m.transcript = append(m.transcript, chatTranscriptEntry{
		kind: transcriptQuestion, content: "继续吗？", pending: true, options: []string{"是", "否"}, selected: 0,
	})
	m.activeQuestion = len(m.transcript) - 1

	status := ansi.Strip(m.renderInlineStatus(100))
	if !strings.Contains(status, "继续吗？") || strings.Contains(status, "工具调用") {
		t.Fatalf("question-priority status = %q", status)
	}
	m.transcript[m.activeQuestion].pending = false
	m.activeQuestion = historyCursorIdle
	if status := ansi.Strip(m.renderInlineStatus(100)); !strings.Contains(status, "1 次工具调用") {
		t.Fatalf("activity group did not return after question: %q", status)
	}
}

func TestStaleAgentUIActivityDoesNotEnterCurrentGroup(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.agentUI = agentui.New()
	stale := agentui.New()
	defer m.agentUI.Close()
	defer stale.Close()

	updated, _ := m.Update(agentUIEventMsg{ui: stale, event: agentui.ActivityEvent{Kind: agentui.ActivityTool, Target: "stale"}})
	if got := len(updated.(model).activities); got != 0 {
		t.Fatalf("stale activity count = %d, want 0", got)
	}
}

func TestActivityGroupDoesNotOverflowTerminalWidth(t *testing.T) {
	for _, width := range []int{23, 59, 99} {
		m := NewModelForLanguage("zh-CN")
		m.width = width + terminalRightMargin
		m.running = true
		m.recordActivity(agentui.ActivityEvent{Kind: agentui.ActivityFile, Target: strings.Repeat("很长的路径/", 20)})
		for _, expanded := range []bool{false, true} {
			m.activitiesExpanded = expanded
			for index, line := range strings.Split(m.renderActivityGroup(width), "\n") {
				if cells := ansi.StringWidth(line); cells > width {
					t.Fatalf("width %d expanded %v line %d cells = %d", width, expanded, index, cells)
				}
			}
		}
	}
}

func TestRuntimeActivityTreeShowsParentAboveCollapsedChild(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.running = true
	m.runningStatus.elapsed = time.Second
	m.recordActivity(agentui.ActivityEvent{Kind: agentui.ActivityTool, Target: "go test"})

	lines := strings.Split(ansi.Strip(m.renderRuntimeActivityTree(100)), "\n")
	if len(lines) != 2 {
		t.Fatalf("collapsed tree has %d lines, want 2: %#v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "◉ LOOPORBIT") || !strings.Contains(lines[0], "执行中") {
		t.Fatalf("parent line = %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "└─ ") || !strings.Contains(lines[1], "1 次工具调用") {
		t.Fatalf("child line = %q", lines[1])
	}
}

func TestRuntimeActivityTreeUsesBranchesWhenExpanded(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.running = true
	m.recordActivity(agentui.ActivityEvent{Kind: agentui.ActivityFile, Target: "config.go"})
	m.recordActivity(agentui.ActivityEvent{Kind: agentui.ActivityTool, Target: "go test"})
	m.activitiesExpanded = true

	lines := strings.Split(ansi.Strip(m.renderRuntimeActivityTree(100)), "\n")
	if len(lines) != 4 {
		t.Fatalf("expanded tree has %d lines, want 4: %#v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "◉ LOOPORBIT") ||
		!strings.HasPrefix(lines[1], "├─ ") || !strings.Contains(lines[1], "config.go") ||
		!strings.HasPrefix(lines[2], "├─ ") || !strings.Contains(lines[2], "go test") ||
		!strings.HasPrefix(lines[3], "└─ ") || !strings.Contains(lines[3], "Ctrl+O 折叠") {
		t.Fatalf("expanded tree = %#v", lines)
	}
}

func TestRuntimeActivityTreeDoesNotOverflowTerminalWidth(t *testing.T) {
	for _, width := range []int{23, 59, 99} {
		m := NewModelForLanguage("zh-CN")
		m.running = true
		m.recordActivity(agentui.ActivityEvent{Kind: agentui.ActivityFile, Target: strings.Repeat("很长的路径/", 20)})
		for _, expanded := range []bool{false, true} {
			m.activitiesExpanded = expanded
			for index, line := range strings.Split(m.renderRuntimeActivityTree(width), "\n") {
				if cells := ansi.StringWidth(line); cells > width {
					t.Fatalf("width %d expanded %v line %d cells = %d", width, expanded, index, cells)
				}
			}
		}
	}
}

func TestActivitySummaryIsCommittedOnceOnEveryTerminalPath(t *testing.T) {
	tests := []struct {
		name string
		stop func(model) model
	}{
		{name: "result", stop: func(m model) model {
			updated, _ := m.handleAgentUIEvent(agentui.ResultEvent{Text: "done"})
			return updated.(model)
		}},
		{name: "error", stop: func(m model) model {
			ui := m.agentUI
			updated, _ := m.Update(agentRunFinishedMsg{ui: ui, err: errors.New("boom")})
			return updated.(model)
		}},
		{name: "cancel", stop: func(m model) model {
			updated, _ := m.cancelRunningTurn()
			return updated
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := NewModelForLanguage("zh-CN")
			m.running = true
			m.agentUI = agentui.New()
			m.recordActivity(agentui.ActivityEvent{Kind: agentui.ActivityFile, Target: "a.go"})
			m.recordActivity(agentui.ActivityEvent{Kind: agentui.ActivityTool, Target: "go test"})

			m = test.stop(m)
			summaries := 0
			for _, entry := range m.transcript {
				if entry.kind == transcriptActivitySummary {
					summaries++
					if entry.content != "› 2 次工具调用" {
						t.Fatalf("summary = %q", entry.content)
					}
				}
			}
			if summaries != 1 || len(m.activities) != 0 || m.activitiesExpanded {
				t.Fatalf("summaries = %d, activities = %d, expanded = %v", summaries, len(m.activities), m.activitiesExpanded)
			}
		})
	}
}

func TestNoActivityDoesNotCreateEmptySummary(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.appendActivitySummary()
	if len(m.transcript) != 0 {
		t.Fatalf("empty activity group created transcript: %#v", m.transcript)
	}
}
