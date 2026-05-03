package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestMCPInitialize(t *testing.T) {
	var stdin, stdout, stderr bytes.Buffer
	stdin.WriteString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}` + "\n")

	srv := NewServer(&stdin, &stdout, &stderr, "test-version", "/path/to/peek")
	err := srv.ServeUntilEOF()
	if err != nil {
		t.Fatalf("serve: %v", err)
	}

	// Stdout should contain a single JSON-RPC response.
	var resp map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v\noutput: %s", err, stdout.String())
	}
	if resp["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc=2.0")
	}
	if resp["id"].(float64) != 1 {
		t.Errorf("expected id=1")
	}

	result := resp["result"].(map[string]any)
	if _, ok := result["protocolVersion"]; !ok {
		t.Errorf("missing protocolVersion")
	}
	if _, ok := result["capabilities"]; !ok {
		t.Errorf("missing capabilities")
	}
	serverInfo := result["serverInfo"].(map[string]any)
	if serverInfo["name"] != "peek" {
		t.Errorf("expected name=peek, got %v", serverInfo["name"])
	}
	if serverInfo["version"] != "test-version" {
		t.Errorf("expected version=test-version, got %v", serverInfo["version"])
	}

	// Stderr should contain version + binary path log line.
	if !strings.Contains(stderr.String(), "test-version") {
		t.Errorf("stderr missing version log: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "/path/to/peek") {
		t.Errorf("stderr missing binary path: %q", stderr.String())
	}
}

func TestMCPToolsList(t *testing.T) {
	var stdin, stdout, stderr bytes.Buffer
	stdin.WriteString(`{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n")
	stdin.WriteString(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n")

	srv := NewServer(&stdin, &stdout, &stderr, "dev", "/peek")
	_ = srv.ServeUntilEOF()

	// Stdout has two JSON-RPC responses, newline-separated.
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d\noutput: %s", len(lines), stdout.String())
	}

	var listResp map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &listResp); err != nil {
		t.Fatalf("parse list response: %v", err)
	}

	result := listResp["result"].(map[string]any)
	tools := result["tools"].([]any)
	if len(tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(tools))
	}

	names := map[string]bool{}
	for _, ti := range tools {
		names[ti.(map[string]any)["name"].(string)] = true
	}
	for _, want := range []string{"list_sessions", "get_logs", "search_logs"} {
		if !names[want] {
			t.Errorf("missing tool: %s", want)
		}
	}

	// search_logs description should mention (?i) and (?m) flags.
	for _, ti := range tools {
		tool := ti.(map[string]any)
		if tool["name"] == "search_logs" {
			desc := tool["description"].(string)
			if !strings.Contains(desc, "(?i)") || !strings.Contains(desc, "(?m)") {
				t.Errorf("search_logs description missing regex flags: %s", desc)
			}
		}
	}
}

func TestMCPUnknownMethod(t *testing.T) {
	var stdin, stdout, stderr bytes.Buffer
	stdin.WriteString(`{"jsonrpc":"2.0","id":1,"method":"unknown/method"}` + "\n")
	srv := NewServer(&stdin, &stdout, &stderr, "dev", "/peek")
	_ = srv.ServeUntilEOF()

	var resp map[string]any
	json.Unmarshal(stdout.Bytes(), &resp)
	errObj := resp["error"].(map[string]any)
	if int(errObj["code"].(float64)) != -32601 {
		t.Errorf("expected -32601, got %v", errObj["code"])
	}
}
