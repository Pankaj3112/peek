package mcp

import "testing"

func TestMatch(t *testing.T) {
	cases := []struct {
		name, query, session string
		want                 bool
	}{
		{"exact", "/home/me/proj1", "/home/me/proj1", true},
		{"session_ancestor_of_query", "/home/me/proj1/sub/components", "/home/me/proj1", true},
		{"session_ancestor_of_query_intermediate", "/home/me/proj1/sub/components", "/home/me/proj1/sub", true},
		{"query_ancestor_of_session_descendant", "/home/me/proj1", "/home/me/proj1/sub", true},
		{"sibling_no_match", "/home/me/proj1", "/home/me/proj2", false},
		{"prefix_not_separator_aware", "/home/me/proj1", "/home/me/proj1other", false},
		{"root_query_matches_all", "/", "/anything", true},
		{"deep_query_root_session", "/home/me", "/home/me/proj1", true},
		{"deep_query_root_session_intermediate", "/home/me", "/home/me/proj1/sub", true},
		{"empty_query_no_filter", "", "/anything", true},
		{"case_sensitive_no_match", "/home/me/proj1", "/HOME/me/proj1", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Match(c.query, c.session)
			if got != c.want {
				t.Errorf("Match(%q, %q) = %v, want %v", c.query, c.session, got, c.want)
			}
		})
	}
}
