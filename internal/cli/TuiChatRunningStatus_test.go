package cli

import (
	"errors"
	"strings"
	"testing"
	"time"

	"looporbit/internal/agentui"

	"github.com/charmbracelet/x/ansi"
)

func TestWorkPromptLinesKeepsRunningStatusOutOfTaskPanel(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.running = true
	m.tasks = []agentui.TaskItem{{Title: "检查代码", Status: agentui.TaskRunning}}

	lines := m.workPromptLines()
	if len(lines) != 4 {
		t.Fatalf("workPromptLines() returned %d lines, want three stats and one task", len(lines))
	}
	content := ansi.Strip(strings.Join(lines, "\n"))
	if strings.Contains(content, "LOOPORBIT") || !strings.Contains(content, "检查代码") {
		t.Fatalf("task panel content = %q", content)
	}
}

func TestShortScreenShowsRuntimeParentBeforeInput(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.running = true
	m.width = 24
	m.height = 2
	m.screenInitialized = true

	view := ansi.Strip(m.renderShortScreenChat(terminalContentWidth(m.width), m.height))
	if !strings.Contains(view, "LOOPORBIT") || strings.Contains(view, m.messages.Chat.ContextUsage) {
		t.Fatalf("short-screen view = %q, want runtime parent instead of task placeholder", view)
	}
}

func TestSubmittingPromptStartsRunningStatus(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	updated, cmd := m.handleVisibleUserMessageSubmit("检查项目")
	defer updated.closeAgentUI()

	if !updated.running || updated.runningStatus.startedAt.IsZero() {
		t.Fatalf("submitted prompt did not start running status: %#v", updated.runningStatus)
	}
	if updated.runningStatus.elapsed != 0 || updated.runningStatus.frame != 0 {
		t.Fatalf("initial running status = %#v, want zero elapsed and frame", updated.runningStatus)
	}
	if cmd == nil {
		t.Fatal("submitted prompt command is nil, want agent and status commands")
	}
}

func TestRunningStatusRotatesChineseTextAtTextPosition(t *testing.T) {
	startedAt := time.Date(2026, time.July, 23, 10, 0, 0, 0, time.UTC)
	m := NewModelForLanguage("zh-CN")
	m.running = true
	m.startRunningStatus(startedAt)

	want := []string{"Thinking"}
	for frame, phrase := range want {
		m.runningStatus.frame = frame
		status := ansi.Strip(m.renderRunningStatus())
		if !strings.Contains(status, "◉ LOOPORBIT") || !strings.Contains(status, phrase) {
			t.Fatalf("frame %d status = %q, want phrase %q", frame, status, phrase)
		}
		if strings.Contains(status, "Thinking...") {
			t.Fatalf("frame %d still animates punctuation: %q", frame, status)
		}
	}
}

func TestRunningStatusTickUsesEightHundredMillisecondCadence(t *testing.T) {
	if runningStatusTickInterval != 800*time.Millisecond {
		t.Fatalf("runningStatusTickInterval = %s, want 800ms", runningStatusTickInterval)
	}
}

func TestRunningStatusUsesEnglishPhrase(t *testing.T) {
	m := NewModelForLanguage("en")
	m.running = true
	m.runningStatus.elapsed = time.Second
	m.runningStatus.frame = 1

	status := ansi.Strip(m.renderRunningStatus())
	if !strings.Contains(status, "Analyzing") || !strings.Contains(status, "1s") {
		t.Fatalf("running status = %q, want English phrase and elapsed seconds", status)
	}
}

func TestRunningStatusTickAdvancesFrameAndElapsedTime(t *testing.T) {
	startedAt := time.Date(2026, time.July, 23, 10, 0, 0, 0, time.UTC)
	m := NewModelForLanguage("zh-CN")
	m.running = true
	firstCmd := m.startRunningStatus(startedAt)
	if firstCmd == nil {
		t.Fatal("startRunningStatus() command is nil, want scheduled tick")
	}
	generation := m.runningStatus.generation

	updated, nextCmd := m.handleRunningStatusTick(runningStatusTickMsg{
		generation: generation,
		now:        startedAt.Add(3250 * time.Millisecond),
	})
	if updated.runningStatus.elapsed != 3250*time.Millisecond {
		t.Fatalf("elapsed = %s, want 3.25s", updated.runningStatus.elapsed)
	}
	if updated.runningStatus.frame != 1 {
		t.Fatalf("frame = %d, want 1", updated.runningStatus.frame)
	}
	if nextCmd == nil {
		t.Fatal("active tick did not schedule the next frame")
	}
}

func TestRunningStatusIgnoresTickFromPreviousGeneration(t *testing.T) {
	startedAt := time.Date(2026, time.July, 23, 10, 0, 0, 0, time.UTC)
	m := NewModelForLanguage("zh-CN")
	m.running = true
	m.startRunningStatus(startedAt)
	oldGeneration := m.runningStatus.generation
	m.stopRunningStatus()
	m.startRunningStatus(startedAt.Add(time.Minute))

	updated, nextCmd := m.handleRunningStatusTick(runningStatusTickMsg{
		generation: oldGeneration,
		now:        startedAt.Add(10 * time.Second),
	})
	if updated.runningStatus.elapsed != 0 || updated.runningStatus.frame != 0 {
		t.Fatalf("stale tick changed current state: %#v", updated.runningStatus)
	}
	if nextCmd != nil {
		t.Fatal("stale tick scheduled another tick")
	}
}

func TestRunningStatusStopsOnEveryTerminalPath(t *testing.T) {
	tests := []struct {
		name string
		stop func(model) model
	}{
		{
			name: "result",
			stop: func(m model) model {
				updated, _ := m.handleAgentUIEvent(agentui.ResultEvent{Text: "done"})
				return updated.(model)
			},
		},
		{
			name: "error",
			stop: func(m model) model {
				ui := m.agentUI
				updated, _ := m.Update(agentRunFinishedMsg{ui: ui, err: errors.New("boom")})
				return updated.(model)
			},
		},
		{
			name: "cancel",
			stop: func(m model) model {
				updated, _ := m.cancelRunningTurn()
				return updated
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m := NewModelForLanguage("zh-CN")
			m.running = true
			m.agentUI = agentui.New()
			m.startRunningStatus(time.Now())

			updated := test.stop(m)
			if updated.running || !updated.runningStatus.startedAt.IsZero() ||
				updated.runningStatus.elapsed != 0 || updated.runningStatus.frame != 0 {
				t.Fatalf("terminal path left running status active: %#v", updated.runningStatus)
			}
			if strings.Contains(ansi.Strip(strings.Join(updated.workPromptLines(), "\n")), "LOOPORBIT") {
				t.Fatal("terminal path still renders LOOPORBIT running status")
			}
		})
	}
}

func TestRunningStatusDoesNotOverflowNarrowOrShortScreens(t *testing.T) {
	for _, size := range []struct{ width, height int }{{60, 20}, {24, 4}} {
		m := NewModelForLanguage("zh-CN")
		m.width = size.width
		m.height = size.height
		m.screenInitialized = true
		m.running = true
		m.runningStatus.elapsed = 12 * time.Second

		for index, line := range strings.Split(m.View(), "\n") {
			if width := ansi.StringWidth(line); width >= size.width {
				t.Fatalf("%dx%d line %d width = %d, want < %d: %q", size.width, size.height, index, width, size.width, ansi.Strip(line))
			}
		}
	}
}
