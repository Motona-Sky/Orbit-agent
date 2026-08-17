package test

import (
	"orbit/internal/config"
	"orbit/internal/oauth"
	"testing"
)

func TestParseAccessToken(t *testing.T) {
	config, _ := config.LoadAppConfig()
	access := config.Providers["codex"].AccessToken
	claims, err := oauth.ParseJWT(access)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	t.Logf("claims: %v", claims)

}

func TestRefreshTokens(t *testing.T) {
	err, body := oauth.RefreshTokens()
	if err != nil {
		t.Fatalf("refresh tokens: %v", err)
	}
	t.Logf("body: %v", body)
}
