package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type File struct {
	Thresholds Thresholds `toml:"thresholds"`
	Features   Features   `toml:"features"`
}

type Thresholds struct {
	History     int     `toml:"history"`
	MaxFiles    int     `toml:"max_files"`
	MinCoupling float64 `toml:"min_coupling"`
	MinCohesion int     `toml:"min_cohesion"`
	MinShared   int     `toml:"min_shared"`
	MinDrift    int     `toml:"min_drift"`
}

type Features struct {
	IgnoreTangled bool `toml:"ignore_tangled"`
	IgnoreSilo    bool `toml:"ignore_silo"`
	HideName      bool `toml:"hide_name"`
}

func Default() File {
	return File{
		Thresholds: Thresholds{
			History:     2000,
			MaxFiles:    30,
			MinCoupling: 60,
			MinCohesion: 30,
			MinShared:   3,
			MinDrift:    2,
		},
		Features: Features{
			IgnoreTangled: false,
			IgnoreSilo:    false,
			HideName:      false,
		},
	}
}

func Load(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return File{}, fmt.Errorf("read config: %w", err)
	}

	var cfg File
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return File{}, fmt.Errorf("parse config: %w", err)
	}

	var raw struct {
		Thresholds struct {
			MinCohesion *int `toml:"min_cohesion"`
		} `toml:"thresholds"`
	}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return File{}, fmt.Errorf("parse config: %w", err)
	}

	cfg.applyDefaults()
	if raw.Thresholds.MinCohesion != nil {
		cfg.Thresholds.MinCohesion = *raw.Thresholds.MinCohesion
	}
	return cfg, nil
}

func Save(path string, cfg File) error {
	cfg.applyDefaults()
	data := []byte(render(cfg))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func (f *File) applyDefaults() {
	d := Default()
	if f.Thresholds.History == 0 {
		f.Thresholds.History = d.Thresholds.History
	}
	if f.Thresholds.MaxFiles == 0 {
		f.Thresholds.MaxFiles = d.Thresholds.MaxFiles
	}
	if f.Thresholds.MinCoupling == 0 {
		f.Thresholds.MinCoupling = d.Thresholds.MinCoupling
	}
	if f.Thresholds.MinCohesion == 0 {
		f.Thresholds.MinCohesion = d.Thresholds.MinCohesion
	}
	if f.Thresholds.MinShared == 0 {
		f.Thresholds.MinShared = d.Thresholds.MinShared
	}
	if f.Thresholds.MinDrift == 0 {
		f.Thresholds.MinDrift = d.Thresholds.MinDrift
	}
}

func render(cfg File) string {
	t := cfg.Thresholds
	f := cfg.Features
	var b strings.Builder
	fmt.Fprintf(&b, "# .gitrot.toml - Configuration for gitrot\n\n")
	fmt.Fprintf(&b, "[thresholds]\n")
	fmt.Fprintf(&b, "history = %d       # Number of past commits to analyze\n", t.History)
	fmt.Fprintf(&b, "max_files = %d       # Ignore merge commits or mass refactors touching >30 files\n", t.MaxFiles)
	fmt.Fprintf(&b, "min_coupling = %.0f    # Files must have been committed together >= 60%% of the time\n", t.MinCoupling)
	fmt.Fprintf(&b, "min_shared = %d       # Files must have at least 3 shared commits to be considered coupled\n", t.MinShared)
	fmt.Fprintf(&b, "min_drift = %d        # Warn if a file is left behind by >= 2 commits\n", t.MinDrift)
	fmt.Fprintf(&b, "min_cohesion = %d    # Minimum cohesion percentage (0-100) for staged commits\n\n", t.MinCohesion)
	fmt.Fprintf(&b, "[features]\n")
	fmt.Fprintf(&b, "ignore_tangled = %t  # Set to true to disable Tangled Commit detection (`gitrot staged`)\n", f.IgnoreTangled)
	fmt.Fprintf(&b, "ignore_silo = %t     # Set to true to disable Context Loss/Silo detection\n", f.IgnoreSilo)
	fmt.Fprintf(&b, "hide_name = %t       # Set to true to obfuscate author names in `gitrot map`\n", f.HideName)
	return b.String()
}
