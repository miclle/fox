package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func repoPaths(t *testing.T) (foxRoot, openapiRoot string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	openapiRoot = filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	foxRoot, err := filepath.Abs(filepath.Join(openapiRoot, ".."))
	if err != nil {
		t.Fatal(err)
	}
	openapiRoot, err = filepath.Abs(openapiRoot)
	if err != nil {
		t.Fatal(err)
	}
	return foxRoot, openapiRoot
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeUserModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	foxRoot, openapiRoot := repoPaths(t)
	writeFile(t, filepath.Join(dir, "go.mod"), `module example.com/app

go 1.25

require (
	github.com/fox-gonic/fox v0.0.0
	github.com/fox-gonic/fox-openapi v0.0.0
	github.com/getkin/kin-openapi v0.133.0
)

replace github.com/fox-gonic/fox => `+filepath.ToSlash(foxRoot)+`
replace github.com/fox-gonic/fox-openapi => `+filepath.ToSlash(openapiRoot)+`
`)
	writeFile(t, filepath.Join(dir, "internal/server/server.go"), `package server

import (
	"errors"
	"reflect"

	"github.com/fox-gonic/fox"
	openapi "github.com/fox-gonic/fox-openapi"
	"github.com/getkin/kin-openapi/openapi3"
)

type GetUserRequest struct {
	ID string `+"`uri:\"id\" binding:\"required\"`"+`
}

type User struct {
	ID string `+"`json:\"id\"`"+`
	Name string `+"`json:\"name\"`"+`
}

// GetUser fetches a user by id.
func GetUser(ctx *fox.Context, req GetUserRequest) (User, error) {
	return User{ID: req.ID, Name: "Ada"}, nil
}

func NewEngine() *fox.Engine {
	e := fox.New()
	e.GET("/users/:id", GetUser)
	return e
}

func NewEngineWithError() (*fox.Engine, error) {
	return NewEngine(), nil
}

func BrokenEntry() (*fox.Engine, error) {
	return nil, errors.New("boom")
}

func ConfigureOpenAPI() []openapi.Option {
	return []openapi.Option{
		openapi.RegisterFormatter(reflect.TypeOf(User{}), openapi3.NewObjectSchema()),
		openapi.Operation("GET", "/users/:id", openapi.Tags("users")),
	}
}

func badHook() []openapi.Option { return nil }

func BadEntry(arg string) *fox.Engine { return nil }
`)
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s", err, out)
	}
	return dir
}
