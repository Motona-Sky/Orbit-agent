package cli

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestConfigModelUsesFetchedModelsAndAppendsCustom(t *testing.T) {
	m := initialConfigModelForLanguage("zh-CN", "gpt-b")
	m.setModels([]string{"gpt-b", "gpt-a"})

	got := make([]string, 0, len(m.options))
	for _, option := range m.options {
		got = append(got, option.Model)
	}
	if want := []string{"gpt-b", "gpt-a", ""}; !reflect.DeepEqual(got, want) {
		t.Fatalf("models = %#v, want %#v", got, want)
	}
	if m.cursor != 0 || !m.options[2].Custom {
		t.Fatalf("model state = %#v", m)
	}
	if view := m.View(); !strings.Contains(view, "gpt-a") || !strings.Contains(view, "自定义") {
		t.Fatalf("view = %q", view)
	}
}

func TestConfigModelConfirmsFetchedModel(t *testing.T) {
	m := initialConfigModelForLanguage("zh-CN", "")
	m.setModels([]string{"gpt-5"})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(ConfigModelModel)
	if !got.Confirmed || got.SelectedModel != "gpt-5" {
		t.Fatalf("model = %#v", got)
	}
}

func TestConfigModelCustomRequiresNonEmptyInput(t *testing.T) {
	m := initialConfigModelForLanguage("zh-CN", "")
	m.setModels(nil)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(ConfigModelModel)
	if m.step != modelStepCustom || m.Confirmed {
		t.Fatalf("custom step = %#v", m)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(ConfigModelModel)
	if m.Confirmed {
		t.Fatal("empty custom model confirmed")
	}

	m.modelInput.SetValue("  my-model  ")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(ConfigModelModel)
	if !m.Confirmed || m.SelectedModel != "my-model" {
		t.Fatalf("custom model = %#v", m)
	}
}
