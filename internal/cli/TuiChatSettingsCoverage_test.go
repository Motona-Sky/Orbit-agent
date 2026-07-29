package cli

import (
	"errors"
	"testing"

	"looporbit/internal/agentui"
)

func TestEnglishSlashCommandsDescribeSeparateResponsibilities(t *testing.T) {
	commands := NewModelForLanguage("en").slashCommands()
	if commands[0].Description != "Select a configured model" {
		t.Fatalf("/model description = %q", commands[0].Description)
	}
	if commands[1].Description != "Configure model providers" {
		t.Fatalf("/provider description = %q", commands[1].Description)
	}
	if commands[3].Description != "List or invoke an available skill" {
		t.Fatalf("/skills description = %q", commands[3].Description)
	}
}

func TestQueuedProviderSetupOpensAfterEveryTurnEnding(t *testing.T) {
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
			name: "user cancel",
			end: func(m model) model {
				m.agentUI = agentui.New()
				updated, _ := m.handleCtrlC()
				return updated
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeModelSelectionConfig(t)
			m := NewModelForLanguage("zh-CN")
			m.running = true
			m, _ = m.handleComposerSubmitValue("/provider")

			m = tt.end(m)

			if m.providerSetup == nil || m.modelSetup != nil || m.pendingInput != "" {
				t.Fatalf("pending provider setup was not consumed: %#v", m)
			}

		})
	}
}
