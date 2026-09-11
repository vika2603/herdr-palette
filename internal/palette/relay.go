package palette

import (
	"context"
	"errors"
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
	// PID is the popup process. herdr closes the popup pane when it exits.
	PID int `json:"pid"`
}

// Relay hands an entry to the exec entrypoint and lets the popup close.
//
// herdr allows one popup at a time, so an entry that opens one — a configured
// popup command, or a plugin action that opens its own pane — is refused with
// ui_busy while the palette is up. herdr runs an action entrypoint outside the
// popup, so the palette writes down what to run, asks for that entrypoint, and
// quits.
func Relay(ctx context.Context, client *herdr.Client, env *plugin.Env, entry Entry, input string, invocation *herdr.PluginInvocationContext) error {
	pending := Pending{
		EntryID: entry.ID,
		Input:   input,
		Context: invocation,
		PID:     os.Getpid(),
	}
	if err := env.WriteStateJSON(PendingFile, pending); err != nil {
		return err
	}

	pluginID := env.PluginID
	_, err := client.PluginActionInvoke(ctx, herdr.PluginActionInvokeParams{
		PluginID: &pluginID,
		ActionID: ExecAction,
		Context:  invocation,
	})
	return err
}

// RunPending runs what Relay wrote down, after waiting for the popup to close.
// Nothing pending is the normal outcome of an action invoked by hand.
func RunPending(ctx context.Context, client *herdr.Client, env *plugin.Env, entries []Entry) error {
	var pending Pending
	// A missing file reads as an empty entry: the action was invoked by hand
	// rather than handed over.
	if err := env.ReadStateJSON(PendingFile, &pending); err != nil || pending.EntryID == "" {
		return nil
	}
	_ = os.Remove(env.StatePath(PendingFile))

	waitForExit(ctx, pending.PID, waitForPopup)

	for _, entry := range entries {
		if entry.ID != pending.EntryID {
			continue
		}
		err := entry.Run(ctx, Exec{Client: client, Ctx: pending.Context, Input: pending.Input})
		if err != nil {
			// The popup that would have shown this is gone, so the reason has
			// to reach the user some other way.
			report(ctx, client, entry.Title, err)
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
	_ = errors.Unwrap(failure)
}
