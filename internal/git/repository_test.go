package git

import "testing"

func TestParseCommitsCollectsPrimaryAndTrailerAuthors(t *testing.T) {
	raw := []byte(
		marker + "abc123" + fieldSep + "def456" + fieldSep + "Alice Doe" + fieldSep + "Bob Builder <bob@example.com>" + trailerSep + "Charlie <charlie@example.com>" + fieldSep + "Dora <dora@example.com>\n" +
			"src/a.go\n" +
			"src/b.go\n",
	)

	commits, err := parseCommits(raw, 10, false)
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

func TestParseCommitsSkipsDotfilesWhenEnabled(t *testing.T) {
	raw := []byte(
		marker + "abc123" + fieldSep + "def456" + fieldSep + "Alice Doe" + fieldSep + "" + fieldSep + "\n" +
			".gitignore\n" +
			"src/a.go\n" +
			".github/workflows/ci.yml\n",
	)

	commits, err := parseCommits(raw, 10, true)
	if err != nil {
		t.Fatalf("parse commits: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	if len(commits[0].Files) != 1 || commits[0].Files[0] != "src/a.go" {
		t.Fatalf("expected only non-dotfile to remain, got %#v", commits[0].Files)
	}
}

func TestIsDotfile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: ".gitignore", want: true},
		{path: ".github/workflows/build.yml", want: true},
		{path: "src/.config/app.yml", want: true},
		{path: "src/main.go", want: false},
		{path: "pkg\\core\\.meta\\x.txt", want: true},
	}
	for _, tc := range tests {
		if got := isDotfile(tc.path); got != tc.want {
			t.Fatalf("isDotfile(%q)=%v want %v", tc.path, got, tc.want)
		}
	}
}
