//go:build !windows

package palette

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

// Shell is what a configured command line runs under, matching herdr, which
// hands the string to a shell rather than splitting it itself.
func Shell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}

// ShellCommand runs a configured command line the way herdr does.
func ShellCommand(ctx context.Context, line string) *exec.Cmd {
	return exec.CommandContext(ctx, Shell(), "-c", line)
}

// editorLine opens path in the editor the environment names.
func editorLine(path string) string {
	return "${VISUAL:-${EDITOR:-vi}} " + shellQuote(path)
}

// shellQuote wraps a path for the shell that runs the command line, which is
// the one thing about it this plugin composes rather than reads.
func shellQuote(path string) string {
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}
