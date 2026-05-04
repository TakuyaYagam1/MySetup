package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyMissingStateFailsClosed(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--state", filepath.Join(t.TempDir(), "missing.json"),
		"apply",
		"--dry-run",
		"--no-switch",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected apply to fail when state is missing")
	}
	if !strings.Contains(err.Error(), "state file does not exist") {
		t.Fatalf("expected missing state error, got %v", err)
	}
}

func TestPrintStateMissingStateFailsClosed(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--state", filepath.Join(t.TempDir(), "missing.json"),
		"print-state",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected print-state to fail when state is missing")
	}
	if !strings.Contains(err.Error(), "state file does not exist") {
		t.Fatalf("expected missing state error, got %v", err)
	}
}

func TestLoadDoctorStateFallsBackOnInvalidState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{ nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	state, err := loadDoctorState(path)
	if err != nil {
		t.Fatal(err)
	}
	if state.Shell.Profile != "caelestia" {
		t.Fatalf("expected default doctor fallback state, got %#v", state.Shell)
	}
}

func TestMakeCheckCleansBuildArtifact(t *testing.T) {
	data, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "$(MAKE) clean") {
		t.Fatalf("check target should run clean after build\n%s", text)
	}
	if strings.Contains(text, "check: fmt-check lint vet test build nix-build") {
		t.Fatalf("check target should not use dependency form that leaves bin/mysetup behind\n%s", text)
	}
}
