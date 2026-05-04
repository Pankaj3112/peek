package mcp

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

	"github.com/Pankaj3112/peek/internal/store"
)

func TestSearchLogsHandler(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Seed an exited session with a known log:
	// - 5 normal lines, then 1 ERROR line, then 5 normal lines, then 1 ERROR line, then 5 normal lines.
	sessionDir := filepath.Join(home, ".peek", "sessions", "01HSEARCHQQQQQQQQQQQQQQQQQ")
	os.MkdirAll(sessionDir, 0o755)
	seedExitedSessionMeta(t, sessionDir, "01HSEARCHQQQQQQQQQQQQQQQQQ", 0)
	writeMixedLogContent(t, filepath.Join(sessionDir, "output.log"))
	// Total lines: 17. Two ERROR matches at line 6 and 12.

	t.Run("basic_match", func(t *testing.T) {
		result := callSearchLogs(t, `{"id":"01HSEARCHQQQQQQQQQQQQQQQQQ","pattern":"ERROR"}`)
		if result["match_count"].(float64) != 2 {
			t.Errorf("expected 2 matches, got %v", result["match_count"])
		}
		matches := result["matches"].(string)
		// Must contain both ERROR matches in chronological order, with grep-style separators.
		if !strings.Contains(matches, "6:") {
			t.Errorf("missing match for line 6: %q", matches)
		}
		if !strings.Contains(matches, "12:") {
			t.Errorf("missing match for line 12: %q", matches)
		}
		// Match line uses ":" separator.
		// Context line uses "-" separator.
		// Lines 6 and 12 are far apart (>= context*2+1 = 7 lines apart): should have "--" separator.
		if !strings.Contains(matches, "\n--\n") {
			t.Errorf("missing -- separator between non-adjacent groups: %q", matches)
		}
		if result["truncated"] != false {
			t.Errorf("truncated=%v, want false", result["truncated"])
		}
	})

	t.Run("regex_flags_case_insensitive", func(t *testing.T) {
		result := callSearchLogs(t, `{"id":"01HSEARCHQQQQQQQQQQQQQQQQQ","pattern":"(?i)error"}`)
		// Same matches as case-sensitive search.
		if result["match_count"].(float64) != 2 {
			t.Errorf("expected 2 with (?i), got %v", result["match_count"])
		}
	})

	t.Run("invalid_regex_errors", func(t *testing.T) {
		_, err := searchLogsHandler(context.Background(), json.RawMessage(`{"id":"01HSEARCHQQQQQQQQQQQQQQQQQ","pattern":"["}`))
		if err == nil {
			t.Errorf("expected error on invalid regex")
		}
	})

	t.Run("context_setting", func(t *testing.T) {
		result := callSearchLogs(t, `{"id":"01HSEARCHQQQQQQQQQQQQQQQQQ","pattern":"ERROR","context":1}`)
		// With context=1: line 6 returns lines 5,6,7. Line 12 returns 11,12,13. Not adjacent; -- separator.
		matches := result["matches"].(string)
		if !strings.Contains(matches, "5-") {
			t.Errorf("missing context line 5")
		}
		if !strings.Contains(matches, "6:") {
			t.Errorf("missing match line 6")
		}
		if !strings.Contains(matches, "7-") {
			t.Errorf("missing context line 7")
		}
		if strings.Contains(matches, "4-") {
			t.Errorf("should not contain line 4 with context=1")
		}
	})

	t.Run("adjacent_match_merging", func(t *testing.T) {
		// Seed a separate session with two close matches.
		adjDir := filepath.Join(home, ".peek", "sessions", "01HADJACENTQQQQQQQQQQQQQQQ")
		os.MkdirAll(adjDir, 0o755)
		seedExitedSessionMeta(t, adjDir, "01HADJACENTQQQQQQQQQQQQQQQ", 0)
		// Lines 1-10. Matches at lines 3 and 5 (within context=3).
		writeAdjacentMatchesContent(t, filepath.Join(adjDir, "output.log"))

		result := callSearchLogs(t, `{"id":"01HADJACENTQQQQQQQQQQQQQQQ","pattern":"ERROR","context":3}`)
		if result["match_count"].(float64) != 2 {
			t.Errorf("expected 2 matches, got %v", result["match_count"])
		}
		matches := result["matches"].(string)
		// Should NOT contain "--" because the two windows merge.
		if strings.Contains(matches, "\n--\n") {
			t.Errorf("adjacent matches should merge, found --: %q", matches)
		}
	})

	t.Run("truncation_keeps_oldest", func(t *testing.T) {
		// Seed a session with 200 ERROR lines.
		manyID := "01HMANYERRORSQQQQQQQQQQQQQ" // 26 chars
		manyDir := filepath.Join(home, ".peek", "sessions", manyID)
		os.MkdirAll(manyDir, 0o755)
		seedExitedSessionMeta(t, manyDir, manyID, 0)
		writeManyErrors(t, filepath.Join(manyDir, "output.log"), 200)

		result := callSearchLogs(t, `{"id":"01HMANYERRORSQQQQQQQQQQQQQ","pattern":"ERROR","max_matches":50}`)
		if result["match_count"].(float64) != 50 {
			t.Errorf("expected 50 returned, got %v", result["match_count"])
		}
		if result["total_matches"].(float64) != 200 {
			t.Errorf("expected 200 total, got %v", result["total_matches"])
		}
		if result["truncated"] != true {
			t.Errorf("expected truncated=true")
		}
		lastMatch := result["last_match"].(map[string]any)
		// The 200th match should be referenced.
		if !strings.Contains(lastMatch["text"].(string), "ERROR") {
			t.Errorf("last_match.text=%q should reference an ERROR line", lastMatch["text"])
		}
	})

	t.Run("max_matches_above_total_no_truncation", func(t *testing.T) {
		result := callSearchLogs(t, `{"id":"01HSEARCHQQQQQQQQQQQQQQQQQ","pattern":"ERROR","max_matches":100}`)
		if result["truncated"] != false {
			t.Errorf("expected truncated=false")
		}
		if _, present := result["last_match"]; present {
			t.Errorf("expected last_match absent when not truncated")
		}
	})
}

// callSearchLogs invokes the handler and returns the parsed result map.
func callSearchLogs(t *testing.T, args string) map[string]any {
	t.Helper()
	res, err := searchLogsHandler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("search_logs error: %v", err)
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

func writeMixedLogContent(t *testing.T, path string) {
	t.Helper()
	base := time.Date(2026, 5, 3, 14, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	for i := 1; i <= 17; i++ {
		ts := base.Add(time.Duration(i) * time.Second)
		text := fmt.Sprintf("normal line %d", i)
		if i == 6 || i == 12 {
			text = fmt.Sprintf("ERROR: failure at line %d", i)
		}
		fmt.Fprintf(&buf, "%s %s\n", store.FormatTimestamp(ts), text)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeAdjacentMatchesContent(t *testing.T, path string) {
	t.Helper()
	base := time.Date(2026, 5, 3, 14, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	for i := 1; i <= 10; i++ {
		ts := base.Add(time.Duration(i) * time.Second)
		text := fmt.Sprintf("line %d", i)
		if i == 3 || i == 5 {
			text = fmt.Sprintf("ERROR at %d", i)
		}
		fmt.Fprintf(&buf, "%s %s\n", store.FormatTimestamp(ts), text)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeManyErrors(t *testing.T, path string, n int) {
	t.Helper()
	base := time.Date(2026, 5, 3, 14, 0, 0, 0, time.UTC)
	var buf bytes.Buffer
	for i := 1; i <= n; i++ {
		ts := base.Add(time.Duration(i) * time.Second)
		fmt.Fprintf(&buf, "%s ERROR %d\n", store.FormatTimestamp(ts), i)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
