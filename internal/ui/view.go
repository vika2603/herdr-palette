package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Colors are ANSI indexes rather than hex, so the popup follows the terminal
// theme herdr is rendered in.
var (
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	titleStyle = lipgloss.NewStyle().Bold(true)
	matchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	thumbStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))

	// The selected row is a band of background rather than a marker, so the
	// eye finds it without reading the first column. The shade matches the
	// selected row in herdr's own agents sidebar: barely off the background,
	// so a long list does not read as a grey block.
	//
	// Every segment of the row carries the background itself: a style wrapped
	// around text that already contains escape sequences would be cut short by
	// the first reset inside it.
	selectedBackground = lipgloss.AdaptiveColor{Dark: "#2C2C3A", Light: "#E6E6EE"}
	selectedText       = lipgloss.NewStyle().Background(selectedBackground).Bold(true)
	selectedMatch      = lipgloss.NewStyle().Background(selectedBackground).Foreground(lipgloss.Color("12")).Bold(true)
	selectedMeta       = lipgloss.NewStyle().Background(selectedBackground).Foreground(lipgloss.Color("7"))
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
		lines = append(lines, dimStyle.Render(" no command matches"))
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

	text, meta := lipgloss.NewStyle(), dimStyle
	if selected {
		text, meta = selectedText, selectedMeta
	}

	// The key is right-aligned against the type column and the type is
	// left-aligned, so the two meet in the middle instead of leaving a gap
	// between them. The key column is only there when something is bound.
	entryType := fmt.Sprintf("%-*s", m.typeWidth, ranked.Entry.Type)
	key, keyGap := "", ""
	if m.keyWidth > 0 {
		key = fmt.Sprintf("%*s", m.keyWidth, ranked.Entry.Key)
		keyGap = "  "
	}

	// The row is padded on both sides, and gives up two more columns to the
	// scrollbar and the blank that keeps it off the text.
	fixed := lipgloss.Width(entryType) + lipgloss.Width(key) + len(keyGap) + 6
	title := highlight(truncate(ranked.Entry.Title, m.cols()-fixed), ranked.Matched, selected)

	gap := m.cols() - 4 - lipgloss.Width(title) - lipgloss.Width(key) - len(keyGap) - lipgloss.Width(entryType)
	if gap < 1 {
		gap = 1
	}

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

	size := max(1, rows*rows/total)
	start := m.offset * rows / total
	// The thumb has to reach the bottom on the last page, which integer
	// division alone does not guarantee.
	if m.offset+rows >= total {
		start = rows - size
	}
	if row >= start && row < start+size {
		return thumbStyle.Render("┃")
	}
	return dimStyle.Render("│")
}

// highlight bolds the runes the query matched.
func highlight(title string, matched []int, selected bool) string {
	base, hit := lipgloss.NewStyle(), matchStyle
	if selected {
		base, hit = selectedText, selectedMatch
	}
	if len(matched) == 0 {
		return base.Render(title)
	}

	matches := make(map[int]bool, len(matched))
	for _, at := range matched {
		matches[at] = true
	}

	var out strings.Builder
	for i, r := range []rune(title) {
		if matches[i] {
			out.WriteString(hit.Render(string(r)))
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
