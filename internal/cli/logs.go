package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pankaj3112/peek/internal/platform"
	"github.com/Pankaj3112/peek/internal/store"
)

// pollInterval is how often to check for new log lines on a running session.
// Var (not const) so tests can override.
var pollInterval = 100 * time.Millisecond

// ResolveIDPrefix finds the session whose ID starts with the given prefix.
// Returns the full session ID on success.
// Returns an error if there are no matches or more than one match.
func ResolveIDPrefix(prefix string) (string, error) {
	views, err := store.Scan()
	if err != nil {
		return "", err
	}
	var matches []string
	for _, v := range views {
		if strings.HasPrefix(v.Meta.ID, prefix) {
			matches = append(matches, v.Meta.ID)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no session found matching %q", prefix)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("ambiguous prefix %q matches %d sessions: %v", prefix, len(matches), matches)
	}
}

// RenderLogs streams the logs for the session with the given ID to w.
// If the session is running it tails the log until the session exits or ctx is done.
// If the session is exited it dumps the full log and returns.
func RenderLogs(ctx context.Context, w io.Writer, id string) error {
	sessionDir, err := platform.SessionDir(id)
	if err != nil {
		return err
	}

	metaPath := filepath.Join(sessionDir, "meta.json")
	m, err := store.ReadMeta(metaPath)
	if err != nil || m == nil {
		return fmt.Errorf("could not read session meta: %v", err)
	}
	view := store.ApplyCrashDetection(m)

	logPath := filepath.Join(sessionDir, "output.log")
	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // empty session
		}
		return err
	}
	defer f.Close()

	lineNum := 0

	// padNum right-aligns n within width columns.
	padNum := func(n, width int) string {
		s := fmt.Sprintf("%d", n)
		for len(s) < width {
			s = " " + s
		}
		return s
	}

	// emit formats and writes a single log line.
	emit := func(line string) error {
		lineNum++
		// Use width 4 for v1 (up to 9999 lines before unaligned).
		numStr := padNum(lineNum, 4)

		if len(line) < 25 {
			// Malformed or short line; emit raw.
			_, err := fmt.Fprintf(w, "%s  %s\n", numStr, line)
			return err
		}
		tsStr := line[:24] // "2026-05-03T14:23:01.230Z"
		text := line[25:]  // skip the space after the timestamp

		ts, parseErr := time.Parse("2006-01-02T15:04:05.000Z", tsStr)
		if parseErr != nil {
			ts, parseErr = time.Parse(time.RFC3339Nano, tsStr)
		}
		var timeRendered string
		if parseErr == nil {
			timeRendered = ts.In(time.Local).Format("15:04:05.000")
		} else {
			timeRendered = strings.Repeat(" ", 12)
		}

		_, werr := fmt.Fprintf(w, "%s  %s  %s\n", numStr, timeRendered, text)
		return werr
	}

	reader := bufio.NewReader(f)

	// If session is already not running, just dump the file and return.
	if view.Meta.Status != store.StatusRunning {
		for {
			line, err := reader.ReadString('\n')
			if line != "" {
				if emitErr := emit(strings.TrimRight(line, "\n")); emitErr != nil {
					return emitErr
				}
			}
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
		}
	}

	// Session is running: tail-f loop.
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			if emitErr := emit(strings.TrimRight(line, "\n")); emitErr != nil {
				return emitErr
			}
		}
		if err == io.EOF {
			// No more data right now. Wait, then re-check.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(pollInterval):
			}

			// Re-check session status.
			m2, _ := store.ReadMeta(metaPath)
			if m2 == nil {
				return nil
			}
			view2 := store.ApplyCrashDetection(m2)
			if view2.Meta.Status != store.StatusRunning {
				// Session ended. Drain any remaining bytes and return.
				for {
					line2, err2 := reader.ReadString('\n')
					if line2 != "" {
						if emitErr := emit(strings.TrimRight(line2, "\n")); emitErr != nil {
							return emitErr
						}
					}
					if err2 != nil {
						return nil
					}
				}
			}
			continue
		}
		if err != nil {
			return err
		}
	}
}
