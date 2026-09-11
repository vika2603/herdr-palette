package palette

import (
	"slices"
	"testing"

	"github.com/vika2603/herdr-client/plugin/plugintest"
)

func TestWriteRecentMovesTheIDToTheFront(t *testing.T) {
	env := plugintest.Env(plugintest.StateDir(t.TempDir()))

	if err := WriteRecent(env, "b", []string{"a", "b", "c"}); err != nil {
		t.Fatalf("WriteRecent() = %v", err)
	}
	want := []string{"b", "a", "c"}
	if got := ReadRecent(env); !slices.Equal(got, want) {
		t.Errorf("ReadRecent() = %v, want %v", got, want)
	}
}

func TestReadRecentWithoutAFile(t *testing.T) {
	env := plugintest.Env(plugintest.StateDir(t.TempDir()))
	if got := ReadRecent(env); len(got) != 0 {
		t.Errorf("ReadRecent() = %v, want nothing before the first run", got)
	}
}

func TestWriteRecentCapsTheFile(t *testing.T) {
	env := plugintest.Env(plugintest.StateDir(t.TempDir()))

	recent := make([]string, 0, recentKept*2)
	for i := range recentKept * 2 {
		recent = append(recent, string(rune('a'+i%26))+string(rune('0'+i/26)))
	}
	if err := WriteRecent(env, "new", recent); err != nil {
		t.Fatalf("WriteRecent() = %v", err)
	}
	if got := ReadRecent(env); len(got) != recentKept {
		t.Errorf("ReadRecent() kept %d ids, want %d", len(got), recentKept)
	}
}
