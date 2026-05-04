package mcp

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LineIterator walks every line in <session-dir>/output.log.1 (if present)
// followed by output.log, with line numbers global across both files.
//
// Line format on disk: "<RFC3339Nano UTC ms> <text>\n"
type LineIterator struct {
	files      []*os.File
	scanners   []*bufio.Scanner
	fileIdx    int // current file index
	line       int // last-emitted line number (0 = nothing yet)
	totalLines int // computed eagerly
}

// NewLineIterator opens the rotated log files in the given session directory
// and prepares to iterate over them in order (output.log.1 first, then output.log).
// Missing files are silently skipped (an empty session dir is valid).
func NewLineIterator(sessionDir string) (*LineIterator, error) {
	paths := []string{
		filepath.Join(sessionDir, "output.log.1"),
		filepath.Join(sessionDir, "output.log"),
	}
	var files []*os.File
	var scanners []*bufio.Scanner
	total := 0
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue // optional file missing
			}
			// Close anything we opened so far.
			for _, opened := range files {
				opened.Close()
			}
			return nil, err
		}
		// Count lines before adding to iteration list (scanner consumes the file).
		n := countLines(f)
		f.Close()

		// Re-open for iteration.
		f, err = os.Open(p)
		if err != nil {
			for _, opened := range files {
				opened.Close()
			}
			return nil, err
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		files = append(files, f)
		scanners = append(scanners, sc)
		total += n
	}
	return &LineIterator{files: files, scanners: scanners, totalLines: total}, nil
}

func countLines(r io.Reader) int {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	n := 0
	for sc.Scan() {
		n++
	}
	return n
}

// Close closes all underlying file handles.
func (it *LineIterator) Close() error {
	for _, f := range it.files {
		f.Close()
	}
	return nil
}

// TotalLines returns the pre-counted total number of lines across all files.
func (it *LineIterator) TotalLines() int {
	return it.totalLines
}

// Next returns (lineNum, timestamp, text, ok). ok is false at EOF.
func (it *LineIterator) Next() (int, time.Time, string, bool) {
	for it.fileIdx < len(it.scanners) {
		if it.scanners[it.fileIdx].Scan() {
			raw := it.scanners[it.fileIdx].Text()
			it.line++
			ts, text := parseLogLine(raw)
			return it.line, ts, text, true
		}
		// EOF on this scanner — move to next file.
		it.fileIdx++
	}
	return 0, time.Time{}, "", false
}

// Seek advances the iterator so the next Next() call returns line `target`.
// If target > total lines, the iterator becomes empty (Next returns !ok).
// Seek does not error on out-of-range per the get_logs spec.
func (it *LineIterator) Seek(target int) error {
	if target <= 0 {
		return errors.New("line number must be positive")
	}
	// Skip lines until we reach target-1 (the line just before what we want).
	for it.line < target-1 {
		_, _, _, ok := it.Next()
		if !ok {
			return nil // past end; iterator is now empty
		}
	}
	return nil
}

// parseLogLine splits a line formatted as "<24-char ISO timestamp> <text>" into
// its timestamp and text components. Tolerates malformed input by returning a
// zero time and the full raw line as text.
func parseLogLine(line string) (time.Time, string) {
	// Timestamp is exactly 24 chars ("2026-05-03T14:23:01.234Z"), followed by a space.
	if len(line) < 25 || line[24] != ' ' {
		return time.Time{}, line
	}
	tsStr := line[:24]
	text := strings.TrimRight(line[25:], "\r")
	ts, err := time.Parse("2006-01-02T15:04:05.000Z", tsStr)
	if err != nil {
		// Fall back to the more permissive RFC3339Nano.
		ts, err = time.Parse(time.RFC3339Nano, tsStr)
		if err != nil {
			return time.Time{}, line
		}
	}
	return ts, text
}
