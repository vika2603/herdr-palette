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

The plugin registers `herdr.palette.toggle`, which opens the popup and closes
it when it is already up, so one key alternates between the two. Bind it in
`~/.config/herdr/config.toml`:

```toml
[[keys.command]]
key = "alt+space"
description = "Toggle command palette"
type = "plugin_action"
command = "herdr.palette.toggle"
```

herdr hands every key to a popup while one is up, before it looks at its own
bindings, so the second press never reaches the action: it arrives inside the
popup, which closes itself when the key is the one bound to `toggle`. The key
is matched as the popup's terminal receives it, which loses what a terminal
cannot pass on to a program: `shift` held with `ctrl` and a letter, so
`ctrl+shift+p` and `ctrl+p` read the same inside the popup, and `cmd` or
`super`, which never arrive at all. A `prefix+` chord works, the two keys
reaching the popup in turn, and the prefix then starts the chord inside the
popup as well.

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

A fourth, `herdr.palette.attend`, opens no popup either: it goes to an agent
that needs you — a blocked one first, then one that is done — and pressed again
from one of them it goes on to the next. The pane it was pressed in becomes the
place `back` returns to, so going to answer an agent and coming back to the
work it interrupted is two keys. Going on from one agent to the next keeps that
way back, so `back` still returns to the work however many agents were
visited:

```toml
[[keys.command]]
key = "prefix+a"
description = "Go to an agent that needs you"
type = "plugin_action"
command = "herdr.palette.attend"
```

`ctrl+shift+p` reaches herdr only if the host terminal encodes it, which needs
the kitty keyboard protocol or CSI u. A prefix binding always works:

```toml
key = "prefix+space"
```

Note that herdr's defaults already use `prefix+p` for `previous_tab` and
`prefix+shift+p` for `rename_pane`.

## What the list contains

A row reads its title first, and then, each in a column of its own at the
right, the state it is in, the namespace it comes from and the key it is bound
to. The namespace says where the row comes from; the line under the list names
the selected row with it in front, as in `herdr: split pane right`.

**`herdr:`** is a list this plugin maintains, because herdr publishes no
equivalent of `plugin.action.list` for its built-in actions. Each one calls the
socket API method with the same effect: workspace, tab, pane, worktree and
agent commands, plus a config reload. A few have no herdr action behind them at
all — splitting left and up, which herdr splits right or down and then swaps
the new pane into place; moving the pane to another tab, a new tab of its own
or a new workspace; swapping the pane with another in its tab, picked from a
list with its screen in the preview; and evening out pane sizes, which gives every pane in the
tab the same share of the row or column it sits in. Built-in actions
with no API equivalent — `settings`, `help`, `toggle_sidebar`, `resize_mode` —
are not in the list.

**`command:`** is your own commands, the `[[keys.command]]` entries in
`config.toml`. herdr offers no way to run one by name, so each type is
reproduced over the API: `shell` is started detached, `pane` opens a temporary
pane over the layout, and `popup` a session-modal terminal, both running the
configured command line. Sizes are accepted in either spelling herdr takes, a
percentage such as `"70%"` or a cell count such as `80`. Each runs with the
variables herdr sets for a command run from a key — `HERDR_ACTIVE_WORKSPACE_ID`,
`HERDR_ACTIVE_TAB_ID`, `HERDR_ACTIVE_PANE_ID` and `HERDR_ACTIVE_PANE_CWD`,
naming where the palette was opened from — so a command line written against
them acts in the same place from either.

**A plugin's own name**, such as `machine manager:`, is every action it
registered, read from `plugin.action.list` at open time. Installing a plugin
adds its actions to the palette with no configuration, and the palette forwards
the invocation context so an action sees the same focused pane it would have
seen from a key. Typing the plugin's id finds them as well. A disabled
plugin's actions are refused with `plugin_disabled`, but herdr lists them all
the same, so the palette leaves them out itself.

**`workspace:`, `tab:`, `pane:` and `agent:`** are what is open in the session,
and running one goes there. A pane running an agent shows what it is doing —
`working · claude`, `blocked · codex` — behind a dot in a colour per status,
kept current
while the popup is open, so the palette doubles as a way to reach the agent
that needs you. An agent that was renamed shows the name it was given there
instead of what it is, and is found by it. Panes also carry the workspace
they sit in and their working directory, so typing a project name finds the
panes inside it. Where the palette was opened from is left out.

A session holds far more panes than there are commands, so there are two ways
to leave the commands out. They all read "go to …", drawn faint so the name
after it stands out, and typing `go to` or `goto` narrows the list to them. `@` does it as a prefix: the rest of the query
filters those rows the way it filters any others, so `@claude` reaches the
agent in one go, and deleting the `@` puts the commands back. A key bound to
the `goto` action opens the popup already narrowed that way, with no commands
in between at all.

Editing herdr's configuration is a command too: it opens `config.toml` in the
editor `$VISUAL` or `$EDITOR` names, in a popup, falling back to `vi`. herdr
reads that file on `reload config`, which is the row below it.

## Finding what a pane printed

**`search what the panes have printed`** reads the last 200 lines of every pane
in the session and offers them as rows, each showing the pane and workspace it
came from. Typing filters them the way it filters commands, and choosing one
goes to that pane. herdr's own copy mode searches the pane you are in; which
pane something was printed in is the question it does not answer.

A command that needs a value, such as a rename, opens a small field
of its own once the palette closes. Renames start from the current label. The
field keeps the terminal's own cursor on the insertion point, which is what
macOS input methods anchor their candidate window to.

A command that acts on one of several things — the worktree to open or remove,
where to move the pane — lists them in the palette's own window instead of
asking for a value. Typing filters them the way it filters the commands,
`enter` runs the command on the row, and `esc` goes back to the command list
with the query it was filtered by. A command with nothing to act on says so on
the line under the list and stays where it is.

## The palette's own window

The palette has a configuration file of its own, in the directory
`herdr plugin config-dir herdr.palette` prints. It holds what herdr's API does
not publish: the size of the popup and the colours it is drawn in.

herdr centres a popup in the pane area and offers no say over where it sits, so
its height is also what decides how high up it starts: a taller one begins
closer to the top:

```toml
[window]
width = "90%"
height = "70%"
```

Sizes take either spelling herdr does, a percentage or a cell count. Left out,
the plugin's own 80% wide by 60% tall stands. The pane area is what a percentage is of,
which is the terminal minus the sidebar — the popup is centred in it, so it
sits half the sidebar's width right of the window's centre, and nothing in
herdr's API moves it.

## What the popup shows

The label at the start of the query line names the screen: `PALETTE` for
everything, `GO TO` once the query starts with `@`, `PICK` while a command's
targets are listed, and `CONFIRM`, in red, while a command that cannot be
undone waits for its answer. The label has a slot of fixed width, so the query
does not move when the label changes. The stretch of rule under the label is
heavier and in the same colour.

The other end of that rule counts the agents in the session — the one in the
pane the palette was opened from as well — by state: blocked, done and working,
leaving out any with none. On a narrow popup the names of the states go first
and the coloured counts stay; on a narrower one the count goes too.

Before anything is typed, the list is laid out in groups under headings:
`NEEDS YOU` for the agents that are blocked or done, blocked first; `RECENT`
for what was run last; `COMMANDS`; and `OPEN` for the rest of what is open. The
first query character puts the matches back into one ranked list. A list that
falls into one group has no heading, and a heading is never selected: the
selection moves between rows, and moving up onto a group's first row scrolls
its heading into view with it.

A popup at least 110 columns wide carries a preview beside the list. With a pane
or an agent selected it shows what the pane shows right now, read once the
selection has rested on it for a moment and again every second while it is on
show, so what a blocked agent is asking is readable without going there. Any
other row shows what it does: the key it is bound to as the configuration
spells it, the value it asks for, the list it picks from, and whether it asks
before it runs. A plugin action's card also carries the description the plugin
gave it, and a command from `config.toml` the command line it runs. A workspace
or a tab is previewed through one of its panes: the focused one when it is
there, and the first otherwise, since herdr says which pane is focused only for
the session as a whole. The preview takes two fifths of the popup, between 36
and 64 columns, and the scrollbar's column doubles as the line between the two.
A click on it selects nothing.

## Answering an agent from the palette

`tab` on an agent that is blocked hands it the keyboard without going there.
The label reads `REPLY`, the agent's screen takes the place of the list, and
what is typed goes to the agent as it is pressed — the digit of an option,
`enter`, the arrows, `tab`, `backspace`, a letter with `ctrl` — read back
within a moment of each key. `esc` gives the keyboard back to the list and is
never passed on, so leaving cannot tell the agent no; `ctrl+c` and the toggle
key close the palette as they do anywhere in it. Once the agent is no longer
blocked it has been answered, and the list comes back with a line saying so.
Keys pressed while the last ones are on their way wait for them, so they reach
the agent in the order they were pressed. A paste goes as it was typed, its line
breaks as `enter`; a chord with `alt` is not passed on. `tab` does nothing while
a command the palette ran is still out.

## Keys in the list

The key column comes from herdr's own configuration: the defaults it ships,
with `config.toml` laid over them. A default binding whose key the
configuration gave to something else is left blank rather than shown for two
commands.

A key is spelled the way it is pressed: the prefix as the prefix key itself,
`ctrl`, `alt` and `super` as `⌃`, `⌥` and `⌘`, shift on a letter as the
capital, and enter, tab and backspace as `⏎`, `⇥` and `⌫`. Under the default
`ctrl+b` prefix, `prefix+shift+x` reads `⌃b X` and `prefix+alt+1..9` reads
`⌃b ⌥1..9`. What comes before the key is drawn fainter than the key.

The column is as wide as the widest key on show, and is left out entirely once
that would leave the rows too little for what they are and the detail beside
them — what an agent is doing is worth more than the key beside a command, and
half a key names nothing. The namespace column goes next, for the same reason;
the line under the list still names both.

## Keys in the popup

| Key | Effect |
| --- | --- |
| type | filter the list |
| `enter` | run the selection |
| `up` / `down`, `ctrl+p` / `ctrl+n` | move the selection |
| `pgup` / `pgdown` | move a page |
| `tab` | answer the selected agent when it is blocked |
| `ctrl+x` | close the selected workspace, tab or pane without going there |
| `ctrl+u` | clear the query |
| `@` | narrow the list to what is open, as the first character |
| `backspace` | leave a list of targets, once the query is empty |
| `esc` | close the popup, or leave a list of targets for the commands |
| `ctrl+c` | close the popup, wherever you are in it |
| the key bound to `toggle` | close the popup, wherever you are in it |
| wheel | move the selection |
| left click | run the row it lands on |

`up` and `down` go round at the ends, so the far end of a long list is one
keystroke away. A page and a turn of the wheel stop there instead: going round
would carry you past what you were looking at.

A command that cannot be undone — closing a workspace, a tab or a pane,
removing a worktree — asks before it runs. The line under
the list names it, `enter` runs it and any other key puts the question away, so
neither a keystroke meant for the row above nor a click on a row that moved
closes somebody's work. Closing a row of what is open with `ctrl+x` asks the
same way, and leaves the popup up afterwards so the next one can be closed
from the same list.

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
the groups above, and inside each what was run recently, and after that the
commands — a session holds as many rows that go somewhere as it has panes, and
the palette opens on what it is for. A pane you just left is one you ran, so
the way back to it is still short.

## Colours

The popup draws in a palette of its own, `herd`, with one set of values for a
dark terminal and one for a light one. Its accent is violet, and marks what
has focus and nothing else: the label, the stretch of rule under it, the bar
of the selected row and the query's letters inside a row. The
colours of the states — amber for working, coral for blocked, mint for done,
slate for idle — sit apart from it in hue, so focus never reads as a state,
and red is kept for what cannot be undone or did not work. Text comes in three
strengths: the terminal's own foreground, a dim one for what a row carries
beside its title, and a faint one for headings, "go to" and the prefix of a
key.

`scheme = "terminal"` draws in ANSI indexes instead, which follow the
terminal's own colours, with magenta as the accent. Two shades sit just off the
terminal's background there, which the ANSI palette has no index for: the
rules and the band behind the selected row.

Where herdr's `[theme.custom]` defines them, its tokens are used over the
scheme: `accent` for the accent, `overlay0` for the rules, `surface0` for the
selected row, `overlay1` for the query's letters inside a row. The query's
letters follow the accent unless something names a colour for them. herdr
publishes no theme over the API, so the tokens it did not write down are not
available.

To set them yourself, write them in the same `config.toml` the window size goes
in. Each value is a hex colour or an ANSI index, and anything left out keeps
what the rules above resolved:

```toml
scheme = "herd"                 # or "terminal"
accent = "#A48BFF"              # the label, the selected row's bar
rule = "#414868"                # the lines above and below the list
selected_background = "#24283b" # the band behind the selected row
match = "#7aa2f7"               # the query's letters inside a title
meta = "8"                      # the detail, the namespace, the key
faint = "8"                     # headings, "go to", the prefix of a key
scrollbar = "8"                 # the scrollbar's thumb, on a track drawn in rule
failure = "1"                   # the label while asking, and the error line

[status]                        # the state a row is in, by its own name
working = "3"                   # what an agent is doing, by herdr's names
blocked = "9"
done = "2"
idle = "8"
```

A command with nothing to act on, such as opening a worktree when every one is
open, says so in the dim colour rather than in `failure`: nothing went wrong.

## Development

```sh
just check   # build, test, lint
just link    # point herdr at this working tree
just open    # open the popup without pressing the key
just logs    # exit codes and output of every plugin command herdr ran
```

See `docs/design.md` for how the entrypoints fit together and why the herdr
command list is maintained by hand.
