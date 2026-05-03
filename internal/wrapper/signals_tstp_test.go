//go:build !windows

package wrapper

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// findChildPid returns the first child PID of parentPid using pgrep -P.
// Returns 0 if no child is found or on error.
func findChildPid(t *testing.T, parentPid int) int {
	t.Helper()
	if parentPid == 0 {
		return 0
	}
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(parentPid)).Output()
	if err != nil {
		// pgrep returns exit code 1 when no match; not necessarily an error.
		return 0
	}
	lines := strings.Fields(strings.TrimSpace(string(out)))
	if len(lines) == 0 {
		return 0
	}
	pid, err := strconv.Atoi(lines[0])
	if err != nil {
		t.Logf("findChildPid: parse error: %v", err)
		return 0
	}
	return pid
}

// isProcessStopped returns true if the process with the given PID is in a
// stopped state. Uses `ps -o stat= -p <pid>`.
// On macOS/Linux, a stopped process reports state beginning with "T".
func isProcessStopped(t *testing.T, pid int) bool {
	t.Helper()
	out, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		// Process may have already exited; treat as not-stopped.
		t.Logf("isProcessStopped(%d): ps error: %v", pid, err)
		return false
	}
	state := strings.TrimSpace(string(out))
	t.Logf("isProcessStopped(%d): state=%q", pid, state)
	return strings.HasPrefix(state, "T")
}

// waitForChild polls for a child PID of parentPid up to maxWait, returning
// the first child found or 0 if none appears in time.
func waitForChild(t *testing.T, parentPid int, maxWait time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		pid := findChildPid(t, parentPid)
		if pid != 0 {
			return pid
		}
		time.Sleep(50 * time.Millisecond)
	}
	return 0
}

// TestSIGTSTPSuspendsBoth verifies that SIGTSTP forwarded to the wrapper
// suspends both the wrapper process and its child, and SIGCONT resumes both.
func TestSIGTSTPSuspendsBoth(t *testing.T) {
	// This test is integration-only: requires a real process tree.
	// Not marked parallel to avoid process-state races with other tests.
	skipIfBashUnavailable(t)

	bin := peekBinary(t)
	home := t.TempDir()

	cmd := exec.Command(bin, "--", "sleep", "30")
	cmd.Env = append(os.Environ(), "HOME="+home)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Use a new process group so SIGTSTP doesn't propagate to the test process itself.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start peek: %v", err)
	}
	wrapperPid := cmd.Process.Pid
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})

	// Wait for the direct child of the wrapper (sleep) to appear.
	// give up to 2s for the pty child to start.
	childPid := waitForChild(t, wrapperPid, 2*time.Second)
	if childPid == 0 {
		t.Skip("could not find child process within 2s; skipping SIGTSTP test")
	}

	// If the child itself has a child (e.g. peek spawns bash which spawns sleep),
	// use the grandchild as the observable process.
	if grandchild := findChildPid(t, childPid); grandchild != 0 {
		t.Logf("using grandchild pid %d (child=%d, wrapper=%d)", grandchild, childPid, wrapperPid)
		childPid = grandchild
	} else {
		t.Logf("using child pid %d (wrapper=%d)", childPid, wrapperPid)
	}

	fmt.Printf("TestSIGTSTPSuspendsBoth: wrapper=%d child=%d\n", wrapperPid, childPid)

	// Send SIGTSTP to the wrapper process.
	if err := cmd.Process.Signal(syscall.SIGTSTP); err != nil {
		t.Fatalf("send SIGTSTP: %v", err)
	}

	// Give signals time to propagate and processes to enter stopped state.
	time.Sleep(400 * time.Millisecond)

	// Assert: wrapper is stopped.
	if !isProcessStopped(t, wrapperPid) {
		t.Errorf("wrapper (pid=%d) not in stopped state after SIGTSTP", wrapperPid)
	}
	// Assert: child is stopped.
	if !isProcessStopped(t, childPid) {
		t.Errorf("child (pid=%d) not in stopped state after SIGTSTP", childPid)
	}

	// Send SIGCONT to the wrapper to resume.
	if err := cmd.Process.Signal(syscall.SIGCONT); err != nil {
		t.Fatalf("send SIGCONT: %v", err)
	}

	// Give processes time to resume.
	time.Sleep(400 * time.Millisecond)

	// Assert: neither is stopped any more.
	if isProcessStopped(t, wrapperPid) {
		t.Errorf("wrapper (pid=%d) still stopped after SIGCONT", wrapperPid)
	}
	if isProcessStopped(t, childPid) {
		t.Errorf("child (pid=%d) still stopped after SIGCONT", childPid)
	}

	// Cleanup: kill the wrapper (t.Cleanup handles this too, but be explicit).
	_ = cmd.Process.Signal(syscall.SIGKILL)
	_ = cmd.Wait()
	// Nil out so t.Cleanup doesn't double-kill.
	cmd.Process = nil
}
