package store

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// MetaVersion is the version of the meta.json schema.
const MetaVersion = 1

// FormatTimestamp converts a time.Time to a string in the format "2006-01-02T15:04:05.000Z".
// The function:
// 1. Converts the time to UTC
// 2. Truncates to millisecond precision
// 3. Formats with always-3-digit fractional seconds and Z suffix
//
// This ensures byte-identical timestamps across peek for string correlation.
func FormatTimestamp(t time.Time) string {
	t = t.UTC().Truncate(time.Millisecond)
	return t.Format("2006-01-02T15:04:05.000Z")
}

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

// metaJSON is an unexported alias type used for custom JSON marshaling/unmarshaling.
// Time fields are serialized as strings to ensure consistent formatting via FormatTimestamp.
type metaJSON struct {
	Version   int      `json:"version"`
	ID        string   `json:"id"`
	Pid       int      `json:"pid"`
	Cwd       string   `json:"cwd"`
	Cmd       []string `json:"cmd"`
	StartedAt string   `json:"started_at"`
	Status    Status   `json:"status"`
	ExitedAt  *string  `json:"exited_at"`
	ExitCode  *int     `json:"exit_code"`
}

// MarshalJSON implements json.Marshaler for Meta.
// Routes StartedAt and ExitedAt through FormatTimestamp to ensure consistent,
// spec-compliant timestamp formatting (always 3-digit fractional seconds, Z suffix, UTC).
func (m *Meta) MarshalJSON() ([]byte, error) {
	var exitedAtStr *string
	if m.ExitedAt != nil {
		formatted := FormatTimestamp(*m.ExitedAt)
		exitedAtStr = &formatted
	}

	mj := metaJSON{
		Version:   m.Version,
		ID:        m.ID,
		Pid:       m.Pid,
		Cwd:       m.Cwd,
		Cmd:       m.Cmd,
		StartedAt: FormatTimestamp(m.StartedAt),
		Status:    m.Status,
		ExitedAt:  exitedAtStr,
		ExitCode:  m.ExitCode,
	}
	return json.Marshal(mj)
}

// UnmarshalJSON implements json.Unmarshaler for Meta.
// Parses timestamp strings back into time.Time, handling both full and truncated fractional seconds.
func (m *Meta) UnmarshalJSON(data []byte) error {
	var mj metaJSON
	if err := json.Unmarshal(data, &mj); err != nil {
		return err
	}

	// Parse StartedAt timestamp
	startedAt, err := time.Parse(time.RFC3339Nano, mj.StartedAt)
	if err != nil {
		return fmt.Errorf("invalid started_at timestamp: %w", err)
	}

	// Parse ExitedAt timestamp if present
	var exitedAt *time.Time
	if mj.ExitedAt != nil {
		parsed, err := time.Parse(time.RFC3339Nano, *mj.ExitedAt)
		if err != nil {
			return fmt.Errorf("invalid exited_at timestamp: %w", err)
		}
		exitedAt = &parsed
	}

	// Unmarshal Status using its custom unmarshaler
	var status Status
	if err := status.UnmarshalText([]byte(mj.Status)); err != nil {
		return err
	}

	// Populate the Meta struct
	m.Version = mj.Version
	m.ID = mj.ID
	m.Pid = mj.Pid
	m.Cwd = mj.Cwd
	m.Cmd = mj.Cmd
	m.StartedAt = startedAt
	m.Status = status
	m.ExitedAt = exitedAt
	m.ExitCode = mj.ExitCode

	return nil
}

// ReadMeta reads and parses a meta.json file from the given path.
// It returns (*Meta, nil) if the file is valid and can be parsed.
// It returns (nil, nil) if the file does not exist or contains malformed JSON (silent skip per spec).
// It returns (nil, error) for other I/O errors (e.g., permission denied).
func ReadMeta(path string) (*Meta, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, nil
	}
	return &m, nil
}
