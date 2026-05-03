//go:build !windows

package platform

import (
	"errors"
	"os"
	"syscall"
)

// IsAlive returns true if a process with the given PID is currently running
// and visible to the caller. Returns false if the process doesn't exist OR
// has been reaped. Returns true if the process exists but we lack permission
// to signal it (treats EPERM as "alive — exists but foreign").
func IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	if errors.Is(err, syscall.ESRCH) {
		return false // No such process.
	}
	if errors.Is(err, syscall.EPERM) {
		return true // Process exists but we can't signal it.
	}
	return false
}
