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

	invocation := &herdr.PluginInvocationContext{WorkspaceID: new("w1")}
	entry := Entry{ID: "config:prefix+f", Title: "Open git jump"}
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
	pending, ok := ReadPending(env)
	if !ok {
		t.Fatal("ReadPending() found nothing to run")
	}
	if _, err := os.Stat(env.StatePath(PendingFile)); !errors.Is(err, os.ErrNotExist) {
		t.Error("the pending file is still there, so the command would run again")
	}
	if err := RunPending(context.Background(), env.Client(), entries, pending); err != nil {
		t.Fatalf("RunPending() = %v", err)
	}
	if got != "value" {
		t.Errorf("the entry ran with %q, want the handed-over input", got)
	}
}

func TestReadPendingWithNothingPending(t *testing.T) {
	server := plugintest.NewServer(t)
	env := server.Env(plugintest.StateDir(t.TempDir()))

	if _, ok := ReadPending(env); ok {
		t.Error("ReadPending() reported work although the action was invoked by hand")
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
	pending, _ := ReadPending(env)
	if err := RunPending(context.Background(), env.Client(), entries, pending); err == nil {
		t.Fatal("RunPending() reported no error although the entry failed")
	}
	if len(server.Calls()) == 0 || server.Calls()[0].Method != herdr.MethodNotificationShow {
		t.Error("a failure outside the popup was not reported to the user")
	}
}

// A plugin action's refusal happens in the plugin's own process, so there is
// nothing for the palette to try; everything else is tried first and relayed
// only when herdr answers ui_busy.
func TestOnlyPluginActionsAreRelayedWithoutTrying(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList())
	entries := load(t, server)

	action, _ := find(entries, "plugin:herdr.machine-manager/open")
	if !action.AlwaysRelay {
		t.Error("a plugin action is not handed over, so one that opens a popup would be refused")
	}
	for _, id := range []string{"config:prefix+f", "config:prefix+alt+g", "herdr:tab.new"} {
		if entry, ok := find(entries, id); ok && entry.AlwaysRelay {
			t.Errorf("%s is handed over without trying, which costs a round trip for nothing", id)
		}
	}
}
