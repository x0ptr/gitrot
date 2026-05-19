package config

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type File struct {
	Thresholds Thresholds `toml:"thresholds"`
}

type Thresholds struct {
	History     int     `toml:"history"`
	MaxFiles    int     `toml:"max_files"`
	MinCoupling float64 `toml:"min_coupling"`
	MinShared   int     `toml:"min_shared"`
	MinDrift    int     `toml:"min_drift"`
}

func Default() File {
	return File{
		Thresholds: Thresholds{
			History:     2000,
			MaxFiles:    30,
			MinCoupling: 60,
			MinShared:   3,
			MinDrift:    3,
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
	cfg.applyDefaults()
	return cfg, nil
}

func Save(path string, cfg File) error {
	cfg.applyDefaults()
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("serialize config: %w", err)
	}
	preamble := []byte("# .gitrot.toml\n")
	data = append(preamble, data...)
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
	if f.Thresholds.MinShared == 0 {
		f.Thresholds.MinShared = d.Thresholds.MinShared
	}
	if f.Thresholds.MinDrift == 0 {
		f.Thresholds.MinDrift = d.Thresholds.MinDrift
	}
}
