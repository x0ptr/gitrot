package analyzer

import "testing"

func TestIdentifyHotspotsSortsByScore(t *testing.T) {
	commits := []Commit{
		{Hash: "c12", Files: []string{"a.go", "b.go", "c.go"}},
		{Hash: "c11", Files: []string{"a.go", "b.go", "d.go"}},
		{Hash: "c10", Files: []string{"a.go", "c.go", "d.go"}},
		{Hash: "c9", Files: []string{"a.go", "b.go", "c.go"}},
		{Hash: "c8", Files: []string{"a.go", "d.go"}},
		{Hash: "c7", Files: []string{"b.go", "c.go"}},
		{Hash: "c6", Files: []string{"b.go", "d.go"}},
		{Hash: "c5", Files: []string{"b.go", "c.go"}},
		{Hash: "c4", Files: []string{"c.go", "d.go"}},
		{Hash: "c3", Files: []string{"x.go", "y.go"}},
		{Hash: "c2", Files: []string{"x.go", "y.go"}},
		{Hash: "c1", Files: []string{"x.go", "y.go"}},
	}

	hotspots := IdentifyHotspots(commits, HotspotConfig{
		CouplingThreshold: 0.5,
		MaxFilesPerCommit: 30,
		MaxResults:        10,
		MinCommits:        5,
	})

	if len(hotspots) == 0 {
		t.Fatalf("expected hotspots")
	}
	if hotspots[0].Path != "a.go" {
		t.Fatalf("expected a.go top by score, got %#v", hotspots[0])
	}
	if hotspots[0].Score != hotspots[0].Churn*hotspots[0].CouplingDegree {
		t.Fatalf("expected score to be churn*degree, got %#v", hotspots[0])
	}
}

func TestIdentifyHotspotsFiltersLowChurnFiles(t *testing.T) {
	commits := []Commit{
		{Hash: "c6", Files: []string{"noisy.go", "a.go", "b.go", "c.go", "d.go"}},
		{Hash: "c5", Files: []string{"a.go"}},
		{Hash: "c4", Files: []string{"b.go"}},
		{Hash: "c3", Files: []string{"c.go"}},
		{Hash: "c2", Files: []string{"d.go"}},
		{Hash: "c1", Files: []string{"a.go", "b.go", "c.go", "d.go"}},
	}

	hotspots := IdentifyHotspots(commits, HotspotConfig{
		CouplingThreshold: 0.5,
		MaxFilesPerCommit: 30,
		MaxResults:        10,
		MinCommits:        5,
	})
	if len(hotspots) != 0 {
		t.Fatalf("expected low-churn files to be filtered out, got %#v", hotspots)
	}
}

func TestIdentifyHotspotsReturnsEmptyWhenNoFilePassesCoupling(t *testing.T) {
	commits := []Commit{
		{Hash: "c7", Files: []string{"a.go"}},
		{Hash: "c6", Files: []string{"a.go"}},
		{Hash: "c5", Files: []string{"a.go"}},
		{Hash: "c4", Files: []string{"a.go"}},
		{Hash: "c3", Files: []string{"a.go"}},
		{Hash: "c2", Files: []string{"b.go"}},
		{Hash: "c1", Files: []string{"b.go"}},
	}

	hotspots := IdentifyHotspots(commits, HotspotConfig{
		CouplingThreshold: 0.6,
		MaxFilesPerCommit: 30,
		MaxResults:        10,
		MinCommits:        5,
	})
	if len(hotspots) != 0 {
		t.Fatalf("expected no hotspots, got %#v", hotspots)
	}
}

func TestIdentifyHotspotsFiltersByTargetPathPrefix(t *testing.T) {
	commits := []Commit{
		{Hash: "c8", Files: []string{"src/api/router.go", "src/core/service.go"}},
		{Hash: "c7", Files: []string{"src/api/router.go", "src/core/service.go"}},
		{Hash: "c6", Files: []string{"src/api/router.go", "src/core/service.go"}},
		{Hash: "c5", Files: []string{"src/api/router.go", "src/core/service.go"}},
		{Hash: "c4", Files: []string{"src/api/router.go", "src/core/service.go"}},
		{Hash: "c3", Files: []string{"src/core/service.go", "pkg/shared/util.go"}},
		{Hash: "c2", Files: []string{"src/core/service.go", "pkg/shared/util.go"}},
		{Hash: "c1", Files: []string{"src/core/service.go", "pkg/shared/util.go"}},
	}

	hotspots := IdentifyHotspots(commits, HotspotConfig{
		CouplingThreshold: 0.5,
		MaxFilesPerCommit: 30,
		MaxResults:        10,
		MinCommits:        5,
		TargetPath:        "src/api",
	})
	if len(hotspots) != 1 {
		t.Fatalf("expected only api hotspot, got %#v", hotspots)
	}
	if hotspots[0].Path != "src/api/router.go" {
		t.Fatalf("unexpected hotspot path: %#v", hotspots[0])
	}
}
