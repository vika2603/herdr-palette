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

func (m model) View() string {
	if m.pending != nil {
		return m.inputView()
	}
	return m.listView()
}

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

	// Both columns are right-aligned, so the row ends on a straight edge
	// whatever the lengths are. The key column is only there when something on
	// show is bound to a key.
	entryType := fmt.Sprintf("%*s", m.typeWidth, ranked.Entry.Type)
	key, keyGap := "", ""
	if m.keyWidth > 0 {
		key = fmt.Sprintf("%*s", m.keyWidth, ranked.Entry.Key)
		keyGap = "  "
	}

	columns := lipgloss.Width(entryType) + lipgloss.Width(key) + len(keyGap)
	title := m.highlight(truncate(ranked.Entry.Title, m.cols()-columns-margins-2), ranked.Matched, selected)

	gap := max(m.cols()-margins-lipgloss.Width(title)-columns, 1)

	row := text.Render(" ") + title + text.Render(strings.Repeat(" ", gap)) +
		meta.Render(key) + text.Render(keyGap) + meta.Render(entryType) + text.Render(" ")
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

// highlight picks out the runes the query matched.
func (m model) highlight(title string, matched []int, selected bool) string {
	base, hit := m.styles.title, m.styles.match
	if selected {
		base, hit = m.styles.selected, m.styles.selectedMatch
	}
	if len(matched) == 0 {
		return base.Render(title)
	}

	// Runs of matched and unmatched runes are rendered in one call each, which
	// keeps a screenful of highlighted titles down to a handful of styled
	// spans. Matched is in ascending order, so one cursor walks it.
	runes := []rune(title)
	var out strings.Builder
	for start, at := 0, 0; start < len(runes); {
		inMatch := at < len(matched) && matched[at] == start
		end := start
		for end < len(runes) && (at < len(matched) && matched[at] == end) == inMatch {
			if inMatch {
				at++
			}
			end++
		}

		style := base
		if inMatch {
			style = hit
		}
		out.WriteString(style.Render(string(runes[start:end])))
		start = end
	}
	return out.String()
}

func (m model) inputView() string {
	lines := []string{
		m.styles.title.Bold(true).Render(m.pending.Title),
		m.styles.meta.Render(m.pending.Input.Label),
		m.input.View(),
	}
	for len(lines) < m.rows()+headerRows {
		lines = append(lines, "")
	}
	return strings.Join(append(lines, m.rule(), m.hint("enter runs · esc goes back")), "\n")
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
