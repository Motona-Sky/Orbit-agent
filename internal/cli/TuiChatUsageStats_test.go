package cli

import (
	"strings"
	"testing"

	"orbit/internal/agentui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestChatFirstWindowSizeCommitsHistoryAndLaterResizeOnlyUpdatesSize(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.transcript = append(m.transcript, chatTranscriptEntry{
		kind: transcriptMessage, role: "assistant", content: "历史回答",
	})

	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(model)
	if cmd == nil {
		t.Fatal("first valid resize did not commit existing history")
	}
	if !m.screenInitialized || m.width != 80 || m.height != 24 || m.terminalTranscriptCursor != 1 {
		t.Fatalf("first resize state = initialized %v, size %dx%d, cursor %d", m.screenInitialized, m.width, m.height, m.terminalTranscriptCursor)
	}
	view := ansi.Strip(m.View())
	if strings.Contains(view, "历史回答") {
		t.Fatalf("stable history repeated in dynamic view: %q", view)
	}
	if !strings.Contains(view, m.messages.Chat.AgentCommandPlaceholder) {
		t.Fatalf("first resize lost composer: %q", view)
	}

	updated, cmd = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(model)
	if cmd != nil {
		t.Fatal("later resize unexpectedly reprinted transcript")
	}
	if m.width != 100 || m.height != 30 || m.terminalTranscriptCursor != 1 {
		t.Fatalf("later resize state = size %dx%d, cursor %d", m.width, m.height, m.terminalTranscriptCursor)
	}
}

func TestUsagePanelShowsStatsAndTasks(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	updated, _ := m.handleAgentUIEvent(agentui.UsageEvent{
		Stats: agentui.UsageStats{
			TodayTokens:  1234,
			CacheHitRate: 42.5,
		},
	})
	m = updated.(model)
	m.tasks = []agentui.TaskItem{
		{Title: "检查代码", Status: agentui.TaskRunning},
	}

	panel := ansi.Strip(m.renderWorkPromptPanel(32, 7))
	for _, want := range []string{
		"使用统计",
		"今日 Token 1,234",
		"缓存命中 42.5%",
		"检查代码",
	} {
		if !strings.Contains(panel, want) {
			t.Fatalf("panel = %q, want %q", panel, want)
		}
	}
}

func TestUsagePanelUsesEnglishTitleAndLabels(t *testing.T) {
	m := NewModelForLanguage("en")
	m.usageStats = agentui.UsageStats{
		TodayTokens:  25,
		CacheHitRate: 20,
	}

	panel := ansi.Strip(m.renderWorkPromptPanel(32, 6))
	for _, want := range []string{"Usage stats", "Today tokens 25", "Cache hit 20.0%"} {
		if !strings.Contains(panel, want) {
			t.Fatalf("panel = %q, want %q", panel, want)
		}
	}
}

func TestUsageStatsPanelDoesNotOverflowNarrowScreens(t *testing.T) {
	for _, width := range []int{24, 60} {
		m := NewModelForLanguage("zh-CN")
		m.width = width
		m.height = 20
		m.screenInitialized = true
		m.usageStats = agentui.UsageStats{
			TodayTokens:  123456789,
			CacheHitRate: 100,
		}

		for index, line := range strings.Split(m.View(), "\n") {
			if got := ansi.StringWidth(line); got >= width {
				t.Fatalf("width %d line %d = %d, want < %d: %q", width, index, got, width, ansi.Strip(line))
			}
		}
	}
}

func TestUsageStatsAndTasksFitAtWorkDockBreakpoint(t *testing.T) {
	for _, width := range []int{71, 72} {
		m := NewModelForLanguage("zh-CN")
		m.usageStats = agentui.UsageStats{
			TodayTokens:  1234,
			CacheHitRate: 42.5,
		}
		m.tasks = []agentui.TaskItem{
			{Title: "检查代码", Status: agentui.TaskRunning},
		}

		dock := ansi.Strip(m.renderWorkDock(terminalContentWidth(width), m.messages.Chat.CompactFooterHelp))
		for _, want := range []string{"使用统计", "今日 Token", "缓存命中", "检查代码"} {
			if !strings.Contains(dock, want) {
				t.Fatalf("width %d dock = %q, want %q", width, dock, want)
			}
		}
		for index, line := range strings.Split(dock, "\n") {
			if got := ansi.StringWidth(line); got >= width {
				t.Fatalf("width %d line %d = %d, want < %d: %q", width, index, got, width, line)
			}
		}
	}
}
