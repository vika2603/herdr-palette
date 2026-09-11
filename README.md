# Command Palette for Herdr

One popup that searches every action available in the current session and runs
it, in the shape of an editor command palette. It lists herdr's own commands
next to the actions every other installed plugin registered, filters them as
you type, and remembers what you ran last.

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

Every row shows the key it is bound to and where it comes from, in the last
column.

**herdr's own commands** are a list this plugin maintains, because herdr
publishes no equivalent of `plugin.action.list` for its built-in actions. Each
one calls the socket API method with the same effect: workspace, tab, pane,
worktree and agent commands, plus a config reload. Their column is the group
they belong to — `Pane`, `Tab`, `Workspace`, `Agent`, `Herdr`. Built-in actions
with no API equivalent — `settings`, `help`, `toggle_sidebar`, `resize_mode` —
are not in the list. Navigation is also left out: herdr's own `goto` and
`workspace_picker` already cover it.

**Your own commands**, the `[[keys.command]]` entries in `config.toml`, are
listed as `Custom`. herdr offers no way to run one by name, so each type is
reproduced over the API: `shell` is started detached, `pane` opens a split, and
`popup` opens a session-modal terminal, both running the configured command
line.

**Plugin actions** come from `plugin.action.list` at open time, listed as
`Plugin`, with the title and description each plugin declared. Installing a
plugin adds its actions to the palette with no configuration, and the palette
forwards the invocation context so an action sees the same focused pane it
would have seen from a key.

A command that needs a value, such as a rename or a prompt, asks for it on a
second screen. Renames start from the current label.

## Keys in the list

The key column comes from herdr's own configuration: the defaults it ships,
with `config.toml` laid over them. A default binding whose key the
configuration gave to something else is left blank rather than shown for two
commands.

## Keys in the popup

| Key | Effect |
| --- | --- |
| type | filter the list |
| `enter` | run the selection, or open the value screen |
| `up` / `down`, `ctrl+p` / `ctrl+n` | move the selection |
| `pgup` / `pgdown` | move a page |
| `ctrl+u` | clear the query |
| `esc` | close the popup, or leave the value screen |
| wheel | move the selection |
| left click | run the row it lands on |

A scrollbar at the right edge shows where the visible rows sit in the whole
list, and stays blank while everything fits.

Matching accepts the words of a title in any order (`pane split`) and a
title's initials (`spr` for "Split pane right"). Recently run commands come
first when the query is empty.

## Development

```sh
just check   # build, test, lint
just link    # point herdr at this working tree
just open    # open the popup without pressing the key
just logs    # exit codes and output of every plugin command herdr ran
```

See `docs/design.md` for how the two entrypoints fit together and why the herdr
command list is maintained by hand.
