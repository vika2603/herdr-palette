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
  moving the workspace toward the front or back,
  `Switch Workspace` and `Switch Tab`, and searching what the panes printed.
- **`command:`** your `[[keys.command]]` entries and scripts, with no key
  binding needed for a script.
- **A plugin's name** every action that plugin registered.
- **`pane:` / `agent:`** the open panes, with what each agent is doing. A
  pane is also found by its tab's, workspace's or directory's name.

Type the start of `agents`, `panes`, `tabs`, `workspaces`, `plugins`, `herdr`
or `commands` (your configured commands and scripts), two letters or more, or its
letters in order such as `cmd` or `wsp`, and press `tab` to list only those; the label in front of the query
names the scope. Tabs and workspaces are listed only in their scope, which
`Switch Tab` and `Switch Workspace` open too. `herdr.palette.goto` opens the
popup in `panes`.

Before you type, the list is grouped into what needs you, what you ran
recently, what you can run and what is open. The pane you opened the palette from
is listed last, marked by a small coloured dot on the right, so `ctrl+x` closes
it without leaving it. From 110 columns wide the popup shows a preview of the
selected pane or command. Press `ctrl+o` to hide or show it
while the palette is open. Hiding it gives the list the full width; answering
a blocked agent still shows its screen.

Agent rows show a coloured state icon before the name: `●` working, `!` blocked,
`✓` done, `○` idle. Unknown states have no icon. State names remain searchable
and appear in the footer for the selected row.

## Keys in the popup

| Key | Effect |
| --- | --- |
| type | filter the list |
| `enter` | run the selection |
| `up` / `down`, `ctrl+p` / `ctrl+n` | move the selection |
| `pgup` / `pgdown` | move a page |
| `tab` | list only the scope the query starts the name of; otherwise move into the fields of a script that takes arguments, or answer the selected agent when it is blocked, and `esc` gives the keyboard back |
| `ctrl+x` | close the selected pane, tab or workspace without going there |
| `ctrl+o` | hide or show the side preview on a wide popup |
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
script_dirs = ["scripts"] # relative to this config file; absolute paths and ~/ work too

scheme = "herd"            # or "terminal", to follow the terminal's colours
accent = "#A48BFF"         # any colour below takes a hex value or an ANSI index
rule = "#414868"
selected_background = "#24283b"
match = "#7aa2f7"
meta = "8"
faint = "8"
scrollbar = "8"
failure = "1"
background = "#181825"     # what the popup is laid on

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

Without `background`, the popup darkens the terminal's background so it stands
apart from the panes under it. A black background cannot be darkened, so set
`background` there. Give it as a hex value: an ANSI index is converted with the
standard palette rather than the terminal's.

## Scripts

Put scripts in `scripts/` beside the palette's `config.toml`, or choose one or
more directories with `script_dirs = ["scripts", "~/dotfiles/herdr-scripts"]`.
The list replaces the default; `script_dirs = []` disables script discovery.
Search their names in the palette and press `enter`; adding or editing a script
takes effect the next time you open it. Repeated paths and links to the same
script are listed once, using the first reference. Different scripts with the
same title stay separate and show their paths.

On Linux and macOS, give each script a shebang and make it executable
(`chmod +x lazygit.sh`). For example, `lazygit.sh`:

```sh
#!/usr/bin/env sh
# @palette.title Lazygit
# @palette.mode pane

exec lazygit
```

Both comments are optional. The title defaults to the filename without its
extension. The mode defaults to `shell`, which runs in the background with no
terminal output; `pane` opens a temporary full-screen pane and `popup` opens a
floating terminal. The terminal closes when the script exits. For an external
editor, a script containing a shebang followed by `zed .` needs no metadata.

A `popup` script can set `@palette.width` and `@palette.height`, as a cell
count (`30`) or a percentage (`80%`).

`@palette.confirm true` makes the palette ask before running the script, with
the values of its arguments in the question: `enter` runs it, any other key
cancels and keeps what was entered.

Scripts run in the directory of the pane you opened the palette from, with
`HERDR_ACTIVE_WORKSPACE_ID`, `HERDR_ACTIVE_TAB_ID`, `HERDR_ACTIVE_PANE_ID` and
`HERDR_ACTIVE_PANE_CWD` available when herdr supplies them.

### Arguments

A script can ask for up to three values, each declared on one line:

```sh
# @palette.argument1 { "name": "branch", "type": "text" }
# @palette.argument2 { "name": "env", "type": "dropdown", "data": [{ "title": "Staging", "value": "staging" }, { "title": "Production", "value": "production" }] }
# @palette.argument3 { "name": "token", "type": "password", "optional": true }
```

`name` and `type` (`text`, `password` or `dropdown`) are required;
`placeholder`, `optional`, `percentEncoded` and `data` work as in Raycast
script commands. When the script is selected its fields follow the query:
`tab` moves through them and `enter` runs it. In a dropdown, `up` and `down`
choose an option and typing filters the options by title. The script gets the
values as `$1` to `$3` and as `HP_` plus the name in capitals (`HP_BRANCH`); on
Windows only as the latter. A password is passed in clear text. The fields'
rounded ends are Nerd Font glyphs.

A dropdown can list its options with a command in place of `data`:

```sh
# @palette.argument1 { "name": "branch", "type": "dropdown", "command": "git branch --format='%(refname:short)'" }
```

The command runs through the shell when the dropdown first gets the focus, in
the directory a script runs in and with the same `HERDR_ACTIVE_` variables.
The arguments before the dropdown are passed as their `HP_` variables, so its
options can depend on them:

```sh
# @palette.argument1 { "name": "repo", "type": "text" }
# @palette.argument2 { "name": "branch", "type": "dropdown", "command": "git -C \"$HP_REPO\" branch --format='%(refname:short)'" }
```

When one of those values changes, the dropdown lists its options again the
next time it gets the focus, and `enter` takes the focus there first. Each line
the command prints is an option; a line holding a tab is the title before the
tab and the value after it. A command that fails, or takes more than five
seconds, reports why in the footer and runs again when the dropdown next gets
the focus.

On Windows, use `.ps1`, `.cmd` or `.bat`. PowerShell scripts use the same `#`
comments; batch scripts use `REM @palette.title ...` and `REM @palette.mode ...`
(or `::` comments), before the first command. PowerShell uses the system's
execution policy: if Windows PowerShell blocks scripts, they must be allowed
there before using `.ps1` commands in the palette. UTF-16 PowerShell files with
a byte-order mark are supported alongside UTF-8 files.

Only each directory's immediate files are listed, including links to scripts;
dotfiles and subdirectories are skipped. Metadata belongs in the initial
comments, within the first 4 KiB. An invalid script reports its filename while
the other commands and directories remain available.

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
