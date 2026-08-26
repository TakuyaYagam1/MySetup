package dots

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteHyprLocalConfigFilesSeedsCanonicalDefault(t *testing.T) {
	sourceDir := t.TempDir()
	hyprDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "hyprland.lua"), []byte("-- canonical\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeHyprLocalConfigFiles(sourceDir, hyprDir); err != nil {
		t.Fatal(err)
	}

	wahrweltDir := filepath.Join(hyprDir, "user")
	defaultPath := filepath.Join(wahrweltDir, "default.lua")
	if got := readTestFile(t, defaultPath); got != wahrweltDefaultLua {
		t.Fatalf("unexpected default.lua template:\n%s", got)
	} else {
		for _, module := range []string{"wahrwelt.execs", "wahrwelt.general", "wahrwelt.rules", "wahrwelt.keybinds"} {
			if !strings.Contains(got, `optional_require("`+module+`")`) {
				t.Fatalf("default.lua lost internal Wahrwelt module namespace %q:\n%s", module, got)
			}
		}
		if strings.Contains(got, `optional_require("user.`) {
			t.Fatalf("default.lua exposed the physical user directory as its Lua namespace:\n%s", got)
		}
	}
	info, err := os.Lstat(defaultPath)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o644 {
		t.Fatalf("default.lua must be a regular 0644 file, mode=%s", info.Mode())
	}
	if got := readTestFile(t, filepath.Join(wahrweltDir, "hyprland.lua")); got != "-- canonical\n" {
		t.Fatalf("managed entrypoint was not copied: %q", got)
	}
	if _, err := os.Lstat(filepath.Join(wahrweltDir, "local.lua")); !os.IsNotExist(err) {
		t.Fatalf("installer must not create local.lua, err=%v", err)
	}
}

func TestSeedWahrweltDefaultPreservesExistingRegularAndSymlinks(t *testing.T) {
	for _, kind := range []string{"regular", "symlink", "broken-symlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "default.lua")
			var wantTarget string
			switch kind {
			case "regular":
				if err := os.WriteFile(path, []byte("-- user config\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				target := filepath.Join(dir, "user.lua")
				if err := os.WriteFile(target, []byte("-- linked user config\n"), 0o640); err != nil {
					t.Fatal(err)
				}
				wantTarget = target
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			case "broken-symlink":
				wantTarget = filepath.Join(dir, "missing.lua")
				if err := os.Symlink(wantTarget, path); err != nil {
					t.Fatal(err)
				}
			}

			if err := seedWahrweltDefault(path); err != nil {
				t.Fatal(err)
			}
			if kind == "regular" {
				if got := readTestFile(t, path); got != "-- user config\n" {
					t.Fatalf("regular user config changed: %q", got)
				}
				info, err := os.Lstat(path)
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != 0o600 {
					t.Fatalf("regular user mode changed: %s", info.Mode())
				}
				return
			}
			gotTarget, err := os.Readlink(path)
			if err != nil {
				t.Fatal(err)
			}
			if gotTarget != wantTarget {
				t.Fatalf("symlink changed: got %q want %q", gotTarget, wantTarget)
			}
		})
	}
}

func TestSeedWahrweltDefaultRejectsDirectoryCollision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "default.lua")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	err := seedWahrweltDefault(path)
	if err == nil || !strings.Contains(err.Error(), "non-regular Wahrwelt user config collision") {
		t.Fatalf("expected preserved directory collision, got %v", err)
	}
	info, statErr := os.Lstat(path)
	if statErr != nil || !info.IsDir() {
		t.Fatalf("directory collision was changed: info=%v err=%v", info, statErr)
	}
}

func TestSeedWahrweltDefaultFaultCleanupNeverRemovesFinalReplacement(t *testing.T) {
	injected := errors.New("injected seed failure")
	for _, stage := range []wahrweltDefaultSeedStage{
		wahrweltDefaultSeedWrite,
		wahrweltDefaultSeedChmod,
		wahrweltDefaultSeedClose,
	} {
		t.Run(string(stage), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "default.lua")
			err := seedWahrweltDefaultWithHook(path, func(gotStage wahrweltDefaultSeedStage, _, finalPath string) error {
				if gotStage != stage {
					return nil
				}
				if err := os.WriteFile(finalPath, []byte("-- concurrent replacement\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				return injected
			})
			if !errors.Is(err, injected) {
				t.Fatalf("seed error = %v, want injected failure", err)
			}
			if got := readTestFile(t, path); got != "-- concurrent replacement\n" {
				t.Fatalf("final replacement was removed after %s failure: %q", stage, got)
			}
			info, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o600 {
				t.Fatalf("replacement mode changed after %s failure: %s", stage, info.Mode())
			}
		})
	}
}

func TestSeedWahrweltDefaultPreservesPublicationRaceWinners(t *testing.T) {
	for _, kind := range []string{"regular", "symlink", "broken-symlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "default.lua")
			var wantTarget string
			err := seedWahrweltDefaultWithHook(path, func(stage wahrweltDefaultSeedStage, tempPath, finalPath string) error {
				if stage != wahrweltDefaultSeedPublish {
					return nil
				}
				if got := readTestFile(t, tempPath); got != wahrweltDefaultLua {
					t.Fatalf("publication temp is incomplete: %q", got)
				}
				info, err := os.Stat(tempPath)
				if err != nil {
					t.Fatal(err)
				}
				if !info.Mode().IsRegular() || info.Mode().Perm() != 0o644 {
					t.Fatalf("publication temp must be a complete regular 0644 file: %s", info.Mode())
				}
				switch kind {
				case "regular":
					return os.WriteFile(finalPath, []byte("-- race winner\n"), 0o600)
				case "symlink":
					wantTarget = filepath.Join(dir, "user.lua")
					if err := os.WriteFile(wantTarget, []byte("-- linked winner\n"), 0o640); err != nil {
						return err
					}
					return os.Symlink(wantTarget, finalPath)
				case "broken-symlink":
					wantTarget = filepath.Join(dir, "missing.lua")
					return os.Symlink(wantTarget, finalPath)
				default:
					return errors.New("unknown test winner")
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			if kind == "regular" {
				if got := readTestFile(t, path); got != "-- race winner\n" {
					t.Fatalf("regular race winner changed: %q", got)
				}
				return
			}
			gotTarget, err := os.Readlink(path)
			if err != nil {
				t.Fatal(err)
			}
			if gotTarget != wantTarget {
				t.Fatalf("symlink race winner changed: got %q want %q", gotTarget, wantTarget)
			}
		})
	}
}

func TestSeedWahrweltDefaultUsesAnonymousTempAndPinsParent(t *testing.T) {
	dir := t.TempDir()
	originalDir := filepath.Join(dir, "user")
	outsideDir := filepath.Join(dir, "outside")
	if err := os.MkdirAll(originalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(originalDir, "default.lua")
	movedDir := filepath.Join(dir, "user-moved")

	err := seedWahrweltDefaultWithHook(path, func(stage wahrweltDefaultSeedStage, tempPath, _ string) error {
		if stage != wahrweltDefaultSeedPublish {
			return nil
		}
		if !strings.HasPrefix(tempPath, "/proc/self/fd/") {
			return errors.New("seed exposed a mutable named temp")
		}
		matches, globErr := filepath.Glob(filepath.Join(originalDir, ".default.lua.tmp-*"))
		if globErr != nil {
			return globErr
		}
		if len(matches) != 0 {
			return errors.New("seed exposed a mutable public temp")
		}
		if err := os.Rename(originalDir, movedDir); err != nil {
			return err
		}
		return os.Symlink(outsideDir, originalDir)
	})
	if err == nil || !strings.Contains(err.Error(), "parent directory changed") {
		t.Fatalf("expected pinned-parent collision, got %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(outsideDir, "default.lua")); !os.IsNotExist(statErr) {
		t.Fatalf("seed followed replacement parent: %v", statErr)
	}
	if info, statErr := os.Lstat(originalDir); statErr != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("replacement parent changed: info=%v err=%v", info, statErr)
	}
}

func TestSeedWahrweltDefaultAnonymousTempCannotBeSubstituted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "default.lua")
	unknown := filepath.Join(dir, "unknown.lua")
	if err := os.WriteFile(unknown, []byte("-- unknown owner\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var substitutionErr error
	err := seedWahrweltDefaultWithHook(path, func(stage wahrweltDefaultSeedStage, tempPath, _ string) error {
		if stage == wahrweltDefaultSeedPublish {
			substitutionErr = os.Rename(unknown, tempPath)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if substitutionErr == nil {
		t.Fatal("anonymous seed file was replaceable by pathname")
	}
	if got := readTestFile(t, path); got != wahrweltDefaultLua {
		t.Fatalf("seed published substituted content: %q", got)
	}
	if got := readTestFile(t, unknown); got != "-- unknown owner\n" {
		t.Fatalf("unknown substitution candidate changed: %q", got)
	}
}

func TestWriteHyprLocalConfigFilesReportsDefaultCollisionBeforeManagedUpdate(t *testing.T) {
	sourceDir := t.TempDir()
	hyprDir := t.TempDir()
	wahrweltDir := filepath.Join(hyprDir, "user")
	if err := os.MkdirAll(filepath.Join(wahrweltDir, "default.lua"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "hyprland.lua"), []byte("-- new canonical\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	managedPath := filepath.Join(wahrweltDir, "hyprland.lua")
	if err := os.WriteFile(managedPath, []byte("-- previous canonical\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := writeHyprLocalConfigFiles(sourceDir, hyprDir)
	if err == nil || !strings.Contains(err.Error(), "non-regular Wahrwelt user config collision") {
		t.Fatalf("expected default.lua collision, got %v", err)
	}
	if got := readTestFile(t, managedPath); got != "-- previous canonical\n" {
		t.Fatalf("managed entrypoint changed before collision report: %q", got)
	}
}

func TestWriteHyprLocalConfigFilesRejectsUserDirectorySymlink(t *testing.T) {
	sourceDir := t.TempDir()
	hyprDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "hyprland.lua"), []byte("-- canonical\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	wahrweltDir := filepath.Join(hyprDir, "user")
	if err := os.Symlink(target, wahrweltDir); err != nil {
		t.Fatal(err)
	}

	err := writeHyprLocalConfigFiles(sourceDir, hyprDir)
	if err == nil || !strings.Contains(err.Error(), "unsupported Hypr user config path") {
		t.Fatalf("expected user directory ownership collision, got %v", err)
	}
	got, readErr := os.Readlink(wahrweltDir)
	if readErr != nil || got != target {
		t.Fatalf("user directory symlink changed: target=%q err=%v", got, readErr)
	}
}
