package cli

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"looporbit/internal/agentui"
	"looporbit/internal/config"
	"looporbit/internal/utils"

	tea "github.com/charmbracelet/bubbletea"
)

func assertCommandMessageType(t *testing.T, cmd tea.Cmd, want tea.Msg) {
	t.Helper()
	if cmd == nil {
		t.Fatal("command = nil")
	}
	if got := cmd(); reflect.TypeOf(got) != reflect.TypeOf(want) {
		t.Fatalf("command message type = %T, want %T", got, want)
	}
}

func writeModelSetupConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), config.AppConfigFileName)
	t.Setenv(config.AppConfigPathEnv, path)
	if err := config.SaveAppConfig(config.AppConfig{
		Language:        "zh-CN",
		DefaultProvider: "openai",
		Providers: map[string]config.ProviderConfig{
			"openai": {ApiKey: "old-key", BaseURL: "https://old.example/v1", Model: "old-model", Type: "openai:completions"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestModelCommandOpensAndCancelsEmbeddedModelSetup(t *testing.T) {
	writeModelSetupConfig(t)
	m := NewModelForLanguage("zh-CN")
	m.setComposerValue(" /model ")
	m, enterCmd := m.handleComposerSubmit()
	if m.modelSetup == nil || m.composerText() != "" {
		t.Fatalf("model setup = %#v, composer = %q", m.modelSetup, m.composerText())
	}
	if view := m.View(); view != "" {
		t.Fatalf("model setup rendered before alternate screen entry:\n%s", view)
	}
	if enterCmd == nil {
		t.Fatal("model setup did not schedule alternate screen entry")
	}

	updated, exitCmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(model)
	if m.modelSetup == nil || exitCmd != nil {
		t.Fatal("model setup accepted cancel before alternate screen entry")
	}
	updated, _ = m.Update(setupScreenReadyMsg{})
	m = updated.(model)
	if !strings.Contains(m.View(), m.messages.ModelSetup.Heading) {
		t.Fatalf("model setup did not render after alternate screen entry:\n%s", m.View())
	}
	updated, exitCmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = updated.(model)
	if m.modelSetup != nil {
		t.Fatal("model setup remained active after ready cancel")
	}
	assertCommandMessageType(t, exitCmd, tea.ExitAltScreen())
}

func TestEmbeddedProviderSetupEscapeReturnsToChatWithoutSaving(t *testing.T) {
	steps := []struct {
		name string
		step providerStep
	}{
		{name: "select", step: providerStepSelect},
		{name: "name", step: providerStepName},
		{name: "base URL", step: providerStepBaseURL},
		{name: "API key", step: providerStepAPIKey},
		{name: "model", step: providerStepModel},
		{name: "type", step: providerStepType},
		{name: "confirm", step: providerStepConfirm},
	}

	for _, tt := range steps {
		t.Run(tt.name, func(t *testing.T) {
			writeModelSetupConfig(t)
			beforeConfig, err := config.LoadAppConfig()
			if err != nil {
				t.Fatal(err)
			}
			oldAPIKey, oldBaseURL, oldModel, oldProvider := utils.ApiKey, utils.BaseUrl, utils.Model, utils.Provider
			t.Cleanup(func() {
				utils.ApiKey, utils.BaseUrl = oldAPIKey, oldBaseURL
				utils.Model, utils.Provider = oldModel, oldProvider
			})

			m := NewModelForLanguage("zh-CN")
			setup := initialConfigProviderForLanguageWithConfig("zh-CN", beforeConfig)
			setup.step = tt.step
			setup.SelectedProvider = "replacement"
			setup.SelectedAPIKey = "new-key"
			setup.SelectedBaseURL = "https://new.example/v1"
			setup.SelectedModel = "new-model"
			setup.SelectedType = "openai:completions"
			m.providerSetup = &setup
			m.setupScreenReady = true
			m.composer.Blur()

			updated, exitCmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			m = updated.(model)
			if m.providerSetup != nil {
				t.Fatal("provider setup remained active after Esc")
			}
			if !m.composer.Focused() {
				t.Fatal("composer did not regain focus after Esc")
			}
			assertCommandMessageType(t, exitCmd, tea.ExitAltScreen())

			afterConfig, err := config.LoadAppConfig()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(afterConfig, beforeConfig) {
				t.Fatalf("config changed after Esc: got %#v, want %#v", afterConfig, beforeConfig)
			}
			if utils.ApiKey != oldAPIKey || utils.BaseUrl != oldBaseURL || utils.Model != oldModel || utils.Provider != oldProvider {
				t.Fatal("runtime provider changed after Esc")
			}
		})
	}
}

func TestEmbeddedEffortSetupEscapeReturnsToChatWithoutSaving(t *testing.T) {
	writeModelSetupConfig(t)
	oldThinkLevel := utils.ThinkLevel
	t.Cleanup(func() { utils.ThinkLevel = oldThinkLevel })
	utils.ThinkLevel = "high"

	m := NewModelForLanguage("zh-CN")
	setup := initialConfigEffortForLanguage("zh-CN", "high")
	setup.effortCursor = 0
	m.effortSetup = &setup
	m.setupScreenReady = true
	m.composer.Blur()

	updated, exitCmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(model)
	if m.effortSetup != nil {
		t.Fatal("effort setup remained active after Esc")
	}
	if !m.composer.Focused() {
		t.Fatal("composer did not regain focus after Esc")
	}
	assertCommandMessageType(t, exitCmd, tea.ExitAltScreen())
	if utils.ThinkLevel != "high" {
		t.Fatalf("runtime think level = %q, want high", utils.ThinkLevel)
	}
	appConfig, err := config.LoadAppConfig()
	if err != nil {
		t.Fatal(err)
	}
	if appConfig.ThinkLevel != "high" {
		t.Fatalf("saved think level = %q, want high", appConfig.ThinkLevel)
	}
}

func TestStandaloneSetupModelsDoNotExitOnEscape(t *testing.T) {
	t.Run("provider", func(t *testing.T) {
		setup := initialConfigProviderForLanguageWithConfig("zh-CN", config.AppConfig{})
		setup.step = providerStepName
		updated, cmd := setup.Update(tea.KeyMsg{Type: tea.KeyEsc})
		got := updated.(ConfigProviderModel)
		if cmd != nil || got.step != providerStepName || got.Confirmed {
			t.Fatalf("standalone provider changed on Esc: step=%v confirmed=%v cmd=%v", got.step, got.Confirmed, cmd)
		}
	})

	t.Run("effort", func(t *testing.T) {
		setup := initialConfigEffortForLanguage("zh-CN", "high")
		beforeCursor := setup.effortCursor
		updated, cmd := setup.Update(tea.KeyMsg{Type: tea.KeyEsc})
		got := updated.(ConfigEffortModel)
		if cmd != nil || got.effortCursor != beforeCursor || got.Confirmed {
			t.Fatalf("standalone effort changed on Esc: cursor=%d confirmed=%v cmd=%v", got.effortCursor, got.Confirmed, cmd)
		}
	})
}

func TestProviderSetupIgnoresInvalidWindowSize(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.width, m.height = 80, 24
	setup := initialConfigProviderForLanguageWithConfig("zh-CN", config.AppConfig{})
	m.providerSetup = &setup
	updated, _ := m.Update(tea.WindowSizeMsg{})
	m = updated.(model)
	if m.width != 80 || m.height != 24 {
		t.Fatalf("invalid provider size changed chat dimensions to %dx%d", m.width, m.height)
	}
}

func TestProviderSetupTracksValidWindowSizeBeforeScreenReady(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	setup := initialConfigProviderForLanguageWithConfig("zh-CN", config.AppConfig{})
	m.providerSetup = &setup
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 96, Height: 32})
	m = updated.(model)
	if m.width != 96 || m.height != 32 || m.providerSetup.terminalWidth != 96 || m.providerSetup.terminalHeight != 32 {
		t.Fatalf("sizes chat=%dx%d setup=%dx%d", m.width, m.height, m.providerSetup.terminalWidth, m.providerSetup.terminalHeight)
	}
}

func TestUnknownSlashCommandKeepsExistingNoOpBehavior(t *testing.T) {
	m := NewModel()
	m.setComposerValue("/unknown")
	m, cmd := m.handleComposerSubmit()
	if cmd != nil || m.providerSetup != nil || m.effortSetup != nil || m.modelSetup != nil || m.pendingInput != "" || m.composerText() != "" {
		t.Fatalf("unknown slash command changed model: %#v, cmd=%v", m, cmd)
	}
}

func TestProviderSetupConfirmSavesReloadsAndReturnsToChat(t *testing.T) {
	writeModelSetupConfig(t)
	oldAPIKey, oldBaseURL, oldModel, oldProvider := utils.ApiKey, utils.BaseUrl, utils.Model, utils.Provider
	t.Cleanup(func() {
		utils.ApiKey, utils.BaseUrl = oldAPIKey, oldBaseURL
		utils.Model, utils.Provider = oldModel, oldProvider
	})

	m := NewModelForLanguage("zh-CN")
	setup := initialConfigProviderForLanguageWithConfig("zh-CN", config.AppConfig{})
	setup.step = providerStepConfirm
	setup.SelectedProvider = "replacement"
	setup.SelectedAPIKey = "new-key"
	setup.SelectedBaseURL = "https://new.example/v1"
	setup.SelectedModel = "new-model"
	setup.SelectedType = "openai:completions"
	m.providerSetup = &setup
	m.setupScreenReady = true

	updated, exitCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.providerSetup != nil {
		t.Fatal("provider setup remained active after confirmation")
	}
	assertCommandMessageType(t, exitCmd, tea.ExitAltScreen())
	saved, err := config.LoadDefaultProviderConfig()
	if err != nil {
		t.Fatal(err)
	}
	if saved.Name != "replacement" || saved.Model != "new-model" {
		t.Fatalf("saved provider = %#v", saved)
	}
	if utils.ApiKey != "new-key" || utils.BaseUrl != "https://new.example/v1" || utils.Model != "new-model" || utils.Provider != "openai:completions" {
		t.Fatal("runtime provider was not refreshed")
	}
}

func TestModelSetupLoadFailureStaysInChat(t *testing.T) {
	t.Setenv(config.AppConfigPathEnv, t.TempDir())
	m := NewModelForLanguage("zh-CN")
	m.setComposerValue("/model")
	m, cmd := m.handleComposerSubmit()
	if m.providerSetup != nil {
		t.Fatal("provider setup opened after load failure")
	}
	if cmd == nil || !strings.Contains(m.transcript[len(m.transcript)-1].content, m.messages.Chat.AgentErrorLabel) {
		t.Fatalf("load failure was not reported: %#v", m.transcript)
	}
}

func TestProviderSetupSaveFailureLeavesRuntimeConfigUnchanged(t *testing.T) {
	t.Setenv(config.AppConfigPathEnv, t.TempDir())
	oldAPIKey, oldBaseURL, oldModel, oldProvider := utils.ApiKey, utils.BaseUrl, utils.Model, utils.Provider
	m := NewModelForLanguage("zh-CN")
	setup := initialConfigProviderForLanguageWithConfig("zh-CN", config.AppConfig{})
	setup.step = providerStepConfirm
	setup.SelectedProvider = "replacement"
	setup.SelectedAPIKey = "new-key"
	setup.SelectedBaseURL = "https://new.example/v1"
	setup.SelectedModel = "new-model"
	setup.SelectedType = "openai:completions"
	m.providerSetup = &setup
	m.setupScreenReady = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.providerSetup != nil {
		t.Fatalf("save failure did not leave setup cleanly: %#v", m)
	}
	if cmd == nil {
		t.Fatal("save failure did not schedule exit and error output")
	}
	if utils.ApiKey != oldAPIKey || utils.BaseUrl != oldBaseURL || utils.Model != oldModel || utils.Provider != oldProvider {
		t.Fatal("save failure changed runtime config")
	}
	if !strings.Contains(m.transcript[len(m.transcript)-1].content, m.messages.Chat.AgentErrorLabel) {
		t.Fatalf("save failure was not reported: %#v", m.transcript)
	}
}

func TestRunningModelCommandUsesSinglePendingInput(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.running = true
	m, _ = m.handleComposerSubmitValue("/model")
	m, secondCmd := m.handleComposerSubmitValue("/effort")
	if m.pendingInput != "/model" || m.providerSetup != nil || m.effortSetup != nil || secondCmd != nil {
		t.Fatalf("pending setup state = %#v", m)
	}
	if len(m.transcript) != 0 {
		t.Fatalf("pending command entered transcript: %#v", m.transcript)
	}
}

func TestQueuedModelSetupOpensAfterEveryTurnEnding(t *testing.T) {
	tests := []struct {
		name string
		end  func(model) model
	}{
		{
			name: "result",
			end: func(m model) model {
				updated, _ := m.Update(agentui.ResultEvent{Text: "done"})
				return updated.(model)
			},
		},
		{
			name: "runner error",
			end: func(m model) model {
				ui := agentui.New()
				m.agentUI = ui
				updated, _ := m.Update(agentRunFinishedMsg{ui: ui, err: errors.New("boom")})
				return updated.(model)
			},
		},
		{
			name: "user cancel",
			end: func(m model) model {
				m.agentUI = agentui.New()
				updated, _ := m.handleCtrlC()
				return updated
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeModelSetupConfig(t)
			m := NewModelForLanguage("zh-CN")
			m.running = true
			m, _ = m.handleComposerSubmitValue("/model")
			m = tt.end(m)
			if m.modelSetup == nil || m.pendingInput != "" {
				t.Fatalf("pending setup was not consumed: %#v", m)
			}

		})
	}
}
