package wrapper

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	pty "github.com/aymanbagabas/go-pty"
	"golang.org/x/term"

	"github.com/Pankaj3112/peek/internal/ansi"
	"github.com/Pankaj3112/peek/internal/store"
)

// parentTerminalSize queries the parent terminal size.
// Falls back to 80×24 (cols×rows) if stdin is not a terminal or if GetSize fails.
func parentTerminalSize() (rows, cols uint16) {
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		w, h, err := term.GetSize(fd)
		if err == nil && w > 0 && h > 0 {
			return uint16(h), uint16(w)
		}
	}
	return 24, 80
}

// Wrap spawns cmd under a pty, captures output to the session log,
// and returns the child's exit code.
// Signal handling (128+signum) is deferred to Task 25.
// Stdin forwarding is deferred to Task 18.
// Tee to parent terminal is deferred to Task 19.
func Wrap(cwd string, cmd []string) (int, error) {
	rows, cols := parentTerminalSize()
	return wrapWithSize(cwd, cmd, rows, cols)
}

// wrapWithSize is the internal implementation of Wrap that accepts explicit pty dimensions.
// Wrap calls this after querying the parent terminal size (or using defaults).
// This is also used directly by tests to verify pty sizing behaviour.
func wrapWithSize(cwd string, cmd []string, rows, cols uint16) (int, error) {
	// 1. Create session (records cwd, cmd, pid, status=running in meta.json)
	sess, err := store.Create(cwd, cmd)
	if err != nil {
		return -1, err
	}

	// 2. Defer Finalize so meta.json is updated even on panic
	exitCode := -1
	defer func() {
		_ = store.Finalize(sess, exitCode) // best-effort; exit code is captured before return
	}()

	// 3. Open log writer
	lw, err := store.NewLogWriter(sess.Dir, time.Now)
	if err != nil {
		return -1, err
	}
	defer lw.Close()

	// 4. Open pty
	p, err := pty.New()
	if err != nil {
		return -1, err
	}
	defer p.Close()

	// 5. Set pty size before starting the child so the child inherits correct dimensions.
	// go-pty Resize signature: Resize(width int, height int) error
	if err := p.Resize(int(cols), int(rows)); err != nil {
		// Non-fatal: child still works at default pty size.
		fmt.Fprintf(os.Stderr, "peek: pty resize failed: %v\n", err)
	}

	// 6. Build the command attached to the pty
	c := p.Command(cmd[0], cmd[1:]...)

	// Inject PEEK_SESSION_ID into child environment
	c.Env = append(os.Environ(), "PEEK_SESSION_ID="+sess.Meta.ID)

	if err := c.Start(); err != nil {
		return -1, err
	}

	// 7. Start the signal loop in a goroutine; cancel it when Wrap returns.
	sigCtx, sigCancel := context.WithCancel(context.Background())
	defer sigCancel()
	go signalLoop(sigCtx, p, parentTerminalSize)

	// 8. Goroutine: pump pty master output through LineDiscipline → LogWriter
	ld := ansi.NewLineDiscipline(func(line []byte) {
		if werr := lw.WriteLine(line); werr != nil {
			fmt.Fprintf(os.Stderr, "peek: log write failed: %v\n", werr)
		}
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		// io.Copy reads from the pty master until EOF (child exited / pty closed)
		io.Copy(ld, p) //nolint:errcheck // EOF on pty close is expected
	}()

	// 9. Wait for child to exit
	waitErr := c.Wait()

	// Wait for byte pump to drain before closing the log
	<-done

	// Flush any partial line buffered in the line discipline
	_ = ld.Close()

	// 10. Capture exit code
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Unexpected error (failed Wait, etc.)
			return -1, waitErr
		}
	} else {
		exitCode = 0
	}

	return exitCode, nil
}
