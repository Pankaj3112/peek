package store

import (
	"fmt"
	"time"
)

// MetaVersion is the version of the meta.json schema.
const MetaVersion = 1

// Status represents the status of a session.
type Status string

const (
	StatusRunning Status = "running"
	StatusExited  Status = "exited"
)

// UnmarshalText implements the encoding.TextUnmarshaler interface for Status.
// It validates that the incoming status value is one of the known statuses.
func (s *Status) UnmarshalText(b []byte) error {
	status := Status(b)
	switch status {
	case StatusRunning, StatusExited:
		*s = status
		return nil
	default:
		return fmt.Errorf("invalid status: %q", string(b))
	}
}

// Meta represents the metadata of a session, serialized to meta.json on disk.
type Meta struct {
	Version int    `json:"version"`
	ID      string `json:"id"`
	// Pid is the wrapper process's PID (not the wrapped child's). Used for read-time crash detection.
	Pid       int        `json:"pid"`
	Cwd       string     `json:"cwd"`
	Cmd       []string   `json:"cmd"`
	StartedAt time.Time  `json:"started_at"`
	Status    Status     `json:"status"`
	ExitedAt  *time.Time `json:"exited_at"`
	ExitCode  *int       `json:"exit_code"`
}

// NewRunningMeta creates a new Meta struct for a running session.
func NewRunningMeta(id string, pid int, cwd string, cmd []string) *Meta {
	return &Meta{
		Version:   MetaVersion,
		ID:        id,
		Pid:       pid,
		Cwd:       cwd,
		Cmd:       cmd,
		StartedAt: time.Now().UTC().Truncate(time.Millisecond),
		Status:    StatusRunning,
		ExitedAt:  nil,
		ExitCode:  nil,
	}
}
