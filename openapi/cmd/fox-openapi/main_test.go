package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCommonHonorsWorkdirAndConfigFlags(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fox-openapi.yaml"), []byte("entry: example.com/app.NewEngine\nout: api/from-config.yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, code := parseCommon("generate", []string{"--workdir", dir, "--out", "api/from-flag.json"})
	if code != 0 {
		t.Fatalf("parseCommon code = %d", code)
	}
	if cfg.Entry != "example.com/app.NewEngine" {
		t.Fatalf("entry = %q", cfg.Entry)
	}
	if cfg.Out != "api/from-flag.json" || cfg.Format != "json" {
		t.Fatalf("out/format = %q/%q", cfg.Out, cfg.Format)
	}

	otherConfig := filepath.Join(dir, "custom.yaml")
	if err := os.WriteFile(otherConfig, []byte("entry: example.com/app.Custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, code = parseCommon("generate", []string{"--config", otherConfig})
	if code != 0 {
		t.Fatalf("parseCommon custom config code = %d", code)
	}
	if cfg.Entry != "example.com/app.Custom" {
		t.Fatalf("custom config entry = %q", cfg.Entry)
	}
}
