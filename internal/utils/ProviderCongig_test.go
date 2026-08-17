package utils

import (
	"path/filepath"
	"testing"

	"orbit/internal/config"
)

func TestReloadProviderConfigUpdatesRuntimeFieldsAtomically(t *testing.T) {
	previousAPIKey, previousBaseURL := ApiKey, BaseUrl
	previousModel, previousProvider := Model, Provider
	previousRefreshToken := RefreshToken
	t.Cleanup(func() {
		ApiKey, BaseUrl = previousAPIKey, previousBaseURL
		Model, Provider = previousModel, previousProvider
		RefreshToken = previousRefreshToken
	})

	path := filepath.Join(t.TempDir(), config.AppConfigFileName)
	t.Setenv(config.AppConfigPathEnv, path)
	if err := config.SaveAppConfig(config.AppConfig{
		DefaultProvider: "replacement",
		Providers: map[string]config.ProviderConfig{
			"replacement": {
				ApiKey:       "new-key",
				RefreshToken: "new-refresh-token",
				BaseURL:      "https://new.example/v1",
				Model:        "new-model",
				Type:         "openai:completions",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	if err := ReloadProviderConfig(); err != nil {
		t.Fatal(err)
	}
	if ApiKey != "new-key" || RefreshToken != "new-refresh-token" || BaseUrl != "https://new.example/v1" || Model != "new-model" || Provider != "openai:completions" {
		t.Fatalf("runtime provider = %q %q %q %q %q", ApiKey, RefreshToken, BaseUrl, Model, Provider)
	}

	ApiKey, RefreshToken, BaseUrl, Model, Provider = "stable-key", "stable-refresh", "stable-url", "stable-model", "stable-type"
	t.Setenv(config.AppConfigPathEnv, t.TempDir())
	if err := ReloadProviderConfig(); err == nil {
		t.Fatal("ReloadProviderConfig() error = nil for directory path")
	}
	if ApiKey != "stable-key" || RefreshToken != "stable-refresh" || BaseUrl != "stable-url" || Model != "stable-model" || Provider != "stable-type" {
		t.Fatal("failed reload partially changed runtime provider")
	}
}
