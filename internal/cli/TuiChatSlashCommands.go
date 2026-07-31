package cli

import (
	"strings"

	"looporbit/internal/skills"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type slashCommandID int

const (
	slashCommandModel slashCommandID = iota
	slashCommandProvider
	slashCommandEffort
	slashCommandSkills
	slashCommandNew
	slashCommandClear
)

type slashCommand struct {
	Name        string
	Description string
	Value       string
	ID          slashCommandID
}

func (m model) slashCommands() []slashCommand {
	return []slashCommand{
		{Name: modelSetupCommand, Description: m.messages.Chat.ModelCommandDescription, ID: slashCommandModel},
		{Name: providerSetupCommand, Description: m.messages.Chat.ProviderCommandDescription, ID: slashCommandProvider},
		{Name: effortSetupCommand, Description: m.messages.Chat.EffortCommandDescription, ID: slashCommandEffort},
		{Name: skillsCommand, Description: m.messages.Chat.SkillsCommandDescription, ID: slashCommandSkills},
		{Name: newCommand, Description: m.messages.Chat.NewCommandDescription, ID: slashCommandNew},
		{Name: clearCommand, Description: m.messages.Chat.ClearCommandDescription, ID: slashCommandClear},
	}
}

func (m model) slashCommandCandidates(input string) []slashCommand {
	if skillPrefix, ok := strings.CutPrefix(input, skillsCommand+" "); ok {
		if strings.ContainsAny(skillPrefix, " \t\r\n") {
			return nil
		}
		prefix := strings.ToLower(skillPrefix)
		matches := make([]slashCommand, 0, len(skills.SkillsLists))
		for _, skill := range skills.SkillsLists {
			name := strings.TrimSpace(skill.Skills.Name)
			if name == "" || !strings.HasPrefix(strings.ToLower(name), prefix) {
				continue
			}
			matches = append(matches, slashCommand{
				Name:        name,
				Description: strings.TrimSpace(skill.Skills.Description),
				Value:       skillsCommand + " " + name + " ",
				ID:          slashCommandSkills,
			})
		}
		return matches
	}
	if !strings.HasPrefix(input, "/") || strings.ContainsAny(input, " \t\r\n") {
		return nil
	}
	input = strings.ToLower(input)
	commands := m.slashCommands()
	matches := make([]slashCommand, 0, len(commands))
	for _, command := range commands {
		if strings.HasPrefix(strings.ToLower(command.Name), input) {
			matches = append(matches, command)
		}
	}
	return matches
}

func (m model) activeSlashCommandCandidates() []slashCommand {
	if m.slashMenuDismissed {
		return nil
	}
	return m.slashCommandCandidates(m.composerText())
}

func (m model) handleSlashCommandMenuKey(msg tea.KeyMsg) (bool, model, tea.Cmd) {
	candidates := m.activeSlashCommandCandidates()
	if len(candidates) == 0 {
		return false, m, nil
	}
	m.slashMenuCursor = (m.slashMenuCursor%len(candidates) + len(candidates)) % len(candidates)
	switch msg.String() {
	case "up":
		m.slashMenuCursor = (m.slashMenuCursor - 1 + len(candidates)) % len(candidates)
		return true, m, nil
	case "down":
		m.slashMenuCursor = (m.slashMenuCursor + 1) % len(candidates)
		return true, m, nil
	case "tab", "enter":
		candidate := candidates[m.slashMenuCursor]
		if msg.String() == "enter" && m.composerText() == candidate.Name {
			return false, m, nil
		}
		value := candidate.Value
		if value == "" {
			value = candidate.Name
		}
		m.setComposerValue(value)
		m.slashMenuDismissed = true
		return true, m, nil
	case "esc":
		m.slashMenuDismissed = true
		return true, m, nil
	}
	return false, m, nil
}

func (m *model) reopenSlashMenuAfterEdit() {
	m.slashMenuDismissed = false
	m.slashMenuCursor = 0
}

func (m model) renderSlashCommandMenu(width int) string {
	candidates := m.activeSlashCommandCandidates()
	if len(candidates) == 0 || width <= 0 {
		return ""
	}
	cursor := (m.slashMenuCursor%len(candidates) + len(candidates)) % len(candidates)
	separator := m.mutedStyle().Render(strings.Repeat("─", width))
	lines := []string{separator}
	for index, command := range candidates {
		marker := "  "
		nameStyle := m.mutedStyle()
		if command.ID == slashCommandSkills && strings.HasPrefix(m.composerText(), skillsCommand+" ") {
			nameStyle = m.pureWhiteStyle()
		}
		if index == cursor {
			marker = "› "
			if command.ID != slashCommandSkills || !strings.HasPrefix(m.composerText(), skillsCommand+" ") {
				nameStyle = m.accentStyle()
			}
		}
		name := marker + command.Name
		nameWidth := lipgloss.Width(name)
		line := nameStyle.Render(name)
		if remaining := width - nameWidth - 2; remaining > 0 && command.Description != "" {
			description := m.mutedStyle().Inline(true).MaxWidth(remaining).Render(command.Description)
			line += "  " + description
		}
		line = lipgloss.NewStyle().Inline(true).MaxWidth(width).Render(line)
		lines = append(lines, line)
	}
	lines = append(lines, separator)
	return strings.Join(lines, "\n")
}
