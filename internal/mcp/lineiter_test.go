package mcp

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/Pankaj3112/peek/internal/store"
)

func TestLineIterator(t *testing.T) {
	t.Run("only_current_log", func(t *testing.T) {
		dir := t.TempDir()
		writeLogLines(t, filepath.Join(dir, "output.log"), 5)

		it, err := NewLineIterator(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer it.Close()

		if it.TotalLines() != 5 {
			t.Errorf("expected 5 lines, got %d", it.TotalLines())
		}

		var nums []int
		for {
			n, _, _, ok := it.Next()
			if !ok {
				break
			}
			nums = append(nums, n)
		}
		wantNums := []int{1, 2, 3, 4, 5}
		if !slices.Equal(nums, wantNums) {
			t.Errorf("got %v, want %v", nums, wantNums)
		}
	})

	t.Run("rotated_then_current", func(t *testing.T) {
		dir := t.TempDir()
		writeLogLines(t, filepath.Join(dir, "output.log.1"), 3) // older 3
		writeLogLines(t, filepath.Join(dir, "output.log"), 4)   // newer 4

		it, err := NewLineIterator(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer it.Close()

		if it.TotalLines() != 7 {
			t.Errorf("expected 7, got %d", it.TotalLines())
		}

		var nums []int
		for {
			n, _, _, ok := it.Next()
			if !ok {
				break
			}
			nums = append(nums, n)
		}
		wantNums := []int{1, 2, 3, 4, 5, 6, 7}
		if !slices.Equal(nums, wantNums) {
			t.Errorf("got %v, want %v", nums, wantNums)
		}
	})

	t.Run("seek_to_line", func(t *testing.T) {
		dir := t.TempDir()
		writeLogLines(t, filepath.Join(dir, "output.log.1"), 3)
		writeLogLines(t, filepath.Join(dir, "output.log"), 4)

		it, _ := NewLineIterator(dir)
		defer it.Close()

		if err := it.Seek(5); err != nil {
			t.Fatalf("seek: %v", err)
		}
		n, _, _, ok := it.Next()
		if !ok || n != 5 {
			t.Errorf("after Seek(5), Next returned (%d, %v)", n, ok)
		}
		n, _, _, ok = it.Next()
		if !ok || n != 6 {
			t.Errorf("expected 6, got %d", n)
		}
	})

	t.Run("seek_past_end", func(t *testing.T) {
		dir := t.TempDir()
		writeLogLines(t, filepath.Join(dir, "output.log"), 3)
		it, _ := NewLineIterator(dir)
		defer it.Close()
		// Seek past total: should not error; just become "no more lines".
		if err := it.Seek(100); err != nil {
			t.Errorf("Seek(100) errored: %v (should not)", err)
		}
		_, _, _, ok := it.Next()
		if ok {
			t.Errorf("Next after Seek(100) should be !ok")
		}
	})

	t.Run("empty_session", func(t *testing.T) {
		dir := t.TempDir()
		it, err := NewLineIterator(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer it.Close()
		if it.TotalLines() != 0 {
			t.Errorf("expected 0 lines, got %d", it.TotalLines())
		}
		_, _, _, ok := it.Next()
		if ok {
			t.Errorf("Next on empty should return !ok")
		}
	})

	t.Run("parses_timestamp_and_text", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "output.log")
		os.WriteFile(path, []byte("2026-05-03T14:23:01.234Z hello world\n"), 0o644)

		it, _ := NewLineIterator(dir)
		defer it.Close()
		n, ts, text, ok := it.Next()
		if !ok || n != 1 {
			t.Errorf("expected line 1, got %d ok=%v", n, ok)
		}
		wantTs := time.Date(2026, 5, 3, 14, 23, 1, 234*int(time.Millisecond), time.UTC)
		if !ts.Equal(wantTs) {
			t.Errorf("timestamp: got %v, want %v", ts, wantTs)
		}
		if text != "hello world" {
			t.Errorf("text: got %q, want hello world", text)
		}
	})
}

// writeLogLines writes N lines with synthetic timestamps to the given path.
func writeLogLines(t *testing.T, path string, n int) {
	t.Helper()
	var buf bytes.Buffer
	base := time.Date(2026, 5, 3, 14, 0, 0, 0, time.UTC)
	for i := 1; i <= n; i++ {
		ts := base.Add(time.Duration(i) * time.Second)
		fmt.Fprintf(&buf, "%s line%d\n", store.FormatTimestamp(ts), i)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
