package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigPrecedenceAndDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "fox-openapi.yaml")
	if err := os.WriteFile(configPath, []byte(`
entry: example.com/app/internal/server.NewEngine
out: api/from-config.json
format: json
sources:
  - ./internal/server
info:
  title: From Config
  version: 1.2.3
metadataHook: example.com/app/internal/server.ConfigureOpenAPI
autoAdd: true
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(Overrides{
		ConfigPath:          configPath,
		ConfigExplicit:      true,
		Out:                 "api/from-flag.yaml",
		OutSet:              true,
		Format:              "yaml",
		FormatSet:           true,
		Entry:               "example.com/app/internal/server.NewEngineWithError",
		EntrySet:            true,
		MetadataHook:        "",
		MetadataHookSet:     true,
		AutoAdd:             false,
		AutoAddSet:          true,
		IncludeTestFiles:    true,
		IncludeTestFilesSet: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Entry != "example.com/app/internal/server.NewEngineWithError" {
		t.Fatalf("entry override not applied: %s", cfg.Entry)
	}
	if cfg.Out != "api/from-flag.yaml" || cfg.Format != "yaml" {
		t.Fatalf("out/format override not applied: out=%s format=%s", cfg.Out, cfg.Format)
	}
	if cfg.Info.Title != "From Config" || cfg.Info.Version != "1.2.3" {
		t.Fatalf("config info not preserved: %+v", cfg.Info)
	}
	if cfg.MetadataHook != "" || cfg.AutoAdd {
		t.Fatalf("metadataHook/autoAdd overrides not applied: hook=%q autoAdd=%v", cfg.MetadataHook, cfg.AutoAdd)
	}
	if !cfg.IncludeTestFiles {
		t.Fatal("includeTestFiles override not applied")
	}
}

func TestLoadConfigMissingFileUsesDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadConfig(Overrides{
		Workdir:    dir,
		WorkdirSet: true,
		Entry:      "example.com/app.NewEngine",
		EntrySet:   true,
		Out:        "api/openapi.json",
		OutSet:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Workdir != dir {
		t.Fatalf("workdir = %q, want %q", cfg.Workdir, dir)
	}
	if cfg.Format != "json" {
		t.Fatalf("format = %q, want json", cfg.Format)
	}
	if len(cfg.Sources) != 1 || cfg.Sources[0] != "./..." {
		t.Fatalf("sources = %#v", cfg.Sources)
	}
}

func TestLoadConfigInvalidFormat(t *testing.T) {
	_, err := LoadConfig(Overrides{
		Entry:     "example.com/app.NewEngine",
		EntrySet:  true,
		Format:    "toml",
		FormatSet: true,
	})
	if err == nil {
		t.Fatal("expected invalid format error")
	}
}
