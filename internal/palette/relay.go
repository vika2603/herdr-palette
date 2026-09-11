package palette

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin"
)

const (
	// ExecAction is the manifest action that runs an entry the popup handed
	// over, and PendingFile is where it finds it.
	ExecAction  = "exec"
	PendingFile = "pending.json"

	// InputEntrypoint is the manifest pane that collects a value for an entry
	// that needs one, and PromptEnv is how it learns what it is collecting.
	InputEntrypoint = "input"
	PromptEnv       = "HERDR_PALETTE_PROMPT"
)

// waitForPopup is how long the exec entrypoint waits for the palette's popup
// to go away. It is a guard against a stuck process, not a delay that is
// normally spent: the popup closes as soon as its process exits.
const waitForPopup = 3 * time.Second

// Pending is the entry the exec entrypoint runs once the popup is gone.
type Pending struct {
	EntryID string                         `json:"entry_id"`
	Input   string                         `json:"input"`
	Context *herdr.PluginInvocationContext `json:"context"`
	// PID is the process holding the popup. herdr closes a popup pane when
	// the process in it exits.
	PID int `json:"pid"`
	// Prompt is set while the entry is still missing its value: the exec
	// entrypoint opens the field for it instead of running the entry.
	Prompt *Prompt `json:"prompt,omitempty"`
}

// Prompt is what the field shows while it collects an entry's value.
type Prompt struct {
	Title   string `json:"title"`
	Label   string `json:"label"`
	Initial string `json:"initial"`
}

// Relay hands an entry to the exec entrypoint and lets the popup close.
//
// herdr allows one popup at a time, so an entry that opens one — a configured
// popup command, or a plugin action that opens its own pane — is refused with
// ui_busy while the palette is up. herdr runs an action entrypoint outside the
// popup, so the palette writes down what to run, asks for that entrypoint, and
// quits.
func Relay(ctx context.Context, client *herdr.Client, env *plugin.Env, entry Entry, invocation *herdr.PluginInvocationContext) error {
	return handOver(ctx, client, env, Pending{
		EntryID: entry.ID,
		Context: invocation,
	})
}

// RelayPrompt hands over an entry that still needs a value. The field it is
// collected in is a popup of its own, which cannot open while the palette's is
// up either, and a rename has no use for the palette's window.
func RelayPrompt(ctx context.Context, client *herdr.Client, env *plugin.Env, entry Entry, invocation *herdr.PluginInvocationContext) error {
	return handOver(ctx, client, env, Pending{
		EntryID: entry.ID,
		Context: invocation,
		Prompt: &Prompt{
			Title:   entry.Name(),
			Label:   entry.Input.Label,
			Initial: entry.Initial(invocation),
		},
	})
}

// RelayValue hands the entry back once the field has its value. The field is
// a popup as well, so the entry runs from the exec entrypoint for the same
// reason it did not run from the palette.
func RelayValue(ctx context.Context, client *herdr.Client, env *plugin.Env, pending Pending, value string) error {
	pending.Input = value
	pending.Prompt = nil
	return handOver(ctx, client, env, pending)
}

// handOver writes down what is left to do and asks for the exec entrypoint.
func handOver(ctx context.Context, client *herdr.Client, env *plugin.Env, pending Pending) error {
	pending.PID = os.Getpid()
	if err := env.WriteStateJSON(PendingFile, pending); err != nil {
		return err
	}

	pluginID := env.PluginID
	_, err := client.PluginActionInvoke(ctx, herdr.PluginActionInvokeParams{
		PluginID: &pluginID,
		ActionID: ExecAction,
		Context:  pending.Context,
	})
	return err
}

// OpenPrompt opens the field for an entry that needs a value, once the popup
// that handed it over is gone. What the field shows, and what it hands back,
// travels in the pane's environment.
func OpenPrompt(ctx context.Context, client *herdr.Client, env *plugin.Env, pending Pending) error {
	waitForExit(ctx, pending.PID, waitForPopup)

	payload, err := json.Marshal(pending)
	if err != nil {
		return err
	}
	_, err = client.PluginPaneOpen(ctx, herdr.PluginPaneOpenParams{
		PluginID:   env.PluginID,
		Entrypoint: InputEntrypoint,
		Focus:      new(true),
		Env:        map[string]string{PromptEnv: string(payload)},
	})
	return err
}

// ReadPrompt returns what the field pane was opened to collect.
func ReadPrompt() (Pending, bool) {
	var pending Pending
	raw := os.Getenv(PromptEnv)
	if raw == "" {
		return Pending{}, false
	}
	if err := json.Unmarshal([]byte(raw), &pending); err != nil || pending.Prompt == nil {
		return Pending{}, false
	}
	return pending, true
}

// ReadPending returns what Relay wrote down and clears it. Nothing pending is
// the normal outcome of the exec action being invoked by hand, and the caller
// can skip assembling the command list for it.
func ReadPending(env *plugin.Env) (Pending, bool) {
	var pending Pending
	// A missing file reads as an empty entry.
	if err := env.ReadStateJSON(PendingFile, &pending); err != nil || pending.EntryID == "" {
		return Pending{}, false
	}
	_ = os.Remove(env.StatePath(PendingFile))
	return pending, true
}

// RunPending runs the handed-over entry, after waiting for the popup to close.
func RunPending(ctx context.Context, client *herdr.Client, entries []Entry, pending Pending) error {
	waitForExit(ctx, pending.PID, waitForPopup)

	for _, entry := range entries {
		if entry.ID != pending.EntryID {
			continue
		}
		err := entry.Run(ctx, Exec{Client: client, Ctx: pending.Context, Input: pending.Input})
		if err != nil {
			// The popup that would have shown this is gone, so the reason has
			// to reach the user some other way.
			report(ctx, client, entry.Name(), err)
		}
		return err
	}
	return fmt.Errorf("no command with id %q", pending.EntryID)
}

// waitForExit blocks until the process is gone or the wait runs out.
func waitForExit(ctx context.Context, pid int, limit time.Duration) {
	if pid <= 0 {
		return
	}
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if !processExists(pid) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(25 * time.Millisecond):
		}
	}
}

func report(ctx context.Context, client *herdr.Client, title string, failure error) {
	body := failure.Error()
	_, _ = client.NotificationShow(ctx, herdr.NotificationShowParams{
		Title: title + " failed",
		Body:  &body,
	})
}
