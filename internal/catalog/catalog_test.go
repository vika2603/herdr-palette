package catalog

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin"
	"github.com/vika2603/herdr-client/plugin/plugintest"

	"github.com/vika2603/herdr-palette/internal/layout"
	"github.com/vika2603/herdr-palette/internal/palette"
)

// fullContext is what herdr passes when the palette opens over a focused pane
// running an agent, inside a worktree workspace.
func fullContext() *herdr.PluginInvocationContext {
	return &herdr.PluginInvocationContext{
		WorkspaceID:      new("w1"),
		WorkspaceLabel:   new("workspace one"),
		WorkspaceCwd:     new("/repo"),
		TabID:            new("t1"),
		TabLabel:         new("tab one"),
		FocusedPaneID:    new("p1"),
		FocusedPaneCwd:   new("/repo/sub"),
		FocusedPaneAgent: new("claude"),
	}
}

// snapshot is a session of two workspaces: the one the palette was opened in,
// whose focused tab is the second of three and holds three panes, and another
// workspace to move a pane to.
func snapshot() herdr.SessionSnapshot {
	return herdr.SessionSnapshot{
		Workspaces: []herdr.WorkspaceInfo{
			{WorkspaceID: "w1", Label: "repo"},
			{WorkspaceID: "w2", Label: "notes"},
		},
		Tabs: []herdr.TabInfo{
			{TabID: "t0", WorkspaceID: "w1", Label: "first", Number: 7},
			{TabID: "t1", WorkspaceID: "w1", Label: "tab one", Number: 3},
			{TabID: "t2", WorkspaceID: "w1", Label: "last", Number: 9},
			{TabID: "t9", WorkspaceID: "w2", Label: "elsewhere", Number: 1},
		},
		Agents: []herdr.AgentInfo{
			{PaneID: "p9", TabID: "t9", WorkspaceID: "w2", Agent: new("codex"), Name: new("reviewer"), AgentStatus: herdr.AgentStatusWorking, Cwd: new("/repo")},
		},
		Panes: []herdr.PaneInfo{
			{PaneID: "p1", TabID: "t1", WorkspaceID: "w1"},
			{PaneID: "p2", TabID: "t1", WorkspaceID: "w1"},
			{PaneID: "p3", TabID: "t1", WorkspaceID: "w1"},
			{PaneID: "p9", TabID: "t9", WorkspaceID: "w2"},
		},
	}
}

// worktrees is what herdr answers worktree.list with: the repository's own
// checkout, a worktree a workspace is open on, and one that is not open.
func worktreeList() herdr.WorktreeListResponse {
	return herdr.WorktreeListResponse{Worktrees: []herdr.WorktreeInfo{
		{Path: "/repo", Branch: new("main"), Label: "repo", OpenWorkspaceID: new("w1")},
		{Path: "/trees/fix", Branch: new("fix"), Label: "fix", IsLinkedWorktree: true, OpenWorkspaceID: new("w2")},
		{Path: "/trees/spike", Branch: new("spike"), Label: "spike", IsLinkedWorktree: true},
	}}
}

// exportedLayout is what herdr answers layout.export with: the arrangement of
// the tab, whose leaves name the panes it holds.
func exportedLayout() herdr.LayoutNode {
	return herdr.LayoutNodeSplit{
		Direction: herdr.SplitDirectionRight,
		Ratio:     0.5,
		First:     herdr.LayoutNodePane{PaneID: new("p1"), Cwd: new("/repo")},
		Second:    herdr.LayoutNodePane{PaneID: new("p2"), Cwd: new("/repo")},
	}
}

// server answers every method the catalog can call, so one script serves the
// whole table.
func server(t *testing.T) *plugintest.Server {
	t.Helper()
	return plugintest.NewServer(t).
		Reply(herdr.MethodSessionSnapshot, herdr.SessionSnapshotResponse{Snapshot: snapshot()}).
		Reply(herdr.MethodWorktreeList, worktreeList()).
		Reply(herdr.MethodWorktreeRemove, herdr.WorktreeRemovedResponse{}).
		Reply(herdr.MethodTabMove, herdr.TabListResponse{}).
		Reply(herdr.MethodPaneMove, herdr.PaneMoveResponse{}).
		Reply(herdr.MethodPaneFocus, herdr.PaneInfoResponse{}).
		Reply(herdr.MethodWorkspaceCreate, herdr.WorkspaceCreatedResponse{}).
		Reply(herdr.MethodWorkspaceRename, herdr.WorkspaceInfoResponse{}).
		Reply(herdr.MethodWorkspaceClose, herdr.OKResponse{}).
		Reply(herdr.MethodWorktreeCreate, herdr.WorktreeCreatedResponse{}).
		Reply(herdr.MethodWorktreeOpen, herdr.WorktreeOpenedResponse{}).
		Reply(herdr.MethodTabCreate, herdr.TabCreatedResponse{}).
		Reply(herdr.MethodTabRename, herdr.TabInfoResponse{}).
		Reply(herdr.MethodTabClose, herdr.OKResponse{}).
		Reply(herdr.MethodPaneSplit, herdr.PaneInfoResponse{Pane: herdr.PaneInfo{PaneID: "w1:p2"}}).
		Reply(herdr.MethodPaneSwap, herdr.PaneSwapResponse{}).
		Reply(herdr.MethodPaneZoom, herdr.PaneZoomResponse{}).
		Reply(herdr.MethodPaneRename, herdr.PaneInfoResponse{}).
		Reply(herdr.MethodPaneClose, herdr.OKResponse{}).
		Reply(herdr.MethodPaneEditScrollback, herdr.OKResponse{}).
		Reply(herdr.MethodPaneFocusDirection, herdr.PaneFocusDirectionResponse{}).
		Reply(herdr.MethodAgentPrompt, herdr.AgentPromptedResponse{}).
		Reply(herdr.MethodAgentStart, herdr.AgentStartedResponse{}).
		Reply(herdr.MethodAgentRename, herdr.AgentInfoResponse{}).
		Reply(herdr.MethodServerAgentManifests, herdr.AgentManifestStatusResponse{
			Manifests: []herdr.AgentManifestInfo{{Agent: "claude"}, {Agent: "codex"}},
		}).
		Reply(herdr.MethodLayoutExport, herdr.LayoutExportResponse{Layout: herdr.LayoutDescription{Root: exportedLayout()}}).
		Reply(herdr.MethodLayoutApply, herdr.LayoutApplyResponse{}).
		Reply(herdr.MethodPluginList, herdr.PluginListResponse{Plugins: []herdr.InstalledPluginInfo{
			{PluginID: "herdr.palette", Name: "Command Palette", Enabled: true},
			{PluginID: "herdr.machine-manager", Name: "Machine Manager", Enabled: true},
			{PluginID: "herdr.auto-title", Name: "Auto Title"},
		}}).
		Reply(herdr.MethodPluginEnable, herdr.PluginEnabledResponse{}).
		Reply(herdr.MethodPluginDisable, herdr.PluginDisabledResponse{}).
		Reply(herdr.MethodPluginPaneOpen, herdr.PluginPaneOpenedResponse{}).
		Reply(herdr.MethodServerReloadConfig, herdr.ConfigReloadResponse{}).
		Reply(herdr.MethodPaneRead, herdr.PaneReadResponse{Read: herdr.PaneReadResult{
			Text: "make: *** [test] Error 1\n\n  panic: nil map\n",
		}}).
		Reply(herdr.MethodAgentGet, herdr.AgentInfoResponse{Agent: herdr.AgentInfo{
			PaneID: "w1:p3", Agent: new("claude"), Name: new("reviewer"),
			AgentStatus: herdr.AgentStatusWorking,
		}}).
		Reply(herdr.MethodAgentWait, herdr.AgentInfoResponse{Agent: herdr.AgentInfo{
			PaneID: "w1:p3", Agent: new("claude"), Name: new("reviewer"),
			AgentStatus: herdr.AgentStatusBlocked, Cwd: new("/repo"),
		}}).
		Reply(herdr.MethodNotificationShow, herdr.NotificationShowResponse{})
}

func entry(t *testing.T, id string) palette.Entry {
	t.Helper()
	for _, e := range Entries() {
		if e.ID == id {
			return e
		}
	}
	t.Fatalf("no catalog entry with id %q", id)
	return palette.Entry{}
}

// paramsOf is what the last call to a method was made with. The calls are
// looked up by name rather than by position: a command makes more than one,
// and which one it makes first is not what a test is about.
func paramsOf(t *testing.T, s *plugintest.Server, method string) json.RawMessage {
	t.Helper()
	for i := len(s.Calls()) - 1; i >= 0; i-- {
		if call := s.Calls()[i]; call.Method == method {
			return call.Params
		}
	}
	t.Fatalf("%s was never called", method)
	return nil
}

func decode(t *testing.T, raw json.RawMessage, into any) {
	t.Helper()
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatalf("decoding %s: %v", raw, err)
	}
}

// step is what the palette collected for an entry before running it: the
// target picked from its list, the value typed into its field, or neither.
type step struct {
	chosen string
	input  string
}

// run executes one entry against the scripted server and returns the calls it
// made.
func run(t *testing.T, id string, collected step) []plugintest.Call {
	t.Helper()
	calls := runIn(t, id, collected, fullContext())
	if len(calls) == 0 {
		t.Fatalf("%s made no calls", id)
	}
	return calls
}

// runIn executes one entry against a context of its own, for the entries whose
// outcome depends on where the palette was opened.
func runIn(t *testing.T, id string, collected step, ctx *herdr.PluginInvocationContext) []plugintest.Call {
	t.Helper()
	s := server(t)
	if err := execute(t, s.Env(plugintest.StateDir(t.TempDir())), id, collected, ctx); err != nil {
		t.Fatalf("%s: Run() = %v", id, err)
	}
	return s.Calls()
}

// execute runs one entry against an environment the caller keeps, which is
// what the entries that save something of their own need.
func execute(t *testing.T, env *plugin.Env, id string, collected step, ctx *herdr.PluginInvocationContext) error {
	t.Helper()
	return entry(t, id).Run(context.Background(), palette.Exec{
		Client: env.Client(),
		Ctx:    ctx,
		Input:  collected.input,
		Chosen: collected.chosen,
		Env:    env,
	})
}

// choices is the list an entry offers against the scripted server.
func choices(t *testing.T, id string) []palette.Choice {
	t.Helper()
	e := entry(t, id)
	if e.Choices == nil {
		t.Fatalf("%s offers no list to pick from", id)
	}
	s := server(t)
	env := s.Env(plugintest.StateDir(t.TempDir()))
	// The palette leaves its own plugin out of what can be disabled, which it
	// knows by the id the entrypoint environment carries.
	env.PluginID = "herdr.palette"
	list, err := e.Choices.List(context.Background(), palette.Exec{
		Client: env.Client(),
		Ctx:    fullContext(),
		Env:    env,
	})
	if err != nil {
		t.Fatalf("%s: List() = %v", id, err)
	}
	return list
}

// values is what picking each row hands the entry.
func values(list []palette.Choice) []string {
	out := make([]string, 0, len(list))
	for _, choice := range list {
		out = append(out, choice.Value)
	}
	return out
}

func TestEachEntryCallsItsMethod(t *testing.T) {
	cases := []struct {
		id        string
		collected step
		method    string
	}{
		{id: "herdr:workspace.new", method: herdr.MethodWorkspaceCreate},
		{id: "herdr:workspace.rename", collected: step{input: "renamed"}, method: herdr.MethodWorkspaceRename},
		{id: "herdr:workspace.close", method: herdr.MethodWorkspaceClose},
		{id: "herdr:worktree.new", collected: step{input: "feature"}, method: herdr.MethodWorktreeCreate},
		{id: "herdr:worktree.open", collected: step{chosen: "/trees/spike"}, method: herdr.MethodWorktreeOpen},
		{id: "herdr:worktree.remove", collected: step{chosen: "w2"}, method: herdr.MethodWorktreeRemove},
		{id: "herdr:tab.new", method: herdr.MethodTabCreate},
		{id: "herdr:tab.rename", collected: step{input: "renamed"}, method: herdr.MethodTabRename},
		{id: "herdr:tab.close", method: herdr.MethodTabClose},
		{id: "herdr:pane.split.right", method: herdr.MethodPaneSplit},
		{id: "herdr:pane.split.down", method: herdr.MethodPaneSplit},
		{id: "herdr:pane.split.left", method: herdr.MethodPaneSplit},
		{id: "herdr:pane.split.up", method: herdr.MethodPaneSplit},
		{id: "herdr:pane.zoom", method: herdr.MethodPaneZoom},
		{id: "herdr:pane.rename", collected: step{input: "renamed"}, method: herdr.MethodPaneRename},
		{id: "herdr:pane.close", method: herdr.MethodPaneClose},
		{id: "herdr:pane.edit_scrollback", method: herdr.MethodPaneEditScrollback},
		{id: "herdr:pane.focus.left", method: herdr.MethodPaneFocusDirection},
		{id: "herdr:pane.focus.down", method: herdr.MethodPaneFocusDirection},
		{id: "herdr:pane.focus.up", method: herdr.MethodPaneFocusDirection},
		{id: "herdr:pane.focus.right", method: herdr.MethodPaneFocusDirection},
		{id: "herdr:pane.move", collected: step{chosen: "t9"}, method: herdr.MethodPaneMove},
		{id: "herdr:agent.start", collected: step{chosen: "codex"}, method: herdr.MethodAgentStart},
		{id: "herdr:agent.rename", collected: step{input: "reviewer"}, method: herdr.MethodAgentRename},
		{id: "herdr:agent.prompt", collected: step{input: "go on"}, method: herdr.MethodAgentPrompt},
		{id: "herdr:agent.prompt.any", collected: step{chosen: "p9", input: "go on"}, method: herdr.MethodAgentPrompt},
		{id: "herdr:config.edit", method: herdr.MethodPluginPaneOpen},
		{id: "herdr:server.reload_config", method: herdr.MethodServerReloadConfig},
	}

	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			if got := run(t, c.id, c.collected)[0].Method; got != c.method {
				t.Errorf("called %q, want %q", got, c.method)
			}
		})
	}
}

func TestNewWorkspaceFollowsTheFocusedWorkspace(t *testing.T) {
	var params herdr.WorkspaceCreateParams
	decode(t, run(t, "herdr:workspace.new", step{})[0].Params, &params)

	if params.SourceWorkspaceID == nil || *params.SourceWorkspaceID != "w1" {
		t.Error("source_workspace_id was not sent, so the new workspace loses the cwd policy")
	}
	if params.Focus == nil || !*params.Focus {
		t.Error("the new workspace is not focused")
	}
}

func TestRenameSendsTheTypedValue(t *testing.T) {
	var params herdr.WorkspaceRenameParams
	decode(t, run(t, "herdr:workspace.rename", step{input: "renamed"})[0].Params, &params)

	if params.WorkspaceID != "w1" || params.Label != "renamed" {
		t.Errorf("renamed %+v, want w1 to become \"renamed\"", params)
	}
}

func TestRenameStartsFromTheCurrentLabel(t *testing.T) {
	if got := entry(t, "herdr:workspace.rename").Initial(fullContext()); got != "workspace one" {
		t.Errorf("initial value = %q, want the current workspace label", got)
	}
	if got := entry(t, "herdr:tab.rename").Initial(fullContext()); got != "tab one" {
		t.Errorf("initial value = %q, want the current tab label", got)
	}
}

func TestNewTabOpensInTheFocusedPaneCwd(t *testing.T) {
	var params herdr.TabCreateParams
	decode(t, run(t, "herdr:tab.new", step{})[0].Params, &params)

	if params.WorkspaceID == nil || *params.WorkspaceID != "w1" {
		t.Error("the tab was not created in the focused workspace")
	}
	if params.Cwd == nil || *params.Cwd != "/repo/sub" {
		t.Error("the tab does not start in the focused pane's directory")
	}
}

func TestSplitTargetsTheFocusedPane(t *testing.T) {
	var params herdr.PaneSplitParams
	decode(t, run(t, "herdr:pane.split.down", step{})[0].Params, &params)

	if params.Direction != herdr.SplitDirectionDown {
		t.Errorf("direction = %q, want down", params.Direction)
	}
	if params.TargetPaneID == nil || *params.TargetPaneID != "p1" {
		t.Error("the split does not target the pane that was focused")
	}
}

func TestPromptTargetsTheFocusedPane(t *testing.T) {
	var params herdr.AgentPromptParams
	decode(t, run(t, "herdr:agent.prompt", step{input: "go on"})[0].Params, &params)

	if params.Target != "p1" || params.Text != "go on" {
		t.Errorf("prompted %+v, want the focused pane to receive the text", params)
	}
}

func TestWorktreeTakesTheBranchAndTheWorkspaceCwd(t *testing.T) {
	var params herdr.WorktreeCreateParams
	decode(t, run(t, "herdr:worktree.new", step{input: "feature"})[0].Params, &params)

	if params.Branch == nil || *params.Branch != "feature" {
		t.Error("the branch name was not sent")
	}
	if params.Cwd == nil || *params.Cwd != "/repo" {
		t.Error("the worktree is not created from the workspace's repository")
	}
}

// An empty context is what an action invoked with nothing focused receives.
// Those entries must fail before the call, so the popup reports why instead
// of herdr rejecting an empty id.
func TestEntriesThatNeedContextFailWithoutIt(t *testing.T) {
	for _, id := range []string{
		"herdr:workspace.rename",
		"herdr:workspace.close",
		"herdr:tab.rename",
		"herdr:tab.close",
		"herdr:pane.rename",
		"herdr:pane.close",
		"herdr:pane.edit_scrollback",
		"herdr:agent.start",
		"herdr:agent.rename",
		"herdr:agent.prompt",
	} {
		t.Run(id, func(t *testing.T) {
			s := server(t)
			err := entry(t, id).Run(context.Background(), palette.Exec{
				Client: s.Env().Client(),
				Ctx:    &herdr.PluginInvocationContext{},
				Input:  "value",
			})
			if err == nil {
				t.Fatal("Run() reported no error although nothing was focused")
			}
			if calls := s.Calls(); len(calls) != 0 {
				t.Errorf("made %d calls, want none before the context check", len(calls))
			}
		})
	}
}

func TestPromptRefusesAPaneWithoutAnAgent(t *testing.T) {
	s := server(t)
	ctx := fullContext()
	ctx.FocusedPaneAgent = nil

	err := entry(t, "herdr:agent.prompt").Run(context.Background(), palette.Exec{
		Client: s.Env().Client(),
		Ctx:    ctx,
		Input:  "go on",
	})
	if err == nil {
		t.Fatal("Run() prompted a pane that runs no agent")
	}
}

func TestEntriesAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range Entries() {
		switch {
		case e.ID == "":
			t.Errorf("%q has no id", e.Title)
		case e.Title == "":
			t.Errorf("%s has no title", e.ID)
		case e.Type == "":
			t.Errorf("%s has no group", e.ID)
		case e.Run == nil:
			t.Errorf("%s has no command", e.ID)
		case seen[e.ID]:
			t.Errorf("%s is listed twice, so the recent order would key both", e.ID)
		case e.Choices != nil && (e.Choices.Label == "" || e.Choices.Empty == "" || e.Choices.List == nil):
			t.Errorf("%s picks from a list that is missing its label, its empty line or its source", e.ID)
		}
		seen[e.ID] = true
	}
}

// The key column is only right if the binding names are the ones herdr uses.
// They come from the default configuration the installed herdr prints.
func TestBindingsNameRealHerdrActions(t *testing.T) {
	out, err := exec.Command("herdr", "--default-config").Output()
	if err != nil {
		t.Skip("herdr is not on PATH")
	}

	config := string(out)
	for _, e := range Entries() {
		if e.Binding == "" {
			continue
		}
		if !strings.Contains(config, e.Binding+" = ") {
			t.Errorf("%s names the action %q, which herdr's configuration does not define", e.ID, e.Binding)
		}
	}
}

// herdr splits right and down only, so the palette's left and up entries split
// and then swap the new pane into place.
func TestSplittingLeftSwapsTheNewPaneIntoPlace(t *testing.T) {
	calls := run(t, "herdr:pane.split.left", step{})
	if len(calls) != 2 {
		t.Fatalf("made %d calls, want a split and a swap", len(calls))
	}

	var split herdr.PaneSplitParams
	decode(t, calls[0].Params, &split)
	if split.Direction != herdr.SplitDirectionRight {
		t.Errorf("split %q, want herdr's right split", split.Direction)
	}

	if calls[1].Method != herdr.MethodPaneSwap {
		t.Fatalf("second call is %q, want a swap", calls[1].Method)
	}
	var swap herdr.PaneSwapParams
	decode(t, calls[1].Params, &swap)
	if swap.PaneID == nil || *swap.PaneID != "w1:p2" {
		t.Error("the swap does not name the pane the split just created")
	}
	if swap.Direction == nil || *swap.Direction != herdr.PaneDirectionLeft {
		t.Error("the new pane was not swapped to the left")
	}
}

func TestSplittingRightDoesNotSwap(t *testing.T) {
	if calls := run(t, "herdr:pane.split.right", step{}); len(calls) != 1 {
		t.Errorf("made %d calls, want the split alone", len(calls))
	}
}

func TestOpeningAWorktreeOffersTheOnesNoWorkspaceIsOn(t *testing.T) {
	list := choices(t, "herdr:worktree.open")

	if got := values(list); len(got) != 1 || got[0] != "/trees/spike" {
		t.Fatalf("offered %v, want the worktree no workspace is open on", got)
	}
	if list[0].Title != "spike" {
		t.Errorf("title = %q, want the branch the worktree is on", list[0].Title)
	}
	if list[0].Search != "/trees/spike" {
		t.Errorf("search text = %q, want the checkout to be searchable", list[0].Search)
	}
}

// The path identifies a worktree whether or not it is on a branch.
func TestOpeningAWorktreeSendsThePath(t *testing.T) {
	var params herdr.WorktreeOpenParams
	decode(t, run(t, "herdr:worktree.open", step{chosen: "/trees/spike"})[0].Params, &params)

	if params.Path == nil || *params.Path != "/trees/spike" {
		t.Errorf("opened %+v, want the chosen checkout", params)
	}
	if params.Cwd == nil || *params.Cwd != "/repo" {
		t.Error("the worktree is not opened from the workspace's repository")
	}
}

func TestRemovingAWorktreeOffersTheWorkspacesOnOne(t *testing.T) {
	list := choices(t, "herdr:worktree.remove")

	if got := values(list); len(got) != 1 || got[0] != "w2" {
		t.Fatalf("offered %v, want the workspace the linked worktree is open in", got)
	}
	if list[0].Title != "fix" {
		t.Errorf("title = %q, want the branch the worktree is on", list[0].Title)
	}
}

// herdr addresses a removal by the workspace, and leaving force unset is what
// keeps a checkout with work in it.
func TestRemovingAWorktreeNamesTheWorkspaceAndDoesNotForce(t *testing.T) {
	var params herdr.WorktreeRemoveParams
	decode(t, run(t, "herdr:worktree.remove", step{chosen: "w2"})[0].Params, &params)

	if params.WorkspaceID != "w2" {
		t.Errorf("removed %q, want the chosen workspace", params.WorkspaceID)
	}
	if params.Force != nil && *params.Force {
		t.Error("the removal is forced, so herdr would not refuse a checkout with work in it")
	}
}

func TestMovingAPaneOffersEveryOtherTab(t *testing.T) {
	list := choices(t, "herdr:pane.move")

	want := []string{"t0", "t2", "t9"}
	if got := values(list); len(got) != len(want) || got[0] != want[0] || got[2] != want[2] {
		t.Fatalf("offered %v, want %v: every tab but the one the palette was opened in", got, want)
	}
	if list[2].Detail != "notes" {
		t.Errorf("detail = %q, want the workspace the tab sits in", list[2].Detail)
	}
}

func TestMovingAPaneTargetsTheChosenTab(t *testing.T) {
	var params herdr.PaneMoveParams
	decode(t, run(t, "herdr:pane.move", step{chosen: "t9"})[0].Params, &params)

	if params.PaneID != "p1" {
		t.Errorf("moved %q, want the focused pane", params.PaneID)
	}
	tab, ok := params.Destination.(herdr.PaneMoveDestinationTab)
	if !ok {
		t.Fatalf("destination is %T, want the tab that was picked", params.Destination)
	}
	if tab.TabID != "t9" {
		t.Errorf("destination tab = %q, want the one that was picked", tab.TabID)
	}
}

// herdr reads an insert index as a position in the list as it stands, so a
// place on is two indexes ahead and a place back one behind.
func TestMovingATabCountsItsPlaceInTheWorkspace(t *testing.T) {
	for _, c := range []struct {
		id     string
		insert uint64
	}{
		{id: "herdr:tab.move.next", insert: 3},
		{id: "herdr:tab.move.previous", insert: 0},
	} {
		t.Run(c.id, func(t *testing.T) {
			calls := run(t, c.id, step{})
			if len(calls) != 2 || calls[1].Method != herdr.MethodTabMove {
				t.Fatalf("made %v, want the snapshot and a move", calls)
			}

			var params herdr.TabMoveParams
			decode(t, calls[1].Params, &params)
			if params.TabID != "t1" || params.InsertIndex != c.insert {
				t.Errorf("moved %+v, want tab t1 to insert index %d", params, c.insert)
			}
		})
	}
}

// Past either end herdr refuses the index, so a tab already there stays put.
func TestMovingATabPastTheEndDoesNothing(t *testing.T) {
	for _, c := range []struct{ id, tab string }{
		{id: "herdr:tab.move.next", tab: "t2"},
		{id: "herdr:tab.move.previous", tab: "t0"},
	} {
		t.Run(c.id, func(t *testing.T) {
			ctx := fullContext()
			ctx.TabID = new(c.tab)

			for _, call := range runIn(t, c.id, step{}, ctx) {
				if call.Method == herdr.MethodTabMove {
					t.Errorf("tab %s was moved although it is at the end it moved toward", c.tab)
				}
			}
		})
	}
}

func TestCyclingFocusWrapsRoundTheTab(t *testing.T) {
	for _, c := range []struct{ id, want string }{
		{id: "herdr:pane.cycle.next", want: "p2"},
		{id: "herdr:pane.cycle.previous", want: "p3"},
	} {
		t.Run(c.id, func(t *testing.T) {
			calls := run(t, c.id, step{})
			if len(calls) != 2 || calls[1].Method != herdr.MethodPaneFocus {
				t.Fatalf("made %v, want the snapshot and a focus", calls)
			}

			var params herdr.PaneTarget
			decode(t, calls[1].Params, &params)
			if params.PaneID != c.want {
				t.Errorf("focused %q, want %q", params.PaneID, c.want)
			}
		})
	}
}

// A pane alone in its tab has nowhere to cycle to.
func TestCyclingASinglePaneDoesNothing(t *testing.T) {
	ctx := fullContext()
	ctx.FocusedPaneID = new("p9")

	for _, call := range runIn(t, "herdr:pane.cycle.next", step{}, ctx) {
		if call.Method == herdr.MethodPaneFocus {
			t.Error("focus was moved although the tab holds one pane")
		}
	}
}

// herdr names an agent after the kind it started, which is what its own UI
// shows until the agent is renamed.
func TestStartingAnAgentNamesItAfterTheKindInTheFocusedPane(t *testing.T) {
	var params herdr.AgentStartParams
	decode(t, run(t, "herdr:agent.start", step{chosen: "codex"})[0].Params, &params)

	if params.Kind != "codex" || params.Name != "codex" {
		t.Errorf("started %+v, want the agent that was picked", params)
	}
	if params.PaneID != "p1" {
		t.Errorf("pane = %q, want the one that was focused", params.PaneID)
	}
}

func TestStartingAnAgentOffersEveryManifest(t *testing.T) {
	if got := values(choices(t, "herdr:agent.start")); len(got) != 2 || got[1] != "codex" {
		t.Errorf("offered %v, want the agents herdr keeps a manifest for", got)
	}
}

func TestRenamingAnAgentRefusesAPaneWithoutOne(t *testing.T) {
	s := server(t)
	ctx := fullContext()
	ctx.FocusedPaneAgent = nil

	err := entry(t, "herdr:agent.rename").Run(context.Background(), palette.Exec{
		Client: s.Env().Client(),
		Ctx:    ctx,
		Input:  "reviewer",
	})
	if err == nil {
		t.Fatal("Run() renamed the agent of a pane that runs none")
	}
}

func TestRenamingAnAgentStartsFromWhatItIs(t *testing.T) {
	if got := entry(t, "herdr:agent.rename").Initial(fullContext()); got != "claude" {
		t.Errorf("initial value = %q, want the agent running in the focused pane", got)
	}
}

// Saving, opening and forgetting a layout work on the palette's own store, so
// one environment serves the three of them.
func TestALayoutIsSavedOpenedAndForgotten(t *testing.T) {
	s := server(t)
	env := s.Env(plugintest.StateDir(t.TempDir()))

	if err := execute(t, env, "herdr:layout.save", step{input: "work"}, fullContext()); err != nil {
		t.Fatalf("saving the layout: %v", err)
	}
	var export herdr.LayoutExportParams
	decode(t, paramsOf(t, s, herdr.MethodLayoutExport), &export)
	if export.TabID == nil || *export.TabID != "t1" {
		t.Errorf("exported %+v, want the focused tab", export)
	}

	e := entry(t, "herdr:layout.apply")
	list, err := e.Choices.List(context.Background(), palette.Exec{Client: env.Client(), Ctx: fullContext(), Env: env})
	if err != nil {
		t.Fatalf("listing the saved layouts: %v", err)
	}
	if got := values(list); len(got) != 1 || got[0] != "work" {
		t.Fatalf("offered %v, want the layout that was just saved", got)
	}

	if err := execute(t, env, "herdr:layout.apply", step{chosen: "work"}, fullContext()); err != nil {
		t.Fatalf("opening the layout: %v", err)
	}
	var apply herdr.LayoutApplyParams
	decode(t, paramsOf(t, s, herdr.MethodLayoutApply), &apply)
	if apply.TabLabel == nil || *apply.TabLabel != "work" {
		t.Errorf("applied %+v, want a new tab named after the layout", apply)
	}
	if apply.WorkspaceID == nil || *apply.WorkspaceID != "w1" {
		t.Error("the layout is not opened in the focused workspace")
	}
	for _, pane := range herdr.LayoutPanes(apply.Root) {
		if pane.PaneID != nil {
			t.Errorf("the applied layout names pane %q, which would be moved instead of opened", *pane.PaneID)
		}
	}

	if err := execute(t, env, "herdr:layout.forget", step{chosen: "work"}, fullContext()); err != nil {
		t.Fatalf("forgetting the layout: %v", err)
	}
	if left := layout.List(env); len(left) != 0 {
		t.Errorf("%d layouts are still saved", len(left))
	}
}

func TestOpeningALayoutThatIsNotSaved(t *testing.T) {
	s := server(t)
	env := s.Env(plugintest.StateDir(t.TempDir()))

	if err := execute(t, env, "herdr:layout.apply", step{chosen: "work"}, fullContext()); err == nil {
		t.Fatal("Run() reported no error for a layout that was never saved")
	}
	for _, call := range s.Calls() {
		if call.Method == herdr.MethodLayoutApply {
			t.Error("herdr was asked to apply a layout the palette does not have")
		}
	}
}

func TestPromptingAnAgentOffersEveryOneInTheSession(t *testing.T) {
	list := choices(t, "herdr:agent.prompt.any")

	if got := values(list); len(got) != 1 || got[0] != "p9" {
		t.Fatalf("offered %v, want the pane running an agent", got)
	}
	if list[0].Title != "reviewer" || list[0].Detail != "working" {
		t.Errorf("row = %+v, want the name the agent goes by and what it is doing", list[0])
	}
	if !strings.Contains(list[0].Search, "notes") {
		t.Errorf("search text = %q, want the workspace the agent sits in", list[0].Search)
	}
}

// The target is picked from the list and the text typed afterwards, so the
// entry runs on both.
func TestPromptingAnAgentSendsTheTextToThePickedPane(t *testing.T) {
	var params herdr.AgentPromptParams
	decode(t, run(t, "herdr:agent.prompt.any", step{chosen: "p9", input: "go on"})[0].Params, &params)

	if params.Target != "p9" || params.Text != "go on" {
		t.Errorf("prompted %+v, want the agent that was picked to receive the text", params)
	}
}

// Turning the palette off would take away the popup the row is being run from,
// with no row left to turn it back on, so it is not one of them.
func TestManagingPluginsListsTheOthersAndWhatTheyAre(t *testing.T) {
	list := choices(t, "herdr:plugin.manage")

	want := []string{"herdr.machine-manager", "herdr.auto-title"}
	got := values(list)
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("offered %v, want %v: every installed plugin but this one", got, want)
	}
	if list[0].Title != "Machine Manager" || list[0].Detail != "enabled" {
		t.Errorf("row = %+v, want the plugin's name and that it is on", list[0])
	}
	if list[1].Detail != "disabled" || list[1].Search != "herdr.auto-title" {
		t.Errorf("row = %+v, want that it is off, found by its id too", list[1])
	}
	// The state is drawn in a colour of its own, the way an agent's status is.
	if list[0].Status != "enabled" || list[1].Status != "disabled" {
		t.Errorf("states = %q and %q, want each row to carry the one it shows", list[0].Status, list[1].Status)
	}
}

// The list is a screen, so what a plugin is now is read again rather than
// taken from the row it was drawn on.
func TestTurningAPluginOverGoesByWhatItIsNow(t *testing.T) {
	for _, c := range []struct{ plugin, method string }{
		{plugin: "herdr.machine-manager", method: herdr.MethodPluginDisable},
		{plugin: "herdr.auto-title", method: herdr.MethodPluginEnable},
	} {
		t.Run(c.plugin, func(t *testing.T) {
			calls := run(t, "herdr:plugin.manage", step{chosen: c.plugin})
			if len(calls) != 2 {
				t.Fatalf("made %v, want the list and the change", calls)
			}
			if calls[1].Method != c.method {
				t.Errorf("called %q, want %q", calls[1].Method, c.method)
			}
		})
	}
}

// The file is the one herdr reads, which is also where the palette reads the
// keys and the configured commands from.
func TestEditingTheConfigOpensTheFileHerdrReads(t *testing.T) {
	t.Setenv("HERDR_CONFIG_PATH", "/tmp/herdr-config.toml")

	var params herdr.PluginPaneOpenParams
	decode(t, run(t, "herdr:config.edit", step{})[0].Params, &params)

	if !strings.HasSuffix(params.Env[palette.RunEnv], "'/tmp/herdr-config.toml'") {
		t.Errorf("opened %q, want herdr's own configuration file", params.Env[palette.RunEnv])
	}
}
