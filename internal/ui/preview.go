package ui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/vika2603/herdr-client/herdr"

	"github.com/vika2603/herdr-palette/internal/palette"
)

const (
	// previewMinCols is the narrowest popup the preview is drawn in: below it
	// the list would be left too little for its own columns.
	previewMinCols = 110
	// The preview takes this share of the popup, within these bounds.
	previewShare    = 0.4
	previewMinWidth = 36
	previewMaxWidth = 64
	// previewSettle is how long the selection rests on a pane before its
	// screen is read, so moving through the list does not read every pane on
	// the way, and previewRefresh how often the screen on show is read again.
	previewSettle  = 80 * time.Millisecond
	previewRefresh = time.Second
	// cardLabel is the width of the labels on a command's card.
	cardLabel = 11
)

// preview is the last screen read for the preview, the pane it is of, and
// which read it was.
type preview struct {
	pane string
	text string
	err  error
	read int
}

// previewTickMsg asks for the preview to be read, and previewMsg carries what
// was read. Both carry the preview's sequence number, which moves on whenever
// the pane on show changes, so a read for a pane already left is dropped and
// the refresh after it is not asked for again.
//
// read numbers the reads, so a slow one does not overwrite the screen a later
// one brought, and echo marks a read made because a key went to the agent,
// which leaves the refresh to the chain already running rather than starting
// another beside it.
type (
	previewTickMsg struct{ seq int }
	previewEchoMsg struct{ seq int }
	previewMsg     struct {
		seq, read  int
		pane, text string
		err        error
		echo       bool
	}
)

// previewWidth is how wide the preview is drawn, zero when disabled or on a
// popup too narrow for one.
func (m model) previewWidth() int {
	cols := m.cols()
	if !m.previewEnabled || cols < previewMinCols {
		return 0
	}
	return min(max(int(float64(cols)*previewShare), previewMinWidth), previewMaxWidth)
}

// listWidth is what the list is drawn in, the columns the preview leaves.
func (m model) listWidth() int { return m.cols() - m.previewWidth() }

// previewTarget is the pane whose screen the preview should show, empty when
// the preview is not drawn or the selected row is about no pane.
func (m model) previewTarget() string {
	if m.replying != nil {
		return m.replying.Entry.Pane
	}
	if m.previewWidth() == 0 || m.cursor >= len(m.ranked) {
		return ""
	}
	return m.ranked[m.cursor].Entry.Pane
}

func (m model) previewAfter(wait time.Duration) tea.Cmd {
	seq := m.previewSeq
	return tea.Tick(wait, func(time.Time) tea.Msg { return previewTickMsg{seq: seq} })
}

// readPreview reads the pane's screen off the update loop, the way a command
// runs.
func (m *model) readPreview(pane string, echo bool) tea.Cmd {
	m.previewReads++
	seq, read, ctx, env := m.previewSeq, m.previewReads, m.ctx, m.env
	return func() tea.Msg {
		text, err := readScreen(ctx, env.Client(), pane)
		return previewMsg{seq: seq, read: read, pane: pane, text: text, err: err, echo: echo}
	}
}

// readScreen is what the pane shows right now. The visible screen rather than
// the scrollback: an agent asking something draws the question where the
// pane's own screen shows it, and that is what the preview is for.
func readScreen(ctx context.Context, client *herdr.Client, pane string) (string, error) {
	strip := true
	read, err := client.PaneRead(ctx, herdr.PaneReadParams{
		PaneID:    pane,
		Source:    herdr.ReadSourceVisible,
		Format:    herdr.ReadFormatText,
		StripANSI: &strip,
	})
	if err != nil {
		return "", err
	}
	return read.Read.Text, nil
}

// screenLines is the last height lines of a screen, the blank lines under
// what it shows left off. The text is drawn inside the popup, so anything
// that would move the cursor or change colours is taken out of it.
func screenLines(text string, height int) []string {
	if height <= 0 {
		return nil
	}
	text = ansi.Strip(strings.ReplaceAll(text, "\t", "    "))
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(strings.Map(printable, l), " ")
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > height {
		lines = lines[len(lines)-height:]
	}
	return lines
}

func printable(r rune) rune {
	if r < 0x20 || r == 0x7f {
		return -1
	}
	return r
}

// previewLines draws the preview, height lines of width columns: the screen
// of the pane the selected row is about, or what the selected command does.
func (m model) previewLines(width, height int) []string {
	inner := max(width-2, 1)
	var content []string
	if m.cursor < len(m.ranked) {
		ranked := m.ranked[m.cursor]
		if ranked.Entry.Pane != "" {
			content = m.screenPanel(ranked, inner, height)
		} else {
			content = m.card(ranked, inner)
		}
	}

	lines := make([]string, height)
	for i := range lines {
		if i < len(content) {
			lines[i] = " " + truncate(content[i], inner)
		}
	}
	return lines
}

// screenPanel is a pane's screen under a heading saying what is in the pane.
func (m model) screenPanel(ranked palette.Ranked, width, height int) []string {
	s := m.styles.plain
	heading := s.text.Bold(true).Render(strings.TrimPrefix(ranked.Entry.Title, palette.GoTo))
	if ranked.Status != "" {
		heading += "  " + m.detail(ranked, detailWidth(ranked), s)
	} else if ranked.Detail != "" {
		heading += s.meta.Render("  " + ranked.Detail)
	}
	lines := []string{heading, m.styles.rule.Render(strings.Repeat("─", width))}

	if m.preview.pane != ranked.Entry.Pane {
		return lines
	}
	if m.preview.err != nil {
		return append(lines, s.faint.Render("the screen could not be read"))
	}
	for _, l := range screenLines(m.preview.text, height-len(lines)) {
		lines = append(lines, s.meta.Render(truncate(l, width)))
	}
	return lines
}

// card says what the selected command is and what running it involves: the
// key it is bound to as the configuration spells it, the value it asks for,
// the list it picks from, and whether it asks first.
func (m model) card(ranked palette.Ranked, width int) []string {
	s := m.styles.plain
	entry := ranked.Entry

	heading := s.text.Bold(true).Render(strings.TrimPrefix(entry.Title, palette.GoTo))
	if from := source(entry); from != "" {
		heading = s.faint.Render(strings.ToUpper(from)+"  ") + heading
	}
	lines := []string{heading, m.styles.rule.Render(strings.Repeat("─", width))}

	field := func(label, value string, style lipgloss.Style) {
		if value != "" {
			lines = append(lines, s.faint.Render(pad(label, cardLabel))+style.Render(value))
		}
	}
	switch {
	case ranked.Status != "":
		field("state", m.detail(ranked, detailWidth(ranked), s), s.text)
	case entry.Goes():
		field("in", ranked.Detail, s.text)
	case entry.Type == "":
		// A target picked from a list: what its row says beside it.
		field("detail", ranked.Detail, s.text)
	}
	field("key", entry.Key, s.text)
	if entry.Input != nil {
		field("asks for", entry.Input.Label, s.text)
	}
	if entry.Choices != nil {
		field("picks", entry.Choices.Label, s.text)
	}
	if entry.Confirm {
		field("confirm", "asks before it runs", s.text)
	}
	if !entry.Goes() && entry.Type != "" && entry.Search != "" {
		field("from", entry.Search, s.meta)
	}
	if entry.Description != "" {
		label := "does"
		if entry.Type == palette.TypeCustom {
			label = "runs"
		}
		for i, line := range wrap(entry.Description, max(width-cardLabel, 1)) {
			if i > 0 {
				label = ""
			}
			field(label, line, s.meta)
		}
	}
	return lines
}

// wrap breaks text into lines of at most width cells, at spaces where it
// can. A command line or a description is read whole on the card rather than
// cut at the edge.
func wrap(text string, width int) []string {
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Fields(paragraph) {
			for lipgloss.Width(word) > width {
				cut := ansi.Truncate(word, width, "")
				if line != "" {
					lines, line = append(lines, line), ""
				}
				lines = append(lines, cut)
				word = strings.TrimPrefix(word, cut)
			}
			switch {
			case line == "":
				line = word
			case lipgloss.Width(line)+1+lipgloss.Width(word) <= width:
				line += " " + word
			default:
				lines, line = append(lines, line), word
			}
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func pad(text string, width int) string {
	return text + strings.Repeat(" ", max(width-lipgloss.Width(text), 0))
}
