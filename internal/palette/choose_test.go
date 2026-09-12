package palette

import (
	"context"
	"os"
	"testing"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin/plugintest"
)

func choiceEntry() Entry {
	return Entry{
		ID:      "herdr:worktree.open",
		Title:   "open worktree workspace",
		Type:    "Herdr",
		Choices: &Choices{Label: "Worktree to open", Empty: "nothing to open"},
		Run:     func(context.Context, Exec) error { return nil },
	}
}

// A row that picks a target stands in for the command it came from, so the
// handover resolves it and the recent order counts the command.
func TestChoiceRowsKeepTheCommandsIdentity(t *testing.T) {
	rows := ChoiceEntries(choiceEntry(), []Choice{
		{Value: "/trees/spike", Title: "spike", Detail: "notes", Search: "/trees/spike"},
	})

	if len(rows) != 1 {
		t.Fatalf("built %d rows, want one per choice", len(rows))
	}
	row := rows[0]
	if row.ID != "herdr:worktree.open" {
		t.Errorf("id = %q, want the command's own", row.ID)
	}
	if row.Chosen != "/trees/spike" {
		t.Errorf("chosen = %q, want what picking the row hands the command", row.Chosen)
	}
	if row.Title != "spike" || row.Detail != "notes" || row.Search != "/trees/spike" {
		t.Errorf("row = %+v, want what the choice says", row)
	}
	if row.Choices != nil {
		t.Error("the row offers a list of its own, so picking it would ask again")
	}
}

// The list is gone by the time a handed-over entry runs, so what was picked
// travels with it as the entry's input.
func TestAHandoverCarriesWhatWasPicked(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionInvoke, herdr.PluginActionInvokedResponse{})
	env := server.Env(plugintest.StateDir(t.TempDir()))

	rows := ChoiceEntries(choiceEntry(), []Choice{{Value: "/trees/spike", Title: "spike"}})
	if err := Relay(context.Background(), env.Client(), env, rows[0], &herdr.PluginInvocationContext{}); err != nil {
		t.Fatalf("Relay() = %v", err)
	}

	var pending Pending
	if err := env.ReadStateJSON(PendingFile, &pending); err != nil {
		t.Fatalf("reading the pending entry: %v", err)
	}
	if pending.EntryID != "herdr:worktree.open" || pending.Chosen != "/trees/spike" {
		t.Errorf("pending = %+v, want the command and the worktree that was picked", pending)
	}
	if pending.PID != os.Getpid() {
		t.Errorf("pending pid = %d, want this process", pending.PID)
	}
}

// An entry that picks a target and then asks for a value keeps the field on
// the row, so the value is collected once the target is known.
func TestARowOfAnEntryThatAlsoAsksForAValueKeepsTheField(t *testing.T) {
	entry := choiceEntry()
	entry.Input = &Input{Label: "Prompt"}

	rows := ChoiceEntries(entry, []Choice{{Value: "w1:p2", Title: "reviewer"}})
	if rows[0].Input == nil || rows[0].Input.Label != "Prompt" {
		t.Fatalf("row = %+v, want the field the entry asks for", rows[0])
	}

	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionInvoke, herdr.PluginActionInvokedResponse{})
	env := server.Env(plugintest.StateDir(t.TempDir()))
	if err := RelayPrompt(context.Background(), env.Client(), env, rows[0], &herdr.PluginInvocationContext{}); err != nil {
		t.Fatalf("RelayPrompt() = %v", err)
	}

	var pending Pending
	if err := env.ReadStateJSON(PendingFile, &pending); err != nil {
		t.Fatalf("reading the pending entry: %v", err)
	}
	if pending.Chosen != "w1:p2" {
		t.Errorf("pending = %+v, want the target to outlive the list it was picked from", pending)
	}
	if pending.Prompt == nil || pending.Prompt.Label != "Prompt" {
		t.Errorf("pending = %+v, want the field to be opened for the value", pending)
	}
}
