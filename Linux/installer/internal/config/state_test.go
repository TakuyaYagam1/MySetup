package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadExistingMissingFileErrors(t *testing.T) {
	_, err := LoadExisting(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("expected missing state error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing state to wrap os.ErrNotExist, got %v", err)
	}
}

func TestLoadExistingInvalidJSONErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{ nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadExisting(path); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func TestLoadExistingMigratesValidState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{
  "user": {
    "username": "alice"
  },
  "git": {
    "email": "alice@example.com"
  }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	state, err := LoadExisting(path)
	if err != nil {
		t.Fatal(err)
	}
	if state.User.HomeDirectory != "/home/alice" {
		t.Fatalf("expected migrated home directory, got %q", state.User.HomeDirectory)
	}
	if state.Shell.Profile != "caelestia" {
		t.Fatalf("expected migrated default shell profile, got %q", state.Shell.Profile)
	}
}
