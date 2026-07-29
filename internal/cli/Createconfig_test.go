package cli

import (
	"path/filepath"
	"testing"

	"looporbit/internal/config"
	"looporbit/internal/style"
)

func TestSaveThemePresetWritesStyleConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.StyleConfigFileName)
	t.Setenv(config.StyleConfigPathEnv, path)

	if err := saveThemePreset("light"); err != nil {
		t.Fatalf("saveThemePreset() error = %v", err)
	}
	got, err := config.LoadStyleConfig()
	if err != nil {
		t.Fatalf("LoadStyleConfig() error = %v", err)
	}
	want, ok := style.ThemeConfigByKey("light")
	if !ok {
		t.Fatal("ThemeConfigByKey(light) not found")
	}
	if got != want {
		t.Fatalf("saved style = %#v, want %#v", got, want)
	}
}

func TestSaveThemePresetRejectsUnknownTheme(t *testing.T) {
	if err := saveThemePreset("unknown"); err == nil {
		t.Fatal("saveThemePreset(unknown) error = nil")
	}
}
