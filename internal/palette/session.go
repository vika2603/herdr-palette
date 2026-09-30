package palette

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/vika2603/herdr-client/herdr"
)

// Namespaces for the rows that go to something open instead of running a
// command. A pane running an agent shows as one: what it is doing is what the
// row is about.
const (
	TypePane      = "Pane"
	TypeAgent     = "Agent"
	TypeTab       = "Tab"
	TypeWorkspace = "Workspace"
)

// GoTo is in front of every row that focuses something rather than running a
// command, so the query narrows the list to them. The spelling without the
// space finds them too: the matcher reads a query as a subsequence.
const GoTo = "Go To "

// Session is what is open right now: the rows that go there, and what every
// agent in the session is doing. Which scope lists a row is its kind's
// business.
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
	entries := paneEntries(snapshot.Snapshot)
	entries = append(entries, tabEntries(snapshot.Snapshot)...)
	entries = append(entries, workspaceEntries(snapshot.Snapshot)...)
	return Session{Entries: entries, Statuses: agentStatuses(snapshot.Snapshot)}, nil
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

// paneEntries turns the open panes into rows that focus them, the one the
// palette was opened from included, so it can be closed from the list. A pane
// is found by the names of the tab and workspace it sits in, which is how the
// command list reaches a place without rows for tabs and workspaces.
//
// A plugin popup is not part of the session's panes, so the palette's own
// window never becomes a row of its list.
func paneEntries(snapshot herdr.SessionSnapshot) []Entry {
	workspaces := make(map[string]string, len(snapshot.Workspaces))
	for _, workspace := range snapshot.Workspaces {
		workspaces[workspace.WorkspaceID] = workspace.Label
	}
	tabs := make(map[string]string, len(snapshot.Tabs))
	for _, tab := range snapshot.Tabs {
		tabs[tab.TabID] = tab.Label
	}

	// A pane carries the agent it runs, but the name the agent was given is
	// only in the snapshot's agents, so a renamed agent is looked up there.
	agents := make(map[string]string, len(snapshot.Agents))
	for _, agent := range snapshot.Agents {
		agents[agent.PaneID] = herdr.Value(agent.Name)
	}

	entries := make([]Entry, 0, len(snapshot.Panes))
	for _, pane := range snapshot.Panes {
		id, kind := pane.PaneID, paneKind(pane)
		entries = append(entries, Entry{
			ID:     "pane:" + id,
			Title:  GoTo + paneLabel(pane),
			Type:   paneType(kind),
			Kind:   kind,
			Here:   pane.Focused,
			Detail: joinNames(paneDetail(pane, agents[pane.PaneID], workspaces[pane.WorkspaceID]), here(pane.Focused)),
			Status: paneStatus(pane),
			Pane:   id,
			// The row says where it goes, not where it is, so where it is —
			// the workspace, the tab and the directory — is searchable, and
			// shown when that is what the query matched.
			Search: joinNames(workspaces[pane.WorkspaceID], tabs[pane.TabID], herdr.Value(pane.Cwd)),
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

// here is what the detail of the row for where the palette was opened from
// ends in. It comes last, so the status a narrow row keeps stays first, and
// it is searched with the row, so typing it finds that row.
func here(focused bool) string {
	if focused {
		return "here"
	}
	return ""
}

// joinNames joins what a pane sits in, leaving out what has no name.
func joinNames(parts ...string) string {
	named := parts[:0]
	for _, part := range parts {
		if part != "" {
			named = append(named, part)
		}
	}
	return strings.Join(named, " · ")
}

// workspaceEntries is every workspace, as rows that focus it. A workspace is
// previewed through the pane its active tab shows, and its detail is what its
// agents are doing and how many tabs it has.
func workspaceEntries(snapshot herdr.SessionSnapshot) []Entry {
	panes := tabPreviews(snapshot)
	var entries []Entry
	for _, workspace := range snapshot.Workspaces {
		id, status := workspace.WorkspaceID, knownStatus(workspace.AgentStatus)
		entries = append(entries, Entry{
			ID:     "workspace:" + id,
			Title:  GoTo + Label(workspace.Label, "workspace", workspace.Number),
			Type:   TypeWorkspace,
			Kind:   KindWorkspace,
			Here:   workspace.Focused,
			Detail: joinNames(status, count(workspace.TabCount, "tab"), here(workspace.Focused)),
			Status: status,
			Pane:   panes[workspace.ActiveTabID],
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
	return entries
}

// tabEntries is every tab, as rows that focus it, each with the workspace it
// sits in.
func tabEntries(snapshot herdr.SessionSnapshot) []Entry {
	workspaces := make(map[string]string, len(snapshot.Workspaces))
	for _, workspace := range snapshot.Workspaces {
		workspaces[workspace.WorkspaceID] = workspace.Label
	}
	panes := tabPreviews(snapshot)
	var entries []Entry
	for _, tab := range snapshot.Tabs {
		id, status := tab.TabID, knownStatus(tab.AgentStatus)
		entries = append(entries, Entry{
			ID:     "tab:" + id,
			Title:  GoTo + Label(tab.Label, "tab", tab.Number),
			Type:   TypeTab,
			Kind:   KindTab,
			Here:   tab.Focused,
			Detail: joinNames(status, workspaces[tab.WorkspaceID], here(tab.Focused)),
			Status: status,
			Pane:   panes[id],
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
	return entries
}

// tabPreviews is the pane each tab is previewed through. The snapshot marks
// only the pane focused in the whole session, not the one each tab would
// show, so it is the tab's first pane unless the focused one is in it.
func tabPreviews(snapshot herdr.SessionSnapshot) map[string]string {
	panes := make(map[string]string, len(snapshot.Tabs))
	for _, pane := range snapshot.Panes {
		if _, seen := panes[pane.TabID]; !seen || pane.Focused {
			panes[pane.TabID] = pane.PaneID
		}
	}
	return panes
}

// knownStatus is the agent status worth showing, which is none for a place
// with no agent or one herdr cannot classify.
func knownStatus(status herdr.AgentStatus) string {
	if status == "" || status == herdr.AgentStatusUnknown {
		return ""
	}
	return string(status)
}

func count(n uint64, what string) string {
	if n == 1 {
		return "1 " + what
	}
	return fmt.Sprintf("%d %ss", n, what)
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

// paneKind separates the panes running an agent from the rest: their rows are
// about the agent, down to what it is doing.
func paneKind(pane herdr.PaneInfo) Kind {
	if herdr.Value(pane.Agent) != "" {
		return KindAgent
	}
	return KindPane
}

func paneType(kind Kind) string {
	if kind == KindAgent {
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
