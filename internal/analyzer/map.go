package analyzer

import "sort"

type KnowledgeMapConfig struct {
	CouplingThreshold float64
	MaxFilesPerCommit int
	MaxCoupledFiles   int
	MaxAuthors        int
}

type CoupledFile struct {
	Path     string
	Coupling float64
}

type AuthorCommitCount struct {
	Author  string
	Commits int
}

type KnowledgeMap struct {
	Target  string
	Coupled []CoupledFile
	Authors []AuthorCommitCount
}

func BuildKnowledgeMap(commits []Commit, target string, cfg KnowledgeMapConfig) (KnowledgeMap, bool) {
	fileCommitCount := map[string]int{}
	pairCounts := map[string]int{}
	authorCounts := map[string]int{}

	for _, c := range commits {
		if len(c.Files) == 0 {
			continue
		}
		if cfg.MaxFilesPerCommit > 0 && len(c.Files) > cfg.MaxFilesPerCommit {
			continue
		}

		files := uniqueSorted(c.Files)
		touchesTarget := false
		for _, f := range files {
			fileCommitCount[f]++
			if f == target {
				touchesTarget = true
			}
		}
		if !touchesTarget {
			continue
		}

		for _, f := range files {
			if f == target {
				continue
			}
			pairCounts[f]++
		}
		for _, author := range c.Authors {
			if author == "" {
				continue
			}
			authorCounts[author]++
		}
	}

	targetCommits := fileCommitCount[target]
	if targetCommits == 0 {
		return KnowledgeMap{}, false
	}

	coupled := make([]CoupledFile, 0, len(pairCounts))
	for path, shared := range pairCounts {
		coupling := float64(shared) / float64(targetCommits)
		if coupling <= cfg.CouplingThreshold {
			continue
		}
		coupled = append(coupled, CoupledFile{
			Path:     path,
			Coupling: coupling,
		})
	}
	sort.Slice(coupled, func(i, j int) bool {
		if coupled[i].Coupling == coupled[j].Coupling {
			return coupled[i].Path < coupled[j].Path
		}
		return coupled[i].Coupling > coupled[j].Coupling
	})
	if cfg.MaxCoupledFiles > 0 && len(coupled) > cfg.MaxCoupledFiles {
		coupled = coupled[:cfg.MaxCoupledFiles]
	}
	if len(coupled) == 0 {
		return KnowledgeMap{}, false
	}

	authors := make([]AuthorCommitCount, 0, len(authorCounts))
	for author, count := range authorCounts {
		authors = append(authors, AuthorCommitCount{
			Author:  author,
			Commits: count,
		})
	}
	sort.Slice(authors, func(i, j int) bool {
		if authors[i].Commits == authors[j].Commits {
			return authors[i].Author < authors[j].Author
		}
		return authors[i].Commits > authors[j].Commits
	})
	if cfg.MaxAuthors > 0 && len(authors) > cfg.MaxAuthors {
		authors = authors[:cfg.MaxAuthors]
	}

	return KnowledgeMap{
		Target:  target,
		Coupled: coupled,
		Authors: authors,
	}, true
}
