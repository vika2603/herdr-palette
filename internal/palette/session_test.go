package palette

import (
	"strings"
	"testing"

	"github.com/vika2603/herdr-client/herdr"
)

func agentSnapshot() herdr.SessionSnapshot {
	return herdr.SessionSnapshot{
		Workspaces: []herdr.WorkspaceInfo{{WorkspaceID: "w1", Label: "palette"}},
		Panes: []herdr.PaneInfo{
			{
				PaneID:                "w1:p1",
				WorkspaceID:           "w1",
				TabID:                 "w1:t1",
				Agent:                 new("claude"),
				AgentStatus:           herdr.AgentStatusWorking,
				TerminalTitleStripped: new("Herdr palette"),
			},
			{
				PaneID:                "w1:p2",
				WorkspaceID:           "w1",
				TabID:                 "w1:t1",
				TerminalTitleStripped: new("zsh"),
			},
		},
	}
}

// The name an agent was given is in the snapshot's agents, not on the pane,
// so a renamed agent is what the row has to show.
func TestARenamedAgentShowsTheNameItWasGiven(t *testing.T) {
	snapshot := agentSnapshot()
	snapshot.Agents = []herdr.AgentInfo{{PaneID: "w1:p1", Agent: new("claude"), Name: new("reviewer")}}

	agent, ok := find(sessionEntries(snapshot), "pane:w1:p1")
	if !ok {
		t.Fatal("the agent's pane is not in the list")
	}
	if agent.Detail != "working · reviewer" {
		t.Errorf("detail = %q, want the name the agent was given and its status", agent.Detail)
	}
}

func TestAPaneRunningAnAgentSaysWhatItIsDoing(t *testing.T) {
	entries := sessionEntries(agentSnapshot())

	agent, ok := find(entries, "pane:w1:p1")
	if !ok {
		t.Fatal("the agent's pane is not in the list")
	}
	if agent.Type != TypeAgent {
		t.Errorf("namespace = %q, want the pane running an agent to show as one", agent.Type)
	}
	if agent.Detail != "working · claude" {
		t.Errorf("detail = %q, want the agent and its status", agent.Detail)
	}

	plain, _ := find(entries, "pane:w1:p2")
	if plain.Type != TypePane || plain.Detail != "palette" {
		t.Errorf("entry = %+v, want a plain pane under the workspace it sits in", plain)
	}
}

// The query prefix that leaves the commands out ranks the rows that carry
// Goes, so a row the session put in the list without it cannot be reached
// that way.
func TestEveryRowOfTheSessionGoesSomewhere(t *testing.T) {
	for _, entry := range sessionEntries(agentSnapshot()) {
		if !entry.Goes {
			t.Errorf("%q does not say it goes somewhere", entry.Name())
		}
	}
}

// The status is drawn next to the row, so it is searched with it.
func TestAnAgentIsFoundByItsStatus(t *testing.T) {
	ranked := Rank(sessionEntries(agentSnapshot()), "working", nil)

	if len(ranked) != 1 {
		t.Fatalf("a query for the status matched %d rows, want the working agent", len(ranked))
	}
	if ranked[0].Entry.ID != "pane:w1:p1" {
		t.Errorf("matched %q, want the agent that is working", ranked[0].Entry.ID)
	}
	status := []rune(ranked[0].Detail)
	var matched string
	for _, at := range ranked[0].DetailMatched {
		matched += string(status[at])
	}
	if matched != "working" {
		t.Errorf("the detail highlights %q, want the status the query matched", matched)
	}
	if !strings.Contains(ranked[0].Entry.Name(), "go to") {
		t.Errorf("row = %q, want it to go to the agent", ranked[0].Entry.Name())
	}
}

// A tab or a workspace is previewed through a pane of it: the focused one when
// the tab holds it, since that is what the tab shows, and its first otherwise.
func TestATabIsPreviewedThroughOneOfItsPanes(t *testing.T) {
	snapshot := herdr.SessionSnapshot{
		Workspaces: []herdr.WorkspaceInfo{
			{WorkspaceID: "w1", Label: "here", Focused: true, ActiveTabID: "w1:t1"},
			{WorkspaceID: "w2", Label: "there", ActiveTabID: "w2:t2"},
		},
		Tabs: []herdr.TabInfo{
			{TabID: "w1:t1", WorkspaceID: "w1", Focused: true},
			{TabID: "w2:t1", WorkspaceID: "w2"},
			{TabID: "w2:t2", WorkspaceID: "w2"},
		},
		Panes: []herdr.PaneInfo{
			{PaneID: "w2:p1", TabID: "w2:t1", WorkspaceID: "w2"},
			{PaneID: "w2:p2", TabID: "w2:t2", WorkspaceID: "w2"},
			{PaneID: "w2:p3", TabID: "w2:t2", WorkspaceID: "w2"},
		},
	}
	snapshot.Panes[2].Focused = true

	previews := map[string]string{}
	for _, tab := range Tabs(snapshot) {
		previews["tab:"+tab.Value] = tab.Pane
	}
	for _, workspace := range Workspaces(snapshot) {
		previews["workspace:"+workspace.Value] = workspace.Pane
	}
	for id, want := range map[string]string{
		"tab:w2:t1":    "w2:p1",
		"tab:w2:t2":    "w2:p3",
		"workspace:w2": "w2:p3",
	} {
		if got, ok := previews[id]; !ok {
			t.Errorf("%s is not listed", id)
		} else if got != want {
			t.Errorf("%s is previewed through %q, want %q", id, got, want)
		}
	}
}
