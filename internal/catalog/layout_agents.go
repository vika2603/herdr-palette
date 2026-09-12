package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/vika2603/herdr-client/herdr"
)

// runningAgents is what each pane of the exported tree has in it, in the order
// the panes are laid out. An exported tree carries a pane's directory but not
// what is running there, so this is read from the session and saved beside it.
// Nothing running anywhere is no list at all, which is what a layout of plain
// panes has.
func runningAgents(ctx context.Context, client *herdr.Client, root herdr.LayoutNode) ([]string, error) {
	snapshot, err := client.SessionSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	running := make(map[string]string, len(snapshot.Snapshot.Panes))
	for _, pane := range snapshot.Snapshot.Panes {
		if agent := herdr.Value(pane.Agent); agent != "" {
			running[pane.PaneID] = agent
		}
	}

	panes := herdr.LayoutPanes(root)
	agents := make([]string, len(panes))
	any := false
	for i, pane := range panes {
		if agents[i] = running[herdr.Value(pane.PaneID)]; agents[i] != "" {
			any = true
		}
	}
	if !any {
		return nil, nil
	}
	return agents, nil
}

// startAgents puts back what the layout was saved with, one agent per pane
// that had one. The arrangement is already open by this point, so an agent
// that will not start is reported without taking the tab down with it.
func startAgents(ctx context.Context, client *herdr.Client, root herdr.LayoutNode, agents []string) error {
	if len(agents) == 0 {
		return nil
	}

	var failures []error
	for i, pane := range herdr.LayoutPanes(root) {
		if i >= len(agents) || agents[i] == "" {
			continue
		}
		id := herdr.Value(pane.PaneID)
		if id == "" {
			continue
		}
		if _, err := client.AgentStart(ctx, herdr.AgentStartParams{
			Kind:   agents[i],
			Name:   agents[i],
			PaneID: id,
		}); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", agents[i], err))
		}
	}
	return errors.Join(failures...)
}
