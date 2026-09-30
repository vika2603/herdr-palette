package palette

import (
	"slices"
	"testing"
)

func TestCompletingAScope(t *testing.T) {
	for query, want := range map[string]Scope{
		"ag":         ScopeAgents,
		"Pane":       ScopePanes,
		" tabs ":     ScopeTabs,
		"workspaces": ScopeWorkspaces,
		"pl":         ScopePlugins,
		"HE":         ScopeHerdr,
		"comm":       ScopeCommands,
		"cmd":        ScopeCommands,
		"wsp":        ScopeWorkspaces,
		"tb":         ScopeTabs,
	} {
		if got, ok := Complete(query); !ok || got != want {
			t.Errorf("Complete(%q) = %q, %v, want %q", query, got.Name, ok, want.Name)
		}
	}
	// One letter is the start of too many names, a second word is a query,
	// and the command list is not a scope to narrow to.
	for _, query := range []string{"", "a", "pane x", "panesx", "palette", "cld", "dmc"} {
		if got, ok := Complete(query); ok {
			t.Errorf("Complete(%q) = %q, want no scope", query, got.Name)
		}
	}
}

func TestAScopeListsItsKinds(t *testing.T) {
	list := List{
		Commands: []Entry{{ID: "herdr"}, {ID: "custom", Kind: KindCustom}, {ID: "plugin", Kind: KindPlugin}},
		Session: Session{Entries: []Entry{
			{ID: "agent", Kind: KindAgent},
			{ID: "pane", Kind: KindPane},
			{ID: "tab", Kind: KindTab},
			{ID: "workspace", Kind: KindWorkspace},
		}},
	}
	for _, c := range []struct {
		scope Scope
		want  []string
	}{
		{ScopePalette, []string{"herdr", "custom", "plugin", "agent", "pane"}},
		{ScopeAgents, []string{"agent"}},
		{ScopePanes, []string{"agent", "pane"}},
		{ScopeTabs, []string{"tab"}},
		{ScopeWorkspaces, []string{"workspace"}},
		{ScopePlugins, []string{"plugin"}},
		{ScopeHerdr, []string{"herdr"}},
		{ScopeCommands, []string{"custom"}},
	} {
		var got []string
		for _, entry := range list.Rows(c.scope) {
			got = append(got, entry.ID)
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%s lists %v, want %v", c.scope.Name, got, c.want)
		}
	}
}
