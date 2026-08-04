package utils

import (
	"path/filepath"
	"testing"

	"orbit/internal/config"
)

func TestReloadThinkLevelConfigUpdatesRuntimeValue(t *testing.T) {
	previous := ThinkLevel
	t.Cleanup(func() { ThinkLevel = previous })

	path := filepath.Join(t.TempDir(), config.AppConfigFileName)
	t.Setenv(config.AppConfigPathEnv, path)
	if err := config.SaveAppConfig(config.AppConfig{ThinkLevel: "max"}); err != nil {
		t.Fatal(err)
	}

	ThinkLevel = "low"
	if err := ReloadThinkLevelConfig(); err != nil {
		t.Fatal(err)
	}
	if ThinkLevel != "max" {
		t.Fatalf("ThinkLevel = %q, want max", ThinkLevel)
	}
}

func TestReloadThinkLevelConfigFailureKeepsRuntimeValue(t *testing.T) {
	previous := ThinkLevel
	t.Cleanup(func() { ThinkLevel = previous })

	ThinkLevel = "medium"
	t.Setenv(config.AppConfigPathEnv, t.TempDir())
	if err := ReloadThinkLevelConfig(); err == nil {
		t.Fatal("ReloadThinkLevelConfig() error = nil for directory path")
	}
	if ThinkLevel != "medium" {
		t.Fatalf("ThinkLevel = %q after failed reload, want medium", ThinkLevel)
	}
}

func TestMustReloadThinkLevelConfigPanicsOnFailureWithoutChangingRuntimeValue(t *testing.T) {
	previous := ThinkLevel
	t.Cleanup(func() { ThinkLevel = previous })

	ThinkLevel = "medium"
	t.Setenv(config.AppConfigPathEnv, t.TempDir())
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("mustReloadThinkLevelConfig() did not panic")
		}
		if ThinkLevel != "medium" {
			t.Fatalf("ThinkLevel = %q after panic, want medium", ThinkLevel)
		}
	}()

	mustReloadThinkLevelConfig()
}
