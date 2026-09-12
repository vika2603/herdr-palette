package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/vika2603/herdr-palette/internal/palette"
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

	// status is what an agent is doing, by herdr's name for it, and selected
	// the same over the band behind the selected row.
	status         map[string]lipgloss.Style
	selectedStatus map[string]lipgloss.Style
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

		status:         statusStyles(colours, lipgloss.NewStyle()),
		selectedStatus: statusStyles(colours, selected),
	}
}

func statusStyles(colours theme.Theme, base lipgloss.Style) map[string]lipgloss.Style {
	styles := make(map[string]lipgloss.Style, len(colours.Status))
	for status, colour := range colours.Status {
		styles[status] = base.Foreground(colour)
	}
	return styles
}

// detailStyle is the colour of the text beside a row: what an agent is doing
// has one of its own, anything else is as dim as the key column.
func (s styles) detailStyle(status string, selected bool) lipgloss.Style {
	styles, fallback := s.status, s.meta
	if selected {
		styles, fallback = s.selectedStatus, s.selectedMeta
	}
	if style, ok := styles[status]; ok {
		return style
	}
	return fallback
}

// Sizes used until the first resize message arrives.
const (
	defaultCols = 72
	defaultRows = 12
)

// searchPlaceholder stands in the empty query line. A list of targets says
// what it is collecting there instead.
const searchPlaceholder = "Search commands"

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
	// detailShare is the fraction of a row the matched search text may take,
	// and minDetail the width below which it is left out: what the row is
	// comes first, and a couple of letters of context explain nothing.
	detailShare = 3
	minDetail   = 10
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
		lines = append(lines, m.styles.meta.Render(m.empty()))
		rows--
	}
	for i := m.offset; i < len(m.ranked) && i < m.offset+rows; i++ {
		lines = append(lines, m.row(i))
	}
	for len(lines) < rows+headerRows {
		lines = append(lines, "")
	}

	lines = append(lines, rule, m.footer())
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

	// The row is what the query is answered with, so it is measured first and
	// the matched search text takes what is left over.
	room := m.cols() - margins - 2 - lipgloss.Width(key)
	full := ranked.Entry.Name()
	detail, detailWidth := m.detail(ranked, room-lipgloss.Width(full)-2, selected)

	name := truncate(full, room-detailWidth)
	namespace := min(len([]rune(ranked.Entry.Namespace())), len([]rune(name)))
	rendered := m.highlight(name, namespace, ranked.Matched, selected)
	gap := max(m.cols()-margins-lipgloss.Width(rendered)-detailWidth-lipgloss.Width(key), 1)

	row := text.Render(" ") + rendered + text.Render(strings.Repeat(" ", gap)) +
		detail + meta.Render(key) + text.Render(" ")
	return row + " " + m.scrollbar(index-m.offset)
}

// detail draws what a row carries beside its title: what an agent is doing,
// the workspace a pane sits in, or the text the query matched when the row
// itself does not show it, which is what keeps that match visible.
func (m model) detail(ranked palette.Ranked, room int, selected bool) (string, int) {
	room = min(room, m.cols()/detailShare)
	if ranked.Detail == "" || room < minDetail {
		return "", 0
	}

	text := truncate(ranked.Detail, room)
	base := m.styles.detailStyle(ranked.Status, selected)
	hit := m.styles.match
	if selected {
		hit = m.styles.selectedMatch
	}
	rendered := paint(text, ranked.DetailMatched, base, hit)

	gap := m.styles.title
	if selected {
		gap = m.styles.selected
	}
	return rendered + gap.Render("  "), lipgloss.Width(rendered) + 2
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

// highlight draws a row: the namespace in front of the title dimmer than the
// title itself, with the runes the query matched picked out in both. namespace
// is how many runes the namespace takes, and matched indexes the whole row.
func (m model) highlight(text string, namespace int, matched []int, selected bool) string {
	base, hit := m.styles.title, m.styles.match
	dim := m.styles.meta
	if selected {
		base, hit, dim = m.styles.selected, m.styles.selectedMatch, m.styles.selectedMeta
	}

	runes := []rune(text)
	at := min(namespace, len(runes))
	return paint(string(runes[:at]), matched, dim, hit) +
		paint(string(runes[at:]), shift(matched, at), base, hit)
}

// paint draws text with the runes at the matched positions picked out, in runs
// of one style each, which keeps a screenful of rows down to a handful of
// styled spans.
func paint(text string, matched []int, base, hit lipgloss.Style) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}

	at := 0
	isMatch := func(index int) bool {
		for at < len(matched) && matched[at] < index {
			at++
		}
		return at < len(matched) && matched[at] == index
	}

	var out strings.Builder
	for start := 0; start < len(runes); {
		current := isMatch(start)
		end := start + 1
		for end < len(runes) && isMatch(end) == current {
			end++
		}
		style := base
		if current {
			style = hit
		}
		out.WriteString(style.Render(string(runes[start:end])))
		start = end
	}
	return out.String()
}

// shift moves the positions onto a slice of the row that starts at by.
func shift(matched []int, by int) []int {
	out := make([]int, 0, len(matched))
	for _, at := range matched {
		if at -= by; at >= 0 {
			out = append(out, at)
		}
	}
	return out
}

func (m model) rule() string {
	return m.styles.rule.Render(strings.Repeat("─", m.cols()))
}

// empty is the line that stands where the rows would be when the query leaves
// none: what was searched is what it names.
func (m model) empty() string {
	if m.choosing != nil {
		return " no match"
	}
	return " no command matches"
}

// footer is the line under the list: what is on show on the left, and what the
// keys do on the right. A failure takes the whole line instead — the popup is
// too small for both.
func (m model) footer() string {
	if m.failure != "" {
		return m.styles.fail.Render(truncate(m.failure, m.cols()))
	}

	left, back := fmt.Sprintf(" %d commands", len(m.ranked)), "close esc"
	if m.choosing != nil {
		// The command the targets belong to is no longer on the list, so the
		// footer is where it stays legible.
		left, back = " "+m.choosing.entry.Name(), "back esc"
	}

	rendered := m.styles.meta.Render(left)
	right := m.styles.meta.Render("run ⏎") + m.styles.rule.Render("  ·  ") + m.styles.meta.Render(back)
	gap := max(m.cols()-lipgloss.Width(rendered)-lipgloss.Width(right)-1, 1)

	return rendered + strings.Repeat(" ", gap) + right + " "
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
