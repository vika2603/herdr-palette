package catalog

import (
	"testing"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin"
	"github.com/vika2603/herdr-client/plugin/plugintest"
)

// withAgent is the session the layout is saved from: the first of the two
// panes runs an agent, the second is a plain shell.
func withAgent() herdr.SessionSnapshot {
	s := snapshot()
	for i := range s.Panes {
		if s.Panes[i].PaneID == "p1" {
			s.Panes[i].Agent = new("claude")
		}
	}
	return s
}

// applied is what herdr answers a layout with: the same shape, opened as new
// panes, which is how the saved agents are lined up with them.
func applied() herdr.LayoutApplyResponse {
	return herdr.LayoutApplyResponse{Layout: herdr.LayoutDescription{
		Root: herdr.LayoutNodeSplit{
			Direction: herdr.SplitDirectionRight,
			Ratio:     0.5,
			First:     herdr.LayoutNodePane{PaneID: new("n1"), Cwd: new("/repo")},
			Second:    herdr.LayoutNodePane{PaneID: new("n2"), Cwd: new("/repo")},
		},
	}}
}

func layoutServer(t *testing.T) (*plugintest.Server, *plugin.Env) {
	t.Helper()
	s := plugintest.NewServer(t).
		Reply(herdr.MethodSessionSnapshot, herdr.SessionSnapshotResponse{Snapshot: withAgent()}).
		Reply(herdr.MethodLayoutExport, herdr.LayoutExportResponse{Layout: herdr.LayoutDescription{Root: exportedLayout()}}).
		Reply(herdr.MethodLayoutApply, applied()).
		Reply(herdr.MethodAgentStart, herdr.AgentStartedResponse{})
	return s, s.Env(plugintest.StateDir(t.TempDir()))
}

// An exported tree carries a pane's directory but not what is running in it,
// so a layout applied again would come back as a row of shells. What each pane
// was running is saved beside the tree and started in the pane that took its
// place.
func TestALayoutBringsItsAgentsBack(t *testing.T) {
	s, env := layoutServer(t)

	if err := execute(t, env, "herdr:layout.save", step{input: "work"}, fullContext()); err != nil {
		t.Fatalf("saving the layout: %v", err)
	}
	if err := execute(t, env, "herdr:layout.apply", step{chosen: "work"}, fullContext()); err != nil {
		t.Fatalf("opening the layout: %v", err)
	}

	var started herdr.AgentStartParams
	decode(t, paramsOf(t, s, herdr.MethodAgentStart), &started)
	if started.Kind != "claude" {
		t.Errorf("started %q, want what the pane was running when it was saved", started.Kind)
	}
	// The agent was in the first pane, so it belongs in the pane that opened
	// in the first pane's place, not in the one beside it.
	if started.PaneID != "n1" {
		t.Errorf("started in %q, want the pane that took the place of the one it was saved in", started.PaneID)
	}

	var starts int
	for _, call := range s.Calls() {
		if call.Method == herdr.MethodAgentStart {
			starts++
		}
	}
	if starts != 1 {
		t.Errorf("started %d agents, want one: the other pane was a plain shell", starts)
	}
}

// A layout of plain panes saves no agents, so opening one starts nothing.
func TestALayoutOfPlainPanesStartsNothing(t *testing.T) {
	s := plugintest.NewServer(t).
		Reply(herdr.MethodSessionSnapshot, herdr.SessionSnapshotResponse{Snapshot: snapshot()}).
		Reply(herdr.MethodLayoutExport, herdr.LayoutExportResponse{Layout: herdr.LayoutDescription{Root: exportedLayout()}}).
		Reply(herdr.MethodLayoutApply, applied())
	env := s.Env(plugintest.StateDir(t.TempDir()))

	if err := execute(t, env, "herdr:layout.save", step{input: "shells"}, fullContext()); err != nil {
		t.Fatalf("saving the layout: %v", err)
	}
	// The mock has no answer for agent.start, so starting one would fail here.
	if err := execute(t, env, "herdr:layout.apply", step{chosen: "shells"}, fullContext()); err != nil {
		t.Fatalf("opening the layout: %v", err)
	}
}

// The arrangement is already open by the time the agents are started, so one
// that will not start is reported without taking the tab down with it.
func TestAnAgentThatWillNotStartIsReportedNotHidden(t *testing.T) {
	s := plugintest.NewServer(t).
		Reply(herdr.MethodSessionSnapshot, herdr.SessionSnapshotResponse{Snapshot: withAgent()}).
		Reply(herdr.MethodLayoutExport, herdr.LayoutExportResponse{Layout: herdr.LayoutDescription{Root: exportedLayout()}}).
		Reply(herdr.MethodLayoutApply, applied()).
		Fail(herdr.MethodAgentStart, "agent_unavailable", "claude is not installed")
	env := s.Env(plugintest.StateDir(t.TempDir()))

	if err := execute(t, env, "herdr:layout.save", step{input: "work"}, fullContext()); err != nil {
		t.Fatalf("saving the layout: %v", err)
	}
	err := execute(t, env, "herdr:layout.apply", step{chosen: "work"}, fullContext())
	if err == nil {
		t.Fatal("an agent that would not start was not reported")
	}
	if !saw(s, herdr.MethodLayoutApply) {
		t.Error("the arrangement was not opened")
	}
}
