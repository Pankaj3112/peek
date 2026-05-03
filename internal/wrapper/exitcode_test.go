//go:build !windows

package wrapper

import (
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Pankaj3112/peek/internal/platform"
	"github.com/Pankaj3112/peek/internal/store"
)

// findSingleSessionDir locates the single session directory under the HOME's
// peek sessions root. It is intended for tests that create exactly one session.
func findSingleSessionDir(t *testing.T) string {
	t.Helper()
	root, err := platform.SessionsRoot()
	if err != nil {
		t.Fatalf("SessionsRoot: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir %s: %v", root, err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(root, e.Name()))
		}
	}
	if len(dirs) != 1 {
		t.Fatalf("expected exactly 1 session dir, got %d: %v", len(dirs), dirs)
	}
	return dirs[0]
}

// readMetaExitCode reads the exit_code field from meta.json in sessDir.
func readMetaExitCode(t *testing.T, sessDir string) int {
	t.Helper()
	metaPath := filepath.Join(sessDir, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read meta.json: %v", err)
	}
	var meta store.Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshal meta.json: %v", err)
	}
	if meta.ExitCode == nil {
		t.Fatalf("meta.json exit_code is nil")
	}
	return *meta.ExitCode
}

// TestNormalExitCode verifies that a child that exits 42 propagates that
// exact code through the wrapper and into meta.json.
func TestNormalExitCode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	exitCode, err := wrapWithStdin("/some/cwd", []string{"bash", "-c", "exit 42"}, nil)
	if err != nil {
		t.Fatalf("wrapWithStdin: %v", err)
	}
	if exitCode != 42 {
		t.Errorf("wrapper exit code: got %d, want 42", exitCode)
	}

	sessDir := findSingleSessionDir(t)
	metaCode := readMetaExitCode(t, sessDir)
	if metaCode != 42 {
		t.Errorf("meta.json exit_code: got %d, want 42", metaCode)
	}
}

// TestSignalExitCode is intentionally skipped: wrapWithStdin cannot be
// externally signaled (it blocks until the child exits). Signal-derived exit
// code verification is done via TestSignalExitCodeSubprocess below, which
// uses an out-of-process peek binary that can receive external signals.
func TestSignalExitCode(t *testing.T) {
	t.Skip("wrapWithStdin cannot be externally signaled; see TestSignalExitCodeSubprocess")
}

// TestSignalExitCodeSubprocess verifies 128+signum via an external peek subprocess.
// It spawns `peek -- bash -c 'trap "" INT TERM; sleep 30'`, sends SIGTERM,
// waits for the 5s graceful timeout to fire SIGKILL, then asserts exit code 137.
func TestSignalExitCodeSubprocess(t *testing.T) {
	skipIfBashUnavailable(t)

	// Override graceful timeout would require access to the binary's globals,
	// which we can't do from outside. Instead, use the default 5s timeout.
	// Total test time: ~5s (graceful timeout) + 2s margin = 7s.
	cmd := spawnPeek(t, "trap '' INT TERM; sleep 30")
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})

	// Give the child time to start and set up the trap.
	time.Sleep(300 * time.Millisecond)

	// Send SIGTERM to the wrapper; child ignores it.
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}

	// Wait for the graceful timeout (5s) + SIGKILL + exit. Up to 8s total.
	code, ok := waitOrTimeout(cmd, 8*time.Second)
	if !ok {
		t.Fatalf("wrapper did not exit within 8s after SIGTERM (expecting SIGKILL escalation)")
	}

	// SIGKILL (signal 9) → 128 + 9 = 137.
	const wantCode = 137
	if code != wantCode {
		t.Errorf("wrapper exit code: got %d, want %d (128+SIGKILL)", code, wantCode)
	}

	t.Logf("wrapper exited with code %d (want %d)", code, wantCode)
	cmd.Process = nil
}

// TestNormalExitCodeSubprocess verifies that a normal exit code propagates
// correctly through the external peek binary.
func TestNormalExitCodeSubprocess(t *testing.T) {
	skipIfBashUnavailable(t)

	cmd := spawnPeek(t, "exit 42")
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})

	code, ok := waitOrTimeout(cmd, 5*time.Second)
	if !ok {
		t.Fatalf("wrapper did not exit within 5s")
	}
	if code != 42 {
		t.Errorf("wrapper exit code: got %d, want 42", code)
	}
	cmd.Process = nil
}
