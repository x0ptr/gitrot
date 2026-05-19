package analyzer

import "testing"

func TestAnalyzeCalculatesFullCoupling(t *testing.T) {
	commits := []Commit{
		{Hash: "c3", Files: []string{"C"}},
		{Hash: "c2", Files: []string{"A", "B"}},
		{Hash: "c1", Files: []string{"A", "B"}},
	}

	findings := Analyze(commits, Config{
		CouplingThreshold: 1.0,
		MinSharedCommits:  2,
		MinDrift:          0,
		MaxFilesPerCommit: 30,
	})

	finding, ok := findFinding(findings, "A", "B")
	if !ok {
		t.Fatalf("expected finding for A->B, got %#v", findings)
	}
	if finding.Coupling != 1.0 {
		t.Fatalf("expected coupling=1.0, got %f", finding.Coupling)
	}
}

func TestAnalyzeCalculatesDriftForSourceFile(t *testing.T) {
	commits := []Commit{
		{Hash: "c3", Files: []string{"A"}},
		{Hash: "c2", Files: []string{"A", "B"}},
		{Hash: "c1", Files: []string{"A", "B"}},
	}

	findings := Analyze(commits, Config{
		CouplingThreshold: 0.6,
		MinSharedCommits:  2,
		MinDrift:          1,
		MaxFilesPerCommit: 30,
	})

	finding, ok := findFinding(findings, "A", "B")
	if !ok {
		t.Fatalf("expected finding for A->B, got %#v", findings)
	}
	if finding.Drift != 1 {
		t.Fatalf("expected drift=1, got %d", finding.Drift)
	}
}

func findFinding(findings []Finding, source, coupledTo string) (Finding, bool) {
	for _, f := range findings {
		if f.Source == source && f.CoupledTo == coupledTo {
			return f, true
		}
	}
	return Finding{}, false
}
