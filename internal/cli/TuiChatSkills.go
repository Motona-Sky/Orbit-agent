package cli

import (
	"fmt"
	"strings"

	"orbit/internal/skills"

	tea "github.com/charmbracelet/bubbletea"
)

const skillsCommand = "/skills"

type skillDisplayItem struct {
	Name        string
	Description string
}

func (m model) handleSkillsCommand(value string) (model, tea.Cmd) {
	fields := strings.Fields(value)
	if len(fields) == 1 {
		return m.showSkillsList()
	}
	if len(fields) < 3 {
		return m.reportSkillsError(m.messages.Chat.SkillsTaskRequired)
	}

	name := fields[1]
	skill, ok := findSkill(name)
	if !ok {
		return m.reportSkillsError(fmt.Sprintf(m.messages.Chat.SkillNotFound, name))
	}
	if strings.TrimSpace(skill.Path) == "" {
		return m.reportSkillsError(fmt.Sprintf(m.messages.Chat.SkillPathMissing, name))
	}

	taskStart := strings.Index(value, name) + len(name)
	task := strings.TrimSpace(value[taskStart:])
	if task == "" {
		return m.reportSkillsError(m.messages.Chat.SkillsTaskRequired)
	}
	agentInput := fmt.Sprintf("Read and follow the skill instructions at: %s\n\nUser task:\n%s", skill.Path, task)
	return m.startAgentTurn(strings.TrimSpace(value), agentInput)
}

func (m model) showSkillsList() (model, tea.Cmd) {
	if len(skills.SkillsLists) == 0 {
		return m.reportSkillsError(m.messages.Chat.SkillsEmpty)
	}
	items := make([]skillDisplayItem, 0, len(skills.SkillsLists))
	for _, skill := range skills.SkillsLists {
		name := strings.TrimSpace(skill.Skills.Name)
		if name == "" {
			continue
		}
		items = append(items, skillDisplayItem{
			Name:        name,
			Description: strings.TrimSpace(skill.Skills.Description),
		})
	}
	if len(items) == 0 {
		return m.reportSkillsError(m.messages.Chat.SkillsEmpty)
	}
	m.transcript = append(m.transcript, chatTranscriptEntry{
		kind: transcriptSkillsList, role: "assistant", skills: items,
	})
	return m.commitTerminalTranscript(nil)
}

func (m model) reportSkillsError(message string) (model, tea.Cmd) {
	m.transcript = append(m.transcript, chatTranscriptEntry{
		kind: transcriptMessage, role: "assistant", content: m.messages.Chat.AgentErrorLabel + message,
	})
	return m.commitTerminalTranscript(nil)
}

func findSkill(name string) (skills.SkillsList, bool) {
	for _, skill := range skills.SkillsLists {
		if skill.Skills.Name == name {
			return skill, true
		}
	}
	return skills.SkillsList{}, false
}
