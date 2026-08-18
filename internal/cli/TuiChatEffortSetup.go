package cli

import (
	"fmt"

	"orbit/internal/config"
	"orbit/internal/utils"

	tea "github.com/charmbracelet/bubbletea"
)

const effortSetupCommand = "/effort"

func (m model) startEffortSetup() (model, tea.Cmd) {

	appConfig, err := config.LoadAppConfig()
	if err != nil {
		m, _ = m.reportModelSetupError(fmt.Errorf("load effort config: %w", err))
		return m.commitTerminalTranscript(nil)
	}
	setup := initialConfigEffortForLanguage(appConfig.Language, appConfig.ThinkLevel)
	setup.terminalWidth = m.width
	setup.terminalHeight = m.height
	m.effortSetup = &setup
	m.setupScreenReady = false
	return m, tea.Sequence(tea.EnterAltScreen, markSetupScreenReady)
}

func (m model) updateEffortSetup(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		if size.Width <= 0 || size.Height <= 0 {
			return m, nil
		}
		m.width, m.height = size.Width, size.Height
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "ctrl+c", "q":
			return m.leaveEffortSetup(nil)
		}
	}
	updated, cmd := m.effortSetup.Update(msg)
	setup := updated.(ConfigEffortModel)
	m.effortSetup = &setup
	if !setup.Confirmed {
		return m, cmd
	}
	if err := config.SaveThinkLevelConfig(setup.SelectedThinkLevel); err != nil {
		return m.leaveEffortSetup(fmt.Errorf("save effort config: %w", err))
	}
	utils.ThinkLevel = setup.SelectedThinkLevel
	return m.leaveEffortSetup(nil)
}

func (m model) leaveEffortSetup(setupErr error) (model, tea.Cmd) {
	m.effortSetup = nil
	m.setupScreenReady = false
	m.composer.Focus()
	if setupErr == nil {
		return m.finishSetupScreenExit(nil)
	}
	m, printCmd := m.reportModelSetupError(setupErr)
	return m.finishSetupScreenExit(printCmd)
}
