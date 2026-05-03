package wrapper

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// ptyResizer is a minimal interface satisfied by go-pty's Pty.
// Using an interface here lets tests inject a mock without a real pty.
// go-pty Pty.Resize signature: Resize(width int, height int) error
type ptyResizer interface {
	Resize(width, height int) error
}

// signalLoop runs in a goroutine and dispatches signals to the appropriate
// handlers. Returns when ctx is cancelled.
//
// Phase 4 wires only SIGWINCH (Task 17). Tasks 21-25 add SIGINT/SIGTERM/etc.
func signalLoop(ctx context.Context, p ptyResizer, getSize func() (rows, cols uint16)) {
	sigs := platformSignals()
	if len(sigs) == 0 {
		// Nothing to listen for on this platform (e.g. Windows has no SIGWINCH).
		return
	}

	ch := make(chan os.Signal, 8)
	signal.Notify(ch, sigs...)
	defer signal.Stop(ch)

	for {
		select {
		case <-ctx.Done():
			return
		case sig := <-ch:
			handleSignal(sig, p, getSize)
		}
	}
}

// handleSignal dispatches a single signal.
func handleSignal(sig os.Signal, p ptyResizer, getSize func() (rows, cols uint16)) {
	switch sig {
	case syscall.SIGWINCH:
		handleSIGWINCH(p, getSize)
		// SIGINT, SIGTERM, SIGHUP, SIGQUIT, SIGTSTP, SIGCONT added in Tasks 21-25.
	}
}

// handleSIGWINCH re-queries the terminal size and propagates it to the pty.
func handleSIGWINCH(p ptyResizer, getSize func() (rows, cols uint16)) {
	rows, cols := getSize()
	// go-pty Resize(width, height): pass cols as width, rows as height.
	_ = p.Resize(int(cols), int(rows)) // best-effort; ignore resize errors
}
