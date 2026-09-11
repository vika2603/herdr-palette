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

	"github.com/vika2603/herdr-client/herdr"

	"github.com/vika2603/herdr-palette/internal/palette"
)

// Groups shown in the last column.
const (
	groupWorkspace = "Workspace"
	groupTab       = "Tab"
	groupPane      = "Pane"
	groupAgent     = "Agent"
	groupHerdr     = "Herdr"
)

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
	entries := []palette.Entry{
		{
			ID:      "herdr:workspace.new",
			Binding: "new_workspace",
			Title:   "New workspace",
			Type:    groupWorkspace,
			Run: func(ctx context.Context, e palette.Exec) error {
				// source_workspace_id lets the new workspace follow the
				// focused pane's cwd policy instead of starting at $HOME.
				_, err := e.Client.WorkspaceCreate(ctx, herdr.WorkspaceCreateParams{
					Focus:             herdr.Ptr(true),
					SourceWorkspaceID: e.Ctx.WorkspaceID,
				})
				return err
			},
		},
		{
			ID:      "herdr:workspace.rename",
			Binding: "rename_workspace",
			Title:   "Rename workspace",
			Type:    groupWorkspace,
			Input: &palette.Input{
				Label:   "Workspace name",
				Initial: func(c *herdr.PluginInvocationContext) string { return deref(c.WorkspaceLabel) },
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
			Title:   "Close workspace",
			Type:    groupWorkspace,
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
			Title:   "New worktree workspace",
			Type:    groupWorkspace,
			Input:   &palette.Input{Label: "New branch"},
			Run: func(ctx context.Context, e palette.Exec) error {
				_, err := e.Client.WorktreeCreate(ctx, herdr.WorktreeCreateParams{
					Branch: &e.Input,
					Cwd:    e.Ctx.WorkspaceCwd,
					Focus:  herdr.Ptr(true),
				})
				return err
			},
		},
		{
			ID:      "herdr:worktree.open",
			Binding: "open_worktree",
			Title:   "Open worktree workspace",
			Type:    groupWorkspace,
			Input:   &palette.Input{Label: "Existing branch"},
			Run: func(ctx context.Context, e palette.Exec) error {
				_, err := e.Client.WorktreeOpen(ctx, herdr.WorktreeOpenParams{
					Branch: &e.Input,
					Cwd:    e.Ctx.WorkspaceCwd,
					Focus:  herdr.Ptr(true),
				})
				return err
			},
		},

		{
			ID:      "herdr:tab.new",
			Binding: "new_tab",
			Title:   "New tab",
			Type:    groupTab,
			Run: func(ctx context.Context, e palette.Exec) error {
				_, err := e.Client.TabCreate(ctx, herdr.TabCreateParams{
					WorkspaceID: e.Ctx.WorkspaceID,
					Cwd:         e.Ctx.FocusedPaneCwd,
					Focus:       herdr.Ptr(true),
				})
				return err
			},
		},
		{
			ID:      "herdr:tab.rename",
			Binding: "rename_tab",
			Title:   "Rename tab",
			Type:    groupTab,
			Input: &palette.Input{
				Label:   "Tab name",
				Initial: func(c *herdr.PluginInvocationContext) string { return deref(c.TabLabel) },
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
			Title:   "Close tab",
			Type:    groupTab,
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
			ID:      "herdr:pane.split.right",
			Binding: "split_vertical",
			Title:   "Split pane right",
			Type:    groupPane,
			Run:     split(herdr.SplitDirectionRight),
		},
		{
			ID:      "herdr:pane.split.down",
			Binding: "split_horizontal",
			Title:   "Split pane down",
			Type:    groupPane,
			Run:     split(herdr.SplitDirectionDown),
		},
		{
			ID:      "herdr:pane.zoom",
			Binding: "zoom",
			Title:   "Toggle pane zoom",
			Type:    groupPane,
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
			Title:   "Rename pane",
			Type:    groupPane,
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
			Title:   "Close pane",
			Type:    groupPane,
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
			Title:   "Edit pane scrollback",
			Type:    groupPane,
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
			Title:   "Focus pane left",
			Type:    groupPane,
			Run:     focus(herdr.PaneDirectionLeft),
		},
		{
			ID:      "herdr:pane.focus.down",
			Binding: "focus_pane_down",
			Title:   "Focus pane down",
			Type:    groupPane,
			Run:     focus(herdr.PaneDirectionDown),
		},
		{
			ID:      "herdr:pane.focus.up",
			Binding: "focus_pane_up",
			Title:   "Focus pane up",
			Type:    groupPane,
			Run:     focus(herdr.PaneDirectionUp),
		},
		{
			ID:      "herdr:pane.focus.right",
			Binding: "focus_pane_right",
			Title:   "Focus pane right",
			Type:    groupPane,
			Run:     focus(herdr.PaneDirectionRight),
		},

		{
			ID:    "herdr:agent.prompt",
			Title: "Prompt the focused agent",
			Type:  groupAgent,
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
			ID:      "herdr:server.reload_config",
			Binding: "reload_config",
			Title:   "Reload herdr config",
			Type:    groupHerdr,
			Run: func(ctx context.Context, e palette.Exec) error {
				_, err := e.Client.ServerReloadConfig(ctx)
				return err
			},
		},
	}
	return entries
}

func split(direction herdr.SplitDirection) func(context.Context, palette.Exec) error {
	return func(ctx context.Context, e palette.Exec) error {
		_, err := e.Client.PaneSplit(ctx, herdr.PaneSplitParams{
			Direction:    direction,
			TargetPaneID: e.Ctx.FocusedPaneID,
			WorkspaceID:  e.Ctx.WorkspaceID,
			Focus:        herdr.Ptr(true),
		})
		return err
	}
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

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
