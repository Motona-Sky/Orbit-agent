package cli

import (
	"errors"
	"testing"

	"orbit/internal/memorys"
)

func TestSessionCommandAliasesUseSameFlow(t *testing.T) {
	for _, arg := range []string{"-s", "--session", "session"} {
		called := 0
		deps := runDependencies{
			openChat:      func() error { t.Fatal("opened new chat"); return nil },
			openSession:   func() error { called++; return nil },
			createConfig:  func() error { return nil },
			runModelSetup: func() error { return nil },
		}
		if code := runWithDependencies([]string{arg}, deps); code != 0 || called != 1 {
			t.Fatalf("arg=%q code=%d called=%d", arg, code, called)
		}
	}
}

func TestSessionFlowCancelDoesNotOpenChat(t *testing.T) {
	opened := false
	deps := sessionFlowDependencies{
		loadConfig: func() (string, error) { return "zh-CN", nil },
		listSessions: func() ([]memorys.SessionSummary, int, error) {
			return []memorys.SessionSummary{{ID: "id"}}, 0, nil
		},
		selectSession: func(string, []memorys.SessionSummary, int) (memorys.SessionSummary, bool, error) {
			return memorys.SessionSummary{}, false, nil
		},
		openChat: func(string, memorys.SessionSummary) error { opened = true; return nil },
	}
	if err := openSessionFlow(deps); err != nil || opened {
		t.Fatalf("opened=%v err=%v", opened, err)
	}
}

func TestSessionFlowOpensSelectedSession(t *testing.T) {
	var selected string
	deps := sessionFlowDependencies{
		loadConfig: func() (string, error) { return "zh-CN", nil },
		listSessions: func() ([]memorys.SessionSummary, int, error) {
			return []memorys.SessionSummary{{ID: "id"}}, 2, nil
		},
		selectSession: func(language string, sessions []memorys.SessionSummary, skipped int) (memorys.SessionSummary, bool, error) {
			if language != "zh-CN" || skipped != 2 {
				t.Fatalf("language=%q skipped=%d", language, skipped)
			}
			return sessions[0], true, nil
		},
		openChat: func(language string, session memorys.SessionSummary) error {
			selected = session.ID
			return nil
		},
	}
	if err := openSessionFlow(deps); err != nil || selected != "id" {
		t.Fatalf("selected=%q err=%v", selected, err)
	}
}

func TestSessionCommandReturnsFailureOnFlowError(t *testing.T) {
	deps := runDependencies{
		openChat:      func() error { return nil },
		openSession:   func() error { return errors.New("cannot read sessions") },
		createConfig:  func() error { return nil },
		runModelSetup: func() error { return nil },
	}
	if code := runWithDependencies([]string{"session"}, deps); code != 1 {
		t.Fatalf("code=%d", code)
	}
}

func TestDebugFlagEnablesDebugAndKeepsCommandRouting(t *testing.T) {
	for _, args := range [][]string{{"--debug"}, {"--debug", "session"}, {"session", "--debug"}} {
		debugEnabled := 0
		chatOpened := 0
		sessionOpened := 0
		deps := runDependencies{
			openChat:      func() error { chatOpened++; return nil },
			openSession:   func() error { sessionOpened++; return nil },
			createConfig:  func() error { return nil },
			runModelSetup: func() error { return nil },
			enableDebug:   func() { debugEnabled++ },
		}
		if code := runWithDependencies(args, deps); code != 0 {
			t.Fatalf("args=%v code=%d", args, code)
		}
		if debugEnabled != 1 {
			t.Fatalf("args=%v debugEnabled=%d", args, debugEnabled)
		}
		if len(args) == 1 && chatOpened != 1 {
			t.Fatalf("args=%v chatOpened=%d", args, chatOpened)
		}
		if len(args) == 2 && sessionOpened != 1 {
			t.Fatalf("args=%v sessionOpened=%d", args, sessionOpened)
		}
	}
}
