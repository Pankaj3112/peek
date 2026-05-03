package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeMetaForLogs writes a meta.json fixture into home/.peek/sessions/<id>/.
func writeMetaForLogs(t *testing.T, home, id string, m map[string]interface{}) string {
	t.Helper()
	dir := filepath.Join(home, ".peek", "sessions", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), b, 0o644); err != nil {
		t.Fatalf("WriteFile meta.json: %v", err)
	}
	return dir
}

func TestResolveIDPrefix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	started := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	id1 := "01H8AAA1AAAAAAAAAAAAAAAAAAA1"
	id2 := "01H8AAB1AAAAAAAAAAAAAAAAAAA2"
	id3 := "01H8BBB1AAAAAAAAAAAAAAAAAAA3"

	for _, id := range []string{id1, id2, id3} {
		writeMetaForLogs(t, home, id, map[string]interface{}{
			"version":    1,
			"id":         id,
			"pid":        12345,
			"cwd":        "/tmp",
			"cmd":        []string{"echo"},
			"started_at": started,
			"status":     "exited",
			"exited_at":  started,
			"exit_code":  0,
		})
	}

	t.Run("unambiguous prefix returns match", func(t *testing.T) {
		got, err := ResolveIDPrefix("01H8B")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != id3 {
			t.Errorf("got %q, want %q", got, id3)
		}
	})

	t.Run("ambiguous prefix returns error", func(t *testing.T) {
		_, err := ResolveIDPrefix("01H8AA")
		if err == nil {
			t.Fatal("expected ambiguous error, got nil")
		}
		if !strings.Contains(err.Error(), "ambiguous") {
			t.Errorf("error should mention ambiguous: %v", err)
		}
	})

	t.Run("no match returns error", func(t *testing.T) {
		_, err := ResolveIDPrefix("99")
		if err == nil {
			t.Fatal("expected not-found error, got nil")
		}
	})

	t.Run("full ULID returns exact match", func(t *testing.T) {
		got, err := ResolveIDPrefix(id1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != id1 {
			t.Errorf("got %q, want %q", got, id1)
		}
	})
}

func TestLogsOutputFormat(t *testing.T) {
	// Fix TZ to UTC so local == UTC, making test deterministic across timezones.
	t.Setenv("TZ", "UTC")
	// Note: Setenv TZ doesn't always work on all platforms for time.Local.
	// We'll verify by checking the actual formatted values.

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	id := "01H9AAAAAAAAAAAAAAAAAAAAAA1"
	started := "2026-05-03T14:23:01.000Z"
	dir := writeMetaForLogs(t, home, id, map[string]interface{}{
		"version":    1,
		"id":         id,
		"pid":        12345,
		"cwd":        "/tmp",
		"cmd":        []string{"echo"},
		"started_at": started,
		"status":     "exited",
		"exited_at":  started,
		"exit_code":  0,
	})

	// Write output.log
	logContent := "2026-05-03T14:23:01.230Z compiling src/foo.ts\n" +
		"2026-05-03T14:23:01.231Z compiling src/bar.ts\n"
	if err := os.WriteFile(filepath.Join(dir, "output.log"), []byte(logContent), 0o644); err != nil {
		t.Fatalf("WriteFile output.log: %v", err)
	}

	var buf bytes.Buffer
	ctx := context.Background()
	if err := RenderLogs(ctx, &buf, id); err != nil {
		t.Fatalf("RenderLogs: %v", err)
	}

	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 output lines, got %d:\n%s", len(lines), out)
	}

	// Parse the times through time.Local to get expected display
	ts1, _ := time.Parse("2006-01-02T15:04:05.000Z", "2026-05-03T14:23:01.230Z")
	ts2, _ := time.Parse("2006-01-02T15:04:05.000Z", "2026-05-03T14:23:01.231Z")
	expected1 := ts1.In(time.Local).Format("15:04:05.000")
	expected2 := ts2.In(time.Local).Format("15:04:05.000")

	if !strings.Contains(lines[0], expected1) {
		t.Errorf("line 1 should contain time %q: %q", expected1, lines[0])
	}
	if !strings.Contains(lines[0], "compiling src/foo.ts") {
		t.Errorf("line 1 should contain text: %q", lines[0])
	}
	if !strings.Contains(lines[1], expected2) {
		t.Errorf("line 2 should contain time %q: %q", expected2, lines[1])
	}
	if !strings.Contains(lines[1], "compiling src/bar.ts") {
		t.Errorf("line 2 should contain text: %q", lines[1])
	}

	// Line numbers should be present
	if !strings.Contains(lines[0], "1") {
		t.Errorf("line 1 should contain line number 1: %q", lines[0])
	}
	if !strings.Contains(lines[1], "2") {
		t.Errorf("line 2 should contain line number 2: %q", lines[1])
	}
}

func TestLogsTailFForRunning(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	id := "01H9BBBBBBBBBBBBBBBBBBBBBBB1"
	started := "2026-05-03T14:23:01.000Z"
	dir := writeMetaForLogs(t, home, id, map[string]interface{}{
		"version":    1,
		"id":         id,
		"pid":        os.Getpid(), // alive pid → running
		"cwd":        "/tmp",
		"cmd":        []string{"echo"},
		"started_at": started,
		"status":     "running",
		"exited_at":  nil,
		"exit_code":  nil,
	})

	// Write 2 initial log lines
	logPath := filepath.Join(dir, "output.log")
	line1 := fmt.Sprintf("2026-05-03T14:23:01.230Z line one\n")
	line2 := fmt.Sprintf("2026-05-03T14:23:01.231Z line two\n")
	if err := os.WriteFile(logPath, []byte(line1+line2), 0o644); err != nil {
		t.Fatalf("WriteFile output.log: %v", err)
	}

	// Use a short poll interval for test speed.
	origInterval := pollInterval
	pollInterval = 20 * time.Millisecond
	defer func() { pollInterval = origInterval }()

	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- RenderLogs(ctx, &buf, id)
	}()

	// Give the goroutine time to read the first 2 lines.
	time.Sleep(60 * time.Millisecond)

	// Append a 3rd line.
	f, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open log for append: %v", err)
	}
	fmt.Fprintf(f, "2026-05-03T14:23:01.232Z line three\n")
	f.Close()

	// Give the poller time to pick it up.
	time.Sleep(80 * time.Millisecond)

	// Now transition the session to exited.
	exitCode := 0
	exitedAt := "2026-05-03T14:23:02.000Z"
	metaMap := map[string]interface{}{
		"version":    1,
		"id":         id,
		"pid":        os.Getpid(),
		"cwd":        "/tmp",
		"cmd":        []string{"echo"},
		"started_at": started,
		"status":     "exited",
		"exited_at":  exitedAt,
		"exit_code":  exitCode,
	}
	metaBytes, _ := json.MarshalIndent(metaMap, "", "  ")
	// Write atomically via temp + rename.
	tmpMeta := filepath.Join(dir, "meta.json.tmp")
	os.WriteFile(tmpMeta, metaBytes, 0o644)
	os.Rename(tmpMeta, filepath.Join(dir, "meta.json"))

	// Wait for RenderLogs to finish.
	select {
	case err := <-done:
		if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
			t.Fatalf("RenderLogs returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RenderLogs did not finish within timeout")
	}

	out := buf.String()
	for _, text := range []string{"line one", "line two", "line three"} {
		if !strings.Contains(out, text) {
			t.Errorf("output should contain %q:\n%s", text, out)
		}
	}
}
