package palette

import (
	"github.com/vika2603/herdr-client/plugin"
)

// recentFile is where the run order lives, under the plugin's state
// directory. It is a convenience, not a record worth recovering: a missing or
// unreadable file just means the list starts unordered.
const recentFile = "recent.json"

// recentKept is how many ids the file holds. Entries past the ranking bonus
// only matter for keeping an id from dropping out during a burst of other
// commands.
const recentKept = 50

type recentState struct {
	IDs []string `json:"ids"`
}

// ReadRecent returns entry ids, most recently run first.
func ReadRecent(env *plugin.Env) []string {
	var state recentState
	if err := env.ReadStateJSON(recentFile, &state); err != nil {
		return nil
	}
	return state.IDs
}

// WriteRecent moves id to the front and saves the order.
func WriteRecent(env *plugin.Env, id string, recent []string) error {
	ids := make([]string, 0, len(recent)+1)
	ids = append(ids, id)
	for _, existing := range recent {
		if existing == id {
			continue
		}
		ids = append(ids, existing)
		if len(ids) == recentKept {
			break
		}
	}
	return env.WriteStateJSON(recentFile, recentState{IDs: ids})
}
