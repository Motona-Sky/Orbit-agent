package utils

import "looporbit/internal/config"

// ReloadThinkLevelConfig refreshes the runtime reasoning effort from application config.
func ReloadThinkLevelConfig() error {
	appConfig, err := config.LoadAppConfig()
	if err != nil {
		return err
	}
	ThinkLevel = appConfig.ThinkLevel
	return nil
}

func mustReloadThinkLevelConfig() {
	if err := ReloadThinkLevelConfig(); err != nil {
		panic(err)
	}
}
