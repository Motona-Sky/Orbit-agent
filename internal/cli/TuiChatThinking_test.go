package cli

import (
	"errors"
	"strings"
	"testing"
	"time"

	"looporbit/internal/agentui"

	"github.com/charmbracelet/x/ansi"
)

func TestThinkingEventReplacesAndClearsWithoutTranscript(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.running = true

	updated, _ := m.Update(agentui.ThinkingEvent{Text: "  正在读取\n 配置  "})
	m = updated.(model)
	if m.thinkingText != "正在读取 配置" {
		t.Fatalf("first thinking text = %q", m.thinkingText)
	}

	updated, _ = m.Update(agentui.ThinkingEvent{Text: "正在验证配置"})
	m = updated.(model)
	if m.thinkingText != "正在验证配置" || len(m.transcript) != 0 || len(m.activities) != 0 {
		t.Fatalf("replacement state = text %q, transcript %d, activities %d", m.thinkingText, len(m.transcript), len(m.activities))
	}

	updated, _ = m.Update(agentui.ThinkingEvent{Text: " \n\t "})
	m = updated.(model)
	if m.thinkingText != "" {
		t.Fatalf("blank event left thinking text %q", m.thinkingText)
	}
}

func TestStaleAgentUIThinkingDoesNotReplaceCurrentText(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.agentUI = agentui.New()
	stale := agentui.New()
	defer m.agentUI.Close()
	defer stale.Close()
	m.thinkingText = "当前过程"

	updated, _ := m.Update(agentUIEventMsg{ui: stale, event: agentui.ThinkingEvent{Text: "旧过程"}})
	if got := updated.(model).thinkingText; got != "当前过程" {
		t.Fatalf("stale event changed thinking text to %q", got)
	}
}

func TestThinkingTextSurvivesNonTerminalEvents(t *testing.T) {
	startedAt := time.Date(2026, time.July, 23, 10, 0, 0, 0, time.UTC)
	m := NewModelForLanguage("zh-CN")
	m.running = true
	m.startRunningStatus(startedAt)
	m.thinkingText = "正在检查配置"

	updated, _ := m.handleAgentUIEvent(agentui.ActivityEvent{Kind: agentui.ActivityFile, Target: "config.go"})
	m = updated.(model)
	updated, _ = m.handleAgentUIEvent(agentui.TaskPlanEvent{Tasks: []agentui.TaskItem{{Title: "检查配置", Status: agentui.TaskRunning}}})
	m = updated.(model)
	m, _ = m.handleRunningStatusTick(runningStatusTickMsg{
		generation: m.runningStatus.generation,
		now:        startedAt.Add(time.Second),
	})

	if m.thinkingText != "正在检查配置" {
		t.Fatalf("non-terminal event changed thinking text to %q", m.thinkingText)
	}
}

func TestResultEventDisplaysWithoutEndingTurn(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.running = true
	m.agentUI = agentui.New()
	defer m.agentUI.Close()
	m.thinkingText = "正在调用工具"
	m.recordActivity(agentui.ActivityEvent{Kind: agentui.ActivityTool, Target: "检查配置"})

	updated, cmd := m.handleAgentUIEvent(agentui.ResultEvent{Text: "先检查配置。"})
	m = updated.(model)

	if !m.running || m.agentUI == nil {
		t.Fatal("non-final result ended the running turn")
	}
	if m.thinkingText != "正在调用工具" {
		t.Fatalf("non-final result cleared thinking text %q", m.thinkingText)
	}
	if len(m.transcript) != 2 || m.transcript[1].content != "先检查配置。" {
		t.Fatalf("transcript = %#v", m.transcript)
	}
	status := ansi.Strip(m.renderInlineStatus(100))
	if strings.Contains(status, "LOOPORBIT") {
		t.Fatalf("status redisplayed LoopOrbit after DisplayResult: %q", status)
	}
	if !strings.Contains(status, "正在调用工具") {
		t.Fatalf("status lost thinking text after DisplayResult: %q", status)
	}
	if cmd == nil {
		t.Fatal("non-final result did not wait for the next agent UI event")
	}
}

func TestFinalResultEventEndsTurnAfterMultipleResults(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.running = true
	m.agentUI = agentui.New()
	m.thinkingText = "正在收尾"

	updated, _ := m.handleAgentUIEvent(agentui.ResultEvent{Text: "第一段"})
	m = updated.(model)
	updated, _ = m.handleAgentUIEvent(agentui.ResultEvent{Text: "第二段"})
	m = updated.(model)
	updated, _ = m.handleAgentUIEvent(agentui.FinalResultEvent{Text: "最终回答"})
	m = updated.(model)

	if m.running || m.agentUI != nil || m.thinkingText != "" {
		t.Fatalf("final state = running %v, ui nil %v, thinking %q", m.running, m.agentUI == nil, m.thinkingText)
	}
	if len(m.transcript) != 3 {
		t.Fatalf("transcript length = %d, want 3", len(m.transcript))
	}
	for index, want := range []string{"第一段", "第二段", "最终回答"} {
		if m.transcript[index].content != want {
			t.Fatalf("transcript[%d] = %q, want %q", index, m.transcript[index].content, want)
		}
	}
}

func TestThinkingTextClearsOnEveryTerminalPath(t *testing.T) {
	tests := []struct {
		name string
		stop func(model) model
	}{
		{name: "result", stop: func(m model) model {
			updated, _ := m.handleAgentUIEvent(agentui.FinalResultEvent{Text: "done"})
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
			m.thinkingText = "内部进度"

			m = test.stop(m)
			if m.thinkingText != "" {
				t.Fatalf("terminal path left thinking text %q", m.thinkingText)
			}
			for _, entry := range m.transcript {
				if strings.Contains(entry.content, "内部进度") {
					t.Fatalf("thinking text entered transcript: %#v", m.transcript)
				}
			}
		})
	}
}

func TestNewTurnClearsStaleThinkingText(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.thinkingText = "旧过程"

	updated, cmd := m.handleVisibleUserMessageSubmit("新问题")
	defer updated.closeAgentUI()
	if updated.thinkingText != "" || cmd == nil {
		t.Fatalf("new turn state = text %q, command nil %v", updated.thinkingText, cmd == nil)
	}
}

func TestThinkingRendersAsFirstRuntimeChild(t *testing.T) {
	t.Run("without activities", func(t *testing.T) {
		m := NewModelForLanguage("zh-CN")
		m.running = true
		m.thinkingText = "正在检查配置"
		lines := strings.Split(ansi.Strip(m.renderRuntimeActivityTree(100)), "\n")
		if len(lines) != 2 || !strings.Contains(lines[0], "LOOPORBIT") ||
			!strings.HasPrefix(lines[1], "└─  ") || !strings.Contains(lines[1], "正在检查配置") {
			t.Fatalf("thinking-only tree = %#v", lines)
		}
	})

	t.Run("with collapsed activities", func(t *testing.T) {
		m := NewModelForLanguage("zh-CN")
		m.running = true
		m.thinkingText = "正在检查配置"
		m.recordActivity(agentui.ActivityEvent{Kind: agentui.ActivityFile, Target: "config.go"})
		lines := strings.Split(ansi.Strip(m.renderRuntimeActivityTree(100)), "\n")
		if len(lines) != 3 || !strings.HasPrefix(lines[1], "├─  ") ||
			!strings.Contains(lines[1], "正在检查配置") ||
			!strings.HasPrefix(lines[2], "└─ ") || !strings.Contains(lines[2], "1 次工具调用") {
			t.Fatalf("collapsed tree = %#v", lines)
		}
	})

	t.Run("with expanded activities", func(t *testing.T) {
		m := NewModelForLanguage("zh-CN")
		m.running = true
		m.thinkingText = "正在验证修改"
		m.recordActivity(agentui.ActivityEvent{Kind: agentui.ActivityFile, Target: "config.go"})
		m.recordActivity(agentui.ActivityEvent{Kind: agentui.ActivityTool, Target: "go test"})
		m.activitiesExpanded = true
		lines := strings.Split(ansi.Strip(m.renderRuntimeActivityTree(100)), "\n")
		if len(lines) != 5 || !strings.HasPrefix(lines[1], "├─  ") ||
			!strings.HasPrefix(lines[2], "├─ ") || !strings.Contains(lines[2], "config.go") ||
			!strings.HasPrefix(lines[3], "├─ ") || !strings.Contains(lines[3], "go test") ||
			!strings.HasPrefix(lines[4], "└─ ") || !strings.Contains(lines[4], "Ctrl+O 折叠") {
			t.Fatalf("expanded tree = %#v", lines)
		}
	})
}

func TestQuestionTemporarilyHidesThinkingAndThenRestoresIt(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.running = true
	m.thinkingText = "正在准备选项"
	m.transcript = append(m.transcript, chatTranscriptEntry{
		kind: transcriptQuestion, content: "继续吗？", pending: true,
		options: []string{"是", "否"}, selected: 0,
	})
	m.activeQuestion = len(m.transcript) - 1

	status := ansi.Strip(m.renderInlineStatus(100))
	if !strings.Contains(status, "继续吗？") || strings.Contains(status, "正在准备选项") {
		t.Fatalf("question status = %q", status)
	}
	m.transcript[m.activeQuestion].pending = false
	m.activeQuestion = historyCursorIdle
	if status := ansi.Strip(m.renderInlineStatus(100)); !strings.Contains(status, "正在准备选项") {
		t.Fatalf("restored status = %q", status)
	}
}

func TestThinkingTreeDoesNotOverflowTerminalWidth(t *testing.T) {
	texts := []struct {
		language string
		text     string
	}{
		{language: "zh-CN", text: strings.Repeat("正在检查很长的配置路径 ", 20)},
		{language: "en", text: strings.Repeat("Reviewing a very long configuration path ", 20)},
	}
	for _, test := range texts {
		for _, width := range []int{23, 59, 99} {
			m := NewModelForLanguage(test.language)
			m.running = true
			m.runningStatus.elapsed = time.Second
			m.thinkingText = test.text
			m.recordActivity(agentui.ActivityEvent{Kind: agentui.ActivityTool, Target: strings.Repeat("long-tool ", 20)})
			for index, line := range strings.Split(m.renderRuntimeActivityTree(width), "\n") {
				if cells := ansi.StringWidth(line); cells > width {
					t.Fatalf("language %s width %d line %d cells = %d", test.language, width, index, cells)
				}
			}
		}
	}
}
