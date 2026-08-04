package cli

import (
	"strings"
	"testing"

	"orbit/internal/agentui"

	"github.com/charmbracelet/x/ansi"
)

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
