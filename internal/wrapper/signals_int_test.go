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

// TestDoubleSIGINTSkipsTimer sends SIGINT twice to the wrapper wrapping a child
// that ignores both INT and TERM.  The second SIGINT should cancel the graceful
// timer and SIGKILL the child immediately, so the wrapper exits well under the
// 5-second graceful window.
func TestDoubleSIGINTSkipsTimer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only")
	}
	skipIfBashUnavailable(t)

	// Child ignores both SIGINT and SIGTERM, so only SIGKILL will terminate it.
	shellCmd := `trap '' INT TERM; sleep 30`
	cmd := spawnPeek(t, shellCmd)
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	// Give child a moment to set up the trap.
	time.Sleep(300 * time.Millisecond)

	start := time.Now()

	// First SIGINT: starts the 5s graceful timer.
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("send first SIGINT: %v", err)
	}

	// Wait 200ms (well within the 5s window) then send the second SIGINT.
	time.Sleep(200 * time.Millisecond)

	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		// Process may have already exited if something went wrong.
		t.Logf("send second SIGINT: %v (may be already dead)", err)
	}

	// The second SIGINT should cancel the timer and SIGKILL immediately.
	// Wrapper should exit within ~1s (well under the 5s timer).
	_, ok := waitOrTimeout(cmd, 3*time.Second)
	elapsed := time.Since(start)

	if !ok {
		t.Fatalf("wrapper did not exit within 3s after double SIGINT (want immediate SIGKILL)")
	}
	t.Logf("wrapper exited after %v (expected << 5s)", elapsed)

	// Must exit well under the 5s graceful timeout.
	if elapsed >= 4*time.Second {
		t.Errorf("wrapper took too long (%v); double-INT should skip the 5s timer", elapsed)
	}
}

// TestDoubleSIGINTUnit verifies the state machine: second forwardWithGraceful call
// cancels the timer and calls SIGKILL on a mock process.
func TestDoubleSIGINTUnit(t *testing.T) {
	// Spawn a long-lived process we can signal safely.
	dummy := exec.Command("sleep", "60")
	if err := dummy.Start(); err != nil {
		t.Fatalf("start dummy sleep: %v", err)
	}
	// Cleanup: kill the process if still alive; ignore double-wait errors.
	t.Cleanup(func() { _ = dummy.Process.Kill() })

	origTimeout := gracefulShutdownTimeout
	gracefulShutdownTimeout = 10 * time.Second // long enough to not fire during test
	defer func() { gracefulShutdownTimeout = origTimeout }()

	state := &signalState{proc: dummy.Process}

	// First signal: sets up timer.
	forwardWithGraceful(syscall.SIGINT, state)

	state.gracefulMu.Lock()
	timerAfterFirst := state.gracefulTimer
	pendingAfterFirst := state.gracefulPending
	state.gracefulMu.Unlock()

	if !pendingAfterFirst {
		t.Error("gracefulPending should be true after first SIGINT")
	}
	if timerAfterFirst == nil {
		t.Error("gracefulTimer should be set after first SIGINT")
	}

	// Second signal: should cancel timer and SIGKILL.
	forwardWithGraceful(syscall.SIGINT, state)

	state.gracefulMu.Lock()
	timerAfterSecond := state.gracefulTimer
	state.gracefulMu.Unlock()

	if timerAfterSecond != nil {
		t.Error("gracefulTimer should be nil after second SIGINT (cancelled)")
	}

	// The dummy process should be dead now (SIGKILL was sent).
	// Wait for it to actually exit; use a channel so we can timeout.
	waitDone := make(chan error, 1)
	go func() { waitDone <- dummy.Wait() }()
	select {
	case waitErr := <-waitDone:
		// Expected: killed process returns an error.
		t.Logf("dummy process exited after SIGKILL: %v", waitErr)
	case <-time.After(2 * time.Second):
		t.Error("dummy process still alive 2s after double SIGINT (expected SIGKILL)")
	}
}

// ---------- Task 23: SIGHUP and SIGQUIT ----------

// TestSIGHUPBehavesLikeSIGTERM sends SIGHUP to the wrapper wrapping a child
// that ignores SIGHUP.  SIGHUP is treated like SIGTERM: after the 5s graceful
// timeout the wrapper escalates to SIGKILL.
func TestSIGHUPBehavesLikeSIGTERM(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only")
	}
	skipIfBashUnavailable(t)

	// Child ignores HUP so only SIGKILL will kill it.
	shellCmd := `trap '' HUP; sleep 30`
	cmd := spawnPeek(t, shellCmd)
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	time.Sleep(300 * time.Millisecond)

	start := time.Now()
	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatalf("send SIGHUP: %v", err)
	}

	// Wait up to 8s (5s timer + 3s margin).
	_, ok := waitOrTimeout(cmd, 8*time.Second)
	elapsed := time.Since(start)
	if !ok {
		t.Fatalf("wrapper did not exit within 8s after SIGHUP (child ignores HUP)")
	}
	t.Logf("wrapper exited after %v (expected ~5s graceful timeout)", elapsed)
	if elapsed < 1*time.Second {
		t.Errorf("wrapper exited too fast (%v); SIGKILL escalation timer may not be working", elapsed)
	}
}

// TestSIGQUITForwarded sends SIGQUIT to the wrapper.  The wrapper forwards it
// directly to the child (no graceful timer).  The child (bash handling SIGQUIT
// by exiting 131) should cause the wrapper to exit promptly.
//
// Note: on macOS, bash in non-interactive mode does not always run traps
// while a foreground job is sleeping.  We test the simpler case: child ignores
// nothing (default SIGQUIT disposition for bash is to exit), so the wrapper
// should exit within 2s.
func TestSIGQUITForwarded(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only")
	}
	skipIfBashUnavailable(t)

	// bash default SIGQUIT disposition: exit (no core dump in scripts).
	// Use `trap "exit 131" QUIT` to get a predictable exit code.
	// However, as noted in SIGINT tests, bash in non-interactive mode running
	// an external `sleep` will not run the trap before sleep exits.
	// Use a busy-wait loop inside bash so the trap fires in the shell itself.
	shellCmd := `trap 'exit 131' QUIT; while true; do :; done`
	cmd := spawnPeek(t, shellCmd)
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	time.Sleep(300 * time.Millisecond)

	if err := cmd.Process.Signal(syscall.SIGQUIT); err != nil {
		t.Fatalf("send SIGQUIT: %v", err)
	}

	// The child's trap fires → exits 131.  Wrapper should exit promptly (< 3s,
	// no timer involved for SIGQUIT).
	_, ok := waitOrTimeout(cmd, 4*time.Second)
	if !ok {
		t.Fatalf("wrapper did not exit within 4s after SIGQUIT")
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
