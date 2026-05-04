package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestWatchDirsResolvesSources(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "internal/server"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "vendor"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := watchDirs(Config{Workdir: dir, Sources: []string{"./internal/server/..."}})
	want := filepath.Join(dir, "internal/server")
	if len(got) != 1 || got[0] != want {
		t.Fatalf("watchDirs = %#v, want [%q]", got, want)
	}
}

func TestShouldRefreshGoAndConfigFiles(t *testing.T) {
	cases := []struct {
		event fsnotify.Event
		want  bool
	}{
		{fsnotify.Event{Name: "server.go", Op: fsnotify.Write}, true},
		{fsnotify.Event{Name: "fox-openapi.yaml", Op: fsnotify.Write}, true},
		{fsnotify.Event{Name: "README.md", Op: fsnotify.Write}, false},
		{fsnotify.Event{Name: "server.go", Op: fsnotify.Chmod}, false},
	}
	for _, tc := range cases {
		if got := shouldRefresh(tc.event); got != tc.want {
			t.Fatalf("shouldRefresh(%v) = %v, want %v", tc.event, got, tc.want)
		}
	}
}
