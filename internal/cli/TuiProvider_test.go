package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"orbit/internal/config"
	"orbit/internal/style"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestProviderSetupStartsWithProviderSelection(t *testing.T) {
	m := initialConfigProviderForLanguageWithConfig("en", config.AppConfig{})
	if m.step != providerStepSelect {
		t.Fatalf("initial step = %v", m.step)
	}
}

func TestNewProviderShowsAuthenticationChoice(t *testing.T) {
	m := initialConfigProviderForLanguageWithConfig("en", config.AppConfig{})
	m.providerCursor = 0
	m.openProviderSetup()
	if m.step != providerStepAuth {
		t.Fatalf("new provider step = %v", m.step)
	}
	if got := m.View(); !strings.Contains(got, "API Key") || !strings.Contains(got, "OAuth") {
		t.Fatalf("authentication choices are missing: %q", got)
	}
}

func TestOAuthAuthenticationOpensCodexActionsDirectly(t *testing.T) {
	m := initialConfigProviderForLanguageWithConfig("en", config.AppConfig{})
	m.providerCursor = 0
	m.openProviderSetup()
	m.authCursor = 1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(ConfigProviderModel)
	if got.step != providerStepCodexAuth {
		t.Fatalf("OAuth authentication step = %v", got.step)
	}
	view := got.View()
	if !strings.Contains(view, got.messages.CodexLoginOption) {
		t.Fatalf("OAuth login action is missing: %q", view)
	}
	if len(got.codexAuthOptions) == 2 && !strings.Contains(view, got.messages.CodexImportOption) {
		t.Fatalf("Codex import action is missing: %q", view)
	}
}

func TestCodexProviderOptionOpensOAuthFlowDirectly(t *testing.T) {
	m := initialConfigProviderForLanguageWithConfig("en", config.AppConfig{})
	found := false
	for index, option := range m.provider {
		if option.Key == "codex" {
			found = true
			m.providerCursor = index
			if option.Label != "Codex Auth" {
				t.Fatalf("Codex provider label = %q", option.Label)
			}
			break
		}
	}
	if !found {
		t.Fatal("Codex Auth provider option is missing")
	}
	m.openProviderSetup()
	if m.step != providerStepCodexAuth {
		t.Fatalf("Codex Auth provider step = %v", m.step)
	}
}

func TestConfiguredAPIKeyProviderSkipsAuthenticationChoice(t *testing.T) {
	m := initialConfigProviderForLanguageWithConfig("en", config.AppConfig{Providers: map[string]config.ProviderConfig{
		"openai": {Auth: "apikey"},
	}})
	m.providerCursor = 0
	m.openProviderSetup()
	if m.step != providerStepName {
		t.Fatalf("configured provider step = %v", m.step)
	}
}

func TestConfiguredOAuthProviderOpensAccountDetails(t *testing.T) {
	m := initialConfigProviderForLanguageWithConfig("en", config.AppConfig{Providers: map[string]config.ProviderConfig{
		"work-codex": {
			Auth:        "codex",
			Type:        "oauth:codex",
			AccessToken: "token",
			AccountID:   "account-123",
			User: &config.OauthUser{
				User:      "user@example.com",
				Workspace: "workspace-123",
			},
		},
	}})
	for index, option := range m.provider {
		if option.Key == "work-codex" {
			m.providerCursor = index
			break
		}
	}
	m.openProviderSetup()
	if m.step != providerStepOAuthDetails {
		t.Fatalf("configured OAuth provider step = %v", m.step)
	}
	if m.codexConfig.Auth != "codex" || m.codexConfig.AccessToken != "token" {
		t.Fatalf("configured OAuth provider = %#v", m.codexConfig)
	}
	view := m.View()
	if !strings.Contains(view, "account-123") || !strings.Contains(view, "user@example.com") || !strings.Contains(view, "workspace-123") || !strings.Contains(view, m.messages.OAuthSelectProvider) || !strings.Contains(view, m.messages.OAuthDeleteProvider) {
		t.Fatalf("OAuth account details are missing: %q", view)
	}
	if strings.Contains(view, m.messages.APIKeySubtitle) {
		t.Fatalf("configured OAuth provider opened API key view: %q", view)
	}
}

func TestOAuthDetailsSelectsProvider(t *testing.T) {
	m := initialConfigProviderForLanguageWithConfig("en", config.AppConfig{})
	m.step = providerStepOAuthDetails
	m.SelectedProvider = "work-codex"
	m.codexConfig = config.ProviderConfig{Auth: "codex", AccountID: "account-123"}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(ConfigProviderModel)
	if !got.Confirmed || cmd == nil {
		t.Fatal("OAuth provider selection did not confirm")
	}
	selected := providerConfigFromModel(got)
	if selected.Name != "work-codex" || selected.Auth != "codex" {
		t.Fatalf("selected OAuth provider = %#v", selected)
	}
}

func TestOAuthDetailsReauthenticateOpensCodexActions(t *testing.T) {
	m := initialConfigProviderForLanguageWithConfig("en", config.AppConfig{})
	m.step = providerStepOAuthDetails
	m.SelectedProvider = "work-codex"
	m.codexConfig = config.ProviderConfig{Auth: "codex", AccountID: "account-123"}
	m.oauthDetailCursor = 1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(ConfigProviderModel)
	if got.step != providerStepCodexAuth {
		t.Fatalf("OAuth reauthentication step = %v", got.step)
	}
}

func TestOAuthDetailsDeleteRemovesProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.AppConfigFileName)
	t.Setenv(config.AppConfigPathEnv, path)
	if err := config.SaveAppConfig(config.AppConfig{
		DefaultProvider: "work-codex",
		Providers: map[string]config.ProviderConfig{
			"work-codex": {Auth: "codex", AccountID: "account-123"},
		},
	}); err != nil {
		t.Fatalf("SaveAppConfig() error = %v", err)
	}
	m := initialConfigProviderForLanguageWithConfig("en", config.AppConfig{})
	m.step = providerStepOAuthDetails
	m.SelectedProvider = "work-codex"
	m.oauthDetailCursor = 2
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("OAuth delete did not return a command")
	}
	updated, quitCmd := updated.(ConfigProviderModel).Update(cmd())
	got := updated.(ConfigProviderModel)
	if got.Confirmed || quitCmd == nil {
		t.Fatal("OAuth delete did not finish provider setup")
	}
	appConfig, err := config.LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}
	if _, ok := appConfig.Providers["work-codex"]; ok {
		t.Fatal("OAuth provider was not deleted")
	}
	if appConfig.DefaultProvider != "" {
		t.Fatalf("default provider = %q", appConfig.DefaultProvider)
	}
}

func TestCodexAuthMessageRequiresProviderName(t *testing.T) {
	m := initialConfigProviderForLanguageWithConfig("en", config.AppConfig{})
	updated, cmd := m.Update(codexAuthMsg{config: config.ProviderConfig{
		Name:        "codex",
		Auth:        "codex",
		Type:        "oauth:codex",
		AccessToken: "token",
	}})
	got := updated.(ConfigProviderModel)
	if got.Confirmed || cmd != nil {
		t.Fatal("successful Codex authentication skipped provider naming")
	}
	if got.step != providerStepCodexName {
		t.Fatalf("step after Codex authentication = %v", got.step)
	}
	if got.codexNameInput.Value() != "" {
		t.Fatalf("new Codex provider name = %q", got.codexNameInput.Value())
	}
}

func TestCodexProviderNameCannotUsePresetLabel(t *testing.T) {
	m := initialConfigProviderForLanguageWithConfig("en", config.AppConfig{})
	m.step = providerStepCodexName
	m.codexConfig = config.ProviderConfig{Auth: "codex", Type: "oauth:codex", AccessToken: "token"}
	m.codexNameInput.SetValue("Codex Auth")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(ConfigProviderModel)
	if got.Confirmed || cmd != nil {
		t.Fatal("reserved Codex Auth name was accepted")
	}
	if got.codexNameError == "" {
		t.Fatal("reserved Codex Auth name did not show an error")
	}
}

func TestCodexProviderNameConfirmsOAuthConfig(t *testing.T) {
	m := initialConfigProviderForLanguageWithConfig("en", config.AppConfig{})
	m.step = providerStepCodexName
	m.codexConfig = config.ProviderConfig{Auth: "codex", Type: "oauth:codex", AccessToken: "token"}
	m.codexNameInput.SetValue("work-codex")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(ConfigProviderModel)
	if !got.Confirmed || cmd == nil {
		t.Fatal("valid Codex provider name did not confirm setup")
	}
	selected := providerConfigFromModel(got)
	if selected.Name != "work-codex" || selected.Auth != "codex" || selected.AccessToken != "token" {
		t.Fatalf("Codex provider config = %#v", selected)
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("Codex naming command = %T", cmd())
	}
}

func TestConfiguredProviderLoadsItsOwnFields(t *testing.T) {
	appConfig := config.AppConfig{Providers: map[string]config.ProviderConfig{
		"openai": {
			ApiKey:  "openai-key",
			BaseURL: "https://openai.example/v1",
			Model:   "gpt-test",
			Type:    "openai:completions",
		},
	}}
	m := initialConfigProviderForLanguageWithConfig("en", appConfig)
	m.providerCursor = 0
	m.openProviderNameStep()
	m.openBaseURLStep()

	if got := m.baseURLInput.Value(); got != "https://openai.example/v1" {
		t.Fatalf("base URL = %q", got)
	}
	if got := m.apiKeyInput.Value(); got != "openai-key" {
		t.Fatalf("API key = %q", got)
	}
	if got := m.modelInput.Value(); got != "gpt-test" {
		t.Fatalf("model = %q", got)
	}
}

func TestOpenAICompatibleTypeMatchesAdapter(t *testing.T) {
	if got := defaultProviderType("openai"); got != "openai:completions" {
		t.Fatalf("OpenAI provider type = %q", got)
	}
}

func TestProviderTypeOptionsIncludeOpenAIResponses(t *testing.T) {
	m := initialConfigProviderForLanguageWithConfig("en", config.AppConfig{})
	option, ok := providerTypeOptionForType(m.typeOptions, "openai:responses")
	if !ok {
		t.Fatal("openai:responses provider type is missing")
	}
	if option.Label != "openai-responses" {
		t.Fatalf("OpenAI Responses label = %q", option.Label)
	}
}

func TestProviderSelectionPaginatesAroundCursor(t *testing.T) {
	providers := make(map[string]config.ProviderConfig)
	for index := 0; index < 12; index++ {
		name := fmt.Sprintf("provider-%02d", index)
		providers[name] = config.ProviderConfig{Auth: "apikey"}
	}
	m := initialConfigProviderForLanguageWithConfig("en", config.AppConfig{Providers: providers})
	m.terminalWidth, m.terminalHeight = 80, 24
	m.providerCursor = len(m.provider) - 2
	width, height := m.viewport()
	options, cursor, offset := m.visibleProviderOptions(width, height)
	if len(options) >= len(m.provider) {
		t.Fatalf("visible options were not paginated: %d", len(options))
	}
	if cursor < 0 || cursor >= len(options) {
		t.Fatalf("visible cursor = %d, options = %d", cursor, len(options))
	}
	if offset == 0 {
		t.Fatal("pagination offset was not advanced")
	}
	selectedLabel := m.providerLabels()[m.providerCursor]
	if options[cursor] != selectedLabel {
		t.Fatalf("visible selected option = %q, want %q", options[cursor], selectedLabel)
	}
	view := m.View()
	if !strings.Contains(view, selectedLabel) {
		t.Fatalf("selected provider is not visible: %q", view)
	}
}

func TestProviderSelectionUsesConfiguredStyle(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.StyleConfigFileName)
	t.Setenv(config.StyleConfigPathEnv, path)
	styleConfig := style.DefaultStyleConfig()
	styleConfig.Palette.Accent = "#224466"
	if err := config.SaveStyleConfig(styleConfig); err != nil {
		t.Fatalf("SaveStyleConfig() error = %v", err)
	}
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })

	m := initialConfigProviderForLanguageWithConfig("en", config.AppConfig{})
	m.terminalWidth, m.terminalHeight = 80, 24
	got := m.View()
	if !strings.Contains(got, "38;2;34;68;102") {
		t.Fatalf("provider selection does not contain configured accent: %q", got)
	}
	if strings.Contains(got, "38;2;18;220;232") {
		t.Fatalf("provider selection still contains default dark accent: %q", got)
	}
}
