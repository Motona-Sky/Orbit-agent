package style

import (
	"strings"
	"testing"

	"orbit/internal/config"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func configuredTestStyle() config.StyleConfig {
	styleConfig := DefaultStyleConfig()
	styleConfig.Palette.Foreground = "#113355"
	styleConfig.Palette.Muted = "#224466"
	styleConfig.Palette.Accent = "#335577"
	styleConfig.Palette.Divider = "#446688"
	styleConfig.Palette.Background = "#557799"
	styleConfig.Palette.PanelFill = "#6688aa"
	styleConfig.Palette.OptionFill = "#7799bb"
	return styleConfig
}

func TestConfiguredAgentCommandUsesPalette(t *testing.T) {
	enableTrueColor(t)
	styleConfig := configuredTestStyle()
	got := RenderAgentCommandInput(AgentCommandInputView{
		Copy: AgentCommandInputCopy{
			Title:       "Input",
			Prompt:      ">",
			Placeholder: "Ask",
		},
		StyleConfig: styleConfig,
		Width:       40,
		Focused:     false,
	})

	for _, color := range []string{
		colorSequence(styleConfig.Palette.Accent),
		colorSequence(styleConfig.Palette.Muted),
		colorSequence(styleConfig.Palette.Divider),
	} {
		if !strings.Contains(got, color) {
			t.Fatalf("configured command output %q does not contain %q", got, color)
		}
	}
	assertNoDefaultAccent(t, got)
}

func TestConfiguredOrbitalMenuUsesPalette(t *testing.T) {
	enableTrueColor(t)
	styleConfig := configuredTestStyle()
	got := RenderOrbitalMenuView(OrbitalMenuView{
		Copy: OrbitalMenuCopy{
			Title: "Orbit", Heading: "Theme", Subtitle: "Select",
			MoveShortcut: "↑↓", MoveAction: "move",
			SelectKey: "Enter", SelectAction: "select",
		},
		Options:        []string{"one", "two"},
		StyleConfig:    styleConfig,
		ViewportWidth:  80,
		ViewportHeight: 24,
	})

	for _, color := range []string{
		colorSequence(styleConfig.Palette.Accent),
		colorSequence(styleConfig.Palette.Foreground),
		colorSequence(styleConfig.Palette.Muted),
		colorSequence(styleConfig.Palette.Divider),
	} {
		if !strings.Contains(got, color) {
			t.Fatalf("configured menu output does not contain %q", color)
		}
	}
	for _, color := range []string{
		backgroundSequence(styleConfig.Palette.Background),
		backgroundSequence(styleConfig.Palette.PanelFill),
		backgroundSequence(styleConfig.Palette.OptionFill),
	} {
		if strings.Contains(got, color) {
			t.Fatalf("configured menu output unexpectedly contains background %q", color)
		}
	}
	assertNoDefaultAccent(t, got)
}

func TestConfiguredOrbitalFormUsesPalette(t *testing.T) {
	enableTrueColor(t)
	styleConfig := configuredTestStyle()
	got := RenderOrbitalFormView(OrbitalFormView{
		Copy:           OrbitalFormCopy{Title: "Orbit", Heading: "Provider", Hint: "Enter"},
		Lines:          []string{"Model", "gpt"},
		StyleConfig:    styleConfig,
		ViewportWidth:  80,
		ViewportHeight: 24,
	})

	for _, color := range []string{
		colorSequence(styleConfig.Palette.Accent),
		colorSequence(styleConfig.Palette.Foreground),
		colorSequence(styleConfig.Palette.Muted),
	} {
		if !strings.Contains(got, color) {
			t.Fatalf("configured form output does not contain %q", color)
		}
	}
	assertNoDefaultAccent(t, got)
}

func TestConfiguredOrbitalUsesLayout(t *testing.T) {
	enableTrueColor(t)
	styleConfig := configuredTestStyle()
	styleConfig.Layout.MaxPanelWidth = 40
	styleConfig.Layout.MinWidth = 20
	got := ansi.Strip(RenderOrbitalMenuView(OrbitalMenuView{
		Copy:           OrbitalMenuCopy{Title: "Orbit", Heading: "Theme"},
		Options:        []string{"one"},
		StyleConfig:    styleConfig,
		ViewportWidth:  100,
		ViewportHeight: 24,
	}))

	for _, line := range strings.Split(got, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "╱") {
			if width := lipgloss.Width(trimmed); width != 40 {
				t.Fatalf("panel width = %d, want configured width 40; line = %q", width, trimmed)
			}
			return
		}
	}
	t.Fatal("panel top border not found")
}

func colorSequence(hex string) string {
	rendered := lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Render("x")
	return sequencePayload(rendered)
}

func backgroundSequence(hex string) string {
	rendered := lipgloss.NewStyle().Background(lipgloss.Color(hex)).Render("x")
	return sequencePayload(rendered)
}

func sequencePayload(rendered string) string {
	prefix := rendered[:strings.Index(rendered, "x")]
	return strings.TrimSuffix(strings.TrimPrefix(prefix, "\x1b["), "m")
}

func enableTrueColor(t *testing.T) {
	t.Helper()
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
}

func assertNoDefaultAccent(t *testing.T, output string) {
	t.Helper()
	defaultAccent := colorSequence(DefaultStyleConfig().Palette.Accent)
	if strings.Contains(output, defaultAccent) {
		t.Fatalf("configured output still contains default dark accent %q", defaultAccent)
	}
}
