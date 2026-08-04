package cli

import (
	"orbit/internal/config"
	"orbit/internal/i18n"
	"orbit/internal/style"

	tea "github.com/charmbracelet/bubbletea"
)

type ConfigLanModel struct {
	language             []string
	languageCodes        []string
	terminalWidth        int
	terminalHeight       int
	LanCursor            int
	Lanselect            map[int]string
	SelectedKey          int
	SelectedLanguage     string
	SelectedLanguageCode string
	Confirmed            bool
	styleConfig          config.StyleConfig
}

func initialConfigLan() ConfigLanModel {
	options := i18n.Options()
	language := make([]string, 0, len(options))
	languageCodes := make([]string, 0, len(options))
	lanSelect := make(map[int]string, len(options))
	for i, option := range options {
		// 仍按原有 int key 保存选择结果，只把展示文案集中到 i18n。
		language = append(language, option.Name)
		languageCodes = append(languageCodes, option.Code)
		lanSelect[i] = option.Name
	}
	return ConfigLanModel{
		language:      language,
		languageCodes: languageCodes,
		LanCursor:     0,
		Lanselect:     lanSelect,
		styleConfig:   loadStyleConfigOrDefault(),
	}
}

func (m ConfigLanModel) Init() tea.Cmd {
	return nil
}

func (m ConfigLanModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.SelectedKey = m.LanCursor
			m.SelectedLanguage = m.Lanselect[m.SelectedKey]
			m.SelectedLanguageCode = m.currentLanguageCode()
			m.Confirmed = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m ConfigLanModel) View() string {
	// 语言选择器跟随当前光标动态切换自身文案，便于用户预览目标语言。
	messages := i18n.For(m.currentLanguageCode()).Messages.LanguageSetup
	width, height := m.viewport()
	return style.RenderOrbitalMenuView(style.OrbitalMenuView{
		Copy:           languageSetupCopy(messages),
		Options:        m.language,
		Cursor:         m.LanCursor,
		StyleConfig:    m.styleConfig,
		ViewportWidth:  width,
		ViewportHeight: height,
	})
}

func (m ConfigLanModel) renderLogo(title string) string {
	return style.RenderOrbitalLogo(title)
}

func (m ConfigLanModel) renderPanel(messages i18n.LanguageSetupMessages, viewportWidth int) string {
	return style.RenderOrbitalMenuPanel(languageSetupCopy(messages), m.language, m.LanCursor, viewportWidth)
}

func languageSetupCopy(messages i18n.LanguageSetupMessages) style.OrbitalMenuCopy {
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

func (m ConfigLanModel) currentLanguageCode() string {
	if m.LanCursor >= 0 && m.LanCursor < len(m.languageCodes) {
		return m.languageCodes[m.LanCursor]
	}
	return i18n.DefaultLanguage
}

func (m *ConfigLanModel) moveCursor(offset int) {
	if len(m.language) == 0 {
		return
	}
	m.LanCursor = (m.LanCursor + offset + len(m.language)) % len(m.language)
}

func (m ConfigLanModel) viewport() (int, int) {
	return orbitalViewport(m.terminalWidth, m.terminalHeight, m.styleConfig)
}
