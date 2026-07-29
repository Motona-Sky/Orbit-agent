package cli

import (
	"looporbit/internal/config"
	"looporbit/internal/i18n"
	"looporbit/internal/style"

	tea "github.com/charmbracelet/bubbletea"
)

type ConfigEffortModel struct {
	efforts            []string
	effortCursor       int
	terminalWidth      int
	terminalHeight     int
	SelectedThinkLevel string
	Confirmed          bool
	messages           i18n.EffortSetupMessages
	styleConfig        config.StyleConfig
}

func initialConfigEffortForLanguage(languageCode, current string) ConfigEffortModel {
	efforts := config.ThinkLevelOptions()
	current = config.NormalizeThinkLevel(current)
	cursor := 0
	for index, effort := range efforts {
		if effort == current {
			cursor = index
			break
		}
	}

	return ConfigEffortModel{
		efforts:      efforts,
		effortCursor: cursor,
		messages:     i18n.For(languageCode).Messages.EffortSetup,
		styleConfig:  loadStyleConfigOrDefault(),
	}
}

func (m ConfigEffortModel) Init() tea.Cmd {
	return nil
}

func (m ConfigEffortModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
		m.terminalHeight = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case "enter", "space", " ":
			if m.effortCursor >= 0 && m.effortCursor < len(m.efforts) {
				m.SelectedThinkLevel = m.efforts[m.effortCursor]
				m.Confirmed = true
			}
		}
	}
	return m, nil
}

func (m ConfigEffortModel) View() string {
	width, height := orbitalViewport(m.terminalWidth, m.terminalHeight, m.styleConfig)
	return style.RenderOrbitalMenuView(style.OrbitalMenuView{
		Copy:           effortSetupCopy(m.messages),
		Options:        m.efforts,
		Cursor:         m.effortCursor,
		StyleConfig:    m.styleConfig,
		ViewportWidth:  width,
		ViewportHeight: height,
	})
}

func effortSetupCopy(messages i18n.EffortSetupMessages) style.OrbitalMenuCopy {
	return style.OrbitalMenuCopy{
		Title:        messages.Title,
		Heading:      messages.Heading,
		Subtitle:     messages.Subtitle,
		MoveShortcut: messages.MoveShortcut,
		MoveAction:   messages.MoveAction,
		SelectKey:    messages.SelectKey,
		SelectAction: messages.SelectAction,
		QuitKey:      messages.QuitKey,
		QuitAction:   messages.QuitAction,
	}
}

func (m *ConfigEffortModel) moveCursor(offset int) {
	if len(m.efforts) == 0 {
		return
	}
	m.effortCursor = (m.effortCursor + offset + len(m.efforts)) % len(m.efforts)
}
