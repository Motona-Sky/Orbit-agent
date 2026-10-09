package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"orbit/internal/billing"
	"orbit/internal/llm"
	"orbit/internal/memorys"
	"orbit/internal/tools"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxSubagentIterations = 10
	subagentTimeout       = 2 * time.Minute
	maxSubagentOutput     = 32 * 1024
	subagentPrompt        = "You are a read-only subagent handling a delegated task. You do not have the parent conversation; use only the task and the available read_file, ls, and Grep tools. Do not modify files, execute commands, or delegate to another agent. Return a concise final answer with relevant file paths and findings."
)

type subagentContextKey struct{}

type subagentRuntime struct {
	config      RunAgentValue
	request     func(context.Context, string) (error, string)
	recordUsage func(string, map[string]any) (billing.DailyUsage, error)
	sessionID   string
}

// 捕获父轮实际使用的凭据；OAuth 刷新后的 credential 可能不同于配置中的 AccessToken。
func newSubagentRuntime(value RunAgentValue, credential, sessionID string) subagentRuntime {
	return subagentRuntime{
		config:    value,
		sessionID: sessionID,
		request: func(ctx context.Context, data string) (error, string) {
			if value.Auth == "codex" {
				return llm.RequOuthStream(ctx, credential, value.AccountID, value.Provider, data, nil)
			}
			return llm.RequProvider(ctx, credential, value.BaseUrl, value.Provider, data)
		},
		recordUsage: billing.InsertCostData,
	}
}

func callSubagent(ctx context.Context, jsonstr []any) (string, error) {
	if len(jsonstr) == 0 {
		return "", errors.New("subagent tool call is empty")
	}
	function, ok := jsonstr[0].(map[string]any)
	if !ok {
		return "", errors.New("subagent function format is invalid")
	}
	arguments, ok := function["arguments"].(string)
	if !ok {
		return "", errors.New("subagent arguments must be a JSON string")
	}
	var args struct {
		Task string `json:"task"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("parse subagent arguments: %w", err)
	}
	if strings.TrimSpace(args.Task) == "" {
		return "", errors.New("subagent task must not be empty")
	}
	runtime, ok := ctx.Value(subagentContextKey{}).(subagentRuntime)
	if !ok {
		return "", errors.New("subagent requires a parent agent runtime")
	}
	ctx, cancel := context.WithTimeout(ctx, subagentTimeout)
	defer cancel()
	return runSubagent(ctx, runtime, args.Task)
}

// 只使用本地函数，避免同名 MCP 工具覆盖只读工具的执行入口。
func subagentToolSet() ([]tools.ToolReg, map[string]tools.ToolFunc) {
	local := map[string]tools.ToolFunc{
		"read_file": {Name: "read_file", Function: tools.CallReadFIleFunc},
		"ls":        {Name: "ls", Function: tools.CallLsFunc},
		"Grep":      {Name: "Grep", Function: tools.CallGrepFunc},
	}
	var schemas []tools.ToolReg
	for _, name := range []string{"read_file", "ls", "Grep"} {
		if tools.IsToolDisabled(name) {
			delete(local, name)
			continue
		}
		schemas = append(schemas, tools.SpecificTool(name))
	}
	return schemas, local
}

func runSubagent(ctx context.Context, runtime subagentRuntime, task string) (string, error) {
	req := llm.GenReqJsonPvRequest{
		Provider:     runtime.config.Provider,
		Model:        runtime.config.Model,
		ThinkLevel:   runtime.config.ThinkLevel,
		SystemPrompt: runtime.config.SystemPt + "\n\n" + subagentPrompt,
		UserInput:    task,
	}
	for iteration := 0; iteration < maxSubagentIterations; iteration++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		schemas, allowed := subagentToolSet()
		inputMemory, data, err := llm.GenReqJsonPv(req, schemas)
		if err != nil {
			return "", fmt.Errorf("generate subagent request: %w", err)
		}
		requestErr, responseJSON := runtime.request(ctx, data)
		if requestErr != nil {
			return "", fmt.Errorf("subagent request: %w", requestErr)
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		calls, assistantMemory, usage, state, err := ParseResponseDetails(req.Provider, responseJSON)
		if err != nil {
			return "", fmt.Errorf("parse subagent response: %w", err)
		}
		if _, err := runtime.recordUsage(runtime.sessionID, usage); err != nil && !errors.Is(err, billing.ErrDailyUsageQuery) {
			return "", fmt.Errorf("record subagent usage: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if err := state.validate(calls); err != nil {
			return "", err
		}
		if len(calls) == 0 {
			if len(assistantMemory) == 0 || strings.TrimSpace(assistantMemory[0].Content) == "" {
				return "", errors.New("subagent final answer is empty")
			}
			return limitSubagentOutput(assistantMemory[0].Content), nil
		}
		for name := range allowed {
			if tools.IsToolDisabled(name) {
				delete(allowed, name)
			}
		}
		results, err := tools.RunToolsWithRegistry(ctx, calls, allowed)
		if err != nil {
			return "", fmt.Errorf("run subagent tools: %w", err)
		}
		for index := range results {
			results[index].Content = limitSubagentOutput(results[index].Content)
		}
		req.Memory = memorys.AppendMemoryMessages(inputMemory, assistantMemory)
		req.Memory = memorys.AppendMemoryMessages(req.Memory, llm.GettoolCallMemory(results))
		req.SystemPrompt = ""
		req.UserInput = ""
	}
	return "", fmt.Errorf("subagent stopped after reaching the maximum of %d iterations", maxSubagentIterations)
}

func limitSubagentOutput(output string) string {
	if len(output) <= maxSubagentOutput {
		return output
	}
	const suffix = "\n[output truncated]"
	end := maxSubagentOutput - len(suffix)
	for end > 0 && !utf8.RuneStart(output[end]) {
		end--
	}
	return output[:end] + suffix
}

func init() {
	tools.RegTools([]tools.ToolReg{{
		Type: "function",
		Function: tools.ToolFunction{
			Name:        "subagent",
			Description: "Delegate a self-contained read-only research task to a subagent using the current model. Include the necessary context and paths; the parent conversation is not shared. Returns the final answer synchronously. Cannot edit files or execute commands.",
			Parameters: tools.ToolParameters{
				Type: "object",
				Properties: map[string]tools.ToolPropertiesArray{
					"task": {Type: "string", Description: "A clear, self-contained task with the context and file paths needed by the subagent."},
				},
				Required:             []string{"task"},
				AdditionalProperties: false,
			},
		},
	}})
	tools.RegToolFuncs["subagent"] = tools.ToolFunc{Name: "subagent", Function: callSubagent}
}
