package palette

import (
	"context"
	"fmt"

	"github.com/vika2603/herdr-client/herdr"

	"github.com/vika2603/herdr-palette/internal/keys"
)

// Load returns the entries to show: the catalog, the commands configured under
// [[keys.command]], and every action the other plugins registered. own is this
// plugin's id, whose own entrypoint would only reopen the palette. cfg
// supplies the key each entry is bound to.
//
// A failure to reach plugin.action.list is not fatal. The rest of the list is
// still worth showing, so the caller reports the error next to it.
func Load(ctx context.Context, client *herdr.Client, own string, catalog []Entry, cfg keys.Config) ([]Entry, error) {
	entries := make([]Entry, 0, len(catalog)+len(cfg.Custom))
	for _, entry := range catalog {
		entry.Key = cfg.Action[entry.Binding]
		entries = append(entries, entry)
	}
	entries = append(entries, customEntries(own, cfg.Custom)...)

	actions, err := client.PluginActionList(ctx, herdr.PluginActionListParams{})
	if err != nil {
		return entries, err
	}

	for _, action := range actions.Actions {
		if action.PluginID == own {
			continue
		}
		entry := pluginEntry(action)
		entry.Key = cfg.Plugin[action.PluginID+"."+action.ActionID]
		entries = append(entries, entry)
	}
	return entries, nil
}

func pluginEntry(action herdr.PluginActionInfo) Entry {
	pluginID := action.PluginID
	actionID := action.ActionID
	return Entry{
		ID:             "plugin:" + pluginID + "/" + actionID,
		Title:          action.Title,
		Type:           TypePlugin,
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
