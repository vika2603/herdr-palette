package palette

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func scriptCommand(ctx context.Context, path string) *exec.Cmd {
	if strings.EqualFold(filepath.Ext(path), ".ps1") {
		return exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-File", path)
	}
	// cmd expands this environment variable once, inside quotes. Inlining a
	// path instead would interpret percent signs in a perfectly valid filename.
	// Delayed expansion is disabled so exclamation marks also remain literal.
	shell := comSpec(os.Getenv)
	cmd := exec.CommandContext(ctx, shell)
	cmd.Env = append(cmd.Environ(), ScriptEnv+"="+path)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `"` + shell + `" /d /v:off /s /c ""%` + ScriptEnv + `%""`}
	return cmd
}
