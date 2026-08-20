package style

import (
	"strings"

	"orbit/internal/config"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type AgentCommandInputCopy struct {
	Title       string
	Prompt      string
	Placeholder string
	Modes       []string
}

type AgentCommandInputView struct {
	Copy        AgentCommandInputCopy
	InfoItems   []AgentCommandInfoItem
	StyleConfig config.StyleConfig
	Value       string
	Focused     bool
	Width       int
}

type AgentCommandInfoItem struct {
	Label string
	Value string
}

type agentCommandStyles struct {
	prompt, value, placeholder, cursor, selectedMode, mode lipgloss.Style
	infoMarker, infoLabel, infoValue, infoSeparator        lipgloss.Style
	border, focusedBorder                                  lipgloss.Style
}

func newAgentCommandStyles(styleConfig config.StyleConfig) agentCommandStyles {
	styleConfig = resolvedStyleConfig(styleConfig)
	accent := lipgloss.Color(styleConfig.Palette.Accent)
	foreground := lipgloss.Color(styleConfig.Palette.Foreground)
	muted := lipgloss.Color(styleConfig.Palette.Muted)
	divider := lipgloss.Color(styleConfig.Palette.Divider)
	border := lipgloss.NewStyle().Foreground(divider)
	return agentCommandStyles{
		prompt:        lipgloss.NewStyle().Foreground(accent).Bold(true),
		value:         lipgloss.NewStyle().Foreground(foreground),
		placeholder:   lipgloss.NewStyle().Foreground(muted),
		cursor:        lipgloss.NewStyle().Foreground(foreground).Reverse(true),
		selectedMode:  lipgloss.NewStyle().Foreground(accent).Padding(0, 1),
		mode:          lipgloss.NewStyle().Foreground(foreground).Padding(0, 1),
		infoMarker:    lipgloss.NewStyle().Foreground(muted),
		infoLabel:     lipgloss.NewStyle().Foreground(muted),
		infoValue:     lipgloss.NewStyle().Foreground(accent).Bold(true),
		infoSeparator: lipgloss.NewStyle().Foreground(divider),
		border:        border,
		focusedBorder: border.Copy().Foreground(accent),
	}
}

func RenderAgentCommandInput(view AgentCommandInputView) string {
	styles := newAgentCommandStyles(view.StyleConfig)
	width := view.Width
	if width < 8 {
		width = 8
	}

	input := renderAgentCommandInputLine(view.Copy, view.Value, width, view.Focused, styles)
	sections := []string{input}
	if len(view.Copy.Modes) > 0 {
		sections = append(sections, renderAgentCommandModes(view.Copy.Modes, width, styles))
	}
	if info := renderAgentCommandInfo(view.InfoItems, width, styles); info != "" {
		sections = append(sections, info)
	}
	return strings.Join(sections, "\n")
}

func renderAgentCommandInputLine(copy AgentCommandInputCopy, value string, width int, focused bool, styles agentCommandStyles) string {
	prompt := styles.prompt.Render(copy.Prompt)
	var lines []string
	if value == "" {
		content := styles.placeholder.Render(copy.Placeholder)
		if focused {
			content = styles.cursor.Render(" ") + content
		}
		lines = []string{prompt + " " + content}
	} else {
		lines = renderAgentCommandValue(copy.Prompt, value, focused, styles)
	}

	return renderAgentCommandBox(copy.Title, lines, width, focused, styles)
}

func renderAgentCommandValue(prompt, value string, focused bool, styles agentCommandStyles) []string {
	lines := strings.Split(value, "\n")
	if focused {
		lines[len(lines)-1] += styles.cursor.Render(" ")
	}
	if len(lines) == 1 {
		return []string{styles.prompt.Render(prompt) + " " + styles.value.Render(lines[0])}
	}

	indent := strings.Repeat(" ", lipgloss.Width(prompt)+1)
	for i, line := range lines {
		lines[i] = styles.value.Render(line)
		if i == 0 {
			lines[i] = styles.prompt.Render(prompt) + " " + lines[i]
			continue
		}
		lines[i] = styles.value.Render(indent) + lines[i]
	}
	return lines
}

func renderAgentCommandBox(title string, lines []string, width int, focused bool, styles agentCommandStyles) string {
	borderStyle := styles.border
	if focused {
		borderStyle = styles.focusedBorder
	}

	innerWidth := maxInt(1, width-4)
	wrappedLines := make([]string, 0, len(lines))
	for _, line := range lines {
		wrappedLines = append(wrappedLines, strings.Split(ansi.Hardwrap(line, innerWidth, true), "\n")...)
	}

	rendered := make([]string, 0, len(wrappedLines)+2)
	rendered = append(rendered, renderAgentCommandTopBorder(title, width, borderStyle, styles))
	for _, line := range wrappedLines {
		rendered = append(rendered, borderStyle.Render("│")+" "+fitStyledLine(line, innerWidth)+" "+borderStyle.Render("│"))
	}
	rendered = append(rendered, borderStyle.Render("╰"+strings.Repeat("─", width-2)+"╯"))
	return strings.Join(rendered, "\n")
}

func renderAgentCommandTopBorder(title string, width int, borderStyle lipgloss.Style, styles agentCommandStyles) string {
	if strings.TrimSpace(title) == "" {
		return borderStyle.Render("╭" + strings.Repeat("─", width-2) + "╮")
	}

	title = truncateCells(title, maxInt(1, width-6))
	dashes := maxInt(1, width-lipgloss.Width(title)-5)
	return borderStyle.Render("╭─ ") + styles.prompt.Render(title) + borderStyle.Render(" "+strings.Repeat("─", dashes)+"╮")
}

func fitStyledLine(line string, width int) string {
	line = lipgloss.NewStyle().Inline(true).MaxWidth(width).Render(line)
	return padCells(line, width)
}

func renderAgentCommandModes(modes []string, width int, styles agentCommandStyles) string {
	items := make([]string, 0, len(modes))
	for i, mode := range modes {
		if i == 0 {
			items = append(items, styles.selectedMode.Render("◉ "+mode))
			continue
		}
		items = append(items, styles.mode.Render("○ "+mode))
	}

	line := strings.Join(items, " ")
	if lipgloss.Width(line) > width {
		return truncateCells(line, width)
	}
	return line
}

func renderAgentCommandInfo(infoItems []AgentCommandInfoItem, width int, styles agentCommandStyles) string {
	items := make([]string, 0, len(infoItems))
	for _, infoItem := range infoItems {
		value := strings.TrimSpace(infoItem.Value)
		if value == "" {
			continue
		}
		item := styles.infoMarker.Render("◇")
		if label := strings.TrimSpace(infoItem.Label); label != "" {
			item += " " + styles.infoValue.Render(strings.ToLower(label))
		}
		item += "  " + styles.infoLabel.Render(strings.ToLower(value))
		items = append(items, item)
	}
	if len(items) == 0 {
		return ""
	}
	separator := "  " + styles.infoSeparator.Render("·") + "  "
	line := strings.Join(items, separator)
	if lipgloss.Width(line) > width {
		return truncateCells(line, width)
	}
	return line
}
