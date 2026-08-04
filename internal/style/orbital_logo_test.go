package style

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestRenderOrbitalLogoUsesMinimalPromptWordmark(t *testing.T) {
	logo := ansi.Strip(RenderOrbitalLogo("Orbit"))
	if logo != "◉ ORBIT / AGENT" {
		t.Fatalf("logo = %q, want minimal prompt wordmark", logo)
	}
	if lipgloss.Height(logo) != 1 || strings.Contains(logo, "██") {
		t.Fatalf("logo is not a one-line minimal wordmark: %q", logo)
	}
}

func TestRenderOrbitalLogoAdaptsAcrossViewportWidths(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		want      string
		forbidden string
	}{
		{name: "full", width: 15, want: "◉ ORBIT / AGENT"},
		{name: "compact", width: 7, want: "◉ ORBIT", forbidden: "/ AGENT"},
		{name: "ultra narrow", width: 5, forbidden: "/ AGENT"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			logo := ansi.Strip(RenderOrbitalLogoForViewport("Orbit", test.width))
			if test.want != "" && logo != test.want {
				t.Fatalf("logo = %q, want %q", logo, test.want)
			}
			if test.forbidden != "" && strings.Contains(logo, test.forbidden) {
				t.Fatalf("logo contains %q: %q", test.forbidden, logo)
			}
			if width := lipgloss.Width(logo); width > test.width {
				t.Fatalf("logo width = %d, want <= %d: %q", width, test.width, logo)
			}
		})
	}
}
