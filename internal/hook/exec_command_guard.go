package hook

import (
	"encoding/json"
	"fmt"
	"orbit/internal/config"
	"orbit/internal/i18n"
	"orbit/internal/tools"
)

const (
	// EventExecCommandBefore 在 exec_command 执行前触发的拦截事件名。
	// Data 中包含 "command" 字段。
	EventExecCommandBefore = "exec_command.before"
)

// GuardResult 是 GuardToolCalls 的返回结果。
type GuardResult struct {
	ApprovedCalls []any              // 用户允许执行的 tool calls
	DeniedResults []tools.ToolResult // 用户拒绝执行的 tool calls 对应的结果
}

// AskFunc 是用户确认回调，接收问题和选项列表，返回用户选择的选项。
type AskFunc func(question string, options []string) (string, error)

// RegisterExecCommandAsk 将 askFn（通常是 ui.Ask）注册为 exec_command.before 的拦截器。
// 每次 exec_command 执行前会弹出确认，让用户选择允许或拒绝。
func RegisterExecCommandAsk(askFn AskFunc) {
	langCode := i18n.DefaultLanguage
	if appCfg, err := config.LoadAppConfig(); err == nil && appCfg.Language != "" {
		langCode = appCfg.Language
	}
	msgs := i18n.For(langCode).Messages.Chat

	OnFilter(EventExecCommandBefore, func(ev Event) (FilterResult, error) {
		command, _ := ev.Data["command"].(string)
		question := fmt.Sprintf(msgs.ExecCommandConfirm, command)
		answer, err := askFn(question, []string{msgs.ExecCommandAllow, msgs.ExecCommandDeny})
		if err != nil {
			return FilterResult{}, err
		}
		if answer == msgs.ExecCommandAllow {
			return FilterResult{Allowed: true}, nil
		}
		return FilterResult{Allowed: false, Reason: msgs.ExecCommandDenied}, nil
	})
}

// GuardToolCalls 遍历 toolCalls，对 exec_command 类型的调用触发 exec_command.before 拦截事件。
// 无 filter 注册时默认全部通过。
func GuardToolCalls(toolCalls []any) (*GuardResult, error) {
	result := &GuardResult{}
	for idx, tc := range toolCalls {
		call, ok := tc.(map[string]any)
		if !ok {
			result.ApprovedCalls = append(result.ApprovedCalls, tc)
			continue
		}
		fn, _ := call["function"].(map[string]any)
		name, _ := fn["name"].(string)
		if name != "exec_command" {
			result.ApprovedCalls = append(result.ApprovedCalls, tc)
			continue
		}
		// Extract command content
		argsStr, _ := fn["arguments"].(string)
		var cmdArgs struct {
			Command string `json:"command"`
		}
		_ = json.Unmarshal([]byte(argsStr), &cmdArgs)

		// 触发拦截型 hook
		fr, err := RunFilter(EventExecCommandBefore, map[string]any{
			"command": cmdArgs.Command,
		})
		if err != nil {
			return nil, err
		}
		if fr.Allowed {
			result.ApprovedCalls = append(result.ApprovedCalls, tc)
		} else {
			callID, _ := call["id"].(string)
			result.DeniedResults = append(result.DeniedResults, tools.ToolResult{
				Role:       "tool",
				ToolCallID: callID,
				Content:    fr.Reason,
				Index:      idx,
			})
		}
	}
	return result, nil
}
