package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/Pankaj3112/peek/internal/platform"
	"github.com/Pankaj3112/peek/internal/store"
)

type getLogsArgs struct {
	ID        string `json:"id"`
	Lines     *int   `json:"lines,omitempty"`
	StartLine *int   `json:"start_line,omitempty"`
}

func getLogsHandler(ctx context.Context, raw json.RawMessage) (any, error) {
	var args getLogsArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("parse args: %w", err)
	}
	if args.ID == "" {
		return nil, errors.New("id is required")
	}

	// Default and bound `lines`.
	lineCount := 100
	if args.Lines != nil {
		if *args.Lines < 1 {
			return nil, errors.New("lines must be >= 1")
		}
		if *args.Lines > 1000 {
			return nil, errors.New("lines must be <= 1000")
		}
		lineCount = *args.Lines
	}

	// Validate start_line.
	if args.StartLine != nil && *args.StartLine <= 0 {
		return nil, errors.New("start_line must be >= 1")
	}

	// Resolve session directory.
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

	view := store.ApplyCrashDetection(m)

	// Open the line iterator.
	it, err := NewLineIterator(sessionDir)
	if err != nil {
		return nil, err
	}
	defer it.Close()
	total := it.TotalLines()

	// Determine the line range to return.
	var fromLine, toLine int
	if args.StartLine != nil {
		fromLine = *args.StartLine
		if fromLine > total {
			// Empty result, no error per spec.
			return buildGetLogsResponse("", 0, 0, total, view), nil
		}
		toLine = fromLine + lineCount - 1
		if toLine > total {
			toLine = total
		}
	} else {
		// Tail mode: return last `lineCount` lines.
		toLine = total
		fromLine = total - lineCount + 1
		if fromLine < 1 {
			fromLine = 1
		}
	}

	if total == 0 {
		return buildGetLogsResponse("", 0, 0, 0, view), nil
	}

	// Seek to fromLine and collect lines through toLine.
	if err := it.Seek(fromLine); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	for {
		n, ts, text, ok := it.Next()
		if !ok || n > toLine {
			break
		}
		fmt.Fprintf(&buf, "%d: %s %s\n", n, store.FormatTimestamp(ts), text)
	}

	return buildGetLogsResponse(buf.String(), fromLine, toLine, total, view), nil
}

func buildGetLogsResponse(lines string, from, to, total int, view store.MetaView) map[string]any {
	resp := map[string]any{
		"lines":          lines,
		"from_line":      from,
		"to_line":        to,
		"total_lines":    total,
		"session_status": string(view.Meta.Status),
	}
	if view.WrapperDied {
		resp["wrapper_died"] = true
	}
	return resp
}

// RegisterGetLogs adds the get_logs tool handler to the server.
func RegisterGetLogs(s *Server) {
	s.RegisterHandler("get_logs", getLogsHandler)
}
