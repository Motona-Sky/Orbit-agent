package tools

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRunToolsReturnsExecutionErrorAsToolResult(t *testing.T) {
	const toolName = "test_execution_error"
	RegToolFuncs[toolName] = ToolFunc{
		Name: toolName,
		Function: func(context.Context, []any) (string, error) {
			return "", errors.New("expected tool failure")
		},
	}
	t.Cleanup(func() { delete(RegToolFuncs, toolName) })

	results, err := RunTools(context.Background(), []any{testToolCall("call-fail", toolName)})
	if err != nil {
		t.Fatalf("RunTools() error = %v, want nil", err)
	}
	if len(results) != 1 {
		t.Fatalf("results length = %d, want 1", len(results))
	}
	result := results[0]
	if result.Role != "tool" || result.ToolCallID != "call-fail" {
		t.Fatalf("result identity = %#v", result)
	}
	if !strings.Contains(result.Content, "expected tool failure") {
		t.Fatalf("result content = %q, want tool error", result.Content)
	}
}

func TestRunToolsPreservesMixedResultOrder(t *testing.T) {
	const successTool = "test_order_success"
	const failureTool = "test_order_failure"
	releaseSuccess := make(chan struct{})

	RegToolFuncs[successTool] = ToolFunc{
		Name: successTool,
		Function: func(context.Context, []any) (string, error) {
			<-releaseSuccess
			return "success output", nil
		},
	}
	RegToolFuncs[failureTool] = ToolFunc{
		Name: failureTool,
		Function: func(context.Context, []any) (string, error) {
			close(releaseSuccess)
			return "", errors.New("failure output")
		},
	}
	t.Cleanup(func() {
		delete(RegToolFuncs, successTool)
		delete(RegToolFuncs, failureTool)
	})

	results, err := RunTools(context.Background(), []any{
		testToolCall("call-success", successTool),
		testToolCall("call-failure", failureTool),
	})
	if err != nil {
		t.Fatalf("RunTools() error = %v, want nil", err)
	}
	if len(results) != 2 {
		t.Fatalf("results length = %d, want 2", len(results))
	}
	if results[0].ToolCallID != "call-success" || results[0].Content != "success output" {
		t.Fatalf("first result = %#v", results[0])
	}
	if results[1].ToolCallID != "call-failure" || !strings.Contains(results[1].Content, "failure output") {
		t.Fatalf("second result = %#v", results[1])
	}
}

func TestRunToolsReturnsWhenStuckToolContextIsCanceled(t *testing.T) {
	const toolName = "test_context"
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	release := make(chan struct{})
	RegToolFuncs[toolName] = ToolFunc{
		Name: toolName,
		Function: func(context.Context, []any) (string, error) {
			close(started)
			<-release
			return "late result", nil
		},
	}
	t.Cleanup(func() {
		close(release)
		delete(RegToolFuncs, toolName)
	})

	done := make(chan error, 1)
	go func() {
		_, err := RunTools(ctx, []any{testToolCall("call-context", toolName)})
		done <- err
	}()
	<-started
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("RunTools() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("RunTools() did not return after context cancellation")
	}
}

func TestRunToolsStillRejectsUnregisteredTool(t *testing.T) {
	_, err := RunTools(context.Background(), []any{testToolCall("call-missing", "test_missing_tool")})
	if err == nil || !strings.Contains(err.Error(), "is not registered") {
		t.Fatalf("RunTools() error = %v, want unregistered tool error", err)
	}
}

func TestRunToolsWithRegistryValidatesBatchBeforeExecution(t *testing.T) {
	called := false
	allowed := map[string]ToolFunc{
		"allowed": {
			Name: "allowed",
			Function: func(context.Context, []any) (string, error) {
				called = true
				return "output", nil
			},
		},
	}
	_, err := RunToolsWithRegistry(context.Background(), []any{
		testToolCall("allowed-call", "allowed"),
		testToolCall("denied-call", "exec_command"),
	}, allowed)
	if err == nil || !strings.Contains(err.Error(), "is not registered") {
		t.Fatalf("error = %v, want denied tool error", err)
	}
	if called {
		t.Fatal("allowed tool executed before the entire batch was validated")
	}
}

func TestRunToolsWithRegistryRejectsCanceledContextBeforeExecution(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	allowed := map[string]ToolFunc{
		"allowed": {
			Name: "allowed",
			Function: func(context.Context, []any) (string, error) {
				t.Error("tool executed with a canceled context")
				return "", nil
			},
		},
	}
	_, err := RunToolsWithRegistry(ctx, []any{testToolCall("call", "allowed")}, allowed)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func testToolCall(id, name string) map[string]any {
	return map[string]any{
		"id": id,
		"function": map[string]any{
			"name":      name,
			"arguments": `{}`,
		},
	}
}
