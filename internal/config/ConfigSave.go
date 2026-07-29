package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// SaveAppConfig 将应用配置写入 YAML 文件，并确保配置目录存在。
func SaveAppConfig(appConfig AppConfig) error {
	appConfig.ThinkLevel = NormalizeThinkLevel(appConfig.ThinkLevel)
	path, err := GetAppConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create app config dir %q: %w", filepath.Dir(path), err)
	}
	data, err := yaml.Marshal(appConfig)
	if err != nil {
		return fmt.Errorf("marshal app config: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write app config %q: %w", path, err)
	}
	return nil
}

// SaveThinkLevelConfig updates the reasoning effort while preserving other settings.
func SaveThinkLevelConfig(thinkLevel string) error {
	appConfig, err := loadAppConfigOrEmpty()
	if err != nil {
		return err
	}
	appConfig.ThinkLevel = thinkLevel
	return SaveAppConfig(appConfig)
}

// SaveLanguageConfig 只更新语言配置，并保留 YAML 中已有的其他配置。
func SaveLanguageConfig(language string) error {
	appConfig, err := loadAppConfigOrEmpty()
	if err != nil {
		return err
	}
	appConfig.Language = language
	return SaveAppConfig(appConfig)
}

// SaveThemeConfig 只更新主题配置，并保留 YAML 中已有的其他配置。
func SaveThemeConfig(theme string) error {
	appConfig, err := loadAppConfigOrEmpty()
	if err != nil {
		return err
	}
	appConfig.Theme = theme
	return SaveAppConfig(appConfig)
}

// SaveProviderConfig 只更新模型提供商配置，并保留 YAML 中已有的其他配置。
func SaveProviderConfig(providerConfig ProviderConfig) error {
	appConfig, err := loadAppConfigOrEmpty()
	if err != nil {
		return err
	}
	if appConfig.Providers == nil {
		appConfig.Providers = make(map[string]ProviderConfig)
	}
	providerConfig.DefaultModel = providerConfig.Model
	appConfig.Providers[providerConfig.Name] = providerConfig
	appConfig.DefaultProvider = providerConfig.Name
	return SaveAppConfig(appConfig)
}

// SaveDefaultProvider 只切换默认供应商，并保留其他配置。
func SaveDefaultProvider(providerName string) error {
	appConfig, err := loadAppConfigOrEmpty()
	if err != nil {
		return err
	}
	if _, ok := appConfig.Providers[providerName]; !ok {
		return fmt.Errorf("provider %q not found", providerName)
	}
	appConfig.DefaultProvider = providerName
	return SaveAppConfig(appConfig)
}

// SaveDefaultModel updates the model used by the current default provider.
func SaveDefaultModel(modelName string) error {
	appConfig, err := loadAppConfigOrEmpty()
	if err != nil {
		return err
	}
	provider, ok := appConfig.Providers[appConfig.DefaultProvider]
	if !ok {
		return fmt.Errorf("default provider %q not found", appConfig.DefaultProvider)
	}
	provider.Model = modelName
	provider.DefaultModel = modelName
	appConfig.Providers[appConfig.DefaultProvider] = provider
	return SaveAppConfig(appConfig)
}

func loadAppConfigOrEmpty() (AppConfig, error) {
	appConfig, err := LoadAppConfig()
	if err == nil {
		return appConfig, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return AppConfig{}, nil
	}
	return AppConfig{}, err
}
