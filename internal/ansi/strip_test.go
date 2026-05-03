package ansi

import (
	"bytes"
	"testing"
)

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "Plain text",
			input:    []byte("hello"),
			expected: []byte("hello"),
		},
		{
			name:     "Simple CSI color",
			input:    []byte("\x1b[31mred\x1b[0m"),
			expected: []byte("red"),
		},
		{
			name:     "CSI multi-arg",
			input:    []byte("\x1b[1;33;44mfoo"),
			expected: []byte("foo"),
		},
		{
			name:     "CSI cursor move",
			input:    []byte("a\x1b[2Hb"),
			expected: []byte("ab"),
		},
		{
			name:     "CSI alt screen on",
			input:    []byte("\x1b[?1049h"),
			expected: []byte(""),
		},
		{
			name:     "CSI alt screen off",
			input:    []byte("\x1b[?1049l"),
			expected: []byte(""),
		},
		{
			name:     "OSC with BEL",
			input:    []byte("\x1b]0;title\x07"),
			expected: []byte(""),
		},
		{
			name:     "OSC with ST",
			input:    []byte("\x1b]2;window\x1b\\"),
			expected: []byte(""),
		},
		{
			name:     "Single-char escape (RIS reset)",
			input:    []byte("\x1bc"),
			expected: []byte(""),
		},
		{
			name:     "Charset select G0",
			input:    []byte("\x1b(B"),
			expected: []byte(""),
		},
		{
			name:     "Charset select G1",
			input:    []byte("\x1b)0"),
			expected: []byte(""),
		},
		{
			name:     "Multi-byte single-shift SS2",
			input:    []byte("\x1bN!"),
			expected: []byte(""),
		},
		{
			name:     "Multi-byte single-shift SS3",
			input:    []byte("\x1bO!"),
			expected: []byte(""),
		},
		{
			name:     "Multiple in one string",
			input:    []byte("\x1b[31mred\x1b[0m and \x1b[32mgreen\x1b[0m"),
			expected: []byte("red and green"),
		},
		{
			name:     "Truncated CSI at end",
			input:    []byte("foo\x1b[3"),
			expected: []byte("foo"),
		},
		{
			name:     "Truncated OSC at end (no terminator)",
			input:    []byte("foo\x1b]0;bar"),
			expected: []byte("foo"),
		},
		{
			name:     "UTF-8 preserved",
			input:    []byte("héllo"),
			expected: []byte("héllo"),
		},
		{
			name:     "Empty input",
			input:    []byte(""),
			expected: []byte(""),
		},
		{
			name:     "Bare ESC at end",
			input:    []byte("foo\x1b"),
			expected: []byte("foo"),
		},
		{
			name:     "Bell character (not escape)",
			input:    []byte("a\x07b"),
			expected: []byte("a\x07b"),
		},
		{
			name:     "Newline preserved",
			input:    []byte("a\nb"),
			expected: []byte("a\nb"),
		},
		{
			name:     "Carriage return preserved",
			input:    []byte("a\rb"),
			expected: []byte("a\rb"),
		},
		{
			name:     "CSI with intermediate bytes",
			input:    []byte("\x1b[? !p"),
			expected: []byte(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Strip(tt.input)
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("Strip(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
