package platform

import (
	"os"
	"os/exec"
	"testing"
)

func TestIsAlive(t *testing.T) {
	if !IsAlive(os.Getpid()) {
		t.Errorf("expected current process (pid=%d) to be alive", os.Getpid())
	}

	// Use a clearly-not-alive PID. PID 99999999 is reserved/unallocated on virtually all systems.
	// If the test runs on a system where this PID happens to exist, switch to a fork+kill+wait helper.
	if IsAlive(99999999) {
		t.Errorf("expected pid 99999999 to not be alive (probably)")
	}
}

func TestIsAliveDeadProcess(t *testing.T) {
	cmd := exec.Command("true") // exits immediately
	if err := cmd.Run(); err != nil {
		t.Fatalf("run true: %v", err)
	}
	pid := cmd.Process.Pid
	// After Wait (implicit in Run), the process is reaped on Unix. Subsequent IsAlive should be false.
	if IsAlive(pid) {
		t.Errorf("reaped process pid=%d should not be alive", pid)
	}
}
