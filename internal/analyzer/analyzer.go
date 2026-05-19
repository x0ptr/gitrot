package analyzer

import "sort"

type Commit struct {
	Hash  string
	Files []string
}

type Config struct {
	CouplingThreshold float64
	MinSharedCommits  int
	MinDrift          int
	MaxFilesPerCommit int
}

type Finding struct {
	Source       string
	CoupledTo    string
	Coupling     float64
	Shared       int
	Drift        int
	LastSyncHash string
}

type LeftBehind struct {
	Path     string
	Coupling float64
	Shared   int
}

type GroupedFinding struct {
	Source       string
	Drift        int
	LastSyncHash string
	LeftBehind   []LeftBehind
}

func Analyze(commits []Commit, cfg Config) []Finding {
	fileCommitCount := map[string]int{}
	fileCommitIndices := map[string][]int{}
	pairCounts := map[string]map[string]int{}
	lastSyncIndex := map[string]map[string]int{}

	for i, c := range commits {
		if len(c.Files) == 0 {
			continue
		}
		if cfg.MaxFilesPerCommit > 0 && len(c.Files) > cfg.MaxFilesPerCommit {
			continue
		}

		sort.Strings(c.Files)
		for _, a := range c.Files {
			fileCommitCount[a]++
			fileCommitIndices[a] = append(fileCommitIndices[a], i)
		}

		for _, a := range c.Files {
			if pairCounts[a] == nil {
				pairCounts[a] = map[string]int{}
			}
			if lastSyncIndex[a] == nil {
				lastSyncIndex[a] = map[string]int{}
			}
			for _, b := range c.Files {
				if a == b {
					continue
				}
				pairCounts[a][b]++
				if _, ok := lastSyncIndex[a][b]; !ok {
					lastSyncIndex[a][b] = i
				}
			}
		}
	}

	findings := make([]Finding, 0)
	for a, targets := range pairCounts {
		totalA := fileCommitCount[a]
		if totalA == 0 {
			continue
		}

		for b, shared := range targets {
			if shared < cfg.MinSharedCommits {
				continue
			}

			coupling := float64(shared) / float64(totalA)
			if coupling < cfg.CouplingThreshold {
				continue
			}

			syncIdx, ok := lastSyncIndex[a][b]
			if !ok || syncIdx <= 0 || syncIdx >= len(commits) {
				continue
			}

			drift := countLessThan(fileCommitIndices[a], syncIdx) - countLessThan(fileCommitIndices[b], syncIdx)
			if drift < cfg.MinDrift {
				continue
			}

			findings = append(findings, Finding{
				Source:       a,
				CoupledTo:    b,
				Coupling:     coupling,
				Shared:       shared,
				Drift:        drift,
				LastSyncHash: commits[syncIdx].Hash,
			})
		}
	}

	return findings
}

func GroupBySource(findings []Finding) []GroupedFinding {
	groups := make(map[string]*GroupedFinding, len(findings))
	representativeCoupling := make(map[string]float64, len(findings))

	for _, f := range findings {
		group, ok := groups[f.Source]
		if !ok {
			group = &GroupedFinding{
				Source:       f.Source,
				Drift:        f.Drift,
				LastSyncHash: f.LastSyncHash,
			}
			groups[f.Source] = group
			representativeCoupling[f.Source] = f.Coupling
		}

		group.LeftBehind = append(group.LeftBehind, LeftBehind{
			Path:     f.CoupledTo,
			Coupling: f.Coupling,
			Shared:   f.Shared,
		})

		if f.Drift > group.Drift || (f.Drift == group.Drift && f.Coupling > representativeCoupling[f.Source]) {
			group.Drift = f.Drift
			group.LastSyncHash = f.LastSyncHash
			representativeCoupling[f.Source] = f.Coupling
		}
	}

	result := make([]GroupedFinding, 0, len(groups))
	for _, group := range groups {
		sort.Slice(group.LeftBehind, func(i, j int) bool {
			if group.LeftBehind[i].Coupling == group.LeftBehind[j].Coupling {
				return group.LeftBehind[i].Path < group.LeftBehind[j].Path
			}
			return group.LeftBehind[i].Coupling > group.LeftBehind[j].Coupling
		})
		result = append(result, *group)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Drift == result[j].Drift {
			if len(result[i].LeftBehind) == len(result[j].LeftBehind) {
				return result[i].Source < result[j].Source
			}
			return len(result[i].LeftBehind) > len(result[j].LeftBehind)
		}
		return result[i].Drift > result[j].Drift
	})

	return result
}

func countLessThan(sorted []int, v int) int {
	return sort.Search(len(sorted), func(i int) bool { return sorted[i] >= v })
}
