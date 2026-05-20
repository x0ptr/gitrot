package git

import "testing"

func TestParseCommitsCollectsPrimaryAndTrailerAuthors(t *testing.T) {
	raw := []byte(
		marker + "abc123" + fieldSep + "def456" + fieldSep + "Alice Doe" + fieldSep + "Bob Builder <bob@example.com>" + trailerSep + "Charlie <charlie@example.com>" + fieldSep + "Dora <dora@example.com>\n" +
			"src/a.go\n" +
			"src/b.go\n",
	)

	commits, err := parseCommits(raw, 10)
	if err != nil {
		t.Fatalf("parse commits: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	if len(commits[0].Authors) != 4 {
		t.Fatalf("expected 4 authors, got %#v", commits[0].Authors)
	}
	expected := []string{"Alice Doe", "Bob Builder", "Charlie", "Dora"}
	for i := range expected {
		if commits[0].Authors[i] != expected[i] {
			t.Fatalf("unexpected author at %d: got %q want %q", i, commits[0].Authors[i], expected[i])
		}
	}
}
