package wrapper_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Pankaj3112/peek/internal/platform"
	"github.com/Pankaj3112/peek/internal/store"
	"github.com/Pankaj3112/peek/internal/wrapper"
)

// readLogLines reads output.log lines and strips the 24-char timestamp prefix + space.
// Format: "2006-01-02T15:04:05.000Z <text>\n"
func readLogLines(t *testing.T, logPath string) []string {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("readLogLines: failed to read %s: %v", logPath, err)
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

// findSessionDir locates the single session dir under ~/.peek/sessions/ and returns it.
func findSessionDir(t *testing.T) string {
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

func TestWrapEcho(t *testing.T) {
	t.Run("exit_code_zero", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())

		exitCode, err := wrapper.Wrap("/some/cwd", []string{"sh", "-c", "echo hello"})
		if err != nil {
			t.Fatalf("Wrap returned error: %v", err)
		}
		if exitCode != 0 {
			t.Errorf("exit code: got %d, want 0", exitCode)
		}

		sessDir := findSessionDir(t)

		// Read meta.json
		metaPath := filepath.Join(sessDir, "meta.json")
		metaData, err := os.ReadFile(metaPath)
		if err != nil {
			t.Fatalf("read meta.json: %v", err)
		}
		var meta store.Meta
		if err := json.Unmarshal(metaData, &meta); err != nil {
			t.Fatalf("unmarshal meta.json: %v", err)
		}

		// Verify meta fields
		if meta.Status != store.StatusExited {
			t.Errorf("status: got %q, want %q", meta.Status, store.StatusExited)
		}
		if meta.ExitCode == nil || *meta.ExitCode != 0 {
			t.Errorf("exit_code: got %v, want 0", meta.ExitCode)
		}
		if len(meta.Cmd) != 3 || meta.Cmd[0] != "sh" || meta.Cmd[1] != "-c" || meta.Cmd[2] != "echo hello" {
			t.Errorf("cmd: got %v, want [sh -c echo hello]", meta.Cmd)
		}
		if meta.Cwd != "/some/cwd" {
			t.Errorf("cwd: got %q, want /some/cwd", meta.Cwd)
		}
		if meta.Pid != os.Getpid() {
			t.Errorf("pid: got %d, want %d", meta.Pid, os.Getpid())
		}

		// Verify output.log contains "hello"
		logPath := filepath.Join(sessDir, "output.log")
		lines := readLogLines(t, logPath)
		found := false
		for _, l := range lines {
			if strings.Contains(l, "hello") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("output.log does not contain 'hello'; lines: %v", lines)
		}
	})

	t.Run("peek_session_id_env", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())

		exitCode, err := wrapper.Wrap("/some/cwd", []string{"sh", "-c", "echo $PEEK_SESSION_ID"})
		if err != nil {
			t.Fatalf("Wrap returned error: %v", err)
		}
		if exitCode != 0 {
			t.Errorf("exit code: got %d, want 0", exitCode)
		}

		sessDir := findSessionDir(t)

		// Read the session ID from meta.json
		metaPath := filepath.Join(sessDir, "meta.json")
		metaData, err := os.ReadFile(metaPath)
		if err != nil {
			t.Fatalf("read meta.json: %v", err)
		}
		var meta store.Meta
		if err := json.Unmarshal(metaData, &meta); err != nil {
			t.Fatalf("unmarshal meta.json: %v", err)
		}
		sessionID := meta.ID

		// Verify output.log line contains the session ULID
		logPath := filepath.Join(sessDir, "output.log")
		lines := readLogLines(t, logPath)
		found := false
		for _, l := range lines {
			if strings.Contains(l, sessionID) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("output.log does not contain session ID %q; lines: %v", sessionID, lines)
		}
		// ULID is 26 chars, alphanumeric
		if len(sessionID) != 26 {
			t.Errorf("session ID length: got %d, want 26", len(sessionID))
		}
	})

	t.Run("nonzero_exit_code", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())

		exitCode, err := wrapper.Wrap("/some/cwd", []string{"sh", "-c", "exit 7"})
		if err != nil {
			t.Fatalf("Wrap returned error: %v", err)
		}
		if exitCode != 7 {
			t.Errorf("exit code: got %d, want 7", exitCode)
		}

		sessDir := findSessionDir(t)

		metaPath := filepath.Join(sessDir, "meta.json")
		metaData, err := os.ReadFile(metaPath)
		if err != nil {
			t.Fatalf("read meta.json: %v", err)
		}
		var meta store.Meta
		if err := json.Unmarshal(metaData, &meta); err != nil {
			t.Fatalf("unmarshal meta.json: %v", err)
		}

		if meta.Status != store.StatusExited {
			t.Errorf("status: got %q, want %q", meta.Status, store.StatusExited)
		}
		if meta.ExitCode == nil || *meta.ExitCode != 7 {
			t.Errorf("exit_code: got %v, want 7", meta.ExitCode)
		}
	})
}
