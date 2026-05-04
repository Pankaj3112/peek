package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/Pankaj3112/peek/internal/platform"
	"github.com/Pankaj3112/peek/internal/store"
)

type searchLogsArgs struct {
	ID         string `json:"id"`
	Pattern    string `json:"pattern"`
	Context    *int   `json:"context,omitempty"`
	MaxMatches *int   `json:"max_matches,omitempty"`
}

type matchInfo struct {
	line int
	text string // the original text of the matched line (no ts prefix)
}

func searchLogsHandler(ctx context.Context, raw json.RawMessage) (any, error) {
	var args searchLogsArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("parse args: %w", err)
	}
	if args.ID == "" {
		return nil, errors.New("id is required")
	}
	if args.Pattern == "" {
		return nil, errors.New("pattern is required")
	}

	// Defaults and bounds.
	contextLines := 3
	if args.Context != nil {
		if *args.Context < 0 {
			return nil, errors.New("context must be >= 0")
		}
		if *args.Context > 50 {
			return nil, errors.New("context must be <= 50")
		}
		contextLines = *args.Context
	}
	maxMatches := 50
	if args.MaxMatches != nil {
		if *args.MaxMatches < 1 {
			return nil, errors.New("max_matches must be >= 1")
		}
		if *args.MaxMatches > 200 {
			return nil, errors.New("max_matches must be <= 200")
		}
		maxMatches = *args.MaxMatches
	}

	// Compile the regex (RE2).
	re, err := regexp.Compile(args.Pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex: %w", err)
	}

	// Resolve session.
	sessionDir, err := platform.SessionDir(args.ID)
	if err != nil {
		return nil, err
	}
	metaPath := filepath.Join(sessionDir, "meta.json")
	m, err := store.ReadMeta(metaPath)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("session not found: %s", args.ID)
	}

	// First pass: find all matches by scanning every line.
	it, err := NewLineIterator(sessionDir)
	if err != nil {
		return nil, err
	}
	defer it.Close()

	var allMatches []matchInfo
	for {
		n, _, text, ok := it.Next()
		if !ok {
			break
		}
		if re.MatchString(text) {
			allMatches = append(allMatches, matchInfo{line: n, text: text})
		}
	}
	totalMatches := len(allMatches)

	truncated := false
	var lastMatch *matchInfo
	if len(allMatches) > maxMatches {
		truncated = true
		last := allMatches[len(allMatches)-1]
		lastMatch = &last
		allMatches = allMatches[:maxMatches]
	}

	// Build merged context windows.
	type window struct{ from, to int }
	var windows []window
	matchSet := make(map[int]bool)
	for _, mi := range allMatches {
		matchSet[mi.line] = true
		from := mi.line - contextLines
		if from < 1 {
			from = 1
		}
		to := mi.line + contextLines
		if len(windows) > 0 && from < windows[len(windows)-1].to {
			// Overlapping (strictly, more than one shared line) — extend the last window.
			if to > windows[len(windows)-1].to {
				windows[len(windows)-1].to = to
			}
		} else {
			windows = append(windows, window{from, to})
		}
	}

	// Second pass: emit lines linearly, tracking windows as we go.
	it2, err := NewLineIterator(sessionDir)
	if err != nil {
		return nil, err
	}
	defer it2.Close()

	total := it2.TotalLines()
	// Clamp window upper bounds to total.
	for i := range windows {
		if windows[i].to > total {
			windows[i].to = total
		}
	}

	var buf bytes.Buffer
	windowIdx := 0
	for {
		n, ts, text, ok := it2.Next()
		if !ok {
			break
		}
		if windowIdx >= len(windows) {
			break
		}
		w := windows[windowIdx]
		if n < w.from {
			continue
		}
		if n > w.to {
			// Current line is past this window; advance to the next.
			windowIdx++
			if windowIdx >= len(windows) {
				break
			}
			buf.WriteString("--\n")
			w = windows[windowIdx]
			if n < w.from {
				continue
			}
		}
		sep := "-"
		if matchSet[n] {
			sep = ":"
		}
		fmt.Fprintf(&buf, "%d%s%s %s\n", n, sep, store.FormatTimestamp(ts), text)
	}

	resp := map[string]any{
		"matches":       buf.String(),
		"match_count":   len(allMatches),
		"total_matches": totalMatches,
		"truncated":     truncated,
	}
	if truncated && lastMatch != nil {
		resp["last_match"] = map[string]any{
			"line": lastMatch.line,
			"text": lastMatch.text,
		}
	}
	return resp, nil
}

// RegisterSearchLogs adds the search_logs tool handler to the server.
func RegisterSearchLogs(s *Server) {
	s.RegisterHandler("search_logs", searchLogsHandler)
}
