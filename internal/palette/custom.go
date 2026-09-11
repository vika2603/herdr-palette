package palette

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/vika2603/herdr-client/herdr"
	"github.com/vika2603/herdr-client/plugin/manifest"

	"github.com/vika2603/herdr-palette/internal/keys"
	"github.com/vika2603/herdr-palette/internal/settings"
)

const (
	// RunEntrypoint is the manifest pane that runs a configured command, and
	// RunEnv is how the command line reaches it.
	RunEntrypoint = "run"
	RunEnv        = "HERDR_PALETTE_COMMAND"
)

// configured is a command from either configuration file: herdr's, where it is
// bound to a key, and the palette's own, where it is not. An empty window runs
// it detached, with nothing to show and nowhere for its output to go.
type configured struct {
	id     string
	title  string
	key    string
	line   string
	window herdr.PluginPanePlacement
	width  manifest.PopupSize
	height manifest.PopupSize
}

// customEntries turns herdr's [[keys.command]] entries into rows.
//
// herdr runs these from a key and offers no method to run one by name, so each
// type is reproduced over the API, placed the way herdr places its own.
func customEntries(own string, commands []keys.Custom) []Entry {
	entries := make([]Entry, 0, len(commands))
	for _, command := range commands {
		entries = append(entries, entryFor(own, configured{
			id:     "config:" + customID(command),
			title:  customTitle(command),
			key:    command.Key,
			line:   command.Command,
			window: herdrWindow(command.Type),
			width:  command.Width,
			height: command.Height,
		}))
	}
	return entries
}

// ownEntries turns the palette's own [[command]] entries into rows.
func ownEntries(own string, commands []settings.Command) []Entry {
	entries := make([]Entry, 0, len(commands))
	for _, command := range commands {
		entries = append(entries, entryFor(own, configured{
			id:     "command:" + command.Title,
			title:  strings.ToLower(command.Title),
			line:   command.Run,
			window: window(command.Window),
			width:  command.Width,
			height: command.Height,
		}))
	}
	return entries
}

func entryFor(own string, command configured) Entry {
	return Entry{
		ID:    command.id,
		Title: command.title,
		Type:  TypeCustom,
		Key:   command.key,
		Run:   run(own, command),
	}
}

// customID keys the recent order. The key is what identifies a command in
// herdr's configuration; a command bound to nothing falls back to its command
// line.
func customID(command keys.Custom) string {
	if command.Key != "" {
		return command.Key
	}
	return command.Command
}

func customTitle(command keys.Custom) string {
	if command.Description != "" {
		return strings.ToLower(command.Description)
	}
	return command.Command
}

// herdrWindow maps herdr's command types onto plugin pane placements: its
// popup type is a session-modal terminal, its pane type a temporary pane that
// takes over the layout, which is herdr's zoomed placement rather than a split
// beside the focused pane, and its shell type no window at all.
func herdrWindow(commandType string) herdr.PluginPanePlacement {
	switch commandType {
	case keys.TypePopup:
		return herdr.PluginPanePlacementPopup
	case keys.TypePane:
		return herdr.PluginPanePlacementZoomed
	}
	return ""
}

// window maps what a [[command]] entry asked for. A tab is the one that can be
// returned to: it is a pane of its own, which the list then offers to go to.
func window(asked string) herdr.PluginPanePlacement {
	switch asked {
	case settings.WindowPopup:
		return herdr.PluginPanePlacementPopup
	case settings.WindowPane:
		return herdr.PluginPanePlacementZoomed
	case settings.WindowTab:
		return herdr.PluginPanePlacementTab
	}
	return ""
}

func run(own string, command configured) func(context.Context, Exec) error {
	return func(ctx context.Context, e Exec) error {
		if command.window == "" {
			return startDetached(command.line, herdr.Value(e.Ctx.FocusedPaneCwd))
		}

		params := herdr.PluginPaneOpenParams{
			PluginID:   own,
			Entrypoint: RunEntrypoint,
			Placement:  &command.window,
			Focus:      new(command.window != herdr.PluginPanePlacementTab),
			Cwd:        e.Ctx.FocusedPaneCwd,
			Env:        map[string]string{RunEnv: command.line},
		}
		// A zoomed pane, like a split, is placed against an existing pane and
		// takes its id; a popup always covers the active pane, and herdr
		// rejects a target alongside it.
		if command.window == herdr.PluginPanePlacementZoomed {
			params.TargetPaneID = e.Ctx.FocusedPaneID
		}
		if size, ok := PopupSize(command.width); ok {
			params.Width = &size
		}
		if size, ok := PopupSize(command.height); ok {
			params.Height = &size
		}

		opened, err := e.Client.PluginPaneOpen(ctx, params)
		if err != nil {
			return fmt.Errorf("%s: %w", command.title, err)
		}
		// A tab is opened to be found again, and every plugin pane carries the
		// manifest's name until it is given the name of what it runs.
		if info, ok := opened.(*herdr.PluginPaneOpenedResponse); ok && command.window == herdr.PluginPanePlacementTab {
			_, _ = e.Client.PaneRename(ctx, herdr.PaneRenameParams{
				PaneID: info.PluginPane.Pane.PaneID,
				Label:  &command.title,
			})
		}
		return nil
	}
}

// Shell is what a configured command line runs under, matching herdr, which
// hands the string to a shell rather than splitting it itself.
func Shell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}

// PopupSize carries a configured size over to the API, reporting whether one
// was configured at all. The two types hold the same two fields; the manifest
// package is what decoded the cell count or percentage herdr accepts.
func PopupSize(size manifest.PopupSize) (herdr.PopupSize, bool) {
	if size == (manifest.PopupSize{}) {
		return herdr.PopupSize{}, false
	}
	return herdr.PopupSize(size), true
}

// startDetached runs a shell command in its own session, so it outlives the
// popup the palette closes on its way out. Its output goes nowhere, which is
// what herdr's own shell type does with it. It runs where the focused pane is,
// as a command that opens a window does.
func startDetached(command, dir string) error {
	cmd := exec.Command(Shell(), "-c", command)
	cmd.Dir = dir
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
