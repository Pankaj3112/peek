package store

import (
	"os"
	"testing"
	"time"
)

func TestApplyCrashDetection(t *testing.T) {
	t.Run("running_with_live_pid_returns_unchanged", func(t *testing.T) {
		m := &Meta{
			Version:   1,
			ID:        "01HQXAMPLE",
			Pid:       os.Getpid(), // current test PID = alive
			Status:    StatusRunning,
			StartedAt: time.Now().UTC(),
		}
		view := ApplyCrashDetection(m)
		if view.Meta.Status != StatusRunning {
			t.Errorf("expected running, got %s", view.Meta.Status)
		}
		if view.WrapperDied {
			t.Errorf("expected WrapperDied=false")
		}
	})

	t.Run("running_with_dead_pid_returns_virtual_exited", func(t *testing.T) {
		m := &Meta{
			Version:   1,
			ID:        "01HQXAMPLE",
			Pid:       99999999, // very unlikely to be alive
			Status:    StatusRunning,
			StartedAt: time.Now().UTC(),
		}
		view := ApplyCrashDetection(m)
		if view.Meta.Status != StatusExited {
			t.Errorf("expected virtual exited, got %s", view.Meta.Status)
		}
		if !view.WrapperDied {
			t.Errorf("expected WrapperDied=true")
		}
		if view.Meta.ExitCode != nil {
			t.Errorf("expected ExitCode=nil for crashed wrapper, got %v", view.Meta.ExitCode)
		}
		if view.Meta.ExitedAt != nil {
			t.Errorf("expected ExitedAt=nil for crashed wrapper, got %v", view.Meta.ExitedAt)
		}
		// The original Meta on the input should NOT be mutated.
		if m.Status != StatusRunning {
			t.Errorf("input Meta was mutated: status changed to %s", m.Status)
		}
	})

	t.Run("already_exited_returns_unchanged", func(t *testing.T) {
		exited := time.Now().UTC()
		code := 0
		m := &Meta{
			Version:   1,
			ID:        "01HQXAMPLE",
			Pid:       99999999, // dead PID, but status is exited so we don't check
			Status:    StatusExited,
			StartedAt: time.Now().UTC().Add(-time.Hour),
			ExitedAt:  &exited,
			ExitCode:  &code,
		}
		view := ApplyCrashDetection(m)
		if view.Meta.Status != StatusExited {
			t.Errorf("expected exited, got %s", view.Meta.Status)
		}
		if view.WrapperDied {
			t.Errorf("expected WrapperDied=false (clean exit, not wrapper-died)")
		}
		// Existing exit_code/exited_at preserved.
		if view.Meta.ExitCode == nil || *view.Meta.ExitCode != 0 {
			t.Errorf("ExitCode should be 0, got %v", view.Meta.ExitCode)
		}
	})
}
