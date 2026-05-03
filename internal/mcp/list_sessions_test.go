package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Pankaj3112/peek/internal/store"
)

func TestListSessionsHandler(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Seed three sessions.
	seedSession(t, home, "01HQQQQQQQQQQQQQQQQQQQQQQQ", "/tmp/proj1", []string{"npm", "run", "dev"}, store.StatusRunning, os.Getpid(), nil, nil)
	exitCode0 := 0
	exitTime := time.Now().UTC()
	seedSession(t, home, "01H777777777777777777777777", "/tmp/proj2", []string{"cargo", "run"}, store.StatusExited, 12345, &exitTime, &exitCode0)
	seedSession(t, home, "01H444444444444444444444444", "/tmp/proj1", []string{"old", "cmd"}, store.StatusRunning, 99999999, nil, nil) // dead pid

	t.Run("no_filter_returns_all", func(t *testing.T) {
		result, err := listSessionsHandler(context.Background(), json.RawMessage(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		sessions := result.(map[string]any)["sessions"].([]any)
		if len(sessions) != 3 {
			t.Errorf("expected 3, got %d", len(sessions))
		}
		// ULID descending: highest ID first
		first := sessions[0].(map[string]any)
		if first["id"].(string) != "01HQQQQQQQQQQQQQQQQQQQQQQQ" {
			t.Errorf("expected newest first, got %s", first["id"])
		}
	})

	t.Run("cwd_filter_returns_matching", func(t *testing.T) {
		result, err := listSessionsHandler(context.Background(), json.RawMessage(`{"cwd":"/tmp/proj1"}`))
		if err != nil {
			t.Fatal(err)
		}
		sessions := result.(map[string]any)["sessions"].([]any)
		if len(sessions) != 2 {
			t.Errorf("expected 2 in /tmp/proj1, got %d", len(sessions))
		}
	})

	t.Run("wrapper_died_field_for_virtual_exited", func(t *testing.T) {
		result, err := listSessionsHandler(context.Background(), json.RawMessage(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		sessions := result.(map[string]any)["sessions"].([]any)
		for _, s := range sessions {
			session := s.(map[string]any)
			if session["id"].(string) == "01H444444444444444444444444" {
				// dead-pid session: status should be virtually "exited" with wrapper_died=true
				if session["status"].(string) != "exited" {
					t.Errorf("expected virtual exited for dead pid session, got %v", session["status"])
				}
				if v, ok := session["wrapper_died"]; !ok || v != true {
					t.Errorf("expected wrapper_died=true, got %v", v)
				}
				return
			}
		}
		t.Errorf("dead-pid session not found in response")
	})

	t.Run("clean_exited_omits_wrapper_died", func(t *testing.T) {
		result, _ := listSessionsHandler(context.Background(), json.RawMessage(`{}`))
		sessions := result.(map[string]any)["sessions"].([]any)
		for _, s := range sessions {
			session := s.(map[string]any)
			if session["id"].(string) == "01H777777777777777777777777" {
				// clean exit: wrapper_died should be ABSENT (not false)
				if _, present := session["wrapper_died"]; present {
					t.Errorf("expected wrapper_died absent for clean exit, got present")
				}
				return
			}
		}
	})
}

// seedSession writes a meta.json directly to disk for testing.
func seedSession(t *testing.T, home, id, cwd string, cmd []string, status store.Status, pid int, exitedAt *time.Time, exitCode *int) {
	t.Helper()
	sessionDir := filepath.Join(home, ".peek", "sessions", id)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	m := store.Meta{
		Version:   1,
		ID:        id,
		Pid:       pid,
		Cwd:       cwd,
		Cmd:       cmd,
		StartedAt: time.Now().UTC().Truncate(time.Millisecond),
		Status:    status,
		ExitedAt:  exitedAt,
		ExitCode:  exitCode,
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "meta.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
