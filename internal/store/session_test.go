package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestSessionCreateAndFinalize(t *testing.T) {
	// Use a temp dir for HOME to isolate the test
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome) // Windows compatibility

	t.Run("Create produces a session directory and meta.json", func(t *testing.T) {
		cwd := "/some/cwd"
		cmd := []string{"npm", "run", "dev"}

		s, err := Create(cwd, cmd)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Verify Dir field exists and points to the right location
		if s.Dir == "" {
			t.Fatal("Session.Dir is empty")
		}

		// Verify the directory exists
		stat, err := os.Stat(s.Dir)
		if err != nil {
			t.Fatalf("Session directory does not exist: %v", err)
		}
		if !stat.IsDir() {
			t.Fatal("Session.Dir is not a directory")
		}

		// Verify meta.json exists and is non-empty
		metaPath := filepath.Join(s.Dir, "meta.json")
		metaData, err := os.ReadFile(metaPath)
		if err != nil {
			t.Fatalf("Failed to read meta.json: %v", err)
		}
		if len(metaData) == 0 {
			t.Fatal("meta.json is empty")
		}

		// Parse meta.json and verify fields
		var meta Meta
		if err := json.Unmarshal(metaData, &meta); err != nil {
			t.Fatalf("Failed to unmarshal meta.json: %v", err)
		}

		// Verify version
		if meta.Version != 1 {
			t.Errorf("version: got %d, want 1", meta.Version)
		}

		// Verify id is a 26-char ULID matching the directory name
		if len(meta.ID) != 26 {
			t.Errorf("id length: got %d, want 26", len(meta.ID))
		}
		expectedID := filepath.Base(s.Dir)
		if meta.ID != expectedID {
			t.Errorf("id: got %s, want %s", meta.ID, expectedID)
		}

		// Verify pid equals the current process PID
		if meta.Pid != os.Getpid() {
			t.Errorf("pid: got %d, want %d", meta.Pid, os.Getpid())
		}

		// Verify cwd
		if meta.Cwd != cwd {
			t.Errorf("cwd: got %q, want %q", meta.Cwd, cwd)
		}

		// Verify cmd
		if len(meta.Cmd) != len(cmd) || (len(cmd) > 0 && meta.Cmd[0] != cmd[0]) {
			t.Errorf("cmd: got %v, want %v", meta.Cmd, cmd)
		}

		// Verify started_at is recent (within 5 seconds)
		now := time.Now().UTC()
		if meta.StartedAt.After(now.Add(1 * time.Second)) {
			t.Errorf("started_at is in the future: %v", meta.StartedAt)
		}
		if meta.StartedAt.Before(now.Add(-5 * time.Second)) {
			t.Errorf("started_at is too old: %v", meta.StartedAt)
		}

		// Verify status
		if meta.Status != StatusRunning {
			t.Errorf("status: got %q, want %q", meta.Status, StatusRunning)
		}

		// Verify exited_at is null
		if meta.ExitedAt != nil {
			t.Errorf("exited_at should be null, got %v", meta.ExitedAt)
		}

		// Verify exit_code is null
		if meta.ExitCode != nil {
			t.Errorf("exit_code should be null, got %v", meta.ExitCode)
		}
	})

	t.Run("Finalize updates meta.json to exited state", func(t *testing.T) {
		cwd := "/some/cwd"
		cmd := []string{"npm", "run", "dev"}

		s, err := Create(cwd, cmd)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// Store the created_at for comparison
		metaPath := filepath.Join(s.Dir, "meta.json")
		initialData, _ := os.ReadFile(metaPath)
		var initialMeta Meta
		json.Unmarshal(initialData, &initialMeta)

		// Call Finalize
		exitCode := 130
		if err := Finalize(s, exitCode); err != nil {
			t.Fatalf("Finalize failed: %v", err)
		}

		// Re-read meta.json
		finalData, err := os.ReadFile(metaPath)
		if err != nil {
			t.Fatalf("Failed to read meta.json after Finalize: %v", err)
		}

		var finalMeta Meta
		if err := json.Unmarshal(finalData, &finalMeta); err != nil {
			t.Fatalf("Failed to unmarshal meta.json: %v", err)
		}

		// Verify status
		if finalMeta.Status != StatusExited {
			t.Errorf("status: got %q, want %q", finalMeta.Status, StatusExited)
		}

		// Verify exited_at is non-null and recent
		if finalMeta.ExitedAt == nil {
			t.Fatal("exited_at should not be null")
		}
		now := time.Now().UTC()
		if finalMeta.ExitedAt.After(now.Add(1 * time.Second)) {
			t.Errorf("exited_at is in the future: %v", finalMeta.ExitedAt)
		}
		if finalMeta.ExitedAt.Before(now.Add(-5 * time.Second)) {
			t.Errorf("exited_at is too old: %v", finalMeta.ExitedAt)
		}

		// Verify exit_code
		if finalMeta.ExitCode == nil {
			t.Fatal("exit_code should not be null")
		}
		if *finalMeta.ExitCode != exitCode {
			t.Errorf("exit_code: got %d, want %d", *finalMeta.ExitCode, exitCode)
		}

		// Verify unchanged fields
		if finalMeta.ID != initialMeta.ID {
			t.Errorf("id changed: got %s, want %s", finalMeta.ID, initialMeta.ID)
		}
		if finalMeta.Pid != initialMeta.Pid {
			t.Errorf("pid changed: got %d, want %d", finalMeta.Pid, initialMeta.Pid)
		}
		if finalMeta.Cwd != initialMeta.Cwd {
			t.Errorf("cwd changed: got %q, want %q", finalMeta.Cwd, initialMeta.Cwd)
		}
		if finalMeta.StartedAt != initialMeta.StartedAt {
			t.Errorf("started_at changed: got %v, want %v", finalMeta.StartedAt, initialMeta.StartedAt)
		}
		if finalMeta.Version != initialMeta.Version {
			t.Errorf("version changed: got %d, want %d", finalMeta.Version, initialMeta.Version)
		}
	})

	t.Run("Two simultaneous Create calls produce distinct directories", func(t *testing.T) {
		cwd := "/some/cwd"
		cmd := []string{"npm", "run", "dev"}

		var s1, s2 *Session
		var err1, err2 error
		var wg sync.WaitGroup

		wg.Add(2)

		go func() {
			defer wg.Done()
			s1, err1 = Create(cwd, cmd)
		}()

		go func() {
			defer wg.Done()
			s2, err2 = Create(cwd, cmd)
		}()

		wg.Wait()

		if err1 != nil {
			t.Fatalf("First Create failed: %v", err1)
		}
		if err2 != nil {
			t.Fatalf("Second Create failed: %v", err2)
		}

		// Verify both directories exist
		if _, err := os.Stat(s1.Dir); err != nil {
			t.Fatalf("First session directory does not exist: %v", err)
		}
		if _, err := os.Stat(s2.Dir); err != nil {
			t.Fatalf("Second session directory does not exist: %v", err)
		}

		// Verify directories are distinct
		if s1.Dir == s2.Dir {
			t.Fatalf("Both creates returned the same directory: %s", s1.Dir)
		}

		// Verify both have valid meta.json
		for i, s := range []*Session{s1, s2} {
			metaPath := filepath.Join(s.Dir, "meta.json")
			metaData, err := os.ReadFile(metaPath)
			if err != nil {
				t.Fatalf("Session %d: failed to read meta.json: %v", i+1, err)
			}
			var meta Meta
			if err := json.Unmarshal(metaData, &meta); err != nil {
				t.Fatalf("Session %d: failed to unmarshal meta.json: %v", i+1, err)
			}
		}
	})

	t.Run("Finalize is idempotent under double-call", func(t *testing.T) {
		cwd := "/some/cwd"
		cmd := []string{"npm", "run", "dev"}

		s, err := Create(cwd, cmd)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		// First Finalize call
		exitCode := 130
		if err := Finalize(s, exitCode); err != nil {
			t.Fatalf("First Finalize failed: %v", err)
		}

		metaPath := filepath.Join(s.Dir, "meta.json")
		data1, _ := os.ReadFile(metaPath)
		var meta1 Meta
		json.Unmarshal(data1, &meta1)

		// Second Finalize call with same exit code
		if err := Finalize(s, exitCode); err != nil {
			t.Fatalf("Second Finalize failed: %v", err)
		}

		data2, _ := os.ReadFile(metaPath)
		var meta2 Meta
		json.Unmarshal(data2, &meta2)

		// Verify the second call didn't error or corrupt the data
		if meta2.Status != StatusExited {
			t.Errorf("status after second Finalize: got %q, want %q", meta2.Status, StatusExited)
		}
		if meta2.ExitCode == nil || *meta2.ExitCode != exitCode {
			t.Errorf("exit_code after second Finalize: got %v, want %d", meta2.ExitCode, exitCode)
		}
	})
}
