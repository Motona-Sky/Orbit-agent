package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"orbit/internal/utils"
	"os/exec"
	"strings"
	"time"
)

const (
	defaultExecCommandTimeout = 300 * time.Second
	maxExecCommandTimeout     = 600 * time.Second
)

func ExecCommand(command string) (string, error) {
	return execCommand(command, defaultExecCommandTimeout)
}

func execCommand(command string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cmd *exec.Cmd
	switch utils.GetSystemVersion() {
	case "windows":
		// Force UTF-8 output encoding so non-ASCII text (e.g. Chinese) is not garbled.
		// -NoProfile skips the user profile, so we must set the encoding per command.
		utf8Setup := "[Console]::OutputEncoding=[System.Text.Encoding]::UTF8;$OutputEncoding=[System.Text.Encoding]::UTF8;"
		cmd = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", utf8Setup+command)
	case "darwin":
		cmd = exec.CommandContext(ctx, "/bin/zsh", "-c", command)
	default:
		cmd = exec.CommandContext(ctx, "/bin/bash", "-c", command)
	}

	output, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("command timed out after %s: %w", timeout, context.DeadlineExceeded)
	}
	if err != nil {
		commandError := strings.TrimSpace(string(output))
		if commandError != "" {
			return "", errors.New(commandError)
		}
		return "", err
	}
	return string(output), nil

}

func CallExecCommandFunc(_ context.Context, jsonstr []any) (string, error) {
	if len(jsonstr) == 0 {
		return "", fmt.Errorf("tool calls is empty")
	}
	function, ok := jsonstr[0].(map[string]any)
	if !ok {
		return "", fmt.Errorf("json is not map[string]any")
	}
	arguments, ok := function["arguments"].(string)
	if !ok {
		return "", fmt.Errorf("arguments is not string")
	}
	command, timeout, err := parseExecCommandArguments(arguments)
	if err != nil {
		return "", err
	}
	return execCommand(command, timeout)
}

func parseExecCommandArguments(arguments string) (string, time.Duration, error) {
	var args struct {
		Command        string `json:"command"`
		TimeoutSeconds *int   `json:"timeout_seconds"`
	}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", 0, err
	}

	timeout := defaultExecCommandTimeout
	if args.TimeoutSeconds != nil {
		if *args.TimeoutSeconds < 1 || *args.TimeoutSeconds > int(maxExecCommandTimeout/time.Second) {
			return "", 0, fmt.Errorf("timeout_seconds must be between 1 and %d", int(maxExecCommandTimeout/time.Second))
		}
		timeout = time.Duration(*args.TimeoutSeconds) * time.Second
	}
	return args.Command, timeout, nil
}

// 提示词
var ExecCommandpDescription = fmt.Sprintf("Execute a command in the shell and return combined stdout/stderr. Use for builds, tests, git, package managers, etc. For searching, reading, listing, editing, and moving files, prefer dedicated tools (grep, read_file, ls, glob, edit_file, move_file) over using the shell, because these tools behave identically on every OS. For symbol searches or architecture questions, prefer LSP/read tools and targeted grep over shell commands.\nCurrent system:%s", GetShellVersion())

func GetShellVersion() string {
	switch utils.GetSystemVersion() {
	case "windows":
		return "powershell.exe"
	case "darwin":
		return "/bin/zsh"
	default:
		return "/bin/bash"
	}
}
func GetExecCommandToolParameters() ToolParameters {
	command := ToolPropertiesArray{
		Type:        "string",
		Description: "Shell command to execute",
	}
	timeoutSeconds := ToolPropertiesArray{
		Type:        "integer",
		Description: "Command timeout in seconds (default 300, range 1-600)",
		Minimum:     utils.IntPtr(1),
		Maximum:     utils.IntPtr(600),
	}
	ToolProperties := map[string]ToolPropertiesArray{
		"command":         command,
		"timeout_seconds": timeoutSeconds,
	}
	return ToolParameters{
		Type:                 "object",
		Properties:           ToolProperties,
		Required:             []string{"command"},
		AdditionalProperties: false,
	}
}

func init() {
	ExecCommandparametersJSON := GetExecCommandToolParameters()
	RegTools([]ToolReg{
		{
			Type: "function",
			Function: ToolFunction{
				Name:        "exec_command",
				Description: ExecCommandpDescription,
				Parameters:  ExecCommandparametersJSON,
			},
		},
	})
	RegToolFuncs["exec_command"] = ToolFunc{
		Name:     "exec_command",
		Function: CallExecCommandFunc,
	}
}
