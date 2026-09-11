package palette

import (
	"strings"
	"testing"
)

func entries() []Entry {
	return []Entry{
		{ID: "a", Title: "Split pane right", Detail: "Pane"},
		{ID: "b", Title: "Split pane down", Detail: "Pane"},
		{ID: "c", Title: "New tab", Detail: "Tab"},
		{ID: "d", Title: "Rename workspace", Detail: "Workspace"},
		{ID: "e", Title: "Manage machines", Detail: "Machine Manager"},
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
		t.Errorf(`Rank(…, "spr") ranked %q first, want "a" (Split pane right)`, got)
	}
}

func TestRankDropsEntriesTheQueryCannotMatch(t *testing.T) {
	ranked := Rank(entries(), "zzz", nil)
	if len(ranked) != 0 {
		t.Errorf(`Rank(…, "zzz") = %v, want no match`, ids(ranked))
	}
}

func TestRankMatchesThroughTheDetailColumn(t *testing.T) {
	ranked := Rank(entries(), "machine manage", nil)
	if len(ranked) == 0 {
		t.Fatal("a query naming the plugin matched nothing")
	}
	if got := ranked[0].Entry.ID; got != "e" {
		t.Errorf("ranked %q first, want the Machine Manager action", got)
	}
}

func TestRankScoresATitleMatchAboveADetailMatch(t *testing.T) {
	list := []Entry{
		{ID: "detail", Title: "Unrelated", Detail: "New tab group"},
		{ID: "title", Title: "New tab", Detail: "Tab"},
	}
	ranked := Rank(list, "new tab", nil)
	if got := ranked[0].Entry.ID; got != "title" {
		t.Errorf("ranked %q first, want the title match", got)
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

func TestMatchedPositionsAreTitleRelative(t *testing.T) {
	ranked := Rank([]Entry{{ID: "e", Title: "Manage machines", Detail: "Machine Manager"}}, "mm", nil)
	if len(ranked) != 1 {
		t.Fatalf("Rank() returned %d entries, want 1", len(ranked))
	}
	for _, at := range ranked[0].Matched {
		if at < 0 || at >= len([]rune("Manage machines")) {
			t.Errorf("matched index %d is outside the title", at)
		}
	}
}

func TestRankDoesNotMatchScatteredLetters(t *testing.T) {
	ranked := Rank(entries(), "spl", nil)

	for _, r := range ranked {
		if !strings.HasPrefix(r.Entry.Title, "Split pane") {
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
		t.Errorf("matched %q, want the initials of Split pane right", got)
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
		{ID: "inside", Title: "Unsplit the layout", Detail: "Pane"},
		{ID: "start", Title: "Split pane right", Detail: "Pane"},
	}
	ranked := Rank(list, "split", nil)
	if got := ranked[0].Entry.ID; got != "start" {
		t.Errorf("ranked %q first, want the title that starts with the word", got)
	}
}
