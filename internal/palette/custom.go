package palette

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

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

// configured is a command from herdr's configuration, bound to a key. An empty
// window runs it detached, with nothing to show and nowhere for its output to
// go.
type configured struct {
	id     string
	title  string
	key    string
	line   string
	script string
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

func entryFor(own string, command configured) Entry {
	return Entry{
		ID:          command.id,
		Title:       command.title,
		Type:        TypeCustom,
		Kind:        KindCustom,
		Key:         command.key,
		Description: command.line,
		Run:         run(own, command),
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
		return command.Description
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

func run(own string, command configured) func(context.Context, Exec) error {
	return func(ctx context.Context, e Exec) error {
		var script *exec.Cmd
		if command.script != "" {
			if e.Ctx == nil || herdr.Value(e.Ctx.FocusedPaneCwd) == "" {
				return fmt.Errorf("%s: the focused pane has no working directory", command.title)
			}
			if !filepath.IsAbs(*e.Ctx.FocusedPaneCwd) {
				return fmt.Errorf("%s: the focused pane's working directory must be absolute", command.title)
			}
			info, err := os.Stat(*e.Ctx.FocusedPaneCwd)
			if err != nil {
				return fmt.Errorf("%s: working directory: %w", command.title, err)
			}
			if !info.IsDir() {
				return fmt.Errorf("%s: working directory is not a directory", command.title)
			}
			// A detached script outlives the action that starts it. Construct
			// it before opening a pane too, so a removed or invalid script
			// fails while the caller can still report the reason.
			script, err = ScriptCommand(context.Background(), command.script)
			if err != nil {
				return err
			}
		}
		active := activeEnv(e.Ctx)
		if command.window == "" {
			if script != nil {
				return startCommandDetached(script, *e.Ctx.FocusedPaneCwd, active)
			}
			return startDetached(command.line, herdr.Value(e.Ctx.FocusedPaneCwd), active)
		}

		// Set both variables so an inherited run-pane environment cannot
		// select a different command type in the new pane.
		env := map[string]string{RunEnv: command.line, ScriptEnv: command.script}
		for name, value := range active {
			env[name] = value
		}
		params := herdr.PluginPaneOpenParams{
			PluginID:   own,
			Entrypoint: RunEntrypoint,
			Placement:  &command.window,
			Focus:      new(true),
			Cwd:        e.Ctx.FocusedPaneCwd,
			Env:        env,
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

		if _, err := e.Client.PluginPaneOpen(ctx, params); err != nil {
			return fmt.Errorf("%s: %w", command.title, err)
		}
		return nil
	}
}

// activeEnv is what herdr sets for a command it runs from a key: the
// workspace, tab and pane focused when the key was pressed, and that pane's
// directory. A configured command line reads them to act where the key was
// pressed, and the palette's own process has none of them — the ids it is
// given describe its popup — so they come from the invocation context. A
// value the context does not carry is left unset rather than set empty.
func activeEnv(c *herdr.PluginInvocationContext) map[string]string {
	env := map[string]string{}
	for name, value := range map[string]*string{
		"HERDR_ACTIVE_WORKSPACE_ID": c.WorkspaceID,
		"HERDR_ACTIVE_TAB_ID":       c.TabID,
		"HERDR_ACTIVE_PANE_ID":      c.FocusedPaneID,
		"HERDR_ACTIVE_PANE_CWD":     c.FocusedPaneCwd,
	} {
		if v := herdr.Value(value); v != "" {
			env[name] = v
		}
	}
	return env
}

// editorWindow is how much of the pane area an editor opens over. A file is
// read down the page, and herdr's default popup is smaller than that.
var editorWindow = manifest.PopupSize{Percent: 90}

// EditFile opens a file in the user's editor, in a popup of the plugin's own,
// which is where a configured popup command runs too. The editor is the one
// the environment herdr passed the plugin names, so it is the one that is set
// there rather than one this plugin decides on.
func EditFile(ctx context.Context, e Exec, title, path string) error {
	if path == "" {
		return fmt.Errorf("%s: there is no file to edit", title)
	}
	return run(e.Env.PluginID, configured{
		title:  title,
		line:   editorLine(runtime.GOOS, os.Getenv, path),
		window: herdr.PluginPanePlacementPopup,
		width:  editorWindow,
		height: editorWindow,
	})(ctx, e)
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
// as a command that opens a window does, with env added to what the plugin
// was started with.
func startDetached(command, dir string, env map[string]string) error {
	cmd := ShellCommand(context.Background(), command)
	return startCommandDetached(cmd, dir, env)
}

func startCommandDetached(cmd *exec.Cmd, dir string, env map[string]string) error {
	cmd.Dir = dir
	cmd.Env = cmd.Environ()
	for name, value := range env {
		cmd.Env = append(cmd.Env, name+"="+value)
	}
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
