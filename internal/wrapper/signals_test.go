//go:build !windows

package wrapper

import (
	"testing"
)

// mockResizer is a test double for ptyResizer that records Resize calls.
type mockResizer struct {
	calls []resizeCall
}

type resizeCall struct {
	width  int
	height int
}

func (m *mockResizer) Resize(width, height int) error {
	m.calls = append(m.calls, resizeCall{width: width, height: height})
	return nil
}

func TestSIGWINCHHandler(t *testing.T) {
	t.Run("resize_called_with_getSize_values", func(t *testing.T) {
		mock := &mockResizer{}

		// getSize returns rows=50, cols=100
		getSize := func() (rows, cols uint16) { return 50, 100 }

		handleSIGWINCH(mock, getSize)

		if len(mock.calls) != 1 {
			t.Fatalf("expected 1 Resize call, got %d", len(mock.calls))
		}
		// go-pty Resize(width, height): width=cols=100, height=rows=50
		if mock.calls[0].width != 100 || mock.calls[0].height != 50 {
			t.Errorf("Resize called with width=%d height=%d, want width=100 height=50",
				mock.calls[0].width, mock.calls[0].height)
		}
	})
}
