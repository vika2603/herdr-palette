package palette

import (
	"context"
	"strings"
	"testing"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin/manifest"
	"github.com/vika2603/herdr-client/plugin/plugintest"

	"github.com/vika2603/herdr-palette/internal/keys"
	"github.com/vika2603/herdr-palette/internal/settings"
)

const own = "herdr.palette"

func actionList() herdr.PluginActionListResponse {
	return herdr.PluginActionListResponse{Actions: []herdr.PluginActionInfo{
		{PluginID: "herdr.machine-manager", ActionID: "open", Title: "Manage machines"},
		{PluginID: own, ActionID: "open", Title: "Open command palette"},
		{
			PluginID: "herdr.notes",
			ActionID: "clip",
			Title:    "Clip the selection",
			Contexts: []herdr.PluginActionContext{herdr.PluginActionContextSelection},
		},
	}}
}

// plugins is what herdr reports about what is installed, which is where the
// name in front of a plugin action comes from.
func plugins() herdr.PluginListResponse {
	return herdr.PluginListResponse{Plugins: []herdr.InstalledPluginInfo{
		{PluginID: "herdr.machine-manager", Name: "Machine Manager"},
		{PluginID: own, Name: "Command Palette"},
	}}
}

// snapshot is a session with one other workspace to go to, and a pane in it.
func snapshot() herdr.SessionSnapshotResponse {
	return herdr.SessionSnapshotResponse{Snapshot: herdr.SessionSnapshot{
		Workspaces: []herdr.WorkspaceInfo{
			{WorkspaceID: "w1", Label: "helix", Focused: true},
			{WorkspaceID: "w2", Label: "palette"},
		},
		Tabs: []herdr.TabInfo{
			{TabID: "w1:t1", WorkspaceID: "w1", Label: "1 · helix", Focused: true},
			{TabID: "w2:t1", WorkspaceID: "w2", Label: "2 · palette › claude"},
		},
		Panes: []herdr.PaneInfo{
			{PaneID: "w1:p1", WorkspaceID: "w1", TabID: "w1:t1", Focused: true},
			{
				PaneID:                "w2:p1",
				WorkspaceID:           "w2",
				TabID:                 "w2:t1",
				TerminalTitleStripped: new("Zoxide jump"),
				Cwd:                   new("/Users/vika/Workspace"),
			},
		},
	}}
}

// config is what the palette would read from herdr's configuration.
func config() keys.Config {
	return keys.Config{
		Action: map[string]string{"new_tab": "prefix+c"},
		Plugin: map[string]string{"herdr.machine-manager.open": "prefix+shift+s"},
		Custom: []keys.Custom{
			{Key: "prefix+f", Description: "Open git jump", Type: keys.TypePopup, Command: "jump.sh", Width: manifest.PopupSize{Percent: 70}, Height: manifest.PopupSize{Percent: 60}},
			{Key: "prefix+alt+g", Description: "Open Lazygit", Type: keys.TypePane, Command: "lazygit"},
		},
	}
}

func load(t *testing.T, server *plugintest.Server) []Entry {
	t.Helper()
	catalog := []Entry{{ID: "herdr:tab.new", Title: "New tab", Type: "herdr", Binding: "new_tab"}}
	list, err := Load(context.Background(), server.Env().Client(), own, catalog, config(), nil)
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	return list.All()
}

func find(entries []Entry, id string) (Entry, bool) {
	for _, entry := range entries {
		if entry.ID == id {
			return entry, true
		}
	}
	return Entry{}, false
}

func TestLoadMergesTheCatalogWithPluginActions(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot())

	entries := load(t, server)

	if _, ok := find(entries, "herdr:tab.new"); !ok {
		t.Error("Load() dropped the catalog entry")
	}
	entry, ok := find(entries, "plugin:herdr.machine-manager/open")
	if !ok {
		t.Fatal("Load() dropped the machine manager action")
	}
	if entry.Title != "manage machines" {
		t.Errorf("title = %q, want the action's own title", entry.Title)
	}
	if entry.Type != "Machine Manager" {
		t.Errorf("namespace = %q, want the plugin's own name", entry.Type)
	}
}

func TestLoadLeavesOutItsOwnEntrypoint(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot())

	if _, ok := find(load(t, server), "plugin:"+own+"/open"); ok {
		t.Error("Load() offered the palette's own action, which would only reopen it")
	}
}

func TestLoadMarksSelectionOnlyActions(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot())

	entry, ok := find(load(t, server), "plugin:herdr.notes/clip")
	if !ok {
		t.Fatal("Load() dropped the selection action")
	}
	if !entry.NeedsSelection {
		t.Error("a selection-only action was not marked, so it would show with nothing selected")
	}
}

func TestLoadKeepsTheCatalogWhenTheSessionIsUnreachable(t *testing.T) {
	server := plugintest.NewServer(t)

	catalog := []Entry{{ID: "herdr:tab.new", Title: "New tab", Type: "herdr"}}
	list, err := Load(context.Background(), server.Env().Client(), own, catalog, keys.Config{}, nil)
	if err == nil {
		t.Fatal("Load() reported no error although the action list was unavailable")
	}
	if len(list.All()) != 1 {
		t.Errorf("Load() returned %d entries, want the catalog to survive", len(list.All()))
	}
}

func TestPluginEntryInvokesTheAction(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot()).
		Reply(herdr.MethodPluginActionInvoke, herdr.PluginActionInvokedResponse{})

	client := server.Env().Client()
	list, err := Load(context.Background(), client, own, nil, config(), nil)
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	entry, ok := find(list.All(), "plugin:herdr.machine-manager/open")
	if !ok {
		t.Fatal("Load() dropped the machine manager action")
	}

	invocation := &herdr.PluginInvocationContext{WorkspaceID: new("w1")}
	if err := entry.Run(context.Background(), Exec{Client: client, Ctx: invocation}); err != nil {
		t.Fatalf("Run() = %v", err)
	}

	calls := server.Calls()
	last := calls[len(calls)-1]
	if last.Method != herdr.MethodPluginActionInvoke {
		t.Fatalf("called %q, want %q", last.Method, herdr.MethodPluginActionInvoke)
	}
	var params herdr.PluginActionInvokeParams
	decode(t, last.Params, &params)
	if params.ActionID != "open" || params.PluginID == nil || *params.PluginID != "herdr.machine-manager" {
		t.Errorf("invoked %+v, want the machine manager's open action", params)
	}
	if params.Context == nil || params.Context.WorkspaceID == nil || *params.Context.WorkspaceID != "w1" {
		t.Error("the invocation context was not passed on, so the action loses what was focused")
	}
}

func TestEntriesCarryTheKeyTheyAreBoundTo(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot())
	entries := load(t, server)

	catalogEntry, _ := find(entries, "herdr:tab.new")
	if catalogEntry.Key != "prefix+c" {
		t.Errorf("catalog key = %q, want the key bound to new_tab", catalogEntry.Key)
	}
	pluginAction, _ := find(entries, "plugin:herdr.machine-manager/open")
	if pluginAction.Key != "prefix+shift+s" {
		t.Errorf("plugin action key = %q, want the key bound to it", pluginAction.Key)
	}
}

func TestConfiguredCommandsAreListed(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot())
	entries := load(t, server)

	entry, ok := find(entries, "config:prefix+f")
	if !ok {
		t.Fatal("the configured popup command is not in the list")
	}
	if entry.Title != "open git jump" || entry.Type != TypeCustom || entry.Key != "prefix+f" {
		t.Errorf("entry = %+v, want the configured description, group and key", entry)
	}
	if _, ok := find(entries, "config:prefix+alt+g"); !ok {
		t.Error("the configured pane command is not in the list")
	}
}

func TestAPopupCommandOpensAPluginPane(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot()).
		Reply(herdr.MethodPluginPaneOpen, herdr.OKResponse{})

	client := server.Env().Client()
	list, err := Load(context.Background(), client, own, nil, config(), nil)
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	entry, _ := find(list.All(), "config:prefix+f")

	invocation := &herdr.PluginInvocationContext{WorkspaceID: new("w1")}
	if err := entry.Run(context.Background(), Exec{Client: client, Ctx: invocation}); err != nil {
		t.Fatalf("Run() = %v", err)
	}

	calls := server.Calls()
	last := calls[len(calls)-1]
	if last.Method != herdr.MethodPluginPaneOpen {
		t.Fatalf("called %q, want %q", last.Method, herdr.MethodPluginPaneOpen)
	}
	var params herdr.PluginPaneOpenParams
	decode(t, last.Params, &params)
	if params.Entrypoint != RunEntrypoint || params.PluginID != own {
		t.Errorf("opened %s/%s, want this plugin's run entrypoint", params.PluginID, params.Entrypoint)
	}
	if params.Placement == nil || *params.Placement != herdr.PluginPanePlacementPopup {
		t.Error("a popup command did not open a popup")
	}
	if params.Env[RunEnv] != "jump.sh" {
		t.Errorf("the pane receives %q, want the configured command line", params.Env[RunEnv])
	}
	if params.Width == nil || params.Width.Percent != 70 {
		t.Errorf("width = %v, want the configured 70%%", params.Width)
	}
}

func TestAPaneCommandOpensAZoomedPane(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot()).
		Reply(herdr.MethodPluginPaneOpen, herdr.OKResponse{})

	client := server.Env().Client()
	list, _ := Load(context.Background(), client, own, nil, config(), nil)
	entry, _ := find(list.All(), "config:prefix+alt+g")

	invocation := &herdr.PluginInvocationContext{FocusedPaneID: new("w1:p1")}
	if err := entry.Run(context.Background(), Exec{Client: client, Ctx: invocation}); err != nil {
		t.Fatalf("Run() = %v", err)
	}
	var params herdr.PluginPaneOpenParams
	calls := server.Calls()
	decode(t, calls[len(calls)-1].Params, &params)
	if params.Placement == nil || *params.Placement != herdr.PluginPanePlacementZoomed {
		t.Error("a pane command opened beside the focused pane instead of taking over the layout")
	}
	if params.TargetPaneID == nil || *params.TargetPaneID != "w1:p1" {
		t.Error("a zoomed pane was opened without the pane it is placed against, which herdr rejects")
	}
	if params.WorkspaceID != nil {
		t.Error("a zoomed pane carried a workspace id, which herdr rejects alongside the target pane")
	}
	if params.Width != nil || params.Height != nil {
		t.Error("a pane command sent a popup size")
	}
}

func TestAShellCommandRunsWithoutTheAPI(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot())

	client := server.Env().Client()
	cfg := config()
	cfg.Custom = []keys.Custom{{Key: "prefix+t", Description: "Touch a file", Type: keys.TypeShell, Command: "true"}}
	list, _ := Load(context.Background(), client, own, nil, cfg, nil)
	entry, ok := find(list.All(), "config:prefix+t")
	if !ok {
		t.Fatal("the shell command is not in the list")
	}

	before := len(server.Calls())
	if err := entry.Run(context.Background(), Exec{Client: client, Ctx: &herdr.PluginInvocationContext{}}); err != nil {
		t.Fatalf("Run() = %v", err)
	}
	if len(server.Calls()) != before {
		t.Error("a shell command went through the API, which cannot run one")
	}
}

func TestPopupSizeIsOnlySentWhenConfigured(t *testing.T) {
	if _, ok := PopupSize(manifest.PopupSize{}); ok {
		t.Error("an unset size was sent as a size")
	}
	size, ok := PopupSize(manifest.PopupSize{Percent: 70})
	if !ok || size.Percent != 70 {
		t.Errorf("PopupSize() = %v, %v, want the configured percentage", size, ok)
	}
	if size, ok := PopupSize(manifest.PopupSize{Cells: 80}); !ok || size.Cells != 80 {
		t.Errorf("PopupSize() = %v, %v, want the configured cell count", size, ok)
	}
}

func TestOpenPanesAndWorkspacesAreListedToGoTo(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot())
	entries := load(t, server)

	workspace, ok := find(entries, "workspace:w2")
	if !ok {
		t.Fatal("the other workspace is not in the list")
	}
	if workspace.Type != TypeWorkspace || workspace.Title != "go to palette" {
		t.Errorf("entry = %+v, want going to the workspace under its own label", workspace)
	}
	pane, ok := find(entries, "pane:w2:p1")
	if !ok {
		t.Fatal("the open pane is not in the list")
	}
	if !strings.Contains(pane.Title, "Zoxide jump") {
		t.Errorf("pane title = %q, want what the program in it reports", pane.Title)
	}
	if pane.Search != "/Users/vika/Workspace" {
		t.Errorf("pane search text = %q, want the directory it sits in", pane.Search)
	}
	if pane.Detail != "palette" {
		t.Errorf("pane detail = %q, want the workspace it sits in", pane.Detail)
	}
	if _, ok := find(entries, "tab:w2:t1"); !ok {
		t.Error("the open tab is not in the list")
	}
}

func TestWhereThePaletteWasOpenedFromIsNotListed(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot())
	entries := load(t, server)

	for _, id := range []string{"workspace:w1", "tab:w1:t1", "pane:w1:p1"} {
		if _, ok := find(entries, id); ok {
			t.Errorf("%s is offered, which goes where the palette already is", id)
		}
	}
}

func TestGoingToAPaneFocusesIt(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot()).
		Reply(herdr.MethodPaneFocus, herdr.PaneInfoResponse{})
	entries := load(t, server)
	pane, _ := find(entries, "pane:w2:p1")

	client := server.Env().Client()
	if err := pane.Run(context.Background(), Exec{Client: client, Ctx: &herdr.PluginInvocationContext{}}); err != nil {
		t.Fatalf("Run() = %v", err)
	}

	calls := server.Calls()
	last := calls[len(calls)-1]
	if last.Method != herdr.MethodPaneFocus {
		t.Fatalf("called %q, want the pane to be focused", last.Method)
	}
	var params herdr.PaneTarget
	decode(t, last.Params, &params)
	if params.PaneID != "w2:p1" {
		t.Errorf("focused %q, want the pane the row stands for", params.PaneID)
	}
}

func TestAPluginActionFallsBackToItsID(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodSessionSnapshot, snapshot())

	list, _ := Load(context.Background(), server.Env().Client(), own, nil, keys.Config{}, nil)
	entry, ok := find(list.All(), "plugin:herdr.machine-manager/open")
	if !ok {
		t.Fatal("Load() dropped the machine manager action when the plugin list was unavailable")
	}
	if entry.Type != "machine-manager" {
		t.Errorf("namespace = %q, want the distinctive half of the plugin id", entry.Type)
	}
}

// Typing what the rows have in common narrows the list to them, in either
// spelling.
func TestTheRowsThatGoSomewhereShareAQuery(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot())
	entries := load(t, server)

	for _, text := range []string{"go to", "goto"} {
		ranked := Rank(entries, text, nil)
		if len(ranked) != 3 {
			t.Errorf("%q matched %d rows, want the workspace, the tab and the pane", text, len(ranked))
		}
		for _, r := range ranked {
			if r.Entry.Type != TypeWorkspace && r.Entry.Type != TypeTab && r.Entry.Type != TypePane {
				t.Errorf("%q matched %q, which runs a command", text, r.Entry.Name())
			}
		}
	}
}

func TestTheOwnConfigurationAddsCommands(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot())

	own := []settings.Command{{Title: "sync dotfiles", Run: "zsh sync.sh"}}
	list, err := Load(context.Background(), server.Env().Client(), "herdr.palette", nil, keys.Config{}, own)
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}

	entry, ok := find(list.All(), "command:sync dotfiles")
	if !ok {
		t.Fatal("a command from the plugin's own configuration is not in the list")
	}
	if entry.Type != TypeCustom || entry.Key != "" {
		t.Errorf("entry = %+v, want a command of your own, bound to no key", entry)
	}
}

// A command with no window runs detached: nothing is shown, and the API is
// not involved at all.
func TestACommandWithNoWindowRunsWithoutTheAPI(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot())

	client := server.Env().Client()
	own := []settings.Command{{Title: "touch a file", Run: "true"}}
	list, _ := Load(context.Background(), client, "herdr.palette", nil, keys.Config{}, own)
	entry, _ := find(list.All(), "command:touch a file")

	before := len(server.Calls())
	if err := entry.Run(context.Background(), Exec{Client: client, Ctx: &herdr.PluginInvocationContext{}}); err != nil {
		t.Fatalf("Run() = %v", err)
	}
	if len(server.Calls()) != before {
		t.Error("a background command went through the API, which cannot run one")
	}
}

func TestACommandCanRunInATabOfItsOwn(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot()).
		Reply(herdr.MethodPluginPaneOpen, herdr.PluginPaneOpenedResponse{
			PluginPane: herdr.PluginPaneInfo{Pane: herdr.PaneInfo{PaneID: "w1:p9"}},
		}).
		Reply(herdr.MethodPaneRename, herdr.PaneInfoResponse{})

	client := server.Env().Client()
	own := []settings.Command{{Title: "watch tests", Run: "just watch", Window: settings.WindowTab}}
	list, _ := Load(context.Background(), client, "herdr.palette", nil, keys.Config{}, own)
	entry, _ := find(list.All(), "command:watch tests")

	if err := entry.Run(context.Background(), Exec{Client: client, Ctx: &herdr.PluginInvocationContext{}}); err != nil {
		t.Fatalf("Run() = %v", err)
	}

	calls := server.Calls()
	var opened herdr.PluginPaneOpenParams
	decode(t, calls[len(calls)-2].Params, &opened)
	if opened.Placement == nil || *opened.Placement != herdr.PluginPanePlacementTab {
		t.Errorf("placement = %v, want a tab of its own", opened.Placement)
	}
	if opened.Focus == nil || *opened.Focus {
		t.Error("the tab took the focus, so it is not running in the background")
	}

	var renamed herdr.PaneRenameParams
	decode(t, calls[len(calls)-1].Params, &renamed)
	if renamed.Label == nil || *renamed.Label != "watch tests" {
		t.Errorf("the pane was named %v, want the command's title so it can be found again", renamed.Label)
	}
}
