package memorys

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"looporbit/internal/llm"
)

func writeSessionFixture(t *testing.T, dir, name string, messages []llm.MemoryMessage, modified time.Time) {
	t.Helper()
	data, err := json.Marshal(messages)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modified, modified); err != nil {
		t.Fatal(err)
	}
}

func TestListSessionsSortsNewestAndExtractsFirstUserMessage(t *testing.T) {
	dir := t.TempDir()
	oldTime := time.Unix(100, 0)
	newTime := time.Unix(200, 0)
	writeSessionFixture(t, dir, "old-id", []llm.MemoryMessage{
		{Role: "system", Content: "system"},
		{Role: "user", Content: "  first\n\nquestion  "},
		{Role: "user", Content: "second"},
	}, oldTime)
	writeSessionFixture(t, dir, "new-id", []llm.MemoryMessage{
		{Role: "user", Content: "newest"},
	}, newTime)

	sessions, skipped, err := ListSessions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if skipped != 0 || len(sessions) != 2 {
		t.Fatalf("sessions=%#v skipped=%d", sessions, skipped)
	}
	if sessions[0].ID != "new-id" || sessions[1].ID != "old-id" {
		t.Fatalf("order=%#v", sessions)
	}
	if sessions[1].FirstUserMessage != "first question" {
		t.Fatalf("summary=%q", sessions[1].FirstUserMessage)
	}
	if len(sessions[1].Messages) != 3 {
		t.Fatalf("messages=%#v", sessions[1].Messages)
	}
}

func TestListSessionsSkipsInvalidEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "broken"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "empty"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "blank"), []byte("  \n\t"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeSessionFixture(t, dir, "assistant-only", []llm.MemoryMessage{{Role: "assistant", Content: "hello"}}, time.Now())
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}

	sessions, skipped, err := ListSessions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 || skipped != 5 {
		t.Fatalf("sessions=%#v skipped=%d", sessions, skipped)
	}
}

func TestListSessionsMissingDirectoryIsEmpty(t *testing.T) {
	sessions, skipped, err := ListSessions(filepath.Join(t.TempDir(), "missing"))
	if err != nil || len(sessions) != 0 || skipped != 0 {
		t.Fatalf("sessions=%#v skipped=%d err=%v", sessions, skipped, err)
	}
}

func TestListSessionsUsesIDAsStableTieBreaker(t *testing.T) {
	dir := t.TempDir()
	modified := time.Unix(300, 0)
	writeSessionFixture(t, dir, "b-id", []llm.MemoryMessage{{Role: "user", Content: "b"}}, modified)
	writeSessionFixture(t, dir, "a-id", []llm.MemoryMessage{{Role: "user", Content: "a"}}, modified)

	sessions, _, err := ListSessions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if sessions[0].ID != "a-id" || sessions[1].ID != "b-id" {
		t.Fatalf("order=%#v", sessions)
	}
}
