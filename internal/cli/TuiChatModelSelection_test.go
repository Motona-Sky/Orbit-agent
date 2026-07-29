package cli

import (
	"errors"
	"strings"
	"testing"

	"looporbit/internal/agentui"
	"looporbit/internal/config"
	"looporbit/internal/utils"

	tea "github.com/charmbracelet/bubbletea"
)

func writeModelSelectionConfig(t *testing.T) config.AppConfig {
	t.Helper()
	writeModelSetupConfig(t)
	appConfig := config.AppConfig{
		Language:        "zh-CN",
		Theme:           "dark",
		ThinkLevel:      "high",
		DefaultProvider: "openai",
		Providers: map[string]config.ProviderConfig{
			"openai": {
				ApiKey: "openai-key", BaseURL: "https://openai.example/v1", Model: "gpt-old", Type: "openai:completions",
			},
			"anthropic": {
				ApiKey: "anthropic-key", BaseURL: "https://anthropic.example", Model: "claude", Type: "anthropic:messages",
			},
		},
	}
	if err := config.SaveAppConfig(appConfig); err != nil {
		t.Fatal(err)
	}
	return appConfig
}

func preserveRuntimeProvider(t *testing.T) {
	t.Helper()
	oldAPIKey, oldBaseURL, oldModel, oldProvider := utils.ApiKey, utils.BaseUrl, utils.Model, utils.Provider
	t.Cleanup(func() {
		utils.ApiKey, utils.BaseUrl = oldAPIKey, oldBaseURL
		utils.Model, utils.Provider = oldModel, oldProvider
	})
}

func TestModelListResponsePopulatesOptionsAndSelectionSavesDefaultModel(t *testing.T) {
	writeModelSelectionConfig(t)
	preserveRuntimeProvider(t)

	m := NewModelForLanguage("zh-CN")
	m, _ = m.handleSlashMessageSubmit("/model")
	m.setupScreenReady = true
	updated, _ := m.Update(modelListLoadedMsg{models: []string{"gpt-new", "gpt-other"}})
	m = updated.(model)
	if len(m.modelSetup.options) != 3 || !m.modelSetup.options[2].Custom {
		t.Fatalf("options = %#v", m.modelSetup.options)
	}

	updated, exitCmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.modelSetup != nil {
		t.Fatal("model setup remained active after confirmation")
	}
	assertCommandMessageType(t, exitCmd, tea.ExitAltScreen())

	saved, err := config.LoadAppConfig()
	if err != nil {
		t.Fatal(err)
	}
	if saved.DefaultProvider != "openai" || saved.Providers["openai"].Model != "gpt-new" || saved.Providers["anthropic"].Model != "claude" {
		t.Fatalf("saved config = %#v", saved)
	}
	if utils.Model != "gpt-new" || utils.ApiKey != "openai-key" {
		t.Fatal("runtime provider was not reloaded")
	}
}

func TestCustomModelInputSavesTrimmedValue(t *testing.T) {
	writeModelSelectionConfig(t)
	preserveRuntimeProvider(t)
	m := NewModelForLanguage("zh-CN")
	m, _ = m.handleSlashMessageSubmit("/model")
	m.setupScreenReady = true
	updated, _ := m.Update(modelListLoadedMsg{})
	m = updated.(model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	m.modelSetup.modelInput.SetValue("  custom-model  ")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)

	saved, err := config.LoadDefaultProviderConfig()
	if err != nil {
		t.Fatal(err)
	}
	if saved.Model != "custom-model" {
		t.Fatalf("saved model = %q", saved.Model)
	}
}

func TestModelListFailureReturnsToChatWithError(t *testing.T) {
	writeModelSelectionConfig(t)
	m := NewModelForLanguage("zh-CN")
	m, _ = m.handleSlashMessageSubmit("/model")
	m.setupScreenReady = true
	updated, exitCmd := m.Update(modelListLoadedMsg{err: errors.New("offline")})
	m = updated.(model)
	if m.modelSetup != nil || exitCmd == nil {
		t.Fatalf("model setup = %#v, cmd=%v", m.modelSetup, exitCmd)
	}
	if !strings.Contains(m.transcript[len(m.transcript)-1].content, "offline") {
		t.Fatalf("transcript = %#v", m.transcript)
	}
}

func TestStaleModelListResponseDoesNotAffectReopenedSetup(t *testing.T) {
	writeModelSelectionConfig(t)
	m := NewModelForLanguage("zh-CN")
	m, _ = m.handleSlashMessageSubmit("/model")
	firstRequestID := m.modelSetup.requestID
	m.setupScreenReady = true
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(model)
	m, _ = m.handleSlashMessageSubmit("/model")
	secondRequestID := m.modelSetup.requestID
	m.setupScreenReady = true

	updated, cmd := m.Update(modelListLoadedMsg{requestID: firstRequestID, err: errors.New("stale")})
	m = updated.(model)
	if cmd != nil || m.modelSetup == nil || m.modelSetup.requestID != secondRequestID {
		t.Fatalf("stale response changed setup: %#v cmd=%v", m.modelSetup, cmd)
	}
}

func TestEmbeddedModelSelectionCancelDoesNotSave(t *testing.T) {
	before := writeModelSelectionConfig(t)
	m := NewModelForLanguage("zh-CN")
	m, _ = m.handleSlashMessageSubmit("/model")
	m.setupScreenReady = true
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(model)
	if m.modelSetup != nil {
		t.Fatal("model setup remained active after cancel")
	}
	after, err := config.LoadAppConfig()
	if err != nil {
		t.Fatal(err)
	}
	if after.Providers["openai"].Model != before.Providers["openai"].Model {
		t.Fatal("config changed after cancel")
	}
}

func TestRunningProviderCommandUsesPendingInputAndOpensAfterResult(t *testing.T) {
	writeModelSelectionConfig(t)
	m := NewModelForLanguage("zh-CN")
	m.running = true
	m, _ = m.handleComposerSubmitValue("/provider")
	m, secondCmd := m.handleComposerSubmitValue("/model")
	if m.pendingInput != "/provider" || secondCmd != nil {
		t.Fatalf("pending state = %#v", m)
	}
	updated, _ := m.Update(agentui.ResultEvent{Text: "done"})
	m = updated.(model)
	if m.providerSetup == nil || m.modelSetup != nil || m.pendingInput != "" {
		t.Fatalf("pending provider setup was not opened: %#v", m)
	}
}
