package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitrot/internal/analyzer"
	"gitrot/internal/config"
	"gitrot/internal/git"
	"gitrot/internal/state"
)

type statusConfig struct {
	history     int
	minCoupling float64
	minShared   int
	minDrift    int
	maxFiles    int
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "status":
		err = runStatus(os.Args[2:])
	case "ack":
		err = runAck(os.Args[2:])
	case "init":
		err = runInit(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "gitrot: %v\n", err)
		os.Exit(1)
	}
}

func runStatus(args []string) error {
	repo, err := git.NewRepository(".")
	if err != nil {
		return err
	}

	cfg, err := loadStatusConfig(repo.Root(), args)
	if err != nil {
		return err
	}

	commits, err := repo.LoadCommits(cfg.history)
	if err != nil {
		return err
	}
	if len(commits) == 0 {
		fmt.Println("No commits found in this repository.")
		return nil
	}

	findings := analyzer.Analyze(toAnalyzerCommits(commits), analyzer.Config{
		CouplingThreshold: cfg.minCoupling / 100.0,
		MinSharedCommits:  cfg.minShared,
		MinDrift:          cfg.minDrift,
		MaxFilesPerCommit: cfg.maxFiles,
	})
	printFindings(analyzer.GroupBySource(findings), len(commits), cfg)
	return nil
}

func loadStatusConfig(repoRoot string, args []string) (statusConfig, error) {
	cfg := statusConfig{
		history:     2000,
		minCoupling: 60,
		minShared:   3,
		minDrift:    3,
		maxFiles:    30,
	}

	repoCfg, err := config.Load(filepath.Join(repoRoot, ".gitrot.toml"))
	if err != nil {
		return statusConfig{}, err
	}
	cfg.history = repoCfg.Thresholds.History
	cfg.maxFiles = repoCfg.Thresholds.MaxFiles
	cfg.minCoupling = repoCfg.Thresholds.MinCoupling
	cfg.minShared = repoCfg.Thresholds.MinShared
	cfg.minDrift = repoCfg.Thresholds.MinDrift

	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.IntVar(&cfg.history, "history", cfg.history, "number of past commits to analyze")
	fs.Float64Var(&cfg.minCoupling, "min-coupling", cfg.minCoupling, "minimum coupling percentage [0-100]")
	fs.IntVar(&cfg.minShared, "min-shared", cfg.minShared, "minimum shared commits between coupled files")
	fs.IntVar(&cfg.minDrift, "min-drift", cfg.minDrift, "minimum drift to report")
	fs.IntVar(&cfg.maxFiles, "max-files", cfg.maxFiles, "ignore commits touching more than this many files")
	if err := fs.Parse(args); err != nil {
		return statusConfig{}, err
	}

	if cfg.history < 1 {
		return statusConfig{}, fmt.Errorf("--history must be >= 1")
	}
	if cfg.minCoupling <= 0 || cfg.minCoupling > 100 {
		return statusConfig{}, fmt.Errorf("--min-coupling must be in (0, 100]")
	}
	if cfg.minShared < 1 {
		return statusConfig{}, fmt.Errorf("--min-shared must be >= 1")
	}
	if cfg.maxFiles < 1 {
		return statusConfig{}, fmt.Errorf("--max-files must be >= 1")
	}
	return cfg, nil
}

func runInit(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: gitrot init")
	}

	repo, err := git.NewRepository(".")
	if err != nil {
		return err
	}

	configPath := filepath.Join(repo.Root(), ".gitrot.toml")
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf(".gitrot.toml already exists")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check config file: %w", err)
	}

	if err := config.Save(configPath, config.Default()); err != nil {
		return err
	}

	fmt.Printf("Initialized %s\n", configPath)
	return nil
}

func runAck(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gitrot ack <file_path>")
	}

	repo, err := git.NewRepository(".")
	if err != nil {
		return err
	}

	filePath, err := normalizeAckPath(repo.Root(), args[0])
	if err != nil {
		return err
	}

	head, err := repo.HeadHash()
	if err != nil {
		return err
	}

	statePath := filepath.Join(repo.Root(), ".gitrot-state.json")
	st, err := state.Load(statePath)
	if err != nil {
		return err
	}

	st.Acknowledged[filePath] = head
	if err := state.Save(statePath, st); err != nil {
		return err
	}

	fmt.Printf("Acknowledged %s at %s\n", filePath, shortHash(head))
	return nil
}

func printFindings(findings []analyzer.GroupedFinding, analyzedCommits int, cfg statusConfig) {
	if len(findings) == 0 {
		fmt.Printf("✓ No dissonance detected (Analyzed %d commits. Thresholds: >%.0f%% coupling, >=%d drift, ignoring commits with >%d files).\n", analyzedCommits, cfg.minCoupling, cfg.minDrift, cfg.maxFiles)
		return
	}

	fmt.Println("⚠️  Dissonance Detected (Logical Coupling Broken)")
	fmt.Println()
	for _, f := range findings {
		fmt.Printf("[+%d Commits] %s (since %s)\n", f.Drift, f.Source, shortHash(f.LastSyncHash))
		fmt.Println(" ↳ Historically coupled files that were left behind:")

		displayCount := len(f.LeftBehind)
		if displayCount > 3 {
			displayCount = 3
		}
		for i := 0; i < displayCount; i++ {
			lb := f.LeftBehind[i]
			fmt.Printf("   - %s (%.0f%% coupling)\n", lb.Path, lb.Coupling*100)
		}
		if remaining := len(f.LeftBehind) - displayCount; remaining > 0 {
			fmt.Printf("   ... and %d more files.\n", remaining)
		}
		fmt.Println()
	}
}

func toAnalyzerCommits(commits []git.Commit) []analyzer.Commit {
	out := make([]analyzer.Commit, 0, len(commits))
	for _, c := range commits {
		out = append(out, analyzer.Commit{
			Hash:  c.Hash,
			Files: c.Files,
		})
	}
	return out
}

func shortHash(hash string) string {
	if len(hash) <= 7 {
		return hash
	}
	return hash[:7]
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  gitrot status [--history 2000] [--min-coupling 60] [--min-shared 3] [--min-drift 3] [--max-files 30]")
	fmt.Fprintln(os.Stderr, "  gitrot ack <file_path>")
	fmt.Fprintln(os.Stderr, "  gitrot init")
}

func normalizeAckPath(repoRoot, filePath string) (string, error) {
	if strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("file_path must not be empty")
	}

	candidate := filePath
	if filepath.IsAbs(candidate) {
		rel, err := filepath.Rel(repoRoot, candidate)
		if err != nil {
			return "", fmt.Errorf("normalize file_path: %w", err)
		}
		candidate = rel
	}

	candidate = filepath.Clean(candidate)
	candidate = filepath.ToSlash(candidate)
	if candidate == "." || candidate == "" {
		return "", fmt.Errorf("file_path must reference a file inside the repository")
	}
	if strings.HasPrefix(candidate, "../") {
		return "", fmt.Errorf("file_path must be inside the repository")
	}
	return candidate, nil
}
