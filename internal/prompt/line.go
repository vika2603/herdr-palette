package prompt

import (
	"unicode"
	"unicode/utf8"
)

// line is the text being edited and where the caret sits in it, counted in
// runes.
type line struct {
	runes []rune
	at    int
}

// action is what a chunk of input ended in.
type action int

const (
	editing action = iota
	submitted
	cancelled
)

// Control bytes the field acts on. The rest are ignored: a field that is one
// line has nothing to do with them.
const (
	ctrlA     = 0x01
	ctrlB     = 0x02
	ctrlC     = 0x03
	ctrlD     = 0x04
	ctrlE     = 0x05
	ctrlF     = 0x06
	backspace = 0x08
	ctrlK     = 0x0b
	ctrlU     = 0x15
	ctrlW     = 0x17
	escape    = 0x1b
	deleteKey = 0x7f
)

// feed applies one chunk of terminal input. It returns what the chunk ended
// in and the bytes of a character split across chunks, which belong in front
// of the next one.
func (l *line) feed(chunk []byte) (action, []byte) {
	for i := 0; i < len(chunk); {
		b := chunk[i]
		switch {
		case b == escape:
			// A lone escape cancels. A terminal writes the whole sequence of
			// a key in one go, so an escape with nothing behind it is the
			// escape key rather than half of an arrow key.
			size, k := parseEscape(chunk[i:])
			if size == 0 {
				return cancelled, nil
			}
			l.move(k)
			i += size
		case b == '\r' || b == '\n':
			return submitted, nil
		case b == ctrlC:
			return cancelled, nil
		case b == backspace || b == deleteKey:
			l.deleteBack()
			i++
		case b < 0x20:
			l.control(b)
			i++
		default:
			r, size := utf8.DecodeRune(chunk[i:])
			if r == utf8.RuneError && size <= 1 {
				if !utf8.FullRune(chunk[i:]) {
					return editing, chunk[i:]
				}
				i++
				continue
			}
			l.insert(r)
			i += size
		}
	}
	return editing, nil
}

func (l *line) control(b byte) {
	switch b {
	case ctrlA:
		l.at = 0
	case ctrlE:
		l.at = len(l.runes)
	case ctrlB:
		l.move(keyLeft)
	case ctrlF:
		l.move(keyRight)
	case ctrlD:
		l.deleteForward()
	case ctrlK:
		l.runes = l.runes[:l.at]
	case ctrlU:
		l.runes = append([]rune{}, l.runes[l.at:]...)
		l.at = 0
	case ctrlW:
		l.deleteWord()
	}
}

func (l *line) insert(r rune) {
	l.runes = append(l.runes[:l.at], append([]rune{r}, l.runes[l.at:]...)...)
	l.at++
}

func (l *line) deleteBack() {
	if l.at == 0 {
		return
	}
	l.runes = append(l.runes[:l.at-1], l.runes[l.at:]...)
	l.at--
}

func (l *line) deleteForward() {
	if l.at >= len(l.runes) {
		return
	}
	l.runes = append(l.runes[:l.at], l.runes[l.at+1:]...)
}

// deleteWord removes the run of spaces before the caret and the word in front
// of it, which is what ctrl+w does in a shell.
func (l *line) deleteWord() {
	end := l.at
	for end > 0 && unicode.IsSpace(l.runes[end-1]) {
		end--
	}
	for end > 0 && !unicode.IsSpace(l.runes[end-1]) {
		end--
	}
	l.runes = append(l.runes[:end], l.runes[l.at:]...)
	l.at = end
}

func (l *line) move(k key) {
	switch k {
	case keyLeft:
		if l.at > 0 {
			l.at--
		}
	case keyRight:
		if l.at < len(l.runes) {
			l.at++
		}
	case keyHome:
		l.at = 0
	case keyEnd:
		l.at = len(l.runes)
	case keyDelete:
		l.deleteForward()
	}
}

// key is a key that arrives as an escape sequence.
type key int

const (
	keyNone key = iota
	keyLeft
	keyRight
	keyHome
	keyEnd
	keyDelete
)

// parseEscape reads the escape sequence at the front of chunk, returning how
// many bytes it took and the key it stands for. A size of zero means the
// escape stands alone.
func parseEscape(chunk []byte) (int, key) {
	if len(chunk) < 2 {
		return 0, keyNone
	}
	if chunk[1] != '[' && chunk[1] != 'O' {
		// Alt and the rest of the two-byte sequences: nothing to do, but the
		// key itself must not reach the value.
		return 2, keyNone
	}

	end := 2
	for end < len(chunk) && chunk[end] >= 0x20 && chunk[end] <= 0x3f {
		end++
	}
	if end >= len(chunk) {
		// Unterminated: the whole chunk is the sequence.
		return len(chunk), keyNone
	}

	switch string(chunk[2 : end+1]) {
	case "D":
		return end + 1, keyLeft
	case "C":
		return end + 1, keyRight
	case "H", "1~", "7~":
		return end + 1, keyHome
	case "F", "4~", "8~":
		return end + 1, keyEnd
	case "3~":
		return end + 1, keyDelete
	}
	return end + 1, keyNone
}
