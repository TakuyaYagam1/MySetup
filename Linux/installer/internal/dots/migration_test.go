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

func TestMigrateLegacyUserPathsMergesCacheWithCanonicalPriority(t *testing.T) {
	home := t.TempDir()
	cacheHome := filepath.Join(home, ".cache")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", cacheHome)

	oldCache := filepath.Join(cacheHome, "mysetup")
	newCache := filepath.Join(cacheHome, "wahrwelt")
	for path, content := range map[string]string{
		filepath.Join(oldCache, "legacy-only"): "legacy\n",
		filepath.Join(oldCache, "shared"):      "legacy\n",
		filepath.Join(newCache, "new-only"):    "canonical\n",
		filepath.Join(newCache, "shared"):      "canonical\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := migrateLegacyUserPaths(context.Background(), run.New(false), home); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(oldCache); !os.IsNotExist(err) {
		t.Fatalf("legacy cache must be removed after merge, got %v", err)
	}
	for name, want := range map[string]string{
		"legacy-only": "legacy\n",
		"new-only":    "canonical\n",
		"shared":      "canonical\n",
	} {
		data, err := os.ReadFile(filepath.Join(newCache, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("unexpected merged cache content for %s: got %q, want %q", name, data, want)
		}
	}
}

func TestHomeManagerMigrationMergesOnlyCacheConflicts(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/home/programs/wahrwelt-migration.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`move_tree "${configHome}/mysetup" "${configHome}/wahrwelt"`,
		`move_tree "${configHome}/hypr/mysetup" "${configHome}/hypr/wahrwelt"`,
		`move_tree "${stateHome}/mysetup" "${stateHome}/wahrwelt"`,
		`merge_cache "${cacheHome}/mysetup" "${cacheHome}/wahrwelt"`,
		`${pkgs.rsync}/bin/rsync -a --ignore-existing "$old/" "$new/"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Home Manager migration contract is missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, `move_tree "${cacheHome}/mysetup" "${cacheHome}/wahrwelt"`) {
		t.Fatalf("cache migration must merge disposable cache collisions instead of failing closed\n%s", text)
	}
}
