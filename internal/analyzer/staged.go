package analyzer

import "sort"

type StagedCohesionConfig struct {
	CouplingThreshold float64
	MaxFilesPerCommit int
}

type StagedCohesionResult struct {
	FilesWithHistory []string
	TotalPairs       int
	CoupledPairs     int
	Cohesion         int
	DissonantFiles   []string
}

func EvaluateStagedCohesion(commits []Commit, stagedFiles []string, cfg StagedCohesionConfig) StagedCohesionResult {
	fileCommitCount := buildFileCommitCount(commits, cfg.MaxFilesPerCommit)

	seen := make(map[string]struct{}, len(stagedFiles))
	filtered := make([]string, 0, len(stagedFiles))
	for _, f := range stagedFiles {
		if _, ok := fileCommitCount[f]; !ok {
			continue
		}
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		filtered = append(filtered, f)
	}
	sort.Strings(filtered)

	result := StagedCohesionResult{
		FilesWithHistory: filtered,
	}
	if len(filtered) < 2 {
		return result
	}

	pairCounts := buildPairCounts(commits, cfg.MaxFilesPerCommit)
	totalPairs := len(filtered) * (len(filtered) - 1) / 2
	coupledPairs := 0
	dissonant := make(map[string]struct{}, len(filtered))

	for i := 0; i < len(filtered); i++ {
		for j := i + 1; j < len(filtered); j++ {
			a, b := filtered[i], filtered[j]
			if pairCoupling(fileCommitCount, pairCounts, a, b) >= cfg.CouplingThreshold {
				coupledPairs++
				continue
			}
			dissonant[a] = struct{}{}
			dissonant[b] = struct{}{}
		}
	}

	result.TotalPairs = totalPairs
	result.CoupledPairs = coupledPairs
	result.Cohesion = int((float64(coupledPairs) * 100.0) / float64(totalPairs))
	if len(dissonant) > 0 {
		result.DissonantFiles = make([]string, 0, len(dissonant))
		for f := range dissonant {
			result.DissonantFiles = append(result.DissonantFiles, f)
		}
		sort.Strings(result.DissonantFiles)
	}
	return result
}

func buildFileCommitCount(commits []Commit, maxFilesPerCommit int) map[string]int {
	fileCommitCount := map[string]int{}
	for _, c := range commits {
		if len(c.Files) == 0 {
			continue
		}
		if maxFilesPerCommit > 0 && len(c.Files) > maxFilesPerCommit {
			continue
		}
		seen := map[string]struct{}{}
		for _, f := range c.Files {
			if _, ok := seen[f]; ok {
				continue
			}
			seen[f] = struct{}{}
			fileCommitCount[f]++
		}
	}
	return fileCommitCount
}

func buildPairCounts(commits []Commit, maxFilesPerCommit int) map[string]map[string]int {
	pairCounts := map[string]map[string]int{}
	for _, c := range commits {
		if len(c.Files) == 0 {
			continue
		}
		if maxFilesPerCommit > 0 && len(c.Files) > maxFilesPerCommit {
			continue
		}

		files := uniqueSorted(c.Files)
		for i, a := range files {
			if pairCounts[a] == nil {
				pairCounts[a] = map[string]int{}
			}
			for _, b := range files[i+1:] {
				if pairCounts[b] == nil {
					pairCounts[b] = map[string]int{}
				}
				pairCounts[a][b]++
				pairCounts[b][a]++
			}
		}
	}
	return pairCounts
}

func pairCoupling(fileCommitCount map[string]int, pairCounts map[string]map[string]int, a, b string) float64 {
	shared := 0
	if pairCounts[a] != nil {
		shared = pairCounts[a][b]
	}
	if shared == 0 {
		return 0
	}
	totalA := fileCommitCount[a]
	totalB := fileCommitCount[b]
	if totalA == 0 || totalB == 0 {
		return 0
	}
	couplingAB := float64(shared) / float64(totalA)
	couplingBA := float64(shared) / float64(totalB)
	if couplingAB > couplingBA {
		return couplingAB
	}
	return couplingBA
}

func uniqueSorted(files []string) []string {
	if len(files) == 0 {
		return nil
	}
	copyFiles := append([]string(nil), files...)
	sort.Strings(copyFiles)
	out := copyFiles[:0]
	var prev string
	for i, f := range copyFiles {
		if i == 0 || f != prev {
			out = append(out, f)
			prev = f
		}
	}
	return out
}
