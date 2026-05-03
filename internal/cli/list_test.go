package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeMeta writes a meta.json fixture into tmpDir/.peek/sessions/<id>/meta.json.
func writeMeta(t *testing.T, home, id string, m map[string]interface{}) {
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
}

func TestPeekListOutput(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // Windows compatibility

	// Fixed start time in UTC; local display depends on local TZ.
	startedAt := time.Date(2026, 5, 3, 14, 23, 1, 0, time.UTC)
	startedStr := startedAt.Format("2006-01-02T15:04:05.000Z")

	// Session 1 (newest): ULID prefix 01HQQQ... running, live pid = os.Getpid()
	id1 := "01HQQQQQQQQQQQQQQQQQQQQQQ1"
	writeMeta(t, home, id1, map[string]interface{}{
		"version":    1,
		"id":         id1,
		"pid":        os.Getpid(),
		"cwd":        home + "/Code/myapp",
		"cmd":        []string{"npm", "run", "dev"},
		"started_at": startedStr,
		"status":     "running",
		"exited_at":  nil,
		"exit_code":  nil,
	})

	// Session 2 (middle): exited(0)
	id2 := "01H7777777777777777777777772"
	exitCode := 0
	exitedAt := startedAt.Add(time.Hour)
	exitedStr := exitedAt.Format("2006-01-02T15:04:05.000Z")
	writeMeta(t, home, id2, map[string]interface{}{
		"version":    1,
		"id":         id2,
		"pid":        12345,
		"cwd":        home + "/Code/otherproj",
		"cmd":        []string{"cargo", "run"},
		"started_at": startedStr,
		"status":     "exited",
		"exited_at":  exitedStr,
		"exit_code":  exitCode,
	})

	// Session 3 (oldest): ULID prefix 01H444... wrapper-died (running on disk but dead pid)
	id3 := "01H4444444444444444444444443"
	writeMeta(t, home, id3, map[string]interface{}{
		"version":    1,
		"id":         id3,
		"pid":        99999999,
		"cwd":        home + "/Code/myapp",
		"cmd":        []string{"next", "dev"},
		"started_at": startedStr,
		"status":     "running",
		"exited_at":  nil,
		"exit_code":  nil,
	})

	var buf bytes.Buffer
	if err := RenderList(&buf, 100); err != nil {
		t.Fatalf("RenderList: %v", err)
	}

	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")

	// Should have header + 3 data rows
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines (header + 3 rows), got %d:\n%s", len(lines), out)
	}

	// Header must contain required column names
	header := lines[0]
	for _, col := range []string{"ID", "STATUS", "STARTED", "CMD", "CWD"} {
		if !strings.Contains(header, col) {
			t.Errorf("header missing %q: %q", col, header)
		}
	}

	// Row 0 = newest (id1), running
	row0 := lines[1]
	if !strings.Contains(row0, id1[:9]) {
		t.Errorf("row0 should have id1 prefix %q: %q", id1[:9], row0)
	}
	if !strings.Contains(row0, "running") {
		t.Errorf("row0 should have status 'running': %q", row0)
	}
	if !strings.Contains(row0, "npm run dev") {
		t.Errorf("row0 should have cmd 'npm run dev': %q", row0)
	}
	if !strings.Contains(row0, "~/Code/myapp") {
		t.Errorf("row0 should have cwd '~/Code/myapp': %q", row0)
	}

	// Row 1 = middle (id2), exited(0)
	row1 := lines[2]
	if !strings.Contains(row1, id2[:9]) {
		t.Errorf("row1 should have id2 prefix %q: %q", id2[:9], row1)
	}
	if !strings.Contains(row1, "exited(0)") {
		t.Errorf("row1 should have status 'exited(0)': %q", row1)
	}
	if !strings.Contains(row1, "cargo run") {
		t.Errorf("row1 should have cmd 'cargo run': %q", row1)
	}
	if !strings.Contains(row1, "~/Code/otherproj") {
		t.Errorf("row1 should have cwd '~/Code/otherproj': %q", row1)
	}

	// Row 2 = oldest (id3), exited(?) — wrapper-died
	row2 := lines[3]
	if !strings.Contains(row2, id3[:9]) {
		t.Errorf("row2 should have id3 prefix %q: %q", id3[:9], row2)
	}
	if !strings.Contains(row2, "exited(?)") {
		t.Errorf("row2 should have status 'exited(?)': %q", row2)
	}
	if !strings.Contains(row2, "next dev") {
		t.Errorf("row2 should have cmd 'next dev': %q", row2)
	}
	if !strings.Contains(row2, "~/Code/myapp") {
		t.Errorf("row2 should have cwd '~/Code/myapp': %q", row2)
	}

	// Verify STARTED is formatted as local time
	expectedStarted := startedAt.In(time.Local).Format("2006-01-02 15:04:05")
	if !strings.Contains(row0, expectedStarted) {
		t.Errorf("row0 should contain started time %q: %q", expectedStarted, row0)
	}
}
