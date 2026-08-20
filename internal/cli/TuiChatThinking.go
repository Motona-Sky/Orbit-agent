package cli

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (m *model) setThinkingText(text string) {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for index := range lines {
		lines[index] = strings.TrimSpace(lines[index])
	}
	m.thinkingText = strings.Join(lines, "\n")
}

func (m *model) clearThinkingText() {
	m.thinkingText = ""
}

func (m model) renderThinkingText(width int) string {
	if m.thinkingText == "" || width <= 0 {
		return ""
	}
	lines := strings.Split(ansi.Hardwrap(" "+m.thinkingText, width, true), "\n")
	for index := range lines {
		lines[index] = m.thinkingStyle().Render(lines[index])
	}
	return strings.Join(lines, "\n")
}
