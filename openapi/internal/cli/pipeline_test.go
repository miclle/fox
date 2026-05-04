package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPipelineGeneratesSpecFromUserModule(t *testing.T) {
	dir := writeUserModule(t)
	cfg := Config{
		Entry:        "example.com/app/internal/server.NewEngine",
		MetadataHook: "example.com/app/internal/server.ConfigureOpenAPI",
		Out:          "api/openapi.yaml",
		Format:       "yaml",
		Sources:      []string{"./internal/server"},
		Info: InfoConfig{
			Title:       "Example API",
			Version:     "1.0.0",
			Description: "Example description",
		},
		Servers: []ServerConfig{{URL: "https://api.example.com", Description: "prod"}},
		Tags:    []TagConfig{{Name: "users", Description: "User endpoints"}},
		Workdir: dir,
	}
	data, warnings, err := RunPipeline(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v", warnings)
	}
	for _, want := range []string{
		"title: Example API",
		"description: Example description",
		"url: https://api.example.com",
		"description: prod",
		"/users/{id}:",
		"tags:",
		"- users",
		"User endpoints",
	} {
		if !bytes.Contains(data, []byte(want)) {
			t.Fatalf("generated spec missing %q:\n%s", want, data)
		}
	}
	if strings.Contains(string(data), "WARN:") {
		t.Fatalf("warnings leaked into stdout:\n%s", data)
	}
}

func TestEnsureOpenAPIModuleRequiresGoModEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := EnsureOpenAPIModule(Config{Workdir: dir})
	if err == nil || !strings.Contains(err.Error(), "go get github.com/fox-gonic/fox-openapi") {
		t.Fatalf("expected go get hint, got %v", err)
	}
}
