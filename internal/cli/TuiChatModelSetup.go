package cli

import (
	"fmt"
	"strings"

	"orbit/internal/config"
	"orbit/internal/utils"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	modelSetupCommand    = "/model"
	providerSetupCommand = "/provider"
	newCommand           = "/new"
	clearCommand         = "/clear"
)

type setupScreenReadyMsg struct{}

type modelListLoadedMsg struct {
	requestID uint64
	models    []string
	err       error
}

func markSetupScreenReady() tea.Msg {
	return setupScreenReadyMsg{}
}

func loadModelList(requestID uint64) tea.Cmd {
	return func() tea.Msg {
		models, err := config.GetModelList()
		return modelListLoadedMsg{requestID: requestID, models: models, err: err}
	}
}

func (m model) handleSlashMessageSubmit(value string) (model, tea.Cmd) {
	m.clearComposer()
	m.exitConfirm = false
	command := strings.TrimSpace(value)
	if command == skillsCommand || strings.HasPrefix(command, skillsCommand+" ") {
		return m.handleSkillsCommand(command)
	}
	switch command {
	case modelSetupCommand:
		return m.startModelSetup()
	case providerSetupCommand:
		return m.startProviderSetup()
	case effortSetupCommand:
		return m.startEffortSetup()
	case mcpCommand:
		return m.startMcpPage()
	case newCommand, clearCommand:
		return m.startNewConversation()
	default:
		return m, nil
	}
}

func (m model) startModelSetup() (model, tea.Cmd) {
	appConfig, err := config.LoadAppConfig()
	if err != nil {
		return m.reportModelSetupError(fmt.Errorf("load model config: %w", err))
	}
	provider, err := config.LoadDefaultProviderConfig()
	if err != nil {
		return m.reportModelSetupError(fmt.Errorf("load default provider: %w", err))
	}
	setup := initialConfigModelForLanguage(appConfig.Language, provider.Model)
	m.modelSetupRequestID++
	setup.requestID = m.modelSetupRequestID
	setup.terminalWidth = m.width
	setup.terminalHeight = m.height
	m.modelSetup = &setup
	m.setupScreenReady = false
	return m, tea.Sequence(tea.EnterAltScreen, markSetupScreenReady, loadModelList(setup.requestID))
}

func (m model) updateModelSetup(msg tea.Msg) (tea.Model, tea.Cmd) {
	if loaded, ok := msg.(modelListLoadedMsg); ok {
		if loaded.requestID != 0 && loaded.requestID != m.modelSetup.requestID {
			return m, nil
		}
		if loaded.err != nil {
			return m.leaveModelSetup(fmt.Errorf("load model list: %w", loaded.err))
		}
		m.modelSetup.setModels(loaded.models)
		return m, nil
	}
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		if size.Width <= 0 || size.Height <= 0 {
			return m, nil
		}
		m.width, m.height = size.Width, size.Height
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "esc" || key.String() == "ctrl+c" ||
			(key.String() == "q" && m.modelSetup.step == modelStepSelect) {
			return m.leaveModelSetup(nil)
		}
	}
	updated, cmd := m.modelSetup.Update(msg)
	setup := updated.(ConfigModelModel)
	m.modelSetup = &setup
	if !setup.Confirmed {
		return m, cmd
	}
	if err := config.SaveDefaultModel(setup.SelectedModel); err != nil {
		return m.leaveModelSetup(fmt.Errorf("save model config: %w", err))
	}
	if err := utils.ReloadProviderConfig(); err != nil {
		return m.leaveModelSetup(fmt.Errorf("reload saved model config: %w", err))
	}
	return m.leaveModelSetup(nil)
}

func (m model) leaveModelSetup(setupErr error) (model, tea.Cmd) {
	m.modelSetup = nil
	m.setupScreenReady = false
	m.composer.Focus()
	if setupErr == nil {
		return m.finishSetupScreenExit(nil)
	}
	m, printCmd := m.reportModelSetupError(setupErr)
	return m.finishSetupScreenExit(printCmd)
}

func (m model) startProviderSetup() (model, tea.Cmd) {
	appConfig, err := config.LoadAppConfig()
	if err != nil {
		return m.reportModelSetupError(fmt.Errorf("load provider config: %w", err))
	}
	setup := initialConfigProviderForLanguageWithConfig(appConfig.Language, appConfig)
	setup.terminalWidth = m.width
	setup.terminalHeight = m.height
	m.providerSetup = &setup
	m.setupScreenReady = false
	return m, tea.Sequence(tea.EnterAltScreen, markSetupScreenReady)
}

func (m model) updateProviderSetup(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		if size.Width <= 0 || size.Height <= 0 {
			return m, nil
		}
		m.width, m.height = size.Width, size.Height
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		if key.String() == "esc" || key.String() == "ctrl+c" ||
			(key.String() == "q" && m.providerSetup.step == providerStepSelect) {
			return m.leaveProviderSetup(nil)
		}
	}
	updated, cmd := m.providerSetup.Update(msg)
	setup := updated.(ConfigProviderModel)
	m.providerSetup = &setup
	if !setup.Confirmed {
		return m, cmd
	}

	selected := providerConfigFromModel(setup)
	if err := config.SaveProviderConfig(selected); err != nil {
		return m.leaveProviderSetup(fmt.Errorf("save provider config: %w", err))
	}
	if err := utils.ReloadProviderConfig(); err != nil {
		return m.leaveProviderSetup(fmt.Errorf("reload saved provider config: %w", err))
	}
	return m.leaveProviderSetup(nil)
}

func (m model) leaveProviderSetup(setupErr error) (model, tea.Cmd) {
	m.providerSetup = nil
	m.setupScreenReady = false
	m.composer.Focus()
	if setupErr == nil {
		return m.finishSetupScreenExit(nil)
	}
	m, printCmd := m.reportModelSetupError(setupErr)
	return m.finishSetupScreenExit(printCmd)
}

func (m model) finishSetupScreenExit(afterExit tea.Cmd) (model, tea.Cmd) {
	exitCmd := tea.Cmd(tea.ExitAltScreen)
	if m.screenInitialized && m.lastTerminalHeight > 0 {
		heightDelta := m.height - m.lastTerminalHeight
		m.lastTerminalHeight = m.height
		if heightDelta > 0 {
			exitCmd = sequenceTeaCommands(exitCmd, tea.Println(terminalBlankLines(heightDelta)))
		}
	}
	return m, sequenceTeaCommands(exitCmd, afterExit)
}

func (m model) reportModelSetupError(err error) (model, tea.Cmd) {
	m.transcript = append(m.transcript, chatTranscriptEntry{
		kind: transcriptMessage, role: "assistant",
		content: m.messages.Chat.AgentErrorLabel + err.Error(),
	})
	return m.commitTerminalTranscript(nil)
}

func sequenceTeaCommands(first, second tea.Cmd) tea.Cmd {
	if first == nil {
		return second
	}
	if second == nil {
		return first
	}
	return tea.Sequence(first, second)
}

// startNewConversation 设置重启标志并退出 TUI，由外层循环重新启动。
func (m model) startNewConversation() (model, tea.Cmd) {
	if m.running {
		m.closeAgentUI()
		m.running = false
		m.stopRunningStatus()
	}
	if m.agentUI != nil {
		m.agentUI.Close()
		m.agentUI = nil
	}
	m.wantsRestart = true
	return m, tea.Quit
}
