package ansi

import (
	"reflect"
	"testing"
)

func TestLineDiscipline(t *testing.T) {
	tests := []struct {
		name            string
		writes          []string // sequence of Write calls
		callClose       bool     // whether to call Close
		expectedEmitted [][]byte // expected emitted lines
	}{
		{
			name:            "Two simple lines",
			writes:          []string{"hello\nworld\n"},
			callClose:       true,
			expectedEmitted: [][]byte{[]byte("hello"), []byte("world")},
		},
		{
			name:            "Plain CRLF",
			writes:          []string{"line1\r\nline2\n"},
			callClose:       true,
			expectedEmitted: [][]byte{[]byte("line1"), []byte("line2")},
		},
		{
			name:            "Bare CR redraw",
			writes:          []string{"loading...\rloaded\n"},
			callClose:       true,
			expectedEmitted: [][]byte{[]byte("loaded")},
		},
		{
			name:            "Multiple CR redraws",
			writes:          []string{"\r| 50%\r| 100%\rdone\n"},
			callClose:       true,
			expectedEmitted: [][]byte{[]byte("done")},
		},
		{
			name:            "Partial line at end (no flush)",
			writes:          []string{"hello\nworld"},
			callClose:       false,
			expectedEmitted: [][]byte{[]byte("hello")},
		},
		{
			name:            "Partial line + Close",
			writes:          []string{"hello\nworld"},
			callClose:       true,
			expectedEmitted: [][]byte{[]byte("hello"), []byte("world")},
		},
		{
			name:            "Bytewise streaming",
			writes:          []string{"h", "e", "l", "l", "o", "\n", "w", "o", "r", "l", "d", "\n"},
			callClose:       true,
			expectedEmitted: [][]byte{[]byte("hello"), []byte("world")},
		},
		{
			name:            "ANSI + line",
			writes:          []string{"\x1b[31merror\x1b[0m\n"},
			callClose:       true,
			expectedEmitted: [][]byte{[]byte("error")},
		},
		{
			name:            "Empty Write",
			writes:          []string{""},
			callClose:       true,
			expectedEmitted: nil,
		},
		{
			name:            "Just newline",
			writes:          []string{"\n"},
			callClose:       true,
			expectedEmitted: [][]byte{[]byte("")},
		},
		{
			name:            "Cross-batch CR + LF",
			writes:          []string{"foo\r", "\nbar\n"},
			callClose:       true,
			expectedEmitted: [][]byte{[]byte("foo"), []byte("bar")},
		},
		{
			name:            "Cross-batch CR + non-LF",
			writes:          []string{"foo\r", "baz\n"},
			callClose:       true,
			expectedEmitted: [][]byte{[]byte("baz")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got [][]byte
			ld := NewLineDiscipline(func(line []byte) {
				// Copy the line because the emit callback may reuse the slice
				cp := make([]byte, len(line))
				copy(cp, line)
				got = append(got, cp)
			})

			// Execute all writes
			for _, w := range tt.writes {
				n, err := ld.Write([]byte(w))
				if err != nil {
					t.Errorf("Write(%q) error: %v", w, err)
				}
				if n != len(w) {
					t.Errorf("Write(%q) returned %d, want %d", w, n, len(w))
				}
			}

			// Optionally close to flush any pending partial line
			if tt.callClose {
				if err := ld.Close(); err != nil {
					t.Errorf("Close() error: %v", err)
				}
			}

			// Compare emitted lines with expected
			if !reflect.DeepEqual(got, tt.expectedEmitted) {
				t.Errorf("emitted lines mismatch\ngot:  %v\nwant: %v", got, tt.expectedEmitted)
			}
		})
	}
}
