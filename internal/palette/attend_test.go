package palette

import (
	"context"
	"slices"
	"testing"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin/plugintest"
)

func agentPane(id string, status herdr.AgentStatus) herdr.PaneInfo {
	return herdr.PaneInfo{PaneID: id, Agent: new("claude"), AgentStatus: status}
}

func waitingPanes() []herdr.PaneInfo {
	return []herdr.PaneInfo{
		{PaneID: "w1:p1"},
		agentPane("w1:p2", herdr.AgentStatusDone),
		agentPane("w1:p3", herdr.AgentStatusWorking),
		agentPane("w2:p1", herdr.AgentStatusBlocked),
		agentPane("w2:p2", herdr.AgentStatusBlocked),
	}
}

func TestAttendGoesThroughWhatIsWaitingBlockedFirst(t *testing.T) {
	panes := waitingPanes()
	var visited []string
	here := "w1:p1"
	for range 4 {
		next, ok := nextWaiting(panes, here)
		if !ok {
			t.Fatalf("nothing to go to from %s", here)
		}
		visited = append(visited, next)
		here = next
	}
	// The blocked agents come first, the done one after them, and the round
	// starts again from the first.
	if want := []string{"w2:p1", "w2:p2", "w1:p2", "w2:p1"}; !slices.Equal(visited, want) {
		t.Errorf("visited %v, want %v", visited, want)
	}
}

func TestAttendHasNowhereToGoFromTheOnlyAgentWaiting(t *testing.T) {
	panes := []herdr.PaneInfo{{PaneID: "w1:p1"}, agentPane("w1:p2", herdr.AgentStatusBlocked)}
	if next, ok := nextWaiting(panes, "w1:p2"); ok {
		t.Errorf("went to %q from the only agent waiting", next)
	}
	if next, ok := nextWaiting([]herdr.PaneInfo{agentPane("w1:p2", herdr.AgentStatusWorking)}, "w1:p1"); ok {
		t.Errorf("went to %q with nothing waiting", next)
	}
}

// Going to an agent and back to the work it interrupted is two keys, so the
// pane it was pressed in becomes the place back returns to.
func TestAttendLeavesTheWayBack(t *testing.T) {
	session := herdr.SessionSnapshotResponse{Snapshot: herdr.SessionSnapshot{Panes: waitingPanes()}}
	server := plugintest.NewServer(t).
		Reply(herdr.MethodSessionSnapshot, session).
		Reply(herdr.MethodPaneFocus, herdr.PaneInfoResponse{})
	env := backEnv(t, server, []string{"pane:w9:p9"}, where()...)

	if err := Attend(context.Background(), env.Client(), env); err != nil {
		t.Fatalf("Attend() = %v", err)
	}
	var focused herdr.PaneTarget
	decode(t, lastCall(t, server).Params, &focused)
	if focused.PaneID != "w2:p1" {
		t.Errorf("focused %q, want the first blocked agent", focused.PaneID)
	}
	if got := ReadRecent(env); len(got) == 0 || got[0] != "pane:w1:p1" {
		t.Errorf("recent = %v, want the pane it was pressed in first", got)
	}
}

func TestAttendSaysWhenNothingNeedsYou(t *testing.T) {
	session := herdr.SessionSnapshotResponse{Snapshot: herdr.SessionSnapshot{Panes: []herdr.PaneInfo{
		{PaneID: "w1:p1"}, agentPane("w1:p2", herdr.AgentStatusWorking),
	}}}
	server := plugintest.NewServer(t).
		Reply(herdr.MethodSessionSnapshot, session).
		Reply(herdr.MethodNotificationShow, herdr.NotificationShowResponse{})
	env := backEnv(t, server, nil, where()...)

	if err := Attend(context.Background(), env.Client(), env); err != nil {
		t.Fatalf("Attend() = %v", err)
	}
	if call := lastCall(t, server); call.Method != herdr.MethodNotificationShow {
		t.Errorf("called %q, want a notification saying nothing is waiting", call.Method)
	}
}

// Going on from agent to agent keeps the way back to the work the first press
// interrupted.
func TestAttendKeepsTheWayBackWhileGoingOn(t *testing.T) {
	session := herdr.SessionSnapshotResponse{Snapshot: herdr.SessionSnapshot{Panes: waitingPanes()}}
	server := plugintest.NewServer(t).
		Reply(herdr.MethodSessionSnapshot, session).
		Reply(herdr.MethodPaneFocus, herdr.PaneInfoResponse{})
	state := t.TempDir()
	press := func(from string) string {
		t.Helper()
		env := server.Env(plugintest.StateDir(state), plugintest.Workspace("w1"), plugintest.FocusedPane(from))
		if err := Attend(context.Background(), env.Client(), env); err != nil {
			t.Fatalf("Attend() = %v", err)
		}
		var focused herdr.PaneTarget
		decode(t, lastCall(t, server).Params, &focused)
		return focused.PaneID
	}

	first := press("w1:p1")
	second := press(first)
	if first == second {
		t.Fatalf("the second press stayed on %s", first)
	}
	env := server.Env(plugintest.StateDir(state))
	if got := ReadRecent(env); len(got) == 0 || got[0] != "pane:w1:p1" {
		t.Errorf("recent = %v after going on to %s, want the work first", got, second)
	}
}
