//go:build !windows

package wrapper

import (
	"os"
	"syscall"
)

// platformSignals returns the set of signals the wrapper listens for on Unix.
// Tasks 21-25 will extend this list with SIGINT, SIGTERM, SIGHUP, SIGQUIT, SIGTSTP, SIGCONT.
func platformSignals() []os.Signal {
	return []os.Signal{
		syscall.SIGWINCH,
	}
}
