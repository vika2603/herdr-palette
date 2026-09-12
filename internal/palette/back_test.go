package palette

import (
	"context"
	"testing"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin"
	"github.com/vika2603/herdr-client/plugin/plugintest"
)

// backEnv is what the action reads: the recent order, and the place it was
// invoked from.
func backEnv(t *testing.T, server *plugintest.Server, recent []string, opts ...plugintest.Option) *plugin.Env {
	t.Helper()
	env := server.Env(append([]plugintest.Option{plugintest.StateDir(t.TempDir())}, opts...)...)
	if err := env.WriteStateJSON(recentFile, recentState{IDs: recent}); err != nil {
		t.Fatalf("writing the recent order: %v", err)
	}
	return env
}

// where is the workspace, tab and pane the palette's own key was pressed in.
func where() []plugintest.Option {
	return []plugintest.Option{
		plugintest.Workspace("w1"),
		plugintest.Tab("w1:t1"),
		plugintest.FocusedPane("w1:p1"),
	}
}

func lastCall(t *testing.T, server *plugintest.Server) plugintest.Call {
	t.Helper()
	calls := server.Calls()
	if len(calls) == 0 {
		t.Fatal("the action made no call at all")
	}
	return calls[len(calls)-1]
}

func TestBackFocusesTheLastPlaceThePaletteWentTo(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodSessionSnapshot, snapshot()).
		Reply(herdr.MethodPaneFocus, herdr.PaneInfoResponse{})
	env := backEnv(t, server, []string{"config:prefix+f", "pane:w2:p1", "workspace:w2"}, where()...)

	if err := Back(context.Background(), env.Client(), env); err != nil {
		t.Fatalf("Back() = %v", err)
	}

	call := lastCall(t, server)
	if call.Method != herdr.MethodPaneFocus {
		t.Fatalf("called %q, want the pane to be focused rather than a command to be run", call.Method)
	}
	var params herdr.PaneTarget
	decode(t, call.Params, &params)
	if params.PaneID != "w2:p1" {
		t.Errorf("focused %q, want the most recent pane the palette went to", params.PaneID)
	}
}

// Going back to where the key was pressed would not move, so the workspace,
// the tab and the pane the action was invoked from are all passed over.
func TestBackSkipsWhereTheSessionAlreadyIs(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodSessionSnapshot, snapshot()).
		Reply(herdr.MethodWorkspaceFocus, herdr.WorkspaceInfoResponse{})
	recent := []string{"pane:w1:p1", "tab:w1:t1", "workspace:w1", "workspace:w2"}
	env := backEnv(t, server, recent, where()...)

	if err := Back(context.Background(), env.Client(), env); err != nil {
		t.Fatalf("Back() = %v", err)
	}

	call := lastCall(t, server)
	if call.Method != herdr.MethodWorkspaceFocus {
		t.Fatalf("called %q, want the first place that is not the current one", call.Method)
	}
	var params herdr.WorkspaceTarget
	decode(t, call.Params, &params)
	if params.WorkspaceID != "w2" {
		t.Errorf("focused %q, want the other workspace", params.WorkspaceID)
	}
}

// A pane the recent order still names may be closed, and an id written in an
// earlier session names nothing at all.
func TestBackSkipsATargetThatIsGone(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodSessionSnapshot, snapshot()).
		Reply(herdr.MethodTabFocus, herdr.TabInfoResponse{})
	env := backEnv(t, server, []string{"pane:w9:p9", "workspace:w9", "tab:w2:t1"}, where()...)

	if err := Back(context.Background(), env.Client(), env); err != nil {
		t.Fatalf("Back() = %v", err)
	}

	call := lastCall(t, server)
	if call.Method != herdr.MethodTabFocus {
		t.Fatalf("called %q, want the first target that is still open", call.Method)
	}
	var params herdr.TabTarget
	decode(t, call.Params, &params)
	if params.TabID != "w2:t1" {
		t.Errorf("focused %q, want the open tab", params.TabID)
	}
}

// The action has no popup to show a reason in, so an empty list of candidates
// has to reach the user some other way.
func TestBackWithNowhereToGoTellsTheUser(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodSessionSnapshot, snapshot()).
		Reply(herdr.MethodNotificationShow, herdr.NotificationShowResponse{})
	env := backEnv(t, server, []string{"config:prefix+f", "pane:w9:p9"}, where()...)

	if err := Back(context.Background(), env.Client(), env); err != nil {
		t.Fatalf("Back() = %v", err)
	}

	call := lastCall(t, server)
	if call.Method != herdr.MethodNotificationShow {
		t.Fatalf("called %q, want the user to be told there is nowhere to go", call.Method)
	}
	var params herdr.NotificationShowParams
	decode(t, call.Params, &params)
	if params.Title == "" {
		t.Error("the notification carries no title, which herdr requires")
	}
}

func TestBackReportsAFocusThatFails(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodSessionSnapshot, snapshot()).
		Fail(herdr.MethodPaneFocus, "internal", "the pane could not be focused").
		Reply(herdr.MethodNotificationShow, herdr.NotificationShowResponse{})
	env := backEnv(t, server, []string{"pane:w2:p1"}, where()...)

	if err := Back(context.Background(), env.Client(), env); err == nil {
		t.Fatal("Back() reported no error although the focus was refused")
	}
	if lastCall(t, server).Method != herdr.MethodNotificationShow {
		t.Error("a failure outside the popup was not reported to the user")
	}
}

// The action follows the ids the palette wrote down, so the rows that go
// somewhere and the ids this action reads have to keep agreeing.
func TestBackFollowsTheIDsTheRowsThatGoSomewhereCarry(t *testing.T) {
	for _, entry := range sessionEntries(snapshot().Snapshot) {
		if _, ok := backTargetOf(entry.ID); !ok {
			t.Errorf("%q goes somewhere the action cannot go back to", entry.ID)
		}
	}
}
