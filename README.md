# Command Palette for Herdr

One popup that searches every action available in the current session and runs
it, in the shape of an editor command palette. It lists herdr's own commands
next to the actions every other installed plugin registered and the workspaces,
tabs and panes you can jump to, filters them as you type, and remembers what
you ran last.

Requires [herdr](https://herdr.dev) 0.9.0 or newer. Linux and macOS.

## Install

```sh
herdr plugin install vika2603/herdr-palette
```

Install builds the binary from source when a Go toolchain is present, and
otherwise downloads the binary attached to the release that matches the
manifest version, accepting it only if its SHA-256 matches
[`scripts/checksums.txt`](scripts/checksums.txt) in the checkout. Release
binaries are built by the `release` workflow for macOS and Linux on amd64 and
arm64.

Or, from a checkout — `herdr plugin link` runs no build command, so `just link`
builds the working tree first:

```sh
just link
```

## Bind a key

The plugin registers `herdr.palette.open`. Bind it in
`~/.config/herdr/config.toml`:

```toml
[[keys.command]]
key = "ctrl+shift+p"
description = "Open command palette"
type = "plugin_action"
command = "herdr.palette.open"
```

A second action, `herdr.palette.goto`, opens the same popup with its query
already narrowed to what is open, for reaching a pane by name without the
commands in the way:

```toml
[[keys.command]]
key = "prefix+j"
description = "Go to an open pane"
type = "plugin_action"
command = "herdr.palette.goto"
```

A third action, `herdr.palette.back`, opens no popup at all: it goes straight
to the workspace, tab or pane the palette last went to, skipping what has been
closed since:

```toml
[[keys.command]]
key = "prefix+alt+b"
description = "Go back to the last place"
type = "plugin_action"
command = "herdr.palette.back"
```

`ctrl+shift+p` reaches herdr only if the host terminal encodes it, which needs
the kitty keyboard protocol or CSI u. A prefix binding always works:

```toml
key = "prefix+space"
```

Note that herdr's defaults already use `prefix+p` for `previous_tab` and
`prefix+shift+p` for `rename_pane`.

## What the list contains

A row reads `namespace: title`, with the key it is bound to at the right edge.
The namespace says where the row comes from.

**`herdr:`** is a list this plugin maintains, because herdr publishes no
equivalent of `plugin.action.list` for its built-in actions. Each one calls the
socket API method with the same effect: workspace, tab, pane, worktree and
agent commands, plus a config reload. A few have no herdr action behind them at
all — splitting left and up, which herdr splits right or down and then swaps
the new pane into place, and moving the pane to another tab. Built-in actions
with no API equivalent — `settings`, `help`, `toggle_sidebar`, `resize_mode` —
are not in the list.

**`command:`** is your own commands, the `[[keys.command]]` entries in
`config.toml`. herdr offers no way to run one by name, so each type is
reproduced over the API: `shell` is started detached, `pane` opens a temporary
pane over the layout, and `popup` a session-modal terminal, both running the
configured command line. Sizes are accepted in either spelling herdr takes, a
percentage such as `"70%"` or a cell count such as `80`.

Managing plugins is one command rather than a pair: it lists every installed
plugin with what it is, `enabled` or `disabled`, and `tab` turns the selected
one over. The list stays up with the row saying what it is now, so several can
be changed in a row, and `esc` leaves it. The palette is not in the list —
turning it off would take away the popup the row is being run from, with no row
left to turn it back on.

herdr enforces it: a disabled plugin's actions are refused with
`plugin_disabled`, and the state outlives a restart. herdr lists those actions
all the same, so the palette leaves them out itself. It reads what is installed
when it opens, so a plugin turned off from inside the popup keeps its rows
until the next time the palette is opened.

**A plugin's own name**, such as `machine manager:`, is every action it
registered, read from `plugin.action.list` at open time. Installing a plugin
adds its actions to the palette with no configuration, and the palette forwards
the invocation context so an action sees the same focused pane it would have
seen from a key. Typing the plugin's id finds them as well.

**`workspace:`, `tab:`, `pane:` and `agent:`** are what is open in the session,
and running one goes there. A pane running an agent shows what it is doing —
`claude · working`, `codex · blocked` — in a colour per status, kept current
while the popup is open, so the palette doubles as a way to reach the agent
that needs you. An agent that was renamed shows the name it was given there
instead of what it is, and is found by it. Prompting one is a command of its
own: it asks which agent and then what to say, so an answer reaches an agent in
another workspace without going there first. Panes also carry the workspace
they sit in and their working directory, so typing a project name finds the
panes inside it. Where the palette was opened from is left out.

A session holds far more panes than there are commands, so there are two ways
to leave the commands out. They all read "go to …", so typing `go to` or `goto`
narrows the list to them. `@` does it as a prefix: the rest of the query
filters those rows the way it filters any others, so `@claude` reaches the
agent in one go, and deleting the `@` puts the commands back. A key bound to
the `goto` action opens the popup already narrowed that way, with no commands
in between at all.

Editing herdr's configuration is a command too: it opens `config.toml` in the
editor `$VISUAL` or `$EDITOR` names, in a popup, falling back to `vi`. herdr
reads that file on `reload config`, which is the row below it.

Saving a tab's layout is the palette's own. herdr exports an arrangement and
applies one back but keeps none, so the palette writes them down in its state
directory under the name you give. Opening one puts it in a new tab of the
focused workspace, named after the layout, and the panes start in the
directories they were saved with — the panes already in a tab are somebody's
work, which is why it is a new one. An exported arrangement carries a pane's
directory but not what is running in it, so the palette saves that too: a
layout whose panes ran agents opens with those agents started again, in the
panes that took their place.

## Handing an agent something to look at

herdr carries the selection and every pane's output, and prompts an agent, but
nothing joins the two — what is on screen reaches an agent only by being copied
there by hand. Three commands close that:

**`ask an agent about the selection`** takes the text selected when the key
opened the palette, asks which agent and what to ask, and sends the question
with the selection behind it. The row is only in the list when something is
selected.

**`ask an agent about this pane's output`** does the same with the last 200
lines the focused pane printed, read unwrapped and without escape sequences —
a failing test or a stack trace goes to an agent in another workspace without
being copied anywhere.

**`tell me when an agent stops`** waits for an agent to reach `done`, `blocked`
or `idle` and shows a notification saying which. The wait runs outside the
popup, in the entrypoint a handed-over command uses, so the palette closes on
the keystroke and the watch outlives it. After an hour it stops waiting and
says the agent is still working, which is news rather than a failure — an agent
left running should not leave a process behind it either.

## Finding what a pane printed

**`search what the panes have printed`** reads the last 200 lines of every pane
in the session and offers them as rows, each showing the pane and workspace it
came from. Typing filters them the way it filters commands, and choosing one
goes to that pane. herdr's own copy mode searches the pane you are in; which
pane something was printed in is the question it does not answer.

A command that needs a value, such as a rename or a prompt, opens a small field
of its own once the palette closes. Renames start from the current label. The
field keeps the terminal's own cursor on the insertion point, which is what
macOS input methods anchor their candidate window to.

A command that acts on one of several things — the worktree to open or remove,
the tab to move the pane to — lists them in the palette's own window instead of
asking for a value. Typing filters them the way it filters the commands,
`enter` runs the command on the row, and `esc` goes back to the command list
with the query it was filtered by. A command with nothing to act on says so on
the line under the list and stays where it is. A few commands are screens
rather than one act — managing plugins is one — and their list stays up after a
row runs, rebuilt so the rows say what they are now. A command that needs both, such
as prompting an agent by name, takes the target from the list and then opens
the field for the value.

## The palette's own window

The palette has a configuration file of its own, in the directory
`herdr plugin config-dir herdr.palette` prints. It holds what herdr's API does
not publish: the size of the popup and the colours it is drawn in.

herdr centres a popup in the pane area and offers no say over where it sits, so
its height is also what decides how high up it starts: a taller one begins
closer to the top:

```toml
[window]
width = "60%"
height = "70%"
```

Sizes take either spelling herdr does, a percentage or a cell count. Left out,
the plugin's own 60% by 60% stands. The pane area is what a percentage is of,
which is the terminal minus the sidebar — the popup is centred in it, so it
sits half the sidebar's width right of the window's centre, and nothing in
herdr's API moves it.

## Keys in the list

The key column comes from herdr's own configuration: the defaults it ships,
with `config.toml` laid over them. A default binding whose key the
configuration gave to something else is left blank rather than shown for two
commands.

The column is as wide as the widest key on show, and is left out entirely once
that would leave the rows too little for what they are and the detail beside
them — what an agent is doing is worth more than the key beside a command, and
half a key names nothing.

## Keys in the popup

| Key | Effect |
| --- | --- |
| type | filter the list |
| `enter` | run the selection |
| `up` / `down`, `ctrl+p` / `ctrl+n` | move the selection |
| `pgup` / `pgdown` | move a page |
| `tab` | turn the selected row over, on a screen that stays up |
| `ctrl+u` | clear the query |
| `@` | narrow the list to what is open, as the first character |
| `backspace` | leave a list of targets, once the query is empty |
| `esc` | close the popup, or leave a list of targets for the commands |
| `ctrl+c` | close the popup, wherever you are in it |
| wheel | move the selection |
| left click | run the row it lands on |

`up` and `down` go round at the ends, so the far end of a long list is one
keystroke away. A page and a turn of the wheel stop there instead: going round
would carry you past what you were looking at.

A command that cannot be undone — closing a workspace, a tab or a pane,
removing a worktree, forgetting a layout — asks before it runs. The line under
the list names it, `enter` runs it and any other key puts the question away, so
neither a keystroke meant for the row above nor a click on a row that moved
closes somebody's work.

The value field takes `enter` to run the command with what is typed and `esc`
to cancel, along with the usual line editing: arrows and `ctrl+a` / `ctrl+e`,
`ctrl+w`, `ctrl+u`, `ctrl+k`.

A scrollbar at the right edge shows where the visible rows sit in the whole
list, and stays blank while everything fits.

The selected row carries a marker in its leading column as well as the band
behind it, so which row is selected is readable where a terminal or a theme
draws no background.

The line under the list names the selected row in full. A row shares its width
with the detail beside it and with the key column, and the line under the list
shares with neither, so what the row had to cut is readable there. On a popup
too narrow to carry both, the name gives way first and the keys are dropped one
at a time only once there is nothing worth reading left of them; neither wraps
onto a line the list would otherwise have. A query longer than the line scrolls
inside it, for the same reason.

A command already out on the socket is not run again by a second keystroke, and
the line under the list says so while the answer is on its way.

Matching is fzf's own: every word of the query has to appear in the row, in
any order (`pane split`), each as a run of letters that need not be adjacent,
so initials work too (`spr` for "herdr: split pane right"). A row can also
match on text it does not show, such as a plugin's id or the directory a pane
is in; it then shows that text next to the title, so the match is visible.
An empty query scores every row the same, so what orders the list then is
what was run recently, and after that the commands — a session holds as many
rows that go somewhere as it has panes, and the palette opens on what it is
for. A pane you just left is one you ran, so the way back to it is still
short.

## Colours

The popup follows the terminal's palette for most of what it draws. Two shades
sit just off the terminal's own background, which the ANSI palette has no index
for: the line above and below the list, and the band behind the selected row.

An agent that is blocked is drawn in bright red rather than the plain red the
error line has: it is the popup's own news rather than a command that would
not run, and the two share the line under the list.

Where herdr's `[theme.custom]` defines them, its tokens are used instead:
`overlay0` for the rules, `surface0` for the selected row, `overlay1` for the
query's letters inside a row. herdr publishes no theme over the API, so the
tokens it did not write down are not available.

To set them yourself, write them in the same `config.toml` the window size goes
in. Each value is a hex colour or an ANSI index, and anything left out keeps
what the rules above resolved:

```toml
rule = "#414868"                # the lines above and below the list
selected_background = "#24283b" # the band behind the selected row
match = "#7aa2f7"               # the query's letters inside a title
meta = "8"                      # the key column and the namespace
scrollbar = "8"                 # the scrollbar's thumb, on a track drawn in rule
failure = "1"                   # the error line

[status]                        # the state a row is in, by its own name
working = "3"                   # what an agent is doing, by herdr's names
blocked = "9"
done = "2"
idle = "8"
enabled = "2"                   # whether a plugin is on
disabled = "8"
```

## Development

```sh
just check   # build, test, lint
just link    # point herdr at this working tree
just open    # open the popup without pressing the key
just logs    # exit codes and output of every plugin command herdr ran
```

See `docs/design.md` for how the entrypoints fit together and why the herdr
command list is maintained by hand.
