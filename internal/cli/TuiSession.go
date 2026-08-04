package cli

import (
	"fmt"

	"orbit/internal/config"
	"orbit/internal/i18n"
	"orbit/internal/memorys"
	"orbit/internal/style"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

type SessionModel struct {
	sessions        []memorys.SessionSummary
	skipped         int
	cursor          int
	terminalWidth   int
	terminalHeight  int
	SelectedSession memorys.SessionSummary
	Confirmed       bool
	messages        i18n.SessionSetupMessages
	styleConfig     config.StyleConfig
}

func initialSessionModel(language string, sessions []memorys.SessionSummary, skipped int) SessionModel {
	return SessionModel{
		sessions:       append([]memorys.SessionSummary(nil), sessions...),
		skipped:        skipped,
		terminalWidth:  80,
		terminalHeight: 24,
		messages:       i18n.For(language).Messages.SessionSetup,
		styleConfig:    loadStyleConfigOrDefault(),
	}
}

func (m SessionModel) Init() tea.Cmd { return nil }

func (m SessionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width > 0 {
			m.terminalWidth = msg.Width
		}
		if msg.Height > 0 {
			m.terminalHeight = msg.Height
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "up", "k":
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case "enter":
			if len(m.sessions) > 0 {
				m.SelectedSession = m.sessions[m.cursor]
				m.Confirmed = true
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m SessionModel) View() string {
	width, height := m.terminalWidth, m.terminalHeight
	if width <= 0 {
		width = m.styleConfig.Layout.FallbackWidth
	}
	if height <= 0 {
		height = m.styleConfig.Layout.FallbackHeight
	}
	copy := style.OrbitalMenuCopy{
		Title:        m.messages.Title,
		Heading:      m.messages.Heading,
		Subtitle:     m.subtitle(width),
		MoveShortcut: m.messages.MoveShortcut,
		MoveAction:   m.messages.MoveAction,
		SelectKey:    m.messages.SelectKey,
		SelectAction: m.messages.SelectAction,
		QuitKey:      m.messages.QuitKey,
		QuitAction:   m.messages.QuitAction,
	}
	options, visibleCursor, sequenceOffset := m.visibleOptions(width, height)
	return style.RenderOrbitalMenuView(style.OrbitalMenuView{
		Copy:           copy,
		Options:        options,
		Cursor:         visibleCursor,
		SequenceOffset: sequenceOffset,
		StyleConfig:    m.styleConfig,
		ViewportWidth:  width,
		ViewportHeight: height,
	})
}

func (m SessionModel) subtitle(width int) string {
	skipped := ""
	if m.skipped > 0 {
		skipped = fmt.Sprintf(m.messages.SkippedFormat, m.skipped)
	}
	if len(m.sessions) == 0 {
		if skipped != "" {
			return m.messages.EmptyState + " · " + skipped
		}
		return m.messages.EmptyState
	}
	if skipped != "" {
		if width < 60 {
			return skipped
		}
		return m.messages.Subtitle + " · " + skipped
	}
	return m.messages.Subtitle
}

func (m SessionModel) visibleOptions(width, height int) ([]string, int, int) {
	if len(m.sessions) == 0 {
		return []string{m.messages.EmptyState}, -1, 0
	}
	capacity := style.OrbitalMenuOptionCapacity(m.messages.Title, width, height, m.styleConfig)
	limit := minInt(len(m.sessions), capacity)
	start := clampInt(m.cursor-limit/2, 0, maxInt(0, len(m.sessions)-limit))
	end := minInt(len(m.sessions), start+limit)
	labels := make([]string, 0, end-start)
	for _, session := range m.sessions[start:end] {
		labels = append(labels, sessionOptionLabel(session, maxInt(8, width-12)))
	}
	return labels, m.cursor - start, start
}

func sessionOptionLabel(session memorys.SessionSummary, width int) string {
	return ansi.Truncate(session.ID+"  "+session.FirstUserMessage, width, "…")
}

func (m *SessionModel) moveCursor(offset int) {
	if len(m.sessions) == 0 {
		return
	}
	m.cursor = (m.cursor + offset + len(m.sessions)) % len(m.sessions)
}
