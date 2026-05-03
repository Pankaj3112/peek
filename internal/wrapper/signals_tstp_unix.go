//go:build !windows

package wrapper

import (
	"os"
	"syscall"
)

// handleTSTP handles SIGTSTP (Ctrl-Z) by stopping the child process and then
// self-suspending the wrapper using SIGSTOP (uncatchable).
// Execution resumes here when SIGCONT is delivered (e.g. user does `fg`).
//
// Note: we send SIGSTOP (not SIGTSTP) to the child. SIGTSTP is a tty-based
// job-control signal and is silently ignored by processes that are not in the
// foreground process group of a controlling terminal (e.g. a process spawned
// inside a pty session from a parent without job-control). SIGSTOP is
// unconditional and always stops the target process.
func handleTSTP(state *signalState) {
	if state == nil || state.proc == nil {
		return
	}
	// Stop the child first so it suspends before the wrapper does.
	_ = state.proc.Signal(syscall.SIGSTOP)
	// Self-suspend the wrapper. SIGSTOP is uncatchable; the kernel stops us here.
	// The Go runtime resumes us automatically when SIGCONT is delivered.
	_ = syscall.Kill(os.Getpid(), syscall.SIGSTOP)
	// Execution resumes here after SIGCONT.
}

// handleCONT handles SIGCONT by forwarding it to the child process.
// By the time this handler runs, the kernel has already resumed the wrapper
// (SIGCONT resumes us before we can observe it), so we just forward to child.
func handleCONT(state *signalState) {
	if state == nil || state.proc == nil {
		return
	}
	// Forward SIGCONT to child so it resumes too.
	_ = state.proc.Signal(syscall.SIGCONT)
}

// syscallSIGTSTP returns the SIGTSTP signal for use in the handleSignal switch.
func syscallSIGTSTP() os.Signal { return syscall.SIGTSTP }

// syscallSIGCONT returns the SIGCONT signal for use in the handleSignal switch.
func syscallSIGCONT() os.Signal { return syscall.SIGCONT }
