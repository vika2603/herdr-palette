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

// GoTo is in front of every row that focuses something rather than running a
// command, so the query narrows the list to them. The spelling without the
// space finds them too: the matcher reads a query as a subsequence.
const GoTo = "go to "

// Session is what is open right now: the rows that go there, and what every
// agent in the session is doing — the one in the focused pane too, which no
// row stands for.
type Session struct {
	Entries  []Entry
	Statuses []string
}

// OpenSession reads what the session holds. The palette reads it again while
// it is open, so an agent's status is the one it has at that moment.
func OpenSession(ctx context.Context, client *herdr.Client) (Session, error) {
	snapshot, err := client.SessionSnapshot(ctx)
	if err != nil {
		return Session{}, fmt.Errorf("open panes unavailable: %w", err)
	}
	return Session{
		Entries:  sessionEntries(snapshot.Snapshot),
		Statuses: agentStatuses(snapshot.Snapshot),
	}, nil
}

// agentStatuses is the status of every pane running an agent whose status
// herdr knows.
func agentStatuses(snapshot herdr.SessionSnapshot) []string {
	var statuses []string
	for _, pane := range snapshot.Panes {
		if status := paneStatus(pane); status != "" {
			statuses = append(statuses, status)
		}
	}
	return statuses
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

	// A pane carries the agent it runs, but the name the agent was given is
	// only in the snapshot's agents, so a renamed agent is looked up there.
	agents := make(map[string]string, len(snapshot.Agents))
	for _, agent := range snapshot.Agents {
		agents[agent.PaneID] = herdr.Value(agent.Name)
	}

	// A tab is previewed through one of its panes. The snapshot marks only the
	// pane focused in the whole session, not the one each tab would show, so
	// it is the tab's first pane unless the focused one is in it.
	tabPanes := make(map[string]string, len(snapshot.Tabs))
	for _, pane := range snapshot.Panes {
		if _, seen := tabPanes[pane.TabID]; !seen || pane.Focused {
			tabPanes[pane.TabID] = pane.PaneID
		}
	}

	entries := make([]Entry, 0, len(snapshot.Workspaces)+len(snapshot.Tabs)+len(snapshot.Panes))
	for _, workspace := range snapshot.Workspaces {
		if workspace.Focused {
			continue
		}
		id := workspace.WorkspaceID
		entries = append(entries, Entry{
			ID:    "workspace:" + id,
			Title: GoTo + Label(workspace.Label, "workspace", workspace.Number),
			Type:  TypeWorkspace,
			Goes:  true,
			Pane:  tabPanes[workspace.ActiveTabID],
			Run: func(ctx context.Context, e Exec) error {
				_, err := e.Client.WorkspaceFocus(ctx, herdr.WorkspaceTarget{WorkspaceID: id})
				return err
			},
			Close: func(ctx context.Context, e Exec) error {
				_, err := e.Client.WorkspaceClose(ctx, herdr.WorkspaceCloseParams{WorkspaceID: id})
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
			Title:  GoTo + Label(tab.Label, "tab", tab.Number),
			Type:   TypeTab,
			Goes:   true,
			Detail: workspaces[tab.WorkspaceID],
			Pane:   tabPanes[tab.TabID],
			Run: func(ctx context.Context, e Exec) error {
				_, err := e.Client.TabFocus(ctx, herdr.TabTarget{TabID: id})
				return err
			},
			Close: func(ctx context.Context, e Exec) error {
				_, err := e.Client.TabClose(ctx, herdr.TabTarget{TabID: id})
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
			Title:  GoTo + paneLabel(pane),
			Type:   paneType(pane),
			Goes:   true,
			Detail: paneDetail(pane, agents[pane.PaneID], workspaces[pane.WorkspaceID]),
			Status: paneStatus(pane),
			Pane:   id,
			// The row says where it goes, not where it is, so the directory is
			// searchable and shown when that is what the query matched.
			Search: herdr.Value(pane.Cwd),
			Run: func(ctx context.Context, e Exec) error {
				_, err := e.Client.PaneFocus(ctx, herdr.PaneTarget{PaneID: id})
				return err
			},
			Close: func(ctx context.Context, e Exec) error {
				_, err := e.Client.PaneClose(ctx, herdr.PaneTarget{PaneID: id})
				return err
			},
		})
	}
	return entries
}

// TabPanes is the other panes of the tab the pane is in, as targets of a
// command that acts on one of them. Each reads the way its row in the list
// does, with its directory in place of the workspace every one of them shares.
func TabPanes(snapshot herdr.SessionSnapshot, paneID string) []Choice {
	tab := ""
	for _, pane := range snapshot.Panes {
		if pane.PaneID == paneID {
			tab = pane.TabID
		}
	}
	agents := make(map[string]string, len(snapshot.Agents))
	for _, agent := range snapshot.Agents {
		agents[agent.PaneID] = herdr.Value(agent.Name)
	}

	var choices []Choice
	for _, pane := range snapshot.Panes {
		if pane.TabID != tab || pane.PaneID == paneID {
			continue
		}
		choices = append(choices, Choice{
			Value:  pane.PaneID,
			Title:  paneLabel(pane),
			Detail: paneDetail(pane, agents[pane.PaneID], herdr.Value(pane.Cwd)),
			Status: paneStatus(pane),
			Search: herdr.Value(pane.Cwd),
			Pane:   pane.PaneID,
		})
	}
	return choices
}

// paneType separates the panes running an agent from the rest: their rows are
// about the agent, down to what it is doing.
func paneType(pane herdr.PaneInfo) string {
	if herdr.Value(pane.Agent) != "" {
		return TypeAgent
	}
	return TypePane
}

// paneDetail is what the row shows next to the title: for an agent its status
// and the name it goes by, and for any other pane the workspace it sits in.
// The status comes first because it is what a narrow row keeps when the
// detail has to be cut, and the palette keeps it current while it is open.
// The name is the agent's own unless it was renamed, so an unnamed agent reads
// as what it is.
func paneDetail(pane herdr.PaneInfo, name, workspace string) string {
	agent := herdr.Value(pane.Agent)
	if agent == "" {
		return workspace
	}
	if name != "" {
		agent = name
	}
	if pane.AgentStatus == "" || pane.AgentStatus == herdr.AgentStatusUnknown {
		return agent
	}
	return string(pane.AgentStatus) + " · " + agent
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

// Label is the name a workspace, tab or pane goes by: the one it was given,
// or what it is and its number when it was given none.
func Label(configured, kind string, number uint64) string {
	if configured != "" {
		return configured
	}
	return fmt.Sprintf("%s %d", kind, number)
}
