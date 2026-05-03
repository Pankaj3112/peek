//go:build !windows

package wrapper

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

// peekBinary returns the path to the pre-built peek test binary.
// The binary is built in testdata/ by TestMain before any tests run.
func peekBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join("testdata", "peek")
	if _, err := os.Stat(bin); err == nil {
		return bin
	}
	t.Skip("testdata/peek binary not found; run: go build -o internal/wrapper/testdata/peek ./cmd/peek/")
	return ""
}

// spawnPeek starts peek wrapping the given shell command as a subprocess.
// Returns the running *exec.Cmd (already started).
func spawnPeek(t *testing.T, shellCmd string) *exec.Cmd {
	t.Helper()
	bin := peekBinary(t)
	home := t.TempDir()

	cmd := exec.Command(bin, "--", "bash", "-c", shellCmd)
	cmd.Env = append(os.Environ(), "HOME="+home)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Put the subprocess in its own process group so signals we send to it
	// don't also hit the test process itself.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawnPeek: start failed: %v", err)
	}
	return cmd
}

// waitOrTimeout waits for cmd to exit within the given timeout.
// Returns (exit code, true) on success, (-1, false) on timeout.
func waitOrTimeout(cmd *exec.Cmd, timeout time.Duration) (int, bool) {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			return 0, true
		}
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode(), true
		}
		return -1, true
	case <-time.After(timeout):
		return -1, false
	}
}

// skipIfBashUnavailable skips the test on platforms without bash.
func skipIfBashUnavailable(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
}

// TestSIGINTHandler tests the unit-level forwardWithGraceful for SIGINT.
// It uses a mock process so no real subprocess is spawned.
func TestSIGINTHandler(t *testing.T) {
	// Spawn a real short-lived process so we have a valid *os.Process handle.
	// The process will likely exit immediately; Signal will fail but we ignore errors.
	dummy := exec.Command("true")
	if err := dummy.Start(); err != nil {
		t.Fatalf("start dummy: %v", err)
	}
	proc := dummy.Process
	_ = dummy.Wait()

	state := &signalState{proc: proc}

	if state.gracefulPending {
		t.Fatal("precondition: gracefulPending should be false")
	}

	// Override timeout to a tiny value so the timer fires quickly in tests.
	origTimeout := gracefulShutdownTimeout
	gracefulShutdownTimeout = 200 * time.Millisecond
	defer func() { gracefulShutdownTimeout = origTimeout }()

	forwardWithGraceful(syscall.SIGINT, state)

	state.gracefulMu.Lock()
	pending := state.gracefulPending
	hasTimer := state.gracefulTimer != nil
	state.gracefulMu.Unlock()

	if !pending {
		t.Error("gracefulPending should be true after first SIGINT")
	}
	if !hasTimer {
		t.Error("gracefulTimer should be set after first SIGINT")
	}
}

// TestSIGINTGraceful sends SIGINT to the wrapper wrapping a bash process that
// ignores SIGINT (non-interactive bash with a child process).  The wrapper
// should forward SIGINT to the child, and when the child doesn't exit within
// the 5s graceful timeout the wrapper escalates to SIGKILL and the whole thing
// exits within ~6s.
//
// We use a compressed graceful timeout (500ms) via the testdata binary? No —
// we cannot configure the binary from outside.  Instead we verify that the
// wrapper exits within gracefulShutdownTimeout + 2s grace.
func TestSIGINTGraceful(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only")
	}
	skipIfBashUnavailable(t)

	// Child: bash that ignores SIGINT explicitly.
	// bash in non-interactive mode already effectively ignores SIGINT on the shell
	// level (the external sleep still gets it via the process group), but let's
	// be explicit: trap '' INT means the shell will ignore it, and sleep's default
	// SIGINT handling means sleep exits, but bash may restart the next command.
	// The real test: after 5s graceful timer, wrapper SIGKILLs the child and exits.
	shellCmd := `trap '' INT TERM; sleep 30`
	cmd := spawnPeek(t, shellCmd)
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	// Give child a moment to start and set up the trap.
	time.Sleep(300 * time.Millisecond)

	start := time.Now()

	// Send SIGINT to the wrapper process.
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("send SIGINT: %v", err)
	}

	// The child ignores both INT and TERM, so the 5s timer should fire → SIGKILL.
	// Wait up to 8s (5s timer + 3s margin).
	_, ok := waitOrTimeout(cmd, 8*time.Second)
	elapsed := time.Since(start)

	if !ok {
		t.Fatalf("wrapper did not exit within 8s after SIGINT (child ignores INT+TERM)")
	}
	t.Logf("wrapper exited after %v (expected ~5s graceful timeout)", elapsed)

	// Sanity: should NOT have exited instantly (that would mean no forwarding at all).
	if elapsed < 500*time.Millisecond {
		t.Errorf("wrapper exited too fast (%v); should wait ~5s for graceful timeout", elapsed)
	}
}

// TestSIGTERMTimeoutEscalates spawns a subprocess that ignores SIGTERM.
// After the 5s graceful timeout, the wrapper should SIGKILL the child.
// Total time should be ~5s; we wait up to 8s to be safe.
func TestSIGTERMTimeoutEscalates(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only")
	}
	skipIfBashUnavailable(t)

	// Child ignores SIGTERM and sleeps forever.
	shellCmd := `trap '' TERM; sleep 30`
	cmd := spawnPeek(t, shellCmd)
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	// Give child a moment to set up the trap.
	time.Sleep(300 * time.Millisecond)

	start := time.Now()
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}

	// Wait up to 8s (5s timer + 3s margin).
	_, ok := waitOrTimeout(cmd, 8*time.Second)
	elapsed := time.Since(start)
	if !ok {
		t.Fatalf("wrapper did not exit within 8s after SIGTERM (ignoring child)")
	}
	t.Logf("wrapper exited after %v (expected ~5s)", elapsed)
	// Confirm the wrapper did NOT exit in < 1s (it should have waited for timer).
	if elapsed < 1*time.Second {
		t.Errorf("wrapper exited too fast (%v); SIGKILL escalation timer may not be working", elapsed)
	}
}
