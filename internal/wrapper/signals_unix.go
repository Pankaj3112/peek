//go:build !windows

package wrapper

import (
	"os"
	"syscall"
)

// platformSignals returns the set of signals the wrapper listens for on Unix.
// Tasks 21-23 added SIGINT, SIGTERM, SIGHUP, SIGQUIT.
// Task 24 adds SIGTSTP, SIGCONT.
func platformSignals() []os.Signal {
	return []os.Signal{
		syscall.SIGWINCH,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGHUP,
		syscall.SIGQUIT,
		syscall.SIGTSTP,
		syscall.SIGCONT,
	}
}
