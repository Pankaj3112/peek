//go:build !windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2EWrap(t *testing.T) {
	tmpDir := t.TempDir()
	binary := filepath.Join(tmpDir, "peek")
	home := filepath.Join(tmpDir, "fakehome")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Build the binary fresh.
	buildCmd := exec.Command("go", "build", "-o", binary, ".")
	buildCmd.Dir = filepath.Join(os.Getenv("PWD"), "")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	// Run peek -- echo hello.
	cmd := exec.Command(binary, "--", "echo", "hello")
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("peek -- echo hello failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "hello") {
		t.Errorf("expected stdout to contain hello, got %q", out)
	}

	// Verify session was created.
	sessionsRoot := filepath.Join(home, ".peek", "sessions")
	entries, err := os.ReadDir(sessionsRoot)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected exactly 1 session under %s, got %d (err=%v)", sessionsRoot, len(entries), err)
	}

	// Verify the log contains hello.
	logPath := filepath.Join(sessionsRoot, entries[0].Name(), "output.log")
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(logBytes), "hello") {
		t.Errorf("expected log to contain hello, got %q", logBytes)
	}
}
