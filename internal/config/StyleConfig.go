package config

import (
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

const (
	StyleConfigFileName      = "style.yaml"
	StyleConfigPathEnv       = "ORBIT_STYLE_CONFIG_PATH"
	LegacyStyleConfigPathEnv = "LOOPORBIT_STYLE_CONFIG_PATH"
)

type StyleConfig struct {
	Palette StylePalette `yaml:"palette"`
	Layout  StyleLayout  `yaml:"layout"`
}

type StylePalette struct {
	Background string `yaml:"background"`
	Foreground string `yaml:"foreground"`
	Muted      string `yaml:"muted"`
	Accent     string `yaml:"accent"`
	PanelFill  string `yaml:"panel_fill"`
	OptionFill string `yaml:"option_fill"`
	Divider    string `yaml:"divider"`
}

type StyleLayout struct {
	FallbackWidth  int `yaml:"fallback_width"`
	FallbackHeight int `yaml:"fallback_height"`
	MinWidth       int `yaml:"min_width"`
	MinHeight      int `yaml:"min_height"`
	MaxPanelWidth  int `yaml:"max_panel_width"`
	PanelHeight    int `yaml:"panel_height"`
	MinPanelHeight int `yaml:"min_panel_height"`
}

func GetStyleConfigPath() (string, error) {
	if path := os.Getenv(StyleConfigPathEnv); path != "" {
		return path, nil
	}
	if path := os.Getenv(LegacyStyleConfigPathEnv); path != "" {
		return path, nil
	}
	return filepath.Join(ConfigPath, StyleConfigFileName), nil
}

func SaveStyleConfig(styleConfig StyleConfig) error {
	path, err := GetStyleConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create style config dir %q: %w", filepath.Dir(path), err)
	}
	data, err := yaml.Marshal(styleConfig)
	if err != nil {
		return fmt.Errorf("marshal style config: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write style config %q: %w", path, err)
	}
	return nil
}

func LoadStyleConfig() (StyleConfig, error) {
	path, err := GetStyleConfigPath()
	if err != nil {
		return StyleConfig{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return StyleConfig{}, fmt.Errorf("read style config %q: %w", path, err)
	}
	var styleConfig StyleConfig
	if err := yaml.Unmarshal(data, &styleConfig); err != nil {
		return StyleConfig{}, fmt.Errorf("parse style config %q: %w", path, err)
	}
	return styleConfig, nil
}
