# Design

## Five entrypoints, one binary

The manifest declares two actions, `open` and `exec`, and three panes,
`palette`, `input` and `run`. All five run `bin/palette`; `plugin.Env` tells
the process which entrypoint started it.

`open` does one thing: it calls `plugin.pane.open` for the `palette`
entrypoint, which runs the TUI. `exec` runs a command the popup handed over,
`input` is the field that collects a value a command needs, and `run` is the
pane a configured command runs in. All three are described below.

`cmd/palette` assembles the command list for the two entrypoints that need one,
so the popup and the handover work from the same list; `RunPending` resolves
the handed-over entry by id against it.

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

## What a row says

A row reads `namespace: title`, the way an editor's command list does, with
the key it is bound to at the right edge. The namespace is where the row comes
from — `herdr` for the built-in commands, `command` for the ones configured
under `[[keys.command]]`, the plugin's own name for a plugin action, and the
kind of thing for the rows that go somewhere. It is drawn in the same colour as
the key column, a shade below the title, because it repeats down the list while
the title is what distinguishes the row.

A command's title is lowercase, authored that way rather than lowercased when
drawn. The rows that go to a workspace, tab or pane keep the name that thing
carries: it is a name, not a command.

## Going to what is open

`session.snapshot` is the whole session in one call, so every workspace, tab
and pane becomes a row that focuses it — `workspace.focus`, `tab.focus`,
`pane.focus`. What is already focused is left out, the palette having been
opened from there. A plugin popup is not part of the session's panes, so the
palette's own window never appears in its list.

A pane running an agent shows under `agent:` instead of `pane:`, with the agent
and its status next to the row, coloured by the status. A pane is addressed by
its id rather than through `agent.focus`, which takes a name the snapshot does
not carry.

## Following the session

These rows are rebuilt while the popup is open, so a status is the one the
agent has at that moment. `events.subscribe` carries the changes: an event sets
a single signal, which the model answers with one `session.snapshot`, so a
burst costs one round trip.

`pane.agent_status_changed` is subscribed per pane — the subscription takes a
`pane_id`, and one without is refused with `pane_not_found` — so the panes
running an agent are named when the subscription opens, and the subscription is
opened again whenever a pane appears, exits or closes. The rest are
session-wide: the pane, tab and workspace events that add or rename a row.
`pane.updated` is among them but does not carry a status change, which is why
the per-pane subscription is there at all. Pane output is left out: it changes
constantly and no row shows it.

Every one of these rows starts with "go to", so typing that — or `goto`, which
a subsequence match reaches as well — narrows the list to them. A pane's row
shows the name it was given, or what the program in it reports, and carries the
workspace it sits in and its directory as search text, so a project name finds
the panes inside it and the row says which one it matched.

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

Two entries have no herdr action behind them: splitting left and up. herdr
splits right and down only, so those split in the direction it has and then
`pane.swap` the new pane into place.

## Running the command

Most commands run while the popup is still open, and the TUI quits on success,
which closes the popup. A command that fails leaves the popup open with the
reason on the help line, so the keystroke is not lost silently.

A command that opens a popup of its own cannot work that way: herdr allows one
popup at a time and answers the second with `ui_busy`. That is the signal the
palette acts on — it runs the command and hands it over only when that code
comes back, so nothing has to predict which commands open a popup.

The exception is a plugin action. It runs in the other plugin's process, where
the refusal is raised and never reported back here, so there is nothing to try:
`AlwaysRelay` hands those over directly.

A handover writes the entry, its input and the invocation context to
`pending.json`, invokes the plugin's own `exec` action, and quits. herdr runs
an action entrypoint outside the popup, so the `exec` process survives the
popup closing; it waits for the popup process to exit — the pid is in the file
— and then runs the entry. A failure there has no popup left to show it, so it
is reported with `notification.show`.

## Borders

herdr draws a pane's title on the border and its manifest requires one — a
blank title is refused with `invalid_plugin_pane_title`, and a popup returns no
pane id to rename afterwards. Every pane here therefore carries a zero-width
space, which passes the check and leaves the border empty. What a pane is
showing is inside it: the palette's query line, or the command and the value
the field is collecting.

## Where the popup sits

herdr centres a popup in the pane area and takes only a size: `plugin.pane.open`
has no position, and config.toml has no popup placement. A percentage is of the
pane area — a popup opened from a pane of half the width still measures 60% of
the whole area, which is what makes it the area rather than the pane it is
centred in. The sidebar is outside that area, so the popup sits half the
sidebar's width right of the window's centre, and the height is the only lever
on how high up it starts. The manifest asks for 60% by 70%, which the
`[window]` table of the plugin's configuration replaces.

## Collecting a value

A command that needs a value — a rename, a branch name, a prompt — gets a
popup of its own, two lines tall, rather than the palette's window. It is the
same handover, one hop longer: the palette writes the entry down with a
`prompt` and quits, `exec` opens the `input` pane for it, and the field writes
the entry back with the value and asks for `exec` again, which runs it once the
field's popup is gone in turn.

The field is not a Bubble Tea screen, and that is the point. Bubble Tea hides
the terminal cursor for the lifetime of the program and paints its caret as
cell content. herdr forwards a pane's cursor to the outer terminal only while
the pane shows one, and macOS input methods place their candidate window at
that cursor, so a Bubble Tea field cannot be typed into with an input method.
`internal/prompt` therefore edits the line itself, keeping the terminal's own
cursor on the insertion point.

## Running a configured command

A `[[keys.command]]` entry cannot be run by name over the API, so each type is
reproduced: `shell` is started in a session of its own, and `pane` and `popup`
open the plugin's `run` pane, which execs the command line under a shell and
exits with it. The placement is what separates the two — `popup` for herdr's
popup type, and `zoomed` for its pane type, which takes over the layout the way
herdr's own temporary pane does. A zoomed pane is placed against an existing
pane and takes its id; a popup covers the active pane and herdr rejects a
target or a workspace alongside it.

## The palette's own configuration

`internal/settings` reads one file in the plugin's config directory, in a
single decode: the `[[command]]` entries and the colours. It exists because
herdr runs a `[[keys.command]]` entry from a key and nothing else, so a command
reached through the palette would otherwise need a binding it never uses.

A command runs in the window it asks for, and detached when it asks for none —
the same shell command herdr's own `shell` type starts. The windows are the
plugin pane placements: `popup` for a session-modal terminal, `pane` for
herdr's zoomed placement, and `tab` for a tab of its own, opened without focus
and renamed to the command's title. A tab is the one a command can be returned
to: it is a pane, so the rows that go somewhere list it, which a detached
process has no way to be. herdr has no hidden pane or tab to put it in instead.

## Colours

herdr does not publish its theme: the API has no method for it, and
config.toml carries only the theme's name plus the tokens the user overrode.
`internal/theme` therefore resolves each colour from three places, each
overriding the one before it — built-in defaults, the `[theme.custom]` tokens
`internal/keys` read, and what `internal/settings` read from the plugin's own
configuration. The statuses an agent can be in are colours of the same kind,
keyed by herdr's names for them.

The defaults are ANSI indexes, which follow the terminal, except the rule and
the selected row: both need a shade just off the terminal's background, which
the ANSI palette has no index for, so they are adaptive hex values.

## Reading herdr's configuration

The key column, the configured commands and the theme tokens all come from
files rather than the API, because herdr exposes none of them: `[keys]`,
`[[keys.command]]` and `[theme.custom]` live in `config.toml`, and the default
key bindings only in what `herdr --default-config` prints. `internal/keys`
reads `config.toml` once for all three, and the defaults from the binary named
by `HERDR_BIN_PATH`.

The `exec` entrypoint skips the defaults: the key column is the one thing it
never renders, and reading them costs a subprocess.

A popup size is decoded by the client library's manifest package, which accepts
both spellings herdr does — a cell count and a percentage string.

The prose in the printed `[keys]` section contains lines shaped like
assignments, such as the `type = "popup"` documenting custom commands, so the
parse collects more names than there are actions. Lookups go through a fixed
set of action names, which is what keeps that harmless.

## Matching

`internal/palette.Rank` scores each row with fzf's own matcher,
`github.com/junegunn/fzf/src/algo`. A query is split on spaces and every word
has to match, the way fzf reads a query with spaces in it, so the words of a
row can be typed in any order; the scores are added and the matched positions
merged for highlighting.

The row it matches is the row as it is drawn, namespace included, so `spr`
reaches "herdr: split pane right" and the highlighted letters are the ones that
were searched. fzf's scoring is what separates a run at the start of a word
from letters scattered through a row, which is the difference the palette
depends on at the size it has.

The matcher folds no case of its own, so the row is lowercased rune by rune
before it is scored, which keeps the positions it returns lined up with what is
drawn.

A query that matches no row is retried with the entry's `Search` text
prepended, scored lower. `Search` is not part of the row — for a plugin action
it is the plugin's id, and for a pane the workspace and directory it sits in —
so a row matched that way would have nothing highlighted. The positions in
front of the row belong to the search text, and the row shows it next to the
title, dimmed and highlighted the same way, in whatever width is left once the
title and the key have theirs.

## Mouse

The program runs with `tea.WithMouseCellMotion`, which reports clicks and the
wheel. herdr captures the mouse for its own UI but forwards events to a pane
app that asks for them, so the popup receives them.

A click's row is `offset + Y - headerRows`, where `headerRows` is the query
line and the rule under it. A click outside the rendered rows does nothing.

## State

`recent.json` in the plugin's state directory holds the ids of the commands
last run, most recent first, capped at 50. It is a convenience: an unreadable
file only means the list opens unordered.
