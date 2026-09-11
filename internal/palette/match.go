package palette

import (
	"sort"
	"strings"
	"unicode"

	"github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"
)

const (
	// searchPenalty applies when the query only matches with the entry's
	// search text prepended: what the row shows is the stronger signal. It is
	// set against the scores fzf returns, where one matched word of a title
	// is worth roughly fifty.
	searchPenalty = 40
	// recentBonus is what the most recently run entry gains. Each older entry
	// gains one less, down to zero. It orders the list while the query is
	// empty and every score is zero, and stays small enough next to a match
	// that it does not reorder one.
	recentBonus = 8
	// Slab sizes for fzf's scoring matrices. A row of this palette is a short
	// line, so the defaults fzf uses for file lists are far more than needed.
	slab16, slab32 = 2048, 512
)

// Ranked is one entry with the query's match positions in its name, for
// highlighting, and the score that ordered it.
type Ranked struct {
	Entry   Entry
	Matched []int
	// Detail is the entry's search text when that is where the query matched,
	// with DetailMatched indexing it. A row whose match is in text it does not
	// show has nothing to highlight, so the text is shown next to it.
	Detail        string
	DetailMatched []int
	Score         int
}

// query is what the user typed, prepared once for a whole pass. Every word has
// to match, which is how fzf reads a query with spaces in it, and lets the
// words of a title be typed in any order.
type query struct {
	words [][]rune
	slab  *util.Slab
}

func newQuery(text string) query {
	fields := strings.Fields(text)
	words := make([][]rune, 0, len(fields))
	for _, field := range fields {
		words = append(words, []rune(fold(field)))
	}
	return query{words: words, slab: util.MakeSlab(slab16, slab32)}
}

func (q query) empty() bool { return len(q.words) == 0 }

// fold lowercases rune by rune, which keeps the positions fzf returns lined up
// with the row as it is drawn.
func fold(text string) string { return strings.Map(unicode.ToLower, text) }

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
	name := entry.Name()
	if score, matched, ok := match(fold(name), q); ok {
		return Ranked{Entry: entry, Matched: matched, Score: score}, true
	}
	if entry.Search == "" {
		return Ranked{}, false
	}
	// Where an entry came from is not part of the row, but typing it is a
	// natural way to narrow the list.
	prefix := entry.Search + " "
	if score, matched, ok := match(fold(prefix+name), q); ok {
		cut := len([]rune(prefix))
		return Ranked{
			Entry:         entry,
			Matched:       shift(matched, cut),
			Detail:        entry.Search,
			DetailMatched: before(matched, cut),
			Score:         score - searchPenalty,
		}, true
	}
	return Ranked{}, false
}

// match scores every word of the query against hay with fzf's own matcher and
// returns the matched rune positions. hay is already folded; the words are
// folded when the query is prepared.
func match(hay string, q query) (int, []int, bool) {
	chars := util.ToChars([]byte(hay))

	score := 0
	var matched []int
	for _, word := range q.words {
		result, positions := algo.FuzzyMatchV2(true, true, true, &chars, word, true, q.slab)
		if positions == nil {
			return 0, nil, false
		}
		score += result.Score
		matched = append(matched, *positions...)
	}

	sort.Ints(matched)
	return score, matched, true
}

// before keeps the indexes that fall in front of the cut, which are the ones
// inside the search text rather than the row.
func before(indexes []int, cut int) []int {
	out := make([]int, 0, len(indexes))
	for _, i := range indexes {
		if i < cut {
			out = append(out, i)
		}
	}
	return out
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
