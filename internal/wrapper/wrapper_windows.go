//go:build windows

package wrapper

import (
	pty "github.com/aymanbagabas/go-pty"
)

// deriveExitCode extracts the exit code from the pty.Cmd after Wait() has
// been called. On Windows, signal-killed exit codes are not applicable
// (Windows uses process exit codes directly), so we return ExitCode() as-is.
func deriveExitCode(c *pty.Cmd, waitErr error) int {
	if waitErr == nil {
		return 0
	}
	if c.ProcessState == nil {
		return -1
	}
	return c.ProcessState.ExitCode()
}
