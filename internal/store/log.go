package store

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LogWriter writes lines to a log file with timestamp prefixes.
type LogWriter struct {
	dir       string
	now       func() time.Time
	f         *os.File // current file (output.log)
	size      int64    // bytes written to current file since open
	threshold int64
}

const defaultRotationThreshold = 50 * 1024 * 1024 // 50 MiB

// NewLogWriter creates a new LogWriter that appends to <dir>/output.log.
// The now parameter is a clock function for testing; production passes time.Now.
func NewLogWriter(dir string, now func() time.Time) (*LogWriter, error) {
	return newLogWriterWithThreshold(dir, now, defaultRotationThreshold)
}

// newLogWriterWithThreshold is the internal constructor used by tests with custom thresholds.
func newLogWriterWithThreshold(dir string, now func() time.Time, threshold int64) (*LogWriter, error) {
	path := filepath.Join(dir, "output.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	return &LogWriter{
		dir:       dir,
		now:       now,
		f:         f,
		size:      info.Size(),
		threshold: threshold,
	}, nil
}

// WriteLine appends a line to the log with a timestamp prefix.
// The line must not contain embedded newlines. The written output is:
// <formatted_now> <line>\n
func (w *LogWriter) WriteLine(line []byte) error {
	if w.f == nil {
		return errors.New("log writer is closed")
	}

	if bytes.IndexByte(line, '\n') >= 0 {
		return fmt.Errorf("WriteLine: embedded newline in input")
	}

	prefix := FormatTimestamp(w.now()) + " "
	// Build the full line in a single Write to keep it atomic on POSIX.
	var buf []byte
	buf = append(buf, prefix...)
	buf = append(buf, line...)
	buf = append(buf, '\n')

	n, err := w.f.Write(buf)
	w.size += int64(n)
	if err != nil {
		return err
	}

	if w.size >= w.threshold {
		if err := w.rotate(); err != nil {
			return err
		}
	}

	return nil
}

// rotate renames the current output.log to output.log.1 and opens a fresh output.log.
func (w *LogWriter) rotate() error {
	if err := w.f.Close(); err != nil {
		return err
	}

	cur := filepath.Join(w.dir, "output.log")
	rotated := filepath.Join(w.dir, "output.log.1")

	if err := os.Rename(cur, rotated); err != nil {
		return err
	}

	f, err := os.OpenFile(cur, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}

	w.f = f
	w.size = 0
	return nil
}

// Close flushes and closes the log file.
func (w *LogWriter) Close() error {
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}
