package cli

import (
	"strings"
	"testing"
)

func TestResolveEntryValidSignatures(t *testing.T) {
	dir := writeUserModule(t)
	entry, err := ResolveEntry(dir, "example.com/app/internal/server.NewEngine")
	if err != nil {
		t.Fatal(err)
	}
	if entry.ImportPath != "example.com/app/internal/server" || entry.FuncName != "NewEngine" || entry.ReturnsError {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	entry, err = ResolveEntry(dir, "example.com/app/internal/server.NewEngineWithError")
	if err != nil {
		t.Fatal(err)
	}
	if !entry.ReturnsError {
		t.Fatalf("expected ReturnsError: %+v", entry)
	}
}

func TestResolveEntryRejectsBadSignature(t *testing.T) {
	dir := writeUserModule(t)
	_, err := ResolveEntry(dir, "example.com/app/internal/server.BadEntry")
	if err == nil || !strings.Contains(err.Error(), "signature mismatch") {
		t.Fatalf("expected signature mismatch, got %v", err)
	}
}

func TestResolveHook(t *testing.T) {
	dir := writeUserModule(t)
	hook, err := ResolveHook(dir, "example.com/app/internal/server.ConfigureOpenAPI")
	if err != nil {
		t.Fatal(err)
	}
	if hook.ImportPath != "example.com/app/internal/server" || hook.FuncName != "ConfigureOpenAPI" {
		t.Fatalf("unexpected hook: %+v", hook)
	}
	_, err = ResolveHook(dir, "example.com/app/internal/server.badHook")
	if err == nil || !strings.Contains(err.Error(), "not exported") {
		t.Fatalf("expected not exported error, got %v", err)
	}
}
