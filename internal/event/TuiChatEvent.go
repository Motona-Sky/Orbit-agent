package event

import (
	"looporbit/internal/agent"
	"looporbit/internal/agentui"
	"looporbit/internal/prompt"
	"looporbit/internal/utils"
)

// MessagesEvent 确定当前会话 ID，将聊天事件转换为 Agent 参数并执行本轮对话。
func MessagesEvent(chatmessages TuiEvent, ui *agentui.AgentUI) error {
	if chatTuiInit.Pwd == "" {
		InitChatTuiEvent(chatmessages.Pwd)
	}
	sessionid := chatmessages.SessionId
	if sessionid == "" {
		sessionid = chatTuiInit.SessionId
	}
	utils.SessionId = sessionid

	run := runAgentValueForEvent(chatmessages)
	return agent.RunAgent(run, ui)
}

// runAgentValueForEvent 将聊天事件与当前全局模型配置组合为 Agent 执行参数。
func runAgentValueForEvent(chatmessages TuiEvent) agent.RunAgentValue {
	return agent.RunAgentValue{
		Provider:      utils.Provider,
		ApiKey:        utils.ApiKey,
		ThinkLevel:    chatmessages.ThinkLevel,
		Model:         utils.Model,
		BaseUrl:       utils.BaseUrl,
		SystemPt:      prompt.DefaultSystemPrompt,
		UserInput:     chatmessages.Text,
		ResumeSession: chatmessages.ResumeSession,
	}
}

// Provider   string
// ApiKey     string
// BaseUrl    string
// Model      string
// Mcp        string
// SystemPt   string
// ThinkLevel string
