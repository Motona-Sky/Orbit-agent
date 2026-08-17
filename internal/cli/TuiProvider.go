package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"orbit/internal/config"
	"orbit/internal/i18n"
	"orbit/internal/oauth"
	"orbit/internal/style"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type providerStep int

const (
	providerStepAuth providerStep = iota
	providerStepCodexAuth
	providerStepCodexName
	providerStepOAuthDetails
	providerStepSelect
	providerStepName
	providerStepBaseURL
	providerStepAPIKey
	providerStepModel
	providerStepType
	providerStepConfirm
)

type codexAuthStartedMsg struct {
	session *oauth.CodexLoginSession
	err     error
}

type codexAuthMsg struct {
	config config.ProviderConfig
	err    error
}

type deleteOAuthProviderMsg struct {
	err error
}

type ConfigProviderModel struct {
	authCursor        int
	codexAuthCursor   int
	codexAuthOptions  []string
	codexAuthLoading  bool
	codexAuthURL      string
	codexAuthError    string
	codexConfig       config.ProviderConfig
	codexNameInput    textinput.Model
	codexNameError    string
	oauthDetailCursor int
	oauthDeleteError  string
	provider          []providerOption
	providerCursor    int
	providerSelect    map[int]string
	terminalWidth     int
	terminalHeight    int
	providerNameInput textinput.Model
	baseURLInput      textinput.Model
	apiKeyInput       textinput.Model
	modelInput        textinput.Model
	typeOptions       []providerTypeOption
	typeCursor        int
	step              providerStep
	messages          i18n.ProviderSetupMessages
	styleConfig       config.StyleConfig
	SelectedKey       int
	SelectedProvider  string
	SelectedBaseURL   string
	SelectedAPIKey    string
	SelectedModel     string
	SelectedType      string
	Confirmed         bool
}

type providerOption struct {
	Key        string
	Label      string
	Config     config.ProviderConfig
	Configured bool
}

type providerTypeOption struct {
	Label string
	Type  string
}

const (
	providerTypeOpenAICompatible  = "openai:completions"
	providerTypeOpenAIResponses   = "openai:responses"
	providerTypeAnthropicMessages = "anthropic:messages"
)

func initialConfigProvider() ConfigProviderModel {
	return initialConfigProviderForLanguage(i18n.DefaultLanguage)
}

func initialConfigProviderForLanguage(languageCode string) ConfigProviderModel {
	return initialConfigProviderForLanguageWithConfig(languageCode, config.AppConfig{})
}

func initialConfigProviderForLanguageWithConfig(languageCode string, appConfig config.AppConfig) ConfigProviderModel {
	messages := i18n.For(languageCode).Messages.ProviderSetup
	providerNameInput := textinput.New()
	providerNameInput.Placeholder = messages.NamePlaceholder
	providerNameInput.CharLimit = 128

	codexNameInput := textinput.New()
	codexNameInput.Placeholder = messages.CodexNamePlaceholder
	codexNameInput.CharLimit = 128

	baseURLInput := textinput.New()
	baseURLInput.Placeholder = messages.BaseURLPlaceholder
	baseURLInput.CharLimit = 512

	apiKeyInput := textinput.New()
	apiKeyInput.Placeholder = messages.APIKeyPlaceholder
	apiKeyInput.CharLimit = 256

	modelInput := textinput.New()
	modelInput.Placeholder = messages.ModelPlaceholder
	modelInput.CharLimit = 256

	providers := providerOptions(messages, appConfig.Providers)
	providerSelect := make(map[int]string, len(providers))
	for index, provider := range providers {
		providerSelect[index] = provider.Key
	}

	return ConfigProviderModel{
		codexAuthOptions:  codexAuthOptions(messages),
		provider:          providers,
		providerCursor:    0,
		providerSelect:    providerSelect,
		providerNameInput: providerNameInput,
		codexNameInput:    codexNameInput,
		baseURLInput:      baseURLInput,
		apiKeyInput:       apiKeyInput,
		modelInput:        modelInput,
		typeOptions:       providerTypeOptions(messages),
		typeCursor:        0,
		step:              providerStepSelect,
		messages:          messages,
		styleConfig:       loadStyleConfigOrDefault(),
	}
}

func codexAuthOptions(messages i18n.ProviderSetupMessages) []string {
	options := make([]string, 0, 2)
	if oauth.CheckCodexAuth() {
		options = append(options, messages.CodexImportOption)
	}
	return append(options, messages.CodexLoginOption)
}

func providerTypeOptions(messages i18n.ProviderSetupMessages) []providerTypeOption {
	return []providerTypeOption{
		{Label: messages.OpenAITypeOption, Type: providerTypeOpenAICompatible},
		{Label: messages.OpenAIResponseTypeOption, Type: providerTypeOpenAIResponses},
		{Label: messages.AnthropicTypeOption, Type: providerTypeAnthropicMessages},
	}
}

func providerOptions(messages i18n.ProviderSetupMessages, savedProviders map[string]config.ProviderConfig) []providerOption {
	builtIn := []providerOption{
		{Key: "openai", Label: messages.OpenAIOption},
		{Key: "anthropic", Label: messages.AnthropicOption},
		{Key: "gemini", Label: messages.GeminiOption},
		{Key: "ollama", Label: messages.OllamaOption},
		{Key: "codex", Label: messages.CodexProviderOption},
	}
	seen := make(map[string]bool, len(builtIn))
	for index := range builtIn {
		option := &builtIn[index]
		seen[option.Key] = true
		if saved, ok := savedProviders[option.Key]; ok {
			option.Config = saved
			option.Configured = true
		}
	}

	customKeys := make([]string, 0, len(savedProviders))
	for key := range savedProviders {
		if !seen[key] && key != "custom" {
			customKeys = append(customKeys, key)
		}
	}
	sort.Strings(customKeys)
	for _, key := range customKeys {
		builtIn = append(builtIn, providerOption{
			Key:        key,
			Label:      key,
			Config:     savedProviders[key],
			Configured: true,
		})
	}

	customOption := providerOption{Key: "custom", Label: messages.CustomOption}
	if saved, ok := savedProviders["custom"]; ok {
		customOption.Config = saved
		customOption.Configured = true
	}
	builtIn = append(builtIn, customOption)

	return builtIn
}

func (m ConfigProviderModel) Init() tea.Cmd {
	return nil
}
func (m ConfigProviderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case codexAuthStartedMsg:
		if msg.err != nil {
			m.codexAuthLoading = false
			m.codexAuthError = msg.err.Error()
			return m, nil
		}
		m.codexAuthURL = msg.session.AuthorizationURL
		return m, m.waitForCodexAuthCmd(msg.session)
	case codexAuthMsg:
		m.codexAuthLoading = false
		m.codexAuthURL = ""
		if msg.err != nil {
			m.codexAuthError = msg.err.Error()
			return m, nil
		}
		m.codexConfig = msg.config
		m.codexNameError = ""
		m.codexNameInput.SetValue(m.SelectedProvider)
		m.codexNameInput.CursorEnd()
		m.step = providerStepCodexName
		return m, nil
	case deleteOAuthProviderMsg:
		if msg.err != nil {
			m.oauthDeleteError = m.messages.OAuthDeleteError + ": " + msg.err.Error()
			return m, nil
		}
		m.Confirmed = false
		return m, tea.Quit
	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
		m.terminalHeight = msg.Height
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.step == providerStepAuth {
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "up", "k":
				m.authCursor = moveOptionCursor(m.authCursor, -1, 2)
				return m, nil
			case "down", "j":
				m.authCursor = moveOptionCursor(m.authCursor, 1, 2)
				return m, nil
			case "enter", "space", " ":
				if m.authCursor == 0 {
					m.openProviderNameStep()
				} else {
					m.SelectedProvider = ""
					m.step = providerStepCodexAuth
				}
				return m, nil
			}
		}
		if m.step == providerStepCodexAuth {
			if m.codexAuthLoading {
				return m, nil
			}
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "up", "k":
				m.codexAuthCursor = moveOptionCursor(m.codexAuthCursor, -1, len(m.codexAuthOptions))
				return m, nil
			case "down", "j":
				m.codexAuthCursor = moveOptionCursor(m.codexAuthCursor, 1, len(m.codexAuthOptions))
				return m, nil
			case "enter", "space", " ":
				m.codexAuthLoading = true
				m.codexAuthError = ""
				return m, m.codexAuthCmd()
			}
		}
		if m.step == providerStepOAuthDetails {
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "up", "k":
				m.oauthDetailCursor = moveOptionCursor(m.oauthDetailCursor, -1, 3)
				return m, nil
			case "down", "j":
				m.oauthDetailCursor = moveOptionCursor(m.oauthDetailCursor, 1, 3)
				return m, nil
			case "enter", "space", " ":
				switch m.oauthDetailCursor {
				case 0:
					m.codexConfig.Name = m.SelectedProvider
					m.Confirmed = true
					return m, tea.Quit
				case 1:
					m.step = providerStepCodexAuth
					return m, nil
				default:
					return m, m.deleteOAuthProviderCmd()
				}
			}
		}
		if m.step == providerStepCodexName {
			if msg.String() == "enter" {
				if m.confirmCodexName() {
					return m, tea.Quit
				}
				return m, nil
			}
			return m.updateCodexName(msg)
		}
		if m.step == providerStepSelect {
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "up", "k":
				m.moveCursor(-1)
				return m, nil
			case "down", "j":
				m.moveCursor(1)
				return m, nil
			case "enter", "space", " ":
				m.openProviderSetup()
				return m, nil
			}
		}

		if m.step == providerStepName {
			switch msg.String() {
			case "enter":
				m.openBaseURLStep()
				return m, nil
			}
			return m.updateProviderName(msg)
		}

		if m.step == providerStepBaseURL {
			switch msg.String() {
			case "enter":
				m.openAPIKeyStep()
				return m, nil
			}
			return m.updateBaseURL(msg)
		}

		if m.step == providerStepAPIKey {
			if msg.String() == "enter" {
				m.openModelStep()
				return m, nil
			}
			return m.updateAPIKey(msg)
		}

		if m.step == providerStepModel {
			if msg.String() == "enter" {
				m.openTypeStep()
				return m, nil
			}
			return m.updateModel(msg)
		}

		if m.step == providerStepType {
			switch msg.String() {
			case "enter":
				m.openConfirmStep()
				return m, nil
			case "up", "k":
				m.moveTypeCursor(-1)
				return m, nil
			case "down", "j":
				m.moveTypeCursor(1)
				return m, nil
			}
		}

		if m.step == providerStepConfirm {
			if msg.String() == "enter" {
				m.confirmSelection()
				return m, tea.Quit
			}
		}
	}

	if m.step == providerStepCodexName {
		return m.updateCodexName(msg)
	}
	if m.step == providerStepBaseURL {
		return m.updateBaseURL(msg)
	}
	if m.step == providerStepName {
		return m.updateProviderName(msg)
	}
	if m.step == providerStepAPIKey {
		return m.updateAPIKey(msg)
	}
	if m.step == providerStepModel {
		return m.updateModel(msg)
	}
	return m, nil
}

func (m ConfigProviderModel) View() string {
	if m.step == providerStepAuth {
		return m.authView()
	}
	if m.step == providerStepCodexAuth {
		return m.codexAuthView()
	}
	if m.step == providerStepOAuthDetails {
		return m.oauthDetailsView()
	}
	if m.step == providerStepCodexName {
		return m.codexNameView()
	}
	if m.step == providerStepName {
		return m.providerNameView()
	}
	if m.step == providerStepBaseURL {
		return m.baseURLView()
	}
	if m.step == providerStepAPIKey {
		return m.apiKeyView()
	}
	if m.step == providerStepModel {
		return m.modelView()
	}
	if m.step == providerStepType {
		return m.typeView()
	}
	if m.step == providerStepConfirm {
		return m.confirmView()
	}

	width, height := m.viewport()
	options, visibleCursor, sequenceOffset := m.visibleProviderOptions(width, height)
	return style.RenderOrbitalMenuView(style.OrbitalMenuView{
		Copy:           providerSetupCopy(m.messages),
		Options:        options,
		Cursor:         visibleCursor,
		SequenceOffset: sequenceOffset,
		StyleConfig:    m.styleConfig,
		ViewportWidth:  width,
		ViewportHeight: height,
	})
}

func (m ConfigProviderModel) authView() string {
	return m.renderMenu(m.messages.AuthSubtitle, m.messages.AuthDescription, []string{
		m.messages.APIKeyAuthOption,
		m.messages.OAuthAuthOption,
	}, m.authCursor)
}

func (m ConfigProviderModel) codexAuthView() string {
	description := m.messages.CodexAuthDescription
	if m.codexAuthLoading {
		description = m.messages.CodexAuthLoading
		if m.codexAuthURL != "" {
			description += "\n\n" + style.TerminalHyperlink(m.codexAuthURL, m.codexAuthURL)
		}
	} else if m.codexAuthError != "" {
		description = m.codexAuthError
	}
	return m.renderMenu(m.messages.CodexAuthSubtitle, description, m.codexAuthOptions, m.codexAuthCursor)
}

func (m ConfigProviderModel) renderMenu(heading, description string, options []string, cursor int) string {
	width, height := m.viewport()
	return style.RenderOrbitalMenuView(style.OrbitalMenuView{
		Copy: style.OrbitalMenuCopy{
			Title:        m.messages.Title,
			Heading:      heading,
			Subtitle:     description,
			MoveShortcut: m.messages.MoveShortcut,
			MoveAction:   m.messages.MoveAction,
			SelectKey:    m.messages.SelectKey,
			SelectAction: m.messages.SelectAction,
			QuitKey:      m.messages.QuitKey,
			QuitAction:   m.messages.QuitAction,
		},
		Options:        options,
		Cursor:         cursor,
		StyleConfig:    m.styleConfig,
		ViewportWidth:  width,
		ViewportHeight: height,
	})
}

func (m ConfigProviderModel) oauthDetailsView() string {
	description := m.messages.OAuthDetailsDescription
	if m.oauthDeleteError != "" {
		description = m.oauthDeleteError
	}
	accountID := m.codexConfig.AccountID
	if accountID == "" {
		accountID = "-"
	}
	user := "-"
	workspace := "-"
	if m.codexConfig.User != nil {
		if m.codexConfig.User.User != "" {
			user = m.codexConfig.User.User
		}
		if m.codexConfig.User.Workspace != "" {
			workspace = m.codexConfig.User.Workspace
		}
	}
	return m.renderMenu(m.messages.OAuthDetailsSubtitle, description+"\n\n"+
		providerFormLine(m.messages.ProviderLabel, m.SelectedProvider)+"\n"+
		providerFormLine(m.messages.OAuthAccountLabel, accountID)+"\n"+
		providerFormLine(m.messages.OAuthUserLabel, user)+"\n"+
		providerFormLine(m.messages.OAuthWorkspaceLabel, workspace), []string{
		m.messages.OAuthSelectProvider,
		m.messages.OAuthReauthenticate,
		m.messages.OAuthDeleteProvider,
	}, m.oauthDetailCursor)
}

func (m ConfigProviderModel) codexNameView() string {
	codexNameInput := m.codexNameInput
	codexNameInput.Focus()
	description := m.messages.CodexNameDescription
	if m.codexNameError != "" {
		description = m.codexNameError
	}
	return m.renderForm(m.messages.CodexNameSubtitle, []string{
		description,
		"",
		m.messages.NameLabel,
		codexNameInput.View(),
	}, m.messages.NameHint)
}

func (m ConfigProviderModel) providerNameView() string {
	providerNameInput := m.providerNameInput
	providerNameInput.Focus()

	return m.renderForm(m.messages.NameSubtitle, []string{
		providerFormLine(m.messages.ProviderLabel, m.selectedProviderLabel()),
		"",
		m.messages.NameLabel,
		providerNameInput.View(),
	}, m.messages.NameHint)
}

func (m ConfigProviderModel) baseURLView() string {
	baseURLInput := m.baseURLInput
	baseURLInput.Focus()

	return m.renderForm(m.messages.BaseURLSubtitle, []string{
		providerFormLine(m.messages.ProviderLabel, m.selectedProviderLabel()),
		"",
		m.messages.BaseURLLabel,
		baseURLInput.View(),
	}, m.messages.BaseURLHint)
}

func (m ConfigProviderModel) apiKeyView() string {
	apiKeyInput := m.apiKeyInput
	apiKeyInput.Focus()

	return m.renderForm(m.messages.APIKeySubtitle, []string{
		providerFormLine(m.messages.ProviderLabel, m.selectedProviderLabel()),
		providerFormLine(m.messages.BaseURLLabel, m.SelectedBaseURL),
		"",
		m.messages.APIKeyLabel,
		apiKeyInput.View(),
	}, m.messages.APIKeyHint)
}

func (m ConfigProviderModel) modelView() string {
	modelInput := m.modelInput
	modelInput.Focus()

	return m.renderForm(m.messages.ModelSubtitle, []string{
		providerFormLine(m.messages.ProviderLabel, m.selectedProviderLabel()),
		providerFormLine(m.messages.BaseURLLabel, m.SelectedBaseURL),
		providerFormLine(m.messages.APIKeyLabel, m.SelectedAPIKey),
		"",
		m.messages.ModelLabel,
		modelInput.View(),
	}, m.messages.ModelHint)
}

func (m ConfigProviderModel) typeView() string {
	width, height := m.viewport()
	lines := m.fullTypeViewLines()
	if m.shouldUseCompactTypeView(width, height, len(lines)) {
		lines = m.compactTypeViewLines()
	}

	return m.renderForm(m.messages.TypeSubtitle, lines, m.messages.TypeHint)
}

func (m ConfigProviderModel) fullTypeViewLines() []string {
	lines := []string{
		providerFormLine(m.messages.ProviderLabel, m.selectedProviderLabel()),
		providerFormLine(m.messages.BaseURLLabel, m.SelectedBaseURL),
		providerFormLine(m.messages.APIKeyLabel, m.SelectedAPIKey),
		providerFormLine(m.messages.ModelLabel, m.SelectedModel),
		"",
		m.messages.TypeLabel,
	}
	return append(lines, m.typeLabels()...)
}

func (m ConfigProviderModel) compactTypeViewLines() []string {
	lines := []string{
		providerFormLine(m.messages.ProviderLabel, m.selectedProviderLabel()),
		"",
		m.messages.TypeLabel,
	}
	return append(lines, m.typeLabels()...)
}

func (m ConfigProviderModel) shouldUseCompactTypeView(width, height, fullLineCount int) bool {
	logoHeight := lipgloss.Height(style.RenderOrbitalLogoForViewportWithStyle(m.messages.Title, width, m.styleConfig))
	availablePanelInnerLines := height - logoHeight - 4
	requiredLinesBeforeFooter := 5 + fullLineCount
	return availablePanelInnerLines < requiredLinesBeforeFooter
}

func (m ConfigProviderModel) confirmView() string {
	return m.renderForm(m.messages.ConfirmSubtitle, []string{
		providerFormLine(m.messages.ProviderLabel, m.selectedProviderLabel()),
		providerFormLine(m.messages.BaseURLLabel, m.SelectedBaseURL),
		providerFormLine(m.messages.APIKeyLabel, m.SelectedAPIKey),
		providerFormLine(m.messages.ModelLabel, m.SelectedModel),
		providerFormLine(m.messages.TypeLabel, m.selectedTypeLabel()),
	}, m.messages.ConfirmHint)
}

func providerSetupCopy(messages i18n.ProviderSetupMessages) style.OrbitalMenuCopy {
	return style.OrbitalMenuCopy{
		Title:        messages.Title,
		Heading:      messages.SelectSubtitle,
		Subtitle:     messages.SelectDescription,
		MoveShortcut: messages.MoveShortcut,
		MoveAction:   messages.MoveAction,
		SelectKey:    messages.SelectKey,
		SelectAction: messages.SelectAction,
		QuitKey:      messages.QuitKey,
		QuitAction:   messages.QuitAction,
	}
}

func (m ConfigProviderModel) renderForm(heading string, lines []string, hint string) string {
	width, height := m.viewport()
	return style.RenderOrbitalFormView(style.OrbitalFormView{
		Copy: style.OrbitalFormCopy{
			Title:   m.messages.Title,
			Heading: heading,
			Hint:    hint,
		},
		Lines:          lines,
		StyleConfig:    m.styleConfig,
		ViewportWidth:  width,
		ViewportHeight: height,
	})
}

func (m ConfigProviderModel) visibleProviderOptions(width, height int) ([]string, int, int) {
	labels := m.providerLabels()
	if len(labels) == 0 {
		return labels, -1, 0
	}
	capacity := style.OrbitalMenuOptionCapacity(m.messages.Title, width, height, m.styleConfig)
	limit := minInt(len(labels), capacity)
	start := clampInt(m.providerCursor-limit/2, 0, maxInt(0, len(labels)-limit))
	end := minInt(len(labels), start+limit)
	return labels[start:end], m.providerCursor - start, start
}

func (m ConfigProviderModel) providerLabels() []string {
	labels := make([]string, 0, len(m.provider))
	for _, option := range m.provider {
		label := option.Label
		if option.Configured {
			label = lipgloss.NewStyle().Foreground(lipgloss.Color(m.styleConfig.Palette.Accent)).Render(label)
		}
		labels = append(labels, label)
	}
	return labels
}

func (m ConfigProviderModel) typeLabels() []string {
	labels := make([]string, 0, len(m.typeOptions))
	for index, option := range m.typeOptions {
		prefix := "  "
		if index == m.typeCursor {
			prefix = "> "
		}
		labels = append(labels, prefix+option.Label)
	}
	return labels
}

func (m ConfigProviderModel) selectedProviderLabel() string {
	key := m.SelectedProvider
	if key == "" && m.providerCursor >= 0 && m.providerCursor < len(m.provider) {
		key = m.provider[m.providerCursor].Key
	}
	for _, option := range m.provider {
		if option.Key == key {
			return option.Label
		}
	}
	return key
}

func (m ConfigProviderModel) selectedTypeLabel() string {
	providerType := m.SelectedType
	if providerType == "" {
		providerType = m.selectedProviderType()
	}
	if option, ok := providerTypeOptionForType(m.typeOptions, providerType); ok {
		return option.Label
	}
	if len(m.typeOptions) > 0 {
		return m.typeOptions[0].Label
	}
	return ""
}

func (m ConfigProviderModel) selectedProviderOption() (providerOption, bool) {
	if m.providerCursor < 0 || m.providerCursor >= len(m.provider) {
		return providerOption{}, false
	}
	return m.provider[m.providerCursor], true
}

func (m ConfigProviderModel) selectedProviderOptionBySelectedKey() (providerOption, bool) {
	if m.SelectedKey >= 0 && m.SelectedKey < len(m.provider) {
		return m.provider[m.SelectedKey], true
	}
	return providerOption{}, false
}

func (m ConfigProviderModel) selectedProviderType() string {
	providerType := defaultProviderType(m.SelectedProvider)
	if option, ok := m.selectedProviderOptionBySelectedKey(); ok {
		providerType = defaultProviderType(option.Key)
		if option.Config.Type != "" {
			providerType = option.Config.Type
		}
	}
	if _, ok := providerTypeOptionForType(m.typeOptions, providerType); ok {
		return providerType
	}
	return providerTypeOpenAICompatible
}

func providerTypeOptionForType(options []providerTypeOption, providerType string) (providerTypeOption, bool) {
	for _, option := range options {
		if option.Type == providerType {
			return option, true
		}
	}
	return providerTypeOption{}, false
}

func providerTypeCursorForType(options []providerTypeOption, providerType string) int {
	for index, option := range options {
		if option.Type == providerType {
			return index
		}
	}
	return 0
}

func providerFormLine(label, value string) string {
	return fmt.Sprintf("%s: %s", label, value)
}

func moveOptionCursor(cursor, offset, count int) int {
	if count == 0 {
		return 0
	}
	return (cursor + offset + count) % count
}

func (m ConfigProviderModel) deleteOAuthProviderCmd() tea.Cmd {
	providerName := m.SelectedProvider
	return func() tea.Msg {
		return deleteOAuthProviderMsg{err: config.DeleteProviderConfig(providerName)}
	}
}

func (m ConfigProviderModel) codexAuthCmd() tea.Cmd {
	importConfig := len(m.codexAuthOptions) == 2 && m.codexAuthCursor == 0
	return func() tea.Msg {
		if importConfig {
			providerConfig, err := oauth.GetConfigCodexAuth()
			return codexAuthMsg{config: providerConfig, err: err}
		}
		session, err := oauth.PrepareCodexLogin()
		return codexAuthStartedMsg{session: session, err: err}
	}
}

func (m ConfigProviderModel) waitForCodexAuthCmd(session *oauth.CodexLoginSession) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
		defer cancel()
		providerConfig, err := session.Config(ctx)
		return codexAuthMsg{config: providerConfig, err: err}
	}
}

func (m *ConfigProviderModel) moveCursor(offset int) {
	if len(m.provider) == 0 {
		return
	}
	m.providerCursor = (m.providerCursor + offset + len(m.provider)) % len(m.provider)
}

func (m *ConfigProviderModel) moveTypeCursor(offset int) {
	if len(m.typeOptions) == 0 {
		return
	}
	m.typeCursor = (m.typeCursor + offset + len(m.typeOptions)) % len(m.typeOptions)
	m.SelectedType = m.typeOptions[m.typeCursor].Type
}

func (m ConfigProviderModel) viewport() (int, int) {
	return orbitalViewport(m.terminalWidth, m.terminalHeight, m.styleConfig)
}

func (m *ConfigProviderModel) openProviderSetup() {
	option, ok := m.selectedProviderOption()
	if ok && option.Configured && option.Config.Auth == "codex" {
		m.SelectedKey = m.providerCursor
		m.SelectedProvider = option.Key
		m.codexConfig = option.Config
		m.codexConfig.Name = option.Key
		m.oauthDetailCursor = 0
		m.oauthDeleteError = ""
		m.step = providerStepOAuthDetails
		return
	}
	if ok && option.Key == "codex" {
		m.SelectedKey = m.providerCursor
		m.SelectedProvider = ""
		m.codexConfig = config.ProviderConfig{}
		m.step = providerStepCodexAuth
		return
	}
	if ok && option.Configured {
		m.openProviderNameStep()
		return
	}
	m.SelectedKey = m.providerCursor
	m.SelectedProvider = m.providerSelect[m.SelectedKey]
	m.step = providerStepAuth
}

func (m *ConfigProviderModel) openProviderNameStep() {
	m.SelectedKey = m.providerCursor
	option, ok := m.selectedProviderOption()
	providerName := m.providerSelect[m.SelectedKey]
	if ok {
		providerName = option.Key
	}
	defaultName := providerName
	if ok && option.Key == "custom" && !option.Configured {
		defaultName = ""
	}
	m.SelectedProvider = providerName
	m.providerNameInput.SetValue(defaultName)
	m.providerNameInput.CursorEnd()
	m.step = providerStepName
}

func (m *ConfigProviderModel) openBaseURLStep() {
	m.SelectedKey = m.providerCursor
	option, ok := m.selectedProviderOption()
	sourceProvider := m.providerSelect[m.SelectedKey]
	if ok {
		sourceProvider = option.Key
	}
	m.SelectedProvider = providerNameOrDefault(m.providerNameInput.Value(), sourceProvider)
	if ok {
		if option.Configured {
			m.baseURLInput.SetValue(option.Config.BaseURL)
			m.apiKeyInput.SetValue(option.Config.ApiKey)
			m.modelInput.SetValue(option.Config.Model)
		} else {
			m.baseURLInput.SetValue(defaultBaseURLForProvider(option.Key))
			m.apiKeyInput.SetValue("")
			m.modelInput.SetValue("")
		}
	} else {
		m.baseURLInput.SetValue(defaultBaseURLForProvider(sourceProvider))
		m.apiKeyInput.SetValue("")
		m.modelInput.SetValue("")
	}
	m.baseURLInput.CursorEnd()
	m.step = providerStepBaseURL
}

func providerNameOrDefault(value, fallback string) string {
	name := strings.TrimSpace(value)
	if name == "" {
		return fallback
	}
	return name
}

func (m *ConfigProviderModel) openAPIKeyStep() {
	m.SelectedBaseURL = m.baseURLInput.Value()
	m.step = providerStepAPIKey
}

func (m ConfigProviderModel) updateCodexName(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.codexNameError = ""
	m.codexNameInput.Focus()
	m.codexNameInput, cmd = m.codexNameInput.Update(msg)
	return m, cmd
}

func (m *ConfigProviderModel) confirmCodexName() bool {
	name := strings.TrimSpace(m.codexNameInput.Value())
	if name == "" {
		m.codexNameError = m.messages.CodexNameRequiredError
		return false
	}
	if strings.EqualFold(name, m.messages.CodexProviderOption) {
		m.codexNameError = m.messages.CodexNameReservedError
		return false
	}
	m.codexConfig.Name = name
	m.SelectedProvider = name
	m.Confirmed = true
	return true
}

func (m ConfigProviderModel) updateProviderName(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.providerNameInput.Focus()
	m.providerNameInput, cmd = m.providerNameInput.Update(msg)
	return m, cmd
}

func (m ConfigProviderModel) updateBaseURL(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.baseURLInput.Focus()
	m.baseURLInput, cmd = m.baseURLInput.Update(msg)
	return m, cmd
}

func (m ConfigProviderModel) updateAPIKey(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.apiKeyInput.Focus()
	m.apiKeyInput, cmd = m.apiKeyInput.Update(msg)
	return m, cmd
}

func (m ConfigProviderModel) updateModel(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.modelInput.Focus()
	m.modelInput, cmd = m.modelInput.Update(msg)
	return m, cmd
}

func (m *ConfigProviderModel) openModelStep() {
	m.SelectedAPIKey = m.apiKeyInput.Value()
	m.step = providerStepModel
}

func (m *ConfigProviderModel) openTypeStep() {
	m.SelectedModel = m.modelInput.Value()
	m.SelectedType = m.selectedProviderType()
	m.typeCursor = providerTypeCursorForType(m.typeOptions, m.SelectedType)
	m.step = providerStepType
}

func (m *ConfigProviderModel) openConfirmStep() {
	if m.SelectedModel == "" {
		m.SelectedModel = m.modelInput.Value()
	}
	if len(m.typeOptions) > 0 {
		m.SelectedType = m.typeOptions[m.typeCursor].Type
	}
	m.step = providerStepConfirm
}

func (m *ConfigProviderModel) confirmSelection() {
	m.SelectedKey = m.providerCursor
	if strings.TrimSpace(m.SelectedProvider) == "" {
		m.SelectedProvider = providerNameOrDefault(m.providerNameInput.Value(), m.providerSelect[m.SelectedKey])
	}
	m.Confirmed = true
}

func defaultBaseURLForProvider(provider string) string {
	switch provider {
	case "openai":
		return "https://api.openai.com/v1"
	case "anthropic":
		return "https://api.anthropic.com"
	case "gemini":
		return "https://generativelanguage.googleapis.com/v1beta"
	case "ollama":
		return "http://localhost:11434/v1"
	default:
		return ""
	}
}
func providerConfigFromModel(m ConfigProviderModel) config.ProviderConfig {
	if m.codexConfig.Auth == "codex" {
		return m.codexConfig
	}
	providerType := m.SelectedType
	if providerType == "" {
		providerType = m.selectedProviderType()
	}
	return config.ProviderConfig{
		Name:    m.SelectedProvider,
		ApiKey:  m.SelectedAPIKey,
		BaseURL: m.SelectedBaseURL,
		Model:   m.SelectedModel,
		Type:    providerType,
	}
}
func defaultProviderType(provider string) string {
	switch provider {
	case "openai":
		return providerTypeOpenAICompatible
	case "anthropic":
		return providerTypeAnthropicMessages
	case "ollama":
		return providerTypeOpenAICompatible
	default:
		return providerTypeOpenAICompatible
	}
}
