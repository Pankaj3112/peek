package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Pankaj3112/peek/internal/store"
)

func TestGetLogsHandler(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Seed an exited session with 10 lines.
	sessionDir := filepath.Join(home, ".peek", "sessions", "01HQQQQQQQQQQQQQQQQQQQQQQQ")
	os.MkdirAll(sessionDir, 0o755)
	seedExitedSessionMeta(t, sessionDir, "01HQQQQQQQQQQQQQQQQQQQQQQQ", 0)
	writeLogLines(t, filepath.Join(sessionDir, "output.log"), 10)

	t.Run("default_tail_returns_last_100_or_all", func(t *testing.T) {
		result := callGetLogs(t, `{"id":"01HQQQQQQQQQQQQQQQQQQQQQQQ"}`)
		// 10 lines exist; default lines=100; should return all 10.
		if result["from_line"].(float64) != 1 {
			t.Errorf("from=%v want 1", result["from_line"])
		}
		if result["to_line"].(float64) != 10 {
			t.Errorf("to=%v want 10", result["to_line"])
		}
		if result["total_lines"].(float64) != 10 {
			t.Errorf("total=%v want 10", result["total_lines"])
		}
		// Lines string contains 10 newline-terminated rows.
		lines := strings.Split(strings.TrimSuffix(result["lines"].(string), "\n"), "\n")
		if len(lines) != 10 {
			t.Errorf("got %d output rows", len(lines))
		}
		// Format check: each row is "N: TS TEXT".
		for i, row := range lines {
			wantNum := fmt.Sprintf("%d: ", i+1)
			if !strings.HasPrefix(row, wantNum) {
				t.Errorf("row %d missing num prefix: %q", i+1, row)
			}
		}
		if result["session_status"] != "exited" {
			t.Errorf("session_status=%v", result["session_status"])
		}
	})

	t.Run("explicit_lines_returns_tail", func(t *testing.T) {
		result := callGetLogs(t, `{"id":"01HQQQQQQQQQQQQQQQQQQQQQQQ","lines":3}`)
		if result["from_line"].(float64) != 8 {
			t.Errorf("from=%v want 8", result["from_line"])
		}
		if result["to_line"].(float64) != 10 {
			t.Errorf("to=%v want 10", result["to_line"])
		}
	})

	t.Run("start_line_returns_forward", func(t *testing.T) {
		result := callGetLogs(t, `{"id":"01HQQQQQQQQQQQQQQQQQQQQQQQ","lines":3,"start_line":5}`)
		if result["from_line"].(float64) != 5 {
			t.Errorf("from=%v want 5", result["from_line"])
		}
		if result["to_line"].(float64) != 7 {
			t.Errorf("to=%v want 7", result["to_line"])
		}
	})

	t.Run("start_line_past_end_returns_empty", func(t *testing.T) {
		result := callGetLogs(t, `{"id":"01HQQQQQQQQQQQQQQQQQQQQQQQ","start_line":100}`)
		if result["lines"].(string) != "" {
			t.Errorf("expected empty lines, got %q", result["lines"])
		}
		if result["total_lines"].(float64) != 10 {
			t.Errorf("total=%v want 10", result["total_lines"])
		}
	})

	t.Run("negative_start_line_errors", func(t *testing.T) {
		_, err := getLogsHandler(context.Background(), json.RawMessage(`{"id":"01HQQQQQQQQQQQQQQQQQQQQQQQ","start_line":-1}`))
		if err == nil {
			t.Errorf("expected error on negative start_line")
		}
	})

	t.Run("unknown_session_errors", func(t *testing.T) {
		_, err := getLogsHandler(context.Background(), json.RawMessage(`{"id":"01HZZZZZZZZZZZZZZZZZZZZZZZ"}`))
		if err == nil {
			t.Errorf("expected error on unknown session")
		}
	})

	t.Run("wrapper_died_field_when_pid_dead", func(t *testing.T) {
		// Seed a session with status=running but pid=99999999.
		deadDir := filepath.Join(home, ".peek", "sessions", "01HRRRRRRRRRRRRRRRRRRRRRRR")
		os.MkdirAll(deadDir, 0o755)
		seedRunningSessionWithPid(t, deadDir, "01HRRRRRRRRRRRRRRRRRRRRRRR", 99999999)
		writeLogLines(t, filepath.Join(deadDir, "output.log"), 3)

		result := callGetLogs(t, `{"id":"01HRRRRRRRRRRRRRRRRRRRRRRR"}`)
		if result["session_status"] != "exited" {
			t.Errorf("expected virtual exited, got %v", result["session_status"])
		}
		if v, ok := result["wrapper_died"]; !ok || v != true {
			t.Errorf("expected wrapper_died=true, got %v", v)
		}
	})

	t.Run("clean_exited_omits_wrapper_died", func(t *testing.T) {
		result := callGetLogs(t, `{"id":"01HQQQQQQQQQQQQQQQQQQQQQQQ"}`)
		if _, present := result["wrapper_died"]; present {
			t.Errorf("expected wrapper_died absent for clean exit")
		}
	})
}

// callGetLogs invokes the handler and returns the parsed result map.
// It round-trips through JSON so numeric fields are float64 (standard json.Unmarshal behavior),
// matching the wire representation the tests assert against.
func callGetLogs(t *testing.T, args string) map[string]any {
	t.Helper()
	res, err := getLogsHandler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("get_logs error: %v", err)
	}
	// Round-trip through JSON so numbers become float64.
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	return out
}

// seedExitedSessionMeta writes a meta.json for a cleanly exited session.
func seedExitedSessionMeta(t *testing.T, dir, id string, exitCode int) {
	t.Helper()
	exitedAt := time.Now().UTC().Truncate(time.Millisecond)
	m := store.Meta{
		Version:   1,
		ID:        id,
		Pid:       12345,
		Cwd:       "/tmp/test",
		Cmd:       []string{"go", "run", "."},
		StartedAt: time.Now().UTC().Truncate(time.Millisecond),
		Status:    store.StatusExited,
		ExitedAt:  &exitedAt,
		ExitCode:  &exitCode,
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// seedRunningSessionWithPid writes a meta.json for a "running" session with the given pid.
func seedRunningSessionWithPid(t *testing.T, dir, id string, pid int) {
	t.Helper()
	m := store.Meta{
		Version:   1,
		ID:        id,
		Pid:       pid,
		Cwd:       "/tmp/test",
		Cmd:       []string{"go", "run", "."},
		StartedAt: time.Now().UTC().Truncate(time.Millisecond),
		Status:    store.StatusRunning,
		ExitedAt:  nil,
		ExitCode:  nil,
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
