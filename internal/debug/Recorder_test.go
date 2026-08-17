package debug

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStartSessionWritesJSONLinesFile(t *testing.T) {
	resetRecorder(t)
	Enable()
	dir := t.TempDir()
	if err := StartSession(dir, "session-id"); err != nil {
		t.Fatal(err)
	}
	Record("message", map[string]string{"text": "hello"})

	data, err := os.ReadFile(filepath.Join(dir, "dbg-session-id"))
	if err != nil {
		t.Fatal(err)
	}
	var records []record
	for _, line := range splitLines(data) {
		var value record
		if err := json.Unmarshal(line, &value); err != nil {
			t.Fatalf("unmarshal %q: %v", line, err)
		}
		records = append(records, value)
	}
	if len(records) != 2 || records[0].Type != "session_start" || records[1].Type != "message" {
		t.Fatalf("records=%#v", records)
	}
	if records[1].SessionID != "session-id" {
		t.Fatalf("session_id=%q", records[1].SessionID)
	}
}

func resetRecorder(t *testing.T) {
	t.Helper()
	mu.Lock()
	oldEnabled, oldFile := enabled.Load(), file
	enabled.Store(false)
	file = nil
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		if file != nil {
			_ = file.Close()
		}
		file = oldFile
		enabled.Store(oldEnabled)
		mu.Unlock()
	})
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for index, value := range data {
		if value == '\n' {
			lines = append(lines, data[start:index])
			start = index + 1
		}
	}
	return lines
}
