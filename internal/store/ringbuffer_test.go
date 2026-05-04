package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRingBufferEviction(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	t.Run("evicts_oldest_when_count_at_limit", func(t *testing.T) {
		// Create 3 sessions in the same cwd with predictable ULIDs.
		cwd := "/test/proj1"
		seedSessionDir(t, home, "01HAAA0000000000000000000A", cwd)
		seedSessionDir(t, home, "01HBBB0000000000000000000B", cwd)
		seedSessionDir(t, home, "01HCCC0000000000000000000C", cwd)

		if err := EvictOldest(cwd); err != nil {
			t.Fatal(err)
		}

		// After eviction: 2 newest remain.
		remaining := listSessionsForCwd(t, home, cwd)
		if len(remaining) != 2 {
			t.Errorf("expected 2 remain, got %d", len(remaining))
		}
		if !contains(remaining, "01HBBB0000000000000000000B") {
			t.Errorf("middle session evicted unexpectedly")
		}
		if !contains(remaining, "01HCCC0000000000000000000C") {
			t.Errorf("newest session evicted unexpectedly")
		}
		if contains(remaining, "01HAAA0000000000000000000A") {
			t.Errorf("oldest session NOT evicted")
		}
	})

	t.Run("noop_when_under_limit", func(t *testing.T) {
		os.RemoveAll(filepath.Join(home, ".peek"))
		cwd := "/test/proj2"
		seedSessionDir(t, home, "01HXXX0000000000000000000X", cwd)
		seedSessionDir(t, home, "01HYYY0000000000000000000Y", cwd)

		if err := EvictOldest(cwd); err != nil {
			t.Fatal(err)
		}

		remaining := listSessionsForCwd(t, home, cwd)
		if len(remaining) != 2 {
			t.Errorf("expected 2 to remain unchanged, got %d", len(remaining))
		}
	})

	t.Run("doesnt_affect_other_cwds", func(t *testing.T) {
		os.RemoveAll(filepath.Join(home, ".peek"))
		seedSessionDir(t, home, "01HAAA0000000000000000000A", "/test/proj1")
		seedSessionDir(t, home, "01HBBB0000000000000000000B", "/test/proj1")
		seedSessionDir(t, home, "01HCCC0000000000000000000C", "/test/proj1")
		seedSessionDir(t, home, "01HDDD0000000000000000000D", "/test/other")

		if err := EvictOldest("/test/proj1"); err != nil {
			t.Fatal(err)
		}

		// Other cwd untouched.
		if !sessionExists(t, home, "01HDDD0000000000000000000D") {
			t.Errorf("other cwd's session was incorrectly evicted")
		}
	})

	t.Run("idempotent_double_call", func(t *testing.T) {
		os.RemoveAll(filepath.Join(home, ".peek"))
		cwd := "/test/proj3"
		seedSessionDir(t, home, "01HAAA0000000000000000000A", cwd)
		seedSessionDir(t, home, "01HBBB0000000000000000000B", cwd)
		seedSessionDir(t, home, "01HCCC0000000000000000000C", cwd)

		EvictOldest(cwd)
		if err := EvictOldest(cwd); err != nil {
			t.Errorf("second call errored: %v", err)
		}

		remaining := listSessionsForCwd(t, home, cwd)
		if len(remaining) != 2 {
			t.Errorf("idempotent fail: %d remain", len(remaining))
		}
	})

	t.Run("tolerates_missing_meta", func(t *testing.T) {
		os.RemoveAll(filepath.Join(home, ".peek"))
		cwd := "/test/proj4"
		seedSessionDir(t, home, "01HAAA0000000000000000000A", cwd)
		seedSessionDir(t, home, "01HBBB0000000000000000000B", cwd)
		// Create a directory without meta.json.
		bareDir := filepath.Join(home, ".peek", "sessions", "01HCCC0000000000000000000C")
		os.MkdirAll(bareDir, 0o755)

		if err := EvictOldest(cwd); err != nil {
			t.Errorf("EvictOldest errored: %v", err)
		}
		// The bare dir is not counted (no meta), so we have only 2 valid sessions, no eviction needed.
	})
}

// TestCreateTriggersEviction verifies Create now calls EvictOldest under the hood.
func TestCreateTriggersEviction(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := "/test/proj-create"

	// Pre-seed 3 sessions in this cwd.
	seedSessionDir(t, home, "01HAAA0000000000000000000A", cwd)
	seedSessionDir(t, home, "01HBBB0000000000000000000B", cwd)
	seedSessionDir(t, home, "01HCCC0000000000000000000C", cwd)

	// Now Create a 4th session via the public API.
	s, err := Create(cwd, []string{"echo", "test"})
	if err != nil {
		t.Fatal(err)
	}

	// Total sessions in this cwd should now be 3 (oldest evicted, 2 remained, +1 new = 3).
	remaining := listSessionsForCwd(t, home, cwd)
	if len(remaining) != 3 {
		t.Errorf("expected 3 after Create, got %d", len(remaining))
	}
	if !contains(remaining, s.Meta.ID) {
		t.Errorf("new session missing")
	}
	if contains(remaining, "01HAAA0000000000000000000A") {
		t.Errorf("oldest still present")
	}

	// Cleanup.
	Finalize(s, 0)
}

// seedSessionDir writes a minimal valid meta.json for a session in the given cwd.
func seedSessionDir(t *testing.T, home, id, cwd string) {
	t.Helper()
	dir := filepath.Join(home, ".peek", "sessions", id)
	os.MkdirAll(dir, 0o755)
	exitCode := 0
	exited := time.Now().UTC().Truncate(time.Millisecond)
	m := Meta{
		Version:   1,
		ID:        id,
		Pid:       os.Getpid(),
		Cwd:       cwd,
		Cmd:       []string{"sh", "-c", "true"},
		StartedAt: time.Now().UTC().Truncate(time.Millisecond),
		Status:    StatusExited,
		ExitCode:  &exitCode,
		ExitedAt:  &exited,
	}
	data, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(filepath.Join(dir, "meta.json"), data, 0o644)
}

func listSessionsForCwd(t *testing.T, home, cwd string) []string {
	t.Helper()
	var out []string
	entries, _ := os.ReadDir(filepath.Join(home, ".peek", "sessions"))
	for _, e := range entries {
		m, _ := ReadMeta(filepath.Join(home, ".peek", "sessions", e.Name(), "meta.json"))
		if m == nil {
			continue
		}
		if m.Cwd == cwd {
			out = append(out, m.ID)
		}
	}
	return out
}

func sessionExists(t *testing.T, home, id string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(home, ".peek", "sessions", id, "meta.json"))
	return err == nil
}

func contains(s []string, x string) bool {
	for _, v := range s {
		if v == x {
			return true
		}
	}
	return false
}
