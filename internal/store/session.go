package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/Pankaj3112/peek/internal/platform"
)

// Session represents a running or completed session.
type Session struct {
	Dir  string
	Meta *Meta
}

// Create creates a new session with the given cwd and command.
// It returns a Session with a directory in ~/.peek/sessions/<ulid>/ and writes meta.json atomically.
func Create(cwd string, cmd []string) (*Session, error) {
	// 1. Generate a new ID
	id := NewID()

	// 2. Get the session directory
	dir, err := platform.SessionDir(id)
	if err != nil {
		return nil, err
	}

	// 3. Create the directory
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	// 4. Build meta
	m := NewRunningMeta(id, os.Getpid(), cwd, cmd)

	// 5. Atomically write meta.json
	if err := writeMetaAtomic(dir, m); err != nil {
		return nil, err
	}

	// 6. Return the Session
	return &Session{Dir: dir, Meta: m}, nil
}

// Finalize updates the session's meta.json to the exited state.
// It sets status to "exited", records the exit time and code, then atomically writes meta.json.
func Finalize(s *Session, exitCode int) error {
	// 1. Update s.Meta in place
	s.Meta.Status = StatusExited
	now := time.Now().UTC().Truncate(time.Millisecond)
	s.Meta.ExitedAt = &now
	s.Meta.ExitCode = &exitCode

	// 2. Atomically write to s.Dir/meta.json
	return writeMetaAtomic(s.Dir, s.Meta)
}

// writeMetaAtomic atomically writes meta to dir/meta.json using a temp file + fsync + rename pattern.
func writeMetaAtomic(dir string, m *Meta) error {
	// 1. Marshal m to pretty-printed JSON
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	// 2. Write to temp file
	tmpPath := filepath.Join(dir, "meta.json.tmp")
	if err := os.WriteFile(tmpPath, b, 0o644); err != nil {
		return err
	}

	// 3. Open temp file and fsync
	f, err := os.OpenFile(tmpPath, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	f.Close()

	// 4. Rename to final location
	metaPath := filepath.Join(dir, "meta.json")
	if err := os.Rename(tmpPath, metaPath); err != nil {
		return err
	}

	return nil
}
