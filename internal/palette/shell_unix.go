//go:build !windows

package palette

import (
	"context"
	"os"
	"os/exec"
)

// ShellCommand runs a configured command line the way herdr does, handed whole
// to a shell rather than split here.
func ShellCommand(ctx context.Context, line string) *exec.Cmd {
	return exec.CommandContext(ctx, shell(), "-c", line)
}

// shell is the user's shell, which is what herdr runs a command line under.
func shell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}
