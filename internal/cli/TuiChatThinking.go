package cli

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *model) setThinkingText(text string) {
	m.thinkingText = strings.Join(strings.Fields(text), " ")
}

func (m *model) clearThinkingText() {
	m.thinkingText = ""
}

func (m model) renderThinkingText(width int) string {
	if m.thinkingText == "" || width <= 0 {
		return ""
	}
	return lipgloss.NewStyle().Inline(true).MaxWidth(width).Render(
		m.thinkingStyle().Render(" " + m.thinkingText),
	)
}
