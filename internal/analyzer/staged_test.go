package analyzer

import "testing"

func TestEvaluateStagedCohesionIgnoresNewFiles(t *testing.T) {
	commits := []Commit{
		{Hash: "c2", Files: []string{"a.go", "b.go"}},
		{Hash: "c1", Files: []string{"a.go", "b.go"}},
	}
	result := EvaluateStagedCohesion(commits, []string{"a.go", "new.go"}, StagedCohesionConfig{
		CouplingThreshold: 0.6,
		MaxFilesPerCommit: 30,
	})

	if len(result.FilesWithHistory) != 1 || result.FilesWithHistory[0] != "a.go" {
		t.Fatalf("expected only file with history to remain, got %#v", result.FilesWithHistory)
	}
	if result.TotalPairs != 0 || result.CoupledPairs != 0 || result.Cohesion != 0 {
		t.Fatalf("expected no pair math for a single historical file, got %#v", result)
	}
}

func TestEvaluateStagedCohesionComputesPercentage(t *testing.T) {
	commits := []Commit{
		{Hash: "c3", Files: []string{"a.go", "b.go"}},
		{Hash: "c2", Files: []string{"a.go", "c.go"}},
		{Hash: "c1", Files: []string{"a.go", "b.go"}},
	}
	result := EvaluateStagedCohesion(commits, []string{"a.go", "b.go", "c.go"}, StagedCohesionConfig{
		CouplingThreshold: 0.6,
		MaxFilesPerCommit: 30,
	})

	if result.TotalPairs != 3 {
		t.Fatalf("expected 3 pairs, got %d", result.TotalPairs)
	}
	if result.CoupledPairs != 2 {
		t.Fatalf("expected 2 coupled pairs, got %d", result.CoupledPairs)
	}
	if result.Cohesion != 66 {
		t.Fatalf("expected 66 cohesion, got %d", result.Cohesion)
	}
	if len(result.DissonantFiles) != 2 || result.DissonantFiles[0] != "b.go" || result.DissonantFiles[1] != "c.go" {
		t.Fatalf("unexpected dissonant files: %#v", result.DissonantFiles)
	}
}
