package prompt

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/vika2603/herdr-palette/internal/theme"
)

func TestTheFieldPutsTheTerminalCursorOnTheCaret(t *testing.T) {
	view := screen{heading: heading(Field{Title: "Rename tab"}, theme.Defaults()), width: 40}
	edit := line{runes: []rune("shell"), at: 2}

	frame := view.render(edit)

	// Column 5: the two cells of the caret, the two runes in front of the
	// insertion point, and the column the cursor sits in.
	if !strings.Contains(frame, "\x1b[2;5H") {
		t.Errorf("frame does not place the cursor on the caret: %q", frame)
	}
	if !strings.Contains(frame, "\x1b[?25h") {
		t.Error("the frame leaves the cursor hidden, which input methods cannot follow")
	}
	if !strings.Contains(frame, "Rename tab") || !strings.Contains(frame, "shell") {
		t.Errorf("frame = %q, want the heading and the value", frame)
	}
}

func TestAWideCharacterCountsTwoColumns(t *testing.T) {
	view := screen{heading: "", width: 40}

	frame := view.render(line{runes: []rune("中文"), at: 1})

	if !strings.Contains(frame, "\x1b[2;5H") {
		t.Errorf("frame does not count the character as two columns: %q", frame)
	}
}

func TestALongValueScrollsWithTheCaret(t *testing.T) {
	runes := []rune("0123456789")

	from, to := window(runes, len(runes), 5)

	if string(runes[from:to]) != "6789" {
		t.Errorf("window = %q, want the end of the value with room for the caret", string(runes[from:to]))
	}
	if from, to = window(runes, 0, 5); string(runes[from:to]) != "0123" {
		t.Errorf("window = %q, want the start of the value", string(runes[from:to]))
	}
}

func TestTheLabelIsShownNextToTheTitle(t *testing.T) {
	got := heading(Field{Title: "Create worktree", Label: "New branch"}, theme.Defaults())
	if !strings.Contains(got, "Create worktree") || !strings.Contains(got, "New branch") {
		t.Errorf("heading = %q, want both the title and what the value means", got)
	}
}

// A heading wider than the pane wraps onto the line the value is edited on,
// which puts the cursor where the text is not.
func TestALongHeadingIsCutToThePane(t *testing.T) {
	view := screen{
		heading: heading(Field{
			Title: strings.Repeat("a name that goes on ", 5),
			Label: "and a label after it",
		}, theme.Defaults()),
		width: 40,
	}

	for _, line := range strings.Split(view.render(line{}), "\r\n") {
		if width := lipgloss.Width(stripCursor(line)); width > 40 {
			t.Errorf("a line is %d wide, want no more than 40: %q", width, line)
		}
	}
}

// stripCursor takes out the escape sequences the field ends a frame with,
// which place the terminal's own cursor and are not content.
func stripCursor(line string) string {
	for _, seq := range []string{"\x1b[?25h", "\x1b[H", "\x1b[2K"} {
		line = strings.ReplaceAll(line, seq, "")
	}
	if at := strings.Index(line, "\x1b[2;"); at >= 0 {
		if end := strings.Index(line[at:], "H"); end >= 0 {
			line = line[:at] + line[at+end+1:]
		}
	}
	return line
}
