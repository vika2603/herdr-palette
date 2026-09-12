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
	if err := Relay(context.Background(), env.Client(), env, entry, invocation); err != nil {
		t.Fatalf("Relay() = %v", err)
	}

	var pending Pending
	if err := env.ReadStateJSON(PendingFile, &pending); err != nil {
		t.Fatalf("reading the pending entry: %v", err)
	}
	if pending.EntryID != entry.ID {
		t.Errorf("pending = %+v, want the chosen entry", pending)
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
	if err := RunPending(context.Background(), env, entries, pending); err != nil {
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
	if err := RunPending(context.Background(), env, entries, pending); err == nil {
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
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot())
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

func TestRelayPromptHandsOverWhatTheFieldShows(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionInvoke, herdr.PluginActionInvokedResponse{})
	env := server.Env(plugintest.StateDir(t.TempDir()))

	invocation := &herdr.PluginInvocationContext{TabLabel: new("shell")}
	entry := Entry{
		ID:    "herdr:tab.rename",
		Title: "rename tab",
		Type:  "Herdr",
		Input: &Input{
			Label:   "Tab name",
			Initial: func(c *herdr.PluginInvocationContext) string { return herdr.Value(c.TabLabel) },
		},
	}
	if err := RelayPrompt(context.Background(), env.Client(), env, entry, invocation); err != nil {
		t.Fatalf("RelayPrompt() = %v", err)
	}

	var pending Pending
	if err := env.ReadStateJSON(PendingFile, &pending); err != nil {
		t.Fatalf("reading the pending entry: %v", err)
	}
	if pending.Prompt == nil {
		t.Fatal("the entry was handed over to run, not to ask for its value")
	}
	if pending.Prompt.Title != "herdr: rename tab" || pending.Prompt.Label != "Tab name" {
		t.Errorf("prompt = %+v, want the entry's title and label", pending.Prompt)
	}
	if pending.Prompt.Initial != "shell" {
		t.Errorf("initial = %q, want the current label to edit", pending.Prompt.Initial)
	}
}

func TestOpenPromptOpensTheFieldWithWhatItCollects(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginPaneOpen, herdr.OKResponse{})
	env := server.Env(plugintest.StateDir(t.TempDir()))

	pending := Pending{
		EntryID: "herdr:tab.rename",
		Context: &herdr.PluginInvocationContext{TabID: new("w1:t1")},
		Prompt:  &Prompt{Title: "Rename tab", Initial: "shell"},
	}
	if err := OpenPrompt(context.Background(), env.Client(), env, pending); err != nil {
		t.Fatalf("OpenPrompt() = %v", err)
	}

	call := server.Calls()[0]
	if call.Method != herdr.MethodPluginPaneOpen {
		t.Fatalf("called %q, want the field pane to be opened", call.Method)
	}
	var params herdr.PluginPaneOpenParams
	decode(t, call.Params, &params)
	if params.Entrypoint != InputEntrypoint {
		t.Errorf("opened %q, want the input entrypoint", params.Entrypoint)
	}

	t.Setenv(PromptEnv, params.Env[PromptEnv])
	opened, ok := ReadPrompt()
	if !ok {
		t.Fatal("the field pane cannot tell what it was opened for")
	}
	if opened.EntryID != pending.EntryID || opened.Prompt.Initial != "shell" {
		t.Errorf("the field was opened for %+v, want the handed-over entry", opened)
	}
	if opened.Context == nil || opened.Context.TabID == nil {
		t.Error("the invocation context was not handed over, so the rename loses its tab")
	}
}

func TestRelayValueHandsTheEntryBackToRun(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionInvoke, herdr.PluginActionInvokedResponse{})
	env := server.Env(plugintest.StateDir(t.TempDir()))

	pending := Pending{
		EntryID: "herdr:tab.rename",
		Context: &herdr.PluginInvocationContext{},
		Prompt:  &Prompt{Title: "Rename tab"},
		PID:     1,
	}
	if err := RelayValue(context.Background(), env.Client(), env, pending, "docs"); err != nil {
		t.Fatalf("RelayValue() = %v", err)
	}

	var handed Pending
	if err := env.ReadStateJSON(PendingFile, &handed); err != nil {
		t.Fatalf("reading the pending entry: %v", err)
	}
	if handed.Input != "docs" {
		t.Errorf("input = %q, want the collected value", handed.Input)
	}
	if handed.Prompt != nil {
		t.Error("the entry was handed back with a prompt, so the field would open again")
	}
	if handed.PID != os.Getpid() {
		t.Errorf("pid = %d, want the field's own process, which holds the popup now", handed.PID)
	}
}

func TestReadPromptWithNothingToCollect(t *testing.T) {
	t.Setenv(PromptEnv, "")
	if _, ok := ReadPrompt(); ok {
		t.Error("ReadPrompt() reported a field although the pane was opened by hand")
	}
}

// The field hands the entry back with the value it collected, and the target
// picked before it has to survive that hop: the entry needs both.
func TestRelayValueKeepsThePickedTarget(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionInvoke, herdr.PluginActionInvokedResponse{})
	env := server.Env(plugintest.StateDir(t.TempDir()))

	pending := Pending{
		EntryID: "herdr:agent.prompt.any",
		Chosen:  "w1:p2",
		Context: &herdr.PluginInvocationContext{},
		Prompt:  &Prompt{Label: "Prompt"},
	}
	if err := RelayValue(context.Background(), env.Client(), env, pending, "go on"); err != nil {
		t.Fatalf("RelayValue() = %v", err)
	}

	var handed Pending
	if err := env.ReadStateJSON(PendingFile, &handed); err != nil {
		t.Fatalf("reading the pending entry: %v", err)
	}
	if handed.Chosen != "w1:p2" || handed.Input != "go on" {
		t.Errorf("pending = %+v, want the picked agent and the text typed for it", handed)
	}
	if handed.Prompt != nil {
		t.Error("the field is still pending, so it would be opened again")
	}
}
