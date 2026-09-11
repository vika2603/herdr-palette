package palette

import (
	"context"
	"fmt"

	"github.com/vika2603/herdr-client/herdr"
)

// Load returns the entries to show, the catalog first and then every action
// the other plugins registered. own is this plugin's id: its own entrypoint
// would only reopen the palette.
//
// A failure to reach plugin.action.list is not fatal. The catalog alone is
// still worth showing, so the caller reports the error next to the list.
func Load(ctx context.Context, client *herdr.Client, own string, catalog []Entry) ([]Entry, error) {
	entries := make([]Entry, 0, len(catalog))
	entries = append(entries, catalog...)

	actions, err := client.PluginActionList(ctx, herdr.PluginActionListParams{})
	if err != nil {
		return entries, err
	}

	names := pluginNames(ctx, client)
	for _, action := range actions.Actions {
		if action.PluginID == own {
			continue
		}
		entries = append(entries, pluginEntry(action, names[action.PluginID]))
	}
	return entries, nil
}

// pluginNames maps plugin ids to their manifest names. The action list only
// carries ids, and a name reads better in the list. An unreachable plugin.list
// leaves the map empty, which falls back to the id.
func pluginNames(ctx context.Context, client *herdr.Client) map[string]string {
	names := map[string]string{}
	plugins, err := client.PluginList(ctx, herdr.PluginListParams{})
	if err != nil {
		return names
	}
	for _, p := range plugins.Plugins {
		names[p.PluginID] = p.Name
	}
	return names
}

func pluginEntry(action herdr.PluginActionInfo, name string) Entry {
	if name == "" {
		name = action.PluginID
	}
	pluginID := action.PluginID
	actionID := action.ActionID
	return Entry{
		ID:             "plugin:" + pluginID + "/" + actionID,
		Title:          action.Title,
		Detail:         name,
		NeedsSelection: onlySelection(action.Contexts),
		Run: func(ctx context.Context, e Exec) error {
			_, err := e.Client.PluginActionInvoke(ctx, herdr.PluginActionInvokeParams{
				PluginID: &pluginID,
				ActionID: actionID,
				Context:  e.Ctx,
			})
			if err != nil {
				return fmt.Errorf("%s: %w", actionID, err)
			}
			return nil
		},
	}
}

// onlySelection reports whether selection is the only context the action
// declares. An action that also declares global or pane contexts stays in the
// list without one.
func onlySelection(contexts []herdr.PluginActionContext) bool {
	if len(contexts) == 0 {
		return false
	}
	for _, c := range contexts {
		if c != herdr.PluginActionContextSelection {
			return false
		}
	}
	return true
}
