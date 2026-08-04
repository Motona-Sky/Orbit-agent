package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"orbit/internal/i18n"
	"orbit/internal/mcp"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const mcpCommand = "/mcp"

type mcpChangedMsg struct{}
type mcpStartedMsg struct{ err error }
type mcpToggledMsg struct{ err error }

type mcpPageState struct {
	cursor   int
	statuses []mcp.ServiceStatus
	error    string
}

func startMcpManager(manager *mcp.Manager) tea.Cmd {
	if manager == nil {
		return nil
	}
	return func() tea.Msg { return mcpStartedMsg{err: manager.Start(contextBackground())} }
}

var contextBackground = func() context.Context { return context.Background() }

func waitForMcpChange(manager *mcp.Manager) tea.Cmd {
	if manager == nil {
		return nil
	}
	return func() tea.Msg {
		<-manager.Events()
		return mcpChangedMsg{}
	}
}

func (m model) startMcpPage() (model, tea.Cmd) {
	m.clearComposer()
	m.exitConfirm = false
	m.mcpPage = &mcpPageState{}
	m.refreshMcpStatus()
	m.setupScreenReady = false
	return m, tea.Sequence(tea.EnterAltScreen, markSetupScreenReady)
}

func (m *model) refreshMcpStatus() {
	if m.mcpManager == nil {
		m.mcpStatuses = nil
	} else {
		m.mcpStatuses = m.mcpManager.Snapshot()
	}
	if m.mcpPage != nil {
		m.mcpPage.statuses = append([]mcp.ServiceStatus(nil), m.mcpStatuses...)
		if len(m.mcpPage.statuses) == 0 {
			m.mcpPage.cursor = 0
		} else {
			m.mcpPage.cursor = clampInt(m.mcpPage.cursor, 0, len(m.mcpPage.statuses)-1)
		}
	}
}

func (m model) updateMcpPage(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok && size.Width > 0 && size.Height > 0 {
		m.width, m.height = size.Width, size.Height
	}
	if changed, ok := msg.(mcpChangedMsg); ok {
		_ = changed
		m.refreshMcpStatus()
		return m, waitForMcpChange(m.mcpManager)
	}
	if toggled, ok := msg.(mcpToggledMsg); ok {
		if toggled.err != nil {
			m.mcpPage.error = toggled.err.Error()
		} else {
			m.mcpPage.error = ""
		}
		m.refreshMcpStatus()
		return m, nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "esc", "ctrl+c", "q":
		return m.leaveMcpPage()
	case "up", "k":
		if len(m.mcpPage.statuses) > 0 {
			m.mcpPage.cursor = (m.mcpPage.cursor - 1 + len(m.mcpPage.statuses)) % len(m.mcpPage.statuses)
		}
	case "down", "j":
		if len(m.mcpPage.statuses) > 0 {
			m.mcpPage.cursor = (m.mcpPage.cursor + 1) % len(m.mcpPage.statuses)
		}
	case "enter", "space", " ":
		if len(m.mcpPage.statuses) == 0 || m.mcpManager == nil {
			return m, nil
		}
		status := m.mcpPage.statuses[m.mcpPage.cursor]
		return m, func() tea.Msg {
			return mcpToggledMsg{err: m.mcpManager.SetEnabled(status.Name, !status.Enabled)}
		}
	}
	return m, nil
}

func (m model) leaveMcpPage() (model, tea.Cmd) {
	m.mcpPage = nil
	m.setupScreenReady = false
	m.composer.Focus()
	return m.finishSetupScreenExit(nil)
}

func (m model) mcpErrorSummary() string {
	if m.mcpStartupError != "" {
		return fmt.Sprintf(m.messages.Chat.MCPStartupError, m.mcpStartupError)
	}
	failed := make([]string, 0)
	for _, status := range m.mcpStatuses {
		if status.State == mcp.StateError {
			failed = append(failed, status.Name)
		}
	}
	if len(failed) == 0 {
		return ""
	}
	return fmt.Sprintf(m.messages.Chat.MCPErrorSummary, len(failed), strings.Join(failed, ", "))
}

func (m model) renderMcpPage() string {
	if m.mcpPage == nil {
		return ""
	}
	width := maxInt(40, m.width-4)
	lines := []string{
		m.accentStyle().Render(m.messages.Chat.MCPPageTitle),
		m.mutedStyle().Render(m.messages.Chat.MCPPageHint),
		"",
	}
	if len(m.mcpPage.statuses) == 0 {
		lines = append(lines, m.mutedStyle().Render(m.messages.Chat.MCPEmpty))
	}
	for index, status := range m.mcpPage.statuses {
		marker := "  "
		if index == m.mcpPage.cursor {
			marker = "› "
		}
		toggle := "[ ]"
		if status.Enabled {
			toggle = "[✓]"
		}
		state := mcpStateLabel(m.messages.Chat, status.State)
		heading := fmt.Sprintf("%s%s %s  %s", marker, toggle, status.Name, state)
		if index == m.mcpPage.cursor {
			heading = m.accentStyle().Render(heading)
		} else {
			heading = m.pureWhiteStyle().Render(heading)
		}
		lines = append(lines, heading, "    "+m.mutedStyle().Render(status.Description))
		if status.Source != "" {
			lines = append(lines, "    "+m.mutedStyle().Render(filepath.Clean(status.Source)))
		}
		if status.Error != "" {
			for _, errorLine := range strings.Split(status.Error, "\n") {
				lines = append(lines, "    "+m.accentStyle().Render(errorLine))
			}
		}
		lines = append(lines, "")
	}
	if m.mcpPage.error != "" {
		lines = append(lines, m.accentStyle().Render(m.mcpPage.error), "")
	}
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Padding(1, 2).Render(strings.Join(lines, "\n"))
}

func mcpStateLabel(messages i18n.ChatMessages, state mcp.ServiceState) string {
	switch state {
	case mcp.StateStarting:
		return messages.MCPStarting
	case mcp.StateReady:
		return messages.MCPReady
	case mcp.StateError:
		return messages.MCPError
	case mcp.StateDisabled:
		return messages.MCPDisabled
	default:
		return messages.MCPStopped
	}
}
