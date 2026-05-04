package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrDrift = errors.New("openapi output is out of date")

func WriteAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".openapi-*")
	if err != nil {
		return fmt.Errorf("create temp output: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp output: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp output: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp output: %w", err)
	}
	return nil
}

func CheckDrift(path string, data []byte) error {
	current, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrDrift
		}
		return fmt.Errorf("read output: %w", err)
	}
	if !bytes.Equal(current, data) {
		return ErrDrift
	}
	return nil
}
