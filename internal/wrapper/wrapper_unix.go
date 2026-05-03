//go:build !windows

package wrapper

import (
	"syscall"

	pty "github.com/aymanbagabas/go-pty"
)

// deriveExitCode extracts the exit code from the pty.Cmd after Wait() has
// been called. On Unix, if the child was killed by a signal, it returns
// 128 + signum (bash/POSIX convention). Normal exits return the actual code.
func deriveExitCode(c *pty.Cmd, waitErr error) int {
	if waitErr == nil {
		return 0
	}
	if c.ProcessState == nil {
		// Wait() failed before obtaining process status.
		return -1
	}
	if c.ProcessState.Exited() {
		// Normal exit (or explicit os.Exit call).
		return c.ProcessState.ExitCode()
	}
	// The process did not exit normally — check if it was signal-killed.
	if status, ok := c.ProcessState.Sys().(syscall.WaitStatus); ok {
		if status.Signaled() {
			// bash convention: 128 + signal number.
			return 128 + int(status.Signal())
		}
	}
	// Fallback: return whatever ExitCode gives us (may be -1).
	return c.ProcessState.ExitCode()
}
