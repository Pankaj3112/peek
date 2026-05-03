package mcp

import (
	"path/filepath"
	"strings"
)

// Match returns true if session is an exact match, an ancestor of query, or a
// descendant of query. An empty query matches everything (no filter applied).
//
// Rules (per spec):
//   - filepath.Clean both sides before comparison
//   - separator-aware prefix check (so /home/foo does not match /home/foobar)
//   - case-sensitive (no case-folding on any platform)
//   - no symlink resolution
func Match(query, session string) bool {
	if query == "" {
		return true
	}
	q := filepath.Clean(query)
	s := filepath.Clean(session)
	if q == s {
		return true
	}
	sep := string(filepath.Separator)
	if q == sep {
		// root query matches every absolute session path
		return strings.HasPrefix(s, sep)
	}
	// s is an ancestor of q: q starts with s + sep
	if strings.HasPrefix(q, s+sep) {
		return true
	}
	// q is an ancestor of s: s starts with q + sep
	if strings.HasPrefix(s, q+sep) {
		return true
	}
	return false
}
