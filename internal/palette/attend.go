package palette

import (
	"context"
	"fmt"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin"
)

// attendTitle heads what this action tells the user. It opens no popup, so a
// notification is the only way anything reaches them.
const attendTitle = "attend"

// attendFile keeps the pane the action last went to, which is how a press
// from there reads as going on to the next agent rather than as leaving work.
const attendFile = "attend.json"

type attendState struct {
	Went string `json:"went"`
}

// Attend goes to an agent that needs you without opening the popup: one that
// is blocked first, since it cannot go on until it is answered, then one that
// is done. Pressed again from one of them it goes to the next, so every agent
// waiting is one key away from the last.
//
// The pane it was pressed in is written down as the last place the palette
// went to, which is what back returns to: answering an agent and going back
// to the work it interrupted is two keys. A press from the pane it last went
// to is going on to the next agent, so the way back stays the work.
func Attend(ctx context.Context, client *herdr.Client, env *plugin.Env) error {
	// An entrypoint invoked outside a workspace has no context, which only
	// means there is no pane to come back to.
	invocation, _ := env.Context()
	here := ""
	if invocation != nil {
		here = herdr.Value(invocation.FocusedPaneID)
	}

	snapshot, err := client.SessionSnapshot(ctx)
	if err != nil {
		err = fmt.Errorf("open panes unavailable: %w", err)
		report(ctx, client, attendTitle, err)
		return err
	}

	target, ok := nextWaiting(snapshot.Snapshot.Panes, here)
	if !ok {
		body := "no agent is blocked or done"
		if here != "" && waiting(snapshot.Snapshot.Panes, here) {
			body = "no other agent is blocked or done"
		}
		_, _ = client.NotificationShow(ctx, herdr.NotificationShowParams{
			Title: "Nothing needs you",
			Body:  &body,
		})
		return nil
	}

	if _, err := client.PaneFocus(ctx, herdr.PaneTarget{PaneID: target}); err != nil {
		report(ctx, client, attendTitle, err)
		return err
	}
	// A failed write only costs the way back, so it does not turn a pane that
	// was reached into a failure.
	var last attendState
	_ = env.ReadStateJSON(attendFile, &last)
	if here != "" && here != last.Went {
		_ = WriteRecent(env, backPane+":"+here, ReadRecent(env))
	}
	_ = env.WriteStateJSON(attendFile, attendState{Went: target})
	return nil
}

// nextWaiting is the agent to go to from here: the blocked ones in the order
// the session lists them, then the done ones, starting after here when here
// is one of them. It reports false when nothing but here is waiting.
func nextWaiting(panes []herdr.PaneInfo, here string) (string, bool) {
	var queue []string
	for _, status := range []herdr.AgentStatus{herdr.AgentStatusBlocked, herdr.AgentStatusDone} {
		for _, pane := range panes {
			if herdr.Value(pane.Agent) != "" && pane.AgentStatus == status {
				queue = append(queue, pane.PaneID)
			}
		}
	}
	for i, id := range queue {
		if id == here {
			if len(queue) == 1 {
				return "", false
			}
			return queue[(i+1)%len(queue)], true
		}
	}
	if len(queue) == 0 {
		return "", false
	}
	return queue[0], true
}

func waiting(panes []herdr.PaneInfo, id string) bool {
	for _, pane := range panes {
		if pane.PaneID == id {
			return herdr.Value(pane.Agent) != "" &&
				(pane.AgentStatus == herdr.AgentStatusBlocked || pane.AgentStatus == herdr.AgentStatusDone)
		}
	}
	return false
}
