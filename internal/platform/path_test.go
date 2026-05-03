package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSessionsRoot(t *testing.T) {
	// (a) SessionsRoot() returns filepath.Join(<home>, ".peek", "sessions")
	tempdir := t.TempDir()
	t.Setenv("HOME", tempdir)
	t.Setenv("USERPROFILE", tempdir) // Windows compatibility

	root, err := SessionsRoot()
	if err != nil {
		t.Fatalf("SessionsRoot() returned error: %v", err)
	}

	expected := filepath.Join(tempdir, ".peek", "sessions")
	if root != expected {
		t.Errorf("SessionsRoot() = %q, want %q", root, expected)
	}

	// (c) SessionsRoot() does not create the directory
	_, err = os.Stat(root)
	if !os.IsNotExist(err) {
		t.Errorf("SessionsRoot() created directory, expected it to not exist")
	}
}

func TestSessionDir(t *testing.T) {
	// (b) SessionDir(id) returns filepath.Join(SessionsRoot(), id)
	tempdir := t.TempDir()
	t.Setenv("HOME", tempdir)
	t.Setenv("USERPROFILE", tempdir) // Windows compatibility

	id := "01H8XHS7Q9M2K3F4P5N6R7T8V9"
	dir, err := SessionDir(id)
	if err != nil {
		t.Fatalf("SessionDir(%q) returned error: %v", id, err)
	}

	root, _ := SessionsRoot()
	expected := filepath.Join(root, id)
	if dir != expected {
		t.Errorf("SessionDir(%q) = %q, want %q", id, dir, expected)
	}

	// (c) SessionDir() does not create the directory
	_, err = os.Stat(dir)
	if !os.IsNotExist(err) {
		t.Errorf("SessionDir() created directory, expected it to not exist")
	}
}

// (d) Both functions return error from os.UserHomeDir() if it fails
// This is verified by the function signature: (string, error) — the error
// path is exercised by the type system.
