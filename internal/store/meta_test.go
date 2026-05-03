package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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

func TestMetaJSONUsesFormatTimestamp(t *testing.T) {
	// Test that Meta's MarshalJSON routes timestamps through FormatTimestamp.
	// Construct a Meta with a non-UTC time at sub-millisecond precision.
	// The output JSON should contain UTC, truncated-to-ms, 3-digit fractional, Z suffix.

	edtZone := time.FixedZone("EDT", -4*3600)
	startedAt := time.Date(2026, 5, 3, 10, 23, 1, 234999999, edtZone)
	// 10:23 EDT == 14:23 UTC; 234999999 ns = 234.999999 ms -> truncates to 234 ms

	meta := &Meta{
		Version:   1,
		ID:        "test-id-001",
		Pid:       999,
		Cwd:       "/tmp/test",
		Cmd:       []string{"test"},
		StartedAt: startedAt,
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

	// Verify the JSON contains the expected formatted timestamp
	// Should be: UTC time (14:23:01), truncated to 234ms, 3 digits, Z suffix
	expectedTimestamp := `"started_at":"2026-05-03T14:23:01.234Z"`
	if !strings.Contains(jsonStr, expectedTimestamp) {
		t.Errorf("JSON should contain %s, got: %s", expectedTimestamp, jsonStr)
	}
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "UTC at exact millisecond",
			input:    time.Date(2026, 5, 3, 14, 23, 1, 234*int(time.Millisecond), time.UTC),
			expected: "2026-05-03T14:23:01.234Z",
		},
		{
			name:     "Non-UTC time converted to UTC",
			input:    time.Date(2026, 5, 3, 10, 23, 1, 234*int(time.Millisecond), time.FixedZone("EDT", -4*3600)),
			expected: "2026-05-03T14:23:01.234Z",
		},
		{
			name:     "Sub-millisecond precision truncated, not rounded",
			input:    time.Date(2026, 5, 3, 14, 23, 1, 234999999, time.UTC),
			expected: "2026-05-03T14:23:01.234Z",
		},
		{
			name:     "Trailing-zero milliseconds preserved as 3 digits",
			input:    time.Date(2026, 5, 3, 14, 23, 1, 500*int(time.Millisecond), time.UTC),
			expected: "2026-05-03T14:23:01.500Z",
		},
		{
			name:     "Zero milliseconds",
			input:    time.Date(2026, 5, 3, 14, 23, 1, 0, time.UTC),
			expected: "2026-05-03T14:23:01.000Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTimestamp(tt.input)
			if result != tt.expected {
				t.Errorf("FormatTimestamp(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestReadMeta(t *testing.T) {
	t.Run("Valid file returns Meta with all fields matching", func(t *testing.T) {
		tempDir := t.TempDir()
		metaPath := filepath.Join(tempDir, "meta.json")

		// Write a known good meta.json
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
		jsonBytes, err := json.Marshal(meta)
		if err != nil {
			t.Fatalf("failed to marshal meta: %v", err)
		}
		if err := os.WriteFile(metaPath, jsonBytes, 0o644); err != nil {
			t.Fatalf("failed to write meta.json: %v", err)
		}

		// Call ReadMeta
		result, err := ReadMeta(metaPath)
		if err != nil {
			t.Fatalf("ReadMeta failed: %v", err)
		}
		if result == nil {
			t.Fatal("ReadMeta returned nil, expected valid Meta")
		}

		// Verify all fields match
		if result.Version != meta.Version {
			t.Errorf("Version mismatch: got %d, want %d", result.Version, meta.Version)
		}
		if result.ID != meta.ID {
			t.Errorf("ID mismatch: got %s, want %s", result.ID, meta.ID)
		}
		if result.Pid != meta.Pid {
			t.Errorf("Pid mismatch: got %d, want %d", result.Pid, meta.Pid)
		}
		if result.Cwd != meta.Cwd {
			t.Errorf("Cwd mismatch: got %s, want %s", result.Cwd, meta.Cwd)
		}
		if !reflect.DeepEqual(result.Cmd, meta.Cmd) {
			t.Errorf("Cmd mismatch: got %v, want %v", result.Cmd, meta.Cmd)
		}
		if !result.StartedAt.Equal(meta.StartedAt) {
			t.Errorf("StartedAt mismatch: got %v, want %v", result.StartedAt, meta.StartedAt)
		}
		if result.Status != meta.Status {
			t.Errorf("Status mismatch: got %s, want %s", result.Status, meta.Status)
		}
		if result.ExitedAt != nil {
			t.Errorf("ExitedAt should be nil, got %v", *result.ExitedAt)
		}
		if result.ExitCode != nil {
			t.Errorf("ExitCode should be nil, got %d", *result.ExitCode)
		}
	})

	t.Run("ENOENT returns nil, nil", func(t *testing.T) {
		result, err := ReadMeta("/non/existent/path/meta.json")
		if result != nil {
			t.Errorf("ReadMeta returned non-nil result for missing file: %v", result)
		}
		if err != nil {
			t.Errorf("ReadMeta returned error for missing file: %v", err)
		}
	})

	t.Run("Malformed JSON returns nil, nil", func(t *testing.T) {
		tempDir := t.TempDir()
		metaPath := filepath.Join(tempDir, "meta.json")
		if err := os.WriteFile(metaPath, []byte("not valid json"), 0o644); err != nil {
			t.Fatalf("failed to write invalid JSON: %v", err)
		}

		result, err := ReadMeta(metaPath)
		if result != nil {
			t.Errorf("ReadMeta returned non-nil result for malformed JSON: %v", result)
		}
		if err != nil {
			t.Errorf("ReadMeta returned error for malformed JSON: %v", err)
		}
	})

	t.Run("Permission denied propagates error", func(t *testing.T) {
		// Skip on Windows or if running as root
		if runtime.GOOS == "windows" {
			t.Skip("requires non-Windows system")
		}

		tempDir := t.TempDir()
		metaPath := filepath.Join(tempDir, "meta.json")

		// Write a valid meta.json
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
		jsonBytes, _ := json.Marshal(meta)
		if err := os.WriteFile(metaPath, jsonBytes, 0o644); err != nil {
			t.Fatalf("failed to write meta.json: %v", err)
		}

		// Deny read access to the directory
		if err := os.Chmod(tempDir, 0o000); err != nil {
			t.Fatalf("failed to chmod tempDir: %v", err)
		}
		// Restore permissions after the test
		t.Cleanup(func() {
			os.Chmod(tempDir, 0o755)
		})

		// Call ReadMeta and expect an error
		result, err := ReadMeta(metaPath)
		if result != nil {
			t.Errorf("ReadMeta should return nil result on permission error, got %v", result)
		}
		if err == nil {
			t.Errorf("ReadMeta should return error on permission denied, got nil")
		}
	})
}

// Helper functions
func containsVersionField(s string) bool {
	return strings.Contains(s, `"version":1`) || strings.Contains(s, `"version": 1`)
}
