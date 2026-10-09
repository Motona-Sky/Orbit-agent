package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"orbit/internal/agentui"
	"orbit/internal/billing"
	"orbit/internal/config"
	"orbit/internal/llm"
	"orbit/internal/tools"
	"orbit/internal/utils"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

func subagentTestRuntime() subagentRuntime {
	return subagentRuntime{
		config: RunAgentValue{
			Provider: "openai:completions", Model: "test-model", ThinkLevel: "low",
			SystemPt: "project constraints", UserInput: "private parent conversation",
		},
		sessionID: "parent-session",
		request: func(context.Context, string) (error, string) {
			return errors.New("unexpected model request"), ""
		},
		recordUsage: func(string, map[string]any) (billing.DailyUsage, error) {
			return billing.DailyUsage{}, nil
		},
	}
}

func subagentTestCall(id, name string, args any) map[string]any {
	encoded, _ := json.Marshal(args)
	return map[string]any{
		"id":       id,
		"function": map[string]any{"name": name, "arguments": string(encoded)},
	}
}

func subagentTestResponse(provider, content string, calls []any) string {
	usage := map[string]any{"prompt_tokens": 2, "completion_tokens": 1, "total_tokens": 3}
	var response any
	if provider == "openai:completions" {
		finishReason := "stop"
		if len(calls) > 0 {
			finishReason = "tool_calls"
		}
		response = map[string]any{
			"usage": usage,
			"choices": []any{map[string]any{
				"finish_reason": finishReason,
				"message":       map[string]any{"role": "assistant", "content": content, "tool_calls": calls},
			}},
		}
	} else {
		output := []any{}
		if content != "" {
			output = append(output, map[string]any{
				"type": "message", "role": "assistant",
				"content": []any{map[string]any{"type": "output_text", "text": content}},
			})
		}
		for _, value := range calls {
			call := value.(map[string]any)
			function := call["function"].(map[string]any)
			output = append(output, map[string]any{
				"type": "function_call", "call_id": call["id"],
				"name": function["name"], "arguments": function["arguments"],
			})
		}
		response = map[string]any{"status": "completed", "usage": usage, "output": output}
	}
	encoded, _ := json.Marshal(response)
	return string(encoded)
}

func invokeTestSubagent(ctx context.Context, runtime subagentRuntime, task string) (string, error) {
	call := subagentTestCall("child", "subagent", map[string]any{"task": task})
	ctx = context.WithValue(ctx, subagentContextKey{}, runtime)
	return callSubagent(ctx, []any{call["function"]})
}

func TestSubagentRegistered(t *testing.T) {
	for _, provider := range []string{"openai:completions", "openai:responses", "oauth:codex"} {
		count := 0
		for _, schema := range tools.GetAllTool(provider) {
			if schema.Function.Name == "subagent" {
				count++
				parameters := schema.Function.Parameters.(tools.ToolParameters)
				if len(parameters.Required) != 1 || parameters.Required[0] != "task" || parameters.Properties["task"].Type != "string" {
					t.Fatalf("subagent parameters = %#v", parameters)
				}
			}
		}
		if count != 1 {
			t.Fatalf("%s subagent schema count = %d, want 1", provider, count)
		}
	}
	if tools.GetEnabledTools()["subagent"].Function == nil {
		t.Fatal("subagent execution function is not registered")
	}
}

func TestSubagentValidatesArguments(t *testing.T) {
	cases := []struct {
		name string
		call []any
		want string
	}{
		{"missing call", nil, "empty"},
		{"invalid function", []any{"bad"}, "format"},
		{"missing arguments", []any{map[string]any{}}, "JSON string"},
		{"malformed JSON", []any{map[string]any{"arguments": "{"}}, "parse"},
		{"wrong task type", []any{map[string]any{"arguments": `{"task":42}`}}, "parse"},
		{"missing task", []any{map[string]any{"arguments": `{}`}}, "empty"},
		{"blank task", []any{map[string]any{"arguments": `{"task":"  "}`}}, "empty"},
		{"missing runtime", []any{map[string]any{"arguments": `{"task":"find a file"}`}}, "parent agent runtime"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := callSubagent(context.Background(), test.call)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestSubagentToolRoundTrip(t *testing.T) {
	for _, provider := range []string{"openai:completions", "openai:responses", "oauth:codex"} {
		t.Run(provider, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "observation.txt")
			if err := os.WriteFile(path, []byte("child observation"), 0600); err != nil {
				t.Fatal(err)
			}
			runtime := subagentTestRuntime()
			runtime.config.Provider = provider
			requests, recorded := 0, 0
			runtime.request = func(ctx context.Context, data string) (error, string) {
				requests++
				if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > subagentTimeout {
					t.Error("subagent request did not inherit its timeout")
				}
				var request map[string]any
				if err := json.Unmarshal([]byte(data), &request); err != nil {
					return err, ""
				}
				if request["model"] != runtime.config.Model || strings.Contains(data, runtime.config.UserInput) {
					t.Error("model was not inherited or parent conversation leaked")
				}
				if requests == 1 {
					if !strings.Contains(data, "project constraints") || !strings.Contains(data, "inspect observation") || !strings.Contains(data, `"low"`) {
						t.Error("task, system constraints or reasoning setting were not inherited")
					}
					return nil, subagentTestResponse(provider, "", []any{subagentTestCall("read-call", "read_file", map[string]any{"filepath": path})})
				}
				if !strings.Contains(data, "child observation") || !strings.Contains(data, "read-call") {
					t.Error("tool result or call ID missing from second request")
				}
				return nil, subagentTestResponse(provider, "found observation", nil)
			}
			runtime.recordUsage = func(sessionID string, usage map[string]any) (billing.DailyUsage, error) {
				recorded++
				if sessionID != runtime.sessionID || usage["total_tokens"] != float64(3) {
					t.Error("usage was not recorded against the captured parent session")
				}
				return billing.DailyUsage{}, nil
			}
			before := contextLength
			output, err := invokeTestSubagent(context.Background(), runtime, "inspect observation")
			if err != nil || output != "found observation" || requests != 2 || recorded != 2 {
				t.Fatalf("output = %q, error = %v, requests = %d, records = %d", output, err, requests, recorded)
			}
			if contextLength != before {
				t.Fatal("subagent changed parent context length")
			}
		})
	}
}

func TestSubagentUsesResolvedCredential(t *testing.T) {
	for _, provider := range []string{"openai:completions", "openai:responses"} {
		t.Run(provider, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer resolved-test-key" {
					t.Error("request did not use the captured resolved credential")
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(subagentTestResponse(provider, "done", nil)))
			}))
			defer server.Close()
			value := RunAgentValue{Provider: provider, ApiKey: "stale-test-key", Model: "test-model", BaseUrl: server.URL}
			runtime := newSubagentRuntime(value, "resolved-test-key", "captured-session")
			runtime.recordUsage = subagentTestRuntime().recordUsage
			value.BaseUrl = "http://unused.invalid"
			output, err := invokeTestSubagent(context.Background(), runtime, "summarize")
			if err != nil || output != "done" {
				t.Fatalf("output = %q, error = %v", output, err)
			}
		})
	}
}

func TestSubagentRejectsDisallowedTools(t *testing.T) {
	for _, name := range []string{"subagent", "exec_command", "update_file", "mcp__unsafe", "unknown"} {
		t.Run(name, func(t *testing.T) {
			runtime := subagentTestRuntime()
			runtime.request = func(context.Context, string) (error, string) {
				return nil, subagentTestResponse(runtime.config.Provider, "", []any{subagentTestCall("denied", name, map[string]any{})})
			}
			_, err := invokeTestSubagent(context.Background(), runtime, "inspect")
			if err == nil || !strings.Contains(err.Error(), "is not registered") {
				t.Fatalf("error = %v, want rejected %s", err, name)
			}
		})
	}
}

func TestSubagentRespectsDisabledTools(t *testing.T) {
	wasDisabled := tools.IsToolDisabled("read_file")
	tools.DisableTool("read_file")
	t.Cleanup(func() {
		if !wasDisabled {
			tools.EnableTool("read_file")
		}
	})
	schemas, allowed := subagentToolSet()
	if _, ok := allowed["read_file"]; ok {
		t.Fatal("disabled read_file is executable")
	}
	for _, schema := range schemas {
		if schema.Function.Name == "read_file" || schema.Function.Name == "subagent" {
			t.Fatal("disabled or recursive tool was advertised")
		}
	}
	runtime := subagentTestRuntime()
	runtime.request = func(context.Context, string) (error, string) {
		return nil, subagentTestResponse(runtime.config.Provider, "", []any{subagentTestCall("disabled", "read_file", map[string]any{})})
	}
	if _, err := invokeTestSubagent(context.Background(), runtime, "inspect"); err == nil {
		t.Fatal("disabled tool was accepted")
	}
}

func TestSubagentIgnoresMcpNameCollision(t *testing.T) {
	old, existed := tools.RegMcpToolFuncs["read_file"]
	called := false
	tools.RegisterMcpToolSet(map[string]tools.ToolFunc{
		"read_file": {Name: "read_file", Function: func(context.Context, []any) (string, error) {
			called = true
			return "unsafe MCP output", nil
		}},
	}, nil)
	t.Cleanup(func() {
		if existed {
			tools.RegisterMcpToolSet(map[string]tools.ToolFunc{"read_file": old}, nil)
		} else {
			tools.RemoveMcpTools([]string{"read_file"})
		}
	})
	path := filepath.Join(t.TempDir(), "safe.txt")
	if err := os.WriteFile(path, []byte("safe local output"), 0600); err != nil {
		t.Fatal(err)
	}
	_, allowed := subagentToolSet()
	results, err := tools.RunToolsWithRegistry(context.Background(), []any{subagentTestCall("read", "read_file", map[string]any{"filepath": path})}, allowed)
	if err != nil || len(results) != 1 || results[0].Content != "safe local output" || called {
		t.Fatalf("results = %#v, error = %v, MCP called = %v", results, err, called)
	}
}

func TestSubagentLimitsAndErrors(t *testing.T) {
	cases := []struct {
		name, response, want string
	}{
		{"malformed", "not JSON", "parse subagent response"},
		{"empty", subagentTestResponse("openai:completions", " ", nil), "empty"},
		{"truncated", `{"usage":{},"choices":[{"finish_reason":"length","message":{"role":"assistant","content":"partial"}}]}`, "token limit"},
		{"incomplete", `{"status":"incomplete","usage":{},"incomplete_details":{},"output":[]}`, "incomplete"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			runtime := subagentTestRuntime()
			if test.name == "incomplete" {
				runtime.config.Provider = "openai:responses"
			}
			runtime.request = func(context.Context, string) (error, string) { return nil, test.response }
			_, err := invokeTestSubagent(context.Background(), runtime, "inspect")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
	t.Run("request failure", func(t *testing.T) {
		runtime := subagentTestRuntime()
		want := errors.New("fake network failure")
		runtime.request = func(context.Context, string) (error, string) { return want, "" }
		if _, err := invokeTestSubagent(context.Background(), runtime, "inspect"); !errors.Is(err, want) {
			t.Fatalf("error = %v, want network failure", err)
		}
	})
	t.Run("iteration limit", func(t *testing.T) {
		runtime := subagentTestRuntime()
		requests := 0
		path := t.TempDir()
		runtime.request = func(context.Context, string) (error, string) {
			requests++
			return nil, subagentTestResponse(runtime.config.Provider, "", []any{subagentTestCall("list", "ls", map[string]any{"path": path, "recursive": false})})
		}
		_, err := invokeTestSubagent(context.Background(), runtime, "inspect")
		if err == nil || !strings.Contains(err.Error(), "maximum") || requests != maxSubagentIterations {
			t.Fatalf("error = %v, requests = %d", err, requests)
		}
	})
}

func TestSubagentRecordsUsageOnlyOnceOnQueryFailure(t *testing.T) {
	runtime := subagentTestRuntime()
	runtime.request = func(context.Context, string) (error, string) {
		return nil, subagentTestResponse(runtime.config.Provider, "done", nil)
	}
	recorded := 0
	runtime.recordUsage = func(string, map[string]any) (billing.DailyUsage, error) {
		recorded++
		return billing.DailyUsage{}, billing.ErrDailyUsageQuery
	}
	output, err := invokeTestSubagent(context.Background(), runtime, "inspect")
	if err != nil || output != "done" || recorded != 1 {
		t.Fatalf("output = %q, error = %v, records = %d", output, err, recorded)
	}
}

func TestSubagentCancellationReachesRequest(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(fmt.Sprintf("deadline=%t", deadline), func(t *testing.T) {
			started, stop := make(chan struct{}), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				close(started)
				select {
				case <-r.Context().Done():
				case <-stop:
				}
			}))
			defer func() { close(stop); server.Close() }()
			runtime := newSubagentRuntime(RunAgentValue{Provider: "openai:completions", Model: "test-model", BaseUrl: server.URL}, "test-key", "session")
			runtime.recordUsage = subagentTestRuntime().recordUsage
			ctx, cancel := context.WithCancel(context.Background())
			if deadline {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 200*time.Millisecond)
			}
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := invokeTestSubagent(ctx, runtime, "inspect")
				done <- err
			}()
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("request did not start")
			}
			want := context.DeadlineExceeded
			if !deadline {
				want = context.Canceled
				cancel()
			}
			select {
			case err := <-done:
				if !errors.Is(err, want) {
					t.Fatalf("error = %v, want %v", err, want)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("subagent did not stop after cancellation")
			}
		})
	}
}

func TestConcurrentSubagentsHaveIndependentMemory(t *testing.T) {
	var wait sync.WaitGroup
	for _, task := range []string{"task-alpha", "task-beta"} {
		wait.Add(1)
		go func(task string) {
			defer wait.Done()
			runtime := subagentTestRuntime()
			runtime.config.Model = task + "-model"
			runtime.sessionID = task + "-session"
			runtime.request = func(_ context.Context, data string) (error, string) {
				other := "task-alpha"
				if task == other {
					other = "task-beta"
				}
				if !strings.Contains(data, task) || strings.Contains(data, other) {
					t.Error("concurrent subagent context was mixed")
				}
				return nil, subagentTestResponse(runtime.config.Provider, task+" result", nil)
			}
			runtime.recordUsage = func(sessionID string, _ map[string]any) (billing.DailyUsage, error) {
				if sessionID != task+"-session" {
					t.Error("concurrent usage session was mixed")
				}
				return billing.DailyUsage{}, nil
			}
			output, err := invokeTestSubagent(context.Background(), runtime, task)
			if err != nil || output != task+" result" {
				t.Errorf("output = %q, error = %v", output, err)
			}
		}(task)
	}
	wait.Wait()
}

func TestLimitSubagentOutputPreservesUTF8(t *testing.T) {
	input := strings.Repeat("中文", maxSubagentOutput)
	output := limitSubagentOutput(input)
	if len(output) > maxSubagentOutput || !utf8.ValidString(output) || !strings.HasSuffix(output, "[output truncated]") {
		t.Fatalf("truncated output length = %d, valid UTF-8 = %v", len(output), utf8.ValidString(output))
	}
	if limitSubagentOutput("short") != "short" {
		t.Fatal("short output changed")
	}
	if output := limitSubagentOutput(strings.Repeat("\x80", maxSubagentOutput+1)); len(output) > maxSubagentOutput {
		t.Fatal("invalid UTF-8 output exceeded the limit")
	}
}

func TestRunAgentDelegatesWithoutSavingChildHistory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if _, err := config.CreateConfigFolder(); err != nil {
		t.Fatal(err)
	}
	oldSession, oldHistory, oldMode, oldContext := utils.SessionId, utils.ChatHistoryFolder, utils.Mode, contextLength
	utils.SessionId, utils.ChatHistoryFolder, utils.Mode = "0000-1111-2222-3333", t.TempDir(), utils.ModeAuto
	t.Cleanup(func() {
		utils.SessionId, utils.ChatHistoryFolder, utils.Mode, contextLength = oldSession, oldHistory, oldMode, oldContext
	})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []llm.MemoryMessage `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		var response string
		switch requests.Add(1) {
		case 1:
			response = subagentTestResponse("openai:completions", "", []any{subagentTestCall("delegate", "subagent", map[string]any{"task": "child-only task"})})
		case 2:
			if len(body.Messages) != 2 || body.Messages[1].Content != "child-only task" {
				t.Error("child inherited parent history")
			}
			response = subagentTestResponse("openai:completions", "child-only answer", nil)
		case 3:
			if len(body.Messages) == 0 || body.Messages[len(body.Messages)-1].ToolCallID != "delegate" || body.Messages[len(body.Messages)-1].Content != "child-only answer" {
				t.Error("child answer was not returned as a parent tool result")
			}
			response = subagentTestResponse("openai:completions", "parent final answer", nil)
		default:
			t.Error("unexpected extra request")
			response = subagentTestResponse("openai:completions", "unexpected", nil)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()
	ui := agentui.New()
	defer ui.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := RunAgent(ctx, RunAgentValue{
		Provider: "openai:completions", ApiKey: "test-key", BaseUrl: server.URL,
		Model: "test-model", SystemPt: "project constraints", UserInput: "parent-only task",
	}, ui); err != nil {
		t.Fatal(err)
	}
	const barrier = "test-drain-barrier"
	if err := ui.DisplayResult(barrier); err != nil {
		t.Fatal(err)
	}
	finals := 0
	for {
		event, err := ui.Next()
		if err != nil {
			t.Fatal(err)
		}
		if result, ok := event.(agentui.ResultEvent); ok && result.Text == barrier {
			break
		}
		if final, ok := event.(agentui.FinalResultEvent); ok {
			finals++
			if final.Text != "parent final answer" {
				t.Fatalf("unexpected final event: %#v", final)
			}
		}
	}
	if finals != 1 || requests.Load() != 3 {
		t.Fatalf("final events = %d, requests = %d", finals, requests.Load())
	}
	history, err := os.ReadFile(filepath.Join(utils.ChatHistoryFolder, utils.SessionId))
	if err != nil {
		t.Fatal(err)
	}
	var memory []llm.MemoryMessage
	if err := json.Unmarshal(history, &memory); err != nil {
		t.Fatal(err)
	}
	if len(memory) != 5 {
		t.Fatalf("parent history has %d messages, want 5 without child messages", len(memory))
	}
	files, err := os.ReadDir(utils.ChatHistoryFolder)
	if err != nil || len(files) != 1 {
		t.Fatalf("history files = %v, error = %v", files, err)
	}
	usage, err := billing.QueryTodayUsage()
	if err != nil || usage.TotalTokens != 9 {
		t.Fatalf("usage = %#v, error = %v, want all three requests counted once", usage, err)
	}
}
