//go:build windows

package palette

import (
	"context"
	"os"
	"os/exec"
	"syscall"
)

// ShellCommand runs a configured command line the way herdr does on Windows,
// through cmd.exe /d /c. The command line is set whole: the arguments Go would
// quote for it are quoted for a program that parses them the C runtime's way,
// which cmd.exe does not.
func ShellCommand(ctx context.Context, line string) *exec.Cmd {
	shell := comSpec(os.Getenv)
	cmd := exec.CommandContext(ctx, shell)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: windowsCommandLine(shell, line)}
	return cmd
}
