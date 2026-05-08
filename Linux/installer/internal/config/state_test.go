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
		t.Fatalf("expected migrated default shell, got %q", state.Shell.Profile)
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

func TestDefaultFeatureAndDotsToggles(t *testing.T) {
	state := Default()
	if state.Features.SecureBoot {
		t.Fatal("secure boot must be disabled by default")
	}
	if state.Features.RussiaMode {
		t.Fatal("russia mode must be disabled by default")
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
}

func TestMigrateOldDefaultRussianConsoleKeyMapToUS(t *testing.T) {
	state := Default()
	state.SchemaVersion = 1
	state.Locale.ConsoleKeyMap = "ruwin_alt_sh-UTF-8"

	got := Migrate(state)
	if got.Locale.ConsoleKeyMap != "us" {
		t.Fatalf("expected old default Russian TTY keymap to migrate to us, got %q", got.Locale.ConsoleKeyMap)
	}
}

func TestMigrateEnablesNewDefaultDots(t *testing.T) {
	state := Default()
	state.SchemaVersion = 2
	state.Dots.Sine = false
	state.Dots.Neovim = false

	got := Migrate(state)
	if !got.Dots.Sine {
		t.Fatal("expected old state to migrate sine default to true")
	}
	if !got.Dots.Neovim {
		t.Fatal("expected old state to migrate neovim default to true")
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
