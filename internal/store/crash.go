package store

import "github.com/Pankaj3112/peek/internal/platform"

// MetaView wraps Meta with read-time crash detection results.
// WrapperDied is true if meta.json said "running" but the wrapper PID is actually dead.
type MetaView struct {
	Meta        *Meta
	WrapperDied bool
}

// ApplyCrashDetection takes a Meta as read from disk and applies PID liveness
// detection. If status was running but the wrapper PID is gone, it returns a
// MetaView with virtually-exited status (status=exited, exit_code=nil,
// exited_at=nil) and WrapperDied=true.
//
// The original Meta is NOT mutated; the returned MetaView contains a copy if
// any virtual transformation happened, or the original pointer if not.
func ApplyCrashDetection(m *Meta) MetaView {
	if m.Status != StatusRunning {
		return MetaView{Meta: m, WrapperDied: false}
	}
	if platform.IsAlive(m.Pid) {
		return MetaView{Meta: m, WrapperDied: false}
	}
	// Wrapper PID is dead. Build a virtual-exited copy.
	cp := *m
	cp.Status = StatusExited
	cp.ExitCode = nil
	cp.ExitedAt = nil
	return MetaView{Meta: &cp, WrapperDied: true}
}
