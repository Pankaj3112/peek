package platform

import (
	"os"
	"path/filepath"
)

// SessionsRoot returns the root directory for peek sessions: ~/.peek/sessions
func SessionsRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".peek", "sessions"), nil
}

// SessionDir returns the directory path for a specific session by id.
func SessionDir(id string) (string, error) {
	root, err := SessionsRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, id), nil
}
