package wrapper

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Pankaj3112/peek/internal/platform"
	"github.com/Pankaj3112/peek/internal/store"
)

// readPTYTestLogLines reads output.log lines and strips the 24-char timestamp prefix + space.
func readPTYTestLogLines(t *testing.T, logPath string) []string {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("readPTYTestLogLines: failed to read %s: %v", logPath, err)
	}
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		// "2006-01-02T15:04:05.000Z " is 25 chars (24 + space)
		if len(line) < 25 {
			lines = append(lines, line)
			continue
		}
		lines = append(lines, line[25:])
	}
	return lines
}

// findPTYTestSessionDir locates the single session dir under ~/.peek/sessions/ and returns it.
func findPTYTestSessionDir(t *testing.T) string {
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

func TestWrapPTYSize(t *testing.T) {
	t.Run("default_when_not_a_terminal", func(t *testing.T) {
		// Test stdin is not a real terminal (test runner), so parentTerminalSize()
		// must fall back to rows=24 cols=80. The child running "stty size" should
		// report "24 80".
		t.Setenv("HOME", t.TempDir())

		exitCode, err := Wrap("/some/cwd", []string{"sh", "-c", "stty size"})
		if err != nil {
			t.Fatalf("Wrap returned error: %v", err)
		}
		if exitCode != 0 {
			t.Errorf("exit code: got %d, want 0", exitCode)
		}

		sessDir := findPTYTestSessionDir(t)
		logPath := filepath.Join(sessDir, "output.log")
		lines := readPTYTestLogLines(t, logPath)

		found := false
		for _, l := range lines {
			if strings.Contains(l, "24 80") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected pty size '24 80' in output, got lines: %v", lines)
		}
	})

	t.Run("explicit_size_via_internal_helper", func(t *testing.T) {
		// Call the internal wrapWithSize helper with explicit dimensions.
		// The child running "stty size" should report "40 120".
		t.Setenv("HOME", t.TempDir())

		exitCode, err := wrapWithSize("/some/cwd", []string{"sh", "-c", "stty size"}, 40, 120)
		if err != nil {
			t.Fatalf("wrapWithSize returned error: %v", err)
		}
		if exitCode != 0 {
			t.Errorf("exit code: got %d, want 0", exitCode)
		}

		sessDir := findPTYTestSessionDir(t)
		logPath := filepath.Join(sessDir, "output.log")
		lines := readPTYTestLogLines(t, logPath)

		found := false
		for _, l := range lines {
			if strings.Contains(l, "40 120") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected pty size '40 120' in output, got lines: %v", lines)
		}
	})
}

// Ensure store is referenced so its import is used.
var _ = store.StatusExited
