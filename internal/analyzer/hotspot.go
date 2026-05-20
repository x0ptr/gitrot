package analyzer

import (
	"sort"
	"strings"
)

type HotspotConfig struct {
	CouplingThreshold float64
	MaxFilesPerCommit int
	MaxResults        int
	MinCommits        int
	TargetPath        string
}

type Hotspot struct {
	Path           string
	CouplingDegree int
	Churn          int
	Score          int
}

func IdentifyHotspots(commits []Commit, cfg HotspotConfig) []Hotspot {
	fileCommitCount := buildFileCommitCount(commits, cfg.MaxFilesPerCommit)
	pairCounts := buildPairCounts(commits, cfg.MaxFilesPerCommit)

	hotspots := make([]Hotspot, 0, len(fileCommitCount))
	for file, churn := range fileCommitCount {
		if churn == 0 {
			continue
		}
		if cfg.TargetPath != "" && !strings.HasPrefix(file, cfg.TargetPath) {
			continue
		}
		if cfg.MinCommits > 0 && churn < cfg.MinCommits {
			continue
		}
		degree := 0
		for _, shared := range pairCounts[file] {
			coupling := float64(shared) / float64(churn)
			if coupling >= cfg.CouplingThreshold {
				degree++
			}
		}
		if degree == 0 {
			continue
		}
		hotspots = append(hotspots, Hotspot{
			Path:           file,
			CouplingDegree: degree,
			Churn:          churn,
			Score:          churn * degree,
		})
	}

	sort.Slice(hotspots, func(i, j int) bool {
		if hotspots[i].Score == hotspots[j].Score {
			if hotspots[i].CouplingDegree == hotspots[j].CouplingDegree {
				if hotspots[i].Churn == hotspots[j].Churn {
					return hotspots[i].Path < hotspots[j].Path
				}
				return hotspots[i].Churn > hotspots[j].Churn
			}
			return hotspots[i].CouplingDegree > hotspots[j].CouplingDegree
		}
		return hotspots[i].Score > hotspots[j].Score
	})

	if cfg.MaxResults > 0 && len(hotspots) > cfg.MaxResults {
		hotspots = hotspots[:cfg.MaxResults]
	}
	return hotspots
}
