package analyzer

import "testing"

func TestBuildKnowledgeMapReturnsCoupledFilesAndAuthors(t *testing.T) {
	commits := []Commit{
		{Hash: "c5", Files: []string{"src/billing/service.go", "tests/billing_test.go"}, Authors: []string{"Alice"}},
		{Hash: "c4", Files: []string{"src/billing/service.go", "docs/billing_architecture.md"}, Authors: []string{"Charlie"}},
		{Hash: "c3", Files: []string{"src/billing/service.go", "tests/billing_test.go", "schema/migrations/billing.sql"}, Authors: []string{"Alice", "Bob"}},
		{Hash: "c2", Files: []string{"src/billing/service.go"}, Authors: []string{"Alice"}},
		{Hash: "c1", Files: []string{"other/file.go"}, Authors: []string{"Dora"}},
	}

	result, ok := BuildKnowledgeMap(commits, "src/billing/service.go", KnowledgeMapConfig{
		CouplingThreshold: 0.20,
		MaxFilesPerCommit: 30,
		MaxCoupledFiles:   10,
		MaxAuthors:        5,
	})
	if !ok {
		t.Fatalf("expected knowledge map data, got none")
	}

	if len(result.Coupled) != 3 {
		t.Fatalf("expected 3 coupled files, got %#v", result.Coupled)
	}
	if result.Coupled[0].Path != "tests/billing_test.go" {
		t.Fatalf("expected tests file first, got %#v", result.Coupled)
	}
	if len(result.Authors) != 3 {
		t.Fatalf("expected 3 authors, got %#v", result.Authors)
	}
	if result.Authors[0].Author != "Alice" || result.Authors[0].Commits != 3 {
		t.Fatalf("unexpected top author list: %#v", result.Authors)
	}
}

func TestBuildKnowledgeMapReturnsFalseWithoutHistoricalData(t *testing.T) {
	commits := []Commit{
		{Hash: "c2", Files: []string{"a.go"}, Authors: []string{"Alice"}},
		{Hash: "c1", Files: []string{"b.go"}, Authors: []string{"Bob"}},
	}

	_, ok := BuildKnowledgeMap(commits, "missing.go", KnowledgeMapConfig{
		CouplingThreshold: 0.20,
		MaxFilesPerCommit: 30,
		MaxCoupledFiles:   10,
		MaxAuthors:        5,
	})
	if ok {
		t.Fatalf("expected no map for unknown file")
	}
}

func TestBuildKnowledgeMapReturnsFalseWhenNoCouplingsPassThreshold(t *testing.T) {
	commits := []Commit{
		{Hash: "c5", Files: []string{"target.go", "a.go"}, Authors: []string{"Alice"}},
		{Hash: "c4", Files: []string{"target.go"}, Authors: []string{"Alice"}},
		{Hash: "c3", Files: []string{"target.go"}, Authors: []string{"Alice"}},
		{Hash: "c2", Files: []string{"target.go"}, Authors: []string{"Bob"}},
		{Hash: "c1", Files: []string{"target.go"}, Authors: []string{"Charlie"}},
	}

	_, ok := BuildKnowledgeMap(commits, "target.go", KnowledgeMapConfig{
		CouplingThreshold: 0.20,
		MaxFilesPerCommit: 30,
		MaxCoupledFiles:   10,
		MaxAuthors:        5,
	})
	if ok {
		t.Fatalf("expected no map when couplings do not pass threshold")
	}
}
