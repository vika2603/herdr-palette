package catalog

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin/plugintest"

	"github.com/vika2603/herdr-palette/internal/palette"
)

// fullContext is what herdr passes when the palette opens over a focused pane
// running an agent, inside a worktree workspace.
func fullContext() *herdr.PluginInvocationContext {
	return &herdr.PluginInvocationContext{
		WorkspaceID:      herdr.Ptr("w1"),
		WorkspaceLabel:   herdr.Ptr("workspace one"),
		WorkspaceCwd:     herdr.Ptr("/repo"),
		TabID:            herdr.Ptr("t1"),
		TabLabel:         herdr.Ptr("tab one"),
		FocusedPaneID:    herdr.Ptr("p1"),
		FocusedPaneCwd:   herdr.Ptr("/repo/sub"),
		FocusedPaneAgent: herdr.Ptr("claude"),
	}
}

// server answers every method the catalog can call, so one script serves the
// whole table.
func server(t *testing.T) *plugintest.Server {
	t.Helper()
	return plugintest.NewServer(t).
		Reply(herdr.MethodWorkspaceCreate, herdr.WorkspaceCreatedResponse{}).
		Reply(herdr.MethodWorkspaceRename, herdr.WorkspaceInfoResponse{}).
		Reply(herdr.MethodWorkspaceClose, herdr.OKResponse{}).
		Reply(herdr.MethodWorktreeCreate, herdr.WorktreeCreatedResponse{}).
		Reply(herdr.MethodWorktreeOpen, herdr.WorktreeOpenedResponse{}).
		Reply(herdr.MethodTabCreate, herdr.TabCreatedResponse{}).
		Reply(herdr.MethodTabRename, herdr.TabInfoResponse{}).
		Reply(herdr.MethodTabClose, herdr.OKResponse{}).
		Reply(herdr.MethodPaneSplit, herdr.PaneInfoResponse{}).
		Reply(herdr.MethodPaneZoom, herdr.PaneZoomResponse{}).
		Reply(herdr.MethodPaneRename, herdr.PaneInfoResponse{}).
		Reply(herdr.MethodPaneClose, herdr.OKResponse{}).
		Reply(herdr.MethodPaneEditScrollback, herdr.OKResponse{}).
		Reply(herdr.MethodPaneFocusDirection, herdr.PaneFocusDirectionResponse{}).
		Reply(herdr.MethodAgentPrompt, herdr.AgentPromptedResponse{}).
		Reply(herdr.MethodServerReloadConfig, herdr.ConfigReloadResponse{})
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

func decode(t *testing.T, raw json.RawMessage, into any) {
	t.Helper()
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatalf("decoding %s: %v", raw, err)
	}
}

// run executes one entry against the scripted server and returns the single
// call it made.
func run(t *testing.T, id, input string) plugintest.Call {
	t.Helper()
	s := server(t)
	e := entry(t, id)
	if err := e.Run(context.Background(), palette.Exec{
		Client: s.Env().Client(),
		Ctx:    fullContext(),
		Input:  input,
	}); err != nil {
		t.Fatalf("%s: Run() = %v", id, err)
	}
	calls := s.Calls()
	if len(calls) != 1 {
		t.Fatalf("%s made %d calls, want 1", id, len(calls))
	}
	return calls[0]
}

func TestEachEntryCallsItsMethod(t *testing.T) {
	cases := []struct {
		id     string
		input  string
		method string
	}{
		{id: "herdr:workspace.new", method: herdr.MethodWorkspaceCreate},
		{id: "herdr:workspace.rename", input: "renamed", method: herdr.MethodWorkspaceRename},
		{id: "herdr:workspace.close", method: herdr.MethodWorkspaceClose},
		{id: "herdr:worktree.new", input: "feature", method: herdr.MethodWorktreeCreate},
		{id: "herdr:worktree.open", input: "feature", method: herdr.MethodWorktreeOpen},
		{id: "herdr:tab.new", method: herdr.MethodTabCreate},
		{id: "herdr:tab.rename", input: "renamed", method: herdr.MethodTabRename},
		{id: "herdr:tab.close", method: herdr.MethodTabClose},
		{id: "herdr:pane.split.right", method: herdr.MethodPaneSplit},
		{id: "herdr:pane.split.down", method: herdr.MethodPaneSplit},
		{id: "herdr:pane.zoom", method: herdr.MethodPaneZoom},
		{id: "herdr:pane.rename", input: "renamed", method: herdr.MethodPaneRename},
		{id: "herdr:pane.close", method: herdr.MethodPaneClose},
		{id: "herdr:pane.edit_scrollback", method: herdr.MethodPaneEditScrollback},
		{id: "herdr:pane.focus.left", method: herdr.MethodPaneFocusDirection},
		{id: "herdr:pane.focus.down", method: herdr.MethodPaneFocusDirection},
		{id: "herdr:pane.focus.up", method: herdr.MethodPaneFocusDirection},
		{id: "herdr:pane.focus.right", method: herdr.MethodPaneFocusDirection},
		{id: "herdr:agent.prompt", input: "go on", method: herdr.MethodAgentPrompt},
		{id: "herdr:server.reload_config", method: herdr.MethodServerReloadConfig},
	}

	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			if got := run(t, c.id, c.input).Method; got != c.method {
				t.Errorf("called %q, want %q", got, c.method)
			}
		})
	}
}

func TestNewWorkspaceFollowsTheFocusedWorkspace(t *testing.T) {
	var params herdr.WorkspaceCreateParams
	decode(t, run(t, "herdr:workspace.new", "").Params, &params)

	if params.SourceWorkspaceID == nil || *params.SourceWorkspaceID != "w1" {
		t.Error("source_workspace_id was not sent, so the new workspace loses the cwd policy")
	}
	if params.Focus == nil || !*params.Focus {
		t.Error("the new workspace is not focused")
	}
}

func TestRenameSendsTheTypedValue(t *testing.T) {
	var params herdr.WorkspaceRenameParams
	decode(t, run(t, "herdr:workspace.rename", "renamed").Params, &params)

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
	decode(t, run(t, "herdr:tab.new", "").Params, &params)

	if params.WorkspaceID == nil || *params.WorkspaceID != "w1" {
		t.Error("the tab was not created in the focused workspace")
	}
	if params.Cwd == nil || *params.Cwd != "/repo/sub" {
		t.Error("the tab does not start in the focused pane's directory")
	}
}

func TestSplitTargetsTheFocusedPane(t *testing.T) {
	var params herdr.PaneSplitParams
	decode(t, run(t, "herdr:pane.split.down", "").Params, &params)

	if params.Direction != herdr.SplitDirectionDown {
		t.Errorf("direction = %q, want down", params.Direction)
	}
	if params.TargetPaneID == nil || *params.TargetPaneID != "p1" {
		t.Error("the split does not target the pane that was focused")
	}
}

func TestPromptTargetsTheFocusedPane(t *testing.T) {
	var params herdr.AgentPromptParams
	decode(t, run(t, "herdr:agent.prompt", "go on").Params, &params)

	if params.Target != "p1" || params.Text != "go on" {
		t.Errorf("prompted %+v, want the focused pane to receive the text", params)
	}
}

func TestWorktreeTakesTheBranchAndTheWorkspaceCwd(t *testing.T) {
	var params herdr.WorktreeCreateParams
	decode(t, run(t, "herdr:worktree.new", "feature").Params, &params)

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
