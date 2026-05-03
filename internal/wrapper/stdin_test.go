//go:build !windows

package wrapper

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Pankaj3112/peek/internal/platform"
)

// TestStdinForward verifies that bytes written to the stdin reader are
// forwarded to the child process via the pty master.
// Raw-mode behaviour is NOT tested here (no terminal in CI); we only test
// the byte-forwarding path using `cat` as a stdin echo target.
func TestStdinForward(t *testing.T) {
	t.Run("cat_echo", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())

		// Create a pipe; write "hello\n" so the shell reads one line and echoes it,
		// then exits on its own. No EOF signal needed through the pty.
		pr, pw := io.Pipe()
		go func() {
			_, _ = io.WriteString(pw, "hello\n")
			pw.Close()
		}()

		// sh -c 'read x; echo "$x"' reads one line from stdin, echoes it, then exits.
		// This avoids having to send a pty EOF signal (^D) to close cat.
		exitCode, err := wrapWithStdin("/test/cwd", []string{"sh", "-c", "read x; echo \"$x\""}, pr)
		if err != nil {
			t.Fatalf("wrapWithStdin returned error: %v", err)
		}
		if exitCode != 0 {
			t.Errorf("exit code: got %d, want 0", exitCode)
		}

		// Locate the session log written during this run.
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
		logPath := filepath.Join(dirs[0], "output.log")

		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("read output.log: %v", err)
		}

		// Strip timestamp prefix (25 chars: "2006-01-02T15:04:05.000Z ")
		// and look for "hello" anywhere in the log.
		found := false
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := scanner.Text()
			stripped := line
			if len(line) >= 25 {
				stripped = line[25:]
			}
			if strings.Contains(stripped, "hello") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("output.log does not contain 'hello'; raw content:\n%s", string(data))
		}
	})
}
