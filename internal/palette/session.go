package palette

import (
	"context"
	"fmt"

	"github.com/vika2603/herdr-client/herdr"
)

// Namespaces for the rows that go to something open instead of running a
// command.
const (
	TypeWorkspace = "Workspace"
	TypeTab       = "Tab"
	TypePane      = "Pane"
)

// goTo is in front of every row that focuses something rather than running a
// command, so the query narrows the list to them. gotoAlias makes the spelling
// without the space find them too.
const (
	goTo      = "go to "
	gotoAlias = "goto"
)

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
	tabs := make(map[string]string, len(snapshot.Tabs))
	for _, tab := range snapshot.Tabs {
		tabs[tab.TabID] = tab.Label
	}

	entries := make([]Entry, 0, len(snapshot.Workspaces)+len(snapshot.Tabs)+len(snapshot.Panes))
	for _, workspace := range snapshot.Workspaces {
		if workspace.Focused {
			continue
		}
		id := workspace.WorkspaceID
		entries = append(entries, Entry{
			ID:     "workspace:" + id,
			Title:  goTo + label(workspace.Label, "workspace", workspace.Number),
			Search: gotoAlias,
			Type:   TypeWorkspace,
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
			Search: gotoAlias + " " + workspaces[tab.WorkspaceID],
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
			ID:    "pane:" + id,
			Title: goTo + paneLabel(pane),
			Type:  TypePane,
			// The pane's own title says little about where it is, so the
			// workspace and tab it sits in are searchable as well.
			Search: gotoAlias + " " + workspaces[pane.WorkspaceID] + " " + tabs[pane.TabID] + " " + herdr.Value(pane.Cwd),
			Run: func(ctx context.Context, e Exec) error {
				_, err := e.Client.PaneFocus(ctx, herdr.PaneTarget{PaneID: id})
				return err
			},
		})
	}
	return entries
}

// paneLabel is the first thing a pane is known by: the name it was given, then
// what the program in it reports, then the agent running there.
func paneLabel(pane herdr.PaneInfo) string {
	for _, name := range []*string{pane.Label, pane.Title, pane.TerminalTitleStripped, pane.Agent} {
		if value := herdr.Value(name); value != "" {
			return value
		}
	}
	return pane.PaneID
}

func label(configured, kind string, number uint64) string {
	if configured != "" {
		return configured
	}
	return fmt.Sprintf("%s %d", kind, number)
}
