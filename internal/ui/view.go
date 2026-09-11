package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/vika2603/herdr-palette/internal/theme"
)

// styles are the popup's rendered colours. The selected row's segments each
// carry the background themselves: a style wrapped around text that already
// contains escape sequences would be cut short by the first reset inside it.
type styles struct {
	title lipgloss.Style
	meta  lipgloss.Style
	match lipgloss.Style
	rule  lipgloss.Style
	thumb lipgloss.Style
	fail  lipgloss.Style

	selected      lipgloss.Style
	selectedMeta  lipgloss.Style
	selectedMatch lipgloss.Style
}

func newStyles(colours theme.Theme) styles {
	selected := lipgloss.NewStyle().Background(colours.Selected)
	return styles{
		title: lipgloss.NewStyle(),
		meta:  lipgloss.NewStyle().Foreground(colours.Meta),
		match: lipgloss.NewStyle().Foreground(colours.Match).Bold(true),
		rule:  lipgloss.NewStyle().Foreground(colours.Rule),
		thumb: lipgloss.NewStyle().Foreground(colours.Scrollbar),
		fail:  lipgloss.NewStyle().Foreground(colours.Failure),

		selected:      selected.Bold(true),
		selectedMeta:  selected.Foreground(colours.Meta),
		selectedMatch: selected.Foreground(colours.Match).Bold(true),
	}
}

// Sizes used until the first resize message arrives.
const (
	defaultCols = 72
	defaultRows = 12
)

const (
	// chrome is the row budget the list does not get: the query line, the two
	// rules, and the help line.
	chrome = 4
	// headerRows is how many lines precede the first command row: the query
	// line and the rule under it. A click's row is counted from there.
	headerRows = 2
	// margins is what a row spends outside the title: the padding on both
	// sides, the blank that keeps the scrollbar off the text, and the
	// scrollbar itself.
	margins = 4
)

func (m model) cols() int {
	if m.width <= 0 {
		return defaultCols
	}
	return m.width
}

// rows is how many commands fit under the query line.
func (m model) rows() int {
	if m.height <= 0 {
		return defaultRows
	}
	if rows := m.height - chrome; rows > 0 {
		return rows
	}
	return 1
}

func (m model) View() string { return m.listView() }

func (m model) listView() string {
	rule := m.rule()
	lines := []string{m.query.View(), rule}

	rows := m.rows()
	if len(m.ranked) == 0 {
		lines = append(lines, m.styles.meta.Render(" no command matches"))
		rows--
	}
	for i := m.offset; i < len(m.ranked) && i < m.offset+rows; i++ {
		lines = append(lines, m.row(i))
	}
	for len(lines) < rows+headerRows {
		lines = append(lines, "")
	}

	lines = append(lines, rule, m.help())
	return strings.Join(lines, "\n")
}

func (m model) row(index int) string {
	ranked := m.ranked[index]
	selected := index == m.cursor

	text, meta := m.styles.title, m.styles.meta
	if selected {
		text, meta = m.styles.selected, m.styles.selectedMeta
	}

	// The key column is right-aligned, so the rows end on a straight edge
	// whatever the keys are, and it is there at all only when something on
	// show is bound to one.
	key := ""
	if m.keyWidth > 0 {
		key = fmt.Sprintf("%*s", m.keyWidth, ranked.Entry.Key)
	}

	width := lipgloss.Width(key)
	name := truncate(ranked.Entry.Name(), m.cols()-width-margins-2)
	namespace := min(len([]rune(ranked.Entry.Namespace())), len([]rune(name)))
	rendered := m.highlight(name, namespace, ranked.Matched, selected)
	gap := max(m.cols()-margins-lipgloss.Width(rendered)-width, 1)

	row := text.Render(" ") + rendered + text.Render(strings.Repeat(" ", gap)) + meta.Render(key) + text.Render(" ")
	return row + " " + m.scrollbar(index-m.offset)
}

// scrollbar draws where the visible window sits in the whole list, one column
// at the right edge. It stays blank while everything fits, so rows do not
// shift as the query narrows the list.
func (m model) scrollbar(row int) string {
	rows, total := m.rows(), len(m.ranked)
	if total <= rows {
		return " "
	}

	// Scaled by how far the window can travel rather than by the list length,
	// so the thumb reaches the bottom exactly on the last page.
	size := max(rows*rows/total, 1)
	start := m.offset * (rows - size) / (total - rows)
	if row >= start && row < start+size {
		return m.styles.thumb.Render("┃")
	}
	return m.styles.rule.Render("│")
}

// span is how one rune of a row is drawn.
type span int

const (
	spanNamespace span = iota
	spanTitle
	spanMatch
)

// highlight draws a row: the namespace in front of the title dimmer than the
// title itself, with the runes the query matched picked out in both. namespace
// is how many runes the namespace takes, and matched indexes the whole row.
func (m model) highlight(text string, namespace int, matched []int, selected bool) string {
	styles := map[span]lipgloss.Style{
		spanNamespace: m.styles.meta,
		spanTitle:     m.styles.title,
		spanMatch:     m.styles.match,
	}
	if selected {
		styles[spanNamespace] = m.styles.selectedMeta
		styles[spanTitle] = m.styles.selected
		styles[spanMatch] = m.styles.selectedMatch
	}

	runes := []rune(text)
	at := 0
	spanAt := func(index int) span {
		for at < len(matched) && matched[at] < index {
			at++
		}
		if at < len(matched) && matched[at] == index {
			return spanMatch
		}
		if index < namespace {
			return spanNamespace
		}
		return spanTitle
	}

	// Runs of equally drawn runes are rendered in one call each, which keeps a
	// screenful of rows down to a handful of styled spans.
	var out strings.Builder
	for start := 0; start < len(runes); {
		current := spanAt(start)
		end := start + 1
		for end < len(runes) && spanAt(end) == current {
			end++
		}
		out.WriteString(styles[current].Render(string(runes[start:end])))
		start = end
	}
	return out.String()
}

func (m model) rule() string {
	return m.styles.rule.Render(strings.Repeat("─", m.cols()))
}

func (m model) help() string {
	return m.hint(fmt.Sprintf("enter runs · esc closes · %d commands", len(m.ranked)))
}

// hint shows the failure instead of the key hints while one is pending: the
// popup is too small to carry both.
func (m model) hint(keys string) string {
	if m.failure != "" {
		return m.styles.fail.Render(truncate(m.failure, m.cols()))
	}
	return m.styles.meta.Render(keys)
}

func truncate(text string, width int) string {
	if width < 1 {
		width = 1
	}
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}
