package cli

import (
	"errors"
	"strings"
	"testing"
	"time"

	"orbit/internal/agentui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestAssistantTranscriptRendersMarkdown(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	lines := m.renderTranscriptEntry(80, chatTranscriptEntry{
		role:    "assistant",
		content: "# 标题\n\n这是 **重点** 和 `code`。\n\n- 第一项\n- 第二项",
	})
	rendered := ansi.Strip(strings.Join(lines, "\n"))

	for _, want := range []string{"标题", "重点", "code", "第一项", "第二项"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered Markdown missing %q: %q", want, rendered)
		}
	}
	for _, marker := range []string{"# 标题", "**重点**", "`code`"} {
		if strings.Contains(rendered, marker) {
			t.Fatalf("rendered Markdown kept marker %q: %q", marker, rendered)
		}
	}
}

func TestMarkdownCodeBlockHasNoBackgroundColor(t *testing.T) {
	lines := renderTerminalMarkdown("```go\nfmt.Println(\"ok\")\n```", 80)
	rendered := strings.Join(lines, "\n")

	if strings.Contains(rendered, "48;") {
		t.Fatalf("code block contains background color sequence: %q", rendered)
	}
	if plain := ansi.Strip(rendered); !strings.Contains(plain, "fmt.Println(\"ok\")") {
		t.Fatalf("code block lost content: %q", plain)
	}
}

func TestPendingAssistantTranscriptUsesLightweightRendering(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	lines := m.renderTranscriptEntry(80, chatTranscriptEntry{
		role:    "assistant",
		content: "# 尚未完成\n\n**流式内容**",
		pending: true,
	})
	rendered := ansi.Strip(strings.Join(lines, "\n"))

	if !strings.Contains(rendered, "# 尚未完成") || !strings.Contains(rendered, "**流式内容**") {
		t.Fatalf("pending assistant content was parsed or lost: %q", rendered)
	}
}

func TestPendingAssistantRenderingKeepsBoundedUTF8Tail(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	content := strings.Repeat("较早内容\n", 2000) + "最终可见内容"
	lines := m.renderPendingAssistantLines(40, content, 4)
	rendered := ansi.Strip(strings.Join(lines, "\n"))

	if len(lines) > 4 {
		t.Fatalf("pending lines = %d, want at most 4", len(lines))
	}
	if !strings.Contains(rendered, "最终可见内容") {
		t.Fatalf("pending tail lost final UTF-8 content: %q", rendered)
	}
}

func BenchmarkPendingAssistantRenderingLongStream(b *testing.B) {
	m := NewModelForLanguage("zh-CN")
	content := strings.Repeat("持续增长的流式回答内容。\n", 10000)
	b.ReportAllocs()
	for b.Loop() {
		_ = m.renderPendingAssistantLines(100, content, 20)
	}
}

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
	if strings.Contains(status, "ORBIT") {
		t.Fatalf("status redisplayed Orbit after DisplayResult: %q", status)
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
	m.width = 80
	m.height = 24
	m.screenInitialized = true
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
	if !m.composer.Focused() {
		t.Fatal("composer lost focus after final result")
	}
	if len(m.transcript) != 3 {
		t.Fatalf("transcript length = %d, want 3", len(m.transcript))
	}
	for index, want := range []string{"第一段", "第二段", "最终回答"} {
		if m.transcript[index].content != want {
			t.Fatalf("transcript[%d] = %q, want %q", index, m.transcript[index].content, want)
		}
	}
	view := ansi.Strip(m.View())
	for _, want := range []string{m.messages.Chat.ComposerTitle, m.messages.Chat.AgentCommandPlaceholder} {
		if !strings.Contains(view, want) {
			t.Fatalf("view after final result missing composer text %q: %q", want, view)
		}
	}
	if strings.Contains(view, "最终回答") {
		t.Fatalf("stable final result repeated in dynamic view: %q", view)
	}
	if len(strings.Split(view, "\n")) > m.height {
		t.Fatalf("view height exceeds terminal height %d: %q", m.height, view)
	}
	if !strings.Contains(view, m.messages.Chat.AgentCommandPrompt) || !strings.Contains(view, m.messages.Chat.AgentCommandPlaceholder) {
		t.Fatalf("view lost composer after final result: %q", view)
	}
}

func TestManagedTranscriptRendersPendingStreamOnce(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.transcript = append(m.transcript, chatTranscriptEntry{
		kind: transcriptMessage, role: "assistant", content: "唯一流式标记", pending: true,
	})
	m.streamingResultIndex = 0

	view := ansi.Strip(m.renderInlineStatus(80))
	if count := strings.Count(view, "唯一流式标记"); count != 1 {
		t.Fatalf("pending stream count = %d, want 1: %q", count, view)
	}
}

func TestCommitTerminalTranscriptAdvancesCursorAndPreservesExistingCommand(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.width = 80
	m.transcript = append(m.transcript, chatTranscriptEntry{
		kind: transcriptMessage, role: "assistant", content: "已完成回答",
	})

	updated, cmd := m.commitTerminalTranscript(nil)
	if cmd == nil {
		t.Fatal("commitTerminalTranscript did not return a print command")
	}
	if updated.terminalTranscriptCursor != 1 {
		t.Fatalf("cursor = %d, want 1", updated.terminalTranscriptCursor)
	}

	updated, cmd = updated.commitTerminalTranscript(nil)
	if cmd != nil {
		t.Fatal("second commit unexpectedly returned a command")
	}

	type existingMsg struct{}
	existing := func() tea.Msg { return existingMsg{} }
	updated.transcript = append(updated.transcript, chatTranscriptEntry{
		kind: transcriptMessage, role: "assistant", content: "下一条回答",
	})
	updated, cmd = updated.commitTerminalTranscript(existing)
	if cmd == nil {
		t.Fatal("commitTerminalTranscript dropped print/existing command sequence")
	}
	if updated.terminalTranscriptCursor != 2 {
		t.Fatalf("cursor = %d, want 2", updated.terminalTranscriptCursor)
	}
}

func TestCommitTerminalTranscriptAdvancesPastStableEmptyEntry(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.width = 80
	m.transcript = append(m.transcript, chatTranscriptEntry{
		kind: transcriptQuestion, content: "未回答问题",
	})

	type existingMsg struct{}
	existing := func() tea.Msg { return existingMsg{} }
	updated, cmd := m.commitTerminalTranscript(existing)
	if updated.terminalTranscriptCursor != 1 {
		t.Fatalf("cursor = %d, want 1", updated.terminalTranscriptCursor)
	}
	if cmd == nil {
		t.Fatal("empty rendered entry dropped existing command")
	}
	if _, ok := cmd().(existingMsg); !ok {
		t.Fatalf("existing command result = %T, want existingMsg", cmd())
	}
}

func TestSetupErrorCommitsOnlyWhenLeavingAltScreen(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.width = 80
	m.modelSetup = &ConfigModelModel{}

	updated, reportCmd := m.reportModelSetupError(errors.New("setup failed"))
	if reportCmd != nil || updated.terminalTranscriptCursor != 0 {
		t.Fatalf("report advanced cursor before alt-screen exit: cmd nil %v, cursor %d", reportCmd == nil, updated.terminalTranscriptCursor)
	}
	updated.modelSetup = nil
	updated, exitCmd := updated.finishSetupScreenExit(nil)
	if exitCmd == nil {
		t.Fatal("setup exit did not sequence alt-screen exit and transcript print")
	}
	if updated.terminalTranscriptCursor != 1 {
		t.Fatalf("cursor after setup exit = %d, want 1", updated.terminalTranscriptCursor)
	}
}

func TestShortScreenChatKeepsInputVisibleAfterLongStreamingAnswer(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.width = 80
	m.height = 6
	m.screenInitialized = true
	m.transcript = append(m.transcript, chatTranscriptEntry{
		kind:    transcriptMessage,
		role:    "assistant",
		content: strings.Repeat("很长的流式回答内容 ", 80),
		pending: true,
	})
	m.streamingResultIndex = 0

	view := ansi.Strip(m.View())
	lines := strings.Split(view, "\n")
	if len(lines) != m.height {
		t.Fatalf("short view height = %d, want %d: %q", len(lines), m.height, view)
	}
	lastLine := lines[len(lines)-1]
	if !strings.Contains(lastLine, m.messages.Chat.AgentCommandPrompt) || !strings.Contains(lastLine, m.messages.Chat.AgentCommandPlaceholder) {
		t.Fatalf("short view last line lost input: %q", lastLine)
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
		if len(lines) != 2 || !strings.Contains(lines[0], "ORBIT") ||
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

func TestRunningStatusWatchdogStopsUnresponsiveTurn(t *testing.T) {
	startedAt := time.Date(2026, time.July, 23, 10, 0, 0, 0, time.UTC)
	m := NewModelForLanguage("zh-CN")
	m.running = true
	ui := agentui.New()
	m.agentUI = ui
	m.startRunningStatus(startedAt)
	m.thinkingText = "仍在处理"
	m.recordActivity(agentui.ActivityEvent{Kind: agentui.ActivityTool, Target: "卡住的工具"})

	updated, _ := m.Update(runningStatusTickMsg{
		generation: m.runningStatus.generation,
		now:        startedAt.Add(runningTurnTimeout),
	})
	m = updated.(model)

	if m.running || m.agentUI != nil || !m.runningStatus.startedAt.IsZero() {
		t.Fatalf("watchdog state = running %v, ui nil %v, startedAt %v", m.running, m.agentUI == nil, m.runningStatus.startedAt)
	}
	if m.thinkingText != "" || len(m.activities) != 0 {
		t.Fatalf("watchdog left runtime state = thinking %q, activities %d", m.thinkingText, len(m.activities))
	}
	if _, err := ui.Next(); !errors.Is(err, agentui.ErrClosed) {
		t.Fatalf("AgentUI.Next() error = %v, want ErrClosed", err)
	}
	if len(m.transcript) == 0 || m.transcript[len(m.transcript)-1].content != m.messages.Chat.TurnTimedOut {
		t.Fatalf("transcript = %#v, want timeout message", m.transcript)
	}
	for _, entry := range m.transcript {
		if entry.content == m.messages.Chat.TurnCanceled {
			t.Fatalf("watchdog used generic cancellation message: %#v", m.transcript)
		}
	}
}

func TestRunningStatusWatchdogKeepsTickingBeforeTimeout(t *testing.T) {
	startedAt := time.Date(2026, time.July, 23, 10, 0, 0, 0, time.UTC)
	m := NewModelForLanguage("zh-CN")
	m.running = true
	m.agentUI = agentui.New()
	defer m.agentUI.Close()
	m.startRunningStatus(startedAt)

	m, cmd := m.handleRunningStatusTick(runningStatusTickMsg{
		generation: m.runningStatus.generation,
		now:        startedAt.Add(runningTurnTimeout - time.Second),
	})

	if !m.running || m.agentUI == nil || cmd == nil {
		t.Fatalf("pre-timeout state = running %v, ui nil %v, cmd nil %v", m.running, m.agentUI == nil, cmd == nil)
	}
	if m.runningStatus.elapsed != runningTurnTimeout-time.Second {
		t.Fatalf("elapsed = %v, want %v", m.runningStatus.elapsed, runningTurnTimeout-time.Second)
	}
}

func TestRunningStatusWatchdogRunsPendingInputAndIgnoresStaleMessages(t *testing.T) {
	startedAt := time.Date(2026, time.July, 23, 10, 0, 0, 0, time.UTC)
	m := NewModelForLanguage("zh-CN")
	m.running = true
	staleUI := agentui.New()
	m.agentUI = staleUI
	m.pendingInput = "排队任务"
	m.startRunningStatus(startedAt)

	updated, _ := m.Update(runningStatusTickMsg{
		generation: m.runningStatus.generation,
		now:        startedAt.Add(runningTurnTimeout),
	})
	m = updated.(model)
	currentUI := m.agentUI
	defer currentUI.Close()

	if !m.running || currentUI == nil || currentUI == staleUI || m.pendingInput != "" {
		t.Fatalf("pending input state = running %v, current ui valid %v, pending %q", m.running, currentUI != nil && currentUI != staleUI, m.pendingInput)
	}
	updated, _ = m.Update(agentRunFinishedMsg{ui: staleUI, err: errors.New("stale failure")})
	m = updated.(model)
	updated, _ = m.Update(agentUIClosedMsg{ui: staleUI, err: agentui.ErrClosed})
	m = updated.(model)
	if !m.running || m.agentUI != currentUI {
		t.Fatal("stale messages changed the queued task")
	}
	for _, entry := range m.transcript {
		if strings.Contains(entry.content, "stale failure") {
			t.Fatalf("stale failure entered transcript: %#v", m.transcript)
		}
	}
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
