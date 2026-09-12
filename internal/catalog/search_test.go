package catalog

import (
	"context"
	"strings"
	"testing"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin/plugintest"

	"github.com/vika2603/herdr-palette/internal/palette"
)

// Every pane's output becomes a row of its own, carrying the pane it was
// printed in: what herdr's copy mode cannot answer is which pane something was
// printed in, only where in the pane you are already in.
func TestSearchingWhatThePanesHavePrinted(t *testing.T) {
	s := server(t)
	env := s.Env(plugintest.StateDir(t.TempDir()))

	lines, err := paneLines(context.Background(), palette.Exec{Client: env.Client(), Env: env})
	if err != nil {
		t.Fatalf("paneLines() = %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("no line of any pane's output is offered")
	}

	// The mock answers every pane with the same output, which has one blank
	// line between two printed ones.
	panes := map[string]int{}
	for _, line := range lines {
		if line.Title == "" || strings.TrimSpace(line.Title) != line.Title {
			t.Errorf("row %q is blank or padded, which matches nothing worth finding", line.Title)
		}
		if line.Value == "" {
			t.Errorf("row %q says nothing about which pane printed it", line.Title)
		}
		if line.Detail == "" {
			t.Errorf("row %q shows no pane beside it", line.Title)
		}
		panes[line.Value]++
	}
	if len(panes) < 2 {
		t.Errorf("rows came from %d panes, want the whole session", len(panes))
	}
}

// The rows are read one pane at a time, and a pane that cannot be read is not
// worth losing the rest of the session's output over.
func TestAPaneThatCannotBeReadIsSkipped(t *testing.T) {
	s := plugintest.NewServer(t).
		Reply(herdr.MethodSessionSnapshot, herdr.SessionSnapshotResponse{Snapshot: snapshot()}).
		Fail(herdr.MethodPaneRead, "pane_not_found", "no such pane")
	env := s.Env(plugintest.StateDir(t.TempDir()))

	lines, err := paneLines(context.Background(), palette.Exec{Client: env.Client(), Env: env})
	if err != nil {
		t.Fatalf("paneLines() = %v, want the unreadable panes left out rather than an error", err)
	}
	if len(lines) != 0 {
		t.Errorf("offered %d rows although no pane could be read", len(lines))
	}
}

// Choosing a line goes to the pane it was printed in, which is what the search
// is for.
func TestGoingToTheLineThatWasFound(t *testing.T) {
	s := server(t)
	env := s.Env(plugintest.StateDir(t.TempDir()))

	if err := execute(t, env, "herdr:pane.search", step{chosen: "w2:p1"}, fullContext()); err != nil {
		t.Fatalf("going to the line: %v", err)
	}
	var focused herdr.PaneTarget
	decode(t, paramsOf(t, s, herdr.MethodPaneFocus), &focused)
	if focused.PaneID != "w2:p1" {
		t.Errorf("focused %q, want the pane the line was printed in", focused.PaneID)
	}
}
