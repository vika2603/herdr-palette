# Command Palette for Herdr

One popup that searches every action available in the current session and runs
it, in the shape of an editor command palette. It lists herdr's own commands
next to the actions every other installed plugin registered and the workspaces,
tabs and panes you can jump to, filters them as you type, and remembers what
you ran last.

Requires herdr 0.9.0 or newer. Linux and macOS.

## Install

```sh
herdr plugin install vika2603/herdr-palette
```

Or, from a checkout:

```sh
just link
```

## Bind a key

The plugin registers one action, `herdr.palette.open`. Bind it in
`~/.config/herdr/config.toml`:

```toml
[[keys.command]]
key = "ctrl+shift+p"
description = "Open command palette"
type = "plugin_action"
command = "herdr.palette.open"
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
agent commands, plus a config reload. Splitting left and up is there too, which
herdr has no action for — it splits right or down and swaps the new pane into
place. Built-in actions with no API equivalent — `settings`, `help`,
`toggle_sidebar`, `resize_mode` — are not in the list.

**`command:`** is your own commands, the `[[keys.command]]` entries in
`config.toml`. herdr offers no way to run one by name, so each type is
reproduced over the API: `shell` is started detached, `pane` opens a temporary
pane over the layout, and `popup` a session-modal terminal, both running the
configured command line. Sizes are accepted in either spelling herdr takes, a
percentage such as `"70%"` or a cell count such as `80`.

**A plugin's own name**, such as `machine manager:`, is every action it
registered, read from `plugin.action.list` at open time. Installing a plugin
adds its actions to the palette with no configuration, and the palette forwards
the invocation context so an action sees the same focused pane it would have
seen from a key. Typing the plugin's id finds them as well.

**`workspace:`, `tab:`, `pane:` and `agent:`** are what is open in the session,
and running one goes there. A pane running an agent shows what it is doing —
`claude · working`, `codex · blocked` — in a colour per status, kept current
while the popup is open, so the palette doubles as a way to reach the agent
that needs you. They all read "go to …", so typing `go to` or `goto`
narrows the list to them. Panes also carry the workspace they sit in and their
working directory, so typing a project name finds the panes inside it. Where
the palette was opened from is left out.

A command that needs a value, such as a rename or a prompt, opens a small field
of its own once the palette closes. Renames start from the current label. The
field keeps the terminal's own cursor on the insertion point, which is what
macOS input methods anchor their candidate window to.

## Your own commands

The palette has a configuration file of its own, in the directory
`herdr plugin config-dir herdr.palette` prints. A command written there needs
no key — the palette is how it is reached — and runs in the background unless
it asks for a window:

```toml
[[command]]
title = "Sync dotfiles"
run = "zsh ~/scripts/sync.sh"   # detached: no window, and its output goes nowhere

[[command]]
title = "Watch tests"
run = "just watch"
window = "tab"                  # a tab of its own, unfocused; "go to watch tests" returns to it

[[command]]
title = "Open lazygit"
run = "lazygit"
window = "popup"                # a session-modal terminal
width = "80%"
height = "80%"
```

`window = "pane"` is the fourth: a temporary pane over the layout, which closes
when the command exits. A command with no `title` is listed under what it runs,
and one with no `run` is left out. These rows are listed as `command:`,
alongside the `[[keys.command]]` entries from herdr's own configuration.

The window is what decides whether a command can be returned to. A background
command is a detached process with no pane, which herdr cannot show later; a
tab is a pane, so the palette lists it and `go to` reaches it.

## The palette's own window

herdr centres a popup in the pane area and offers no say over where it sits, so
its height is also what decides how high up it starts: a taller one begins
closer to the top. The same file sets it:

```toml
[window]
width = "60%"
height = "70%"
```

Sizes take either spelling herdr does, a percentage or a cell count. Left out,
the plugin's own 60% by 70% stands. The pane area is what a percentage is of,
which is the terminal minus the sidebar — the popup is centred in it, so it
sits half the sidebar's width right of the window's centre, and nothing in
herdr's API moves it.

## Keys in the list

The key column comes from herdr's own configuration: the defaults it ships,
with `config.toml` laid over them. A default binding whose key the
configuration gave to something else is left blank rather than shown for two
commands.

## Keys in the popup

| Key | Effect |
| --- | --- |
| type | filter the list |
| `enter` | run the selection |
| `up` / `down`, `ctrl+p` / `ctrl+n` | move the selection |
| `pgup` / `pgdown` | move a page |
| `ctrl+u` | clear the query |
| `esc` | close the popup |
| wheel | move the selection |
| left click | run the row it lands on |

The value field takes `enter` to run the command with what is typed and `esc`
to cancel, along with the usual line editing: arrows and `ctrl+a` / `ctrl+e`,
`ctrl+w`, `ctrl+u`, `ctrl+k`.

A scrollbar at the right edge shows where the visible rows sit in the whole
list, and stays blank while everything fits.

Matching is fzf's own: every word of the query has to appear in the row, in
any order (`pane split`), each as a run of letters that need not be adjacent,
so initials work too (`spr` for "herdr: split pane right"). A row can also
match on text it does not show, such as a plugin's id or the directory a pane
is in; it then shows that text next to the title, so the match is visible.
Recently run commands come first when the query is empty.

## Colours

The popup follows the terminal's palette for most of what it draws. Two shades
sit just off the terminal's own background, which the ANSI palette has no index
for: the line above and below the list, and the band behind the selected row.

Where herdr's `[theme.custom]` defines them, its tokens are used instead:
`overlay0` for the rules, `surface0` for the selected row, `overlay1` for the
query's letters inside a row. herdr publishes no theme over the API, so the
tokens it did not write down are not available.

To set them yourself, write them in the same `config.toml` the commands above
go in. Each value is a hex colour or an ANSI index, and anything left out keeps
what the rules above resolved:

```toml
rule = "#414868"                # the lines above and below the list
selected_background = "#24283b" # the band behind the selected row
match = "#7aa2f7"               # the query's letters inside a title
meta = "8"                      # the key column and the namespace
scrollbar = "8"                 # the scrollbar's thumb, on a track drawn in rule
failure = "1"                   # the error line

[status]                        # what an agent is doing, by herdr's names
working = "3"
blocked = "1"
done = "2"
idle = "8"
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
