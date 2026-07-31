// Package i18n 集中定义界面多语言结构，并按语言代码返回文案。
package i18n

// DefaultLanguage 是当前未接入配置读取时的默认语言。
const DefaultLanguage = "en"

// Language 描述一种可用语言，具体文案定义放在独立 language_*.go 文件中。
type Language struct {
	Code     string
	Name     string
	Messages Messages
}

// Messages 聚合当前 TUI 会用到的多语言文案。
type Messages struct {
	Chat          ChatMessages
	LanguageSetup LanguageSetupMessages
	SessionSetup  SessionSetupMessages
	ThemeSetup    ThemeSetupMessages
	ModelSetup    ModelSetupMessages
	ProviderSetup ProviderSetupMessages
	EffortSetup   EffortSetupMessages
	OtherActivity OtherActivityMessages
}

// ChatMessages 是主聊天 TUI 的文案结构。
type ChatMessages struct {
	BrandName                  string
	AgentWorkspace             string
	FooterHelp                 string
	CompactFooterHelp          string
	ExitConfirm                string
	SessionsTitle              string
	CurrentTask                string
	TodayTokens                string
	CacheHitRate               string
	TUILayout                  string
	Notes                      string
	ShortcutsTitle             string
	TabFocus                   string
	Submit                     string
	Quit                       string
	AgentChatTitle             string
	YouLabel                   string
	SampleUserMessage          string
	AssistantLabel             string
	SampleAssistantMessage     string
	ComposerTitle              string
	MessageAgent               string
	ContextTitle               string
	WorkspaceTitle             string
	WorkspacePath              string
	ToolsTitle                 string
	PlanTool                   string
	DiffTool                   string
	TerminalTool               string
	StateTitle                 string
	IdleState                  string
	AgentCommandPrompt         string
	AgentCommandPlaceholder    string
	AgentModeStub1             string
	AgentModeStub2             string
	AgentModeStub3             string
	AgentModeStub4             string
	ToolActivityLabel          string
	FileActivityLabel          string
	QuestionSelectedLabel      string
	QuestionHelp               string
	AgentErrorLabel            string
	AgentThinkingPhrases       []string
	AgentReading               string
	AgentExecuting             string
	ToolCallSummary            string
	ToolCallsSummary           string
	LatestActivityLabel        string
	ExpandToolCallsHint        string
	CollapseToolCallsHint      string
	ModelCommandDescription    string
	ProviderCommandDescription string
	EffortCommandDescription   string
	SkillsCommandDescription   string
	NewCommandDescription      string
	ClearCommandDescription    string
	SkillsEmpty                string
	SkillsTaskRequired         string
	SkillNotFound              string
	SkillPathMissing           string

	ContextUsage  string
	TaskCompleted string
	TaskCanceled  string
	TurnCanceled  string
}

// LanguageSetupMessages 是语言选择 TUI 自身的文案结构。
type LanguageSetupMessages struct {
	Title        string
	Heading      string
	Subtitle     string
	MoveShortcut string
	MoveAction   string
	SelectKey    string
	SelectAction string
	QuitKey      string
	QuitAction   string
}

// SessionSetupMessages 是历史会话选择 TUI 的文案结构。
type SessionSetupMessages struct {
	Title         string
	Heading       string
	Subtitle      string
	EmptyState    string
	SkippedFormat string
	MoveShortcut  string
	MoveAction    string
	SelectKey     string
	SelectAction  string
	QuitKey       string
	QuitAction    string
}

// ThemeSetupMessages 是主题选择 TUI 自身的文案结构。
type ThemeSetupMessages struct {
	Title              string
	Subtitle           string
	Description        string
	DarkOption         string
	LightOption        string
	HighContrastOption string
	Hint               string
	MoveShortcut       string
	MoveAction         string
	SelectKey          string
	SelectAction       string
	QuitKey            string
	QuitAction         string
}

// ModelSetupMessages 是已配置模型选择 TUI 的文案结构。
type ModelSetupMessages struct {
	Title             string
	Heading           string
	Subtitle          string
	LoadingMessage    string
	CustomOption      string
	CustomSubtitle    string
	CustomLabel       string
	CustomPlaceholder string
	CustomHint        string
	MoveShortcut      string
	MoveAction        string
	SelectKey         string
	SelectAction      string
	QuitKey           string
	QuitAction        string
}

// EffortSetupMessages 是思考强度选择 TUI 的文案结构。
type EffortSetupMessages struct {
	Title        string
	Heading      string
	Subtitle     string
	MoveShortcut string
	MoveAction   string
	SelectKey    string
	SelectAction string
	QuitKey      string
	QuitAction   string
}

// ProviderSetupMessages 是模型提供商配置 TUI 的文案结构。
type ProviderSetupMessages struct {
	Title               string
	SelectSubtitle      string
	SelectDescription   string
	NameSubtitle        string
	BaseURLSubtitle     string
	APIKeySubtitle      string
	ModelSubtitle       string
	TypeSubtitle        string
	ConfirmSubtitle     string
	ProviderLabel       string
	NameLabel           string
	BaseURLLabel        string
	APIKeyLabel         string
	ModelLabel          string
	TypeLabel           string
	OpenAIOption        string
	AnthropicOption     string
	GeminiOption        string
	OllamaOption        string
	CustomOption        string
	OpenAITypeOption    string
	AnthropicTypeOption string
	NamePlaceholder     string
	BaseURLPlaceholder  string
	APIKeyPlaceholder   string
	ModelPlaceholder    string
	SelectHint          string
	NameHint            string
	BaseURLHint         string
	APIKeyHint          string
	ModelHint           string
	TypeHint            string
	ConfirmHint         string
	MoveShortcut        string
	MoveAction          string
	SelectKey           string
	SelectAction        string
	QuitKey             string
	QuitAction          string
}

var languageOrder = []Language{
	englishLanguage,
	simplifiedChineseLanguage,
}

var languagesByCode = map[string]Language{
	englishLanguage.Code:           englishLanguage,
	simplifiedChineseLanguage.Code: simplifiedChineseLanguage,
}

// For 按语言代码返回文案；未知代码回退到默认英文，避免调用方额外处理空值。
func For(code string) Language {
	if language, ok := languagesByCode[code]; ok {
		return language
	}
	return languagesByCode[DefaultLanguage]
}

// Options 按原有语言选择器顺序返回可选语言，返回副本避免外部修改注册表。
func Options() []Language {
	options := make([]Language, len(languageOrder))
	copy(options, languageOrder)
	return options
}

type OtherActivityMessages struct {
	ActivityLabel string
	RetryLabel    string
	CompressLabel string
}
