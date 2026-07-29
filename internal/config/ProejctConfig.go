package config

import (
	"os"
	"path/filepath"
)

func CreateProjectConfig() error {
	configPath := filepath.Join(Cwd, ".looporbit")

	if err := os.MkdirAll(configPath, 0755); err != nil {
		return err
	}

	return nil
}
