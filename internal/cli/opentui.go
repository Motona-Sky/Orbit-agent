package cli

import (
	"context"
	"errors"
	"fmt"
	"orbit/internal/config"
	"orbit/internal/mcp"
	"orbit/internal/memorys"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// OpenLanTui 打开语言选择 TUI。
func OpenLanTui() (string, bool, error) {
	p := tea.NewProgram(initialConfigLan(), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", false, fmt.Errorf("run language setup tui: %w", err)
	}
	m, ok := finalModel.(ConfigLanModel)
	if !ok || !m.Confirmed {
		return "", false, nil
	}
	return m.SelectedLanguageCode, true, nil
}

// // 打开主题选择 TUI
// func OpenThemeTui() (string, bool, error) {
// 	return OpenThemeTuiForLanguage("")
// }

func OpenThemeTuiForLanguage(languageCode string) (string, bool, error) {
	p := tea.NewProgram(initialConfigThemeForLanguage(languageCode), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", false, fmt.Errorf("run theme setup tui: %w", err)
	}
	m, ok := finalModel.(ConfigThemeModel)
	if !ok || !m.Confirmed {
		return "", false, nil
	}
	return m.SelectedTheme, true, nil
}

// 模型提供者选择 TUI
// func OpenProviderTui() (config.ProviderConfig, bool, error) {
// 	return OpenProviderTuiForLanguage("")
// }

func OpenProviderTuiForLanguage(languageCode string) (config.ProviderConfig, bool, error) {
	appConfig, err := config.LoadAppConfig()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return config.ProviderConfig{}, false, err
	}
	p := tea.NewProgram(initialConfigProviderForLanguageWithConfig(languageCode, appConfig), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return config.ProviderConfig{}, false, fmt.Errorf("run provider setup tui: %w", err)
	}
	m, ok := finalModel.(ConfigProviderModel)
	if !ok || !m.Confirmed {
		return config.ProviderConfig{}, false, nil
	}
	return providerConfigFromModel(m), true, nil
}

// 入口，打开chattui (主界面)
func OpenSessionTui(language string, sessions []memorys.SessionSummary, skipped int) (memorys.SessionSummary, bool, error) {
	p := tea.NewProgram(initialSessionModel(language, sessions, skipped), tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return memorys.SessionSummary{}, false, fmt.Errorf("run session selector tui: %w", err)
	}
	m, ok := finalModel.(SessionModel)
	if !ok {
		return memorys.SessionSummary{}, false, fmt.Errorf("unexpected session selector model %T", finalModel)
	}
	if !m.Confirmed {
		return memorys.SessionSummary{}, false, nil
	}
	return m.SelectedSession, true, nil
}

func runWithMcp(run func() error) error {
	ctx, cancel := context.WithCancel(context.Background())
	clients, err := mcp.RunMcp(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "MCP startup warning: %v\n", err)
	}
	defer func() {
		cancel()
		for _, client := range clients {
			_ = client.Close()
		}
		mcp.ClearMcpState()
	}()
	return run()
}

func OpenChatTuiForSession(language string, session memorys.SessionSummary) error {
	return runWithMcp(func() error {
		p := newChatProgram(NewModelForSession(language, session))
		finalModel, err := p.Run()
		if err != nil {
			return fmt.Errorf("run restored chat tui: %w", err)
		}
		if m, ok := finalModel.(model); ok && m.wantsRestart {
			return runChatLoopWithoutMcp()
		}
		return nil
	})
}

func OpenChatTui() error {
	configPath, err := config.GetAppConfigPath() //获取配置文件路径 get config file path
	if err != nil {
		return fmt.Errorf("resolve app config path: %w", err)
	}
	exists, err := appConfigFileExists(configPath) //判断配置文件是否存在
	if err != nil {
		return err
	}
	if !exists {
		return CreateConfig() //配置文件入口函数 config file entry function
	}

	return runChatLoop()
}

func runChatLoop() error {
	return runWithMcp(runChatLoopWithoutMcp)
}

// runChatLoopWithoutMcp 运行主聊天 TUI，当用户执行 /new 或 /clear 时自动重启新会话。
func runChatLoopWithoutMcp() error {
	for {
		p := newChatProgram(NewModelFromConfig())
		finalModel, err := p.Run()
		if err != nil {
			return fmt.Errorf("run chat tui: %w", err)
		}
		m, ok := finalModel.(model)
		if !ok || !m.wantsRestart {
			return nil
		}
		// wantsRestart == true，循环重新创建 TUI
	}
}

// newChatProgram 使用普通终端屏幕启动主聊天，并允许测试注入输入输出选项。
func newChatProgram(initial tea.Model, options ...tea.ProgramOption) *tea.Program {
	return tea.NewProgram(initial, options...)
}

// 检测配置是否存在
func appConfigFileExists(configPath string) (bool, error) {
	info, err := os.Stat(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat app config %q: %w", configPath, err)
	}
	if info.IsDir() {
		return false, fmt.Errorf("app config path %q is a directory", configPath)
	}
	return true, nil
}

// ===========================================================
