package wrapper

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// gracefulShutdownTimeout is the time to wait for the child to exit after
// SIGINT/SIGTERM/SIGHUP before escalating to SIGKILL.
// Exported as a package-level var so tests can override it temporarily.
var gracefulShutdownTimeout = 5 * time.Second

// ptyResizer is a minimal interface satisfied by go-pty's Pty.
// Using an interface here lets tests inject a mock without a real pty.
// go-pty Pty.Resize signature: Resize(width int, height int) error
type ptyResizer interface {
	Resize(width, height int) error
}

// signalState holds the mutable state shared between the signal loop and the
// graceful-shutdown timer goroutine.
type signalState struct {
	proc *os.Process // child process (to send signals via Signal)

	gracefulMu      sync.Mutex
	gracefulPending bool
	gracefulTimer   *time.Timer
}

// signalLoop runs in a goroutine and dispatches signals to the appropriate
// handlers. Returns when ctx is cancelled.
//
// Phase 4 wires SIGWINCH (Task 17). Tasks 21-25 add SIGINT/SIGTERM/etc.
func signalLoop(ctx context.Context, proc *os.Process, p ptyResizer, getSize func() (rows, cols uint16)) {
	sigs := platformSignals()
	if len(sigs) == 0 {
		// Nothing to listen for on this platform (e.g. Windows has no signals).
		return
	}

	ch := make(chan os.Signal, 8)
	signal.Notify(ch, sigs...)
	defer signal.Stop(ch)

	state := &signalState{proc: proc}

	for {
		select {
		case <-ctx.Done():
			return
		case sig := <-ch:
			handleSignal(sig, state, p, getSize)
		}
	}
}

// handleSignal dispatches a single signal.
func handleSignal(sig os.Signal, state *signalState, p ptyResizer, getSize func() (rows, cols uint16)) {
	switch sig {
	case syscall.SIGWINCH:
		handleSIGWINCH(p, getSize)
	case syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP:
		forwardWithGraceful(sig, state)
	case syscall.SIGQUIT:
		if state != nil && state.proc != nil {
			_ = state.proc.Signal(syscall.SIGQUIT)
		}
	case syscallSIGTSTP():
		handleTSTP(state)
	case syscallSIGCONT():
		handleCONT(state)
	}
}

// forwardWithGraceful forwards sig to the child process and starts a graceful
// shutdown timer. If a second signal arrives while the timer is pending, it
// immediately SIGKILLs the child (Task 22 double-INT behaviour).
func forwardWithGraceful(sig os.Signal, state *signalState) {
	if state == nil || state.proc == nil {
		return
	}

	state.gracefulMu.Lock()
	defer state.gracefulMu.Unlock()

	if state.gracefulPending {
		// Second signal within the graceful window: cancel timer, SIGKILL now.
		// (Task 22 introduces this path; Task 21 forward-declares it as SIGKILL.)
		if state.gracefulTimer != nil {
			state.gracefulTimer.Stop()
			state.gracefulTimer = nil
		}
		_ = state.proc.Signal(syscall.SIGKILL)
		return
	}

	state.gracefulPending = true
	_ = state.proc.Signal(sig)
	proc := state.proc
	state.gracefulTimer = time.AfterFunc(gracefulShutdownTimeout, func() {
		// Child didn't exit within gracefulShutdownTimeout — escalate to SIGKILL.
		_ = proc.Signal(syscall.SIGKILL)
	})
}

// handleSIGWINCH re-queries the terminal size and propagates it to the pty.
func handleSIGWINCH(p ptyResizer, getSize func() (rows, cols uint16)) {
	if p == nil || getSize == nil {
		return
	}
	rows, cols := getSize()
	// go-pty Resize(width, height): pass cols as width, rows as height.
	_ = p.Resize(int(cols), int(rows)) // best-effort; ignore resize errors
}
