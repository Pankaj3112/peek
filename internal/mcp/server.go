package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// Server is a JSON-RPC 2.0 MCP server that reads from in and writes to out.
type Server struct {
	in       io.Reader
	out      io.Writer
	stderr   io.Writer
	version  string
	binary   string
	handlers map[string]toolHandler
}

// toolHandler is a function that handles a tool call.
type toolHandler func(ctx context.Context, args json.RawMessage) (any, error)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// NewServer creates a new MCP server. It logs a startup line to stderr immediately.
func NewServer(in io.Reader, out, stderr io.Writer, version, binary string) *Server {
	s := &Server{
		in:       in,
		out:      out,
		stderr:   stderr,
		version:  version,
		binary:   binary,
		handlers: make(map[string]toolHandler),
	}
	fmt.Fprintf(stderr, "peek mcp %s binary=%s\n", version, binary)
	return s
}

// RegisterHandler registers a tool handler for the given tool name.
// Used by Tasks 32, 37, 38 to add tool implementations.
func (s *Server) RegisterHandler(name string, h toolHandler) {
	s.handlers[name] = h
}

// ServeUntilEOF reads newline-delimited JSON-RPC messages from in, dispatches
// them, and writes responses to out until in returns EOF.
func (s *Server) ServeUntilEOF() error {
	scanner := bufio.NewScanner(s.in)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeError(nil, -32700, "Parse error: "+err.Error())
			continue
		}
		s.handle(req)
	}
	return scanner.Err()
}

func (s *Server) handle(req request) {
	switch req.Method {
	case "initialize":
		s.writeResult(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "peek",
				"version": s.version,
			},
		})
	case "notifications/initialized":
		// Notification — no response (no id).
	case "tools/list":
		s.writeResult(req.ID, map[string]any{
			"tools": s.toolDefinitions(),
		})
	case "tools/call":
		s.handleToolCall(req)
	default:
		s.writeError(req.ID, -32601, "Method not found: "+req.Method)
	}
}

func (s *Server) handleToolCall(req request) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.writeError(req.ID, -32602, "Invalid params: "+err.Error())
		return
	}
	h, ok := s.handlers[params.Name]
	if !ok {
		s.writeError(req.ID, -32601, "Unknown tool: "+params.Name)
		return
	}
	result, err := h(context.Background(), params.Arguments)
	if err != nil {
		s.writeError(req.ID, -32603, err.Error())
		return
	}
	// Wrap result as a text content block with JSON payload.
	payload, _ := json.Marshal(result)
	s.writeResult(req.ID, map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": string(payload)},
		},
	})
}

func (s *Server) writeResult(id json.RawMessage, result any) {
	resp := response{JSONRPC: "2.0", ID: id, Result: result}
	enc := json.NewEncoder(s.out)
	_ = enc.Encode(resp)
}

func (s *Server) writeError(id json.RawMessage, code int, msg string) {
	resp := response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}}
	enc := json.NewEncoder(s.out)
	_ = enc.Encode(resp)
}

// toolDefinitions returns the JSON Schema definitions for all registered tools.
// This is the single source of truth for tool metadata exposed via tools/list.
func (s *Server) toolDefinitions() []map[string]any {
	return []map[string]any{
		{
			"name":        "list_sessions",
			"description": "List captured peek sessions, optionally filtered by cwd (ancestor/exact/descendant match).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"cwd": map[string]any{
						"type":        "string",
						"description": "Optional. Returns sessions whose cwd is exact/ancestor/descendant of this path. Omit for all sessions.",
					},
				},
				"required": []string{},
			},
		},
		{
			"name":        "get_logs",
			"description": "Return log lines from a session's output.log + output.log.1 (spans rotation).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{
						"type":        "string",
						"description": "Session ID (full ULID, exact match).",
					},
					"lines": map[string]any{
						"type":        "integer",
						"description": "How many lines to return. Default 100, max 1000.",
						"minimum":     1,
						"maximum":     1000,
					},
					"start_line": map[string]any{
						"type":        "integer",
						"description": "Optional 1-indexed line to start at. Without start_line, tails the last `lines` lines. With start_line, returns up to `lines` lines forward from there.",
						"minimum":     1,
					},
				},
				"required": []string{"id"},
			},
		},
		{
			"name":        "search_logs",
			"description": "Search a session's logs with a Go regexp/RE2 pattern. Use (?i) for case-insensitive, (?m) for multiline. Returns grep -C style output.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":      map[string]any{"type": "string", "description": "Session ID (full ULID, exact match)."},
					"pattern": map[string]any{"type": "string", "description": "RE2 regex pattern. Prefix with (?i) for case-insensitive."},
					"context": map[string]any{
						"type":        "integer",
						"description": "Lines of context before and after each match. Default 3, max 50.",
						"minimum":     0,
						"maximum":     50,
					},
					"max_matches": map[string]any{
						"type":        "integer",
						"description": "Hard cap on returned matches. Default 50, max 200. Returns oldest matches first; truncation summary in last_match.",
						"minimum":     1,
						"maximum":     200,
					},
				},
				"required": []string{"id", "pattern"},
			},
		},
	}
}
