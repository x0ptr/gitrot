package main

import (
	"path/filepath"
	"testing"

	"github.com/x0ptr/gitrot/internal/config"
)

func TestObfuscateAuthorDeterministic(t *testing.T) {
	first := obfuscateAuthor("Max Mustermann")
	second := obfuscateAuthor("Max Mustermann")
	if first != second {
		t.Fatalf("expected stable obfuscation, got %q and %q", first, second)
	}
	if len(first) != len("auth-8f2c3d1a") || first[:5] != "auth-" {
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
