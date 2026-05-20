package analyzer

import (
	"sort"
	"strings"
)

type Commit struct {
	Hash    string
	Files   []string
	Authors []string
}

type Config struct {
	CouplingThreshold float64
	MinSharedCommits  int
	MinDrift          int
	MaxFilesPerCommit int
	IgnoreSilo        bool
}

type Finding struct {
	Source            string
	CoupledTo         string
	Coupling          float64
	Shared            int
	Drift             int
	LastSyncHash      string
	ContextLoss       bool
	HistoricalAuthors []string
	DriftAuthors      []string
}

type LeftBehind struct {
	Path              string
	Coupling          float64
	Shared            int
	ContextLoss       bool
	HistoricalAuthors []string
	DriftAuthors      []string
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
	pairHistoricalAuthors := map[string]map[string]map[string]struct{}{}
	lastSyncIndex := map[string]map[string]int{}

	for i, c := range commits {
		if len(c.Files) == 0 {
			continue
		}
		if cfg.MaxFilesPerCommit > 0 && len(c.Files) > cfg.MaxFilesPerCommit {
			continue
		}

		files := uniqueSorted(c.Files)
		for _, a := range files {
			fileCommitCount[a]++
			fileCommitIndices[a] = append(fileCommitIndices[a], i)
		}

		for _, a := range files {
			if pairCounts[a] == nil {
				pairCounts[a] = map[string]int{}
			}
			if pairHistoricalAuthors[a] == nil {
				pairHistoricalAuthors[a] = map[string]map[string]struct{}{}
			}
			if lastSyncIndex[a] == nil {
				lastSyncIndex[a] = map[string]int{}
			}
			for _, b := range files {
				if a == b {
					continue
				}
				pairCounts[a][b]++
				if pairHistoricalAuthors[a][b] == nil {
					pairHistoricalAuthors[a][b] = map[string]struct{}{}
				}
				for _, author := range c.Authors {
					pairHistoricalAuthors[a][b][author] = struct{}{}
				}
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

			var historicalAuthors []string
			var driftAuthors []string
			contextLoss := false
			if !cfg.IgnoreSilo {
				historicalAuthors = sortedKeys(pairHistoricalAuthors[a][b])
				driftAuthors = driftAuthorsForPair(commits, fileCommitIndices[a], syncIdx, b)
				contextLoss = len(historicalAuthors) > 0 && len(driftAuthors) > 0 && !hasAnyAuthorOverlap(historicalAuthors, driftAuthors)
			}

			findings = append(findings, Finding{
				Source:            a,
				CoupledTo:         b,
				Coupling:          coupling,
				Shared:            shared,
				Drift:             drift,
				LastSyncHash:      commits[syncIdx].Hash,
				ContextLoss:       contextLoss,
				HistoricalAuthors: historicalAuthors,
				DriftAuthors:      driftAuthors,
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
			Path:              f.CoupledTo,
			Coupling:          f.Coupling,
			Shared:            f.Shared,
			ContextLoss:       f.ContextLoss,
			HistoricalAuthors: f.HistoricalAuthors,
			DriftAuthors:      f.DriftAuthors,
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

func sortedKeys(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func driftAuthorsForPair(commits []Commit, sourceIndices []int, syncIdx int, coupledTo string) []string {
	limit := countLessThan(sourceIndices, syncIdx)
	authors := map[string]struct{}{}
	for i := 0; i < limit; i++ {
		idx := sourceIndices[i]
		if commitTouchesFile(commits[idx].Files, coupledTo) {
			continue
		}
		for _, author := range commits[idx].Authors {
			if author == "" {
				continue
			}
			authors[author] = struct{}{}
		}
	}
	return sortedKeys(authors)
}

func commitTouchesFile(files []string, file string) bool {
	for _, f := range files {
		if f == file {
			return true
		}
	}
	return false
}

func hasAnyAuthorOverlap(a []string, b []string) bool {
	set := make(map[string]struct{}, len(a))
	for _, author := range a {
		set[strings.ToLower(author)] = struct{}{}
	}
	for _, author := range b {
		if _, ok := set[strings.ToLower(author)]; ok {
			return true
		}
	}
	return false
}
