package cli

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"orbit/internal/agentui"
	"orbit/internal/event"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type agentUIClosedMsg struct {
	ui  *agentui.AgentUI
	err error
}

type agentUIEventMsg struct {
	ui    *agentui.AgentUI
	event agentui.Event
}

type agentRunFinishedMsg struct {
	ui  *agentui.AgentUI
	err error
}

func waitForAgentUI(ui *agentui.AgentUI) tea.Cmd {
	return func() tea.Msg {
		event, err := ui.Next()
		if err != nil {
			return agentUIClosedMsg{ui: ui, err: err}
		}
		return agentUIEventMsg{ui: ui, event: event}
	}
}

func runAgent(ui *agentui.AgentUI, input event.TuiEvent, errorLabel string) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	ui.SetCancel(cancel)
	return func() tea.Msg {
		defer cancel()
		return finishAgentRun(ui, errorLabel, event.MessagesEvent(ctx, input, ui))
	}
}

func finishAgentRun(ui *agentui.AgentUI, errorLabel string, runErr error) agentRunFinishedMsg {
	finished := agentRunFinishedMsg{ui: ui}
	if runErr == nil {
		return finished
	}
	if displayErr := ui.DisplayFinalResult(errorLabel + runErr.Error()); displayErr != nil {
		finished.err = errors.Join(runErr, displayErr)
	}
	return finished
}

func (m model) handleAgentUIEvent(msg agentui.Event) (tea.Model, tea.Cmd) {
	m.markRunningActivity(time.Now())
	switch msg := msg.(type) {
	case agentui.ResultEvent:
		m.clearPendingUserTranscript()
		m.appendActivitySummary()
		m.streamingResultIndex = historyCursorIdle
		m.transcript = append(m.transcript, chatTranscriptEntry{
			kind:    transcriptMessage,
			role:    "assistant",
			content: msg.Text,
		})
		m, transcriptCmd := m.commitTerminalTranscript(nil)
		return m, sequenceTeaCommands(transcriptCmd, m.waitForNextAgentUIEvent())

	case agentui.StreamResultEvent:
		m.clearPendingUserTranscript()
		var transcriptCmd tea.Cmd
		if m.streamingResultIndex < 0 || m.streamingResultIndex >= len(m.transcript) {
			m.appendActivitySummary()
			m, transcriptCmd = m.commitTerminalTranscript(nil)
			m.transcript = append(m.transcript, chatTranscriptEntry{
				kind:    transcriptMessage,
				role:    "assistant",
				content: msg.Text,
				pending: true,
			})
			m.streamingResultIndex = len(m.transcript) - 1
		} else {
			m.transcript[m.streamingResultIndex].content = msg.Text
		}
		return m, sequenceTeaCommands(transcriptCmd, m.waitForNextAgentUIEvent())

	case agentui.StreamResultDoneEvent:
		if m.streamingResultIndex >= 0 && m.streamingResultIndex < len(m.transcript) {
			entry := &m.transcript[m.streamingResultIndex]
			entry.content = msg.Text
			entry.pending = false
			m.streamingResultIndex = historyCursorIdle
			m, transcriptCmd := m.commitTerminalTranscript(nil)
			return m, sequenceTeaCommands(transcriptCmd, m.waitForNextAgentUIEvent())
		}
		return m, m.waitForNextAgentUIEvent()

	case agentui.FinalResultEvent:
		failed := strings.HasPrefix(strings.TrimSpace(msg.Text), m.messages.Chat.AgentErrorLabel)
		m.clearPendingUserTranscript()
		m.appendActivitySummary()
		if m.streamingResultIndex >= 0 && m.streamingResultIndex < len(m.transcript) {
			entry := &m.transcript[m.streamingResultIndex]
			entry.content = msg.Text
			entry.pending = false
			m.streamingResultIndex = historyCursorIdle
		} else if len(m.transcript) == 0 || m.transcript[len(m.transcript)-1].role != "assistant" || m.transcript[len(m.transcript)-1].content != msg.Text {
			m.transcript = append(m.transcript, chatTranscriptEntry{
				kind:    transcriptMessage,
				role:    "assistant",
				content: msg.Text,
			})
		}
		m.appendTaskSummary(failed)
		m.running = false
		m.stopRunningStatus()
		m.closeAgentUI()
		m, transcriptCmd := m.commitTerminalTranscript(nil)
		return m.runPendingInput(transcriptCmd)

	case agentui.ThinkingEvent:
		m.setThinkingText(msg.Text)
		return m, m.waitForNextAgentUIEvent()
	case agentui.ActivityEvent:
		m.recordActivity(msg)
		return m, m.waitForNextAgentUIEvent()
	case agentui.TaskPlanEvent:
		m.tasks = append([]agentui.TaskItem(nil), msg.Tasks...)
		return m, m.waitForNextAgentUIEvent()
	case agentui.UsageEvent:
		m.usageStats = msg.Stats
		return m, m.waitForNextAgentUIEvent()
	case *agentui.QuestionEvent:
		m.transcript = append(m.transcript, chatTranscriptEntry{
			kind:     transcriptQuestion,
			content:  msg.Question,
			pending:  true,
			options:  append([]string(nil), msg.Options...),
			selected: 0,
			question: msg,
		})
		m.activeQuestion = len(m.transcript) - 1
		return m, m.waitForNextAgentUIEvent()
	}
	return m, nil
}

func (m model) waitForNextAgentUIEvent() tea.Cmd {
	if m.agentUI == nil {
		return nil
	}
	return waitForAgentUI(m.agentUI)
}

func (m *model) closeAgentUI() {
	if m.activeQuestion >= 0 && m.activeQuestion < len(m.transcript) {
		m.transcript[m.activeQuestion].pending = false
		m.activeQuestion = historyCursorIdle
	}
	if m.streamingResultIndex >= 0 && m.streamingResultIndex < len(m.transcript) {
		m.transcript[m.streamingResultIndex].pending = false
		m.streamingResultIndex = historyCursorIdle
	}
	m.clearThinkingText()
	if m.agentUI == nil {
		return
	}
	m.agentUI.Close()
	m.agentUI = nil
}

func (m *model) clearPendingUserTranscript() {
	for index := len(m.transcript) - 1; index >= 0; index-- {
		entry := &m.transcript[index]
		if entry.role == "user" && entry.pending {
			entry.pending = false
			return
		}
	}
}

func (m model) handleQuestionKey(msg tea.KeyMsg) (bool, model, tea.Cmd) {
	if m.activeQuestion < 0 || m.activeQuestion >= len(m.transcript) {
		return false, m, nil
	}

	entry := &m.transcript[m.activeQuestion]
	if entry.kind != transcriptQuestion || !entry.pending || len(entry.options) == 0 {
		return false, m, nil
	}

	switch msg.String() {
	case "up":
		if entry.selected > 0 {
			entry.selected--
		}
		return true, m, nil
	case "down":
		if entry.selected < len(entry.options)-1 {
			entry.selected++
		}
		return true, m, nil
	case "enter":
		if entry.question == nil || !entry.question.Answer(entry.selected) {
			return true, m, nil
		}
		entry.answer = entry.options[entry.selected]
		entry.pending = false
		m.activeQuestion = historyCursorIdle
		m.composer.Focus()
		m, cmd := m.commitTerminalTranscript(nil)
		return true, m, cmd
	}

	optionNumber, err := strconv.Atoi(msg.String())
	if err != nil || optionNumber < 1 || optionNumber > len(entry.options) {
		return false, m, nil
	}
	entry.selected = optionNumber - 1
	return true, m, nil
}

// renderActivity 将工具调用或文件查看渲染为单行紧凑状态，并限制终端宽度。
func (m model) renderActivity(entry chatTranscriptEntry, width int) []string {
	target := strings.TrimSpace(entry.content)
	if target == "" {
		return nil
	}

	prefix := "⚙ " + m.messages.Chat.ToolActivityLabel
	if entry.activityKind == agentui.ActivityFile {
		prefix = "◉ " + m.messages.Chat.FileActivityLabel
	}
	line := m.mutedStyle().Render(prefix + target)
	line = lipgloss.NewStyle().Inline(true).MaxWidth(maxInt(1, width)).Render(line)
	return []string{line}
}

// renderQuestion 渲染可交互的用户选择卡，确认后保留最终选择。
func (m model) renderQuestion(entry chatTranscriptEntry, width int) []string {
	question := strings.TrimSpace(entry.content)
	if question == "" {
		return nil
	}

	lines := []string{m.accentStyle().Render(question), ""}
	for index, option := range entry.options {
		marker := "  "
		if index == entry.selected {
			marker = "› "
		}
		lines = append(lines, marker+strconv.Itoa(index+1)+". "+option)
	}
	if entry.pending {
		lines = append(lines, "", m.mutedStyle().Render(m.messages.Chat.QuestionHelp))
	} else if entry.answer != "" {
		lines = append(lines, "", m.accentStyle().Render(m.messages.Chat.QuestionSelectedLabel+entry.answer))
	}

	cardWidth := maxInt(8, width-2)
	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(m.styleConfig.Palette.Divider)).
		Foreground(lipgloss.Color(m.styleConfig.Palette.Foreground)).
		Padding(0, 1).
		MaxWidth(cardWidth).
		Render(strings.Join(lines, "\n"))
	return append(
		[]string{m.accentStyle().Render(m.messages.Chat.AssistantLabel)},
		strings.Split(card, "\n")...,
	)
}

// renderTaskItem 将单个任务状态转换为动态任务框中的状态行。
func (m model) renderTaskItem(task agentui.TaskItem) string {
	prefix := "[ ] "
	switch task.Status {
	case agentui.TaskRunning:
		prefix = "[>] "
	case agentui.TaskDone:
		prefix = "[✓] "
	case agentui.TaskFailed:
		prefix = "[!] "
	}

	line := prefix + task.Title
	if task.Status == agentui.TaskRunning {
		return m.accentStyle().Render(line)
	}
	return m.mutedStyle().Render(line)
}
