//go:build !windows

package wrapper

import (
	"os"
	"syscall"
)

// platformSignals returns the set of signals the wrapper listens for on Unix.
// Tasks 21-23 extend the list: SIGINT, SIGTERM, SIGHUP, SIGQUIT.
// Tasks 24-25 will add SIGTSTP, SIGCONT.
func platformSignals() []os.Signal {
	return []os.Signal{
		syscall.SIGWINCH,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGHUP,
		syscall.SIGQUIT,
	}
}
