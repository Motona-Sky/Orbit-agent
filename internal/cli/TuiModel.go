package cli

import (
	"strings"

	"looporbit/internal/config"
	"looporbit/internal/i18n"
	"looporbit/internal/style"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type modelStep int

const (
	modelStepSelect modelStep = iota
	modelStepCustom
)

type modelOption struct {
	Model  string
	Custom bool
}

type ConfigModelModel struct {
	options        []modelOption
	cursor         int
	terminalWidth  int
	terminalHeight int
	currentModel   string
	modelInput     textinput.Model
	step           modelStep
	loading        bool
	requestID      uint64
	SelectedModel  string
	Confirmed      bool
	messages       i18n.ModelSetupMessages
	styleConfig    config.StyleConfig
}

func initialConfigModelForLanguage(languageCode, currentModel string) ConfigModelModel {
	messages := i18n.For(languageCode).Messages.ModelSetup
	modelInput := textinput.New()
	modelInput.Placeholder = messages.CustomPlaceholder
	modelInput.CharLimit = 256
	return ConfigModelModel{
		currentModel: currentModel,
		modelInput:   modelInput,
		loading:      true,
		messages:     messages,
		styleConfig:  loadStyleConfigOrDefault(),
	}
}

func (m *ConfigModelModel) setModels(models []string) {
	m.options = make([]modelOption, 0, len(models)+1)
	m.cursor = 0
	for index, modelName := range models {
		m.options = append(m.options, modelOption{Model: modelName})
		if modelName == m.currentModel {
			m.cursor = index
		}
	}
	m.options = append(m.options, modelOption{Custom: true})
	m.loading = false
}

func (m ConfigModelModel) Init() tea.Cmd {
	return nil
}

func (m ConfigModelModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width > 0 && msg.Height > 0 {
			m.terminalWidth = msg.Width
			m.terminalHeight = msg.Height
		}
	case tea.KeyMsg:
		if m.step == modelStepCustom {
			if msg.String() == "enter" {
				modelName := strings.TrimSpace(m.modelInput.Value())
				if modelName != "" {
					m.SelectedModel = modelName
					m.Confirmed = true
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.modelInput.Focus()
			m.modelInput, cmd = m.modelInput.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "up", "k":
			m.moveCursor(-1)
		case "down", "j":
			m.moveCursor(1)
		case "enter", "space", " ":
			if m.cursor >= 0 && m.cursor < len(m.options) {
				option := m.options[m.cursor]
				if option.Custom {
					m.step = modelStepCustom
					m.modelInput.SetValue("")
					m.modelInput.Focus()
				} else {
					m.SelectedModel = option.Model
					m.Confirmed = true
				}
			}
		}
	}
	return m, nil
}

func (m ConfigModelModel) View() string {
	width, height := orbitalViewport(m.terminalWidth, m.terminalHeight, m.styleConfig)
	if m.step == modelStepCustom {
		modelInput := m.modelInput
		modelInput.Focus()
		return style.RenderOrbitalFormView(style.OrbitalFormView{
			Copy: style.OrbitalFormCopy{
				Title:   m.messages.Title,
				Heading: m.messages.CustomSubtitle,
				Hint:    m.messages.CustomHint,
			},
			Lines:          []string{m.messages.CustomLabel, modelInput.View()},
			StyleConfig:    m.styleConfig,
			ViewportWidth:  width,
			ViewportHeight: height,
		})
	}

	copy := modelSetupCopy(m.messages)
	if m.loading {
		copy.Subtitle = m.messages.LoadingMessage
	}
	return style.RenderOrbitalMenuView(style.OrbitalMenuView{
		Copy:           copy,
		Options:        m.optionLabels(),
		Cursor:         m.cursor,
		StyleConfig:    m.styleConfig,
		ViewportWidth:  width,
		ViewportHeight: height,
	})
}

func modelSetupCopy(messages i18n.ModelSetupMessages) style.OrbitalMenuCopy {
	return style.OrbitalMenuCopy{
		Title:        messages.Title,
		Heading:      messages.Heading,
		Subtitle:     messages.Subtitle,
		MoveShortcut: messages.MoveShortcut,
		MoveAction:   messages.MoveAction,
		SelectKey:    messages.SelectKey,
		SelectAction: messages.SelectAction,
		QuitKey:      messages.QuitKey,
		QuitAction:   messages.QuitAction,
	}
}

func (m ConfigModelModel) optionLabels() []string {
	labels := make([]string, 0, len(m.options))
	for _, option := range m.options {
		if option.Custom {
			labels = append(labels, m.messages.CustomOption)
		} else {
			labels = append(labels, option.Model)
		}
	}
	return labels
}

func (m *ConfigModelModel) moveCursor(offset int) {
	if len(m.options) == 0 {
		return
	}
	m.cursor = (m.cursor + offset + len(m.options)) % len(m.options)
}
