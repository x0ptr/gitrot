package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRespectsExplicitZeroMinCohesion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitrot.toml")
	content := []byte("[thresholds]\nmin_cohesion = 0\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Thresholds.MinCohesion != 0 {
		t.Fatalf("expected min_cohesion=0, got %d", cfg.Thresholds.MinCohesion)
	}
}

func TestLoadParsesFeatures(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitrot.toml")
	content := []byte("[features]\nignore_tangled = true\nignore_silo = true\nhide_name = true\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.Features.IgnoreTangled || !cfg.Features.IgnoreSilo || !cfg.Features.HideName {
		t.Fatalf("expected both feature flags true, got %#v", cfg.Features)
	}
}

func TestSaveWritesExpectedTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitrot.toml")
	if err := Save(path, Default()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	want := `# .gitrot.toml - Configuration for gitrot

[thresholds]
history = 2000       # Number of past commits to analyze
max_files = 30       # Ignore merge commits or mass refactors touching >30 files
min_coupling = 60    # Files must have been committed together >= 60% of the time
min_shared = 3       # Files must have at least 3 shared commits to be considered coupled
min_drift = 2        # Warn if a file is left behind by >= 2 commits
min_cohesion = 30    # Minimum cohesion percentage (0-100) for staged commits

[features]
ignore_tangled = false  # Set to true to disable Tangled Commit detection (` + "`gitrot staged`" + `)
ignore_silo = false     # Set to true to disable Context Loss/Silo detection
hide_name = false       # Set to true to obfuscate author names in ` + "`gitrot map`" + `
`

	if string(got) != want {
		t.Fatalf("unexpected config template:\n--- got ---\n%s\n--- want ---\n%s", string(got), want)
	}
}
