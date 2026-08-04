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
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.styleConfig.Palette.Muted))
}

func (m model) pureWhiteStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
}

func (m model) accentStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.styleConfig.Palette.Accent)).Bold(true)
}

func (m model) thinkingStyle() lipgloss.Style {
	return m.mutedStyle()
}
