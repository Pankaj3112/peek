package wrapper

import (
	"io"
	"os"

	"golang.org/x/term"
)

// enableRawModeIfTerminal puts os.Stdin in raw mode if it is a terminal.
// Returns a restore function. The caller MUST defer the restore function
// immediately, BEFORE any other defers, so panics don't leave the terminal broken.
//
// If stdin is not a terminal (test env, redirected pipe), this is a no-op
// and returns a no-op restore.
func enableRawModeIfTerminal() (restore func(), err error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return func() {}, nil
	}
	state, err := term.MakeRaw(fd)
	if err != nil {
		return func() {}, err
	}
	return func() {
		_ = term.Restore(fd, state)
	}, nil
}

// forwardStdin starts a goroutine that copies bytes from src to the pty master.
// The goroutine exits naturally when src is closed (EOF).
// For tests, src is a pipe that is closed after test input is written.
// For production use, src is os.Stdin and the goroutine exits when the user
// types EOF (Ctrl+D) or closes the terminal.
func forwardStdin(dst io.Writer, src io.Reader) {
	go func() {
		_, _ = io.Copy(dst, src)
	}()
}
