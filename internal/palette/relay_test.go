package palette

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin/plugintest"
)

func TestRelayWritesThePendingEntryAndAsksForTheExecAction(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionInvoke, herdr.PluginActionInvokedResponse{})
	env := server.Env(plugintest.StateDir(t.TempDir()))

	invocation := &herdr.PluginInvocationContext{WorkspaceID: herdr.Ptr("w1")}
	entry := Entry{ID: "config:prefix+f", Title: "Open git jump", OpensPopup: true}
	if err := Relay(context.Background(), env.Client(), env, entry, "value", invocation); err != nil {
		t.Fatalf("Relay() = %v", err)
	}

	var pending Pending
	if err := env.ReadStateJSON(PendingFile, &pending); err != nil {
		t.Fatalf("reading the pending entry: %v", err)
	}
	if pending.EntryID != entry.ID || pending.Input != "value" {
		t.Errorf("pending = %+v, want the chosen entry and its input", pending)
	}
	if pending.PID != os.Getpid() {
		t.Errorf("pending pid = %d, want this process so the wait knows what to watch", pending.PID)
	}
	if pending.Context == nil || pending.Context.WorkspaceID == nil {
		t.Error("the invocation context was not handed over")
	}

	call := server.Calls()[0]
	if call.Method != herdr.MethodPluginActionInvoke {
		t.Fatalf("called %q, want the action invoke", call.Method)
	}
	var params herdr.PluginActionInvokeParams
	decode(t, call.Params, &params)
	if params.ActionID != ExecAction {
		t.Errorf("invoked %q, want the exec entrypoint", params.ActionID)
	}
}

func TestRunPendingRunsTheEntryAndClearsTheFile(t *testing.T) {
	server := plugintest.NewServer(t)
	env := server.Env(plugintest.StateDir(t.TempDir()))
	if err := env.WriteStateJSON(PendingFile, Pending{
		EntryID: "config:prefix+f",
		Input:   "value",
		Context: &herdr.PluginInvocationContext{},
	}); err != nil {
		t.Fatal(err)
	}

	var got string
	entries := []Entry{{
		ID: "config:prefix+f",
		Run: func(_ context.Context, e Exec) error {
			got = e.Input
			return nil
		},
	}}
	if err := RunPending(context.Background(), env.Client(), env, entries); err != nil {
		t.Fatalf("RunPending() = %v", err)
	}
	if got != "value" {
		t.Errorf("the entry ran with %q, want the handed-over input", got)
	}
	if _, err := os.Stat(env.StatePath(PendingFile)); !errors.Is(err, os.ErrNotExist) {
		t.Error("the pending file is still there, so the command would run again")
	}
}

func TestRunPendingWithNothingPending(t *testing.T) {
	server := plugintest.NewServer(t)
	env := server.Env(plugintest.StateDir(t.TempDir()))

	if err := RunPending(context.Background(), env.Client(), env, nil); err != nil {
		t.Errorf("RunPending() = %v, want an action invoked by hand to be a no-op", err)
	}
}

func TestRunPendingReportsAFailure(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodNotificationShow, herdr.NotificationShowResponse{})
	env := server.Env(plugintest.StateDir(t.TempDir()))
	if err := env.WriteStateJSON(PendingFile, Pending{EntryID: "x"}); err != nil {
		t.Fatal(err)
	}

	entries := []Entry{{
		ID:    "x",
		Title: "Open git jump",
		Run:   func(context.Context, Exec) error { return errors.New("ui_busy") },
	}}
	if err := RunPending(context.Background(), env.Client(), env, entries); err == nil {
		t.Fatal("RunPending() reported no error although the entry failed")
	}
	if len(server.Calls()) == 0 || server.Calls()[0].Method != herdr.MethodNotificationShow {
		t.Error("a failure outside the popup was not reported to the user")
	}
}

func TestPluginActionsAndPopupCommandsAreRelayed(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList())
	entries := load(t, server)

	action, _ := find(entries, "plugin:herdr.machine-manager/open")
	if !action.OpensPopup {
		t.Error("a plugin action is not relayed, so one that opens a popup would be refused")
	}
	popup, _ := find(entries, "config:prefix+f")
	if !popup.OpensPopup {
		t.Error("a configured popup command is not relayed")
	}
	pane, _ := find(entries, "config:prefix+alt+g")
	if pane.OpensPopup {
		t.Error("a configured pane command is relayed, which costs a round trip for nothing")
	}
}
