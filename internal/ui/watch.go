package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin"
)

// watch reports that something the list shows has changed: an agent's status,
// the panes running them, or the workspaces and tabs they sit in. Each event
// sets the one signal the channel holds, so a burst costs a single rebuild,
// and the stream closes with the popup.
func watch(ctx context.Context, env *plugin.Env) <-chan struct{} {
	changes := make(chan struct{}, 1)
	go func() {
		defer close(changes)
		client := env.Client()
		// A subscription follows the agents of the panes that exist when it is
		// opened, so it is opened again whenever a pane comes or goes.
		for ctx.Err() == nil && follow(ctx, client, changes) {
		}
	}()
	return changes
}

// follow reads one subscription until it ends, reporting whether the panes it
// covers have changed, which is what a new subscription is needed for.
func follow(ctx context.Context, client *herdr.Client, changes chan<- struct{}) bool {
	stream, err := client.Subscribe(ctx, subscriptions(ctx, client)...)
	if err != nil {
		// The list is the one the popup opened with; it just stops following
		// the session.
		return false
	}
	defer func() { _ = stream.Close() }()

	for {
		event, err := stream.Next(ctx)
		if err != nil {
			return false
		}
		select {
		case changes <- struct{}{}:
		default:
		}
		if movesPanes(event) {
			return true
		}
	}
}

// subscriptions are the events that change what the palette lists. An agent's
// status is reported per pane, so every pane running one is named; the rest
// are session-wide. Pane output is not among them: it changes constantly and
// no row shows it.
func subscriptions(ctx context.Context, client *herdr.Client) []herdr.Subscription {
	subs := []herdr.Subscription{
		herdr.PaneAgentDetectedSubscription{},
		herdr.PaneCreatedSubscription{},
		herdr.PaneClosedSubscription{},
		herdr.PaneExitedSubscription{},
		herdr.PaneUpdatedSubscription{},
		herdr.TabCreatedSubscription{},
		herdr.TabClosedSubscription{},
		herdr.TabRenamedSubscription{},
		herdr.WorkspaceCreatedSubscription{},
		herdr.WorkspaceClosedSubscription{},
		herdr.WorkspaceRenamedSubscription{},
		herdr.WorkspaceUpdatedSubscription{},
	}

	snapshot, err := client.SessionSnapshot(ctx)
	if err != nil {
		return subs
	}
	for _, pane := range snapshot.Snapshot.Panes {
		if herdr.Value(pane.Agent) == "" {
			continue
		}
		subs = append(subs, herdr.PaneAgentStatusChangedSubscription{PaneID: pane.PaneID})
	}
	return subs
}

// movesPanes reports whether an event changes which panes there are to follow.
func movesPanes(event herdr.Event) bool {
	switch event.(type) {
	case *herdr.PaneCreatedEvent, *herdr.PaneClosedEvent, *herdr.PaneExitedEvent, *herdr.PaneAgentDetectedEvent:
		return true
	}
	return false
}

// listen waits for the next signal. A closed channel ends the loop by
// returning no message.
func listen(changes <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		if _, ok := <-changes; !ok {
			return nil
		}
		return changedMsg{}
	}
}
