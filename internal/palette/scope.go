package palette

import "strings"

// Kind is what a row is. A scope lists rows by their kind, so a new scope is a
// set of kinds rather than code of its own.
type Kind uint8

const (
	// KindCommand is herdr's own commands, and the zero value: a target picked
	// from a list is one too, since it runs the command it belongs to.
	KindCommand Kind = iota
	KindCustom
	KindPlugin
	KindPane
	KindAgent
	KindTab
	KindWorkspace
)

// Kinds is a set of kinds.
type Kinds uint

// KindsOf is the set holding the kinds.
func KindsOf(kinds ...Kind) Kinds {
	var set Kinds
	for _, kind := range kinds {
		set |= 1 << kind
	}
	return set
}

func (s Kinds) Has(kind Kind) bool { return s&(1<<kind) != 0 }

// places is every kind of row that focuses something open rather than running
// a command.
var places = KindsOf(KindPane, KindAgent, KindTab, KindWorkspace)

// Scope is what the popup lists: the command list, or the rows of a few kinds.
// Its name is the word tab completes to it, and its label capitalised. Empty
// is what it says when it holds nothing, which depends on why it would.
type Scope struct {
	Name  string
	Kinds Kinds
	Empty string
}

var (
	// ScopePalette is the command list the popup opens on. Tabs and workspaces
	// are left out of it: most hold a single tab or a single pane, so a row
	// for each would put the same place on the list two or three times, and a
	// pane is found by the names of the tab and workspace it sits in.
	ScopePalette    = Scope{Name: "palette", Kinds: KindsOf(KindCommand, KindCustom, KindPlugin, KindPane, KindAgent), Empty: "no command is listed"}
	ScopeAgents     = Scope{Name: "agents", Kinds: KindsOf(KindAgent), Empty: "no agent is open"}
	ScopePanes      = Scope{Name: "panes", Kinds: KindsOf(KindPane, KindAgent), Empty: "no pane is open"}
	ScopeTabs       = Scope{Name: "tabs", Kinds: KindsOf(KindTab), Empty: "no tab is open"}
	ScopeWorkspaces = Scope{Name: "workspaces", Kinds: KindsOf(KindWorkspace), Empty: "no workspace is open"}
	ScopePlugins    = Scope{Name: "plugins", Kinds: KindsOf(KindPlugin), Empty: "no plugin is installed"}
	ScopeHerdr      = Scope{Name: "herdr", Kinds: KindsOf(KindCommand), Empty: "no herdr command is listed"}
	// ScopeCommands is the commands configured under [[keys.command]], which
	// the rows name with the "command" namespace.
	ScopeCommands = Scope{Name: "commands", Kinds: KindsOf(KindCustom), Empty: "no command is configured"}
)

// scopes is every scope the command list narrows to, in the order a typed
// word is completed against.
var scopes = []Scope{ScopeAgents, ScopePanes, ScopeTabs, ScopeWorkspaces, ScopePlugins, ScopeHerdr, ScopeCommands}

// ScopeEnv names the scope the popup opens in. The action that goes straight
// to what is open sets it, so a key bound to that action reaches a pane by
// name with the commands out of the way.
const ScopeEnv = "HERDR_PALETTE_SCOPE"

// minCompletion is the shortest word tab completes to a scope. A single letter
// is too often the start of the name being searched for, and tab on such a
// query also answers a waiting agent.
const minCompletion = 2

// Complete is the scope a query of a single word, two letters or more, names:
// the one whose name it starts, or failing that the one whose name holds its
// letters in order from the same first letter, the way the list matches a
// query, so "cmd" reaches commands. The first letter has to be the name's, or
// the letters of most names searched for would spell out some scope.
func Complete(query string) (Scope, bool) {
	word := strings.ToLower(strings.TrimSpace(query))
	if len([]rune(word)) < minCompletion || strings.ContainsAny(word, " \t") {
		return Scope{}, false
	}
	for _, scope := range scopes {
		if strings.HasPrefix(scope.Name, word) {
			return scope, true
		}
	}
	for _, scope := range scopes {
		if word[0] == scope.Name[0] && subsequence(word, scope.Name) {
			return scope, true
		}
	}
	return Scope{}, false
}

// subsequence reports whether every letter of word is in name, in order.
func subsequence(word, name string) bool {
	rest := name
	for _, r := range word {
		at := strings.IndexRune(rest, r)
		if at < 0 {
			return false
		}
		rest = rest[at+len(string(r)):]
	}
	return true
}

// ParseScope reads a scope's name, as ScopeEnv carries it.
func ParseScope(name string) (Scope, bool) {
	for _, scope := range scopes {
		if scope.Name == name {
			return scope, true
		}
	}
	return Scope{}, false
}

// Label is what the popup shows while the scope is on.
func (s Scope) Label() string { return strings.ToUpper(s.Name) }

// Noun is what one row of the scope is, such as "agent".
func (s Scope) Noun() string { return strings.TrimSuffix(s.Name, "s") }

// Places reports whether the scope lists only what is open, as opposed to
// what runs.
func (s Scope) Places() bool { return s.Kinds != 0 && s.Kinds&^places == 0 }

// Rows is what the scope lists, in the order the list holds it.
func (l List) Rows(scope Scope) []Entry {
	var rows []Entry
	for _, entry := range l.All() {
		if scope.Kinds.Has(entry.Kind) {
			rows = append(rows, entry)
		}
	}
	return rows
}
