package store

import (
	"time"

	"github.com/oklog/ulid/v2"
)

// entropy is a thread-safe, process-level entropy source for ULID generation.
// DefaultEntropy() provides monotonically increasing entropy that is safe for
// concurrent use across all goroutines.
var entropy = ulid.DefaultEntropy()

// NewID generates a new ULID-based session ID and returns it as a 26-character string.
// The returned ID is guaranteed to be:
// - exactly 26 characters long
// - lexicographically sortable (time-ordered)
// - monotonically increasing within the same millisecond
// - unique across calls
func NewID() string {
	id := ulid.MustNew(ulid.Timestamp(time.Now().UTC()), entropy)
	return id.String()
}
