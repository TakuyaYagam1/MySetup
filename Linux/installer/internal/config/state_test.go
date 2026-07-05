package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestLoadExistingMigratesStateWithoutSchema(t *testing.T) {
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

	got, err := LoadExisting(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != SchemaVersion {
		t.Fatalf("expected migrated schema %d, got %d", SchemaVersion, got.SchemaVersion)
	}
	if got.User.Username != "alice" {
		t.Fatalf("expected existing username to be preserved, got %q", got.User.Username)
	}
	if got.Host.StateVersion != Default().Host.StateVersion {
		t.Fatalf("expected default stateVersion to be filled, got %q", got.Host.StateVersion)
	}
	if got.Source.Channel != SourceChannelStable {
		t.Fatalf("expected migrated source channel %q, got %q", SourceChannelStable, got.Source.Channel)
	}
}

func TestLoadExistingRejectsFutureSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"schemaVersion":999}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadExisting(path)
	if err == nil {
		t.Fatal("expected schema error")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported schema error, got %v", err)
	}
}

func TestDefaultConsoleKeyMapIsUS(t *testing.T) {
	state := Default()
	if state.Locale.ConsoleKeyMap != "us" {
		t.Fatalf("expected safe TTY console keymap default us, got %q", state.Locale.ConsoleKeyMap)
	}
	if state.Locale.KeyboardLayouts != "us,ru" {
		t.Fatalf("expected graphical Hypr layouts to keep ru support, got %q", state.Locale.KeyboardLayouts)
	}
}

func TestDefaultSourceChannelIsStable(t *testing.T) {
	state := Default()
	if state.Source.Channel != SourceChannelStable {
		t.Fatalf("expected default source channel %q, got %q", SourceChannelStable, state.Source.Channel)
	}
}

func TestDefaultFeatureAndDotsToggles(t *testing.T) {
	state := Default()
	if state.Features.SecureBoot {
		t.Fatal("secure boot must be disabled by default")
	}
	if state.Zapret.Enable {
		t.Fatal("zapret must be disabled by default")
	}
	if !state.Dots.Sine {
		t.Fatal("sine profile install should be enabled by default")
	}
	if !state.Dots.Neovim {
		t.Fatal("neovim sync should be enabled by default")
	}
	if state.Noctalia.Version != NoctaliaVersionV5 {
		t.Fatalf("expected noctalia version default %q, got %q", NoctaliaVersionV5, state.Noctalia.Version)
	}
}

func TestValidateRejectsUnsafeHomeDirectory(t *testing.T) {
	for name, home := range map[string]string{
		"traversal":       "/home/../root",
		"relative":        "home/takuya",
		"wrong username":  "/home/other",
		"unclean subpath": "/home/takuya/..",
	} {
		t.Run(name, func(t *testing.T) {
			state := Default()
			state.User.Username = "takuya"
			state.User.HomeDirectory = home

			err := Validate(state)
			if err == nil {
				t.Fatalf("expected unsafe home directory %q to fail validation", home)
			}
		})
	}
}
