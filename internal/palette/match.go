package palette

import (
	"slices"
	"sort"
	"strings"
	"unicode"
)

// Scores for the two shapes a query can match in.
const (
	// scoreWord is what a query word earns inside a title, and scoreWordAtStart
	// what it earns at the start of a word there: "split" reaching "Split pane"
	// is a stronger signal than "pan" reaching it.
	scoreWord        = 2
	scoreWordAtStart = 8
	// scoreInitial is what one letter of an initials match earns, and
	// scoreInitialsExact the bonus for a query that uses every word's initial
	// with none skipped, such as "spr" for "Split pane right".
	scoreInitial       = 6
	scoreInitialsExact = 4
	// startPenalty caps how much a late first match costs, so a long title is
	// not ranked out entirely.
	startPenalty = 10
	// detailPenalty applies when the query only matches with the group or
	// plugin name prepended: a title match is the stronger signal.
	detailPenalty = 20
	// recentBonus is what the most recently run entry gains. Each older entry
	// gains one less, down to zero, which keeps the bonus below a clearly
	// better title match.
	recentBonus = 12
)

// Ranked is one entry with the query's match positions in its title, for
// highlighting, and the score that ordered it.
type Ranked struct {
	Entry   Entry
	Matched []int
	Score   int
}

// query is what the user typed, prepared once for a whole pass: the words to
// find, and the same letters read as initials.
type query struct {
	words    []string
	initials []rune
}

func newQuery(text string) query {
	words := strings.Fields(strings.ToLower(text))
	return query{words: words, initials: []rune(strings.Join(words, ""))}
}

func (q query) empty() bool { return len(q.words) == 0 }

// Rank filters entries against the query and orders them, most relevant
// first. recent holds entry ids, most recently run first. An empty query
// keeps every entry and orders it by recency.
func Rank(entries []Entry, text string, recent []string) []Ranked {
	order := recentOrder(recent)
	q := newQuery(text)

	ranked := make([]Ranked, 0, len(entries))
	for _, entry := range entries {
		r, ok := rankOne(entry, q)
		if !ok {
			continue
		}
		if index, found := order[entry.ID]; found {
			r.Score += recentBonus - index
		}
		ranked = append(ranked, r)
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		if ranked[i].Entry.Type != ranked[j].Entry.Type {
			return ranked[i].Entry.Type < ranked[j].Entry.Type
		}
		return ranked[i].Entry.Title < ranked[j].Entry.Title
	})
	return ranked
}

func rankOne(entry Entry, q query) (Ranked, bool) {
	if q.empty() {
		return Ranked{Entry: entry}, true
	}
	if score, matched, ok := match(entry.Title, q); ok {
		return Ranked{Entry: entry, Matched: matched, Score: score}, true
	}
	// The type and the entry's source are not part of the title, but typing
	// either is a natural way to narrow the list.
	prefix := entry.Type + " "
	if entry.Search != "" {
		prefix += entry.Search + " "
	}
	if score, matched, ok := match(prefix+entry.Title, q); ok {
		return Ranked{
			Entry:   entry,
			Matched: shift(matched, len([]rune(prefix))),
			Score:   score - detailPenalty,
		}, true
	}
	return Ranked{}, false
}

// match scores the query against hay, case-insensitively, and returns the
// matched rune indexes. Two shapes count, in this order: every query word
// appearing as a substring, in any order, and the query read as the initials
// of hay's words. Letters merely scattered through hay are not a match, so
// "spl" does not reach "Close workspace".
func match(hay string, q query) (int, []int, bool) {
	runes := []rune(strings.ToLower(hay))
	if score, matched, ok := matchWords(runes, q.words); ok {
		return score, matched, true
	}
	return matchInitials(runes, q.initials)
}

// matchWords requires every word to appear in hay. A word is looked up at a
// word start first, so "pane" prefers "pane right" over "pane" inside another
// word.
func matchWords(runes []rune, words []string) (int, []int, bool) {
	score, first := 0, len(runes)
	var matched []int

	for _, word := range words {
		wanted := []rune(word)
		at, atWordStart := findWord(runes, wanted)
		if at < 0 {
			return 0, nil, false
		}
		if atWordStart {
			score += scoreWordAtStart
		} else {
			score += scoreWord
		}
		for i := range wanted {
			matched = append(matched, at+i)
		}
		first = min(first, at)
	}

	sort.Ints(matched)
	return score - min(first, startPenalty), matched, true
}

// findWord returns where word occurs in runes, preferring an occurrence at the
// start of a word, and whether the returned position is one.
func findWord(runes, word []rune) (int, bool) {
	fallback := -1
	for at := 0; at+len(word) <= len(runes); at++ {
		if !slices.Equal(runes[at:at+len(word)], word) {
			continue
		}
		if isWordStart(runes, at) {
			return at, true
		}
		if fallback < 0 {
			fallback = at
		}
	}
	return fallback, false
}

// matchInitials reads the query as the first letters of hay's words, allowing
// words to be skipped: "sr" still reaches "Split pane right".
func matchInitials(runes, wanted []rune) (int, []int, bool) {
	starts := wordStarts(runes)
	matched := make([]int, 0, len(wanted))
	at := 0
	for _, want := range wanted {
		for at < len(starts) && runes[starts[at]] != want {
			at++
		}
		if at == len(starts) {
			return 0, nil, false
		}
		matched = append(matched, starts[at])
		at++
	}

	score := len(wanted) * scoreInitial
	if len(wanted) == len(starts) {
		score += scoreInitialsExact
	}
	return score - min(matched[0], startPenalty), matched, true
}

func wordStarts(runes []rune) []int {
	starts := make([]int, 0, len(runes)/4+1)
	for at, r := range runes {
		if (unicode.IsLetter(r) || unicode.IsDigit(r)) && isWordStart(runes, at) {
			starts = append(starts, at)
		}
	}
	return starts
}

func isWordStart(runes []rune, at int) bool {
	if at == 0 {
		return true
	}
	prev := runes[at-1]
	return !unicode.IsLetter(prev) && !unicode.IsDigit(prev)
}

func shift(indexes []int, by int) []int {
	out := make([]int, 0, len(indexes))
	for _, i := range indexes {
		if i -= by; i >= 0 {
			out = append(out, i)
		}
	}
	return out
}

func recentOrder(recent []string) map[string]int {
	order := make(map[string]int, len(recent))
	for i, id := range recent {
		if i >= recentBonus {
			break
		}
		if _, seen := order[id]; !seen {
			order[id] = i
		}
	}
	return order
}
