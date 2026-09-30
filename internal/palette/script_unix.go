//go:build !windows

package palette

import (
	"context"
	"os/exec"
)

func scriptCommand(ctx context.Context, path string) *exec.Cmd {
	return exec.CommandContext(ctx, path)
}
