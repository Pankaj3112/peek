package ansi

// Strip removes ANSI escape sequences from a byte slice.
// It handles:
// - CSI sequences (Control Sequence Introducer: ESC [ ... final)
// - OSC sequences (Operating System Command: ESC ] ... BEL or ST)
// - Single-shift sequences (SS2: ESC N, SS3: ESC O)
// - Character set select sequences (ESC ( | ) | * | +)
// - Single-character escapes
//
// Returns a new byte slice with escape sequences removed.
// Preserves UTF-8 and non-escape control characters.
func Strip(b []byte) []byte {
	const (
		stateNormal = iota
		stateEscape
		stateCSI
		stateOSC
		stateOSCST
		stateCharset
		stateSS
	)

	out := make([]byte, 0, len(b))
	state := stateNormal
	csiParams := make([]byte, 0, 16) // Buffer to accumulate CSI parameters

	for i := 0; i < len(b); i++ {
		c := b[i]

		switch state {
		case stateNormal:
			if c == 0x1b { // ESC
				state = stateEscape
			} else {
				out = append(out, c)
			}

		case stateEscape:
			switch c {
			case '[':
				state = stateCSI
				csiParams = csiParams[:0] // Reset parameter buffer
			case ']':
				state = stateOSC
			case '(', ')', '*', '+':
				state = stateCharset
			case 'N', 'O':
				state = stateSS
			default:
				// Single-character escape (e.g., RIS, IND, etc.)
				// Drop both the ESC and this byte
				state = stateNormal
			}

		case stateCSI:
			// Parameter bytes: 0x30-0x3F
			// Intermediate bytes: 0x20-0x2F
			// Final byte: 0x40-0x7E
			if c >= 0x30 && c <= 0x3F {
				// Parameter byte, accumulate it
				csiParams = append(csiParams, c)
			} else if c >= 0x20 && c <= 0x2F {
				// Intermediate byte, accumulate it
				csiParams = append(csiParams, c)
			} else if c >= 0x40 && c <= 0x7E {
				// Final byte, sequence complete
				// Check if this is CHA ('G') with parameter 1 or empty
				if c == 'G' && (len(csiParams) == 0 || (len(csiParams) == 1 && csiParams[0] == '1')) {
					// CHA col 1 or default col 1: emit CR
					out = append(out, '\r')
				}
				// For all other CSI sequences, drop them
				state = stateNormal
			} else {
				// Unexpected byte; bail to normal
				state = stateNormal
			}

		case stateOSC:
			if c == 0x07 { // BEL
				state = stateNormal
			} else if c == 0x1b { // ESC, might be start of ST (\x1b\\)
				state = stateOSCST
			}
			// else: stay in OSC, dropping byte

		case stateOSCST:
			if c == '\\' {
				// ST terminator complete
				state = stateNormal
			} else {
				// Not ST; this is a new escape sequence
				state = stateEscape
			}

		case stateCharset:
			// Skip exactly one byte after the introducer
			state = stateNormal

		case stateSS:
			// Skip exactly one byte after the single-shift introducer
			state = stateNormal
		}
	}

	return out
}
