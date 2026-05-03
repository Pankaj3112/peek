package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogWriter(t *testing.T) {
	t.Run("opens file for append", func(t *testing.T) {
		tmpDir := t.TempDir()
		clockFunc := func() time.Time {
			return time.Date(2026, 5, 3, 14, 23, 1, 234*int(time.Millisecond), time.UTC)
		}
		lw, err := NewLogWriter(tmpDir, clockFunc)
		if err != nil {
			t.Fatalf("NewLogWriter failed: %v", err)
		}
		defer lw.Close()

		logPath := filepath.Join(tmpDir, "output.log")
		if _, err := os.Stat(logPath); err != nil {
			t.Fatalf("output.log does not exist: %v", err)
		}
	})

	t.Run("append mode does not truncate", func(t *testing.T) {
		tmpDir := t.TempDir()
		clockFunc := func() time.Time {
			return time.Date(2026, 5, 3, 14, 23, 1, 234*int(time.Millisecond), time.UTC)
		}

		// First write
		lw, err := NewLogWriter(tmpDir, clockFunc)
		if err != nil {
			t.Fatalf("NewLogWriter failed: %v", err)
		}
		if err := lw.WriteLine([]byte("first line")); err != nil {
			t.Fatalf("WriteLine failed: %v", err)
		}
		lw.Close()

		// Second write to same file
		lw2, err := NewLogWriter(tmpDir, clockFunc)
		if err != nil {
			t.Fatalf("NewLogWriter second time failed: %v", err)
		}
		if err := lw2.WriteLine([]byte("second line")); err != nil {
			t.Fatalf("WriteLine failed: %v", err)
		}
		lw2.Close()

		// Read and verify both lines are present
		logPath := filepath.Join(tmpDir, "output.log")
		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}

		contentStr := string(content)
		if !strings.Contains(contentStr, "first line") {
			t.Errorf("first line not found in output")
		}
		if !strings.Contains(contentStr, "second line") {
			t.Errorf("second line not found in output")
		}

		// Count newlines; should be 2
		lineCount := strings.Count(contentStr, "\n")
		if lineCount != 2 {
			t.Errorf("expected 2 lines, got %d", lineCount)
		}
	})

	t.Run("writes line with timestamp prefix", func(t *testing.T) {
		tmpDir := t.TempDir()
		clockFunc := func() time.Time {
			return time.Date(2026, 5, 3, 14, 23, 1, 234*int(time.Millisecond), time.UTC)
		}
		lw, err := NewLogWriter(tmpDir, clockFunc)
		if err != nil {
			t.Fatalf("NewLogWriter failed: %v", err)
		}
		defer lw.Close()

		if err := lw.WriteLine([]byte("hello world")); err != nil {
			t.Fatalf("WriteLine failed: %v", err)
		}

		logPath := filepath.Join(tmpDir, "output.log")
		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}

		expected := "2026-05-03T14:23:01.234Z hello world\n"
		if string(content) != expected {
			t.Errorf("expected %q, got %q", expected, string(content))
		}
	})

	t.Run("multiple writes with different timestamps", func(t *testing.T) {
		tmpDir := t.TempDir()

		i := 0
		baseTime := time.Date(2026, 5, 3, 14, 23, 1, 0, time.UTC)
		clockFunc := func() time.Time {
			i++
			return baseTime.Add(time.Duration(i) * time.Second)
		}

		lw, err := NewLogWriter(tmpDir, clockFunc)
		if err != nil {
			t.Fatalf("NewLogWriter failed: %v", err)
		}
		defer lw.Close()

		if err := lw.WriteLine([]byte("line1")); err != nil {
			t.Fatalf("WriteLine 1 failed: %v", err)
		}
		if err := lw.WriteLine([]byte("line2")); err != nil {
			t.Fatalf("WriteLine 2 failed: %v", err)
		}
		if err := lw.WriteLine([]byte("line3")); err != nil {
			t.Fatalf("WriteLine 3 failed: %v", err)
		}

		logPath := filepath.Join(tmpDir, "output.log")
		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}

		contentStr := string(content)
		lines := strings.Split(strings.TrimSuffix(contentStr, "\n"), "\n")
		if len(lines) != 3 {
			t.Errorf("expected 3 lines, got %d", len(lines))
		}

		// Verify each line has a different timestamp
		timestamps := make(map[string]bool)
		for _, line := range lines {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) < 1 {
				t.Errorf("line does not have timestamp: %q", line)
			}
			timestamps[parts[0]] = true
		}

		if len(timestamps) != 3 {
			t.Errorf("expected 3 unique timestamps, got %d", len(timestamps))
		}
	})

	t.Run("rejects embedded newlines", func(t *testing.T) {
		tmpDir := t.TempDir()
		clockFunc := func() time.Time {
			return time.Date(2026, 5, 3, 14, 23, 1, 234*int(time.Millisecond), time.UTC)
		}
		lw, err := NewLogWriter(tmpDir, clockFunc)
		if err != nil {
			t.Fatalf("NewLogWriter failed: %v", err)
		}
		defer lw.Close()

		err = lw.WriteLine([]byte("foo\nbar"))
		if err == nil {
			t.Errorf("expected error for embedded newline, got nil")
		}

		if !strings.Contains(err.Error(), "embedded newline") && !strings.Contains(err.Error(), "newline") {
			t.Errorf("error message should mention newline: %v", err)
		}

		// Verify nothing was written
		logPath := filepath.Join(tmpDir, "output.log")
		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}

		if len(content) != 0 {
			t.Errorf("expected no content written, got %q", string(content))
		}
	})

	t.Run("writes empty line with timestamp", func(t *testing.T) {
		tmpDir := t.TempDir()
		clockFunc := func() time.Time {
			return time.Date(2026, 5, 3, 14, 23, 1, 234*int(time.Millisecond), time.UTC)
		}
		lw, err := NewLogWriter(tmpDir, clockFunc)
		if err != nil {
			t.Fatalf("NewLogWriter failed: %v", err)
		}
		defer lw.Close()

		if err := lw.WriteLine([]byte("")); err != nil {
			t.Fatalf("WriteLine failed: %v", err)
		}

		logPath := filepath.Join(tmpDir, "output.log")
		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}

		expected := "2026-05-03T14:23:01.234Z \n"
		if string(content) != expected {
			t.Errorf("expected %q, got %q", expected, string(content))
		}
	})

	t.Run("close flushes and prevents further writes", func(t *testing.T) {
		tmpDir := t.TempDir()
		clockFunc := func() time.Time {
			return time.Date(2026, 5, 3, 14, 23, 1, 234*int(time.Millisecond), time.UTC)
		}
		lw, err := NewLogWriter(tmpDir, clockFunc)
		if err != nil {
			t.Fatalf("NewLogWriter failed: %v", err)
		}

		if err := lw.WriteLine([]byte("before close")); err != nil {
			t.Fatalf("WriteLine before close failed: %v", err)
		}

		if err := lw.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}

		// Verify file has the content
		logPath := filepath.Join(tmpDir, "output.log")
		content, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}

		if !strings.Contains(string(content), "before close") {
			t.Errorf("content not written before close")
		}

		// Try writing after close
		err = lw.WriteLine([]byte("after close"))
		if err == nil {
			t.Errorf("expected error after close, got nil")
		}

		if !strings.Contains(err.Error(), "closed") {
			t.Errorf("error should mention closed: %v", err)
		}
	})
}
