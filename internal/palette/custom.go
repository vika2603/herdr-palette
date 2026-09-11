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
)

const (
	// RunEntrypoint is the manifest pane that runs a configured command, and
	// RunEnv is how the command line reaches it.
	RunEntrypoint = "run"
	RunEnv        = "HERDR_PALETTE_COMMAND"
)

// customEntries turns the [[keys.command]] entries into rows.
//
// herdr runs these from a key and offers no method to run one by name, so each
// type is reproduced over the API: a shell command is started detached, and a
// pane or popup command runs in a plugin pane of this plugin, placed the way
// herdr places its own.
func customEntries(own string, commands []keys.Custom) []Entry {
	entries := make([]Entry, 0, len(commands))
	for _, command := range commands {
		entries = append(entries, Entry{
			ID:    "config:" + customID(command),
			Title: customTitle(command),
			Type:  TypeCustom,
			Key:   command.Key,
			Run:   runCustom(own, command),
		})
	}
	return entries
}

// customID keys the recent order. The key is what identifies a command in the
// configuration; a command bound to nothing falls back to its command line.
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

func runCustom(own string, command keys.Custom) func(context.Context, Exec) error {
	return func(ctx context.Context, e Exec) error {
		if command.Type == keys.TypeShell {
			return startDetached(command.Command)
		}

		where := placement(command.Type)
		params := herdr.PluginPaneOpenParams{
			PluginID:   own,
			Entrypoint: RunEntrypoint,
			Placement:  new(where),
			Focus:      new(true),
			Cwd:        e.Ctx.FocusedPaneCwd,
			Env:        map[string]string{RunEnv: command.Command},
		}
		// A zoomed pane, like a split, is placed against an existing pane and
		// takes its id; a popup always covers the active pane, and herdr
		// rejects a target alongside it.
		if where != herdr.PluginPanePlacementPopup {
			params.TargetPaneID = e.Ctx.FocusedPaneID
		}
		if size, ok := popupSize(command.Width); ok {
			params.Width = &size
		}
		if size, ok := popupSize(command.Height); ok {
			params.Height = &size
		}

		if _, err := e.Client.PluginPaneOpen(ctx, params); err != nil {
			return fmt.Errorf("%s: %w", customTitle(command), err)
		}
		return nil
	}
}

// placement maps herdr's command types onto plugin pane placements: its popup
// type is a session-modal terminal, and its pane type a temporary pane that
// takes over the layout, which is herdr's zoomed placement rather than a
// split beside the focused pane.
func placement(commandType string) herdr.PluginPanePlacement {
	if commandType == keys.TypePopup {
		return herdr.PluginPanePlacementPopup
	}
	return herdr.PluginPanePlacementZoomed
}

// Shell is what a configured command line runs under, matching herdr, which
// hands the string to a shell rather than splitting it itself.
func Shell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}

// popupSize carries a configured size over to the API, reporting whether one
// was configured at all. The two types hold the same two fields; the manifest
// package is what decoded the cell count or percentage herdr accepts.
func popupSize(size manifest.PopupSize) (herdr.PopupSize, bool) {
	if size == (manifest.PopupSize{}) {
		return herdr.PopupSize{}, false
	}
	return herdr.PopupSize(size), true
}

// startDetached runs a shell command in its own session, so it outlives the
// popup the palette closes on its way out. Its output goes nowhere: herdr's
// own shell type runs detached in the background too.
func startDetached(command string) error {
	cmd := exec.Command(Shell(), "-c", command)
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
