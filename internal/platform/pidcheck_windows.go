//go:build windows

package platform

import (
	"golang.org/x/sys/windows"
)

const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000

func IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	return true
}
