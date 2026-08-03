package cli

import (
	"errors"
	"fmt"
	"os"

	"looporbit/internal/config"
	"looporbit/internal/memorys"
)

type runDependencies struct {
	openChat      func() error
	openSession   func() error
	createConfig  func() error
	runModelSetup func() error
}

type sessionFlowDependencies struct {
	loadConfig    func() (string, error)
	listSessions  func() ([]memorys.SessionSummary, int, error)
	selectSession func(string, []memorys.SessionSummary, int) (memorys.SessionSummary, bool, error)
	openChat      func(string, memorys.SessionSummary) error
}

// Run 使用默认依赖分派命令行参数，并返回适合作为进程退出状态的状态码。
func Run(args []string) int {
	return runWithDependencies(args, runDependencies{
		openChat:      OpenChatTui,
		openSession:   func() error { return openSessionFlow(defaultSessionFlowDependencies()) },
		createConfig:  CreateConfig,
		runModelSetup: runModelSetupCommand,
	})
}

// runWithDependencies 使用注入的依赖分派命令，统一输出错误并转换为退出状态码。
func runWithDependencies(args []string, deps runDependencies) int {
	var err error
	switch {
	case len(args) == 0:
		err = deps.openChat()
	case args[0] == "setup":
		err = deps.createConfig()
	case args[0] == "model":
		err = deps.runModelSetup()
	case args[0] == "-s" || args[0] == "--session" || args[0] == "session":
		err = deps.openSession()
	default:
		err = deps.openChat()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

// runModelSetupCommand 读取当前语言，完成模型提供商选择并持久化选择结果。
func runModelSetupCommand() error {
	language, err := config.LoadAppConfigValue("language")
	if err != nil {
		return err
	}
	providerConfig, providerOK, err := OpenProviderTuiForLanguage(language)
	if err != nil {
		return err
	}
	if !providerOK {
		return errors.New("provider setup canceled")
	}
	return config.SaveProviderConfig(providerConfig)
}

// defaultSessionFlowDependencies 组装历史会话扫描、选择和恢复聊天所需的生产依赖。
func defaultSessionFlowDependencies() sessionFlowDependencies {
	return sessionFlowDependencies{
		loadConfig: func() (string, error) {
			appConfig, err := config.LoadAppConfig()
			return appConfig.Language, err
		},
		listSessions: func() ([]memorys.SessionSummary, int, error) {
			// 直接从 config 包取会话目录路径，避免依赖 utils.ChatHistoryFolder 全局变量
			// —— 该全局变量只在 TUI Init() 里通过 utils.GInit() 初始化，
			// 而 -s 流程在列出会话时尚未进入 TUI，全局变量仍为空串会导致扫描当前工作目录。
			paths, err := config.GetConfigFolderPath()
			if err != nil {
				return nil, 0, err
			}
			return memorys.ListSessions(paths["ChatHistoryFolder"])
		},
		selectSession: OpenSessionTui,
		openChat:      OpenChatTuiForSession,
	}
}

// openSessionFlow 依次加载配置、列出会话、处理选择结果并打开选中的历史会话。
func openSessionFlow(deps sessionFlowDependencies) error {
	language, err := deps.loadConfig()
	if err != nil {
		return fmt.Errorf("load app config: %w; run looporbit setup first", err)
	}
	sessions, skipped, err := deps.listSessions()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}
	selected, confirmed, err := deps.selectSession(language, sessions, skipped)
	if err != nil {
		return err
	}
	if !confirmed {
		return nil
	}
	if selected.ID == "" {
		return errors.New("selected session id is empty")
	}
	return deps.openChat(language, selected)
}
