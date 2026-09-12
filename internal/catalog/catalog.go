// Package catalog is the hand-written list of herdr's own commands.
//
// herdr registers no action list of its own: plugin.action.list covers plugin
// actions, and the built-in actions exist only as config keys under [keys],
// with no method that runs one by name. command.invoke needs an opaque id
// issued through the client-shell projection, which a plugin does not receive.
// So each entry here names the socket API method that has the same effect as
// the built-in action. Built-ins with no API equivalent — settings, help,
// toggle_sidebar, resize_mode — are left out.
package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/vika2603/herdr-client/herdr"

	"github.com/vika2603/herdr-palette/internal/keys"
	"github.com/vika2603/herdr-palette/internal/layout"
	"github.com/vika2603/herdr-palette/internal/palette"
)

// groupHerdr is the namespace every command in this file shows under: they
// are all herdr's own, and the title says which part of it they act on.
const groupHerdr = "Herdr"

// Binding names come from herdr's [keys] section. split_vertical opens the new
// pane to the right and split_horizontal below it, as the keyboard
// documentation for 0.9.0 states.
//

// Errors for a command the current context cannot run. herdr fills the
// invocation context from what is focused, so a missing id means the palette
// was opened from somewhere the command does not apply to.
var (
	errNoWorkspace = errors.New("no focused workspace")
	errNoTab       = errors.New("no focused tab")
	errNoPane      = errors.New("no focused pane")
	errNoAgent     = errors.New("the focused pane runs no agent")
)

// Entries returns the catalog. The ids are stable: they key the recent order.
func Entries() []palette.Entry {
	return []palette.Entry{
		{
			ID:      "herdr:workspace.new",
			Binding: "new_workspace",
			Title:   "new workspace",
			Type:    groupHerdr,
			Run: func(ctx context.Context, e palette.Exec) error {
				// source_workspace_id lets the new workspace follow the
				// focused pane's cwd policy instead of starting at $HOME.
				_, err := e.Client.WorkspaceCreate(ctx, herdr.WorkspaceCreateParams{
					Focus:             new(true),
					SourceWorkspaceID: e.Ctx.WorkspaceID,
				})
				return err
			},
		},
		{
			ID:      "herdr:workspace.rename",
			Binding: "rename_workspace",
			Title:   "rename workspace",
			Type:    groupHerdr,
			Input: &palette.Input{
				Label:   "Workspace name",
				Initial: func(c *herdr.PluginInvocationContext) string { return herdr.Value(c.WorkspaceLabel) },
			},
			Run: func(ctx context.Context, e palette.Exec) error {
				id, err := need(e.Ctx.WorkspaceID, errNoWorkspace)
				if err != nil {
					return err
				}
				_, err = e.Client.WorkspaceRename(ctx, herdr.WorkspaceRenameParams{
					WorkspaceID: id,
					Label:       e.Input,
				})
				return err
			},
		},
		{
			ID:      "herdr:workspace.close",
			Binding: "close_workspace",
			Title:   "close workspace",
			Type:    groupHerdr,
			Run: func(ctx context.Context, e palette.Exec) error {
				id, err := need(e.Ctx.WorkspaceID, errNoWorkspace)
				if err != nil {
					return err
				}
				_, err = e.Client.WorkspaceClose(ctx, herdr.WorkspaceCloseParams{WorkspaceID: id})
				return err
			},
		},
		{
			ID:      "herdr:worktree.new",
			Binding: "new_worktree",
			Title:   "new worktree workspace",
			Type:    groupHerdr,
			Input:   &palette.Input{Label: "New branch"},
			Run: func(ctx context.Context, e palette.Exec) error {
				_, err := e.Client.WorktreeCreate(ctx, herdr.WorktreeCreateParams{
					Branch: &e.Input,
					Cwd:    e.Ctx.WorkspaceCwd,
					Focus:  new(true),
				})
				return err
			},
		},
		{
			ID:      "herdr:worktree.open",
			Binding: "open_worktree",
			Title:   "open worktree workspace",
			Type:    groupHerdr,
			Choices: &palette.Choices{
				Label: "Worktree to open",
				Empty: "every worktree of this repository is open already",
				List:  worktreesToOpen,
			},
			// The path identifies a worktree whether or not it is on a branch,
			// which a detached one is not.
			Run: func(ctx context.Context, e palette.Exec) error {
				_, err := e.Client.WorktreeOpen(ctx, herdr.WorktreeOpenParams{
					Path:  &e.Chosen,
					Cwd:   e.Ctx.WorkspaceCwd,
					Focus: new(true),
				})
				return err
			},
		},
		{
			ID:      "herdr:worktree.remove",
			Binding: "remove_worktree",
			Title:   "remove worktree workspace",
			Type:    groupHerdr,
			Choices: &palette.Choices{
				Label: "Worktree to remove",
				Empty: "no worktree of this repository is open in a workspace",
				List:  worktreesToRemove,
			},
			// Force is left unset, so herdr refuses a checkout with work in it
			// and the reason reaches the popup. Picking the worktree from the
			// list is the step herdr's own binding asks a confirmation for.
			Run: func(ctx context.Context, e palette.Exec) error {
				_, err := e.Client.WorktreeRemove(ctx, herdr.WorktreeRemoveParams{WorkspaceID: e.Chosen})
				return err
			},
		},

		{
			ID:      "herdr:tab.new",
			Binding: "new_tab",
			Title:   "new tab",
			Type:    groupHerdr,
			Run: func(ctx context.Context, e palette.Exec) error {
				_, err := e.Client.TabCreate(ctx, herdr.TabCreateParams{
					WorkspaceID: e.Ctx.WorkspaceID,
					Cwd:         e.Ctx.FocusedPaneCwd,
					Focus:       new(true),
				})
				return err
			},
		},
		{
			ID:      "herdr:tab.rename",
			Binding: "rename_tab",
			Title:   "rename tab",
			Type:    groupHerdr,
			Input: &palette.Input{
				Label:   "Tab name",
				Initial: func(c *herdr.PluginInvocationContext) string { return herdr.Value(c.TabLabel) },
			},
			Run: func(ctx context.Context, e palette.Exec) error {
				id, err := need(e.Ctx.TabID, errNoTab)
				if err != nil {
					return err
				}
				_, err = e.Client.TabRename(ctx, herdr.TabRenameParams{TabID: id, Label: e.Input})
				return err
			},
		},
		{
			ID:      "herdr:tab.close",
			Binding: "close_tab",
			Title:   "close tab",
			Type:    groupHerdr,
			Run: func(ctx context.Context, e palette.Exec) error {
				id, err := need(e.Ctx.TabID, errNoTab)
				if err != nil {
					return err
				}
				_, err = e.Client.TabClose(ctx, herdr.TabTarget{TabID: id})
				return err
			},
		},
		{
			ID:      "herdr:tab.move.previous",
			Binding: "move_tab_previous",
			Title:   "move tab toward the front",
			Type:    groupHerdr,
			Run:     moveTab(-1),
		},
		{
			ID:      "herdr:tab.move.next",
			Binding: "move_tab_next",
			Title:   "move tab toward the back",
			Type:    groupHerdr,
			Run:     moveTab(1),
		},

		{
			ID:      "herdr:pane.split.right",
			Binding: "split_vertical",
			Title:   "split pane right",
			Type:    groupHerdr,
			Run:     split(herdr.SplitDirectionRight, ""),
		},
		{
			ID:      "herdr:pane.split.down",
			Binding: "split_horizontal",
			Title:   "split pane down",
			Type:    groupHerdr,
			Run:     split(herdr.SplitDirectionDown, ""),
		},
		{
			ID:    "herdr:pane.split.left",
			Title: "split pane left",
			Type:  groupHerdr,
			Run:   split(herdr.SplitDirectionRight, herdr.PaneDirectionLeft),
		},
		{
			ID:    "herdr:pane.split.up",
			Title: "split pane up",
			Type:  groupHerdr,
			Run:   split(herdr.SplitDirectionDown, herdr.PaneDirectionUp),
		},
		{
			ID:      "herdr:pane.zoom",
			Binding: "zoom",
			Title:   "toggle pane zoom",
			Type:    groupHerdr,
			Run: func(ctx context.Context, e palette.Exec) error {
				_, err := e.Client.PaneZoom(ctx, herdr.PaneZoomParams{
					PaneID: e.Ctx.FocusedPaneID,
					Mode:   herdr.PaneZoomModeToggle,
				})
				return err
			},
		},
		{
			ID:      "herdr:pane.rename",
			Binding: "rename_pane",
			Title:   "rename pane",
			Type:    groupHerdr,
			Input:   &palette.Input{Label: "Pane name"},
			Run: func(ctx context.Context, e palette.Exec) error {
				id, err := need(e.Ctx.FocusedPaneID, errNoPane)
				if err != nil {
					return err
				}
				_, err = e.Client.PaneRename(ctx, herdr.PaneRenameParams{PaneID: id, Label: &e.Input})
				return err
			},
		},
		{
			ID:      "herdr:pane.close",
			Binding: "close_pane",
			Title:   "close pane",
			Type:    groupHerdr,
			Run: func(ctx context.Context, e palette.Exec) error {
				id, err := need(e.Ctx.FocusedPaneID, errNoPane)
				if err != nil {
					return err
				}
				_, err = e.Client.PaneClose(ctx, herdr.PaneTarget{PaneID: id})
				return err
			},
		},
		{
			ID:      "herdr:pane.edit_scrollback",
			Binding: "edit_scrollback",
			Title:   "edit pane scrollback",
			Type:    groupHerdr,
			Run: func(ctx context.Context, e palette.Exec) error {
				id, err := need(e.Ctx.FocusedPaneID, errNoPane)
				if err != nil {
					return err
				}
				_, err = e.Client.PaneEditScrollback(ctx, herdr.PaneTarget{PaneID: id})
				return err
			},
		},
		{
			ID:      "herdr:pane.focus.left",
			Binding: "focus_pane_left",
			Title:   "focus pane left",
			Type:    groupHerdr,
			Run:     focus(herdr.PaneDirectionLeft),
		},
		{
			ID:      "herdr:pane.focus.down",
			Binding: "focus_pane_down",
			Title:   "focus pane down",
			Type:    groupHerdr,
			Run:     focus(herdr.PaneDirectionDown),
		},
		{
			ID:      "herdr:pane.focus.up",
			Binding: "focus_pane_up",
			Title:   "focus pane up",
			Type:    groupHerdr,
			Run:     focus(herdr.PaneDirectionUp),
		},
		{
			ID:      "herdr:pane.focus.right",
			Binding: "focus_pane_right",
			Title:   "focus pane right",
			Type:    groupHerdr,
			Run:     focus(herdr.PaneDirectionRight),
		},
		{
			ID:      "herdr:pane.cycle.next",
			Binding: "cycle_pane_next",
			Title:   "focus next pane",
			Type:    groupHerdr,
			Run:     cycle(1),
		},
		{
			ID:      "herdr:pane.cycle.previous",
			Binding: "cycle_pane_previous",
			Title:   "focus previous pane",
			Type:    groupHerdr,
			Run:     cycle(-1),
		},
		{
			ID:      "herdr:pane.resize.left",
			Binding: "resize_pane_left",
			Title:   "resize pane left",
			Type:    groupHerdr,
			Run:     resize(herdr.PaneDirectionLeft),
		},
		{
			ID:      "herdr:pane.resize.down",
			Binding: "resize_pane_down",
			Title:   "resize pane down",
			Type:    groupHerdr,
			Run:     resize(herdr.PaneDirectionDown),
		},
		{
			ID:      "herdr:pane.resize.up",
			Binding: "resize_pane_up",
			Title:   "resize pane up",
			Type:    groupHerdr,
			Run:     resize(herdr.PaneDirectionUp),
		},
		{
			ID:      "herdr:pane.resize.right",
			Binding: "resize_pane_right",
			Title:   "resize pane right",
			Type:    groupHerdr,
			Run:     resize(herdr.PaneDirectionRight),
		},
		{
			ID:    "herdr:pane.move",
			Title: "move pane to another tab",
			Type:  groupHerdr,
			Choices: &palette.Choices{
				Label: "Tab to move the pane to",
				Empty: "no other tab is open",
				List:  otherTabs,
			},
			Run: func(ctx context.Context, e palette.Exec) error {
				id, err := need(e.Ctx.FocusedPaneID, errNoPane)
				if err != nil {
					return err
				}
				_, err = e.Client.PaneMove(ctx, herdr.PaneMoveParams{
					PaneID: id,
					Destination: herdr.PaneMoveDestinationTab{
						TabID: e.Chosen,
						Split: herdr.SplitDirectionRight,
					},
					Focus: new(true),
				})
				return err
			},
		},

		{
			ID:    "herdr:layout.save",
			Title: "save the tab's layout",
			Type:  groupHerdr,
			Input: &palette.Input{Label: "Layout name"},
			Run: func(ctx context.Context, e palette.Exec) error {
				id, err := need(e.Ctx.TabID, errNoTab)
				if err != nil {
					return err
				}
				exported, err := e.Client.LayoutExport(ctx, herdr.LayoutExportParams{TabID: &id})
				if err != nil {
					return err
				}
				return layout.Save(e.Env, e.Input, exported.Layout.Root)
			},
		},
		{
			ID:    "herdr:layout.apply",
			Title: "open a saved layout in a new tab",
			Type:  groupHerdr,
			Choices: &palette.Choices{
				Label: "Layout to open",
				Empty: "no layout has been saved yet",
				List:  savedLayouts,
			},
			// A new tab rather than this one: the arrangement opens panes of
			// its own, and the panes already in a tab are somebody's work.
			Run: func(ctx context.Context, e palette.Exec) error {
				root, err := layout.Root(e.Env, e.Chosen)
				if err != nil {
					return err
				}
				_, err = e.Client.LayoutApply(ctx, herdr.LayoutApplyParams{
					Root:        root,
					WorkspaceID: e.Ctx.WorkspaceID,
					TabLabel:    &e.Chosen,
					Focus:       new(true),
				})
				return err
			},
		},
		{
			ID:    "herdr:layout.forget",
			Title: "forget a saved layout",
			Type:  groupHerdr,
			Choices: &palette.Choices{
				Label: "Layout to forget",
				Empty: "no layout has been saved yet",
				List:  savedLayouts,
			},
			Run: func(_ context.Context, e palette.Exec) error {
				return layout.Remove(e.Env, e.Chosen)
			},
		},

		{
			ID:    "herdr:agent.start",
			Title: "start an agent in the focused pane",
			Type:  groupHerdr,
			Choices: &palette.Choices{
				Label: "Agent to start",
				Empty: "herdr keeps no agent manifest to start from",
				List:  agentKinds,
			},
			// herdr answers as soon as the command is running, with the agent
			// still pending detection, so the popup is not held open for it.
			// An agent whose command is not installed says so in the pane.
			Run: func(ctx context.Context, e palette.Exec) error {
				id, err := need(e.Ctx.FocusedPaneID, errNoPane)
				if err != nil {
					return err
				}
				_, err = e.Client.AgentStart(ctx, herdr.AgentStartParams{
					Kind:   e.Chosen,
					Name:   e.Chosen,
					PaneID: id,
				})
				return err
			},
		},
		{
			ID:    "herdr:agent.rename",
			Title: "rename the focused agent",
			Type:  groupHerdr,
			Input: &palette.Input{
				Label:   "Agent name",
				Initial: func(c *herdr.PluginInvocationContext) string { return herdr.Value(c.FocusedPaneAgent) },
			},
			Run: func(ctx context.Context, e palette.Exec) error {
				if e.Ctx.FocusedPaneAgent == nil {
					return errNoAgent
				}
				id, err := need(e.Ctx.FocusedPaneID, errNoPane)
				if err != nil {
					return err
				}
				_, err = e.Client.AgentRename(ctx, herdr.AgentRenameParams{
					Target: id,
					Name:   &e.Input,
				})
				return err
			},
		},
		{
			ID:    "herdr:agent.prompt.any",
			Title: "prompt an agent",
			Type:  groupHerdr,
			Choices: &palette.Choices{
				Label: "Agent to prompt",
				Empty: "no pane in the session runs an agent",
				List:  sessionAgents,
			},
			Input: &palette.Input{Label: "Prompt"},
			Run: func(ctx context.Context, e palette.Exec) error {
				_, err := e.Client.AgentPrompt(ctx, herdr.AgentPromptParams{
					Target: e.Chosen,
					Text:   e.Input,
				})
				return err
			},
		},
		{
			ID:    "herdr:agent.prompt",
			Title: "prompt the focused agent",
			Type:  groupHerdr,
			Input: &palette.Input{Label: "Prompt"},
			Run: func(ctx context.Context, e palette.Exec) error {
				if e.Ctx.FocusedPaneAgent == nil {
					return errNoAgent
				}
				id, err := need(e.Ctx.FocusedPaneID, errNoPane)
				if err != nil {
					return err
				}
				_, err = e.Client.AgentPrompt(ctx, herdr.AgentPromptParams{
					Target: id,
					Text:   e.Input,
				})
				return err
			},
		},

		{
			ID:    "herdr:plugin.manage",
			Title: "manage plugins",
			Type:  groupHerdr,
			// The list says what each plugin is and turning one over leaves it
			// up: what this command is for is the state of all of them, and
			// more than one is usually changed at a time.
			Choices: &palette.Choices{
				Label: "Plugin to turn on or off",
				Empty: "no other plugin is installed",
				Stays: true,
				List:  installedPlugins,
			},
			Run: togglePlugin,
		},

		{
			ID:    "herdr:config.edit",
			Title: "edit herdr config",
			Type:  groupHerdr,
			// herdr reads config.toml and offers nothing over the API to show
			// or change it, so the palette opens the same file it reads the
			// keys and the configured commands from.
			Run: func(ctx context.Context, e palette.Exec) error {
				return palette.EditFile(ctx, e, "herdr config", keys.ConfigPath())
			},
		},
		{
			ID:      "herdr:server.reload_config",
			Binding: "reload_config",
			Title:   "reload config",
			Type:    groupHerdr,
			Run: func(ctx context.Context, e palette.Exec) error {
				_, err := e.Client.ServerReloadConfig(ctx)
				return err
			},
		},
	}
}

// split opens a pane next to the focused one. herdr splits right and down
// only, so a pane to the left or above is that same split followed by swapping
// the new pane with the neighbour in the direction asked for; swapWith is
// empty for the two directions herdr splits in directly.
func split(direction herdr.SplitDirection, swapWith herdr.PaneDirection) func(context.Context, palette.Exec) error {
	return func(ctx context.Context, e palette.Exec) error {
		pane, err := e.Client.PaneSplit(ctx, herdr.PaneSplitParams{
			Direction:    direction,
			TargetPaneID: e.Ctx.FocusedPaneID,
			WorkspaceID:  e.Ctx.WorkspaceID,
			Focus:        new(true),
		})
		if err != nil || swapWith == "" {
			return err
		}
		_, err = e.Client.PaneSwap(ctx, herdr.PaneSwapParams{
			PaneID:    &pane.Pane.PaneID,
			Direction: &swapWith,
		})
		return err
	}
}

// resize moves the border the focused pane shares with its neighbour in the
// direction asked for. The amount is left to herdr, which is the step its own
// resize keys take.
func resize(direction herdr.PaneDirection) func(context.Context, palette.Exec) error {
	return func(ctx context.Context, e palette.Exec) error {
		_, err := e.Client.PaneResize(ctx, herdr.PaneResizeParams{
			Direction: direction,
			PaneID:    e.Ctx.FocusedPaneID,
		})
		return err
	}
}

// cycle focuses the pane after or before the focused one among the panes of
// its tab, wrapping round at the ends. herdr's own cycle keys have no API
// behind them, so the order is the one the snapshot lists the tab's panes in.
func cycle(by int) func(context.Context, palette.Exec) error {
	return func(ctx context.Context, e palette.Exec) error {
		id, err := need(e.Ctx.FocusedPaneID, errNoPane)
		if err != nil {
			return err
		}
		snapshot, err := e.Client.SessionSnapshot(ctx)
		if err != nil {
			return err
		}

		siblings, at := tabPanes(snapshot.Snapshot, id)
		if at < 0 {
			return errNoPane
		}
		if len(siblings) < 2 {
			return nil
		}
		next := siblings[((at+by)%len(siblings)+len(siblings))%len(siblings)]
		_, err = e.Client.PaneFocus(ctx, herdr.PaneTarget{PaneID: next})
		return err
	}
}

// tabPanes is the panes of the tab the pane is in, in the order the snapshot
// lists them, and where the pane sits among them.
func tabPanes(snapshot herdr.SessionSnapshot, paneID string) ([]string, int) {
	tab := ""
	for _, pane := range snapshot.Panes {
		if pane.PaneID == paneID {
			tab = pane.TabID
		}
	}
	if tab == "" {
		return nil, -1
	}

	var panes []string
	at := -1
	for _, pane := range snapshot.Panes {
		if pane.TabID != tab {
			continue
		}
		if pane.PaneID == paneID {
			at = len(panes)
		}
		panes = append(panes, pane.PaneID)
	}
	return panes, at
}

// moveTab moves the focused tab one place among the tabs of its workspace.
//
// herdr reads an insert index as a position in the list as it stands and puts
// the tab in front of whatever is there, so a place back is one index less and
// a place on is two more — the tab itself still occupies the index between.
// The index may be the length, which is the end; past it herdr refuses, so a
// tab already at the end it is moved toward stays where it is.
func moveTab(by int) func(context.Context, palette.Exec) error {
	return func(ctx context.Context, e palette.Exec) error {
		id, err := need(e.Ctx.TabID, errNoTab)
		if err != nil {
			return err
		}
		snapshot, err := e.Client.SessionSnapshot(ctx)
		if err != nil {
			return err
		}

		count, at := workspaceTabs(snapshot.Snapshot, id)
		if at < 0 {
			return errNoTab
		}
		insert := at + by
		if by > 0 {
			insert++
		}
		if insert < 0 || insert > count {
			return nil
		}
		_, err = e.Client.TabMove(ctx, herdr.TabMoveParams{TabID: id, InsertIndex: uint64(insert)})
		return err
	}
}

// workspaceTabs is how many tabs the tab's workspace holds and where the tab
// sits among them. The snapshot lists them in the order they are shown, which
// is what an insert index counts: a tab's number stays with it when it moves
// and is not its position.
func workspaceTabs(snapshot herdr.SessionSnapshot, tabID string) (int, int) {
	workspace := ""
	for _, tab := range snapshot.Tabs {
		if tab.TabID == tabID {
			workspace = tab.WorkspaceID
		}
	}
	if workspace == "" {
		return 0, -1
	}

	count, at := 0, -1
	for _, tab := range snapshot.Tabs {
		if tab.WorkspaceID != workspace {
			continue
		}
		if tab.TabID == tabID {
			at = count
		}
		count++
	}
	return count, at
}

// otherTabs is every tab but the one the palette was opened in, as targets to
// move the focused pane to.
func otherTabs(ctx context.Context, e palette.Exec) ([]palette.Choice, error) {
	snapshot, err := e.Client.SessionSnapshot(ctx)
	if err != nil {
		return nil, err
	}

	workspaces := make(map[string]string, len(snapshot.Snapshot.Workspaces))
	for _, workspace := range snapshot.Snapshot.Workspaces {
		workspaces[workspace.WorkspaceID] = workspace.Label
	}

	choices := make([]palette.Choice, 0, len(snapshot.Snapshot.Tabs))
	for _, tab := range snapshot.Snapshot.Tabs {
		if tab.TabID == herdr.Value(e.Ctx.TabID) {
			continue
		}
		choices = append(choices, palette.Choice{
			Value:  tab.TabID,
			Title:  palette.Label(tab.Label, "tab", tab.Number),
			Detail: workspaces[tab.WorkspaceID],
		})
	}
	return choices, nil
}

// savedLayouts is every arrangement the palette has been asked to keep. They
// are the plugin's own, so herdr is not asked for them.
func savedLayouts(_ context.Context, e palette.Exec) ([]palette.Choice, error) {
	saved := layout.List(e.Env)
	choices := make([]palette.Choice, 0, len(saved))
	for _, one := range saved {
		choices = append(choices, palette.Choice{Value: one.Name, Title: one.Name})
	}
	return choices, nil
}

// Words for what a plugin is, which the row shows and the query matches.
const (
	pluginEnabled  = "enabled"
	pluginDisabled = "disabled"
)

// installedPlugins is every plugin herdr has installed, saying which are on.
// The palette leaves itself out: turning it off would take away the popup the
// row is being run from, with no row left to turn it back on, and the list of
// plugin actions leaves its own out for the same reason.
func installedPlugins(ctx context.Context, e palette.Exec) ([]palette.Choice, error) {
	installed, err := e.Client.PluginList(ctx, herdr.PluginListParams{})
	if err != nil {
		return nil, err
	}

	choices := make([]palette.Choice, 0, len(installed.Plugins))
	for _, plugin := range installed.Plugins {
		if plugin.PluginID == e.Env.PluginID {
			continue
		}
		choices = append(choices, palette.Choice{
			Value:  plugin.PluginID,
			Title:  plugin.Name,
			Detail: pluginState(plugin.Enabled),
			// The row shows the name, and the id is how a plugin is spelt
			// everywhere else, so typing it finds the row too.
			Search: plugin.PluginID,
		})
	}
	return choices, nil
}

// togglePlugin turns the picked plugin over. What it is now is read again
// rather than taken from the row: the list is a screen, and herdr is where the
// state lives.
func togglePlugin(ctx context.Context, e palette.Exec) error {
	installed, err := e.Client.PluginList(ctx, herdr.PluginListParams{PluginID: &e.Chosen})
	if err != nil {
		return err
	}

	// The id is asked for and looked for: what comes back is a list either
	// way, and the plugin to turn over is the one it names.
	for _, plugin := range installed.Plugins {
		if plugin.PluginID != e.Chosen {
			continue
		}
		params := herdr.PluginSetEnabledParams{PluginID: e.Chosen}
		if plugin.Enabled {
			_, err = e.Client.PluginDisable(ctx, params)
		} else {
			_, err = e.Client.PluginEnable(ctx, params)
		}
		return err
	}
	return fmt.Errorf("no plugin with id %q is installed", e.Chosen)
}

func pluginState(enabled bool) string {
	if enabled {
		return pluginEnabled
	}
	return pluginDisabled
}

// sessionAgents is every pane in the session running an agent, as targets to
// prompt. A pane is addressed by its id, the way the rows that go to one are,
// and reads as the name the agent goes by.
func sessionAgents(ctx context.Context, e palette.Exec) ([]palette.Choice, error) {
	snapshot, err := e.Client.SessionSnapshot(ctx)
	if err != nil {
		return nil, err
	}

	workspaces := make(map[string]string, len(snapshot.Snapshot.Workspaces))
	for _, workspace := range snapshot.Snapshot.Workspaces {
		workspaces[workspace.WorkspaceID] = workspace.Label
	}

	choices := make([]palette.Choice, 0, len(snapshot.Snapshot.Agents))
	for _, agent := range snapshot.Snapshot.Agents {
		name := herdr.Value(agent.Name)
		if name == "" {
			name = herdr.Value(agent.Agent)
		}
		choices = append(choices, palette.Choice{
			Value:  agent.PaneID,
			Title:  name,
			Detail: string(agent.AgentStatus),
			Search: workspaces[agent.WorkspaceID] + " " + herdr.Value(agent.Cwd),
		})
	}
	return choices, nil
}

// agentKinds is every agent herdr keeps a manifest for. It is not what is
// installed: herdr runs the command the manifest names, and a pane where that
// command is missing says so itself, which is the same set herdr starts from.
func agentKinds(ctx context.Context, e palette.Exec) ([]palette.Choice, error) {
	manifests, err := e.Client.ServerAgentManifests(ctx)
	if err != nil {
		return nil, err
	}

	choices := make([]palette.Choice, 0, len(manifests.Manifests))
	for _, manifest := range manifests.Manifests {
		choices = append(choices, palette.Choice{Value: manifest.Agent, Title: manifest.Agent})
	}
	return choices, nil
}

// worktreesToOpen is every worktree of the repository the palette was opened
// in that no workspace is on yet.
func worktreesToOpen(ctx context.Context, e palette.Exec) ([]palette.Choice, error) {
	return worktrees(ctx, e, func(tree herdr.WorktreeInfo) (palette.Choice, bool) {
		if tree.OpenWorkspaceID != nil {
			return palette.Choice{}, false
		}
		return palette.Choice{
			Value: tree.Path,
			Title: worktreeName(tree),
			// The row is the branch, so the checkout it is in is searchable
			// and shown when that is what the query matched.
			Search: tree.Path,
		}, true
	})
}

// worktreesToRemove is the worktrees a workspace is open on. The repository's
// own checkout is not one of them: removing it is not what the command means,
// and herdr addresses a removal by the workspace the worktree is open in.
func worktreesToRemove(ctx context.Context, e palette.Exec) ([]palette.Choice, error) {
	return worktrees(ctx, e, func(tree herdr.WorktreeInfo) (palette.Choice, bool) {
		if tree.OpenWorkspaceID == nil || !tree.IsLinkedWorktree {
			return palette.Choice{}, false
		}
		return palette.Choice{
			Value:  *tree.OpenWorkspaceID,
			Title:  worktreeName(tree),
			Search: tree.Path,
		}, true
	})
}

// worktrees lists the repository the palette was opened in, by the same cwd
// the commands that create and open a worktree resolve it from.
func worktrees(ctx context.Context, e palette.Exec, pick func(herdr.WorktreeInfo) (palette.Choice, bool)) ([]palette.Choice, error) {
	list, err := e.Client.WorktreeList(ctx, herdr.WorktreeListParams{Cwd: e.Ctx.WorkspaceCwd})
	if err != nil {
		return nil, err
	}

	choices := make([]palette.Choice, 0, len(list.Worktrees))
	for _, tree := range list.Worktrees {
		if choice, ok := pick(tree); ok {
			choices = append(choices, choice)
		}
	}
	return choices, nil
}

// worktreeName is the branch the worktree is on, or the name herdr gave it
// when it is on none.
func worktreeName(tree herdr.WorktreeInfo) string {
	if branch := herdr.Value(tree.Branch); branch != "" {
		return branch
	}
	return tree.Label
}

func focus(direction herdr.PaneDirection) func(context.Context, palette.Exec) error {
	return func(ctx context.Context, e palette.Exec) error {
		_, err := e.Client.PaneFocusDirection(ctx, herdr.PaneFocusDirectionParams{
			Direction: direction,
			PaneID:    e.Ctx.FocusedPaneID,
		})
		return err
	}
}

func need(value *string, absent error) (string, error) {
	if value == nil || *value == "" {
		return "", absent
	}
	return *value, nil
}
