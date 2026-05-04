package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

func watchAndRefresh(ctx context.Context, cfg Config, state *servedSpec) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		state.setError(fmt.Sprintf("watcher: %v", err))
		return
	}
	defer watcher.Close()

	for _, dir := range watchDirs(cfg) {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			name := d.Name()
			if name == "vendor" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			_ = watcher.Add(path)
			return nil
		})
	}

	var debounce <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if shouldRefresh(event) {
				debounce = time.After(300 * time.Millisecond)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			state.setError(fmt.Sprintf("watcher: %v", err))
		case <-debounce:
			debounce = nil
			_ = refreshSpec(cfg, state)
		}
	}
}

func watchDirs(cfg Config) []string {
	seen := map[string]struct{}{}
	dirs := make([]string, 0, len(cfg.Sources)+1)
	add := func(path string) {
		if path == "" {
			return
		}
		if strings.HasSuffix(path, "/...") {
			path = strings.TrimSuffix(path, "/...")
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(cfg.Workdir, path)
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return
		}
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			return
		}
		if _, ok := seen[abs]; ok {
			return
		}
		seen[abs] = struct{}{}
		dirs = append(dirs, abs)
	}
	for _, source := range cfg.Sources {
		add(source)
	}
	if len(dirs) == 0 {
		add(cfg.Workdir)
	}
	return dirs
}

func shouldRefresh(event fsnotify.Event) bool {
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
		return false
	}
	return strings.HasSuffix(event.Name, ".go") || strings.HasSuffix(filepath.Base(event.Name), "fox-openapi.yaml")
}
