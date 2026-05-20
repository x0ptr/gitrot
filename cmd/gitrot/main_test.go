package main

import (
	"path/filepath"
	"testing"

	"github.com/x0ptr/gitrot/internal/config"
	"github.com/x0ptr/gitrot/internal/utils"
)

func TestObfuscateAuthorDeterministic(t *testing.T) {
	first := utils.FormatAuthorName("Max Mustermann", true)
	second := utils.FormatAuthorName("Max Mustermann", true)
	if first != second {
		t.Fatalf("expected stable obfuscation, got %q and %q", first, second)
	}
	if len(first) != len("usr-12345678") || first[:4] != "usr-" {
		t.Fatalf("unexpected obfuscated format: %q", first)
	}
}

func TestLoadMapConfigHideNamePrecedence(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".gitrot.toml")
	cfg := config.Default()
	cfg.Features.HideName = true
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	fromConfig, _, err := loadMapConfig(dir, []string{"src/a.go"})
	if err != nil {
		t.Fatalf("load map config from file: %v", err)
	}
	if !fromConfig.hideName {
		t.Fatalf("expected hide-name from config to be true")
	}

	fromFlag, _, err := loadMapConfig(dir, []string{"--hide-name=false", "src/a.go"})
	if err != nil {
		t.Fatalf("load map config from flag: %v", err)
	}
	if fromFlag.hideName {
		t.Fatalf("expected explicit --hide-name=false to override config")
	}
}

func TestLoadHotspotConfigParsesOptionalPath(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".gitrot.toml")
	if err := config.Save(cfgPath, config.Default()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	withoutPath, err := loadHotspotConfig(dir, []string{})
	if err != nil {
		t.Fatalf("load hotspot config without path: %v", err)
	}
	if withoutPath.targetPath != "" {
		t.Fatalf("expected empty target path, got %q", withoutPath.targetPath)
	}
	if withoutPath.limit != 10 {
		t.Fatalf("expected default limit=10, got %d", withoutPath.limit)
	}

	withPath, err := loadHotspotConfig(dir, []string{"src/api"})
	if err != nil {
		t.Fatalf("load hotspot config with path: %v", err)
	}
	if withPath.targetPath != "src/api" {
		t.Fatalf("expected normalized target path src/api, got %q", withPath.targetPath)
	}

	withLimit, err := loadHotspotConfig(dir, []string{"--limit=25", "src/api"})
	if err != nil {
		t.Fatalf("load hotspot config with limit: %v", err)
	}
	if withLimit.limit != 25 {
		t.Fatalf("expected limit=25, got %d", withLimit.limit)
	}
}

func TestLoadStatusConfigHideNamePrecedence(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".gitrot.toml")
	cfg := config.Default()
	cfg.Features.HideName = true
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	fromConfig, err := loadStatusConfig(dir, []string{})
	if err != nil {
		t.Fatalf("load status config: %v", err)
	}
	if !fromConfig.hideName {
		t.Fatalf("expected hide-name from config to be true")
	}

	fromFlag, err := loadStatusConfig(dir, []string{"--hide-name=false"})
	if err != nil {
		t.Fatalf("load status config from flag: %v", err)
	}
	if fromFlag.hideName {
		t.Fatalf("expected explicit --hide-name=false to override config")
	}
}

func TestLoadStagedConfigHideNamePrecedence(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".gitrot.toml")
	cfg := config.Default()
	cfg.Features.HideName = true
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	fromConfig, err := loadStagedConfig(dir, []string{})
	if err != nil {
		t.Fatalf("load staged config: %v", err)
	}
	if !fromConfig.hideName {
		t.Fatalf("expected hide-name from config to be true")
	}

	fromFlag, err := loadStagedConfig(dir, []string{"--hide-name=false"})
	if err != nil {
		t.Fatalf("load staged config from flag: %v", err)
	}
	if fromFlag.hideName {
		t.Fatalf("expected explicit --hide-name=false to override config")
	}
}

func TestLoadHotspotConfigHideNamePrecedence(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".gitrot.toml")
	cfg := config.Default()
	cfg.Features.HideName = true
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	fromConfig, err := loadHotspotConfig(dir, []string{})
	if err != nil {
		t.Fatalf("load hotspot config: %v", err)
	}
	if !fromConfig.hideName {
		t.Fatalf("expected hide-name from config to be true")
	}

	fromFlag, err := loadHotspotConfig(dir, []string{"--hide-name=false"})
	if err != nil {
		t.Fatalf("load hotspot config from flag: %v", err)
	}
	if fromFlag.hideName {
		t.Fatalf("expected explicit --hide-name=false to override config")
	}
}

func TestLoadStatusConfigIgnoreDotfilesPrecedence(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".gitrot.toml")
	cfg := config.Default()
	cfg.Features.IgnoreDotfiles = true
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	fromConfig, err := loadStatusConfig(dir, []string{})
	if err != nil {
		t.Fatalf("load status config: %v", err)
	}
	if !fromConfig.ignoreDotfiles {
		t.Fatalf("expected ignore-dotfiles from config to be true")
	}

	fromFlag, err := loadStatusConfig(dir, []string{"--ignore-dotfiles=false"})
	if err != nil {
		t.Fatalf("load status config from flag: %v", err)
	}
	if fromFlag.ignoreDotfiles {
		t.Fatalf("expected explicit --ignore-dotfiles=false to override config")
	}
}
