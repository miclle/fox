package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDriverSeparatesStdoutAndStderr(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/driver\n\ngo 1.25\n")
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "WARN: noisy")
	fmt.Print("spec bytes")
}
`)
	stdout, stderr, err := RunDriver(dir)
	if err != nil {
		t.Fatal(err)
	}
	if string(stdout) != "spec bytes" {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(string(stderr), "WARN: noisy") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestRunDriverExitCodes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/driver\n\ngo 1.25\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\nfunc main() { doesNotCompile }\n")
	_, _, err := RunDriver(dir)
	if err == nil {
		t.Fatal("expected build error")
	}
	var driverErr *DriverError
	if !errors.As(err, &driverErr) || driverErr.ExitCode != ExitBuildFailed {
		t.Fatalf("error = %#v, want build DriverError", err)
	}

	dir = t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/driver\n\ngo 1.25\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\nfunc main() { panic(\"boom\") }\n")
	_, _, err = RunDriver(dir)
	if !errors.As(err, &driverErr) || driverErr.ExitCode != ExitRunFailed {
		t.Fatalf("error = %#v, want run DriverError", err)
	}
}

func TestWriteAtomicAndCheckDrift(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "api", "openapi.yaml")
	if err := WriteAtomic(out, []byte("one\n")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "one\n" {
		t.Fatalf("data = %q", data)
	}
	if err := CheckDrift(out, []byte("one\n")); err != nil {
		t.Fatal(err)
	}
	if err := CheckDrift(out, []byte("two\n")); !errors.Is(err, ErrDrift) {
		t.Fatalf("expected ErrDrift, got %v", err)
	}
}
