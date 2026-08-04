package event

import (
	"testing"

	"orbit/internal/utils"
)

func preserveSessionState(t *testing.T) {
	t.Helper()
	oldInit, oldID, oldCwd := chatTuiInit, utils.SessionId, utils.Cwd
	t.Cleanup(func() {
		chatTuiInit, utils.SessionId, utils.Cwd = oldInit, oldID, oldCwd
	})
}

func TestResumeChatTuiEventUsesExistingSessionID(t *testing.T) {
	preserveSessionState(t)
	got := ResumeChatTuiEvent("workspace", "existing-id")
	if got.SessionId != "existing-id" || chatTuiInit.SessionId != "existing-id" {
		t.Fatalf("state=%#v global=%#v", got, chatTuiInit)
	}
	if utils.SessionId != "existing-id" || utils.Cwd != "workspace" {
		t.Fatalf("session=%q cwd=%q", utils.SessionId, utils.Cwd)
	}
}

func TestResumeChatTuiEventRejectsEmptySessionID(t *testing.T) {
	preserveSessionState(t)
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	ResumeChatTuiEvent("workspace", "")
}
