package store

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMetaJSONRoundTripRunning(t *testing.T) {
	// Behavior (a) and (b): Round-trip preserves all fields for a running session
	// and JSON contains "version": 1 with nullables as null

	meta := &Meta{
		Version:   1,
		ID:        "01H8XHS7Q9M2K3F4P5N6R7T8V9",
		Pid:       42,
		Cwd:       "/Users/test/proj",
		Cmd:       []string{"npm", "run", "dev"},
		StartedAt: time.Date(2026, 5, 3, 14, 23, 1, 234*int(time.Millisecond), time.UTC),
		Status:    StatusRunning,
		ExitedAt:  nil,
		ExitCode:  nil,
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(jsonBytes)

	// Check for "version":1 or "version": 1 in JSON
	if !containsVersionField(jsonStr) {
		t.Errorf("JSON does not contain version field properly: %s", jsonStr)
	}

	// Check for null serialization of nullable fields
	if !strings.Contains(jsonStr, `"exited_at":null`) && !strings.Contains(jsonStr, `"exited_at": null`) {
		t.Errorf("JSON should contain exited_at as null but got: %s", jsonStr)
	}

	if !strings.Contains(jsonStr, `"exit_code":null`) && !strings.Contains(jsonStr, `"exit_code": null`) {
		t.Errorf("JSON should contain exit_code as null but got: %s", jsonStr)
	}

	// Unmarshal into a fresh Meta
	var unmarshalled Meta
	err = json.Unmarshal(jsonBytes, &unmarshalled)
	if err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Compare all fields
	if unmarshalled.Version != meta.Version {
		t.Errorf("Version mismatch: got %d, want %d", unmarshalled.Version, meta.Version)
	}
	if unmarshalled.ID != meta.ID {
		t.Errorf("ID mismatch: got %s, want %s", unmarshalled.ID, meta.ID)
	}
	if unmarshalled.Pid != meta.Pid {
		t.Errorf("Pid mismatch: got %d, want %d", unmarshalled.Pid, meta.Pid)
	}
	if unmarshalled.Cwd != meta.Cwd {
		t.Errorf("Cwd mismatch: got %s, want %s", unmarshalled.Cwd, meta.Cwd)
	}
	if !reflect.DeepEqual(unmarshalled.Cmd, meta.Cmd) {
		t.Errorf("Cmd mismatch: got %v, want %v", unmarshalled.Cmd, meta.Cmd)
	}
	if !unmarshalled.StartedAt.Equal(meta.StartedAt) {
		t.Errorf("StartedAt mismatch: got %v, want %v", unmarshalled.StartedAt, meta.StartedAt)
	}
	if unmarshalled.Status != meta.Status {
		t.Errorf("Status mismatch: got %s, want %s", unmarshalled.Status, meta.Status)
	}
	if unmarshalled.ExitedAt != nil {
		t.Errorf("ExitedAt should be nil, got %v", *unmarshalled.ExitedAt)
	}
	if unmarshalled.ExitCode != nil {
		t.Errorf("ExitCode should be nil, got %d", *unmarshalled.ExitCode)
	}
}

func TestMetaJSONRoundTripExited(t *testing.T) {
	// Behavior (c): Exited Meta round-trips with non-nil pointer values

	exitedAt := time.Date(2026, 5, 3, 14, 25, 0, 500*int(time.Millisecond), time.UTC)
	exitCode := 0

	meta := &Meta{
		Version:   1,
		ID:        "01H8XHS7Q9M2K3F4P5N6R7T8V9",
		Pid:       42,
		Cwd:       "/Users/test/proj",
		Cmd:       []string{"npm", "run", "dev"},
		StartedAt: time.Date(2026, 5, 3, 14, 23, 1, 234*int(time.Millisecond), time.UTC),
		Status:    StatusExited,
		ExitedAt:  &exitedAt,
		ExitCode:  &exitCode,
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(jsonBytes)

	// exit_code=0 must serialize as 0, not null (regression guard for omitempty)
	if strings.Contains(jsonStr, `"exit_code":null`) {
		t.Errorf("exit_code=0 must serialize as 0, not null; JSON: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"exit_code":0`) {
		t.Errorf("exit_code=0 must serialize as 0; JSON: %s", jsonStr)
	}

	// exited_at must serialize as a non-null string
	if strings.Contains(jsonStr, `"exited_at":null`) {
		t.Errorf("exited_at must not be null when set; JSON: %s", jsonStr)
	}

	// Unmarshal into a fresh Meta
	var unmarshalled Meta
	err = json.Unmarshal(jsonBytes, &unmarshalled)
	if err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Compare all fields, checking pointer values match
	if unmarshalled.Version != meta.Version {
		t.Errorf("Version mismatch: got %d, want %d", unmarshalled.Version, meta.Version)
	}
	if unmarshalled.ID != meta.ID {
		t.Errorf("ID mismatch: got %s, want %s", unmarshalled.ID, meta.ID)
	}
	if unmarshalled.Pid != meta.Pid {
		t.Errorf("Pid mismatch: got %d, want %d", unmarshalled.Pid, meta.Pid)
	}
	if unmarshalled.Cwd != meta.Cwd {
		t.Errorf("Cwd mismatch: got %s, want %s", unmarshalled.Cwd, meta.Cwd)
	}
	if !reflect.DeepEqual(unmarshalled.Cmd, meta.Cmd) {
		t.Errorf("Cmd mismatch: got %v, want %v", unmarshalled.Cmd, meta.Cmd)
	}
	if !unmarshalled.StartedAt.Equal(meta.StartedAt) {
		t.Errorf("StartedAt mismatch: got %v, want %v", unmarshalled.StartedAt, meta.StartedAt)
	}
	if unmarshalled.Status != meta.Status {
		t.Errorf("Status mismatch: got %s, want %s", unmarshalled.Status, meta.Status)
	}

	// Check ExitedAt pointer value
	if unmarshalled.ExitedAt == nil {
		t.Errorf("ExitedAt should not be nil after round-trip")
	} else if !unmarshalled.ExitedAt.Equal(*meta.ExitedAt) {
		t.Errorf("ExitedAt value mismatch: got %v, want %v", *unmarshalled.ExitedAt, *meta.ExitedAt)
	}

	// Check ExitCode pointer value
	if unmarshalled.ExitCode == nil {
		t.Errorf("ExitCode should not be nil after round-trip")
	} else if *unmarshalled.ExitCode != *meta.ExitCode {
		t.Errorf("ExitCode value mismatch: got %d, want %d", *unmarshalled.ExitCode, *meta.ExitCode)
	}
}

func TestMetaUnmarshalRejectsUnknownStatus(t *testing.T) {
	// Behavior (d): Unknown status rejected at unmarshal time

	jsonStr := `{
		"version": 1,
		"id": "01H8XHS7Q9M2K3F4P5N6R7T8V9",
		"pid": 42,
		"cwd": "/Users/test/proj",
		"cmd": ["npm", "run", "dev"],
		"started_at": "2026-05-03T14:23:01.234Z",
		"status": "crashed",
		"exited_at": null,
		"exit_code": null
	}`

	var meta Meta
	err := json.Unmarshal([]byte(jsonStr), &meta)

	if err == nil {
		t.Errorf("Expected error for invalid status, but got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "crashed") && !strings.Contains(errMsg, "invalid status") {
		t.Errorf("Error message should mention the invalid value or be descriptive, got: %s", errMsg)
	}
}

// Helper functions
func containsVersionField(s string) bool {
	return strings.Contains(s, `"version":1`) || strings.Contains(s, `"version": 1`)
}
