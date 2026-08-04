package cli

import (
	"fmt"
	"orbit/internal/config"
	"orbit/internal/style"
)

func CreateConfig() error { //创建配置文件 setup
	// 创建目录
	if _, err := config.CreateConfigFolder(); err != nil {
		return fmt.Errorf("create config folder: %w", err)
	}

	languageCode, lanOk, err := OpenLanTui()
	if err != nil {
		return fmt.Errorf("select language: %w", err)
	}
	if !lanOk {
		return fmt.Errorf("language selection cancelled")
	}

	theme, themeOk, err := OpenThemeTuiForLanguage(languageCode)
	if err != nil {
		return fmt.Errorf("select theme: %w", err)
	}
	if !themeOk {
		return fmt.Errorf("theme selection cancelled")
	}
	if err := saveThemePreset(theme); err != nil {
		return err
	}

	providerConfig, providerOk, err := OpenProviderTuiForLanguage(languageCode)
	if err != nil {
		return fmt.Errorf("select provider: %w", err)
	}
	if !providerOk {
		return fmt.Errorf("provider selection cancelled")
	}

	if err := config.SaveLanguageConfig(languageCode); err != nil {
		return fmt.Errorf("save language config: %w", err)
	}
	if err := config.SaveProviderConfig(providerConfig); err != nil {
		return fmt.Errorf("save provider config: %w", err)
	}
	if err := OpenChatTui(); err != nil {
		return fmt.Errorf("Create config open chat: %w", err)
	}
	return nil
}

func saveThemePreset(theme string) error {
	styleConfig, ok := style.ThemeConfigByKey(theme)
	if !ok {
		return fmt.Errorf("unknown theme style %q", theme)
	}
	if err := config.SaveStyleConfig(styleConfig); err != nil {
		return fmt.Errorf("save style config: %w", err)
	}
	return nil
}
