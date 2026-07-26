package dots

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
)

func TestMigrateLegacyUserPathsMovesCanonicalTrees(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	for path, content := range map[string]string{
		filepath.Join(home, ".config", "mysetup", "boot-theme", "README.txt"): "theme\n",
		filepath.Join(home, ".config", "hypr", "mysetup", "keybinds.lua"):     "binds\n",
		filepath.Join(home, ".local", "state", "mysetup", "active-shell"):     "caelestia\n",
		filepath.Join(home, ".cache", "mysetup", "staging", "sentinel"):       "cache\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	oldLink := filepath.Join(home, ".config", "hypr", "lib", "mysetup.lua")
	if err := os.MkdirAll(filepath.Dir(oldLink), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/nix/store/legacy", oldLink); err != nil {
		t.Fatal(err)
	}

	if err := migrateLegacyUserPaths(context.Background(), run.New(false), home); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		filepath.Join(home, ".config", "wahrwelt", "boot-theme", "README.txt"),
		filepath.Join(home, ".config", "hypr", "wahrwelt", "keybinds.lua"),
		filepath.Join(home, ".local", "state", "wahrwelt", "active-shell"),
		filepath.Join(home, ".cache", "wahrwelt", "staging", "sentinel"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected migrated path %s: %v", path, err)
		}
	}
	if _, err := os.Lstat(oldLink); !os.IsNotExist(err) {
		t.Fatalf("expected legacy Home Manager symlink removed, got %v", err)
	}
}

func TestMigrateLegacyUserPathsRejectsDivergentTrees(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	for _, path := range []string{
		filepath.Join(home, ".config", "mysetup"),
		filepath.Join(home, ".config", "wahrwelt"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	err := migrateLegacyUserPaths(context.Background(), run.New(false), home)
	if err == nil || !strings.Contains(err.Error(), "migration conflict") {
		t.Fatalf("expected an explicit migration conflict, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "mysetup")); err != nil {
		t.Fatalf("legacy tree must remain untouched after preflight conflict: %v", err)
	}
}
