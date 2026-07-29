package cli

import (
	"strings"
	"testing"

	"looporbit/internal/skills"

	tea "github.com/charmbracelet/bubbletea"
)

func setTestSkills(t *testing.T, values []skills.SkillsList) {
	t.Helper()
	old := skills.SkillsLists
	skills.SkillsLists = values
	t.Cleanup(func() { skills.SkillsLists = old })
}

func TestSkillsListUsesInitializedSkillsLists(t *testing.T) {
	setTestSkills(t, []skills.SkillsList{
		{Skills: skills.Skills{Name: "genimages", Description: "生成图片"}, Path: `C:\skills\genimages\SKILL.md`},
		{Skills: skills.Skills{Name: "审查", Description: "审查代码"}, Path: `C:\skills\review\SKILL.md`},
	})
	m := NewModelForLanguage("zh-CN")

	m, cmd := m.handleSlashMessageSubmit("/skills")

	if cmd == nil || len(m.transcript) != 1 {
		t.Fatalf("skills list state = %#v, cmd nil = %v", m, cmd == nil)
	}
	entry := m.transcript[0]
	if entry.kind != transcriptSkillsList || len(entry.skills) != 2 {
		t.Fatalf("skills list entry = %#v", entry)
	}
	rendered := strings.Join(m.renderTranscriptEntry(80, entry), "\n")
	if !strings.Contains(rendered, m.pureWhiteStyle().Render("genimages")) ||
		!strings.Contains(rendered, m.mutedStyle().Render("生成图片")) ||
		!strings.Contains(rendered, m.pureWhiteStyle().Render("审查")) ||
		!strings.Contains(rendered, m.mutedStyle().Render("审查代码")) {
		t.Fatalf("skills list = %q", rendered)
	}
}

func TestSkillsMenuRendersNamePureWhiteAndDescriptionMuted(t *testing.T) {
	setTestSkills(t, []skills.SkillsList{
		{Skills: skills.Skills{Name: "genimages", Description: "生成图片"}},
	})
	m := NewModelForLanguage("zh-CN")
	m.setComposerValue("/skills ")

	rendered := m.renderSlashCommandMenu(80)
	if !strings.Contains(rendered, m.pureWhiteStyle().Render("genimages")) ||
		!strings.Contains(rendered, m.mutedStyle().Render("生成图片")) {
		t.Fatalf("skills menu = %q", rendered)
	}
}

func TestSkillsArgumentCandidatesShowNamesAfterCommandSpace(t *testing.T) {
	setTestSkills(t, []skills.SkillsList{
		{Skills: skills.Skills{Name: "genimages", Description: "生成图片"}},
		{Skills: skills.Skills{Name: "review", Description: "审查代码"}},
	})
	m := NewModelForLanguage("zh-CN")

	candidates := m.slashCommandCandidates("/skills ")
	if len(candidates) != 2 || candidates[0].Name != "genimages" || candidates[0].Description != "生成图片" ||
		candidates[0].Value != "/skills genimages " || candidates[1].Name != "review" {
		t.Fatalf("skill candidates = %#v", candidates)
	}

	filtered := m.slashCommandCandidates("/skills gen")
	if len(filtered) != 1 || filtered[0].Name != "genimages" {
		t.Fatalf("filtered skill candidates = %#v", filtered)
	}
}

func TestExactSkillsCommandEnterSubmitsInsteadOfSelectingMenu(t *testing.T) {
	setTestSkills(t, []skills.SkillsList{{Skills: skills.Skills{Name: "genimages"}}})
	m := NewModelForLanguage("zh-CN")
	m.setComposerValue("/skills")

	handled, _, _ := m.handleSlashCommandMenuKey(tea.KeyMsg{Type: tea.KeyEnter})
	if handled {
		t.Fatal("exact /skills enter should be submitted to display the skills list")
	}
}

func TestSkillsInvocationKeepsVisibleCommandAndBuildsAgentInput(t *testing.T) {
	path := `C:\skills\genimages\SKILL.md`
	setTestSkills(t, []skills.SkillsList{
		{Skills: skills.Skills{Name: "genimages", Description: "生成图片"}, Path: path},
	})
	m := NewModelForLanguage("zh-CN")

	m, cmd := m.handleSlashMessageSubmit("/skills genimages 生成猫图")
	defer m.closeAgentUI()

	if cmd == nil || !m.running {
		t.Fatalf("skill invocation did not start agent: %#v", m)
	}
	if got := m.transcript[len(m.transcript)-1]; got.role != "user" || got.content != "/skills genimages 生成猫图" {
		t.Fatalf("visible transcript = %#v", got)
	}
	if !strings.Contains(m.activeAgentInput, path) || !strings.Contains(m.activeAgentInput, "生成猫图") {
		t.Fatalf("agent input = %q", m.activeAgentInput)
	}
}

func TestSkillsErrorsDoNotStartAgent(t *testing.T) {
	tests := []struct {
		name   string
		skills []skills.SkillsList
		input  string
	}{
		{name: "empty list", input: "/skills"},
		{name: "missing task", skills: []skills.SkillsList{{Skills: skills.Skills{Name: "genimages"}, Path: "skill.md"}}, input: "/skills genimages"},
		{name: "unknown skill", skills: []skills.SkillsList{{Skills: skills.Skills{Name: "genimages"}, Path: "skill.md"}}, input: "/skills unknown 做事"},
		{name: "empty path", skills: []skills.SkillsList{{Skills: skills.Skills{Name: "genimages"}}}, input: "/skills genimages 做事"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setTestSkills(t, test.skills)
			m := NewModelForLanguage("zh-CN")
			m, _ = m.handleSlashMessageSubmit(test.input)
			if m.running || len(m.transcript) != 1 || !strings.HasPrefix(m.transcript[0].content, m.messages.Chat.AgentErrorLabel) {
				t.Fatalf("error state = %#v", m)
			}
		})
	}
}
