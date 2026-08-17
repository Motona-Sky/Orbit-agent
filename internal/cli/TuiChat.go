package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"orbit/internal/agentui"
	"orbit/internal/billing"
	"orbit/internal/config"
	"orbit/internal/event"
	"orbit/internal/i18n"
	"orbit/internal/llm"
	"orbit/internal/mcp"
	"orbit/internal/memorys"
	"orbit/internal/style"
	"orbit/internal/utils"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ChatTuimodel
type model struct {
	composer           textinput.Model // 用户输入框。
	composerValue      string
	Pwd                string
	sessionID          string
	width              int           // 最近一次收到的终端宽度。
	height             int           // 最近一次收到的终端高度。
	screenInitialized  bool          // 主聊天是否已收到有效尺寸并可渲染完整终端网格。
	lastTerminalHeight int           // 上一次有效窗口高度，用于终端增高时补空行。
	messages           i18n.Messages // 当前界面使用的文案，默认来自 i18n.DefaultLanguage。
	styleConfig        config.StyleConfig

	transcript               []chatTranscriptEntry
	terminalTranscriptCursor int
	promptHistory            []string
	historyCursor            int
	running                  bool
	runningStatus            runningStatusState
	thinkingText             string
	activities               []chatTranscriptEntry
	activitiesExpanded       bool
	turnCanceled             bool
	exitConfirm              bool
	agentUI                  *agentui.AgentUI
	activeQuestion           int
	tasks                    []agentui.TaskItem
	usageStats               agentui.UsageStats
	modelSetup               *ConfigModelModel
	modelSetupRequestID      uint64
	providerSetup            *ConfigProviderModel
	effortSetup              *ConfigEffortModel
	mcpPage                  *mcpPageState
	mcpManager               *mcp.Manager
	mcpStatuses              []mcp.ServiceStatus
	mcpStartupError          string
	setupScreenReady         bool
	pendingInput             string
	activeAgentInput         string
	streamingResultIndex     int
	slashMenuCursor          int
	slashMenuDismissed       bool
	wantsRestart             bool
}

const (
	// defaultWidth 用于尚未收到窗口尺寸事件时的默认渲染宽度。
	defaultWidth        = 118
	workDockBreakpoint  = 72
	workDockGap         = 2
	terminalRightMargin = 1

	historyCursorIdle = -1
)

type chatTranscriptEntry struct {
	kind         transcriptKind
	role         string
	content      string
	pending      bool
	activityKind agentui.ActivityKind
	options      []string
	selected     int
	answer       string
	question     *agentui.QuestionEvent
	skills       []skillDisplayItem
}

type transcriptKind int

const (
	transcriptMessage transcriptKind = iota
	transcriptActivity
	transcriptActivitySummary
	transcriptQuestion
	transcriptStatus
	transcriptSkillsList
)

type chatMessageKind int

const (
	chatMessageNormal  chatMessageKind = iota //0
	chatMessageBang                           //1
	chatMessageMention                        //2
	chatMessageSlash                          //3
)

// NewModel 创建可被 cmd 入口和测试复用的 TUI 初始模型。
func NewModel() model {
	return NewModelForLanguage(i18n.DefaultLanguage)
}

// NewModelForLanguage 创建指定语言的 TUI 模型，未知语言由 i18n 回退为默认英文。
func NewModelForLanguage(languageCode string) model {
	return initialModel(languageCode)
}

func newModelForLanguageWithMcp(languageCode string, manager *mcp.Manager) model {
	m := initialModel(languageCode)
	m.mcpManager = manager
	m.refreshMcpStatus()
	return m
}

func NewModelForSession(languageCode string, session memorys.SessionSummary) model {
	m := initialModel(languageCode)
	m.sessionID = session.ID
	m.transcript = transcriptFromMemory(session.Messages)
	m.terminalTranscriptCursor = 0
	m.usageStats.ContextUsed = estimateContextTokens(session.Messages)
	return m
}

// estimateContextTokens 根据消息内容的字符数粗略估算 token 数量。
func estimateContextTokens(messages []llm.MemoryMessage) float64 {
	var total int
	for _, msg := range messages {
		total += len(msg.Content)
		for _, tc := range msg.ToolCalls {
			if m, ok := tc.(map[string]any); ok {
				if fn, ok := m["function"].(map[string]any); ok {
					if args, ok := fn["arguments"].(string); ok {
						total += len(args)
					}
				}
			}
		}
	}
	// 粗略估算：每 4 个字符约 1 个 token
	return float64(total) / 4
}

func transcriptFromMemory(messages []llm.MemoryMessage) []chatTranscriptEntry {
	transcript := make([]chatTranscriptEntry, 0, len(messages))
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if content == "" || (message.Role != "user" && message.Role != "assistant") {
			continue
		}
		transcript = append(transcript, chatTranscriptEntry{
			kind:    transcriptMessage,
			role:    message.Role,
			content: message.Content,
		})
	}
	return transcript
}

// NewModelFromConfig 按 YAML 配置中的语言初始化主 TUI，读取失败时回退默认英文。
func NewModelFromConfig() model {
	return newModelFromConfigWithMcp(nil)
}

func newModelFromConfigWithMcp(manager *mcp.Manager) model {
	appConfig, err := config.LoadAppConfig()
	if err != nil {
		return newModelForLanguageWithMcp(i18n.DefaultLanguage, manager)
	}
	return newModelForLanguageWithMcp(appConfig.Language, manager)
}

// initialModel 设置默认焦点和输入框占位文案。
func initialModel(languageCode string) model {
	messages := i18n.For(languageCode).Messages
	messages.Chat.WorkspacePath = currentWorkspacePath()
	composer := textinput.New()
	composer.Placeholder = messages.Chat.AgentCommandPlaceholder
	composer.CharLimit = 280
	composer.Cursor.SetMode(cursor.CursorStatic)
	composer.Focus()

	m := model{
		composer:             composer,
		Pwd:                  currentWorkingDirectory(),
		width:                defaultWidth,
		messages:             messages,
		styleConfig:          loadStyleConfigOrDefault(),
		historyCursor:        historyCursorIdle,
		activeQuestion:       historyCursorIdle,
		streamingResultIndex: historyCursorIdle,
	}
	// 加载今日已有的 token 用量（可能来自之前的 session）
	if usage, err := billing.QueryTodayUsage(); err == nil {
		m.usageStats = agentui.UsageStats{
			TodayTokens:  usage.TotalTokens,
			CacheHitRate: usage.CacheHitRate(),
			ContextTotal: utils.MaxContextLength,
		}
	} else {
		m.usageStats = agentui.UsageStats{
			ContextTotal: utils.MaxContextLength,
		}
	}
	return m
}

func currentWorkspacePath() string {
	workspace := currentWorkingDirectory()
	if workspace == "" {
		return "  -"
	}
	return "  " + workspace
}

func currentWorkingDirectory() string {
	workspace, err := os.Getwd()
	if err != nil || workspace == "" {
		return ""
	}
	return workspace
}

// startupBanner 生成主聊天进入普通终端时只输出一次的 Logo 和工作区。
func (m model) startupBanner() string {
	width := terminalContentWidth(m.width)
	if width == 0 {
		return ""
	}

	indent := ""
	contentWidth := width
	if width >= 3 {
		indent = "  "
		contentWidth = width - lipgloss.Width(indent)
	}

	lines := []string{style.RenderOrbitalLogoForViewportWithStyle(m.messages.Chat.BrandName, width, m.styleConfig)}
	workspace := strings.TrimSpace(m.messages.Chat.WorkspacePath)
	if workspace != "" && workspace != "-" {
		workspace = lipgloss.NewStyle().Inline(true).MaxWidth(contentWidth).Render(workspace)
		lines = append(lines, m.mutedStyle().Render(indent+workspace))
	}
	dividerWidth := minInt(21, contentWidth)
	lines = append(lines, m.mutedStyle().Render(indent+strings.Repeat("─", dividerWidth)))
	return strings.Join(lines, "\n")
}

// startupPrelude 生成首次清屏后永久输出的横幅和底部定位空行。
func (m model) startupPrelude() string {
	banner := m.startupBanner()
	targetHeight := m.height - lipgloss.Height(m.View())
	outputHeight := maxInt(lipgloss.Height(banner), targetHeight)
	return banner + strings.Repeat("\n", outputHeight-lipgloss.Height(banner))
}

// terminalBlankLines 返回让 tea.Println 精确输出 count 个空行的消息体。
func terminalBlankLines(count int) string {
	if count <= 1 {
		return ""
	}
	return strings.Repeat("\n", count-1)
}

func (m model) handleComposerKey(msg tea.KeyMsg) (bool, model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+j", "alt+enter", "shift+enter":
		m.appendComposerText("\n")
		return true, m, nil
	case "enter":
		next, cmd := m.handleComposerSubmit()
		return true, next, cmd
	case "up":
		if m.browseComposerHistory(-1) {
			return true, m, nil
		}
	case "down":
		if m.browseComposerHistory(1) {
			return true, m, nil
		}
	}

	if m.applyComposerEdit(msg) {
		return true, m, nil
	}

	return false, m, nil
}

// 输入框输入类型 / ! @
func (m model) handleComposerSubmit() (model, tea.Cmd) {
	return m.handleComposerSubmitValue(strings.TrimSpace(m.composerText()))
}

func (m model) handleComposerSubmitValue(value string) (model, tea.Cmd) {
	if value == "" {
		m.exitConfirm = false
		return m, nil
	}
	m.clearComposer()
	m.exitConfirm = false
	if m.running {
		if m.pendingInput == "" {
			m.pendingInput = value
		}
		return m, nil
	}
	return m.executeSubmittedInput(value)
}

func (m model) executeSubmittedInput(value string) (model, tea.Cmd) {
	switch classifyChatMessage(value) {
	case chatMessageBang, chatMessageNormal:
		return m.handleVisibleUserMessageSubmit(value) //普通消息
	case chatMessageMention:
		return m.handleMentionMessageSubmit(value)
	case chatMessageSlash:
		return m.handleSlashMessageSubmit(value)
	default:
		return m, nil
	}
}

func classifyChatMessage(value string) chatMessageKind {
	value = strings.TrimSpace(value)
	if value == "" {
		return chatMessageNormal
	}

	switch value[0] {
	case '!':
		return chatMessageBang
	case '@':
		return chatMessageMention
	case '/':
		return chatMessageSlash
	default:
		return chatMessageNormal
	}
}

// 普通消息对话
func (m model) handleVisibleUserMessageSubmit(value string) (model, tea.Cmd) {
	return m.startAgentTurn(value, value)
}

func (m model) startAgentTurn(visibleValue, agentValue string) (model, tea.Cmd) {
	m.appendUserTranscript(visibleValue, false)
	m.promptHistory = append(m.promptHistory, visibleValue)
	m.historyCursor = historyCursorIdle
	m.clearThinkingText()
	m.running = true
	m.turnCanceled = false
	m.exitConfirm = false
	m.clearComposer()
	if m.agentUI != nil {
		m.agentUI.Close()
	}
	m.agentUI = agentui.New()
	m.activeAgentInput = agentValue
	runCmd := runAgent(m.agentUI, m.eventForPrompt(agentValue), m.messages.Chat.AgentErrorLabel)
	waitCmd := waitForAgentUI(m.agentUI)
	statusCmd := m.startRunningStatus(time.Now())
	return m.commitTerminalTranscript(tea.Batch(runCmd, waitCmd, statusCmd))
}

func (m model) eventForPrompt(value string) event.TuiEvent {
	return event.TuiEvent{
		SessionId:     m.sessionID,
		ResumeSession: m.sessionID != "",
		Pwd:           m.Pwd,
		Text:          value,
		ThinkLevel:    utils.ThinkLevel,
	}
}

func (m *model) appendUserTranscript(value string, pending bool) {
	m.transcript = append(m.transcript, chatTranscriptEntry{
		role:    "user",
		content: value,
		pending: pending,
	})
}

func (m model) handleMentionMessageSubmit(_ string) (model, tea.Cmd) {
	m.clearComposer()
	m.exitConfirm = false
	return m, nil
}

func (m model) handleCtrlC() (model, tea.Cmd) {
	if m.running {
		return m.cancelRunningTurn()
	}

	if strings.TrimSpace(m.composerText()) != "" {
		m.clearComposer()
		m.exitConfirm = false
		return m, nil
	}

	if !m.exitConfirm {
		m.exitConfirm = true
		return m, nil
	}

	return m, tea.Quit
}

func (m model) handleEsc() (model, tea.Cmd) {
	if m.running {
		return m.cancelRunningTurn()
	}

	if strings.TrimSpace(m.composerText()) != "" {
		m.clearComposer()
		m.exitConfirm = false
		return m, nil
	}

	m.exitConfirm = false
	return m, nil
}

func (m model) cancelRunningTurn() (model, tea.Cmd) {
	m.closeAgentUI()
	m.clearPendingUserTranscript()
	m.running = false
	m.stopRunningStatus()
	m.turnCanceled = true
	m.exitConfirm = false
	m.appendActivitySummary()
	m.appendTaskSummary(true)
	m, transcriptCmd := m.commitTerminalTranscript(nil)
	return m.runPendingInput(transcriptCmd)
}

func (m model) runPendingInput(existing tea.Cmd) (model, tea.Cmd) {
	value := m.pendingInput
	if strings.TrimSpace(value) == "" {
		return m, existing
	}
	m.pendingInput = ""
	next, nextCmd := m.executeSubmittedInput(value)
	return next, sequenceTeaCommands(existing, nextCmd)
}

func (m *model) appendTaskSummary(canceled bool) {
	total := len(m.tasks)
	if total == 0 {
		if canceled {
			m.transcript = append(m.transcript, chatTranscriptEntry{kind: transcriptStatus, content: m.messages.Chat.TurnCanceled})
		}
		return
	}
	done := 0
	for _, task := range m.tasks {
		if task.Status == agentui.TaskDone {
			done++
		}
	}
	format := m.messages.Chat.TaskCompleted
	if canceled {
		format = m.messages.Chat.TaskCanceled
	} else {
		done = total
	}
	m.transcript = append(m.transcript, chatTranscriptEntry{kind: transcriptStatus, content: fmt.Sprintf(format, done, total)})
	m.tasks = nil
}

func (m *model) browseComposerHistory(direction int) bool {
	if m.running {
		return false
	}

	if strings.TrimSpace(m.composerText()) != "" && m.historyCursor == historyCursorIdle {
		return false
	}
	return m.browseStringList(m.promptHistory, &m.historyCursor, direction)
}

func (m *model) browseStringList(values []string, cursor *int, direction int) bool {
	if len(values) == 0 {
		return false
	}

	if *cursor == historyCursorIdle {
		if direction > 0 {
			return false
		}
		*cursor = len(values) - 1
		m.setComposerValue(values[*cursor])
		m.reopenSlashMenuAfterEdit()
		return true
	}

	*cursor += direction
	if *cursor < 0 {
		*cursor = 0
	}
	if *cursor >= len(values) {
		*cursor = historyCursorIdle
		m.clearComposer()
		return true
	}

	m.setComposerValue(values[*cursor])
	m.reopenSlashMenuAfterEdit()
	return true
}

func (m *model) appendComposerText(value string) {
	m.setComposerValue(m.composerText() + value)
	m.reopenSlashMenuAfterEdit()
	m.historyCursor = historyCursorIdle
	m.exitConfirm = false
}

func (m *model) clearComposer() {
	m.setComposerValue("")
	m.slashMenuDismissed = false
	m.slashMenuCursor = 0
}

func (m *model) setComposerValue(value string) {
	if limit := m.composer.CharLimit; limit > 0 {
		runes := []rune(value)
		if len(runes) > limit {
			value = string(runes[:limit])
		}
	}
	m.composerValue = value
	m.composer.SetValue(value)
	m.composer.CursorEnd()
}

func (m model) composerText() string {
	return m.composerValue
}

func (m *model) applyComposerEdit(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyRunes:
		m.appendComposerText(string(msg.Runes))
		return true
	case tea.KeySpace:
		m.appendComposerText(" ")
		return true
	case tea.KeyBackspace, tea.KeyCtrlH:
		m.setComposerValue(dropLastRune(m.composerText()))
		m.reopenSlashMenuAfterEdit()
		m.historyCursor = historyCursorIdle
		m.exitConfirm = false
		return true
	case tea.KeyDelete:
		m.historyCursor = historyCursorIdle
		m.exitConfirm = false
		return true
	default:
		return false
	}
}

func dropLastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return value
	}
	return string(runes[:len(runes)-1])
}

// View 在普通终端底部渲染原工作坞；稳定聊天记录由终端原生历史承载。
func (m model) View() string {
	if m.modelSetup != nil {
		if !m.setupScreenReady {
			return ""
		}
		return m.modelSetup.View()
	}
	if m.providerSetup != nil {
		if !m.setupScreenReady {
			return ""
		}
		return m.providerSetup.View()
	}
	if m.effortSetup != nil {
		if !m.setupScreenReady {
			return ""
		}
		return m.effortSetup.View()
	}
	if m.mcpPage != nil {
		if !m.setupScreenReady {
			return ""
		}
		return m.renderMcpPage()
	}
	width := terminalContentWidth(m.width)
	if width == 0 {
		return ""
	}
	footerHelp := m.messages.Chat.CompactFooterHelp
	if m.exitConfirm {
		footerHelp = m.messages.Chat.ExitConfirm
	}
	lines := []string{}
	if status := m.renderInlineStatus(width); status != "" {
		lines = append(lines, status, "")
	}
	lines = append(lines, m.renderWorkDock(width, footerHelp))
	dock := strings.TrimRight(strings.Join(lines, "\n"), "\n")
	if m.screenInitialized && m.height > 0 && lipgloss.Height(dock) > m.height {
		return m.renderShortScreenChat(width, m.height)
	}
	return dock
}

// renderShortScreenChat 在原工作坞无法放入终端时保留当前状态与输入。
func (m model) renderShortScreenChat(width, height int) string {
	if height <= 0 {
		return ""
	}
	value := strings.TrimSpace(m.composerText())
	if value == "" {
		value = m.messages.Chat.AgentCommandPlaceholder
	}
	input := fitChatLine(m.messages.Chat.AgentCommandPrompt+" "+value, width)
	lines := []string{}
	if m.streamingResultIndex >= 0 && m.streamingResultIndex < len(m.transcript) {
		entry := m.transcript[m.streamingResultIndex]
		if entry.pending && strings.TrimSpace(entry.content) != "" {
			streamLines := clampTerminalLines(m.renderTranscriptEntry(width, entry), width)
			lines = append(lines, streamLines...)
		}
	}
	if m.activeQuestion >= 0 && m.activeQuestion < len(m.transcript) {
		entry := m.transcript[m.activeQuestion]
		if entry.kind == transcriptQuestion && entry.pending {
			question := strings.TrimSpace(entry.content)
			if entry.selected >= 0 && entry.selected < len(entry.options) {
				question += " · " + entry.options[entry.selected]
			}
			lines = append(lines, fitChatLine(question, width))
		}
	}
	if len(lines) == 0 && height > 1 {
		status := ""
		if strings.TrimSpace(m.pendingInput) != "" {
			status = "> " + m.pendingInput
		}
		if status == "" {
			status = m.renderRuntimeActivityTree(width)
		}
		if status == "" {
			status = m.workPromptLines()[0]
		}
		lines = append(lines, fitChatLine(strings.Split(status, "\n")[0], width))
	}
	if len(lines) < height {
		lines = append(lines, input)
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append([]string{""}, lines...)
	}
	return strings.Join(lines, "\n")
}

func fitChatLine(value string, width int) string {
	value = strings.Join(strings.Fields(value), " ")
	return lipgloss.NewStyle().Inline(true).MaxWidth(maxInt(1, width)).Render(value)
}

// renderInlineStatus 在工作提示坞上方显示当前待回答询问。
func (m model) renderInlineStatus(width int) string {
	if m.activeQuestion >= 0 && m.activeQuestion < len(m.transcript) {
		entry := m.transcript[m.activeQuestion]
		if entry.kind == transcriptQuestion && entry.pending {
			return strings.Join(m.renderQuestion(entry, width), "\n")
		}
	}
	parts := make([]string, 0, 3)
	if streaming := m.renderStreamingResult(width); streaming != "" {
		parts = append(parts, streaming)
	}
	if runtime := m.renderRuntimeActivityTree(width); runtime != "" {
		parts = append(parts, runtime)
	}
	if pending := m.renderPendingInput(width); pending != "" {
		parts = append(parts, pending)
	}
	return strings.Join(parts, "\n\n")
}

func (m model) renderStreamingResult(width int) string {
	if m.streamingResultIndex < 0 || m.streamingResultIndex >= len(m.transcript) || width <= 0 {
		return ""
	}
	entry := m.transcript[m.streamingResultIndex]
	if !entry.pending || entry.role != "assistant" || strings.TrimSpace(entry.content) == "" {
		return ""
	}
	lines := clampTerminalLines(m.renderTranscriptEntry(width, entry), width)
	return strings.Join(lines, "\n")
}

func (m model) renderPendingInput(width int) string {
	if strings.TrimSpace(m.pendingInput) == "" || width <= 0 {
		return ""
	}
	lines := m.renderTranscriptEntry(width, chatTranscriptEntry{
		kind:    transcriptMessage,
		role:    "user",
		content: m.pendingInput,
		pending: true,
	})
	return strings.Join(clampTerminalLines(lines, width), "\n")
}

// workPromptLines 将当前任务状态转换为工作提示框中的显示行。
func (m model) workPromptLines() []string {
	lines := []string{
		m.mutedStyle().Render(fmt.Sprintf(
			"%s %s",
			m.messages.Chat.TodayTokens,
			formatTokenCount(m.usageStats.TodayTokens),
		)),
		m.mutedStyle().Render(fmt.Sprintf(
			"%s %.1f%%",
			m.messages.Chat.CacheHitRate,
			m.usageStats.CacheHitRate,
		)),
		m.mutedStyle().Render(fmt.Sprintf(
			"%s %.1f%% %s/%s",
			m.messages.Chat.ContextUsage,
			contextUsagePercent(m.usageStats.ContextUsed, m.usageStats.ContextTotal),
			formatTokenCount(int64(m.usageStats.ContextUsed)),
			formatTokenCount(int64(m.usageStats.ContextTotal)),
		)),
	}
	if summary := m.mcpErrorSummary(); summary != "" {
		lines = append(lines, m.accentStyle().Render(summary))
	}
	if len(m.tasks) == 0 {
		return lines
	}
	for _, task := range m.tasks {
		lines = append(lines, m.renderTaskItem(task))
	}
	return lines
}

func contextUsagePercent(used, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return used / total * 100
}

func formatTokenCount(value int64) string {
	digits := strconv.FormatInt(value, 10)
	for index := len(digits) - 3; index > 0; index -= 3 {
		digits = digits[:index] + "," + digits[index:]
	}
	return digits
}

// renderWorkDock 将输入区放在左侧、工作提示框放在右侧，并让右框延伸至动态区底部。
func (m model) renderWorkDock(width int, footerHelp string) string {
	if width < 8 {
		value := strings.TrimSpace(m.composerText())
		if value == "" {
			value = m.messages.Chat.AgentCommandPlaceholder
		}
		return lipgloss.NewStyle().Inline(true).MaxWidth(width).Render(m.messages.Chat.AgentCommandPrompt + " " + value)
	}
	if width+terminalRightMargin < workDockBreakpoint {
		return m.renderStackedWorkDock(width, footerHelp)
	}
	taskWidth := maxInt(18, width/4)
	inputWidth := maxInt(1, width-taskWidth-workDockGap)
	input := m.renderAgentCommandInput(inputWidth)
	panelHeight := maxInt(lipgloss.Height(input)+2, len(m.workPromptLines())+2)
	inputLines := []string{input, m.renderDockFooterHelp(footerHelp, inputWidth)}
	for lipgloss.Height(strings.Join(inputLines, "\n")) < panelHeight {
		inputLines = append(inputLines, " ")
	}
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		strings.Join(inputLines, "\n"),
		strings.Repeat(" ", workDockGap),
		m.renderWorkPromptPanel(taskWidth, panelHeight),
	)
}

// renderStackedWorkDock 在窄终端中将任务框放到输入区上方。
func (m model) renderStackedWorkDock(width int, footerHelp string) string {
	panel := m.renderWorkPromptPanel(width, len(m.workPromptLines())+2)
	return strings.Join([]string{
		panel,
		"",
		m.renderAgentCommandInput(width),
		m.renderDockFooterHelp(footerHelp, width),
		" ",
	}, "\n")
}

// renderDockFooterHelp 将底部帮助限制在所属输入栏宽度内。
func (m model) renderDockFooterHelp(footerHelp string, width int) string {
	return m.mutedStyle().Inline(true).MaxWidth(maxInt(1, width)).Render(footerHelp)
}

// renderWorkPromptPanel 绘制指定宽高的工作提示框，标题嵌入顶部边框。
func (m model) renderWorkPromptPanel(width, height int) string {
	width = maxInt(8, width)
	lines := m.workPromptLines()
	height = maxInt(height, len(lines)+2)
	innerWidth := maxInt(1, width-4)
	title := strings.TrimSpace(strings.TrimPrefix(m.messages.Chat.CurrentTask, ">"))
	title = lipgloss.NewStyle().Inline(true).MaxWidth(maxInt(1, width-6)).Render(title)
	dashes := maxInt(1, width-lipgloss.Width(title)-5)
	rendered := []string{
		m.mutedStyle().Render("╭─ ") + m.accentStyle().Render(title) + m.mutedStyle().Render(" "+strings.Repeat("─", dashes)+"╮"),
	}
	for index := 0; index < height-2; index++ {
		line := ""
		if index < len(lines) {
			line = lipgloss.NewStyle().Inline(true).MaxWidth(innerWidth).Render(lines[index])
		}
		line += strings.Repeat(" ", maxInt(0, innerWidth-lipgloss.Width(line)))
		rendered = append(rendered, m.mutedStyle().Render("│")+" "+line+" "+m.mutedStyle().Render("│"))
	}
	rendered = append(rendered, m.mutedStyle().Render("╰"+strings.Repeat("─", width-2)+"╯"))
	return strings.Join(rendered, "\n")
}

// pendingTerminalTranscriptOutput 生成从当前游标开始的连续稳定记录文本。
func (m model) pendingTerminalTranscriptOutput(width int) (string, int) {
	end := m.terminalTranscriptCursor
	lines := []string{}
	for end < len(m.transcript) {
		entry := m.transcript[end]
		if entry.pending {
			break
		}
		entryLines := clampTerminalLines(m.renderTranscriptEntry(width, entry), width)
		if len(entryLines) > 0 {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, entryLines...)
		}
		end++
	}
	return strings.Join(lines, "\n"), end
}

// commitTerminalTranscript 将新的稳定记录写入原生终端历史并推进游标。
func (m model) commitTerminalTranscript(existing tea.Cmd) (model, tea.Cmd) {
	width := terminalContentWidth(m.width)
	if width == 0 {
		return m, existing
	}
	output, end := m.pendingTerminalTranscriptOutput(width)
	if end == m.terminalTranscriptCursor {
		return m, existing
	}
	m.terminalTranscriptCursor = end
	if output == "" {
		return m, existing
	}
	return m, sequenceTeaCommands(tea.Println(output), existing)
}

func terminalContentWidth(terminalWidth int) int {
	if terminalWidth <= 0 {
		terminalWidth = defaultWidth
	}
	return maxInt(0, terminalWidth-terminalRightMargin)
}

func clampTerminalLines(lines []string, width int) []string {
	if width <= 0 {
		return nil
	}
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		wrapped = append(wrapped, strings.Split(ansi.Hardwrap(line, width, true), "\n")...)
	}
	return wrapped
}

// renderTranscriptEntry 按消息、活动或询问类型选择对应的展示样式。
func (m model) renderTranscriptEntry(width int, entry chatTranscriptEntry) []string {
	switch entry.kind {
	case transcriptActivity:
		return m.renderActivity(entry, width)
	case transcriptActivitySummary:
		if strings.TrimSpace(entry.content) == "" {
			return nil
		}
		return []string{m.mutedStyle().Render(entry.content)}
	case transcriptQuestion:
		if strings.TrimSpace(entry.answer) == "" {
			return nil
		}
		return []string{m.accentStyle().Render(strings.TrimSpace(entry.content) + " → " + entry.answer)}
	case transcriptStatus:
		if strings.TrimSpace(entry.content) == "" {
			return nil
		}
		return []string{m.accentStyle().Render(entry.content)}
	case transcriptSkillsList:
		lines := make([]string, 0, len(entry.skills)+1)
		lines = append(lines, m.accentStyle().Render(m.messages.Chat.AssistantLabel))
		for _, skill := range entry.skills {
			line := m.pureWhiteStyle().Render(skill.Name)
			if skill.Description != "" {
				line += "  " + m.mutedStyle().Render(skill.Description)
			}
			lines = append(lines, line)
		}
		return lines
	}

	content := strings.TrimSpace(entry.content)
	if content == "" {
		return nil
	}

	if entry.role == "user" {
		return []string{m.mutedStyle().Render(m.messages.Chat.YouLabel), prefixFirstLine("> ", content)}
	}
	if strings.HasPrefix(content, m.messages.Chat.AgentErrorLabel) {
		return []string{m.accentStyle().Render("✗ ") + strings.TrimPrefix(content, m.messages.Chat.AgentErrorLabel)}
	}
	return []string{m.accentStyle().Render(m.messages.Chat.AssistantLabel), content}
}

func prefixFirstLine(prefix, content string) string {
	lines := strings.Split(content, "\n")
	lines[0] = prefix + lines[0]
	return strings.Join(lines, "\n")
}

func (m model) renderAgentCommandInput(width int) string {
	messages := m.messages.Chat
	//底部功能区
	input := style.RenderAgentCommandInput(style.AgentCommandInputView{
		Copy: style.AgentCommandInputCopy{
			Title:       messages.ComposerTitle,
			Prompt:      messages.AgentCommandPrompt,
			Placeholder: messages.AgentCommandPlaceholder,
			Modes:       nil,
		},
		InfoItems:   []style.AgentCommandInfoItem{{Label: "Think", Value: utils.ThinkLevel}, {Label: "Model", Value: utils.Model}, {Label: "CWD", Value: utils.Cwd}},
		StyleConfig: m.styleConfig,
		Value:       m.composerText(),
		Focused:     true,
		Width:       width,
	})
	menu := m.renderSlashCommandMenu(width)
	if menu == "" {
		return input
	}
	return menu + "\n" + input
}

func clampInt(value, minValue, maxValue int) int {
	return maxInt(minValue, minInt(value, maxValue))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ChatTuiUpdate
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if tick, ok := msg.(runningStatusTickMsg); ok {
		updated, tickCmd := m.handleRunningStatusTick(tick)
		return updated, tickCmd
	}
	if _, ok := msg.(setupScreenReadyMsg); ok {
		m.setupScreenReady = true
		return m, nil
	}
	if started, ok := msg.(mcpStartedMsg); ok {
		if started.err != nil {
			m.mcpStartupError = started.err.Error()
		}
		m.refreshMcpStatus()
		return m, waitForMcpChange(m.mcpManager)
	}
	if _, ok := msg.(mcpChangedMsg); ok && m.mcpPage == nil {
		m.refreshMcpStatus()
		return m, waitForMcpChange(m.mcpManager)
	}
	if (m.modelSetup != nil || m.providerSetup != nil || m.effortSetup != nil || m.mcpPage != nil) && !m.setupScreenReady {
		if _, ok := msg.(tea.KeyMsg); ok {
			return m, nil
		}
	}
	if m.modelSetup != nil {
		return m.updateModelSetup(msg)
	}
	if m.providerSetup != nil {
		return m.updateProviderSetup(msg)
	}
	if m.effortSetup != nil {
		return m.updateEffortSetup(msg)
	}
	if m.mcpPage != nil {
		return m.updateMcpPage(msg)
	}

	switch msg := msg.(type) {
	case agentUIEventMsg:
		if msg.ui != m.agentUI {
			return m, nil
		}
		return m.handleAgentUIEvent(msg.event)
	case agentui.ResultEvent:
		return m.handleAgentUIEvent(msg)
	case agentui.StreamResultEvent:
		return m.handleAgentUIEvent(msg)
	case agentui.StreamResultDoneEvent:
		return m.handleAgentUIEvent(msg)
	case agentui.FinalResultEvent:
		return m.handleAgentUIEvent(msg)
	case agentui.ThinkingEvent:
		return m.handleAgentUIEvent(msg)
	case agentui.ActivityEvent:
		return m.handleAgentUIEvent(msg)
	case agentui.TaskPlanEvent:
		return m.handleAgentUIEvent(msg)
	case *agentui.QuestionEvent:
		return m.handleAgentUIEvent(msg)
	case agentUIClosedMsg:
		return m, nil
	case agentRunFinishedMsg:
		if msg.ui != m.agentUI {
			return m, nil
		}
		if msg.err != nil && !m.turnCanceled {
			m.clearPendingUserTranscript()
			m.appendActivitySummary()
			m.transcript = append(m.transcript, chatTranscriptEntry{
				kind:    transcriptMessage,
				role:    "assistant",
				content: m.messages.Chat.AgentErrorLabel + msg.err.Error(),
			})
			m.appendTaskSummary(true)
			m.running = false
			m.stopRunningStatus()
			m.closeAgentUI()
			m, transcriptCmd := m.commitTerminalTranscript(nil)
			return m.runPendingInput(transcriptCmd)
		}
		return m, nil
	case tea.WindowSizeMsg:
		if msg.Width <= 0 || msg.Height <= 0 {
			return m, nil
		}
		m.width = msg.Width
		m.height = msg.Height
		if !m.screenInitialized {
			m.screenInitialized = true
			m.lastTerminalHeight = msg.Height
			m, transcriptCmd := m.commitTerminalTranscript(nil)
			startupCmd := tea.Sequence(
				tea.ClearScreen,
				tea.Println(m.startupPrelude()),
			)
			return m, sequenceTeaCommands(startupCmd, transcriptCmd)
		}
		heightDelta := msg.Height - m.lastTerminalHeight
		m.lastTerminalHeight = msg.Height
		if heightDelta > 0 {
			return m, tea.Println(terminalBlankLines(heightDelta))
		}
		return m, nil
	case tea.KeyMsg:
		if msg.String() != "ctrl+c" {
			m.exitConfirm = false
		}
		if handled, next, questionCmd := m.handleQuestionKey(msg); handled {
			return next, questionCmd
		}
		if handled, next, menuCmd := m.handleSlashCommandMenuKey(msg); handled {
			return next, menuCmd
		}
		switch msg.String() {
		case "ctrl+o":
			if handled, updated := m.handleActivityToggle(); handled {
				return updated, nil
			}
		case "ctrl+c":
			return m.handleCtrlC()
		case "esc":
			return m.handleEsc()
		}

		if handled, next, cmd := m.handleComposerKey(msg); handled {
			return next, cmd
		}
	}

	m.composer.Focus()
	m.composer, cmd = m.composer.Update(msg)

	return m, cmd
}

// ChatTuiinit
func (m model) Init() tea.Cmd {
	pwd := currentWorkingDirectory()
	if m.sessionID == "" {
		event.InitChatTuiEvent(pwd)
	} else {
		event.ResumeChatTuiEvent(pwd, m.sessionID)
	}
	//项目文件夹
	_, err := os.Stat(config.ProejctConfigPath)
	if err != nil {
		config.CreateProjectConfig()
	}
	utils.GInit()
	return startMcpManager(m.mcpManager)
}
