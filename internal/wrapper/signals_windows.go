//go:build windows

package wrapper

import "os"

// platformSignals returns the set of signals the wrapper listens for on Windows.
// Windows does not have SIGWINCH. Other signals are added in Tasks 21-25.
func platformSignals() []os.Signal {
	return nil
}
