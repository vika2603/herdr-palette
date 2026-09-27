//go:build windows

package palette

import (
	"context"
	"os"
	"os/exec"
	"syscall"
)

// ShellCommand runs a configured command line the way herdr does on Windows,
// through cmd.exe /d /c. The line is passed verbatim: cmd.exe does its own
// parsing, which Go's argument quoting would break.
func ShellCommand(ctx context.Context, line string) *exec.Cmd {
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = "cmd.exe"
	}
	cmd := exec.CommandContext(ctx, comspec)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd.exe /d /c ` + line}
	return cmd
}

// editorLine opens path in the editor the environment names, resolved here
// because cmd.exe has no default-value expansion.
func editorLine(path string) string {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "notepad"
	}
	return editor + ` "` + path + `"`
}
