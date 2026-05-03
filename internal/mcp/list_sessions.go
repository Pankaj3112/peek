package mcp

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Pankaj3112/peek/internal/store"
)

type listSessionsArgs struct {
	Cwd string `json:"cwd,omitempty"`
}

// listSessionsHandler implements the list_sessions MCP tool. It returns all
// sessions from the store, optionally filtered by cwd using Match.
func listSessionsHandler(ctx context.Context, raw json.RawMessage) (any, error) {
	var args listSessionsArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
	}

	views, err := store.Scan()
	if err != nil {
		return nil, err
	}

	sessions := make([]any, 0, len(views))
	for _, v := range views {
		if !Match(args.Cwd, v.Meta.Cwd) {
			continue
		}
		m := v.Meta
		s := map[string]any{
			"id":         m.ID,
			"cwd":        m.Cwd,
			"cmd":        m.Cmd,
			"started_at": store.FormatTimestamp(m.StartedAt),
			"status":     string(m.Status),
			"exited_at":  formatExitedAt(m.ExitedAt),
			"exit_code":  m.ExitCode,
		}
		if v.WrapperDied {
			s["wrapper_died"] = true
		}
		sessions = append(sessions, s)
	}
	return map[string]any{"sessions": sessions}, nil
}

// formatExitedAt converts an optional *time.Time to a wire-format string, or
// nil when the pointer is nil.
func formatExitedAt(t *time.Time) any {
	if t == nil {
		return nil
	}
	return store.FormatTimestamp(*t)
}

// RegisterListSessions registers the list_sessions tool handler on srv.
// Called from main.go during MCP server startup.
func RegisterListSessions(s *Server) {
	s.RegisterHandler("list_sessions", listSessionsHandler)
}
