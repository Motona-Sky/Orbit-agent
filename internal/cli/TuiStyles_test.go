package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"looporbit/internal/config"
	"looporbit/internal/style"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestChatModelLoadsStyleConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.StyleConfigFileName)
	t.Setenv(config.StyleConfigPathEnv, path)
	want, ok := style.ThemeConfigByKey("light")
	if !ok {
		t.Fatal("ThemeConfigByKey(light) not found")
	}
	if err := config.SaveStyleConfig(want); err != nil {
		t.Fatalf("SaveStyleConfig() error = %v", err)
	}

	m := NewModelForLanguage("zh-CN")
	if m.styleConfig != want {
		t.Fatalf("styleConfig = %#v, want %#v", m.styleConfig, want)
	}
}

func TestChatModelFallsBackFromInvalidStyleConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.StyleConfigFileName)
	t.Setenv(config.StyleConfigPathEnv, path)
	if err := os.WriteFile(path, []byte("palette: ["), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	m := NewModelForLanguage("zh-CN")
	if want := style.DefaultStyleConfig(); m.styleConfig != want {
		t.Fatalf("styleConfig = %#v, want fallback %#v", m.styleConfig, want)
	}
}

func TestChatModelFallsBackWhenStyleConfigIsMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.StyleConfigFileName)
	t.Setenv(config.StyleConfigPathEnv, path)

	m := NewModelForLanguage("zh-CN")
	if want := style.DefaultStyleConfig(); m.styleConfig != want {
		t.Fatalf("styleConfig = %#v, want fallback %#v", m.styleConfig, want)
	}
}

func TestChatModelDoesNotReloadStyleDuringView(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.StyleConfigFileName)
	t.Setenv(config.StyleConfigPathEnv, path)
	m := NewModelForLanguage("zh-CN")

	changed := style.DefaultStyleConfig()
	changed.Palette.Accent = "#224466"
	if err := config.SaveStyleConfig(changed); err != nil {
		t.Fatalf("SaveStyleConfig() error = %v", err)
	}
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })

	got := m.startupBanner()
	if !strings.Contains(got, "38;2;18;220;232") {
		t.Fatalf("startup banner did not keep construction-time style: %q", got)
	}
	if strings.Contains(got, "38;2;34;68;102") {
		t.Fatalf("startup banner reloaded style during view: %q", got)
	}
}

func TestChatModelRendersConfiguredPalette(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })
	path := filepath.Join(t.TempDir(), config.StyleConfigFileName)
	t.Setenv(config.StyleConfigPathEnv, path)
	styleConfig := style.DefaultStyleConfig()
	styleConfig.Palette.Accent = "#123456"
	styleConfig.Palette.Muted = "#664422"
	if err := config.SaveStyleConfig(styleConfig); err != nil {
		t.Fatalf("SaveStyleConfig() error = %v", err)
	}

	m := NewModelForLanguage("zh-CN")
	got := m.renderRunningStatus()
	if !strings.Contains(got, "38;2;18;52;86") {
		t.Fatalf("running status %q does not contain configured accent", got)
	}
	if !strings.Contains(got, "38;2;102;68;34") {
		t.Fatalf("running status %q does not contain configured muted color", got)
	}
}

func TestOrbitalViewportUsesConfiguredLayout(t *testing.T) {
	styleConfig := style.DefaultStyleConfig()
	styleConfig.Layout.FallbackWidth = 91
	styleConfig.Layout.FallbackHeight = 29
	styleConfig.Layout.MinWidth = 41
	styleConfig.Layout.MinHeight = 17

	if width, height := orbitalViewport(0, 0, styleConfig); width != 91 || height != 29 {
		t.Fatalf("fallback viewport = %dx%d, want 91x29", width, height)
	}
	if width, height := orbitalViewport(10, 10, styleConfig); width != 41 || height != 17 {
		t.Fatalf("minimum viewport = %dx%d, want 41x17", width, height)
	}
}
