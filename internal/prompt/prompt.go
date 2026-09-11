// Package prompt is the field the palette collects a value in: one line, in a
// popup of its own, with the terminal's own cursor on the insertion point.
//
// The cursor is what makes this a package rather than a screen of the palette.
// macOS input methods place their candidate window at the terminal cursor, and
// herdr forwards a pane's cursor to the outer terminal only while the pane
// shows one. A field that hides the cursor and paints its caret as cell
// content, which is what the Bubble Tea field does, cannot be typed into with
// an input method.
package prompt

import (
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"

	"github.com/vika2603/herdr-palette/internal/theme"
)

// Field is one prompt: what it is for, what the value means, and the text the
// editing starts from.
type Field struct {
	Title   string
	Label   string
	Initial string
}

// caret is what stands in front of the edited line, matching the palette's
// query line.
const caret = "› "

// defaultWidth is used when the pane's size is unavailable.
const defaultWidth = 60

// Ask edits the field's value until enter submits it or esc cancels it. It
// reports false when the user cancelled, and for an empty value, which every
// entry that asks for one has nothing to do with.
func Ask(in *os.File, out io.Writer, field Field, colours theme.Theme) (string, bool, error) {
	state, err := term.MakeRaw(in.Fd())
	if err != nil {
		return "", false, err
	}
	defer func() { _ = term.Restore(in.Fd(), state) }()

	view := screen{heading: heading(field, colours), width: width(in)}
	edit := line{runes: []rune(field.Initial)}
	edit.at = len(edit.runes)

	buf := make([]byte, 1024)
	var split []byte
	for {
		if _, err := io.WriteString(out, view.render(edit)); err != nil {
			return "", false, err
		}

		n, err := in.Read(buf)
		if n == 0 || err != nil {
			// A closed input is the pane going away, not a value.
			return "", false, nil
		}

		chunk := append(split, buf[:n]...)
		what, rest := edit.feed(chunk)
		split = append([]byte(nil), rest...)

		switch what {
		case submitted:
			value := string(edit.runes)
			return value, value != "", nil
		case cancelled:
			return "", false, nil
		}
	}
}

func width(in *os.File) int {
	cols, _, err := term.GetSize(in.Fd())
	if err != nil || cols <= 0 {
		return defaultWidth
	}
	return cols
}

func heading(field Field, colours theme.Theme) string {
	title := lipgloss.NewStyle().Bold(true).Render(field.Title)
	if field.Label == "" {
		return title
	}
	meta := lipgloss.NewStyle().Foreground(colours.Meta)
	return title + meta.Render(" · "+field.Label)
}

// screen draws the two lines the field shows: the heading, and the value with
// the terminal's cursor on the caret.
type screen struct {
	heading string
	width   int
}

func (s screen) render(l line) string {
	room := max(s.width-lipgloss.Width(caret), 1)
	from, to := window(l.runes, l.at, room)

	var out strings.Builder
	// Home, then each line cleared as it is drawn: the field never draws more
	// than these two lines, so there is nothing else to erase.
	out.WriteString("\x1b[H\x1b[2K")
	out.WriteString(s.heading)
	out.WriteString("\r\n\x1b[2K")
	out.WriteString(caret)
	out.WriteString(string(l.runes[from:to]))

	column := lipgloss.Width(caret) + lipgloss.Width(string(l.runes[from:l.at])) + 1
	out.WriteString("\x1b[2;" + strconv.Itoa(column) + "H")
	out.WriteString("\x1b[?25h")
	return out.String()
}

// window is the part of the value the field shows, which is all of it until
// the value outgrows the line. The caret stays inside it, with a column to
// sit in past the last rune.
func window(runes []rune, at, room int) (from, to int) {
	for lipgloss.Width(string(runes[from:at]))+1 > room {
		from++
	}
	for to = at; to < len(runes); to++ {
		if lipgloss.Width(string(runes[from:to+1]))+1 > room {
			break
		}
	}
	return from, to
}
