# Design

## Two entrypoints, one binary

The manifest declares an action, `open`, and a popup pane, `palette`. Both run
`bin/palette`; `plugin.Env` tells the process which entrypoint started it.

The action does one thing: it calls `plugin.pane.open` for the `palette`
entrypoint. The popup pane runs the TUI.

## Passing the invocation context to the popup

Commands act on what was focused when the key was pressed. The action
entrypoint receives that as `HERDR_PLUGIN_CONTEXT_JSON`, but the popup is a
separate process whose own environment describes the popup pane instead — its
`HERDR_PANE_ID` is the popup, and the session's focused pane becomes the popup
as long as it is open.

So the action hands the context over explicitly, in the `env` map of
`plugin.pane.open`, as `HERDR_PALETTE_CONTEXT`. The popup reads it from there
and falls back to its own entrypoint context, which is what happens when the
pane is opened directly (`herdr plugin pane open`).

## Why herdr's commands are a hand-written list

`plugin.action.list` covers plugin actions completely: id, title, description,
contexts. herdr's built-in actions have no counterpart. They exist as config
keys under `[keys]`, and the API exposes no method that runs one by name.
`command.invoke` takes a `command_id` that the schema documents as issued
through the client-shell projection, which a plugin process does not receive;
becoming a client shell would mean taking over pane presentation from the
attached client.

`internal/catalog` therefore maps each command to the socket API method with
the same effect, and `internal/palette` merges that list with the plugin
actions. The cost is that a new built-in action does not appear on its own. The
API method behind each entry is stable, so the list only needs revisiting when
herdr adds commands worth offering.

## Running the command

Most commands run while the popup is still open, and the TUI quits on success,
which closes the popup. A command that fails leaves the popup open with the
reason on the help line, so the keystroke is not lost silently.

Commands that open a popup of their own cannot work that way: herdr allows one
popup at a time and answers the second with `ui_busy`. That covers the
configured `popup` commands and every plugin action, since what another
plugin's action does is its own business.

Those are handed over instead. The palette writes the entry, its input and the
invocation context to `pending.json`, invokes its own `exec` action, and quits.
herdr runs an action entrypoint outside the popup, so the `exec` process
survives the popup closing; it waits for the popup process to exit — the pid is
in the file — and then runs the entry. A failure there has no popup left to
show it, so it is reported with `notification.show`.

## Colours

herdr does not publish its theme: the API has no method for it, and
config.toml carries only the theme's name plus the tokens the user overrode.
`internal/theme` therefore resolves each colour from three places, each
overriding the one before it — built-in defaults, herdr's `[theme.custom]`
tokens, and the plugin's own config.toml.

The defaults are ANSI indexes, which follow the terminal, except the rule and
the selected row: both need a shade just off the terminal's background, which
the ANSI palette has no index for, so they are adaptive hex values.

## Reading herdr's configuration

The key column and the configured commands both come from files rather than the
API, because herdr exposes neither: `[keys]` and `[[keys.command]]` live in
`config.toml`, and the defaults only in what `herdr --default-config` prints.
`internal/keys` parses both, the defaults from the binary named by
`HERDR_BIN_PATH`.

The prose in the printed `[keys]` section contains lines shaped like
assignments, such as the `type = "popup"` documenting custom commands, so the
parse collects more names than there are actions. Lookups go through a fixed
set of action names, which is what keeps that harmless.

## Matching

`internal/palette.Rank` accepts two shapes, in this order:

1. every word of the query appearing as a substring of the title, in any
   order, scored higher when a word sits at the start of a word;
2. the query read as the initials of the title's words, skipping allowed.

Letters that only appear scattered through a title do not match. A plain
subsequence match makes a query like `spl` reach "Close workspace", which
makes the list unpredictable at the size the palette actually has.

A query that matches no title is retried against the group or plugin name
prepended, scored lower, so typing `machine` finds the Machine Manager's
actions.

## Mouse

The program runs with `tea.WithMouseCellMotion`, which reports clicks and the
wheel. herdr captures the mouse for its own UI but forwards events to a pane
app that asks for them, so the popup receives them.

A click's row is `offset + Y - headerRows`, where `headerRows` is the query
line and the rule under it. A click outside the rendered rows does nothing,
and the value screen takes no mouse input.

## State

`recent.json` in the plugin's state directory holds the ids of the commands
last run, most recent first, capped at 50. It is a convenience: an unreadable
file only means the list opens unordered.
