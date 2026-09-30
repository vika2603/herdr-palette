package palette

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin/manifest"
	"github.com/vika2603/herdr-client/plugin/plugintest"

	"github.com/vika2603/herdr-palette/internal/keys"
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
		{PluginID: "herdr.clipboard", ActionID: "paste", Title: "Paste"},
	}}
}

// plugins is what herdr reports about what is installed, which is where the
// name in front of a plugin action comes from, and whether it is enabled.
// herdr.clipboard is installed but off, and herdr lists its action all the
// same.
func plugins() herdr.PluginListResponse {
	return herdr.PluginListResponse{Plugins: []herdr.InstalledPluginInfo{
		{PluginID: "herdr.machine-manager", Name: "Machine Manager", Enabled: true},
		{PluginID: "herdr.notes", Name: "Notes", Enabled: true},
		{PluginID: "herdr.clipboard", Name: "Clipboard"},
		{PluginID: own, Name: "Command Palette", Enabled: true},
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
	list, err := Load(context.Background(), server.Env().Client(), own, catalog, config())
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	return list.Rows(ScopePalette)
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

// herdr lists the actions of a disabled plugin and then refuses to invoke one
// with plugin_disabled, so the palette would offer a row that cannot run.
func TestLoadLeavesOutADisabledPluginsActions(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot())

	if _, ok := find(load(t, server), "plugin:herdr.clipboard/paste"); ok {
		t.Error("Load() offered an action of a disabled plugin, which herdr refuses to invoke")
	}
}

func TestLoadKeepsTheCatalogWhenTheSessionIsUnreachable(t *testing.T) {
	server := plugintest.NewServer(t)

	catalog := []Entry{{ID: "herdr:tab.new", Title: "New tab", Type: "herdr"}}
	list, err := Load(context.Background(), server.Env().Client(), own, catalog, keys.Config{})
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
	list, err := Load(context.Background(), client, own, nil, config())
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
	list, err := Load(context.Background(), client, own, nil, config())
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
	list, _ := Load(context.Background(), client, own, nil, config())
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
	list, _ := Load(context.Background(), client, own, nil, cfg)
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

func TestOpenPanesAreListedToGoTo(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot())
	entries := load(t, server)

	pane, ok := find(entries, "pane:w2:p1")
	if !ok {
		t.Fatal("the open pane is not in the list")
	}
	if !strings.Contains(pane.Title, "Zoxide jump") {
		t.Errorf("pane title = %q, want what the program in it reports", pane.Title)
	}
	if want := "palette · 2 · palette › claude · /Users/vika/Workspace"; pane.Search != want {
		t.Errorf("pane search text = %q, want %q: the workspace, tab and directory it sits in", pane.Search, want)
	}
	if pane.Detail != "palette" {
		t.Errorf("pane detail = %q, want the workspace it sits in", pane.Detail)
	}
	// A workspace or a tab would mostly be the same place as the pane in it,
	// so they are in scopes of their own rather than in the command list.
	for _, id := range []string{"workspace:w2", "tab:w2:t1"} {
		if _, ok := find(entries, id); ok {
			t.Errorf("%s is in the command list", id)
		}
	}
}

// A pane is found by the name of the tab it sits in, which is not on its row.
func TestAPaneIsFoundByItsTabsName(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot())
	ranked := Rank(load(t, server), "claude", nil)
	if len(ranked) == 0 || ranked[0].Entry.ID != "pane:w2:p1" {
		t.Fatalf("\"claude\" ranked %v first, want the pane in the tab of that name", ranked)
	}
}

func TestWorkspacesAndTabsAreListed(t *testing.T) {
	s := snapshot().Snapshot
	s.Workspaces[1].TabCount = 3
	s.Workspaces[1].ActiveTabID = "w2:t1"
	s.Workspaces[1].AgentStatus = herdr.AgentStatusBlocked
	workspaces := workspaceEntries(s)
	w, ok := find(workspaces, "workspace:w2")
	if len(workspaces) != 2 || !ok || w.Title != GoTo+"palette" {
		t.Fatalf("workspaces = %+v, want both, under their labels", workspaces)
	}
	if w.Detail != "blocked · 3 tabs" || w.Status != "blocked" || w.Pane != "w2:p1" || w.Here {
		t.Errorf("workspace = %+v, want what its agents do and its tab count, previewed through its pane", w)
	}

	tabs := tabEntries(s)
	tab, ok := find(tabs, "tab:w2:t1")
	if len(tabs) != 2 || !ok || tab.Detail != "palette" || tab.Pane != "w2:p1" {
		t.Errorf("tabs = %+v, want both, the other one with its workspace and its pane", tabs)
	}
}

// Where the palette was opened from is listed so it can be closed from there,
// marked as such, but it is never where a keystroke goes first: going there
// goes nowhere, even when it is where the palette went last.
func TestWhereThePaletteWasOpenedFromIsListedLast(t *testing.T) {
	s := snapshot().Snapshot
	rows := append(paneEntries(s), tabEntries(s)...)
	rows = append(rows, workspaceEntries(s)...)
	for _, id := range []string{"pane:w1:p1", "tab:w1:t1", "workspace:w1"} {
		row, ok := find(rows, id)
		if !ok {
			t.Fatalf("%s is not listed", id)
		}
		if !row.Here || !strings.HasSuffix(row.Detail, "here") {
			t.Errorf("%s = %+v, want it marked as where the palette is", id, row)
		}
		if row.Close == nil {
			t.Errorf("%s cannot be closed from the list", id)
		}
	}

	panes := paneEntries(s)
	ranked := Rank(panes, "", []string{"pane:w1:p1", "pane:w2:p1"})
	if got := ranked[0].Entry.ID; got != "pane:w2:p1" {
		t.Errorf("first row is %q, want the other pane ahead of the one the palette is in", got)
	}
	if got := Rank(panes, "here", nil); len(got) != 1 || got[0].Entry.ID != "pane:w1:p1" {
		t.Errorf("\"here\" matched %v, want the pane the palette is in", got)
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

	list, _ := Load(context.Background(), server.Env().Client(), own, nil, keys.Config{})
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
		if len(ranked) != 2 {
			t.Errorf("%q matched %d rows, want the two panes", text, len(ranked))
		}
		for _, r := range ranked {
			if r.Entry.Type != TypePane && r.Entry.Type != TypeAgent {
				t.Errorf("%q matched %q, which runs a command", text, r.Entry.Name())
			}
		}
	}
}

// A background command runs where the focused pane is, the way one that opens
// a window does.
func TestABackgroundCommandRunsInTheFocusedPanesDirectory(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, plugins()).
		Reply(herdr.MethodSessionSnapshot, snapshot())

	dir := t.TempDir()
	cfg := keys.Config{Custom: []keys.Custom{
		{Key: "prefix+m", Description: "Mark the directory", Type: keys.TypeShell, Command: "touch marker"},
	}}
	list, _ := Load(context.Background(), server.Env().Client(), "herdr.palette", nil, cfg)
	entry, _ := find(list.All(), "config:prefix+m")

	invocation := &herdr.PluginInvocationContext{FocusedPaneCwd: &dir}
	if err := entry.Run(context.Background(), Exec{Client: server.Env().Client(), Ctx: invocation}); err != nil {
		t.Fatalf("Run() = %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(dir, "marker")); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("the command did not run in the focused pane's directory")
}

// A tab or a workspace closes from its row the way a pane does, through the
// herdr call that closes that kind of place.
func TestClosingATabOrAWorkspaceFromItsRow(t *testing.T) {
	s := snapshot().Snapshot
	rows := append(tabEntries(s), workspaceEntries(s)...)
	for _, c := range []struct {
		id, method, target string
	}{
		{"tab:w2:t1", herdr.MethodTabClose, "w2:t1"},
		{"workspace:w2", herdr.MethodWorkspaceClose, "w2"},
	} {
		row, _ := find(rows, c.id)
		server := plugintest.NewServer(t).Reply(c.method, herdr.OKResponse{})
		if err := row.Close(context.Background(), Exec{Client: server.Env().Client()}); err != nil {
			t.Fatalf("%s: Close() = %v", c.id, err)
		}
		calls := server.Calls()
		last := calls[len(calls)-1]
		var params struct {
			TabID       string `json:"tab_id"`
			WorkspaceID string `json:"workspace_id"`
		}
		decode(t, last.Params, &params)
		if last.Method != c.method || params.TabID+params.WorkspaceID != c.target {
			t.Errorf("%s called %q on %+v, want %q on %s", c.id, last.Method, params, c.method, c.target)
		}
	}
}
