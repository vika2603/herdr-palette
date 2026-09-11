package palette

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/vika2603/herdr-client/herdr"
)

// Namespaces for the rows that go to something open instead of running a
// command. A pane running an agent shows as one: what it is doing is what the
// row is about.
const (
	TypeWorkspace = "Workspace"
	TypeTab       = "Tab"
	TypePane      = "Pane"
	TypeAgent     = "Agent"
)

// goTo is in front of every row that focuses something rather than running a
// command, so the query narrows the list to them. The spelling without the
// space finds them too: the matcher reads a query as a subsequence.
const goTo = "go to "

// OpenEntries is what the session holds right now, as rows that go there. The
// palette rebuilds them while it is open, so an agent's status is the one it
// has at that moment.
func OpenEntries(ctx context.Context, client *herdr.Client) ([]Entry, error) {
	snapshot, err := client.SessionSnapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("open panes unavailable: %w", err)
	}
	return sessionEntries(snapshot.Snapshot), nil
}

// sessionEntries turns what is open into rows that focus it. What is already
// focused is left out: the palette was opened from there.
//
// A plugin popup is not part of the session's panes, so the palette's own
// window never becomes a row of its list.
func sessionEntries(snapshot herdr.SessionSnapshot) []Entry {
	workspaces := make(map[string]string, len(snapshot.Workspaces))
	for _, workspace := range snapshot.Workspaces {
		workspaces[workspace.WorkspaceID] = workspace.Label
	}

	entries := make([]Entry, 0, len(snapshot.Workspaces)+len(snapshot.Tabs)+len(snapshot.Panes))
	for _, workspace := range snapshot.Workspaces {
		if workspace.Focused {
			continue
		}
		id := workspace.WorkspaceID
		entries = append(entries, Entry{
			ID:    "workspace:" + id,
			Title: goTo + label(workspace.Label, "workspace", workspace.Number),
			Type:  TypeWorkspace,
			Run: func(ctx context.Context, e Exec) error {
				_, err := e.Client.WorkspaceFocus(ctx, herdr.WorkspaceTarget{WorkspaceID: id})
				return err
			},
		})
	}

	for _, tab := range snapshot.Tabs {
		if tab.Focused {
			continue
		}
		id := tab.TabID
		entries = append(entries, Entry{
			ID:     "tab:" + id,
			Title:  goTo + label(tab.Label, "tab", tab.Number),
			Type:   TypeTab,
			Detail: workspaces[tab.WorkspaceID],
			Run: func(ctx context.Context, e Exec) error {
				_, err := e.Client.TabFocus(ctx, herdr.TabTarget{TabID: id})
				return err
			},
		})
	}

	for _, pane := range snapshot.Panes {
		if pane.Focused {
			continue
		}
		id := pane.PaneID
		entries = append(entries, Entry{
			ID:     "pane:" + id,
			Title:  goTo + paneLabel(pane),
			Type:   paneType(pane),
			Detail: paneDetail(pane, workspaces[pane.WorkspaceID]),
			Status: paneStatus(pane),
			// The row says where it goes, not where it is, so the directory is
			// searchable and shown when that is what the query matched.
			Search: herdr.Value(pane.Cwd),
			Run: func(ctx context.Context, e Exec) error {
				_, err := e.Client.PaneFocus(ctx, herdr.PaneTarget{PaneID: id})
				return err
			},
		})
	}
	return entries
}

// paneType separates the panes running an agent from the rest: their rows are
// about the agent, down to what it is doing.
func paneType(pane herdr.PaneInfo) string {
	if herdr.Value(pane.Agent) != "" {
		return TypeAgent
	}
	return TypePane
}

// paneDetail is what the row shows next to the title: for an agent the agent
// and its status, which the palette keeps current while it is open, and for
// any other pane the workspace it sits in.
func paneDetail(pane herdr.PaneInfo, workspace string) string {
	agent := herdr.Value(pane.Agent)
	if agent == "" {
		return workspace
	}
	if pane.AgentStatus == "" || pane.AgentStatus == herdr.AgentStatusUnknown {
		return agent
	}
	return agent + " · " + string(pane.AgentStatus)
}

// paneStatus is the state the row's detail describes, which only a pane
// running an agent has.
func paneStatus(pane herdr.PaneInfo) string {
	if herdr.Value(pane.Agent) == "" || pane.AgentStatus == herdr.AgentStatusUnknown {
		return ""
	}
	return string(pane.AgentStatus)
}

// paneLabel is the first thing a pane is known by: the name it was given, then
// what the program in it reports, then the agent running there, then the
// directory it is in.
func paneLabel(pane herdr.PaneInfo) string {
	for _, name := range []*string{pane.Label, pane.Title, pane.TerminalTitleStripped, pane.Agent} {
		if value := herdr.Value(name); value != "" {
			return value
		}
	}
	if cwd := herdr.Value(pane.Cwd); cwd != "" {
		return filepath.Base(cwd)
	}
	return pane.PaneID
}

func label(configured, kind string, number uint64) string {
	if configured != "" {
		return configured
	}
	return fmt.Sprintf("%s %d", kind, number)
}
