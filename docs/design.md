# Design

## Eight entrypoints, one binary

The manifest declares five actions, `open`, `goto`, `toggle`, `back` and
`exec`, and three panes, `palette`, `input` and `run`. All eight run
`bin/palette`; `plugin.Env` tells the process which entrypoint started it.

`open` does one thing: it calls `plugin.pane.open` for the `palette`
entrypoint, which runs the TUI. `goto` opens the same pane with `HERDR_PALETTE_GOES`
set, which is the popup starting with the query that leaves the commands out —
a key for reaching a pane by name, separate from the key for running a command.
`back` goes to the last place the palette took you, with no popup at all.
`exec` runs a command the popup handed over,
`input` is the field that collects a value a command needs, and `run` is the
pane a configured command runs in. All three are described below.

`cmd/palette` assembles the command list for the two entrypoints that need one,
so the popup and the handover work from the same list; `RunPending` resolves
the handed-over entry by id against it.

## Closing the popup with the key that opened it

`toggle` opens the popup the way `open` does; what makes it a toggle is in the
popup. herdr's client hands every key press to the popup while one is up,
ahead of its own binding lookup — `route_key_press` in
`src/client/shell/input.rs` of herdr 0.9.0 returns the popup as the target
before direct bindings or the prefix are considered — so a second press of
the key never invokes the action. It arrives inside the popup as terminal
input, and the popup quits when it is the key bound to `toggle`.

Which key that is comes from the same configuration the key column is read
from. The popup asks for no keyboard protocol, so herdr encodes a key for it
the way a legacy terminal does: alt is an escape in front, ctrl with a
character is the control character, shift held with ctrl has no encoding and
is lost, and cmd or super never arrive. The binding is translated to the name
bubbletea gives that encoding and compared with what each keystroke reports.
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

These rows sit in the command list rather than behind a command of their own:
reaching a pane by name is what the palette is used for most, and a step in
front of it costs more than the length of the list does. A session does hold
far more panes than there are commands, so they carry `Goes`, and the `@`
prefix ranks only those. The prefix is read where the query is ranked rather
than as a mode of its own, so nothing else in the popup has to know about it,
and deleting the character undoes it. The `goto` action opens the popup with
that character already typed.

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
constantly and no row shows it.

Every one of these rows starts with "go to", so typing that — or `goto`, which
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

Three entries have no herdr action behind them. Splitting left and up: herdr
splits right and down only, so those split in the direction it has and then
`pane.swap` the new pane into place. Moving the pane to another tab: `pane.move`
takes a destination herdr's keys have no binding for.

Two of the built-in actions are reproduced from the session rather than by one
call. `tab.move` takes an insert index counted in the list as it stands, and
puts the tab in front of whatever is at it, so a place on is two indexes ahead
and a place back one behind; a tab's number stays with it when it moves and is
not its position, which leaves the snapshot's order as the only thing to count.
The pane cycle has no method at all, so it focuses the pane before or after the
focused one among the panes the snapshot lists for the tab.

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
a pane, removing a worktree, forgetting a layout — between the keystroke that
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

What a row spends its width on is ordered. What the row is comes first, then
the detail that tells two rows of the same name apart, then the key. The key
column is as wide as the widest key on show and is left out entirely below the
width where the rows would have too little left: half a key names nothing, and
what an agent is doing is worth more than the key beside a command. The line
under the list carries the selected row in full, which is what a row that had
to cut its title cannot; there the name gives way before the keys do, since the
name is also on show above.

## Handing an agent something to look at

herdr carries the selection, reads a pane's output and prompts an agent, but
has no path between them: text on screen reaches an agent by being copied
there. The palette is where that path costs nothing to add — it is already the
place that picks a target and collects a value.

The selection travels in the invocation context, so the text is the one that
was selected when the key opened the palette rather than whatever is selected
by the time the command runs, which matters because the command runs after the
popup has taken the keyboard. `NeedsSelection` keeps the row out of the list
when there is none, and the entry checks again when it runs: the context it is
handed is not the one the list was filtered against.

A pane's output is read with `pane.read` at `recent_unwrapped`, so a line that
ran past the pane's width is the one line it is, and with `strip_ansi`, since
the colours are not what the agent is being asked about. The tail is bounded:
enough for a stack trace, and the agent reads the pane itself if it needs more.

The question goes in front of what was collected, because it is what the agent
is being asked to do with what follows and what follows can run to hundreds of
lines.

Watching an agent is the one command whose work outlasts the popup by design.
`agent.wait` blocks until the agent reaches a state that is not `working`, so
it is marked `AlwaysRelay` and runs in the `exec` entrypoint, where there is no
popup to hold open. That also means there is nowhere to report to, so the
notification is the whole point rather than a report on the way out, and the
wait carries a timeout: an agent left running overnight should not leave a
process behind it.

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

## Bringing a layout back with its agents

An exported tree carries a pane's directory but not what is running in it, so
an arrangement applied again comes back as a row of shells. What each pane was
running is read from the session when the layout is saved and written down
beside the tree.

Lining them up again relies on the tree being the same shape both times:
`herdr.LayoutPanes` walks it first-before-second at every split, so the nth
pane of the saved tree is the nth pane of the applied one, and `layout.apply`
answers with the tree it opened, ids and all. A layout saved before there was
anywhere to write the agents down has none, which reads as a layout of plain
panes.

The arrangement is open by the time the agents are started, so an agent that
will not start is reported without taking the tab down with it.

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
on how high up it starts. The manifest asks for 60% by 60%, which the
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

A list may also stay up after a row of it runs, which is what makes managing
plugins a screen rather than a pair of commands: the rows carry the state, so
running one and closing over it would hide what it did. The list is asked for
again instead, and the rows are replaced under the query and the selection they
were picked with, so the next keystroke lands where the last one did. `tab`
runs a row there, which is what turning one over reads as; it does nothing in
the command list, where there is no state on a row to turn.

An entry may ask for both, which is what prompting an agent by name does: the
row carries the entry's field with it, so picking the agent opens the field for
the text. The target and the value are separate all the way through — `Chosen`
beside `Input` in what is handed over, and in what an entry runs with — because
the field collects one of them and has nothing to say about the other.

## Opening the configuration

herdr's `config.toml` has no API behind it at all — the palette reads it for
the key column, the configured commands and the theme tokens — so editing it is
the editor run over the file, in the plugin's own popup, the way a configured
popup command runs. The editor is named by the shell that runs the line, from
the environment herdr passed the plugin, rather than resolved here: that is
where `$EDITOR` is set for herdr's own editing too. The path is the one thing
composed rather than read, so it is quoted for that shell.

## Saving a tab's layout

`layout.export` answers with a tab's arrangement and `layout.apply` opens one,
but herdr keeps none: an arrangement lives as long as the tab does. So the
palette keeps them itself, in `layouts.json` in its state directory, most
recently saved first, replacing a layout of the same name.

The pane ids are dropped on the way in. An exported tree names the panes it
came from, and a saved layout outlives them: applying one with the ids left in
would move those panes rather than open the arrangement again. What is left is
the splits, their ratios and each pane's directory, which is what herdr needs
to build it.

It is opened in a new tab of the focused workspace rather than over the current
one, whose panes are somebody's work, and the tab takes the layout's name.

A layout node is a union herdr's client decodes by a type tag, and the decoder
is reachable only through the apply parameters, so a saved tree is stored as it
was written and handed back through those. That also leaves a tree this plugin
does not understand listed and openable rather than lost.

Reaching the state directory is why `Exec` carries the entrypoint's
environment: it is what every other state the plugin keeps is read and written
through, and the two paths that run an entry — the popup and the exec
entrypoint — both have one.

## The palette's own configuration

`internal/settings` reads one file in the plugin's config directory, in a
single decode: the popup's size and the colours. It carries what herdr's API
does not publish and nothing else — a command belongs in herdr's own
`[[keys.command]]`, which the list already reads, rather than in a second place
that would compete with it.

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

An empty query scores every row the same, so the order is then whatever else
the sort has to go on: recency first, then the commands, then the rows that go
somewhere. Without the second of those the palette would open on however many
panes the session happens to hold rather than on what it is for, and a session
holds as many of those rows as it has panes. A pane just left is a row just
run, so recency still puts the way back at the top. The namespace is folded
before it is compared, since it is a name a plugin gave itself and where it
sits should not turn on how it capitalised it.

`@` in front of the query ranks only the rows that carry `Goes`, which is
every row the session put in the list. It is read where the query is ranked
rather than kept as a mode, so nothing else in the popup has to know about it
and deleting the character undoes it.

## Mouse

The program runs with `tea.WithMouseCellMotion`, which reports clicks and the
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
