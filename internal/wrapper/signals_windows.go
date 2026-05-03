//go:build windows

package wrapper

import (
	"os"
	"syscall"
)

// platformSignals returns the set of signals the wrapper listens for on Windows.
// Windows does not have SIGWINCH, SIGTSTP, or SIGCONT.
func platformSignals() []os.Signal {
	return nil
}

// syscallSIGTSTP returns a sentinel signal value that never matches on Windows.
func syscallSIGTSTP() os.Signal { return os.Signal(syscall.Signal(0)) }

// syscallSIGCONT returns a sentinel signal value that never matches on Windows.
func syscallSIGCONT() os.Signal { return os.Signal(syscall.Signal(0)) }

// handleTSTP is a no-op on Windows (SIGTSTP does not exist).
func handleTSTP(state *signalState) {}

// handleCONT is a no-op on Windows (SIGCONT does not exist).
func handleCONT(state *signalState) {}
