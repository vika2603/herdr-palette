# Design

## Eight entrypoints, one binary

The manifest declares five actions, `toggle`, `goto`, `back`, `attend` and
`exec`, and three panes, `palette`, `input` and `run`. All eight run
`bin/palette`; `plugin.Env` tells the process which entrypoint started it.

`toggle` calls `plugin.pane.open` for the `palette` entrypoint, which runs the
TUI; closing is the popup's own half, below. `goto` opens the same pane with
`HERDR_PALETTE_SCOPE=panes` set, which is the popup starting in the scope of the
open panes — a key for reaching a pane by name, separate from the key for
running a command.
`back` goes to the last place the palette took you, with no popup at all.
`attend` goes to an agent that is blocked or done, also without a popup, and
records the pane it was pressed in as the place `back` returns to.
`exec` runs a command the popup handed over,
`input` is the field that collects a value a command needs, and `run` is the
pane a configured command runs in. All three are described below.

`cmd/palette` assembles the command list for the two entrypoints that need one,
so the popup and the handover work from the same list; `RunPending` resolves
the handed-over entry by id against it.

## Closing the popup with the key that opened it

What makes `toggle` a toggle is in the popup. herdr's client hands every key press to the popup while one is up,
ahead of its own binding lookup — `route_key_press` in
`src/client/shell/input.rs` of herdr 0.9.0 returns the popup as the target
before direct bindings or the prefix are considered — so a second press of
the key never invokes the action. It arrives inside the popup as terminal
input, and the popup quits when it is the key bound to `toggle`.

Which key that is comes from the same configuration the key column is read
from. Herdr's legacy key encoding makes alt an escape in front, ctrl with a
character is the control character, shift held with ctrl has no encoding and
is lost, and cmd or super never arrive. The binding is translated to the name
bubbletea gives that encoding and compared with what each keystroke reports.
Native v2 events that retain modifiers are also compared through their legacy
equivalent, so a binding works with either input encoding.
A `prefix+` chord arrives as two keys, so the popup holds the prefix and takes
the key that follows, the way herdr's own prefix mode does.

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

A row's name is `namespace: title`, the way an editor's command list reads,
and that is what a query matches. The namespace is where the row comes from —
`herdr` for the built-in commands, `command` for user scripts and the ones
configured under `[[keys.command]]`, the plugin's own name for a plugin action,
and the kind of thing for the rows that go somewhere. The row draws the title
first and the namespace in a column of its own at the right, next to the detail and the key:
it repeats down the list while the title is what distinguishes the row, and a
namespace in front would leave every title starting in a different column. The
matched positions index the whole name, so they are split between the two when
the row is drawn. The line under the list names the selected row in the
`namespace: title` form.

Built-in command titles use Title Case. Configured commands, scripts and plugin
actions keep the casing their authors supplied, as do pane, tab and workspace
names. Matching and sorting fold case separately from display.

## Going to what is open

`session.snapshot` is the whole session in one call, so every pane becomes a
row that focuses it with `pane.focus`. The pane, tab and workspace the palette
was opened from are rows too, so they can be closed without leaving them. They
carry `Here` and a small coloured origin marker at the right edge, separate
from the selection cursor on the left. Going there goes nowhere, so such a row
gains nothing from the recent order, sorts after the rows its score ties with,
and is not among what needs you. A plugin popup is not part of the
session's panes, so the palette's own window never appears in its list.

Every other workspace and tab is a row as well, previewed through the pane it
shows, but not in the command list. Most hold one tab or one pane, so a row for
each there would put the same place on the list two or three times, and the
pane is what a search is usually for. A pane's search text carries the names of
the workspace and tab it sits in, so either still finds it. A row's id names the
place, `workspace:`, `tab:` or `pane:`, so `back` returns to whichever the
palette went to last.

The panes sit in the command list rather than behind a command of their own:
reaching a pane by name is what the palette is used for most, and a step in
front of it costs more than the length of the list does.

## Scopes

Every row has a `Kind`: a herdr command, a configured command, a plugin action,
a pane, an agent, a tab or a workspace. A scope is a name and a set of kinds,
and what the popup lists is the rows of the scope that is on, whatever their
source. The command list is a scope too, `palette`, holding every kind but tabs
and workspaces. The others are `agents`, `panes` (agents included, since an
agent runs in a pane), `tabs`, `workspaces`, `plugins`, the actions every
installed plugin registered, `herdr`, herdr's own commands, and `commands`,
user scripts and the ones configured under `[[keys.command]]`, which their rows
already name `command:`. Searching
`herdr` in the command list is no substitute for the last: a plugin's id and a
pane's directory are searched too, and both often carry the word. A new scope
is a line naming its kinds; the label, the placeholder, the empty line and completion
follow from its name.

From the command list, `tab` on a query of one word, two letters or more, that
starts the name of a scope switches to it and empties the query; shorter, a
letter is too often the start of the name being searched for. Failing a start,
a word whose letters appear in a scope's name in order, from its first letter,
completes too, the way the list matches a query: `cmd` reaches `commands`. The
first letter has to match, or the letters of most names searched for would
spell out some scope. Each scope carries the line it shows when it holds
nothing, since why it would depends on the scope: nothing is open, no plugin is
installed, no command is configured. The footer names
the scope while `tab` would go there, since the same key answers a waiting
agent otherwise. `Switch Workspace` and `Switch Tab` are entries whose `Scope`
opens theirs in place of running anything. `backspace` on an empty query and
`esc` go back to the command list, except that `esc` closes a popup the `goto`
action opened in its scope, the way it closes the command list.

A scope is not grouped under headings: they tell commands from places, and a
scope holds rows of a kind. Its rows keep the order the groups give them, what
needs you first and where the palette went recently next. A scope of places
also leaves out the source column, since the label says what every row is and
the detail says which pane runs an agent; `plugins` keeps it for the plugin's
name.

The query starts a blank after the label, however long the label is, rather
than after room kept for the longest one. It moves when the label changes,
which is when a scope starts on an empty query or a question is up and nothing
is being typed.

A pane running an agent shows under `agent:` instead of `pane:`, with the agent
and its status next to the row, coloured by the status. A pane is addressed by
its id rather than through `agent.focus`, which takes a name the snapshot does
not carry.

The name an agent was given is not on the pane either — only the agent it runs
is — so it comes from the snapshot's agents, keyed by pane. It stands where the
agent would otherwise be, and is what the row is found by once an agent has
been renamed.

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
constantly and no row shows it. The preview beside a wide list reads the
selected pane's visible screen with `pane.read` instead, after the selection
rests for 80 ms and then once a second while it is on show. `ctrl+o` hides or
shows the panel for the current popup, without changing its query or selection.
Hiding it stops passive reads and gives the list the full width. Each read
carries a sequence number that moves on whenever the pane to show changes, so a read for
a pane already left is dropped and its refresh is not asked for again.

Every one of these rows starts with "Go To", so typing that — or `goto`, which
a subsequence match reaches as well — narrows the list to them. A pane's row
shows the name it was given, or what the program in it reports, and carries the
workspace it sits in and its directory as search text, so a project name finds
the panes inside it and the row says which one it matched.

## Why herdr's commands are a hand-written list


`plugin.action.list` covers plugin actions completely: id, title, description,
contexts. It covers a disabled plugin's actions too, with nothing on them to
say so, and `plugin.action.invoke` then refuses one with `plugin_disabled`, so
what is installed is read alongside and those actions are left out.

herdr's built-in actions have no counterpart. They exist as config keys under
`[keys]`, and the API exposes no method that runs one by name.
`command.invoke` takes a `command_id` that the schema documents as issued
through the client-shell projection, which a plugin process does not receive;
becoming a client shell would mean taking over pane presentation from the
attached client.

`internal/catalog` therefore maps each command to the socket API method with
the same effect, and `internal/palette` merges that list with the plugin
actions. The cost is that a new built-in action does not appear on its own. The
API method behind each entry is stable, so the list only needs revisiting when
herdr adds commands worth offering.

Five entries have no herdr action behind them. Splitting left and up: herdr
splits right and down only, so those split in the direction it has and then
`pane.swap` the new pane into place. Moving the pane: `pane.move` takes a
destination herdr's keys have no binding for, an existing tab or a new tab or
workspace, offered together as one list of where to move it. Swapping the pane
with another: `pane.swap` takes a source and a target, and herdr binds no key
to it; the targets are the other panes of the tab,
since herdr refuses a swap across tabs, and a refusal comes back as a reason
on a successful answer, so the palette reports it itself. Evening out pane
sizes: `layout.export` gives the tab's split tree and `layout.set_split_ratio`
sets one split by its path from the root, `false` for first and `true` for
second, with the ratio as the first half's share. A split's ratio is the count
of panes side by side in its direction on the first side over the count in
all; a subtree split the other way counts as its wider half, so its panes line
up with the ones beside it. herdr clamps a ratio to between 0.1 and 0.9.

One of the built-in actions is reproduced from the session rather than by one
call. `tab.move` takes an insert index counted in the list as it stands, and
puts the tab in front of whatever is at it, so a place on is two indexes ahead
and a place back one behind; a tab's number stays with it when it moves and is
not its position, which leaves the snapshot's order as the only thing to count.

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

## Answering a keystroke

Three things can be true of the list while it is up, and each owns the line
under it.

`pending` is set from the keystroke that chose a row until the socket answers.
It is what the line says while the answer is out, and it turns `choose` into a
no-op: a second keystroke on a slow command would run it twice, which for
anything that creates something leaves two of it.

`confirming` holds a row that cannot be undone — closing a workspace, a tab or
a pane, removing a worktree — between the keystroke that
chose it and the one that answers. `enter` runs it and anything else puts the
question away, so the answer cannot be typed into the query by mistake, and a
click on the row answers the way `enter` does. The question names the row under
the selection, so a rebuild that moves the selection off it — the pane it went
to has closed — takes the question with it rather than leaving it standing over
whatever row the selection landed on.

`choosing` is the list of targets, which is described under picking a target.

Moving the selection clears the first two, because the line they stand in is
also where the selected row and the keys are, and neither should be held there
for the rest of the popup's life.

Leaving a screen has to reach what that screen asked for. `epoch` counts the
screens walked out of; a request carries the one it was made in, and an answer
from an earlier one is dropped. Without it, `esc` out of a list of targets
whose rows were still on their way would be undone when they arrived, and `esc`
out of one whose command was still running would close the popup instead, since
the command finishing reads as a command finishing on the command list.

## Fitting a row to the popup

Every line the popup draws is measured in terminal cells rather than runes: a
wide character costs two, and a line that overruns the popup wraps, which
pushes every row below it down and puts a click on the wrong row — the list
holds commands that close somebody's work, so that is worth the care.

The title and its shortcut sit together on the left, with no shortcut slot on
a row that has none. Detail and namespace form a compact block at the right
edge, with a small gap between them. Each row reserves only its actual source
width, so a long plugin name does not create a gap in every other row. The
title can use the space left by shorter metadata, with a wider gap separating
it from that block. Shortcuts disappear first when space runs out, then
namespaces: half a key names nothing, and an
agent's state matters more than the key beside a command. Agent detail shows
a coloured state icon before the name: working `●`, blocked `!`, done `✓`,
idle `○`. Unknown states have no icon. A missing reported state is unknown,
not idle.
The underlying detail retains the full state for searching and the footer;
a match in that state highlights the icon. A long name is shortened before
the icon, so the state remains visible in a narrow detail area.

An empty query lays the list out in groups, and the list is then drawn as
lines rather than rows: a heading is a line no selection lands on. The cursor
still indexes the ranked rows, and the window's offset, the scrollbar and a
click's row are counted in lines. The line
under the list carries the selected row in full, which is what a row that had
to cut its title cannot; there the name gives way before the keys do, since the
name is also on show above.

## Closing a row without going there

A pane's, tab's or workspace's row carries a `Close` beside the `Run` that
goes there, and `ctrl+x` asks the question a command that cannot be undone
asks, with the row as its subject. `ctrl+x` because the query field binds
nothing to it. The footer names what the key closes, since a workspace takes
every pane in it along. The place the palette was opened from is a row as
well, so it closes without going anywhere first.

Unlike a command, closing keeps the popup up: the list is where the next one to
close is chosen from, and the session's events take the closed row away. It is
not written to the recent order either, which counts commands run. `closing`
beside `confirming` is what tells the second keystroke which of the two it
answers.

## Answering an agent from the palette

A blocked agent is waiting on a dialog, and `agent.prompt` refuses it with
`agent_blocked`, so answering one means sending keys. They go through
`pane.send_keys` to the agent's pane: `agent.send_keys` takes only an agent
herdr started under a name and refuses a detected one with `agent_not_ready`,
and whether the pane is waiting on an answer is the palette's own check. The
palette hands the keyboard over rather than offering a command per answer: the
answers are the agent's own, drawn on its screen, and the screen read back
with `pane.read` is what the reader answers from. The keys are sent as herdr
spells them — each character on its own, `space` by name, the named keys a
dialog is answered with, `ctrl` and a letter — and esc is kept by the palette,
since in an agent's dialog it usually means no.

Keys go out in `pane.send_keys` calls one at a time. Each call is a connection
of its own, so two in flight could land in either order; keys pressed while one
is out wait in a queue and go together in the next. An alt chord is dropped
rather than sent as its bare key, and a paste's line breaks and tabs go as
`enter` and `tab`, since herdr refuses a whole call over one key it cannot
spell.

The screen is read every 300 ms while the agent is being answered, and once
more 120 ms after each call returns. That extra read runs beside the refresh
rather than restarting it, which would hold every read off for as long as
keys keep coming, and reads are numbered so a slow one does not overwrite the
screen a later one brought.

The session's events already say when the agent's status changes, and a status
other than blocked means the dialog was answered, so that is when the list
comes back.

## Finding what a pane printed

herdr searches the scrollback of the pane you are in. Which pane something was
printed in has no answer, and that is the question worth asking when a session
holds a dozen of them.

`pane.read` on every pane, a row per non-blank line, each carrying the pane it
came from as the value a match goes to. The panes are read one at a time: a
socket client is not documented to take concurrent calls, and a session holds
few enough that the wait is one the popup already says it is having. Both the
tail per pane and the total number of rows are capped — the matcher is quick
enough for a few hundred lines of each pane, and a query needing more than that
is a query for the pane itself.

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
pane area — a popup opened from a pane of half the width still measures 80% of
the whole area, which is what makes it the area rather than the pane it is
centred in. The sidebar is outside that area, so the popup sits half the
sidebar's width right of the window's centre, and the height is the only lever
on how high up it starts. The manifest asks for 80% wide by 60% tall:
the preview is drawn only from 110 columns inside the popup and takes 40% of
them, so a narrower default leaves the list too little room for its columns
on a common terminal. The `[window]` table of the plugin's configuration
replaces both.

## Collecting a value

A command that needs a value — a rename or a branch name — gets a
popup of its own, two lines tall, rather than the palette's window. It is the
same handover, one hop longer: the palette writes the entry down with a
`prompt` and quits, `exec` opens the `input` pane for it, and the field writes
the entry back with the value and asks for `exec` again, which runs it once the
field's popup is gone in turn.

`internal/prompt` edits this small field directly, keeping the terminal's own
cursor on the insertion point. The palette's search field uses Bubble Tea v2's
native `View.Cursor`, with a blinking bar and a position measured in terminal
cells after horizontal scrolling. No caret character is inserted into the
query. Herdr can then forward the visible cursor to the outer terminal, where
input methods use it to place their candidate windows. The framework handles
cursor positioning, visibility and cleanup for the palette.

## Running a configured command

A `[[keys.command]]` entry cannot be run by name over the API, so each type is
reproduced: `shell` is started detached, and `pane` and `popup` open the
plugin's `run` pane, which execs the command line under a shell and exits with
it. The placement is what separates the two — `popup` for herdr's
popup type, and `zoomed` for its pane type, which takes over the layout the way
herdr's own temporary pane does. A zoomed pane is placed against an existing
pane and takes its id; a popup covers the active pane and herdr rejects a
target or a workspace alongside it.

## User scripts

`script_dirs` lists directories, defaulting to `["scripts"]` beside the
palette's configuration when unset. An explicit list replaces the default,
and an empty list disables discovery. Relative paths resolve against that
configuration directory and `~/` resolves against the user's home. Discovery is shallow:
regular executable files with a shebang on Unix, and `.ps1`, `.cmd` and `.bat`
files on Windows. Links to regular scripts work too; dotfiles and
directories are left out. Nothing runs during discovery.

Directories are scanned in configured order, once per directory file identity.
Resolved paths deduplicate the
same script reached through repeated directories or symbolic links; the first
reference supplies its title, id and execution path. Different files with the
same title stay separate, with paths added to their row details. Errors in one
directory do not prevent the other directories from loading.

Only the initial comments are read, up to 4 KiB of the original file. PowerShell
headers with a UTF-16 byte-order mark are decoded before reading metadata;
execution still uses the original file. `@palette.title` names the
row and `@palette.mode` selects the existing `shell`, `pane` or `popup` behavior.
Without them the filename supplies the title and the mode is `shell`. Bad
metadata leaves that script out and reports the filename alongside the valid
list. A missing directory is empty, so installing the plugin needs no setup.

Scripts are commands in the existing list and scope, with ids derived from
their absolute paths. The popup and exec entrypoint discover them the same
way, so handing one over uses its id, not copied script contents. Every script
is handed over: even a background script might ask herdr to open a popup, and
herdr cannot show one while the palette is still up. Invocation context is
preserved just as for configured commands; an absent or invalid working
directory is an error, not permission to run in the plugin's directory.

The run pane receives a script path separately from a shell command line.
Unix executes the file through its shebang. Windows runs PowerShell scripts
with `powershell.exe -File`, respecting its execution policy, and batch scripts
with cmd.exe. A batch path is passed through an environment variable with
delayed expansion disabled, so spaces, percent signs and exclamation marks in
the path are not interpreted as shell source.

Each command-list load scans the directories; typing only searches the in-memory
entries. No watcher or persistent index needs to keep a cache in sync with the
short-lived popup processes.

## Picking a target from a list

A command whose target is one of several things herdr knows about — the
worktree to open, the tab to move the pane to — asks for the list and shows it
in the palette's own window, in place of the commands. The rows are ranked and
drawn by the same code, so a target is filtered, highlighted and clicked the
way a command is.

It is the palette's window rather than the field's because nothing is typed
that the list does not already hold, and because the field is a popup of its
own: it costs the handover and a second popup to collect what a list can
answer directly. The commands come back on `esc`, filtered by the query that
led to them.

Each row carries the command's own id and the value picked. The id is what
keeps the handover working — the exec entrypoint resolves an entry against the
command list, where the row itself does not appear — and what the recent order
counts, so it remembers the command rather than the worktree it was run on. The
value travels as the entry's input, the same field a collected value uses, so
the two paths meet at `Exec.Input` and only one of them can be set on an entry.

The list is asked for off the update loop, like a command, and the rows the
session pushes in the meantime are kept for the way back rather than drawn over
the targets.

An entry may ask for both: the row carries the entry's field with it, so
picking the target opens the field for the value. The target and the value are separate all the way through — `Chosen`
beside `Input` in what is handed over, and in what an entry runs with — because
the field collects one of them and has nothing to say about the other.

## Opening the configuration

herdr's `config.toml` has no API behind it at all — the palette reads it for
the key column, the configured commands and the theme tokens — so editing it is
the editor run over the file, in the plugin's own popup, the way a configured
popup command runs. The editor is the one the environment herdr passed the
plugin names, `$VISUAL` then `$EDITOR`: that is where it is set for herdr's own
editing too. A POSIX shell resolves it as it runs the line, falling back to
`vi`; cmd.exe has no such fallback, so on Windows the palette picks it from the
same variables, falling back to notepad. The path is the one thing composed
rather than read, so it is quoted for the shell that runs the line.

## Windows

Each platform difference follows what herdr does there, so a command behaves
the same from the palette as from a key.

- A command line runs as `%ComSpec% /d /c <line>`, the line exactly as it was
  written. Go would quote it as one argument, for a program that parses its
  arguments the C runtime's way, which cmd.exe does not, so the command line
  is set whole.
- A detached command gets `CREATE_NO_WINDOW` and `CREATE_NEW_PROCESS_GROUP`.
  Closing a popup takes down what is attached to its console, and herdr ends
  the popup's process tree with `TerminateProcess`; the palette exits before
  the popup closes, so the tree it leaves is not searched, and a console of
  the command's own keeps it alive. A hidden console rather than none keeps a
  console program the command starts from opening a window.
- Whether a process is still running is its exit code, `STILL_ACTIVE`, read
  through `OpenProcess`, as herdr reads it.
- herdr's configuration is found as herdr finds it: `XDG_CONFIG_HOME`, then
  `%APPDATA%`, then `%USERPROFILE%\AppData\Roaming`, then `~/.config`.
- `./bin/palette` in the manifest runs `bin\palette.exe`: herdr resolves a
  relative command against the plugin root and the process launcher adds
  `.exe`. The build step for Windows is its own, `scripts/build.ps1` under
  Windows PowerShell 5.1, since `build.sh` needs a POSIX shell.

herdr skips SIGHUP on Windows and goes straight to `TerminateProcess`, so the
gate that holds the exit for a state write has nothing to wait on there. The
write takes far less than the quarter second herdr leaves before it ends the
process.

## Releasing

The `release` workflow builds an archive per platform — a `.tar.gz` holding
`palette`, or on Windows a `.zip` holding `palette.exe` — and publishes them
with a `checksums.txt` beside them, tagging the commit it runs on with the
version the manifest names. It commits nothing: a workflow that pushes to the
branch leaves every local checkout behind it, and a push from one of those
then has to be a merge or a force. The version is bumped by a commit of its
own before the workflow is run.

The build scripts fall back to that archive only without a Go toolchain, and
verify it against the `checksums.txt` of the same release. That catches a
download cut short or corrupted; a replaced archive and a replaced checksum
file would come from the same account, which a copy of the checksums in the
repository would not have protected against either.

## The palette's own configuration

`internal/settings` reads one file in the plugin's config directory, in a
single decode: the popup's size, colours and script directories. Commands bound
to keys stay in herdr's `[[keys.command]]`; scripts need no key binding or
separate registration. Their names and run modes live with the script itself.

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
reaches "herdr: Split Pane Right" and the highlighted letters are the ones that
were searched. fzf's scoring is what separates a run at the start of a word
from letters scattered through a row, which is the difference the palette
depends on at the size it has. Those bonuses live in tables `algo.Init` fills,
which fzf's own command line calls on start; the package does not, and
without it every position scores the same, so `internal/palette` calls it
once with fzf's default scheme.

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

An empty query scores every row the same, so the order is then whatever else
the sort has to go on: recency first, then the commands, then the rows that go
somewhere. Without the second of those the palette would open on however many
panes the session happens to hold rather than on what it is for, and a session
holds as many of those rows as it has panes. A pane just left is a row just
run, so recency still puts the way back at the top. The namespace is folded
before it is compared, since it is a name a plugin gave itself and where it
sits should not turn on how it capitalised it.

A scope ranks its rows the same way, so the places in one are ordered by how
recently the palette went there.

## Mouse

The view uses `tea.MouseModeCellMotion`, which reports clicks and the
wheel, and the pointer only while a button is down. The bare pointer is not
wanted: the selection belongs to the keyboard, and all motion mode would let a
touch of the trackpad carry it to whatever row the pointer came to rest on, so
the next `enter` would run that row rather than the one being read. A click
names its own row, so following the pointer buys nothing. herdr captures the
mouse for its own UI but forwards events to a pane app that asks for them, so
the popup receives them.

A click's row is `offset + Y - headerRows`, where `headerRows` is the query
line and the rule under it. Two things have to hold for that to name the row
under the pointer. No line the popup draws may wrap, or every row below the
wrap moves out from under the arithmetic. And `Y` has to fall inside the rows
that were drawn: the rule under the list and the line below it are inside the
popup as well, and the arithmetic alone reads them as the rows that would have
been there had the window been taller — a click on the line under the list
would run a command that is not on screen.

A click off the rows puts away a question waiting to be answered rather than
doing nothing, the way a key other than `enter` does.

## State

`recent.json` in the plugin's state directory holds the ids of the commands
last run, most recent first, capped at 50. It is a convenience: an unreadable
file only means the list opens unordered.

`attend.json` holds the pane `attend` last went to. A press from that pane is
going on to the next agent rather than leaving work, so it does not replace
the way back.
