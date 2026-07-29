package config

import (
	"errors"
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

// LoadAppConfig 从 YAML 文件读取应用配置。
func LoadAppConfig() (AppConfig, error) {
	path, err := GetAppConfigPath()
	if err != nil {
		return AppConfig{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return AppConfig{}, fmt.Errorf("read app config %q: %w", path, err)
	}
	var appConfig AppConfig
	if err := yaml.Unmarshal(data, &appConfig); err != nil {
		return AppConfig{}, fmt.Errorf("parse app config %q: %w", path, err)
	}
	appConfig.ThinkLevel = NormalizeThinkLevel(appConfig.ThinkLevel)
	return appConfig, nil
}

// LoadDefaultProviderConfig 读取当前默认供应商的完整配置。
func LoadDefaultProviderConfig() (ProviderConfig, error) {
	appConfig, err := LoadAppConfig()
	if err != nil {
		return ProviderConfig{}, err
	}
	if appConfig.DefaultProvider == "" {
		return ProviderConfig{}, errors.New("default_provider is empty")
	}
	providerConfig, ok := appConfig.Providers[appConfig.DefaultProvider]
	if !ok {
		return ProviderConfig{}, fmt.Errorf("default provider %q not found", appConfig.DefaultProvider)
	}
	providerConfig.Name = appConfig.DefaultProvider
	return providerConfig, nil
}

func LoadAppConfigValue(key string) (string, error) {
	path, err := GetAppConfigPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read app config %q: %w", path, err)
	}

	var appConfig map[string]interface{}
	if err := yaml.Unmarshal(data, &appConfig); err != nil {
		return "", fmt.Errorf("parse app config %q: %w", path, err)
	}

	value, ok := appConfig[key]
	if !ok {
		return "", fmt.Errorf("config key %q not found", key)
	}
	valuestr, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("config key %q is not a string", key)
	}
	return valuestr, nil
}
