package palette

import (
	"context"
	"testing"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin/plugintest"
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

func pluginList() herdr.PluginListResponse {
	return herdr.PluginListResponse{Plugins: []herdr.InstalledPluginInfo{
		{PluginID: "herdr.machine-manager", Name: "Machine Manager"},
	}}
}

func load(t *testing.T, server *plugintest.Server) []Entry {
	t.Helper()
	catalog := []Entry{{ID: "herdr:tab.new", Title: "New tab", Detail: "Tab"}}
	entries, err := Load(context.Background(), server.Env().Client(), own, catalog)
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
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, pluginList())

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
	if entry.Detail != "Machine Manager" {
		t.Errorf("detail = %q, want the plugin's manifest name", entry.Detail)
	}
}

func TestLoadLeavesOutItsOwnEntrypoint(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, pluginList())

	if _, ok := find(load(t, server), "plugin:"+own+"/open"); ok {
		t.Error("Load() offered the palette's own action, which would only reopen it")
	}
}

func TestLoadMarksSelectionOnlyActions(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList()).
		Reply(herdr.MethodPluginList, pluginList())

	entry, ok := find(load(t, server), "plugin:herdr.notes/clip")
	if !ok {
		t.Fatal("Load() dropped the selection action")
	}
	if !entry.NeedsSelection {
		t.Error("a selection-only action was not marked, so it would show with nothing selected")
	}
}

func TestLoadFallsBackToThePluginIDWithoutAPluginList(t *testing.T) {
	server := plugintest.NewServer(t).
		Reply(herdr.MethodPluginActionList, actionList())

	entry, ok := find(load(t, server), "plugin:herdr.machine-manager/open")
	if !ok {
		t.Fatal("Load() dropped the machine manager action")
	}
	if entry.Detail != "herdr.machine-manager" {
		t.Errorf("detail = %q, want the plugin id when plugin.list is unavailable", entry.Detail)
	}
}

func TestLoadKeepsTheCatalogWhenTheActionListFails(t *testing.T) {
	server := plugintest.NewServer(t)

	catalog := []Entry{{ID: "herdr:tab.new", Title: "New tab", Detail: "Tab"}}
	entries, err := Load(context.Background(), server.Env().Client(), own, catalog)
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
		Reply(herdr.MethodPluginList, pluginList()).
		Reply(herdr.MethodPluginActionInvoke, herdr.PluginActionInvokedResponse{})

	client := server.Env().Client()
	entries, err := Load(context.Background(), client, own, nil)
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	entry, ok := find(entries, "plugin:herdr.machine-manager/open")
	if !ok {
		t.Fatal("Load() dropped the machine manager action")
	}

	invocation := &herdr.PluginInvocationContext{WorkspaceID: herdr.Ptr("w1")}
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
