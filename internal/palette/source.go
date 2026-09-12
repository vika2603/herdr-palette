package palette

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/vika2603/herdr-client/herdr"

	"github.com/vika2603/herdr-palette/internal/keys"
)

// List is the palette's rows in two halves. Commands is fixed for as long as
// the popup is up; Open is what the session holds, which the popup rebuilds as
// herdr reports changes.
type List struct {
	Commands []Entry
	Open     []Entry
}

// All is both halves in the order they are shown.
func (l List) All() []Entry {
	return append(append(make([]Entry, 0, len(l.Commands)+len(l.Open)), l.Commands...), l.Open...)
}

// Load returns the entries to show: the catalog, the commands configured under
// [[keys.command]], every action the other plugins registered, and what is
// open in the session. own is this plugin's id, whose own entrypoint would only
// reopen the palette. cfg supplies the key each entry is bound to.
//
// Neither call is fatal. The rest of the list is still worth showing, so the
// caller reports what was missed next to it.
func Load(
	ctx context.Context,
	client *herdr.Client,
	own string,
	catalog []Entry,
	cfg keys.Config,
) (List, error) {
	commands := make([]Entry, 0, len(catalog)+len(cfg.Custom))
	for _, entry := range catalog {
		entry.Key = cfg.Action[entry.Binding]
		commands = append(commands, entry)
	}
	commands = append(commands, customEntries(own, cfg.Custom)...)

	var failures []error
	if actions, err := client.PluginActionList(ctx, herdr.PluginActionListParams{}); err != nil {
		failures = append(failures, fmt.Errorf("plugin actions unavailable: %w", err))
	} else {
		installed := installedPlugins(ctx, client)
		for _, action := range actions.Actions {
			if action.PluginID == own {
				continue
			}
			// herdr lists the actions of a disabled plugin as well, and
			// refuses to invoke one with plugin_disabled, so they are dropped
			// here rather than offered and failed. An action whose plugin is
			// not in the list — which is what an unreachable list leaves —
			// stays, since nothing says it cannot run.
			plugin, listed := installed[action.PluginID]
			if listed && !plugin.Enabled {
				continue
			}
			entry := pluginEntry(action, plugin.Name)
			entry.Key = cfg.Plugin[keys.PluginBinding(action.PluginID, action.ActionID)]
			commands = append(commands, entry)
		}
	}

	open, err := OpenEntries(ctx, client)
	if err != nil {
		failures = append(failures, err)
	}
	return List{Commands: commands, Open: open}, errors.Join(failures...)
}

// installedPlugins maps each installed plugin to what herdr knows about it:
// the name it gave itself, which is the namespace its actions show under, and
// whether it is enabled. An unreachable list is not worth reporting — the id
// carries a usable name of its own — and leaves the map empty, which rules out
// no action.
func installedPlugins(ctx context.Context, client *herdr.Client) map[string]herdr.InstalledPluginInfo {
	plugins, err := client.PluginList(ctx, herdr.PluginListParams{})
	if err != nil {
		return nil
	}
	installed := make(map[string]herdr.InstalledPluginInfo, len(plugins.Plugins))
	for _, plugin := range plugins.Plugins {
		installed[plugin.PluginID] = plugin
	}
	return installed
}

// pluginNamespace is the plugin's own name, or the distinctive half of its id
// when the name is unavailable: "herdr.machine-manager" reads as
// "machine-manager".
func pluginNamespace(pluginID, name string) string {
	if name != "" {
		return name
	}
	if _, after, found := strings.Cut(pluginID, "."); found {
		return after
	}
	return pluginID
}

func pluginEntry(action herdr.PluginActionInfo, name string) Entry {
	pluginID := action.PluginID
	actionID := action.ActionID
	return Entry{
		ID:             "plugin:" + pluginID + "/" + actionID,
		Title:          strings.ToLower(action.Title),
		Type:           pluginNamespace(pluginID, name),
		NeedsSelection: onlySelection(action.Contexts),
		// A plugin action runs in the plugin's own process, so a popup it
		// cannot open is refused there and never reported back here. There is
		// nothing to try, so it is handed over unconditionally.
		AlwaysRelay: true,
		// The row shows the plugin's name, but typing its id is a natural way
		// to find its actions too.
		Search: pluginID,
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
