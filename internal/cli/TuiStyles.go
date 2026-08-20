package cli

import (
	"orbit/internal/config"
	"orbit/internal/style"

	"github.com/charmbracelet/lipgloss"
)

func loadStyleConfigOrDefault() config.StyleConfig {
	styleConfig, err := config.LoadStyleConfig()
	if err != nil {
		return style.DefaultStyleConfig()
	}
	return styleConfig
}

func (m model) mutedStyle() lipgloss.Style {
	return m.styleMuted
}

func (m model) pureWhiteStyle() lipgloss.Style {
	return m.stylePureWhite
}

func (m model) accentStyle() lipgloss.Style {
	return m.styleAccent
}

func (m model) thinkingStyle() lipgloss.Style {
	return m.styleMuted
}
