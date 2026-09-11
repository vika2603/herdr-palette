package prompt

import (
	"strings"
	"testing"
)

// feed runs the input as one chunk per string, which is how a terminal
// delivers a key: the whole sequence in one write.
func feed(t *testing.T, start string, chunks ...string) (line, action) {
	t.Helper()
	edit := line{runes: []rune(start)}
	edit.at = len(edit.runes)

	var rest []byte
	for _, chunk := range chunks {
		what, split := edit.feed(append(rest, chunk...))
		rest = append([]byte(nil), split...)
		if what != editing {
			return edit, what
		}
	}
	return edit, editing
}

func TestTypingInsertsAtTheCaret(t *testing.T) {
	edit, _ := feed(t, "shell", "\x1b[D\x1b[D", "ok ")
	if got := string(edit.runes); got != "sheok ll" {
		t.Errorf("value = %q, want the text inserted where the caret was", got)
	}
}

func TestEnterSubmitsAndEscapeCancels(t *testing.T) {
	if _, what := feed(t, "shell", "\r"); what != submitted {
		t.Errorf("enter ended in %v, want the value submitted", what)
	}
	if _, what := feed(t, "shell", "\x1b"); what != cancelled {
		t.Errorf("escape ended in %v, want the field cancelled", what)
	}
	if _, what := feed(t, "shell", "\x03"); what != cancelled {
		t.Errorf("ctrl+c ended in %v, want the field cancelled", what)
	}
}

func TestAnArrowKeyIsNotAnEscape(t *testing.T) {
	edit, what := feed(t, "shell", "\x1b[D", "x")
	if what != editing {
		t.Fatalf("the left arrow ended in %v, want the field still open", what)
	}
	if got := string(edit.runes); got != "shelxl" {
		t.Errorf("value = %q, want the arrow to have moved the caret only", got)
	}
}

func TestEditingKeys(t *testing.T) {
	for _, test := range []struct {
		name  string
		keys  string
		start string
		want  string
	}{
		{name: "backspace", keys: "\x7f", start: "shell", want: "shel"},
		{name: "delete word", keys: "\x17", start: "one two", want: "one "},
		{name: "clear", keys: "\x15", start: "shell", want: ""},
		{name: "kill to end", keys: "\x01\x1b[C\x0b", start: "shell", want: "s"},
		{name: "delete forward", keys: "\x1b[H\x1b[3~", start: "shell", want: "hell"},
		{name: "home and end", keys: "\x01x\x05y", start: "ab", want: "xaby"},
	} {
		t.Run(test.name, func(t *testing.T) {
			edit, _ := feed(t, test.start, test.keys)
			if got := string(edit.runes); got != test.want {
				t.Errorf("value = %q, want %q", got, test.want)
			}
		})
	}
}

// An input method commits its text as UTF-8, which can arrive split across
// reads. The halves belong to one rune, not to two unreadable bytes.
func TestACharacterSplitAcrossChunksIsKept(t *testing.T) {
	edit, _ := feed(t, "", "\xe4\xb8", "\xad\xe6\x96\x87")
	if got := string(edit.runes); got != "中文" {
		t.Errorf("value = %q, want both characters", got)
	}
}

func TestTheCaretStaysInsideTheValue(t *testing.T) {
	edit, _ := feed(t, "ab", "\x1b[C\x1b[C\x1b[C")
	if edit.at != 2 {
		t.Errorf("caret at %d, want the end of the value", edit.at)
	}
	edit, _ = feed(t, "ab", strings.Repeat("\x1b[D", 5))
	if edit.at != 0 {
		t.Errorf("caret at %d, want the start of the value", edit.at)
	}
}
