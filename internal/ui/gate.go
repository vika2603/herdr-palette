package ui

import "sync"

// stateGate keeps the process from exiting in the middle of writing the
// palette's state. A command can take away the tab the popup is over, and
// herdr then closes the popup: its SIGHUP cancels the program's context, the
// program returns without waiting for the command still running, and main
// exits. A write cut short there leaves its temporary file behind and the
// file itself unwritten.
//
// Closing waits for a write in progress and turns away any that come after,
// which is well inside the half second herdr allows before it kills.
type stateGate struct {
	mu     sync.Mutex
	closed bool
}

// write runs f unless the gate has closed.
func (g *stateGate) write(f func()) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.closed {
		f()
	}
}

func (g *stateGate) close() {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
}
