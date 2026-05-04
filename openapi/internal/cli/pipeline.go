package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/mod/modfile"
)

func RunPipeline(cfg Config) ([]byte, []string, error) {
	if err := EnsureOpenAPIModule(cfg); err != nil {
		return nil, nil, err
	}
	entry, err := ResolveEntry(cfg.Workdir, cfg.Entry)
	if err != nil {
		return nil, nil, err
	}
	var hook *Hook
	if cfg.MetadataHook != "" {
		resolved, err := ResolveHook(cfg.Workdir, cfg.MetadataHook)
		if err != nil {
			return nil, nil, err
		}
		hook = &resolved
	}
	driverDir, err := WriteDriver(cfg, entry, hook)
	if err != nil {
		return nil, nil, err
	}
	defer CleanupDriver(driverDir, cfg.KeepDriver)
	data, stderr, err := RunDriver(driverDir)
	if err != nil {
		return nil, warningLines(stderr), err
	}
	return data, warningLines(stderr), nil
}

func ResolveOutputPath(cfg Config) string {
	if filepath.IsAbs(cfg.Out) {
		return cfg.Out
	}
	return filepath.Join(cfg.Workdir, cfg.Out)
}

func EnsureOpenAPIModule(cfg Config) error {
	hasRequire, err := hasOpenAPIRequire(cfg.Workdir)
	if err != nil {
		return err
	}
	if hasRequire {
		return nil
	}
	if !cfg.AutoAdd {
		return fmt.Errorf("user module must require github.com/fox-gonic/fox-openapi; run `go get github.com/fox-gonic/fox-openapi` or pass --auto-add")
	}
	add := exec.Command("go", "get", "github.com/fox-gonic/fox-openapi")
	add.Dir = cfg.Workdir
	add.Stdout = os.Stdout
	add.Stderr = os.Stderr
	if err := add.Run(); err != nil {
		return fmt.Errorf("auto-add fox-openapi: %w", err)
	}
	return nil
}

func hasOpenAPIRequire(workdir string) (bool, error) {
	path := filepath.Join(workdir, "go.mod")
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	file, err := modfile.Parse(path, data, nil)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", path, err)
	}
	for _, req := range file.Require {
		if req.Mod.Path == "github.com/fox-gonic/fox-openapi" {
			return true, nil
		}
	}
	return false, nil
}
