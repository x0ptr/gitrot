package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/x0ptr/gitrot/internal/analyzer"
	"github.com/x0ptr/gitrot/internal/config"
	"github.com/x0ptr/gitrot/internal/git"
	"github.com/x0ptr/gitrot/internal/state"
	"github.com/x0ptr/gitrot/internal/utils"
)

type statusConfig struct {
	history        int
	minCoupling    float64
	minCohesion    int
	minShared      int
	minDrift       int
	maxFiles       int
	ignoreTangled  bool
	ignoreSilo     bool
	hideName       bool
	ignoreDotfiles bool
}

type stagedConfig struct {
	history        int
	minCoupling    float64
	minCohesion    int
	maxFiles       int
	ignoreTangled  bool
	ignoreSilo     bool
	hideName       bool
	ignoreDotfiles bool
}

type mapConfig struct {
	history        int
	maxFiles       int
	hideName       bool
	ignoreDotfiles bool
}

type hotspotConfig struct {
	history        int
	minCoupling    float64
	maxFiles       int
	targetPath     string
	hideName       bool
	ignoreDotfiles bool
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

var (
	warningLabel = color.New(color.FgRed, color.Bold).SprintFunc()
	insightLabel = color.New(color.FgYellow).SprintFunc()
	tipLabel     = color.New(color.FgYellow).SprintFunc()
	pathText     = color.New(color.FgCyan).SprintFunc()
	metricText   = color.New(color.FgCyan).SprintFunc()
	nameText     = color.New(color.FgMagenta).SprintFunc()
)

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
	case "map":
		err = runMap(os.Args[2:])
	case "hotspot":
		err = runHotspot(os.Args[2:])
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

	commits, err := repo.LoadCommits(cfg.history, cfg.ignoreDotfiles)
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
			history:        cfg.history,
			minCoupling:    cfg.minCoupling,
			minCohesion:    cfg.minCohesion,
			maxFiles:       cfg.maxFiles,
			ignoreTangled:  cfg.ignoreTangled,
			ignoreSilo:     cfg.ignoreSilo,
			hideName:       cfg.hideName,
			ignoreDotfiles: cfg.ignoreDotfiles,
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
		history:        2000,
		minCoupling:    60,
		minCohesion:    30,
		minShared:      3,
		minDrift:       2,
		maxFiles:       30,
		ignoreTangled:  false,
		ignoreSilo:     false,
		hideName:       false,
		ignoreDotfiles: true,
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
	cfg.hideName = repoCfg.Features.HideName
	cfg.ignoreDotfiles = repoCfg.Features.IgnoreDotfiles

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
	fs.BoolVar(&cfg.hideName, "hide-name", cfg.hideName, "obfuscate developer names in output")
	fs.BoolVar(&cfg.ignoreDotfiles, "ignore-dotfiles", cfg.ignoreDotfiles, "exclude dotfiles and hidden directories from analysis")
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
		history:        2000,
		minCoupling:    60,
		minCohesion:    30,
		maxFiles:       30,
		ignoreTangled:  false,
		ignoreSilo:     false,
		hideName:       false,
		ignoreDotfiles: true,
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
	cfg.hideName = repoCfg.Features.HideName
	cfg.ignoreDotfiles = repoCfg.Features.IgnoreDotfiles

	fs := flag.NewFlagSet("staged", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.IntVar(&cfg.history, "history", cfg.history, "number of past commits to analyze")
	fs.Float64Var(&cfg.minCoupling, "min-coupling", cfg.minCoupling, "minimum coupling percentage [0-100]")
	fs.IntVar(&cfg.minCohesion, "min-cohesion", cfg.minCohesion, "minimum staged cohesion percentage [0-100]")
	fs.IntVar(&cfg.maxFiles, "max-files", cfg.maxFiles, "ignore commits touching more than this many files")
	fs.BoolVar(&cfg.ignoreTangled, "ignore-tangled", cfg.ignoreTangled, "disable tangled commit detection")
	fs.BoolVar(&cfg.ignoreSilo, "ignore-silo", cfg.ignoreSilo, "disable silo detection (reserved)")
	fs.BoolVar(&cfg.hideName, "hide-name", cfg.hideName, "obfuscate developer names in output")
	fs.BoolVar(&cfg.ignoreDotfiles, "ignore-dotfiles", cfg.ignoreDotfiles, "exclude dotfiles and hidden directories from analysis")
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

func runMap(args []string) error {
	repo, err := git.NewRepository(".")
	if err != nil {
		return err
	}

	cfg, targetFile, err := loadMapConfig(repo.Root(), args)
	if err != nil {
		return err
	}
	targetFile, err = normalizeAckPath(repo.Root(), targetFile)
	if err != nil {
		return err
	}

	commits, err := repo.LoadCommits(cfg.history, cfg.ignoreDotfiles)
	if err != nil {
		return err
	}
	if len(commits) == 0 {
		fmt.Fprintf(os.Stderr, "Error: No historical data found for %s\n", targetFile)
		return commandExit{code: 1}
	}

	knowledge, ok := analyzer.BuildKnowledgeMap(toAnalyzerCommits(commits), targetFile, analyzer.KnowledgeMapConfig{
		CouplingThreshold: 0.20,
		MaxFilesPerCommit: cfg.maxFiles,
		MaxCoupledFiles:   8,
		MaxAuthors:        5,
	})
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: No historical data found for %s\n", targetFile)
		return commandExit{code: 1}
	}

	printKnowledgeMap(os.Stdout, knowledge, cfg.hideName)
	return nil
}

func runHotspot(args []string) error {
	repo, err := git.NewRepository(".")
	if err != nil {
		return err
	}

	cfg, err := loadHotspotConfig(repo.Root(), args)
	if err != nil {
		return err
	}

	commits, err := repo.LoadCommits(cfg.history, cfg.ignoreDotfiles)
	if err != nil {
		return err
	}
	if len(commits) == 0 {
		fmt.Println("No critical hotspots detected based on current thresholds.")
		return nil
	}

	hotspots := analyzer.IdentifyHotspots(toAnalyzerCommits(commits), analyzer.HotspotConfig{
		CouplingThreshold: cfg.minCoupling / 100.0,
		MaxFilesPerCommit: cfg.maxFiles,
		MaxResults:        10,
		MinCommits:        5,
		TargetPath:        cfg.targetPath,
	})
	if len(hotspots) == 0 {
		fmt.Println("No critical hotspots detected based on current thresholds.")
		return nil
	}

	printHotspots(os.Stdout, hotspots, cfg.targetPath)
	return nil
}

func loadHotspotConfig(repoRoot string, args []string) (hotspotConfig, error) {
	cfg := hotspotConfig{
		history:        2000,
		minCoupling:    60,
		maxFiles:       30,
		hideName:       false,
		ignoreDotfiles: true,
	}

	repoCfg, err := config.Load(filepath.Join(repoRoot, ".gitrot.toml"))
	if err != nil {
		return hotspotConfig{}, err
	}
	cfg.history = repoCfg.Thresholds.History
	cfg.minCoupling = repoCfg.Thresholds.MinCoupling
	cfg.maxFiles = repoCfg.Thresholds.MaxFiles
	cfg.hideName = repoCfg.Features.HideName
	cfg.ignoreDotfiles = repoCfg.Features.IgnoreDotfiles

	fs := flag.NewFlagSet("hotspot", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.IntVar(&cfg.history, "history", cfg.history, "number of past commits to analyze")
	fs.Float64Var(&cfg.minCoupling, "min-coupling", cfg.minCoupling, "minimum coupling percentage [0-100]")
	fs.IntVar(&cfg.maxFiles, "max-files", cfg.maxFiles, "ignore commits touching more than this many files")
	fs.BoolVar(&cfg.hideName, "hide-name", cfg.hideName, "obfuscate developer names in output")
	fs.BoolVar(&cfg.ignoreDotfiles, "ignore-dotfiles", cfg.ignoreDotfiles, "exclude dotfiles and hidden directories from analysis")
	if err := fs.Parse(args); err != nil {
		return hotspotConfig{}, err
	}
	if fs.NArg() > 1 {
		return hotspotConfig{}, fmt.Errorf("usage: gitrot hotspot [--history 2000] [--min-coupling 60] [--max-files 30] [--hide-name] [--ignore-dotfiles] [path]")
	}
	if fs.NArg() == 1 {
		targetPath, err := normalizeHotspotTargetPath(repoRoot, fs.Arg(0))
		if err != nil {
			return hotspotConfig{}, err
		}
		cfg.targetPath = targetPath
	}

	if cfg.history < 1 {
		return hotspotConfig{}, fmt.Errorf("--history must be >= 1")
	}
	if cfg.minCoupling <= 0 || cfg.minCoupling > 100 {
		return hotspotConfig{}, fmt.Errorf("--min-coupling must be in (0, 100]")
	}
	if cfg.maxFiles < 1 {
		return hotspotConfig{}, fmt.Errorf("--max-files must be >= 1")
	}
	return cfg, nil
}

func loadMapConfig(repoRoot string, args []string) (mapConfig, string, error) {
	cfg := mapConfig{
		history:        2000,
		maxFiles:       30,
		hideName:       false,
		ignoreDotfiles: true,
	}

	repoCfg, err := config.Load(filepath.Join(repoRoot, ".gitrot.toml"))
	if err != nil {
		return mapConfig{}, "", err
	}
	cfg.history = repoCfg.Thresholds.History
	cfg.maxFiles = repoCfg.Thresholds.MaxFiles
	cfg.hideName = repoCfg.Features.HideName
	cfg.ignoreDotfiles = repoCfg.Features.IgnoreDotfiles

	fs := flag.NewFlagSet("map", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.BoolVar(&cfg.hideName, "hide-name", cfg.hideName, "obfuscate developer names in output")
	fs.BoolVar(&cfg.ignoreDotfiles, "ignore-dotfiles", cfg.ignoreDotfiles, "exclude dotfiles and hidden directories from analysis")
	if err := fs.Parse(args); err != nil {
		return mapConfig{}, "", err
	}
	if fs.NArg() != 1 {
		return mapConfig{}, "", fmt.Errorf("usage: gitrot map [--hide-name] [--ignore-dotfiles] <file_path>")
	}
	targetFile := fs.Arg(0)

	if cfg.history < 1 {
		return mapConfig{}, "", fmt.Errorf("history must be >= 1")
	}
	if cfg.maxFiles < 1 {
		return mapConfig{}, "", fmt.Errorf("max_files must be >= 1")
	}
	return cfg, targetFile, nil
}

func printFindings(findings []analyzer.GroupedFinding, analyzedCommits int, cfg statusConfig) {
	if len(findings) == 0 {
		fmt.Printf("No coupling drift detected (Analyzed %s commits. Thresholds: >%s coupling, >=%s drift, ignoring commits with >%s files).\n", metricText(analyzedCommits), metricText(fmt.Sprintf("%.0f%%", cfg.minCoupling)), metricText(cfg.minDrift), metricText(cfg.maxFiles))
		return
	}

	fmt.Printf("%s Coupling Drift Signals Detected\n", warningLabel("[WARNING]"))
	fmt.Println()
	for _, f := range findings {
		fmt.Printf("[+%s commits] %s (since %s)\n", metricText(f.Drift), pathText(f.Source), metricText(shortHash(f.LastSyncHash)))
		fmt.Println("   Historically coupled files with reduced co-change activity:")

		displayCount := len(f.LeftBehind)
		if displayCount > 3 {
			displayCount = 3
		}
		for i := 0; i < displayCount; i++ {
			lb := f.LeftBehind[i]
			fmt.Printf("   - %s (%s coupling)\n", pathText(lb.Path), metricText(fmt.Sprintf("%.0f%%", lb.Coupling*100)))
		}
		for i := 0; i < displayCount; i++ {
			lb := f.LeftBehind[i]
			if !lb.ContextLoss {
				continue
			}
			fmt.Printf("   %s Knowledge Transfer Gap: %s is modifying this file, but %s usually handles the coupled files. Consider a review sync.\n", insightLabel("[INSIGHT]"), formatNames(lb.DriftAuthors, cfg.hideName), formatNames(lb.HistoricalAuthors, cfg.hideName))
		}
		if remaining := len(f.LeftBehind) - displayCount; remaining > 0 {
			fmt.Printf("   ... and %s more files.\n", metricText(remaining))
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
	fmt.Fprintln(os.Stderr, "  gitrot status [--history 2000] [--min-coupling 60] [--min-cohesion 30] [--min-shared 3] [--min-drift 2] [--max-files 30] [--ignore-tangled] [--ignore-silo] [--hide-name] [--ignore-dotfiles]")
	fmt.Fprintln(os.Stderr, "  gitrot staged [--history 2000] [--min-coupling 60] [--min-cohesion 30] [--max-files 30] [--ignore-tangled] [--ignore-silo] [--hide-name] [--ignore-dotfiles]")
	fmt.Fprintln(os.Stderr, "  gitrot map [--hide-name] [--ignore-dotfiles] <file_path>")
	fmt.Fprintln(os.Stderr, "  gitrot hotspot [--history 2000] [--min-coupling 60] [--max-files 30] [--hide-name] [--ignore-dotfiles] [path]")
	fmt.Fprintln(os.Stderr, "  gitrot ack <file_path>")
	fmt.Fprintln(os.Stderr, "  gitrot init")
}

func printKnowledgeMap(w io.Writer, knowledge analyzer.KnowledgeMap, hideName bool) {
	fmt.Fprintf(w, "Knowledge Map for: %s\n", pathText(knowledge.Target))
	fmt.Fprintln(w, "--------------------------------------------------")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Historically, when this file changes, the following files also change:")
	for _, c := range knowledge.Coupled {
		fmt.Fprintf(w, "  - %s (%s coupling)\n", pathText(c.Path), metricText(fmt.Sprintf("%.0f%%", c.Coupling*100)))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Primary Knowledge Holders (Top Authors):")
	for _, a := range knowledge.Authors {
		fmt.Fprintf(w, "  - %s (%s commits)\n", nameText(utils.FormatAuthorName(a.Author, hideName)), metricText(a.Commits))
	}
}

func printHotspots(w io.Writer, hotspots []analyzer.Hotspot, targetPath string) {
	fmt.Fprintln(w, "Refactoring Hotspots (High Coupling + High Churn)")
	if targetPath == "" {
		fmt.Fprintln(w, "Target: Entire Repository")
	} else {
		fmt.Fprintf(w, "Target: %s\n", pathText(targetPath))
	}
	fmt.Fprintln(w, "--------------------------------------------------")
	fmt.Fprintln(w)
	for i, h := range hotspots {
		fmt.Fprintf(w, "%d. %s\n", i+1, pathText(h.Path))
		fmt.Fprintf(w, "   Score: %s | Coupled to: %s files | Changes: %s commits\n", metricText(h.Score), metricText(h.CouplingDegree), metricText(h.Churn))
		if i == 0 {
			fmt.Fprintf(w, "   %s High coupling and churn indicate architectural debt. Consider splitting this module.\n", insightLabel("[INSIGHT]"))
		}
		fmt.Fprintln(w)
	}
}

func normalizeHotspotTargetPath(repoRoot, target string) (string, error) {
	t := strings.TrimSpace(target)
	if t == "" || t == "." {
		return "", nil
	}

	candidate := t
	if filepath.IsAbs(candidate) {
		rel, err := filepath.Rel(repoRoot, candidate)
		if err != nil {
			return "", fmt.Errorf("normalize hotspot path: %w", err)
		}
		candidate = rel
	}

	candidate = filepath.Clean(candidate)
	candidate = filepath.ToSlash(candidate)
	if candidate == "." {
		return "", nil
	}
	if strings.HasPrefix(candidate, "../") {
		return "", fmt.Errorf("hotspot path must be inside the repository")
	}
	return candidate, nil
}

func evaluateStagedGuard(repo *git.Repository, cfg stagedConfig) (*tangledWarning, error) {
	if cfg.ignoreTangled {
		return nil, nil
	}

	stagedFiles, err := repo.StagedFiles(cfg.ignoreDotfiles)
	if err != nil {
		return nil, err
	}
	if len(stagedFiles) == 0 {
		return nil, nil
	}

	commits, err := repo.LoadCommits(cfg.history, cfg.ignoreDotfiles)
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
	fmt.Fprintf(w, "%s Atypical Commit Composition (Cohesion: %s / Target: %s)\n", warningLabel("[WARNING]"), metricText(fmt.Sprintf("%d%%", warning.Cohesion)), metricText(fmt.Sprintf("%d%%", warning.Threshold)))
	fmt.Fprintf(w, "This commit currently includes %s files that rarely change together.\n", metricText(warning.StagedCount))
	if len(warning.ProblemFiles) > 0 {
		fmt.Fprintln(w, "Files contributing most to low cohesion:")
		for _, f := range warning.ProblemFiles {
			fmt.Fprintf(w, " - %s\n", pathText(f))
		}
	}
	fmt.Fprintf(w, "%s Splitting these changes using 'git add -p' might make code review easier.\n", tipLabel("[TIP]"))
}

func formatNames(names []string, hideName bool) string {
	if len(names) == 0 {
		return nameText("unknown authors")
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, nameText(utils.FormatAuthorName(n, hideName)))
	}
	return strings.Join(out, ", ")
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
