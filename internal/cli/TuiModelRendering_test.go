package cli

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestConfigModelMovesCursorAndIgnoresInvalidWindowSize(t *testing.T) {
	m := initialConfigModelForLanguage("zh-CN", "")
	m.setModels([]string{"claude", "gpt-5"})
	m.terminalWidth, m.terminalHeight = 80, 24

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(ConfigModelModel)
	if m.cursor != 2 {
		t.Fatalf("wrapped cursor = %d, want 2", m.cursor)
	}

	updated, _ = m.Update(tea.WindowSizeMsg{})
	m = updated.(ConfigModelModel)
	if m.terminalWidth != 80 || m.terminalHeight != 24 {
		t.Fatalf("invalid size changed dimensions to %dx%d", m.terminalWidth, m.terminalHeight)
	}
}

func TestConfigModelRendersWithinNarrowViewportInBothLanguages(t *testing.T) {
	for _, languageCode := range []string{"zh-CN", "en"} {
		t.Run(languageCode, func(t *testing.T) {
			m := initialConfigModelForLanguage(languageCode, "a-very-long-model-name-for-narrow-terminals")
			m.setModels([]string{"a-very-long-model-name-for-narrow-terminals"})
			m.terminalWidth, m.terminalHeight = 48, 18

			view := m.View()
			for _, line := range strings.Split(view, "\n") {
				if width := lipgloss.Width(line); width > 48 {
					t.Fatalf("line width = %d, want <= 48: %q", width, line)
				}
			}
			if !strings.Contains(view, ">") {
				t.Fatalf("selected item marker missing from view: %q", view)
			}
		})
	}
}
