# Command Palette for Herdr

One popup that searches herdr's commands, every installed plugin's actions and
the open panes, and runs what you pick.

Requires [herdr](https://herdr.dev) 0.9.0 or newer, on Linux, macOS or Windows.

## Install

```sh
herdr plugin install vika2603/herdr-palette
```

Install builds from source when Go is present, and otherwise downloads the
release archive for your platform and checks it against the release's
`checksums.txt`.

## Bind keys

In herdr's `config.toml` (`~/.config/herdr/`, or `%APPDATA%\herdr\` on
Windows):

```toml
[[keys.command]]
key = "alt+space"
type = "plugin_action"
command = "herdr.palette.toggle"   # open the palette; the same key closes it

[[keys.command]]
key = "prefix+j"
type = "plugin_action"
command = "herdr.palette.goto"     # open it on the open panes only

[[keys.command]]
key = "prefix+a"
type = "plugin_action"
command = "herdr.palette.attend"   # go to the next agent that is blocked or done

[[keys.command]]
key = "prefix+alt+b"
type = "plugin_action"
command = "herdr.palette.back"     # go back to where the palette last took you
```

Inside the popup a key reads as a terminal passes it on, so `ctrl+shift+p` is
`ctrl+p` there and `cmd` never arrives; a `prefix+` binding always works.

## What is in the list

- **`herdr:`** herdr's own commands, including a few herdr has no key for:
  splitting left and up, moving or swapping the pane, evening out pane sizes,
  `switch workspace` and `switch tab`, and searching what the panes printed.
- **`command:`** your `[[keys.command]]` entries, run the way herdr runs them.
- **A plugin's name** every action that plugin registered.
- **`pane:` / `agent:`** the open panes, with what each agent is doing. A
  pane is also found by its tab's, workspace's or directory's name.

Type the start of `agents`, `panes`, `tabs`, `workspaces`, `plugins`, `herdr`
or `commands` (your `[[keys.command]]` entries), two letters or more, or its
letters in order such as `cmd` or `wsp`, and press `tab` to list only those; the label in front of the query
names the scope. Tabs and workspaces are listed only in their scope, which
`switch tab` and `switch workspace` open too. `herdr.palette.goto` opens the
popup in `panes`.

Before you type, the list is grouped into what needs you, what you ran
recently, what you can run and what is open. The pane you opened the palette from
is listed last, marked `here`, so `ctrl+x` closes it without leaving it. From 110 columns wide the popup shows
a preview of the selected pane or command.

## Keys in the popup

| Key | Effect |
| --- | --- |
| type | filter the list |
| `enter` | run the selection |
| `up` / `down`, `ctrl+p` / `ctrl+n` | move the selection |
| `pgup` / `pgdown` | move a page |
| `tab` | list only the scope the query starts the name of; otherwise answer the selected agent when it is blocked, and `esc` gives the keyboard back |
| `ctrl+x` | close the selected pane, tab or workspace without going there |
| `ctrl+u` | clear the query |
| `backspace` | leave a scope or a list of targets, once the query is empty |
| `esc` | close the popup, or leave a scope or a list of targets |
| `ctrl+c`, the `toggle` key | close the popup |

A command that cannot be undone asks first: `enter` runs it, any other key
cancels.

## Configuration

The palette's own `config.toml` is in the directory
`herdr plugin config-dir herdr.palette` prints. Everything is optional:

```toml
scheme = "herd"            # or "terminal", to follow the terminal's colours
accent = "#A48BFF"         # any colour below takes a hex value or an ANSI index
rule = "#414868"
selected_background = "#24283b"
match = "#7aa2f7"
meta = "8"
faint = "8"
scrollbar = "8"
failure = "1"

[window]                   # a percentage of the pane area or a cell count
width = "80%"
height = "60%"

[status]                   # the colour of each agent state
working = "3"
blocked = "9"
done = "2"
idle = "8"
```

Colours herdr's `[theme.custom]` defines (`accent`, `overlay0`, `surface0`,
`overlay1`) are used when the file sets none.

## Development

```sh
just check   # build, test, lint
just link    # point herdr at this working tree
just open    # open the popup without pressing the key
just logs    # what every plugin command herdr ran printed
```

To release, bump `version` in `herdr-plugin.toml`, push, and run the `release`
workflow: it tags that commit and publishes the archives and `checksums.txt`.

How it works, and why, is in [`docs/design.md`](docs/design.md).
