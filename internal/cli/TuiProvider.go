package cli

import (
	"fmt"
	"sort"
	"strings"

	"orbit/internal/config"
	"orbit/internal/i18n"
	"orbit/internal/style"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type providerStep int

const (
	providerStepSelect providerStep = iota
	providerStepName
	providerStepBaseURL
	providerStepAPIKey
	providerStepModel
	providerStepType
	providerStepConfirm
)

type ConfigProviderModel struct {
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
		provider:          providers,
		providerCursor:    0,
		providerSelect:    providerSelect,
		providerNameInput: providerNameInput,
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
	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
		m.terminalHeight = msg.Height
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
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
				m.openProviderNameStep()
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
	return style.RenderOrbitalMenuView(style.OrbitalMenuView{
		Copy:           providerSetupCopy(m.messages),
		Options:        m.providerLabels(),
		Cursor:         m.providerCursor,
		StyleConfig:    m.styleConfig,
		ViewportWidth:  width,
		ViewportHeight: height,
	})
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
