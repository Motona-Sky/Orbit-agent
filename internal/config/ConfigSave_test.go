package config

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestUpdateProviderOAuthTokens(t *testing.T) {
	path := filepath.Join(t.TempDir(), AppConfigFileName)
	t.Setenv(AppConfigPathEnv, path)

	originalOther := ProviderConfig{
		Auth:         "apikey",
		ApiKey:       "other-key",
		BaseURL:      "https://other.example/v1",
		Model:        "other-model",
		DefaultModel: "other-model",
	}
	if err := SaveAppConfig(AppConfig{
		DefaultProvider: "other",
		Providers: map[string]ProviderConfig{
			"codex": {
				Auth:         "codex",
				AccessToken:  "old-access",
				IDToken:      "old-id",
				RefreshToken: "old-refresh",
				AccountID:    "old-account",
			},
			"other": originalOther,
		},
	}); err != nil {
		t.Fatal(err)
	}

	updated, err := UpdateProviderOAuthTokens("codex", "new-access", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "codex" {
		t.Fatalf("updated provider name = %q, want codex", updated.Name)
	}
	if updated.AccessToken != "new-access" {
		t.Fatalf("updated access token = %q, want new-access", updated.AccessToken)
	}
	if updated.IDToken != "old-id" || updated.RefreshToken != "old-refresh" || updated.AccountID != "old-account" {
		t.Fatalf("empty optional values replaced existing tokens: %#v", updated)
	}

	appConfig, err := LoadAppConfig()
	if err != nil {
		t.Fatal(err)
	}
	if appConfig.DefaultProvider != "other" {
		t.Fatalf("default provider = %q, want other", appConfig.DefaultProvider)
	}
	if !reflect.DeepEqual(appConfig.Providers["other"], originalOther) {
		t.Fatalf("other provider changed: %#v", appConfig.Providers["other"])
	}
	if got := appConfig.Providers["codex"]; got.AccessToken != "new-access" || got.IDToken != "old-id" || got.RefreshToken != "old-refresh" || got.AccountID != "old-account" {
		t.Fatalf("persisted provider = %#v", got)
	}
}

func TestUpdateProviderOAuthTokensRejectsEmptyAccessToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), AppConfigFileName)
	t.Setenv(AppConfigPathEnv, path)
	if err := SaveAppConfig(AppConfig{
		DefaultProvider: "codex",
		Providers: map[string]ProviderConfig{
			"codex": {AccessToken: "old-access"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := UpdateProviderOAuthTokens("codex", " ", "new-id", "new-refresh", "new-account"); err == nil {
		t.Fatal("UpdateProviderOAuthTokens() error = nil for empty access token")
	}
	appConfig, err := LoadAppConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got := appConfig.Providers["codex"].AccessToken; got != "old-access" {
		t.Fatalf("access token = %q after rejected update, want old-access", got)
	}
}
