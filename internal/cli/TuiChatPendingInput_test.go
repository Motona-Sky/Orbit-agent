package cli

import (
	"errors"
	"strings"
	"testing"

	"looporbit/internal/agentui"

	"github.com/charmbracelet/x/ansi"
)

func TestRunningTurnStoresOnlyOnePendingInput(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.running = true

	m, firstCmd := m.handleComposerSubmitValue("检查第二个目录")
	m, secondCmd := m.handleComposerSubmitValue("这条应被忽略")

	if firstCmd != nil || secondCmd != nil {
		t.Fatal("running turn must not start another command immediately")
	}
	if m.pendingInput != "检查第二个目录" {
		t.Fatalf("pendingInput = %q", m.pendingInput)
	}
}

func TestPendingInputUsesVisibleUserMessageStyle(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.pendingInput = "/model"

	rendered := ansi.Strip(m.renderPendingInput(80))
	if !strings.Contains(rendered, m.messages.Chat.YouLabel) ||
		!strings.Contains(rendered, "> /model") ||
		strings.Contains(rendered, "待执行") {
		t.Fatalf("pending input = %q", rendered)
	}
}

func TestShortScreenShowsPendingInputAboveComposer(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.running = true
	m.pendingInput = "/skills genimages 生成猫图"
	m.width = 50
	m.height = 2

	view := ansi.Strip(m.renderShortScreenChat(terminalContentWidth(m.width), m.height))
	lines := strings.Split(view, "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "/skills genimages") ||
		!strings.Contains(lines[1], m.messages.Chat.AgentCommandPrompt) {
		t.Fatalf("short-screen view = %q", view)
	}
}

func TestPendingInputRunsAfterEveryTurnEnding(t *testing.T) {
	tests := []struct {
		name string
		end  func(model) model
	}{
		{
			name: "result",
			end: func(m model) model {
				updated, _ := m.Update(agentui.ResultEvent{Text: "done"})
				return updated.(model)
			},
		},
		{
			name: "runner error",
			end: func(m model) model {
				ui := agentui.New()
				m.agentUI = ui
				updated, _ := m.Update(agentRunFinishedMsg{ui: ui, err: errors.New("boom")})
				return updated.(model)
			},
		},
		{
			name: "cancel",
			end: func(m model) model {
				m.agentUI = agentui.New()
				updated, _ := m.handleCtrlC()
				return updated
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			writeModelSetupConfig(t)
			m := NewModelForLanguage("zh-CN")
			m.running = true
			m.pendingInput = "/model"

			m = test.end(m)

			if m.pendingInput != "" || m.modelSetup == nil {
				t.Fatalf("pending input was not consumed: %#v", m)
			}
		})
	}
}

func TestPendingNormalInputStartsNextTurn(t *testing.T) {
	m := NewModelForLanguage("zh-CN")
	m.running = true
	m.pendingInput = "继续检查"

	updated, cmd := m.handleAgentUIEvent(agentui.ResultEvent{Text: "done"})
	m = updated.(model)
	defer m.closeAgentUI()

	if m.pendingInput != "" || !m.running || cmd == nil {
		t.Fatalf("next turn state = %#v, cmd nil = %v", m, cmd == nil)
	}
	if got := m.transcript[len(m.transcript)-1]; got.role != "user" || got.content != "继续检查" {
		t.Fatalf("last transcript = %#v", got)
	}
}
