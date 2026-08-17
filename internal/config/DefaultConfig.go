package config

import (
	"os"
	"path/filepath"
)

const (
	// AppConfigFileName 是 Orbit 当前使用的 YAML 配置文件名。
	AppConfigFileName = "Orbitconfig.yaml"

	// AppConfigPathEnv 只用于测试或显式覆盖配置文件位置，避免测试写入真实用户目录。
	AppConfigPathEnv = "ORBIT_CONFIG_PATH"
)

// AppConfig 是持久化到 YAML 的最小应用配置。
type AppConfig struct {
	Language        string                    `yaml:"language"`
	Theme           string                    `yaml:"theme"`
	ThinkLevel      string                    `yaml:"think_level"`
	DefaultProvider string                    `yaml:"default_provider"`
	Providers       map[string]ProviderConfig `yaml:"providers"`
}

// ProviderConfig 是模型提供商相关的持久化配置值。
type OauthUser struct {
	User      string `yaml:"user,omitempty"`
	Workspace string `yaml:"workspace,omitempty"`
}

type ProviderConfig struct {
	Name         string     `yaml:"-"`
	Auth         string     `yaml:"auth"`
	ApiKey       string     `yaml:"api_key,omitempty"`
	BaseURL      string     `yaml:"base_url,omitempty"`
	Model        string     `yaml:"model,omitempty"`
	Type         string     `yaml:"type,omitempty"`
	DefaultModel string     `yaml:"default_model"`
	IDToken      string     `yaml:"Id_token,omitempty"`
	AccessToken  string     `yaml:"AccessToken,omitempty"`
	RefreshToken string     `yaml:"Refresh_token,omitempty"`
	AccountID    string     `yaml:"Account_id,omitempty"`
	User         *OauthUser `yaml:"User,omitempty"`
}

// GetAppConfigPath 返回 YAML 配置文件路径，优先使用环境变量覆盖。
func GetAppConfigPath() (string, error) {
	if path := os.Getenv(AppConfigPathEnv); path != "" {
		return path, nil
	}
	return filepath.Join(ConfigPath, AppConfigFileName), nil
}
