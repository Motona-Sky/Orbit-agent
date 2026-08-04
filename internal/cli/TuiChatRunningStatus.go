package cli

import (
	"fmt"
	"strings"
	"time"

	"orbit/internal/agentui"

	tea "github.com/charmbracelet/bubbletea"
)

const runningStatusTickInterval = 800 * time.Millisecond

type runningStatusState struct {
	generation uint64
	startedAt  time.Time
	elapsed    time.Duration
	frame      int
}

type runningStatusTickMsg struct {
	generation uint64
	now        time.Time
}

func scheduleRunningStatusTick(generation uint64) tea.Cmd {
	return tea.Tick(runningStatusTickInterval, func(now time.Time) tea.Msg {
		return runningStatusTickMsg{generation: generation, now: now}
	})
}

func (m *model) startRunningStatus(now time.Time) tea.Cmd {
	m.runningStatus.generation++
	m.runningStatus.startedAt = now
	m.runningStatus.elapsed = 0
	m.runningStatus.frame = 0
	return scheduleRunningStatusTick(m.runningStatus.generation)
}

func (m *model) stopRunningStatus() {
	m.runningStatus.startedAt = time.Time{}
	m.runningStatus.elapsed = 0
	m.runningStatus.frame = 0
}

func (m model) handleRunningStatusTick(msg runningStatusTickMsg) (model, tea.Cmd) {
	if !m.running || m.runningStatus.startedAt.IsZero() ||
		msg.generation != m.runningStatus.generation {
		return m, nil
	}
	elapsed := msg.now.Sub(m.runningStatus.startedAt)
	if elapsed < 0 {
		elapsed = 0
	}
	m.runningStatus.elapsed = elapsed
	phraseCount := len(m.messages.Chat.AgentThinkingPhrases)
	if phraseCount > 0 {
		m.runningStatus.frame = (m.runningStatus.frame + 1) % phraseCount
	}
	return m, scheduleRunningStatusTick(m.runningStatus.generation)
}

func (m model) runningStatusText() string {
	if len(m.activities) > 0 {
		if m.activities[len(m.activities)-1].activityKind == agentui.ActivityFile {
			return m.messages.Chat.AgentReading
		}
		return m.messages.Chat.AgentExecuting
	}
	phrases := m.messages.Chat.AgentThinkingPhrases
	if len(phrases) == 0 {
		return ""
	}
	return phrases[m.runningStatus.frame%len(phrases)]
}

func (m model) renderRunningStatus() string {
	seconds := maxInt(0, int(m.runningStatus.elapsed/time.Second))
	brand := "◉ " + strings.ToUpper(strings.TrimSpace(m.messages.Chat.BrandName))
	return m.accentStyle().Render(brand) + m.mutedStyle().Render(fmt.Sprintf(" · %s (%ds)", m.runningStatusText(), seconds))
}
