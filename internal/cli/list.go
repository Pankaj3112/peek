package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Pankaj3112/peek/internal/store"
)

// RenderList writes a human-readable table of sessions to w.
// termWidth is the terminal width for CMD truncation (fallback 100 if <= 0).
func RenderList(w io.Writer, termWidth int) error {
	views, err := store.Scan()
	if err != nil {
		return err
	}
	if termWidth <= 0 {
		termWidth = 100
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSTATUS\tSTARTED\tCMD\tCWD")

	home, _ := os.UserHomeDir() // best-effort; empty string if it fails

	for _, v := range views {
		m := v.Meta
		id := truncateID(m.ID, 9)
		status := renderStatus(m, v.WrapperDied)
		started := m.StartedAt.In(time.Local).Format("2006-01-02 15:04:05")
		cmd := strings.Join(m.Cmd, " ")
		cwd := substituteHome(m.Cwd, home)

		// Cap CMD at termWidth/3 to avoid overwhelming the table.
		maxCmd := termWidth / 3
		if maxCmd < 10 {
			maxCmd = 10
		}
		if len([]rune(cmd)) > maxCmd {
			// Trim to maxCmd-1 runes and append ellipsis.
			runes := []rune(cmd)
			cmd = string(runes[:maxCmd-1]) + "…"
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", id, status, started, cmd, cwd)
	}
	return tw.Flush()
}

func truncateID(id string, n int) string {
	if len(id) <= n {
		return id
	}
	return id[:n]
}

func renderStatus(m *store.Meta, wrapperDied bool) string {
	if m.Status == store.StatusRunning {
		return "running"
	}
	if m.ExitCode == nil {
		return "exited(?)"
	}
	return fmt.Sprintf("exited(%d)", *m.ExitCode)
}

func substituteHome(path, home string) string {
	if home != "" && strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
