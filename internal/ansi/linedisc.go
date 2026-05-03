package ansi

// LineDiscipline implements line discipline for a byte stream.
// It reads raw bytes, strips ANSI escape sequences, and emits finalized lines
// (terminated by \n or \r\n) to a callback function.
//
// Line discipline semantics:
//   - \r\n is treated as a single line terminator (CRLF).
//   - Bare \r (not followed by \n) is treated as a redraw: it clears the pending
//     line, and subsequent bytes overwrite it.
//   - Only lines terminated by \n or \r\n are emitted.
//   - Partial lines (no trailing newline) are buffered and emitted on Close().
//
// The discipline handles cross-batch state: if a Write() ends with \r, the
// discipline holds that \r in a "saw_cr" flag. On the next Write():
// - If the first byte is \n, the \r\n is a line terminator.
// - Otherwise, the \r was a bare CR redraw, and the pending line is discarded.
type LineDiscipline struct {
	emit    func(line []byte)
	pending []byte
	sawCR   bool
}

// NewLineDiscipline creates a new line discipline with the given emit callback.
// The callback is called with finalized lines (no trailing newline or CR).
func NewLineDiscipline(emit func(line []byte)) *LineDiscipline {
	return &LineDiscipline{
		emit:    emit,
		pending: make([]byte, 0, 256), // Pre-allocate reasonable buffer
	}
}

// Write processes a chunk of bytes through the line discipline.
// It strips ANSI sequences, handles line terminators (\n and \r\n),
// and treats bare \r as a redraw (clearing the pending line).
// Returns the length of the input (as per io.Writer contract), or an error.
func (ld *LineDiscipline) Write(b []byte) (int, error) {
	originalLen := len(b)

	// Strip ANSI escape sequences from input
	b = Strip(b)

	for _, c := range b {
		if ld.sawCR {
			ld.sawCR = false
			if c == '\n' {
				// CRLF terminator: emit pending line and clear
				ld.emit(ld.pending)
				ld.pending = ld.pending[:0]
				continue
			}
			// else: bare CR was a redraw. Clear pending and fall through
			// to treat the current byte as a normal byte (start of new content).
			ld.pending = ld.pending[:0]
		}

		switch c {
		case '\r':
			// Don't know yet if CRLF or bare CR. Hold the \r state.
			ld.sawCR = true

		case '\n':
			// Line terminator (LF without preceding CR).
			ld.emit(ld.pending)
			ld.pending = ld.pending[:0]

		default:
			// Normal content byte: append to pending
			ld.pending = append(ld.pending, c)
		}
	}

	// Return the original input length per io.Writer contract
	return originalLen, nil
}

// Close flushes any pending partial line and resets state.
// It emits non-empty pending lines as a final output, even if they don't
// end with \n. This ensures that log messages without trailing newlines
// are not lost.
func (ld *LineDiscipline) Close() error {
	// If we ended on a bare \r, that was a redraw, so clear pending.
	// (sawCR=true means pending was not yet cleared by the \r logic.)
	if ld.sawCR {
		ld.pending = ld.pending[:0]
		ld.sawCR = false
	}

	// Emit any remaining partial line
	if len(ld.pending) > 0 {
		ld.emit(ld.pending)
		ld.pending = ld.pending[:0]
	}

	return nil
}
