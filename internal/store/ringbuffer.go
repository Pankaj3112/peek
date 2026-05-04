package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Pankaj3112/peek/internal/platform"
)

const ringBufferLimit = 3 // per-cwd cap (after Create, total = ringBufferLimit)

// EvictOldest scans sessions for the given cwd and removes the oldest
// (by ULID ascending) until at most (ringBufferLimit - 1) remain. The caller
// is about to create a new session that will bring the count to ringBufferLimit.
//
// Idempotent and ENOENT-tolerant. Errors are best-effort: RemoveAll failures
// other than ENOENT are logged to stderr but don't block.
func EvictOldest(cwd string) error {
	root, err := platform.SessionsRoot()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	type entry struct {
		id  string
		dir string
	}
	var matching []entry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m, _ := ReadMeta(filepath.Join(root, e.Name(), "meta.json"))
		if m == nil {
			continue // ENOENT or parse error: skip
		}
		if m.Cwd != cwd {
			continue
		}
		matching = append(matching, entry{id: m.ID, dir: filepath.Join(root, e.Name())})
	}

	if len(matching) < ringBufferLimit {
		return nil
	}

	// Sort by ID ascending (oldest first).
	sort.Slice(matching, func(i, j int) bool {
		return matching[i].id < matching[j].id
	})

	// Evict the oldest until count == ringBufferLimit - 1.
	toEvict := len(matching) - (ringBufferLimit - 1)
	for i := 0; i < toEvict; i++ {
		if err := os.RemoveAll(matching[i].dir); err != nil {
			if !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "peek: ring buffer eviction failed for %s: %v\n", matching[i].dir, err)
			}
		}
	}
	return nil
}
