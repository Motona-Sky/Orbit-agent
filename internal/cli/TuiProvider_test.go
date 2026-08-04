package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"orbit/internal/config"
	"orbit/internal/style"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestConfiguredProviderLoadsItsOwnFields(t *testing.T) {
	appConfig := config.AppConfig{Providers: map[string]config.ProviderConfig{
		"openai": {
			ApiKey:  "openai-key",
			BaseURL: "https://openai.example/v1",
			Model:   "gpt-test",
			Type:    "openai:completions",
		},
	}}
	m := initialConfigProviderForLanguageWithConfig("en", appConfig)
	m.providerCursor = 0
	m.openProviderNameStep()
	m.openBaseURLStep()

	if got := m.baseURLInput.Value(); got != "https://openai.example/v1" {
		t.Fatalf("base URL = %q", got)
	}
	if got := m.apiKeyInput.Value(); got != "openai-key" {
		t.Fatalf("API key = %q", got)
	}
	if got := m.modelInput.Value(); got != "gpt-test" {
		t.Fatalf("model = %q", got)
	}
}

func TestOpenAICompatibleTypeMatchesAdapter(t *testing.T) {
	if got := defaultProviderType("openai"); got != "openai:completions" {
		t.Fatalf("OpenAI provider type = %q", got)
	}
}

func TestProviderSelectionUsesConfiguredStyle(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.StyleConfigFileName)
	t.Setenv(config.StyleConfigPathEnv, path)
	styleConfig := style.DefaultStyleConfig()
	styleConfig.Palette.Accent = "#224466"
	if err := config.SaveStyleConfig(styleConfig); err != nil {
		t.Fatalf("SaveStyleConfig() error = %v", err)
	}
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })

	m := initialConfigProviderForLanguageWithConfig("en", config.AppConfig{})
	m.terminalWidth, m.terminalHeight = 80, 24
	got := m.View()
	if !strings.Contains(got, "38;2;34;68;102") {
		t.Fatalf("provider selection does not contain configured accent: %q", got)
	}
	if strings.Contains(got, "38;2;18;220;232") {
		t.Fatalf("provider selection still contains default dark accent: %q", got)
	}
}
