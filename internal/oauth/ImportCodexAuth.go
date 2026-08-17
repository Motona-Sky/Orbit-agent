package oauth

import (
	"encoding/json"
	"fmt"
	"orbit/internal/config"
	"os"
	"path/filepath"
)

type CodexToken struct {
	Auth_mode      string            `json:"auth_mode"`
	OPENAI_API_KEY string            `json:"OPENAI_API_KEY"`
	Tokens         map[string]string `json:"tokens"`
	Last_refresh   string            `json:"last_refresh"`
}
type CodexAuthToken struct {
	Id_token      string `json:"id_token"`
	AccessToken   string `json:"access_token"`
	Refresh_token string `json:"refresh_token"`
	Account_id    string `json:"account_id"`
}

func ImportCodexAuth() (*CodexToken, error) {
	if !CheckCodexAuth() {
		return nil, os.ErrNotExist
	}
	return loadCodexAuth()
}

func CheckCodexAuth() bool {
	userpath, err := config.GetUserHomePath()
	if err != nil {
		return false
	}
	path := filepath.Join(userpath, ".codex", "auth.json")
	_, err = os.Stat(path)
	return err == nil
}
func loadCodexAuth() (*CodexToken, error) {
	userpath, _ := config.GetUserHomePath()
	path := filepath.Join(userpath, ".codex", "auth.json")
	_, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("load codex auth: %w", err)
	}
	Codexauth, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load codex auth: %w", err)
	}
	var codextoken CodexToken
	err = json.Unmarshal(Codexauth, &codextoken)
	if err != nil {
		return nil, fmt.Errorf("load codex auth: %w", err)
	}
	return &codextoken, nil
}
func GetConfigCodexAuth() (config.ProviderConfig, error) {
	authconfig, err := ImportCodexAuth()
	if err != nil {
		return config.ProviderConfig{}, err
	}
	return codexProviderConfig(
		authconfig.Tokens["id_token"],
		authconfig.Tokens["access_token"],
		authconfig.Tokens["refresh_token"],
		authconfig.Tokens["account_id"],
	), nil
}

func codexProviderConfig(idToken, accessToken, refreshToken, accountID string) config.ProviderConfig {
	providerConfig := config.ProviderConfig{
		Name:         "codex",
		Auth:         "codex",
		BaseURL:      "https://chatgpt.com/backend-api",
		Model:        "",
		Type:         "oauth:codex",
		DefaultModel: "",
		AccountID:    accountID,
		IDToken:      idToken,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}
	if user, err := GetOauthUser(accessToken); err == nil {
		providerConfig.User = &config.OauthUser{
			User:      user.User,
			Workspace: user.Workspace,
		}
	}
	return providerConfig
}
