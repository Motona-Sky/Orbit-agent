package cli

import (
	"fmt"
	"strings"

	"looporbit/internal/agentui"

	"github.com/charmbracelet/lipgloss"
)

const (
	activityTreeBranch = "├─ "
	activityTreeLast   = "└─ "
)

func (m *model) recordActivity(event agentui.ActivityEvent) {
	m.activities = append(m.activities, chatTranscriptEntry{
		kind:         transcriptActivity,
		content:      event.Target,
		activityKind: event.Kind,
	})
	m.activitiesExpanded = false
}

func (m model) activityCountText() string {
	format := m.messages.Chat.ToolCallsSummary
	if len(m.activities) == 1 {
		format = m.messages.Chat.ToolCallSummary
	}
	return fmt.Sprintf(format, len(m.activities))
}

func (m model) activityDescription(entry chatTranscriptEntry) string {
	label := m.messages.Chat.AgentExecuting
	if entry.activityKind == agentui.ActivityFile {
		label = m.messages.Chat.AgentReading
	}
	label = strings.TrimSuffix(strings.TrimSpace(label), "中")
	return strings.TrimSpace(label + " " + strings.TrimSpace(entry.content))
}

func (m model) renderActivityGroup(width int) string {
	if len(m.activities) == 0 || width <= 0 {
		return ""
	}
	if !m.activitiesExpanded {
		latest := m.activities[len(m.activities)-1]
		line := "› " + m.activityCountText() + " · " + m.messages.Chat.LatestActivityLabel +
			m.activityDescription(latest) + " (" + m.messages.Chat.ExpandToolCallsHint + ")"
		return lipgloss.NewStyle().Inline(true).MaxWidth(width).Render(m.mutedStyle().Render(line))
	}
	lines := make([]string, 0, len(m.activities)+1)
	for _, activity := range m.activities {
		lines = append(lines, m.renderActivity(activity, width)...)
	}
	lines = append(lines, m.mutedStyle().Render(m.messages.Chat.CollapseToolCallsHint))
	return strings.Join(clampTerminalLines(lines, width), "\n")
}

func (m model) renderRuntimeActivityTree(width int) string {
	if !m.running || width <= 0 {
		return ""
	}
	parent := ""
	if m.terminalTranscriptCursor == 0 || m.terminalTranscriptCursor > len(m.transcript) || m.transcript[m.terminalTranscriptCursor-1].role != "assistant" {
		parent = lipgloss.NewStyle().Inline(true).MaxWidth(width).Render(m.renderRunningStatus())
	}
	childWidth := maxInt(1, width-lipgloss.Width(activityTreeBranch))
	children := make([]string, 0, len(m.activities)+2)
	if thinking := m.renderThinkingText(childWidth); thinking != "" {
		children = append(children, thinking)
	}
	if activities := m.renderActivityGroup(childWidth); activities != "" {
		children = append(children, strings.Split(activities, "\n")...)
	}
	if len(children) == 0 {
		return parent
	}

	lines := make([]string, 0, len(children)+1)
	if parent != "" {
		lines = append(lines, parent)
	}
	for index, child := range children {
		prefix := activityTreeBranch
		if index == len(children)-1 {
			prefix = activityTreeLast
		}
		line := m.mutedStyle().Render(prefix) + child
		lines = append(lines, lipgloss.NewStyle().Inline(true).MaxWidth(width).Render(line))
	}
	return strings.Join(lines, "\n")
}

func (m model) handleActivityToggle() (bool, model) {
	if len(m.activities) == 0 {
		return false, m
	}
	m.activitiesExpanded = !m.activitiesExpanded
	return true, m
}

func (m *model) appendActivitySummary() {
	if len(m.activities) == 0 {
		m.activitiesExpanded = false
		return
	}
	m.transcript = append(m.transcript, chatTranscriptEntry{
		kind:    transcriptActivitySummary,
		content: "› " + m.activityCountText(),
	})
	m.activities = nil
	m.activitiesExpanded = false
}
