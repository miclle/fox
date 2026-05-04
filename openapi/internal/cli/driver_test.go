package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteDriverRendersMetadataAndAbsoluteSources(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Workdir: dir,
		Format:  "json",
		Sources: []string{"./internal/server", "./pkg/..."},
		Info: InfoConfig{
			Title:       "Acme",
			Version:     "1.0.0",
			Description: "API description",
		},
		Servers: []ServerConfig{{URL: "https://api.example.com", Description: "prod"}},
		Tags: []TagConfig{{
			Name:        "users",
			Description: "User endpoints",
			ExternalDocs: &ExternalDocsConfig{
				URL:         "https://docs.example.com/users",
				Description: "User docs",
			},
		}},
		SecuritySchemes: map[string]Scheme{
			"BearerAuth": {
				Type:         "http",
				Scheme:       "bearer",
				BearerFormat: "JWT",
			},
		},
	}
	driverDir, err := WriteDriver(cfg, Entry{ImportPath: "example.com/app/internal/server", FuncName: "NewEngine"}, &Hook{ImportPath: "example.com/app/internal/server", FuncName: "ConfigureOpenAPI"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(driverDir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`openapi.Info("Acme", "1.0.0")`,
		`openapi.Server("https://api.example.com")`,
		`spec.Info.Description = "API description"`,
		`spec.Servers[0].Description = "prod"`,
		`spec.Tags = openapi3.Tags`,
		`openapi.SecurityScheme("BearerAuth"`,
		`opts = append(opts, userhook.ConfigureOpenAPI()...)`,
		filepath.ToSlash(filepath.Join(dir, "internal/server")),
		filepath.ToSlash(filepath.Join(dir, "pkg")) + "/...",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("driver missing %q:\n%s", want, text)
		}
	}
}

func TestCleanupDriverRemovesEmptyParent(t *testing.T) {
	dir := t.TempDir()
	driverDir := filepath.Join(dir, ".fox-openapi", "driver")
	if err := os.MkdirAll(driverDir, 0o755); err != nil {
		t.Fatal(err)
	}
	CleanupDriver(driverDir, false)
	if _, err := os.Stat(filepath.Join(dir, ".fox-openapi")); !os.IsNotExist(err) {
		t.Fatalf("expected .fox-openapi to be removed, got %v", err)
	}
}
