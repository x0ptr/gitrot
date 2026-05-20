package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/x0ptr/gitrot/internal/analyzer"
	"github.com/x0ptr/gitrot/internal/config"
	"github.com/x0ptr/gitrot/internal/git"
	"github.com/x0ptr/gitrot/internal/state"
)

type statusConfig struct {
	history       int
	minCoupling   float64
	minCohesion   int
	minShared     int
	minDrift      int
	maxFiles      int
	ignoreTangled bool
	ignoreSilo    bool
}

type stagedConfig struct {
	history       int
	minCoupling   float64
	minCohesion   int
	maxFiles      int
	ignoreTangled bool
	ignoreSilo    bool
}

type exitCoder interface {
	error
	ExitCode() int
}

type commandExit struct {
	code int
}

func (e commandExit) Error() string {
	return ""
}

func (e commandExit) ExitCode() int {
	return e.code
}

type tangledWarning struct {
	Cohesion     int
	Threshold    int
	StagedCount  int
	ProblemFiles []string
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
	case "staged":
		err = runStaged(os.Args[2:])
	case "ack":
		err = runAck(os.Args[2:])
	case "init":
		err = runInit(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
	if err != nil {
		if exitErr, ok := err.(exitCoder); ok {
			os.Exit(exitErr.ExitCode())
		}
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
		IgnoreSilo:        cfg.ignoreSilo,
	})
	printFindings(analyzer.GroupBySource(findings), len(commits), cfg)

	if !cfg.ignoreTangled {
		warning, err := evaluateStagedGuard(repo, stagedConfig{
			history:       cfg.history,
			minCoupling:   cfg.minCoupling,
			minCohesion:   cfg.minCohesion,
			maxFiles:      cfg.maxFiles,
			ignoreTangled: cfg.ignoreTangled,
			ignoreSilo:    cfg.ignoreSilo,
		})
		if err != nil {
			return err
		}
		if warning != nil {
			printTangledWarning(os.Stderr, *warning)
		}
	}
	return nil
}

func loadStatusConfig(repoRoot string, args []string) (statusConfig, error) {
	cfg := statusConfig{
		history:       2000,
		minCoupling:   60,
		minCohesion:   30,
		minShared:     3,
		minDrift:      2,
		maxFiles:      30,
		ignoreTangled: false,
		ignoreSilo:    false,
	}

	repoCfg, err := config.Load(filepath.Join(repoRoot, ".gitrot.toml"))
	if err != nil {
		return statusConfig{}, err
	}
	cfg.history = repoCfg.Thresholds.History
	cfg.maxFiles = repoCfg.Thresholds.MaxFiles
	cfg.minCoupling = repoCfg.Thresholds.MinCoupling
	cfg.minCohesion = repoCfg.Thresholds.MinCohesion
	cfg.minShared = repoCfg.Thresholds.MinShared
	cfg.minDrift = repoCfg.Thresholds.MinDrift
	cfg.ignoreTangled = repoCfg.Features.IgnoreTangled
	cfg.ignoreSilo = repoCfg.Features.IgnoreSilo

	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.IntVar(&cfg.history, "history", cfg.history, "number of past commits to analyze")
	fs.Float64Var(&cfg.minCoupling, "min-coupling", cfg.minCoupling, "minimum coupling percentage [0-100]")
	fs.IntVar(&cfg.minCohesion, "min-cohesion", cfg.minCohesion, "minimum staged cohesion percentage [0-100]")
	fs.IntVar(&cfg.minShared, "min-shared", cfg.minShared, "minimum shared commits between coupled files")
	fs.IntVar(&cfg.minDrift, "min-drift", cfg.minDrift, "minimum drift to report")
	fs.IntVar(&cfg.maxFiles, "max-files", cfg.maxFiles, "ignore commits touching more than this many files")
	fs.BoolVar(&cfg.ignoreTangled, "ignore-tangled", cfg.ignoreTangled, "disable tangled commit detection")
	fs.BoolVar(&cfg.ignoreSilo, "ignore-silo", cfg.ignoreSilo, "disable silo detection (reserved)")
	if err := fs.Parse(args); err != nil {
		return statusConfig{}, err
	}

	if cfg.history < 1 {
		return statusConfig{}, fmt.Errorf("--history must be >= 1")
	}
	if cfg.minCoupling <= 0 || cfg.minCoupling > 100 {
		return statusConfig{}, fmt.Errorf("--min-coupling must be in (0, 100]")
	}
	if cfg.minCohesion < 0 || cfg.minCohesion > 100 {
		return statusConfig{}, fmt.Errorf("--min-cohesion must be in [0, 100]")
	}
	if cfg.minShared < 1 {
		return statusConfig{}, fmt.Errorf("--min-shared must be >= 1")
	}
	if cfg.maxFiles < 1 {
		return statusConfig{}, fmt.Errorf("--max-files must be >= 1")
	}
	return cfg, nil
}

func runStaged(args []string) error {
	if cliIgnoreTangled(args) {
		return nil
	}

	repo, err := git.NewRepository(".")
	if err != nil {
		return err
	}

	cfg, err := loadStagedConfig(repo.Root(), args)
	if err != nil {
		return err
	}
	if cfg.ignoreTangled {
		return nil
	}

	warning, err := evaluateStagedGuard(repo, cfg)
	if err != nil {
		return err
	}
	if warning == nil {
		return nil
	}

	printTangledWarning(os.Stderr, *warning)
	return commandExit{code: 1}
}

func cliIgnoreTangled(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "--ignore-tangled", "--ignore-tangled=true":
			return true
		}
	}
	return false
}

func loadStagedConfig(repoRoot string, args []string) (stagedConfig, error) {
	cfg := stagedConfig{
		history:       2000,
		minCoupling:   60,
		minCohesion:   30,
		maxFiles:      30,
		ignoreTangled: false,
		ignoreSilo:    false,
	}

	repoCfg, err := config.Load(filepath.Join(repoRoot, ".gitrot.toml"))
	if err != nil {
		return stagedConfig{}, err
	}
	cfg.history = repoCfg.Thresholds.History
	cfg.maxFiles = repoCfg.Thresholds.MaxFiles
	cfg.minCoupling = repoCfg.Thresholds.MinCoupling
	cfg.minCohesion = repoCfg.Thresholds.MinCohesion
	cfg.ignoreTangled = repoCfg.Features.IgnoreTangled
	cfg.ignoreSilo = repoCfg.Features.IgnoreSilo

	fs := flag.NewFlagSet("staged", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.IntVar(&cfg.history, "history", cfg.history, "number of past commits to analyze")
	fs.Float64Var(&cfg.minCoupling, "min-coupling", cfg.minCoupling, "minimum coupling percentage [0-100]")
	fs.IntVar(&cfg.minCohesion, "min-cohesion", cfg.minCohesion, "minimum staged cohesion percentage [0-100]")
	fs.IntVar(&cfg.maxFiles, "max-files", cfg.maxFiles, "ignore commits touching more than this many files")
	fs.BoolVar(&cfg.ignoreTangled, "ignore-tangled", cfg.ignoreTangled, "disable tangled commit detection")
	fs.BoolVar(&cfg.ignoreSilo, "ignore-silo", cfg.ignoreSilo, "disable silo detection (reserved)")
	if err := fs.Parse(args); err != nil {
		return stagedConfig{}, err
	}

	if cfg.history < 1 {
		return stagedConfig{}, fmt.Errorf("--history must be >= 1")
	}
	if cfg.minCoupling <= 0 || cfg.minCoupling > 100 {
		return stagedConfig{}, fmt.Errorf("--min-coupling must be in (0, 100]")
	}
	if cfg.minCohesion < 0 || cfg.minCohesion > 100 {
		return stagedConfig{}, fmt.Errorf("--min-cohesion must be in [0, 100]")
	}
	if cfg.maxFiles < 1 {
		return stagedConfig{}, fmt.Errorf("--max-files must be >= 1")
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
		for i := 0; i < displayCount; i++ {
			lb := f.LeftBehind[i]
			if !lb.ContextLoss {
				continue
			}
			fmt.Printf("   \x1b[31m🛑 Context Loss: Historically coupled by [%s], but recent drift caused by [%s].\x1b[0m\n", strings.Join(lb.HistoricalAuthors, ", "), strings.Join(lb.DriftAuthors, ", "))
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
			Hash:    c.Hash,
			Files:   c.Files,
			Authors: c.Authors,
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
	fmt.Fprintln(os.Stderr, "  gitrot status [--history 2000] [--min-coupling 60] [--min-cohesion 30] [--min-shared 3] [--min-drift 2] [--max-files 30] [--ignore-tangled] [--ignore-silo]")
	fmt.Fprintln(os.Stderr, "  gitrot staged [--history 2000] [--min-coupling 60] [--min-cohesion 30] [--max-files 30] [--ignore-tangled] [--ignore-silo]")
	fmt.Fprintln(os.Stderr, "  gitrot ack <file_path>")
	fmt.Fprintln(os.Stderr, "  gitrot init")
}

func evaluateStagedGuard(repo *git.Repository, cfg stagedConfig) (*tangledWarning, error) {
	if cfg.ignoreTangled {
		return nil, nil
	}

	stagedFiles, err := repo.StagedFiles()
	if err != nil {
		return nil, err
	}
	if len(stagedFiles) == 0 {
		return nil, nil
	}

	commits, err := repo.LoadCommits(cfg.history)
	if err != nil {
		return nil, err
	}

	result := analyzer.EvaluateStagedCohesion(toAnalyzerCommits(commits), stagedFiles, analyzer.StagedCohesionConfig{
		CouplingThreshold: cfg.minCoupling / 100.0,
		MaxFilesPerCommit: cfg.maxFiles,
	})
	if len(result.FilesWithHistory) < 2 {
		return nil, nil
	}
	if len(result.FilesWithHistory) >= 3 && result.Cohesion < cfg.minCohesion {
		return &tangledWarning{
			Cohesion:     result.Cohesion,
			Threshold:    cfg.minCohesion,
			StagedCount:  len(result.FilesWithHistory),
			ProblemFiles: result.DissonantFiles,
		}, nil
	}
	return nil, nil
}

func printTangledWarning(w io.Writer, warning tangledWarning) {
	fmt.Fprintf(w, "❌ Tangled Commit Detected (Low Cohesion: %d%% / Threshold: %d%%)\n", warning.Cohesion, warning.Threshold)
	fmt.Fprintf(w, "You are about to commit %d files that have rarely or never been committed together.\n", warning.StagedCount)
	if len(warning.ProblemFiles) > 0 {
		fmt.Fprintln(w, "Staged files causing dissonance:")
		for _, f := range warning.ProblemFiles {
			fmt.Fprintf(w, " - %s\n", f)
		}
	}
	fmt.Fprintln(w, "💡 Tip: Split this into smaller commits using `git add -p`, or bypass with `gitrot staged --ignore-tangled`.")
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
