package test

import (
	"orbit/internal/oauth"
	"orbit/internal/utils"
	"testing"
)

func TestLoadCodexAuth(t *testing.T) {
	tokentime, _ := oauth.ParseAccessToken(utils.AccessToken)
	if tokentime {
		t.Logf("access token is valid")
	}
}
