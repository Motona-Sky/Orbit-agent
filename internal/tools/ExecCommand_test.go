package tools

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestExecCommandReturnsCommandErrorOutput(t *testing.T) {
	const marker = "exec-command-stderr-marker"

	command := "printf '" + marker + "\n' >&2; exit 7"
	if runtime.GOOS == "windows" {
		command = "Write-Error '" + marker + "'"
	}

	_, err := execCommand(command, 5*time.Second)
	if err == nil {
		t.Fatal("execCommand() error = nil, want command error output")
	}
	if !strings.Contains(err.Error(), marker) {
		t.Fatalf("execCommand() error = %q, want marker %q", err, marker)
	}
	if strings.Contains(err.Error(), "exit status") {
		t.Fatalf("execCommand() error = %q, want command output without exit status", err)
	}
}

func TestExecCommandKeepsUnderlyingErrorWhenCommandHasNoOutput(t *testing.T) {
	_, err := execCommand("exit 7", 5*time.Second)
	if err == nil {
		t.Fatal("execCommand() error = nil, want non-empty underlying error")
	}
}

// TestExecCommandPreservesUTF8Output verifies that non-ASCII text (e.g. Chinese)
// is returned as valid UTF-8 without garbled characters.
func TestExecCommandPreservesUTF8Output(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("UTF-8 console encoding test is Windows-specific")
	}

	const marker = "你好，世界！"
	output, err := execCommand("Write-Output '"+marker+"'", 5*time.Second)
	if err != nil {
		t.Fatalf("execCommand() error = %v, want nil", err)
	}
	if !strings.Contains(output, marker) {
		t.Fatalf("execCommand() output = %q, want it to contain %q", output, marker)
	}
}
