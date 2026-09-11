package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Colors are ANSI indexes rather than hex, so the popup follows the terminal
// theme herdr is rendered in.
var (
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	titleStyle    = lipgloss.NewStyle().Bold(true)
	matchStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	thumbStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
)

// Sizes used until the first resize message arrives.
const (
	defaultCols = 72
	defaultRows = 12
)

// chrome is the row budget the list does not get: the query line, the two
// rules, and the help line.
const chrome = 4

// headerRows is how many lines precede the first command row: the query line
// and the rule under it. A click's row is counted from there.
const headerRows = 2

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
	rule := dimStyle.Render(strings.Repeat("─", m.cols()))
	lines := []string{m.query.View(), rule}

	rows := m.rows()
	if len(m.ranked) == 0 {
		lines = append(lines, dimStyle.Render("  no command matches"))
		rows--
	}
	for i := m.offset; i < len(m.ranked) && i < m.offset+rows; i++ {
		lines = append(lines, m.row(i))
	}
	for len(lines) < rows+2 {
		lines = append(lines, "")
	}

	lines = append(lines, rule, m.help())
	return strings.Join(lines, "\n")
}

func (m model) row(index int) string {
	ranked := m.ranked[index]
	selected := index == m.cursor

	marker := "  "
	if selected {
		marker = selectedStyle.Render("▌ ")
	}

	detail := ranked.Entry.Detail
	// The title gets whatever the marker, the detail column, a gap and the
	// scrollbar leave.
	width := m.cols() - lipgloss.Width(detail) - 5
	title := highlight(truncate(ranked.Entry.Title, width), ranked.Matched, selected)

	gap := m.cols() - 3 - lipgloss.Width(title) - lipgloss.Width(detail)
	if gap < 1 {
		gap = 1
	}
	return marker + title + strings.Repeat(" ", gap) + dimStyle.Render(detail) + m.scrollbar(index-m.offset)
}

// scrollbar draws where the visible window sits in the whole list, one column
// at the right edge. It stays blank while everything fits, so rows do not
// shift as the query narrows the list.
func (m model) scrollbar(row int) string {
	rows, total := m.rows(), len(m.ranked)
	if total <= rows {
		return " "
	}

	size := max(1, rows*rows/total)
	start := m.offset * rows / total
	// The thumb has to reach the bottom on the last page, which integer
	// division alone does not guarantee.
	if m.offset+rows >= total {
		start = rows - size
	}
	if row >= start && row < start+size {
		return thumbStyle.Render("█")
	}
	return dimStyle.Render("│")
}

// highlight bolds the runes the query matched.
func highlight(title string, matched []int, selected bool) string {
	base := lipgloss.NewStyle()
	if selected {
		base = titleStyle
	}
	if len(matched) == 0 {
		return base.Render(title)
	}

	hit := make(map[int]bool, len(matched))
	for _, at := range matched {
		hit[at] = true
	}

	var out strings.Builder
	for i, r := range []rune(title) {
		if hit[i] {
			out.WriteString(matchStyle.Render(string(r)))
			continue
		}
		out.WriteString(base.Render(string(r)))
	}
	return out.String()
}

func (m model) inputView() string {
	rule := dimStyle.Render(strings.Repeat("─", m.cols()))
	lines := []string{
		titleStyle.Render(m.pending.Title),
		dimStyle.Render(m.pending.Input.Label),
		m.input.View(),
	}
	for len(lines) < m.rows()+2 {
		lines = append(lines, "")
	}
	lines = append(lines, rule, m.hint("enter runs · esc goes back"))
	return strings.Join(lines, "\n")
}

func (m model) help() string {
	return m.hint(fmt.Sprintf("enter runs · esc closes · %d commands", len(m.ranked)))
}

// hint shows the failure instead of the key hints while one is pending: the
// popup is too small to carry both.
func (m model) hint(keys string) string {
	if m.failure != "" {
		return errStyle.Render(truncate(m.failure, m.cols()))
	}
	return dimStyle.Render(keys)
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
