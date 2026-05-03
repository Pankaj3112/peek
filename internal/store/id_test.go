package store

import (
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
)

func TestNewID(t *testing.T) {
	// Test (a): Length - exactly 26 characters
	id1 := NewID()
	if len(id1) != 26 {
		t.Errorf("NewID() returned %d chars, want 26; got %q", len(id1), id1)
	}

	// Test (b): Parseability - can be parsed by ulid.Parse
	_, err := ulid.Parse(id1)
	if err != nil {
		t.Errorf("NewID() returned unparseable ID %q: %v", id1, err)
	}

	// Test (c): Monotonicity within tight loop
	// Generate 10 IDs in a loop and verify each is >= previous lexicographically
	ids := make([]string, 10)
	for i := 0; i < 10; i++ {
		ids[i] = NewID()
	}

	for i := 1; i < 10; i++ {
		if strings.Compare(ids[i], ids[i-1]) < 0 {
			t.Errorf("Monotonicity violated: ids[%d] (%s) < ids[%d] (%s)", i, ids[i], i-1, ids[i-1])
		}
	}

	// Test (d): Uniqueness - generate 1000 IDs, all should be distinct
	idSet := make(map[string]struct{})
	for i := 0; i < 1000; i++ {
		id := NewID()
		idSet[id] = struct{}{}
	}

	if len(idSet) != 1000 {
		t.Errorf("Generated 1000 IDs but only got %d unique IDs", len(idSet))
	}
}
