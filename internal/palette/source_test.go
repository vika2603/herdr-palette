package palette

import (
	"context"
	"testing"

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
	entries, err := Load(context.Background(), server.Env().Client(), own, catalog, config())
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	return entries
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
		Reply(herdr.MethodPluginActionList, actionList())

	entries := load(t, server)

	if _, ok := find(entries, "herdr:tab.new"); !ok {
		t.Error("Load() dropped the catalog entry")
	}
	entry, ok := find(entries, "plugin:herdr.machine-manager/open")
	if !ok {
		t.Fatal("Load() dropped the machine manager action")
	}
	if entry.Title != "Manage machines" {
		t.Errorf("title = %q, want the action's own title", entry.Title)
	}
	if entry.Type != TypePlugin {
		t.Errorf("type = %q, want a plugin action to be marked as one", entry.Type)
	}
}

func TestLoadLeavesOutItsOwnEntrypoint(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList())

	if _, ok := find(load(t, server), "plugin:"+own+"/open"); ok {
		t.Error("Load() offered the palette's own action, which would only reopen it")
	}
}

func TestLoadMarksSelectionOnlyActions(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList())

	entry, ok := find(load(t, server), "plugin:herdr.notes/clip")
	if !ok {
		t.Fatal("Load() dropped the selection action")
	}
	if !entry.NeedsSelection {
		t.Error("a selection-only action was not marked, so it would show with nothing selected")
	}
}

func TestLoadKeepsTheCatalogWhenTheActionListFails(t *testing.T) {
	server := plugintest.NewServer(t)

	catalog := []Entry{{ID: "herdr:tab.new", Title: "New tab", Type: "herdr"}}
	entries, err := Load(context.Background(), server.Env().Client(), own, catalog, keys.Config{})
	if err == nil {
		t.Fatal("Load() reported no error although the action list was unavailable")
	}
	if len(entries) != 1 {
		t.Errorf("Load() returned %d entries, want the catalog to survive", len(entries))
	}
}

func TestPluginEntryInvokesTheAction(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginActionInvoke, herdr.PluginActionInvokedResponse{})

	client := server.Env().Client()
	entries, err := Load(context.Background(), client, own, nil, config())
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	entry, ok := find(entries, "plugin:herdr.machine-manager/open")
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
		Reply(herdr.MethodPluginActionList, actionList())
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
		Reply(herdr.MethodPluginActionList, actionList())
	entries := load(t, server)

	entry, ok := find(entries, "config:prefix+f")
	if !ok {
		t.Fatal("the configured popup command is not in the list")
	}
	if entry.Title != "Open git jump" || entry.Type != TypeCustom || entry.Key != "prefix+f" {
		t.Errorf("entry = %+v, want the configured description, group and key", entry)
	}
	if _, ok := find(entries, "config:prefix+alt+g"); !ok {
		t.Error("the configured pane command is not in the list")
	}
}

func TestAPopupCommandOpensAPluginPane(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginPaneOpen, herdr.OKResponse{})

	client := server.Env().Client()
	entries, err := Load(context.Background(), client, own, nil, config())
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	entry, _ := find(entries, "config:prefix+f")

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
		Reply(herdr.MethodPluginPaneOpen, herdr.OKResponse{})

	client := server.Env().Client()
	entries, _ := Load(context.Background(), client, own, nil, config())
	entry, _ := find(entries, "config:prefix+alt+g")

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
		Reply(herdr.MethodPluginActionList, actionList())

	client := server.Env().Client()
	cfg := config()
	cfg.Custom = []keys.Custom{{Key: "prefix+t", Description: "Touch a file", Type: keys.TypeShell, Command: "true"}}
	entries, _ := Load(context.Background(), client, own, nil, cfg)
	entry, ok := find(entries, "config:prefix+t")
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
	if _, ok := popupSize(manifest.PopupSize{}); ok {
		t.Error("an unset size was sent as a size")
	}
	size, ok := popupSize(manifest.PopupSize{Percent: 70})
	if !ok || size.Percent != 70 {
		t.Errorf("popupSize() = %v, %v, want the configured percentage", size, ok)
	}
	if size, ok := popupSize(manifest.PopupSize{Cells: 80}); !ok || size.Cells != 80 {
		t.Errorf("popupSize() = %v, %v, want the configured cell count", size, ok)
	}
}
