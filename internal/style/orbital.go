package style

import (
	"fmt"
	"strings"

	"orbit/internal/config"

	"github.com/charmbracelet/lipgloss"
)

type OrbitalMenuCopy struct {
	Title        string
	Heading      string
	Subtitle     string
	MoveShortcut string
	MoveAction   string
	SelectKey    string
	SelectAction string
	QuitKey      string
	QuitAction   string
}

type OrbitalMenuView struct {
	Copy           OrbitalMenuCopy
	Options        []string
	Cursor         int
	SequenceOffset int
	StyleConfig    config.StyleConfig
	ViewportWidth  int
	ViewportHeight int
}

type OrbitalFormCopy struct {
	Title   string
	Heading string
	Hint    string
}

type OrbitalFormView struct {
	Copy           OrbitalFormCopy
	Lines          []string
	StyleConfig    config.StyleConfig
	ViewportWidth  int
	ViewportHeight int
}

type orbitalStyles struct {
	page, logoLoop, logoAccent, logoWord, logoMeta lipgloss.Style
	panelLine, panelBorder, heading, subtitle      lipgloss.Style
	divider, option, active, hintKey, hintAction   lipgloss.Style
	layout                                         config.StyleLayout
}

func newOrbitalStyles(styleConfig config.StyleConfig) orbitalStyles {
	styleConfig = resolvedStyleConfig(styleConfig)
	accent := lipgloss.Color(styleConfig.Palette.Accent)
	foreground := lipgloss.Color(styleConfig.Palette.Foreground)
	muted := lipgloss.Color(styleConfig.Palette.Muted)
	divider := lipgloss.Color(styleConfig.Palette.Divider)
	return orbitalStyles{
		page:        lipgloss.NewStyle().Foreground(foreground),
		logoLoop:    lipgloss.NewStyle().Foreground(foreground).Bold(true),
		logoAccent:  lipgloss.NewStyle().Foreground(accent).Bold(true),
		logoWord:    lipgloss.NewStyle().Foreground(accent).Bold(true),
		logoMeta:    lipgloss.NewStyle().Foreground(muted),
		panelLine:   lipgloss.NewStyle(),
		panelBorder: lipgloss.NewStyle().Foreground(accent),
		heading:     lipgloss.NewStyle().Foreground(accent).Bold(true),
		subtitle:    lipgloss.NewStyle().Foreground(muted),
		divider:     lipgloss.NewStyle().Foreground(divider),
		option:      lipgloss.NewStyle().Foreground(foreground),
		active:      lipgloss.NewStyle().Foreground(accent).Bold(true),
		hintKey:     lipgloss.NewStyle().Foreground(accent).Bold(true),
		hintAction:  lipgloss.NewStyle().Foreground(muted).Bold(true),
		layout:      styleConfig.Layout,
	}
}

func TerminalHyperlink(text, target string) string {
	if text == "" || target == "" {
		return text
	}
	return "\x1b]8;;" + target + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

func RenderOrbitalMenuView(view OrbitalMenuView) string {
	styles := newOrbitalStyles(view.StyleConfig)
	width := view.ViewportWidth
	height := view.ViewportHeight
	logo := renderOrbitalLogoForViewport(view.Copy.Title, width, styles)
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		logo,
		"",
		renderOrbitalPanelForViewport(view.Copy, view.Options, view.Cursor, view.SequenceOffset, width, height, lipgloss.Height(logo), styles),
	)

	page := lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)

	return styles.page.Render(page)
}

func OrbitalMenuOptionCapacity(title string, viewportWidth, viewportHeight int, styleConfig config.StyleConfig) int {
	styles := newOrbitalStyles(styleConfig)
	logo := renderOrbitalLogoForViewport(title, viewportWidth, styles)
	panelHeight := viewportHeight - lipgloss.Height(logo) - 2
	panelHeight = clampInt(panelHeight, styles.layout.MinPanelHeight, styles.layout.PanelHeight)
	if panelHeight < 18 {
		return maxInt(1, panelHeight-8)
	}
	return maxInt(1, (panelHeight-13)/2)
}

func RenderOrbitalFormView(view OrbitalFormView) string {
	styles := newOrbitalStyles(view.StyleConfig)
	width := view.ViewportWidth
	height := view.ViewportHeight
	logo := renderOrbitalLogoForViewport(view.Copy.Title, width, styles)
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		logo,
		"",
		renderOrbitalFormPanelForViewport(view.Copy, view.Lines, width, height, lipgloss.Height(logo), styles),
	)

	page := lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)

	return styles.page.Render(page)
}

func RenderOrbitalLogo(title string) string {
	return renderOrbitalLogo(title, newOrbitalStyles(config.StyleConfig{}))
}

func renderOrbitalLogo(title string, styles orbitalStyles) string {
	if title != "Orbit" {
		return styles.logoLoop.Render(title)
	}
	return renderOrbitalPromptLogo(true, styles)
}

func RenderOrbitalLogoForViewport(title string, viewportWidth int) string {
	return RenderOrbitalLogoForViewportWithStyle(title, viewportWidth, config.StyleConfig{})
}

func RenderOrbitalLogoForViewportWithStyle(title string, viewportWidth int, styleConfig config.StyleConfig) string {
	return renderOrbitalLogoForViewport(title, viewportWidth, newOrbitalStyles(styleConfig))
}

func renderOrbitalLogoForViewport(title string, viewportWidth int, styles orbitalStyles) string {
	if viewportWidth <= 0 {
		return ""
	}
	if title != "Orbit" {
		return styles.logoLoop.Render(truncateCells(title, viewportWidth))
	}

	full := renderOrbitalPromptLogo(true, styles)
	if lipgloss.Width(full) <= viewportWidth {
		return full
	}
	compact := renderOrbitalPromptLogo(false, styles)
	if lipgloss.Width(compact) <= viewportWidth {
		return compact
	}
	return styles.logoAccent.Render(truncateCells("◉ ORBIT", viewportWidth))
}

func RenderOrbitalMenuPanel(copy OrbitalMenuCopy, options []string, cursor, viewportWidth int) string {
	styles := newOrbitalStyles(config.StyleConfig{})
	return renderOrbitalPanelWithHeight(copy, options, cursor, 0, viewportWidth, styles.layout.PanelHeight, styles)
}

func RenderOrbitalFormPanel(copy OrbitalFormCopy, lines []string, viewportWidth int) string {
	styles := newOrbitalStyles(config.StyleConfig{})
	return renderOrbitalFormPanelWithHeight(copy, lines, viewportWidth, styles.layout.PanelHeight, styles)
}

func renderOrbitalPanelForViewport(copy OrbitalMenuCopy, options []string, cursor, sequenceOffset, viewportWidth, viewportHeight, logoHeight int, styles orbitalStyles) string {
	panelHeight := viewportHeight - logoHeight - 2
	panelHeight = clampInt(panelHeight, styles.layout.MinPanelHeight, styles.layout.PanelHeight)
	return renderOrbitalPanelWithHeight(copy, options, cursor, sequenceOffset, viewportWidth, panelHeight, styles)
}

func renderOrbitalFormPanelForViewport(copy OrbitalFormCopy, lines []string, viewportWidth, viewportHeight, logoHeight int, styles orbitalStyles) string {
	panelHeight := viewportHeight - logoHeight - 2
	panelHeight = clampInt(panelHeight, styles.layout.MinPanelHeight, styles.layout.PanelHeight)
	return renderOrbitalFormPanelWithHeight(copy, lines, viewportWidth, panelHeight, styles)
}

func renderOrbitalPanelWithHeight(copy OrbitalMenuCopy, options []string, cursor, sequenceOffset, viewportWidth, panelHeight int, styles orbitalStyles) string {
	panelWidth := orbitalPanelOuterWidth(viewportWidth, styles.layout)
	contentWidth := orbitalPanelContentWidth(panelWidth)

	if panelHeight < 18 {
		return renderCompactOrbitalPanel(copy, options, cursor, sequenceOffset, panelWidth, contentWidth, panelHeight, styles)
	}

	innerLines := []string{
		orbitalPanelBlank(panelWidth, styles),
		orbitalPanelText(panelWidth, copy.Heading, styles.heading, styles),
	}
	innerLines = append(innerLines, orbitalPanelTextLines(panelWidth, copy.Subtitle, styles.subtitle, styles)...)
	innerLines = append(innerLines,
		orbitalPanelBlank(panelWidth, styles),
		orbitalPanelDivider(panelWidth, contentWidth, styles),
		orbitalPanelBlank(panelWidth, styles),
	)

	for i, choice := range options {
		if cursor == i {
			innerLines = append(innerLines, orbitalPanelActiveOption(panelWidth, fmt.Sprintf(">  %s", choice), styles)...)
		} else {
			innerLines = append(innerLines, orbitalPanelText(panelWidth, fmt.Sprintf("%d   %s", sequenceOffset+i+1, choice), styles.option, styles))
		}
		innerLines = append(innerLines, orbitalPanelBlank(panelWidth, styles))
	}

	innerLines = append(innerLines,
		orbitalPanelDivider(panelWidth, contentWidth, styles),
		orbitalPanelBlank(panelWidth, styles),
		orbitalPanelContent(panelWidth, renderOrbitalShortcutBar(copy, contentWidth, styles), styles),
	)

	for len(innerLines) < panelHeight-2 {
		innerLines = append(innerLines, orbitalPanelBlank(panelWidth, styles))
	}
	if len(innerLines) > panelHeight-2 {
		innerLines = innerLines[:panelHeight-2]
	}

	lines := append([]string{orbitalPanelTop(panelWidth, styles)}, innerLines...)
	lines = append(lines, orbitalPanelBottom(panelWidth, styles))
	return strings.Join(lines, "\n")
}

func renderCompactOrbitalPanel(copy OrbitalMenuCopy, options []string, cursor, sequenceOffset, panelWidth, contentWidth, panelHeight int, styles orbitalStyles) string {
	innerLines := []string{
		orbitalPanelBlank(panelWidth, styles),
		orbitalPanelText(panelWidth, copy.Heading, styles.heading, styles),
	}
	innerLines = append(innerLines, orbitalPanelTextLines(panelWidth, copy.Subtitle, styles.subtitle, styles)...)
	innerLines = append(innerLines, orbitalPanelDivider(panelWidth, contentWidth, styles))

	for i, choice := range options {
		label := fmt.Sprintf("%d   %s", sequenceOffset+i+1, choice)
		textStyle := styles.option
		if cursor == i {
			label = fmt.Sprintf(">  %s", choice)
			textStyle = styles.active
		}
		innerLines = append(innerLines, orbitalPanelText(panelWidth, label, textStyle, styles))
	}

	innerLines = append(innerLines,
		orbitalPanelDivider(panelWidth, contentWidth, styles),
		orbitalPanelContent(panelWidth, renderOrbitalShortcutBar(copy, contentWidth, styles), styles),
	)

	for len(innerLines) < panelHeight-2 {
		innerLines = append(innerLines, orbitalPanelBlank(panelWidth, styles))
	}
	if len(innerLines) > panelHeight-2 {
		innerLines = innerLines[:panelHeight-2]
	}

	lines := append([]string{orbitalPanelTop(panelWidth, styles)}, innerLines...)
	lines = append(lines, orbitalPanelBottom(panelWidth, styles))
	return strings.Join(lines, "\n")
}

func renderOrbitalFormPanelWithHeight(copy OrbitalFormCopy, formLines []string, viewportWidth, panelHeight int, styles orbitalStyles) string {
	panelWidth := orbitalPanelOuterWidth(viewportWidth, styles.layout)
	contentWidth := orbitalPanelContentWidth(panelWidth)

	innerLines := []string{
		orbitalPanelBlank(panelWidth, styles),
		orbitalPanelText(panelWidth, copy.Heading, styles.heading, styles),
		orbitalPanelBlank(panelWidth, styles),
		orbitalPanelDivider(panelWidth, contentWidth, styles),
		orbitalPanelBlank(panelWidth, styles),
	}

	for _, line := range formLines {
		if line == "" {
			innerLines = append(innerLines, orbitalPanelBlank(panelWidth, styles))
			continue
		}
		innerLines = append(innerLines, orbitalPanelText(panelWidth, line, styles.option, styles))
	}

	innerLines = append(innerLines,
		orbitalPanelBlank(panelWidth, styles),
		orbitalPanelDivider(panelWidth, contentWidth, styles),
		orbitalPanelBlank(panelWidth, styles),
		orbitalPanelText(panelWidth, copy.Hint, styles.subtitle, styles),
	)

	for len(innerLines) < panelHeight-2 {
		innerLines = append(innerLines, orbitalPanelBlank(panelWidth, styles))
	}
	if len(innerLines) > panelHeight-2 {
		innerLines = innerLines[:panelHeight-2]
	}

	lines := append([]string{orbitalPanelTop(panelWidth, styles)}, innerLines...)
	lines = append(lines, orbitalPanelBottom(panelWidth, styles))
	return strings.Join(lines, "\n")
}

func renderOrbitalShortcutBar(copy OrbitalMenuCopy, width int, styles orbitalStyles) string {
	items := []string{
		renderOrbitalShortcut(copy.MoveShortcut, copy.MoveAction, styles),
		renderOrbitalShortcut(copy.SelectKey, copy.SelectAction, styles),
		renderOrbitalShortcut(copy.QuitKey, copy.QuitAction, styles),
	}
	return spreadItems(width, items)
}

func renderOrbitalPromptLogo(includeAgent bool, styles orbitalStyles) string {
	logo := styles.logoAccent.Render("◉ ORBIT")
	if includeAgent {
		logo += styles.logoMeta.Render(" / AGENT")
	}
	return logo
}

func orbitalPanelOuterWidth(viewportWidth int, layout config.StyleLayout) int {
	margin := 4
	if viewportWidth >= 140 {
		margin = 47
	} else if viewportWidth >= 100 {
		margin = 16
	}
	width := minInt(layout.MaxPanelWidth, viewportWidth-margin)
	if width < layout.MinWidth-2 {
		width = maxInt(viewportWidth-2, 20)
	}
	return width
}

func orbitalPanelContentWidth(panelWidth int) int {
	return maxInt(panelWidth-17, 28)
}

func orbitalPanelTop(width int, styles orbitalStyles) string {
	return styles.panelLine.Render(
		styles.panelBorder.Render("╱") +
			styles.panelBorder.Render(strings.Repeat("─", width-2)) +
			styles.panelBorder.Render("╲"),
	)
}

func orbitalPanelBottom(width int, styles orbitalStyles) string {
	return styles.panelLine.Render(
		styles.panelBorder.Render("╲") +
			styles.panelBorder.Render(strings.Repeat("─", width-2)) +
			styles.panelBorder.Render("╱"),
	)
}

func orbitalPanelBlank(width int, styles orbitalStyles) string {
	return orbitalPanelFramed(width, "", styles)
}

func orbitalPanelContent(width int, content string, styles orbitalStyles) string {
	return orbitalPanelFramed(width, strings.Repeat(" ", 6)+content, styles)
}

func orbitalPanelText(width int, text string, textStyle lipgloss.Style, styles orbitalStyles) string {
	contentWidth := maxInt(width-14, 8)
	text = truncateCells(text, contentWidth)
	return orbitalPanelContent(width, textStyle.Render(text), styles)
}

func orbitalPanelTextLines(width int, text string, textStyle lipgloss.Style, styles orbitalStyles) []string {
	contentWidth := maxInt(width-14, 8)
	wrapped := lipgloss.NewStyle().Width(contentWidth).Render(text)
	lines := strings.Split(wrapped, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		result = append(result, orbitalPanelContent(width, textStyle.Render(line), styles))
	}
	return result
}

func orbitalPanelDivider(width, dividerWidth int, styles orbitalStyles) string {
	divider := styles.divider.Render(strings.Repeat("┄", dividerWidth))
	return orbitalPanelContent(width, divider, styles)
}

func orbitalPanelActiveOption(width int, label string, styles orbitalStyles) []string {
	boxWidth := maxInt(width-10, 28)
	innerWidth := boxWidth - 2
	top := styles.panelBorder.Render("┌" + strings.Repeat("─", innerWidth) + "┐")
	label = truncateCells("  "+label, innerWidth)
	middle := styles.panelBorder.Render("│") +
		styles.active.Render(padCells(label, innerWidth)) +
		styles.panelBorder.Render("│")
	bottom := styles.panelBorder.Render("└" + strings.Repeat("─", innerWidth) + "┘")

	return []string{
		orbitalPanelFramed(width, strings.Repeat(" ", 4)+top, styles),
		orbitalPanelFramed(width, strings.Repeat(" ", 4)+middle, styles),
		orbitalPanelFramed(width, strings.Repeat(" ", 4)+bottom, styles),
	}
}

func orbitalPanelFramed(width int, content string, styles orbitalStyles) string {
	innerWidth := width - 2
	return styles.panelLine.Render(
		styles.panelBorder.Render("│") +
			padCells(content, innerWidth) +
			styles.panelBorder.Render("│"),
	)
}

func renderOrbitalShortcut(key, action string, styles orbitalStyles) string {
	return styles.hintKey.Render(key) + " " + styles.hintAction.Render(action)
}

func spreadItems(width int, items []string) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return items[0]
	}

	usedWidth := 0
	for _, item := range items {
		usedWidth += lipgloss.Width(item)
	}
	if usedWidth >= width {
		return strings.Join(items, "  ")
	}

	totalGap := width - usedWidth
	baseGap := totalGap / (len(items) - 1)
	remainder := totalGap % (len(items) - 1)

	var builder strings.Builder
	for i, item := range items {
		if i > 0 {
			gap := baseGap
			if i <= remainder {
				gap++
			}
			builder.WriteString(strings.Repeat(" ", gap))
		}
		builder.WriteString(item)
	}
	return builder.String()
}

func padCells(value string, width int) string {
	valueWidth := lipgloss.Width(value)
	if valueWidth >= width {
		return value
	}
	return value + strings.Repeat(" ", width-valueWidth)
}

func truncateCells(value string, width int) string {
	if lipgloss.Width(value) <= width {
		return value
	}

	var builder strings.Builder
	used := 0
	for _, r := range value {
		cellWidth := lipgloss.Width(string(r))
		if used+cellWidth > width-1 {
			break
		}
		builder.WriteRune(r)
		used += cellWidth
	}
	return builder.String() + "…"
}

func clampInt(value, minValue, maxValue int) int {
	return maxInt(minValue, minInt(value, maxValue))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
