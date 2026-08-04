package cli

import (
	"orbit/internal/config"
	"orbit/internal/i18n"
	"orbit/internal/style"

	tea "github.com/charmbracelet/bubbletea"
)

type ConfigThemeModel struct {
	theme          []themeOption
	themeCursor    int
	themeSelect    map[int]string
	terminalWidth  int
	terminalHeight int
	SelectedKey    int
	SelectedTheme  string
	Confirmed      bool
	messages       i18n.ThemeSetupMessages
	styleConfig    config.StyleConfig
}

type themeOption struct {
	Key   string
	Label string
}

func initialConfigTheme() ConfigThemeModel {
	return initialConfigThemeForLanguage(i18n.DefaultLanguage)
}

func initialConfigThemeForLanguage(languageCode string) ConfigThemeModel {
	messages := i18n.For(languageCode).Messages.ThemeSetup
	return ConfigThemeModel{
		theme: []themeOption{
			{Key: "dark", Label: messages.DarkOption},
			{Key: "light", Label: messages.LightOption},
			{Key: "high-contrast", Label: messages.HighContrastOption},
		},
		themeCursor: 0,
		themeSelect: map[int]string{0: "dark", 1: "light", 2: "high-contrast"},
		messages:    messages,
		styleConfig: loadStyleConfigOrDefault(),
	}
}
func (m ConfigThemeModel) Init() tea.Cmd {
	return nil
}
func (m ConfigThemeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
		m.terminalHeight = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case "enter", "space", " ":
			m.SelectedKey = m.themeCursor
			m.SelectedTheme = m.themeSelect[m.SelectedKey]
			m.Confirmed = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m ConfigThemeModel) View() string {
	width, height := m.viewport()
	return style.RenderOrbitalMenuView(style.OrbitalMenuView{
		Copy:           themeSetupCopy(m.messages),
		Options:        m.themeLabels(),
		Cursor:         m.themeCursor,
		StyleConfig:    m.styleConfig,
		ViewportWidth:  width,
		ViewportHeight: height,
	})
}

func themeSetupCopy(messages i18n.ThemeSetupMessages) style.OrbitalMenuCopy {
	return style.OrbitalMenuCopy{
		Title:        messages.Title,
		Heading:      messages.Subtitle,
		Subtitle:     messages.Description,
		MoveShortcut: messages.MoveShortcut,
		MoveAction:   messages.MoveAction,
		SelectKey:    messages.SelectKey,
		SelectAction: messages.SelectAction,
		QuitKey:      messages.QuitKey,
		QuitAction:   messages.QuitAction,
	}
}

func (m ConfigThemeModel) themeLabels() []string {
	labels := make([]string, 0, len(m.theme))
	for _, option := range m.theme {
		labels = append(labels, option.Label)
	}
	return labels
}

func (m *ConfigThemeModel) moveCursor(offset int) {
	if len(m.theme) == 0 {
		return
	}
	m.themeCursor = (m.themeCursor + offset + len(m.theme)) % len(m.theme)
}

func (m ConfigThemeModel) viewport() (int, int) {
	return orbitalViewport(m.terminalWidth, m.terminalHeight, m.styleConfig)
}
