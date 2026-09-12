package palette

import (
	"strings"
	"testing"
)

func entries() []Entry {
	return []Entry{
		{ID: "a", Title: "Split pane right", Type: "Herdr"},
		{ID: "b", Title: "Split pane down", Type: "Herdr"},
		{ID: "c", Title: "New tab", Type: "Herdr"},
		{ID: "d", Title: "Rename workspace", Type: "Herdr"},
		{ID: "e", Title: "Manage machines", Type: "Machine Manager", Search: "herdr.machine-manager"},
	}
}

func ids(ranked []Ranked) []string {
	out := make([]string, 0, len(ranked))
	for _, r := range ranked {
		out = append(out, r.Entry.ID)
	}
	return out
}

func TestRankPrefersWordStartsAndRuns(t *testing.T) {
	ranked := Rank(entries(), "spr", nil)
	if len(ranked) == 0 {
		t.Fatal(`Rank(…, "spr") matched nothing`)
	}
	if got := ranked[0].Entry.ID; got != "a" {
		t.Errorf(`Rank(…, "spr") ranked %q first, want "a" (herdr: split pane right)`, got)
	}
}

func TestRankDropsEntriesTheQueryCannotMatch(t *testing.T) {
	ranked := Rank(entries(), "zzz", nil)
	if len(ranked) != 0 {
		t.Errorf(`Rank(…, "zzz") = %v, want no match`, ids(ranked))
	}
}

func TestRankMatchesThroughTheSearchText(t *testing.T) {
	ranked := Rank(entries(), "herdr.machine", nil)
	if len(ranked) == 0 {
		t.Fatal("a query naming the plugin matched nothing")
	}
	if got := ranked[0].Entry.ID; got != "e" {
		t.Errorf("ranked %q first, want the Machine Manager action", got)
	}
}

func TestRankScoresANameMatchAboveASearchMatch(t *testing.T) {
	list := []Entry{
		{ID: "search", Title: "Unrelated", Type: "Machine Manager", Search: "new tab"},
		{ID: "name", Title: "New tab", Type: "Herdr"},
	}
	ranked := Rank(list, "new tab", nil)
	if got := ranked[0].Entry.ID; got != "name" {
		t.Errorf("ranked %q first, want the row the query reads off", got)
	}
}

func TestRankPutsRecentFirstWithoutAQuery(t *testing.T) {
	ranked := Rank(entries(), "", []string{"c", "d"})
	if got := ids(ranked)[:2]; got[0] != "c" || got[1] != "d" {
		t.Errorf("empty query ordered %v first, want the recent ids in order", got)
	}
	if len(ranked) != len(entries()) {
		t.Errorf("empty query kept %d entries, want all %d", len(ranked), len(entries()))
	}
}

func TestRecentDoesNotOutrankAClearlyBetterMatch(t *testing.T) {
	ranked := Rank(entries(), "spr", []string{"c"})
	if got := ranked[0].Entry.ID; got != "a" {
		t.Errorf("ranked %q first, want the match to beat the recent entry", got)
	}
}

// The highlighted positions index the row as it is drawn, so a match found
// through the search text must be shifted back onto the name.
func TestMatchedPositionsAreNameRelative(t *testing.T) {
	entry := Entry{ID: "e", Title: "Manage machines", Type: "Machine Manager", Search: "herdr.machine-manager"}
	ranked := Rank([]Entry{entry}, "herdr.machine", nil)
	if len(ranked) != 1 {
		t.Fatalf("Rank() returned %d entries, want 1", len(ranked))
	}
	for _, at := range ranked[0].Matched {
		if at < 0 || at >= len([]rune(entry.Name())) {
			t.Errorf("matched index %d is outside the row", at)
		}
	}
}

func TestRankDoesNotMatchScatteredLetters(t *testing.T) {
	ranked := Rank(entries(), "spl", nil)

	for _, r := range ranked {
		if !strings.HasPrefix(r.Entry.Title, "Split") {
			t.Errorf(`Rank(…, "spl") included %q, want only the split commands`, r.Entry.Title)
		}
	}
	if len(ranked) != 2 {
		t.Errorf(`Rank(…, "spl") returned %d entries, want the two split commands`, len(ranked))
	}
}

func TestRankMatchesInitials(t *testing.T) {
	ranked := Rank(entries(), "spr", nil)
	if len(ranked) != 1 {
		t.Fatalf(`Rank(…, "spr") returned %d entries, want 1`, len(ranked))
	}
	if got := ranked[0].Entry.ID; got != "a" {
		t.Errorf("matched %q, want the initials of herdr: split pane right", got)
	}
}

func TestRankMatchesWordsInAnyOrder(t *testing.T) {
	ranked := Rank(entries(), "pane split", nil)
	if len(ranked) != 2 {
		t.Fatalf(`Rank(…, "pane split") returned %d entries, want both split commands`, len(ranked))
	}
}

func TestRankPrefersAWordStart(t *testing.T) {
	list := []Entry{
		{ID: "inside", Title: "Unsplit the layout", Type: "Herdr"},
		{ID: "start", Title: "Split pane right", Type: "Herdr"},
	}
	ranked := Rank(list, "split", nil)
	if got := ranked[0].Entry.ID; got != "start" {
		t.Errorf("ranked %q first, want the title that starts with the word", got)
	}
}

// The positions are what the row highlights, so they have to cover the query
// and index the row as it is drawn.
func TestRankReportsWhereTheQueryMatched(t *testing.T) {
	entry := Entry{ID: "a", Title: "split pane right", Type: "Herdr"}

	ranked := Rank([]Entry{entry}, "split", nil)
	if len(ranked) != 1 {
		t.Fatalf("Rank() returned %d entries, want 1", len(ranked))
	}
	name := []rune(entry.Name())
	var matched string
	for _, at := range ranked[0].Matched {
		matched += string(name[at])
	}
	if matched != "split" {
		t.Errorf("the row highlights %q, want the query", matched)
	}
}

func TestEveryWordOfTheQueryIsHighlighted(t *testing.T) {
	entry := Entry{ID: "a", Title: "split pane right", Type: "Herdr"}

	ranked := Rank([]Entry{entry}, "right split", nil)
	if len(ranked) != 1 {
		t.Fatalf("Rank() returned %d entries, want 1", len(ranked))
	}
	name := []rune(entry.Name())
	var matched string
	for _, at := range ranked[0].Matched {
		matched += string(name[at])
	}
	if matched != "splitright" {
		t.Errorf("the row highlights %q, want both words in the order they are drawn", matched)
	}
}

// A row matched on text it does not show has nothing to highlight, so the
// matched text comes back with it.
func TestAMatchInTheSearchTextIsReported(t *testing.T) {
	entry := Entry{
		ID:     "e",
		Title:  "manage machines",
		Type:   "Machine Manager",
		Search: "herdr.machine-manager",
	}

	ranked := Rank([]Entry{entry}, "herdr.machine", nil)
	if len(ranked) != 1 {
		t.Fatalf("Rank() returned %d entries, want 1", len(ranked))
	}
	if ranked[0].Detail != entry.Search {
		t.Errorf("detail = %q, want the text the query matched", ranked[0].Detail)
	}

	search := []rune(entry.Search)
	var matched string
	for _, at := range ranked[0].DetailMatched {
		matched += string(search[at])
	}
	if matched != "herdr.machine" {
		t.Errorf("the detail highlights %q, want the query", matched)
	}
	if len(ranked[0].Matched) != 0 {
		t.Error("the row itself was highlighted, although the query is not in it")
	}
}

// An empty query scores every row the same, so what decides the order is the
// group: a session holds as many rows that go somewhere as it has panes, and
// the palette opens on what it is for.
func TestAnEmptyQueryLeadsWithTheCommands(t *testing.T) {
	entries := []Entry{
		{ID: "pane:p1", Title: "go to shell", Type: "Agent", Goes: true},
		{ID: "herdr:tab.new", Title: "new tab", Type: "Herdr"},
		{ID: "pane:p2", Title: "go to nvim", Type: "Pane", Goes: true},
		{ID: "config:x", Title: "open git jump", Type: TypeCustom},
	}

	ranked := Rank(entries, "", nil)
	if len(ranked) != len(entries) {
		t.Fatalf("Rank() kept %d of %d rows on an empty query", len(ranked), len(entries))
	}
	for i, r := range ranked {
		if r.Entry.Goes && i < 2 {
			t.Errorf("row %d (%q) goes somewhere, want the commands first", i, r.Entry.Name())
		}
		if !r.Entry.Goes && i >= 2 {
			t.Errorf("row %d (%q) runs a command, want it above the rows that go somewhere", i, r.Entry.Name())
		}
	}
}

// A row run recently scores above its group, so the way back to a pane just
// left is still short.
func TestARecentlyUsedRowOutranksItsGroup(t *testing.T) {
	entries := []Entry{
		{ID: "herdr:tab.new", Title: "new tab", Type: "Herdr"},
		{ID: "pane:p1", Title: "go to shell", Type: "Agent", Goes: true},
	}

	ranked := Rank(entries, "", []string{"pane:p1"})
	if ranked[0].Entry.ID != "pane:p1" {
		t.Errorf("the list leads with %q, want the row just used", ranked[0].Entry.ID)
	}
}
