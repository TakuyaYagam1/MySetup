package dots

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
)

const (
	historicalHyprLegacyLinkTarget = "/nix/store/00000000000000000000000000000000-home-manager-files/.config/hypr/lib/mysetup.lua"
	historicalSelectorLinkTarget   = "/nix/store/11111111111111111111111111111111-home-manager-files/.config/quickshell/mysetup-shell-selector"
)

func TestMoveLegacyPathRejectsDestinationCreatedAfterPreflightWithoutNesting(t *testing.T) {
	parent := t.TempDir()
	legacy := filepath.Join(parent, "mysetup")
	canonical := filepath.Join(parent, "wahrwelt")
	if err := os.Mkdir(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "legacy"), []byte("legacy owner\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	snapshot, err := snapshotLegacyPath(legacy)
	if err != nil {
		t.Fatal(err)
	}
	move, err := moveLegacyPathWithSnapshotHook(
		context.Background(), run.New(false), legacy, canonical, true, snapshot,
		func(parent *migrationPinnedDirectory) error {
			if err := os.Mkdir(parent.child("wahrwelt"), 0o700); err != nil {
				return err
			}
			return os.WriteFile(parent.child(filepath.Join("wahrwelt", "winner")), []byte("canonical owner\n"), 0o600)
		},
	)
	if move != nil {
		defer move.close()
	}
	if err == nil || !strings.Contains(err.Error(), "appeared during migration") {
		t.Fatalf("moveLegacyPath() error = %v, want atomic destination collision", err)
	}
	if got := readTestFile(t, filepath.Join(legacy, "legacy")); got != "legacy owner\n" {
		t.Fatalf("legacy source changed after destination race: %q", got)
	}
	if got := readTestFile(t, filepath.Join(canonical, "winner")); got != "canonical owner\n" {
		t.Fatalf("canonical winner changed after destination race: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(canonical, "mysetup")); !os.IsNotExist(statErr) {
		t.Fatalf("legacy source was nested into canonical target: %v", statErr)
	}
}

func TestMoveLegacyPathRetainsExactSourceAfterPreRenameReplacement(t *testing.T) {
	parent := t.TempDir()
	legacy := filepath.Join(parent, "mysetup")
	canonical := filepath.Join(parent, "wahrwelt")
	saved := filepath.Join(parent, "mysetup-before-race")
	if err := os.Mkdir(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "legacy"), []byte("legacy owner\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := snapshotLegacyPath(legacy)
	if err != nil {
		t.Fatal(err)
	}

	move, err := moveLegacyPathWithSnapshotHook(
		context.Background(), run.New(false), legacy, canonical, true, snapshot,
		func(pinned *migrationPinnedDirectory) error {
			if err := os.Rename(pinned.child("mysetup"), pinned.child("mysetup-before-race")); err != nil {
				return err
			}
			if err := os.Mkdir(pinned.child("mysetup"), 0o700); err != nil {
				return err
			}
			return os.WriteFile(pinned.child(filepath.Join("mysetup", "winner")), []byte("replacement owner\n"), 0o600)
		},
	)
	if move != nil {
		defer move.close()
	}
	if err == nil || !strings.Contains(err.Error(), "recovery retained at "+saved) {
		t.Fatalf("source replacement was accepted: move=%v err=%v", move, err)
	}
	if got := readTestFile(t, filepath.Join(canonical, "winner")); got != "replacement owner\n" {
		t.Fatalf("replacement source was not restored: %q", got)
	}
	if got := readTestFile(t, filepath.Join(saved, "legacy")); got != "legacy owner\n" {
		t.Fatalf("expected legacy source changed: %q", got)
	}
	if _, statErr := os.Lstat(legacy); !os.IsNotExist(statErr) {
		t.Fatalf("legacy basename unexpectedly reclaimed after replacement: %v", statErr)
	}
}

func TestMoveLegacyPathReportsExactRecoveryAfterPostRenameTargetReplacement(t *testing.T) {
	parent := t.TempDir()
	legacy := filepath.Join(parent, "mysetup")
	canonical := filepath.Join(parent, "wahrwelt")
	saved := filepath.Join(parent, "wahrwelt-moved-after-rename")
	if err := os.Mkdir(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "legacy"), []byte("legacy owner\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := snapshotLegacyPath(legacy)
	if err != nil {
		t.Fatal(err)
	}
	move, err := moveLegacyPathWithPinnedParentHooks(
		legacy,
		canonical,
		true,
		snapshot,
		nil,
		func(_ *legacyPathMove) error {
			if err := os.Rename(canonical, saved); err != nil {
				return err
			}
			if err := os.Mkdir(canonical, 0o700); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(canonical, "winner"), []byte("canonical replacement\n"), 0o600)
		},
	)
	if move != nil {
		defer move.close()
	}
	if err == nil || !strings.Contains(err.Error(), "recovery retained at "+saved) {
		t.Fatalf("post-rename target replacement error = %v, want exact recovery %s", err, saved)
	}
	if got := readTestFile(t, filepath.Join(saved, "legacy")); got != "legacy owner\n" {
		t.Fatalf("exact migrated recovery = %q", got)
	}
	if got := readTestFile(t, filepath.Join(canonical, "winner")); got != "canonical replacement\n" {
		t.Fatalf("canonical replacement = %q", got)
	}
	if _, statErr := os.Lstat(legacy); !os.IsNotExist(statErr) {
		t.Fatalf("legacy basename unexpectedly recreated: %v", statErr)
	}
}

func TestPinOrdinaryDirectoryRejectsFIFOWithoutBlocking(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "fifo")
	if err := unix.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := pinOrdinaryDirectory(fifo)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO was accepted as a pinned ordinary directory")
		}
	case <-time.After(time.Second):
		t.Fatal("pinning a FIFO blocked instead of failing closed")
	}
}

func TestPinDirectoryBeneathRejectsIntermediateSymlinkSwap(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "hypr", "lib")
	attacker := filepath.Join(root, "attacker", "lib")
	for _, path := range []string{original, attacker} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	pinnedRoot, err := pinOrdinaryDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	defer pinnedRoot.close()
	if err := os.Rename(filepath.Join(root, "hypr"), filepath.Join(root, "hypr-original")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "attacker"), filepath.Join(root, "hypr")); err != nil {
		t.Fatal(err)
	}

	pinned, err := pinDirectoryBeneath(pinnedRoot, filepath.Join("hypr", "lib"))
	if pinned != nil {
		pinned.close()
		t.Fatal("intermediate symlink swap was accepted beneath pinned root")
	}
	if err == nil {
		t.Fatal("intermediate symlink swap did not fail closed")
	}
}

func TestMigrateLegacyUserPathsRollsBackFirstMoveAfterLateSecondMoveCollision(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	stateHome := filepath.Join(home, ".local", "state")
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	configLegacy := filepath.Join(configHome, "mysetup")
	stateLegacy := filepath.Join(stateHome, "mysetup")
	stateCanonical := filepath.Join(stateHome, "wahrwelt")
	for path, content := range map[string]string{
		filepath.Join(configLegacy, "config"): "legacy config\n",
		filepath.Join(stateLegacy, "state"):   "legacy state\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	err := migrateLegacyUserPathsWithHooks(context.Background(), run.New(false), home, legacyUserMigrationHooks{
		namespace: func(index int, parent *migrationPinnedDirectory) error {
			if index != 1 {
				return nil
			}
			if err := os.Mkdir(parent.child("wahrwelt"), 0o700); err != nil {
				return err
			}
			return os.WriteFile(parent.child(filepath.Join("wahrwelt", "winner")), []byte("state winner\n"), 0o600)
		},
	})
	if err == nil || !strings.Contains(err.Error(), "appeared during migration") {
		t.Fatalf("migration error = %v, want late second-step collision", err)
	}
	if got := readTestFile(t, filepath.Join(configLegacy, "config")); got != "legacy config\n" {
		t.Fatalf("first move was not rolled back: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(configHome, "wahrwelt")); !os.IsNotExist(statErr) {
		t.Fatalf("rolled-back canonical config remains: %v", statErr)
	}
	if got := readTestFile(t, filepath.Join(stateLegacy, "state")); got != "legacy state\n" {
		t.Fatalf("second-step legacy source changed: %q", got)
	}
	if got := readTestFile(t, filepath.Join(stateCanonical, "winner")); got != "state winner\n" {
		t.Fatalf("concurrent second-step winner changed: %q", got)
	}
}

func TestMigrateLegacyUserPathsLeavesRecoveryWhenRollbackSourceWinnerAppears(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	stateHome := filepath.Join(home, ".local", "state")
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	configLegacy := filepath.Join(configHome, "mysetup")
	configCanonical := filepath.Join(configHome, "wahrwelt")
	stateLegacy := filepath.Join(stateHome, "mysetup")
	stateCanonical := filepath.Join(stateHome, "wahrwelt")
	for path, content := range map[string]string{
		filepath.Join(configLegacy, "original"): "original config\n",
		filepath.Join(stateLegacy, "state"):     "legacy state\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	err := migrateLegacyUserPathsWithHooks(context.Background(), run.New(false), home, legacyUserMigrationHooks{
		namespace: func(index int, parent *migrationPinnedDirectory) error {
			if index != 1 {
				return nil
			}
			if err := os.Mkdir(parent.child("wahrwelt"), 0o700); err != nil {
				return err
			}
			if err := os.WriteFile(parent.child(filepath.Join("wahrwelt", "winner")), []byte("state winner\n"), 0o600); err != nil {
				return err
			}
			if err := os.Mkdir(configLegacy, 0o700); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(configLegacy, "winner"), []byte("config winner\n"), 0o600)
		},
	})
	if err == nil || !strings.Contains(err.Error(), "rollback incomplete") || !strings.Contains(err.Error(), configCanonical) {
		t.Fatalf("migration error = %v, want explicit retained recovery path", err)
	}
	if got := readTestFile(t, filepath.Join(configLegacy, "winner")); got != "config winner\n" {
		t.Fatalf("concurrent rollback-source winner changed: %q", got)
	}
	if got := readTestFile(t, filepath.Join(configCanonical, "original")); got != "original config\n" {
		t.Fatalf("original config recovery was not retained: %q", got)
	}
	if got := readTestFile(t, filepath.Join(stateCanonical, "winner")); got != "state winner\n" {
		t.Fatalf("concurrent second-step winner changed: %q", got)
	}
}

func TestMigrateLegacyUserPathsRollsBackDirectoriesCacheAndLinkAfterLateHyprFailure(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	stateHome := filepath.Join(home, ".local", "state")
	cacheHome := filepath.Join(home, ".cache")
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	legacyLink := filepath.Join(configHome, "hypr", "lib", "mysetup.lua")
	for path, content := range map[string]string{
		filepath.Join(configHome, "mysetup", "config"):               "legacy config\n",
		filepath.Join(stateHome, "mysetup", "state"):                 "legacy state\n",
		filepath.Join(cacheHome, "mysetup", "cache"):                 "legacy cache\n",
		filepath.Join(configHome, "hypr", "wahrwelt", "default.lua"): "-- user config\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(legacyLink), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(historicalHyprLegacyLinkTarget, legacyLink); err != nil {
		t.Fatal(err)
	}

	err := migrateLegacyUserPathsWithHyprMigrationHook(context.Background(), run.New(false), home, func(stage hyprUserMigrationCommitStage, migration hyprUserMigration) error {
		if stage != hyprUserMigrationBeforeRename {
			t.Fatalf("unexpected migration stage %q", stage)
		}
		if err := os.MkdirAll(migration.target, 0o700); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(migration.target, "winner"), []byte("hypr winner\n"), 0o600)
	})
	if err == nil || !strings.Contains(err.Error(), "appeared during migration") {
		t.Fatalf("migration error = %v, want late Hypr collision", err)
	}
	for path, want := range map[string]string{
		filepath.Join(configHome, "mysetup", "config"):               "legacy config\n",
		filepath.Join(stateHome, "mysetup", "state"):                 "legacy state\n",
		filepath.Join(cacheHome, "mysetup", "cache"):                 "legacy cache\n",
		filepath.Join(configHome, "hypr", "wahrwelt", "default.lua"): "-- user config\n",
		filepath.Join(configHome, "hypr", "user", "winner"):          "hypr winner\n",
	} {
		if got := readTestFile(t, path); got != want {
			t.Fatalf("path %s changed after rollback: %q", path, got)
		}
	}
	for _, path := range []string{
		filepath.Join(configHome, "wahrwelt"),
		filepath.Join(stateHome, "wahrwelt"),
		filepath.Join(cacheHome, "wahrwelt"),
	} {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("canonical path remains after rollback: %s: %v", path, statErr)
		}
	}
	if target, readErr := os.Readlink(legacyLink); readErr != nil || target != historicalHyprLegacyLinkTarget {
		t.Fatalf("legacy link was not restored: target=%q err=%v", target, readErr)
	}
}

func TestMigrateLegacyUserPathsPreservesLinkReplacementAfterPreflight(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	legacyConfig := filepath.Join(configHome, "mysetup")
	legacyLink := filepath.Join(configHome, "hypr", "lib", "mysetup.lua")
	if err := os.MkdirAll(legacyConfig, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyConfig, "config"), []byte("legacy config\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyLink), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(historicalHyprLegacyLinkTarget, legacyLink); err != nil {
		t.Fatal(err)
	}

	err := migrateLegacyUserPathsWithHooks(context.Background(), run.New(false), home, legacyUserMigrationHooks{
		link: func(_ int, _ *legacyLinkRecovery, sourceParent *migrationPinnedDirectory) error {
			pinnedLink := sourceParent.child(filepath.Base(legacyLink))
			if err := os.Remove(pinnedLink); err != nil {
				return err
			}
			return os.WriteFile(pinnedLink, []byte("concurrent owner\n"), 0o600)
		},
	})
	if err == nil || !strings.Contains(err.Error(), "changed after preflight") {
		t.Fatalf("migration error = %v, want late link replacement collision", err)
	}
	if got := readTestFile(t, legacyLink); got != "concurrent owner\n" {
		t.Fatalf("concurrent link-path owner changed: %q", got)
	}
	if info, statErr := os.Lstat(legacyLink); statErr != nil || !info.Mode().IsRegular() {
		t.Fatalf("concurrent link-path owner is not the preserved regular file: info=%v err=%v", info, statErr)
	}
	if got := readTestFile(t, filepath.Join(legacyConfig, "config")); got != "legacy config\n" {
		t.Fatalf("earlier config move was not rolled back: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(configHome, "wahrwelt")); !os.IsNotExist(statErr) {
		t.Fatalf("canonical config remains after link replacement collision: %v", statErr)
	}
	matches, globErr := filepath.Glob(filepath.Join(configHome, ".wahrwelt-migration-recovery-links-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 1 || !strings.Contains(err.Error(), matches[0]) {
		t.Fatalf("link replacement collision recovery mismatch: paths=%v err=%v", matches, err)
	}
}

func TestMoveLegacyPathPreservesSourceAppearingAfterAbsentPreflight(t *testing.T) {
	parent := t.TempDir()
	legacy := filepath.Join(parent, "mysetup")
	canonical := filepath.Join(parent, "wahrwelt")
	snapshot, err := snapshotLegacyPath(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.exists {
		t.Fatal("legacy source unexpectedly exists in absent preflight snapshot")
	}
	if err := os.Mkdir(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "winner"), []byte("concurrent owner\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	move, err := moveLegacyPathWithSnapshot(context.Background(), run.New(false), legacy, canonical, true, snapshot)
	if err == nil || !strings.Contains(err.Error(), "appeared after preflight") {
		t.Fatalf("move error = %v, want concurrent source collision", err)
	}
	if move != nil {
		t.Fatalf("concurrent source unexpectedly produced a committed move: %#v", move)
	}
	if got := readTestFile(t, filepath.Join(legacy, "winner")); got != "concurrent owner\n" {
		t.Fatalf("concurrent source changed: %q", got)
	}
	if _, statErr := os.Lstat(canonical); !os.IsNotExist(statErr) {
		t.Fatalf("canonical target appeared after concurrent source collision: %v", statErr)
	}
}

func TestMoveLegacyPathRollsBackThroughPinnedParentAfterCommitParentReplacement(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "config")
	legacy := filepath.Join(parent, "mysetup")
	canonical := filepath.Join(parent, "wahrwelt")
	originalParent := filepath.Join(root, "config-before-race")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "legacy"), []byte("legacy owner\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := snapshotLegacyPath(legacy)
	if err != nil {
		t.Fatal(err)
	}

	move, err := moveLegacyPathWithSnapshotHook(
		context.Background(), run.New(false), legacy, canonical, true, snapshot,
		func(_ *migrationPinnedDirectory) error {
			if err := os.Rename(parent, originalParent); err != nil {
				return err
			}
			if err := os.Mkdir(parent, 0o700); err != nil {
				return err
			}
			if err := os.Mkdir(filepath.Join(parent, "mysetup"), 0o700); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(parent, "mysetup", "winner"), []byte("concurrent owner\n"), 0o600)
		},
	)
	if move != nil {
		move.close()
	}
	if err == nil || !strings.Contains(err.Error(), "pinned directory path identity changed") {
		t.Fatalf("move error = %v, want visible parent replacement collision", err)
	}
	if got := readTestFile(t, filepath.Join(originalParent, "mysetup", "legacy")); got != "legacy owner\n" {
		t.Fatalf("original source was not restored through pinned parent: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(originalParent, "wahrwelt")); !os.IsNotExist(statErr) {
		t.Fatalf("original target remains after pinned rollback: %v", statErr)
	}
	if got := readTestFile(t, filepath.Join(parent, "mysetup", "winner")); got != "concurrent owner\n" {
		t.Fatalf("visible concurrent source changed: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(parent, "wahrwelt")); !os.IsNotExist(statErr) {
		t.Fatalf("move escaped into replacement parent: %v", statErr)
	}
}

func TestRollbackLegacyPathMoveUsesRetainedParentAndPreservesVisibleWinners(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "config")
	legacy := filepath.Join(parent, "mysetup")
	canonical := filepath.Join(parent, "wahrwelt")
	originalParent := filepath.Join(root, "config-before-race")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "legacy"), []byte("legacy owner\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := snapshotLegacyPath(legacy)
	if err != nil {
		t.Fatal(err)
	}
	move, err := moveLegacyPathWithSnapshot(context.Background(), run.New(false), legacy, canonical, true, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	defer move.close()
	if err := os.Rename(parent, originalParent); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(parent, "mysetup", "winner"):  "source winner\n",
		filepath.Join(parent, "wahrwelt", "winner"): "target winner\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := rollbackLegacyPathMove(move); err != nil {
		t.Fatalf("rollback through retained parent: %v", err)
	}
	if got := readTestFile(t, filepath.Join(originalParent, "mysetup", "legacy")); got != "legacy owner\n" {
		t.Fatalf("original move was not rolled back: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(originalParent, "wahrwelt")); !os.IsNotExist(statErr) {
		t.Fatalf("original target remains after rollback: %v", statErr)
	}
	for path, want := range map[string]string{
		filepath.Join(parent, "mysetup", "winner"):  "source winner\n",
		filepath.Join(parent, "wahrwelt", "winner"): "target winner\n",
	} {
		if got := readTestFile(t, path); got != want {
			t.Fatalf("visible concurrent winner changed at %s: %q", path, got)
		}
	}
}

type configHomeSwapOnCommandRunner struct {
	commands int
}

func (r *configHomeSwapOnCommandRunner) Command(context.Context, string, ...string) error {
	r.commands++
	return errors.New("unexpected path-based command after pinned recovery creation")
}

func (*configHomeSwapOnCommandRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (*configHomeSwapOnCommandRunner) IsDryRun() bool { return false }

func TestQuarantineLegacyLinksDoesNotRunPathBasedCommandAfterPinningRecovery(t *testing.T) {
	configHome := filepath.Join(t.TempDir(), ".config")
	legacyLink := filepath.Join(configHome, "hypr", "lib", "mysetup.lua")
	if err := os.MkdirAll(filepath.Dir(legacyLink), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(historicalHyprLegacyLinkTarget, legacyLink); err != nil {
		t.Fatal(err)
	}
	snapshots, err := snapshotLegacyLinks(configHome, []string{legacyLink})
	if err != nil {
		t.Fatal(err)
	}
	runner := &configHomeSwapOnCommandRunner{}
	journal := legacyMigrationJournal{}
	recovery, err := quarantineLegacyLinks(context.Background(), runner, configHome, snapshots, &journal)
	if recovery != nil {
		defer recovery.close()
	}
	defer journal.close()
	if err != nil {
		t.Fatalf("quarantine exact legacy link: %v", err)
	}
	if runner.commands != 0 {
		t.Fatalf("quarantine invoked %d path-based commands after pinning recovery", runner.commands)
	}
	if _, statErr := os.Lstat(legacyLink); !os.IsNotExist(statErr) {
		t.Fatalf("managed link remains after quarantine: %v", statErr)
	}
}

func TestQuarantineLegacyLinksRejectsConfigRootReplacementBeforeRecoveryCreation(t *testing.T) {
	root := t.TempDir()
	configHome := filepath.Join(root, ".config")
	originalRoot := filepath.Join(root, ".config-before-race")
	relative := filepath.Join("hypr", "lib", "mysetup.lua")
	legacyLink := filepath.Join(configHome, relative)
	if err := os.MkdirAll(filepath.Dir(legacyLink), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(historicalHyprLegacyLinkTarget, legacyLink); err != nil {
		t.Fatal(err)
	}
	snapshots, err := snapshotLegacyLinks(configHome, []string{legacyLink})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(configHome, originalRoot); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(configHome, relative)
	if err := os.MkdirAll(filepath.Dir(replacement), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(historicalHyprLegacyLinkTarget, replacement); err != nil {
		t.Fatal(err)
	}
	journal := legacyMigrationJournal{}
	defer journal.close()

	recovery, err := quarantineLegacyLinks(context.Background(), run.New(false), configHome, snapshots, &journal)
	if recovery != nil {
		defer recovery.close()
		t.Fatalf("replacement config root produced recovery at %s", recovery.path())
	}
	if err == nil || !strings.Contains(err.Error(), "config root changed after preflight") {
		t.Fatalf("quarantine error = %v, want bound config root collision", err)
	}
	for path := range map[string]struct{}{
		filepath.Join(originalRoot, relative): {},
		replacement:                           {},
	} {
		if target, readErr := os.Readlink(path); readErr != nil || target != historicalHyprLegacyLinkTarget {
			t.Fatalf("managed link changed at %s: target=%q err=%v", path, target, readErr)
		}
	}
	matches, globErr := filepath.Glob(filepath.Join(configHome, ".wahrwelt-migration-recovery-links-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 0 {
		t.Fatalf("recovery created in replacement config root: %v", matches)
	}
}

func TestMigrateLegacyUserPathsRollsBackAfterLateLinkFailureAndRetainsRecovery(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	for path, content := range map[string]string{
		filepath.Join(configHome, "mysetup", "config"):               "legacy config\n",
		filepath.Join(configHome, "hypr", "wahrwelt", "default.lua"): "-- user config\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	links := map[string]string{
		filepath.Join(configHome, "hypr", "lib", "mysetup.lua"):           historicalHyprLegacyLinkTarget,
		filepath.Join(configHome, "quickshell", "mysetup-shell-selector"): historicalSelectorLinkTarget,
	}
	for path, target := range links {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
	}

	err := migrateLegacyUserPathsWithHooks(context.Background(), run.New(false), home, legacyUserMigrationHooks{
		link: func(index int, _ *legacyLinkRecovery, _ *migrationPinnedDirectory) error {
			if index == 1 {
				return errors.New("injected second link quarantine failure")
			}
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "injected second link quarantine failure") {
		t.Fatalf("migration error = %v, want late link failure", err)
	}
	if got := readTestFile(t, filepath.Join(configHome, "mysetup", "config")); got != "legacy config\n" {
		t.Fatalf("config move was not rolled back: %q", got)
	}
	if got := readTestFile(t, filepath.Join(configHome, "hypr", "wahrwelt", "default.lua")); got != "-- user config\n" {
		t.Fatalf("Hypr move was not rolled back: %q", got)
	}
	for path, wantTarget := range links {
		if target, readErr := os.Readlink(path); readErr != nil || target != wantTarget {
			t.Fatalf("legacy link %s was not restored: target=%q err=%v", path, target, readErr)
		}
	}
	matches, globErr := filepath.Glob(filepath.Join(configHome, ".wahrwelt-migration-recovery-links-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 1 || !strings.Contains(err.Error(), matches[0]) {
		t.Fatalf("late link rollback did not retain explicit recovery: paths=%v err=%v", matches, err)
	}
}

type linkRecoveryRootSwapRunner struct {
	recovery string
	moved    string
	attacker string
	swapped  bool
}

func (r *linkRecoveryRootSwapRunner) Command(ctx context.Context, name string, args ...string) error {
	if err := exec.CommandContext(ctx, name, args...).Run(); err != nil {
		return err
	}
	if name != "mkdir" || r.swapped || len(args) == 0 || !strings.Contains(args[len(args)-1], ".wahrwelt-migration-recovery-links-") {
		return nil
	}
	r.swapped = true
	r.recovery = args[len(args)-1]
	r.moved = r.recovery + ".moved"
	r.attacker = r.recovery + ".attacker"
	if err := os.Rename(r.recovery, r.moved); err != nil {
		return err
	}
	if err := os.Mkdir(r.attacker, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(r.attacker, "winner"), []byte("attacker owner\n"), 0o600); err != nil {
		return err
	}
	return os.Symlink(r.attacker, r.recovery)
}

func (*linkRecoveryRootSwapRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (*linkRecoveryRootSwapRunner) IsDryRun() bool { return false }

func TestMigrateLegacyUserPathsPinsLinkRecoveryAgainstParentSwap(t *testing.T) {
	home := t.TempDir()
	legacyConfig := filepath.Join(home, ".config", "mysetup")
	legacyLink := filepath.Join(home, ".config", "hypr", "lib", "mysetup.lua")
	if err := os.MkdirAll(legacyConfig, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyConfig, "config"), []byte("legacy config\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyLink), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(historicalHyprLegacyLinkTarget, legacyLink); err != nil {
		t.Fatal(err)
	}
	runner := &linkRecoveryRootSwapRunner{}

	err := migrateLegacyUserPathsWithHooks(context.Background(), run.New(false), home, legacyUserMigrationHooks{
		link: func(index int, recovery *legacyLinkRecovery, _ *migrationPinnedDirectory) error {
			if index != 0 || runner.swapped {
				return nil
			}
			runner.swapped = true
			runner.recovery = recovery.path()
			runner.moved = runner.recovery + ".moved"
			runner.attacker = runner.recovery + ".attacker"
			if err := os.Rename(runner.recovery, runner.moved); err != nil {
				return err
			}
			if err := os.Mkdir(runner.attacker, 0o700); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(runner.attacker, "winner"), []byte("attacker owner\n"), 0o600); err != nil {
				return err
			}
			return os.Symlink(runner.attacker, runner.recovery)
		},
	})
	if err == nil || !strings.Contains(err.Error(), "pinned directory path changed") {
		t.Fatalf("migration error = %v, want pinned link recovery collision", err)
	}
	if target, readErr := os.Readlink(legacyLink); readErr != nil || target != historicalHyprLegacyLinkTarget {
		t.Fatalf("managed legacy link was not restored through pinned recovery: target=%q err=%v", target, readErr)
	}
	if got := readTestFile(t, filepath.Join(legacyConfig, "config")); got != "legacy config\n" {
		t.Fatalf("config move was not rolled back after recovery root swap: %q", got)
	}
	if target, readErr := os.Readlink(runner.recovery); readErr != nil || target != runner.attacker {
		t.Fatalf("concurrent recovery-path winner changed: target=%q err=%v", target, readErr)
	}
	if got := readTestFile(t, filepath.Join(runner.attacker, "winner")); got != "attacker owner\n" {
		t.Fatalf("concurrent recovery directory changed: %q", got)
	}
	if entries, readErr := os.ReadDir(runner.moved); readErr != nil || len(entries) != 0 {
		t.Fatalf("pinned moved recovery should be retained and empty after rollback: entries=%v err=%v", entries, readErr)
	}
}

func TestMigrateLegacyUserPathsRollsBackHyprMoveThroughRetainedParentAfterLateFailure(t *testing.T) {
	home := t.TempDir()
	hyprDir := filepath.Join(home, ".config", "hypr")
	originalParent := filepath.Join(home, ".config", "hypr-before-race")
	cacheHome := filepath.Join(home, ".cache")
	for path, content := range map[string]string{
		filepath.Join(hyprDir, "wahrwelt", "default.lua"): "-- legacy user config\n",
		filepath.Join(cacheHome, "mysetup", "legacy"):     "legacy cache\n",
		filepath.Join(cacheHome, "wahrwelt", "canonical"): "canonical cache\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	err := migrateLegacyUserPathsWithHooks(context.Background(), run.New(false), home, legacyUserMigrationHooks{
		cache: func(_ *migrationPinnedDirectory, _ string) error {
			if err := os.Rename(hyprDir, originalParent); err != nil {
				return err
			}
			for path, content := range map[string]string{
				filepath.Join(hyprDir, "wahrwelt", "winner"): "legacy-name winner\n",
				filepath.Join(hyprDir, "user", "winner"):     "canonical-name winner\n",
			} {
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					return err
				}
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					return err
				}
			}
			return errors.New("injected cache failure after Hypr parent replacement")
		},
	})
	if err == nil || !strings.Contains(err.Error(), "injected cache failure after Hypr parent replacement") {
		t.Fatalf("migration error = %v, want late cache failure", err)
	}
	if got := readTestFile(t, filepath.Join(originalParent, "wahrwelt", "default.lua")); got != "-- legacy user config\n" {
		t.Fatalf("Hypr move was not restored through retained parent: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(originalParent, "user")); !os.IsNotExist(statErr) {
		t.Fatalf("Hypr canonical target remains in original parent: %v", statErr)
	}
	for path, want := range map[string]string{
		filepath.Join(hyprDir, "wahrwelt", "winner"): "legacy-name winner\n",
		filepath.Join(hyprDir, "user", "winner"):     "canonical-name winner\n",
	} {
		if got := readTestFile(t, path); got != want {
			t.Fatalf("visible Hypr parent winner changed at %s: %q", path, got)
		}
	}
}

func TestMigrateLegacyUserPathsCacheHookFailureDoesNotMutateEitherCache(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	cacheHome := filepath.Join(home, ".cache")
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	for path, content := range map[string]string{
		filepath.Join(configHome, "mysetup", "config"):         "legacy config\n",
		filepath.Join(cacheHome, "mysetup", "legacy-only"):     "legacy cache\n",
		filepath.Join(cacheHome, "wahrwelt", "canonical-only"): "canonical cache\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	err := migrateLegacyUserPathsWithHooks(context.Background(), run.New(false), home, legacyUserMigrationHooks{
		cache: func(_ *migrationPinnedDirectory, _ string) error {
			return errors.New("injected cache quarantine failure")
		},
	})
	if err == nil || !strings.Contains(err.Error(), "injected cache quarantine failure") {
		t.Fatalf("migration error = %v, want cache hook failure", err)
	}
	if got := readTestFile(t, filepath.Join(configHome, "mysetup", "config")); got != "legacy config\n" {
		t.Fatalf("config move was not rolled back: %q", got)
	}
	if got := readTestFile(t, filepath.Join(cacheHome, "mysetup", "legacy-only")); got != "legacy cache\n" {
		t.Fatalf("legacy cache changed after failed staging: %q", got)
	}
	if got := readTestFile(t, filepath.Join(cacheHome, "wahrwelt", "canonical-only")); got != "canonical cache\n" {
		t.Fatalf("canonical cache changed after failed staging: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(cacheHome, "wahrwelt", "legacy-only")); !os.IsNotExist(statErr) {
		t.Fatalf("failed cache staging partially mutated canonical cache: %v", statErr)
	}
	matches, globErr := filepath.Glob(filepath.Join(cacheHome, ".wahrwelt-migration-recovery-cache-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 1 || !strings.Contains(err.Error(), matches[0]) {
		t.Fatalf("failed cache quarantine did not retain explicit recovery: paths=%v err=%v", matches, err)
	}
	if entries, readErr := os.ReadDir(matches[0]); readErr != nil || len(entries) != 0 {
		t.Fatalf("pre-quarantine recovery must remain empty: entries=%v err=%v", entries, readErr)
	}
}

func TestMigrateLegacyUserPathsPinsCacheRecoveryAndRollsBackParentSwap(t *testing.T) {
	home := t.TempDir()
	legacyConfig := filepath.Join(home, ".config", "mysetup")
	oldCache := filepath.Join(home, ".cache", "mysetup")
	newCache := filepath.Join(home, ".cache", "wahrwelt")
	for path, content := range map[string]string{
		filepath.Join(legacyConfig, "config"):     "legacy config\n",
		filepath.Join(oldCache, "legacy-only"):    "legacy cache\n",
		filepath.Join(newCache, "canonical-only"): "canonical cache\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var recoveryPath, movedRecovery, attacker string
	err := migrateLegacyUserPathsWithHooks(context.Background(), run.New(false), home, legacyUserMigrationHooks{
		cache: func(recovery *migrationPinnedDirectory, _ string) error {
			recoveryPath = recovery.path
			movedRecovery = recoveryPath + ".moved"
			attacker = recoveryPath + ".attacker"
			if err := os.Rename(recoveryPath, movedRecovery); err != nil {
				return err
			}
			if err := os.Mkdir(attacker, 0o700); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(attacker, "winner"), []byte("attacker owner\n"), 0o600); err != nil {
				return err
			}
			return os.Symlink(attacker, recoveryPath)
		},
	})
	if err == nil || !strings.Contains(err.Error(), "pinned directory path changed") {
		t.Fatalf("migration error = %v, want pinned cache recovery collision", err)
	}
	for path, want := range map[string]string{
		filepath.Join(legacyConfig, "config"):     "legacy config\n",
		filepath.Join(oldCache, "legacy-only"):    "legacy cache\n",
		filepath.Join(newCache, "canonical-only"): "canonical cache\n",
		filepath.Join(attacker, "winner"):         "attacker owner\n",
	} {
		if got := readTestFile(t, path); got != want {
			t.Fatalf("path %s = %q, want %q", path, got, want)
		}
	}
	if entries, readErr := os.ReadDir(movedRecovery); readErr != nil || len(entries) != 0 {
		t.Fatalf("pinned moved cache recovery should be empty after rollback: entries=%v err=%v", entries, readErr)
	}
	if target, readErr := os.Readlink(recoveryPath); readErr != nil || target != attacker {
		t.Fatalf("concurrent cache recovery-path winner changed: target=%q err=%v", target, readErr)
	}
}

func TestMigrateLegacyUserPathsRejectsSymlinkNamespaceSource(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	outside := filepath.Join(home, "outside")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(configHome, "mysetup")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, legacy); err != nil {
		t.Fatal(err)
	}

	err := migrateLegacyUserPaths(context.Background(), run.New(false), home)
	if err == nil || !strings.Contains(err.Error(), "source namespace must be an ordinary directory") {
		t.Fatalf("migration error = %v, want symlink namespace rejection", err)
	}
	if target, readErr := os.Readlink(legacy); readErr != nil || target != outside {
		t.Fatalf("legacy namespace symlink changed: target=%q err=%v", target, readErr)
	}
	if _, statErr := os.Lstat(filepath.Join(configHome, "wahrwelt")); !os.IsNotExist(statErr) {
		t.Fatalf("canonical namespace appeared after symlink rejection: %v", statErr)
	}
}

func TestMigrateLegacyUserPathsRejectsUnmanagedLegacySymlink(t *testing.T) {
	home := t.TempDir()
	legacyConfig := filepath.Join(home, ".config", "mysetup")
	legacyLink := filepath.Join(home, ".config", "hypr", "lib", "mysetup.lua")
	if err := os.MkdirAll(legacyConfig, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyConfig, "config"), []byte("legacy config\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyLink), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../user-owned.lua", legacyLink); err != nil {
		t.Fatal(err)
	}

	err := migrateLegacyUserPaths(context.Background(), run.New(false), home)
	if err == nil || !strings.Contains(err.Error(), "unmanaged legacy symlink collision") {
		t.Fatalf("migration error = %v, want unmanaged symlink ownership collision", err)
	}
	if target, readErr := os.Readlink(legacyLink); readErr != nil || target != "../user-owned.lua" {
		t.Fatalf("unmanaged legacy symlink changed: target=%q err=%v", target, readErr)
	}
	if got := readTestFile(t, filepath.Join(legacyConfig, "config")); got != "legacy config\n" {
		t.Fatalf("config moved before unmanaged symlink collision: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(home, ".config", "wahrwelt")); !os.IsNotExist(statErr) {
		t.Fatalf("canonical config appeared before unmanaged symlink collision: %v", statErr)
	}
}

func TestMigrateLegacyUserPathsRejectsUnrelatedNixStoreLegacySymlinkBeforeAnyMove(t *testing.T) {
	for _, test := range []struct {
		name string
		rel  string
	}{
		{name: "hypr library", rel: filepath.Join("hypr", "lib", "mysetup.lua")},
		{name: "shell selector", rel: filepath.Join("quickshell", "mysetup-shell-selector")},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			configHome := filepath.Join(home, ".config")
			legacyConfig := filepath.Join(configHome, "mysetup")
			legacyLink := filepath.Join(configHome, test.rel)
			if err := os.MkdirAll(legacyConfig, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(legacyConfig, "config"), []byte("legacy config\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(legacyLink), 0o700); err != nil {
				t.Fatal(err)
			}
			unrelated := "/nix/store/22222222222222222222222222222222-unrelated-object"
			if err := os.Symlink(unrelated, legacyLink); err != nil {
				t.Fatal(err)
			}

			err := migrateLegacyUserPaths(context.Background(), run.New(false), home)
			if err == nil || !strings.Contains(err.Error(), "unmanaged legacy symlink collision") {
				t.Fatalf("migration error = %v, want unrelated Nix store ownership collision", err)
			}
			if target, readErr := os.Readlink(legacyLink); readErr != nil || target != unrelated {
				t.Fatalf("unrelated Nix store symlink changed: target=%q err=%v", target, readErr)
			}
			if got := readTestFile(t, filepath.Join(legacyConfig, "config")); got != "legacy config\n" {
				t.Fatalf("config moved before unrelated Nix store collision: %q", got)
			}
			if _, statErr := os.Lstat(filepath.Join(configHome, "wahrwelt")); !os.IsNotExist(statErr) {
				t.Fatalf("canonical config appeared before unrelated Nix store collision: %v", statErr)
			}
		})
	}
}

func TestMigrateLegacyUserPathsIgnoresHostileProcessXDGForManagedHome(t *testing.T) {
	managedHome := t.TempDir()
	victimHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(victimHome, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(victimHome, "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(victimHome, "cache"))
	for path, content := range map[string]string{
		filepath.Join(managedHome, ".config", "mysetup", "config"):        "managed config\n",
		filepath.Join(managedHome, ".local", "state", "mysetup", "state"): "managed state\n",
		filepath.Join(managedHome, ".cache", "mysetup", "cache"):          "managed cache\n",
		filepath.Join(victimHome, "config", "mysetup", "victim-config"):   "victim config\n",
		filepath.Join(victimHome, "state", "mysetup", "victim-state"):     "victim state\n",
		filepath.Join(victimHome, "cache", "mysetup", "victim-cache"):     "victim cache\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := migrateLegacyUserPaths(context.Background(), run.New(false), managedHome); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		filepath.Join(managedHome, ".config", "wahrwelt", "config"):        "managed config\n",
		filepath.Join(managedHome, ".local", "state", "wahrwelt", "state"): "managed state\n",
		filepath.Join(managedHome, ".cache", "wahrwelt", "cache"):          "managed cache\n",
		filepath.Join(victimHome, "config", "mysetup", "victim-config"):    "victim config\n",
		filepath.Join(victimHome, "state", "mysetup", "victim-state"):      "victim state\n",
		filepath.Join(victimHome, "cache", "mysetup", "victim-cache"):      "victim cache\n",
	} {
		if got := readTestFile(t, path); got != want {
			t.Fatalf("path %s = %q, want %q", path, got, want)
		}
	}
	for _, path := range []string{
		filepath.Join(victimHome, "config", "wahrwelt"),
		filepath.Join(victimHome, "state", "wahrwelt"),
		filepath.Join(victimHome, "cache", "wahrwelt"),
	} {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("hostile process XDG target was mutated: %s: %v", path, statErr)
		}
	}
}

type commandCountingRunner struct {
	commands int
}

func (r *commandCountingRunner) Command(context.Context, string, ...string) error {
	r.commands++
	return nil
}

func (*commandCountingRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (*commandCountingRunner) IsDryRun() bool { return false }

func TestMigrateLegacyUserPathsRejectsSymlinkCacheSourceBeforeAnyMove(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	cacheHome := filepath.Join(home, ".cache")
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	configLegacy := filepath.Join(configHome, "mysetup")
	if err := os.MkdirAll(configLegacy, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(home, "outside-cache")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	cacheLegacy := filepath.Join(cacheHome, "mysetup")
	if err := os.MkdirAll(filepath.Dir(cacheLegacy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, cacheLegacy); err != nil {
		t.Fatal(err)
	}
	runner := &commandCountingRunner{}

	err := migrateLegacyUserPaths(context.Background(), runner, home)
	if err == nil || !strings.Contains(err.Error(), "cache paths must be directories") {
		t.Fatalf("migration error = %v, want cache symlink rejection", err)
	}
	if runner.commands != 0 {
		t.Fatalf("cache preflight collision ran %d commands before failing", runner.commands)
	}
	if _, statErr := os.Lstat(configLegacy); statErr != nil {
		t.Fatalf("config namespace changed before cache preflight failure: %v", statErr)
	}
	if target, readErr := os.Readlink(cacheLegacy); readErr != nil || target != outside {
		t.Fatalf("cache source symlink changed: target=%q err=%v", target, readErr)
	}
}

func TestCreatePinnedRecoveryDirectoryRejectsReplacementBeforeOpen(t *testing.T) {
	root := t.TempDir()
	parent, err := pinOrdinaryDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.close()
	var createdPath string
	var createdRecovery string
	recovery, err := createPinnedRecoveryDirectoryWithHook(
		parent,
		".wahrwelt-migration-recovery-test-",
		func(path string) error {
			createdPath = path
			createdRecovery = filepath.Join(root, "created-recovery-before-open")
			if err := os.Rename(path, createdRecovery); err != nil {
				return err
			}
			if err := os.Mkdir(path, 0o711); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(path, "winner"), []byte("unknown owner\n"), 0o600)
		},
	)
	if recovery != nil {
		recovery.close()
		t.Fatal("replacement recovery directory was adopted")
	}
	if err == nil || !strings.Contains(err.Error(), "changed before pinning") {
		t.Fatalf("createPinnedRecoveryDirectoryWithHook() error = %v, want creator-token rejection", err)
	}
	if got := readTestFile(t, filepath.Join(createdPath, "winner")); got != "unknown owner\n" {
		t.Fatalf("unknown replacement changed: %q", got)
	}
	created, err := os.Stat(createdRecovery)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := created.Mode().Perm(), os.FileMode(0o700); got != want {
		t.Fatalf("created recovery mode = %o, want %o", got, want)
	}
}

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
	if err := os.Symlink(historicalHyprLegacyLinkTarget, oldLink); err != nil {
		t.Fatal(err)
	}

	if err := migrateLegacyUserPaths(context.Background(), run.New(false), home); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		filepath.Join(home, ".config", "wahrwelt", "boot-theme", "README.txt"),
		filepath.Join(home, ".config", "hypr", "user", "keybinds.lua"),
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
	linkRecovery, err := filepath.Glob(filepath.Join(home, ".config", ".wahrwelt-migration-recovery-links-*", "link-0"))
	if err != nil {
		t.Fatal(err)
	}
	if len(linkRecovery) != 1 {
		t.Fatalf("expected one retained legacy link recovery, got %v", linkRecovery)
	}
	if target, err := os.Readlink(linkRecovery[0]); err != nil || target != historicalHyprLegacyLinkTarget {
		t.Fatalf("retained legacy link recovery changed: target=%q err=%v", target, err)
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

func TestMigrateLegacyUserPathsPreflightsAllHyprNamespacesBeforeOtherMoves(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	for path, content := range map[string]string{
		filepath.Join(home, ".config", "mysetup", "root.txt"):             "root legacy\n",
		filepath.Join(home, ".config", "hypr", "mysetup", "user.lua"):     "very old\n",
		filepath.Join(home, ".config", "hypr", "wahrwelt", "default.lua"): "legacy\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	err := migrateLegacyUserPaths(context.Background(), run.New(false), home)
	if err == nil || !strings.Contains(err.Error(), "migration conflict") {
		t.Fatalf("expected all-source Hypr preflight failure, got %v", err)
	}
	for path, want := range map[string]string{
		filepath.Join(home, ".config", "mysetup", "root.txt"):             "root legacy\n",
		filepath.Join(home, ".config", "hypr", "mysetup", "user.lua"):     "very old\n",
		filepath.Join(home, ".config", "hypr", "wahrwelt", "default.lua"): "legacy\n",
	} {
		if got := readTestFile(t, path); got != want {
			t.Fatalf("path changed before complete Hypr preflight: %s = %q", path, got)
		}
	}
}

func TestMigrateLegacyUserPathsAcceptsExactActiveHomeManagerHyprAdapter(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, ".config", "hypr", "wahrwelt")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "default.lua"), []byte("-- local user config\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	generation := filepath.Join(home, "home-manager-generation")
	managedTarget := filepath.Join(generation, "home-files", ".config", "hypr", "wahrwelt", "hyprland.lua")
	if err := os.MkdirAll(filepath.Dir(managedTarget), 0o755); err != nil {
		t.Fatal(err)
	}
	managedSource, err := os.ReadFile("../../../dots/hypr/hyprland.lua")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managedTarget, managedSource, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(managedTarget, filepath.Join(legacy, "hyprland.lua")); err != nil {
		t.Fatal(err)
	}
	gcroot := filepath.Join(home, ".local", "state", "home-manager", "gcroots", "current-home")
	if err := os.MkdirAll(filepath.Dir(gcroot), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(generation, gcroot); err != nil {
		t.Fatal(err)
	}

	if err := migrateLegacyUserPaths(context.Background(), run.New(false), home); err != nil {
		t.Fatalf("migrate active Home Manager adapter: %v", err)
	}
	canonical := filepath.Join(home, ".config", "hypr", "user")
	if got, err := os.Readlink(filepath.Join(canonical, "hyprland.lua")); err != nil || got != managedTarget {
		t.Fatalf("managed adapter link changed: target=%q want=%q err=%v", got, managedTarget, err)
	}
	if got := readTestFile(t, filepath.Join(canonical, "default.lua")); got != "-- local user config\n" {
		t.Fatalf("local user config changed: %q", got)
	}
	if _, err := os.Lstat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy Hypr namespace remains: %v", err)
	}
}

func TestMigrateLegacyUserPathsLeavesHyprLegacyTreeOnLaterConfigCollision(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	legacy := filepath.Join(home, ".config", "hypr", "wahrwelt")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "default.lua"), []byte("-- local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
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
		t.Fatalf("expected config migration collision, got %v", err)
	}
	if got := readTestFile(t, filepath.Join(legacy, "default.lua")); got != "-- local\n" {
		t.Fatalf("legacy Hypr tree changed before later migration failure: %q", got)
	}
	if _, err := os.Lstat(filepath.Join(home, ".config", "hypr", "user")); !os.IsNotExist(err) {
		t.Fatalf("canonical Hypr tree exists after failed migration: %v", err)
	}
}

func TestMigrateLegacyUserPathsRejectsTargetCreatedAfterPreflightWithoutNestingSource(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	legacy := filepath.Join(home, ".config", "hypr", "wahrwelt")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "default.lua"), []byte("-- legacy user config\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := migrateLegacyUserPathsWithHyprMigrationHook(context.Background(), run.New(false), home, func(stage hyprUserMigrationCommitStage, migration hyprUserMigration) error {
		if stage != hyprUserMigrationBeforeRename {
			t.Fatalf("unexpected migration stage %q", stage)
		}
		if err := os.MkdirAll(migration.target, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(migration.target, "concurrent-owner.lua"), []byte("-- concurrent owner\n"), 0o600)
	})
	if err == nil || !strings.Contains(err.Error(), "appeared during migration") {
		t.Fatalf("expected atomic no-clobber target collision, got %v", err)
	}
	if got := readTestFile(t, filepath.Join(legacy, "default.lua")); got != "-- legacy user config\n" {
		t.Fatalf("legacy Hypr tree changed after target race: %q", got)
	}
	user := filepath.Join(home, ".config", "hypr", "user")
	if got := readTestFile(t, filepath.Join(user, "concurrent-owner.lua")); got != "-- concurrent owner\n" {
		t.Fatalf("concurrent canonical tree changed: %q", got)
	}
	if _, err := os.Lstat(filepath.Join(user, "wahrwelt")); !os.IsNotExist(err) {
		t.Fatalf("legacy tree was nested into concurrent canonical target: %v", err)
	}
}

func TestMigrateLegacyUserPathsPreflightsWholeCollisionSequenceBeforeAnyMove(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	for path, content := range map[string]string{
		filepath.Join(home, ".config", "mysetup", "root.txt"):             "root legacy\n",
		filepath.Join(home, ".local", "state", "mysetup", "active-shell"): "end4-pc\n",
		filepath.Join(home, ".cache", "mysetup", "staging", "sentinel"):   "cache legacy\n",
		filepath.Join(home, ".config", "hypr", "wahrwelt", "default.lua"): "-- local user config\n",
		filepath.Join(home, ".config", "hypr", "lib", "mysetup.lua"):      "not a managed symlink\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	err := migrateLegacyUserPaths(context.Background(), run.New(false), home)
	if err == nil || !strings.Contains(err.Error(), "refusing to remove non-symlink") {
		t.Fatalf("expected late legacy-link preflight conflict, got %v", err)
	}
	for path, want := range map[string]string{
		filepath.Join(home, ".config", "mysetup", "root.txt"):             "root legacy\n",
		filepath.Join(home, ".local", "state", "mysetup", "active-shell"): "end4-pc\n",
		filepath.Join(home, ".cache", "mysetup", "staging", "sentinel"):   "cache legacy\n",
		filepath.Join(home, ".config", "hypr", "wahrwelt", "default.lua"): "-- local user config\n",
		filepath.Join(home, ".config", "hypr", "lib", "mysetup.lua"):      "not a managed symlink\n",
	} {
		if got := readTestFile(t, path); got != want {
			t.Fatalf("path changed before full migration preflight: %s = %q", path, got)
		}
	}
	for _, path := range []string{
		filepath.Join(home, ".config", "wahrwelt"),
		filepath.Join(home, ".local", "state", "wahrwelt"),
		filepath.Join(home, ".cache", "wahrwelt"),
		filepath.Join(home, ".config", "hypr", "user"),
	} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("canonical path appeared before failure: %s: %v", path, err)
		}
	}
}

func TestMigrateLegacyUserPathsPreservesCanonicalCacheAndQuarantinesLegacy(t *testing.T) {
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
		t.Fatalf("legacy cache public name must be removed after quarantine, got %v", err)
	}
	for name, want := range map[string]string{
		"new-only": "canonical\n",
		"shared":   "canonical\n",
	} {
		data, err := os.ReadFile(filepath.Join(newCache, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("unexpected merged cache content for %s: got %q, want %q", name, data, want)
		}
	}
	if _, err := os.Lstat(filepath.Join(newCache, "legacy-only")); !os.IsNotExist(err) {
		t.Fatalf("legacy-only cache data must not be merged into canonical cache: %v", err)
	}
	recovery, err := filepath.Glob(filepath.Join(cacheHome, ".wahrwelt-migration-recovery-cache-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(recovery) != 1 {
		t.Fatalf("expected one retained cache recovery, got %v", recovery)
	}
	for path, want := range map[string]string{
		filepath.Join(recovery[0], "legacy-original", "legacy-only"): "legacy\n",
		filepath.Join(recovery[0], "legacy-original", "shared"):      "legacy\n",
	} {
		if got := readTestFile(t, path); got != want {
			t.Fatalf("retained cache recovery %s = %q, want %q", path, got, want)
		}
	}
}

func TestMigrateLegacyUserPathsPreservesConcurrentCanonicalCacheWinner(t *testing.T) {
	home := t.TempDir()
	cacheHome := filepath.Join(home, ".cache")
	oldCache := filepath.Join(cacheHome, "mysetup")
	newCache := filepath.Join(cacheHome, "wahrwelt")
	for path, content := range map[string]string{
		filepath.Join(oldCache, "raced"):    "legacy bytes\n",
		filepath.Join(newCache, "existing"): "canonical bytes\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	err := migrateLegacyUserPathsWithHooks(context.Background(), run.New(false), home, legacyUserMigrationHooks{
		cache: func(_ *migrationPinnedDirectory, pinnedNew string) error {
			return os.WriteFile(filepath.Join(pinnedNew, "raced"), []byte("concurrent winner\n"), 0o600)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, filepath.Join(newCache, "raced")); got != "concurrent winner\n" {
		t.Fatalf("concurrent canonical cache winner changed: %q", got)
	}
	recoveries, err := filepath.Glob(filepath.Join(cacheHome, ".wahrwelt-migration-recovery-cache-*"))
	if err != nil || len(recoveries) != 1 {
		t.Fatalf("legacy cache recovery = %v, err=%v", recoveries, err)
	}
	if got := readTestFile(t, filepath.Join(recoveries[0], "legacy-original", "raced")); got != "legacy bytes\n" {
		t.Fatalf("legacy raced bytes were not recoverable: %q", got)
	}
}

func TestMigrateLegacyUserPathsRejectsLegacyNamespaceAppearingDuringCacheCommit(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	cacheHome := filepath.Join(home, ".cache")
	legacyConfig := filepath.Join(configHome, "mysetup")
	oldCache := filepath.Join(cacheHome, "mysetup")
	newCache := filepath.Join(cacheHome, "wahrwelt")
	for path, content := range map[string]string{
		filepath.Join(oldCache, "legacy"):    "legacy cache\n",
		filepath.Join(newCache, "canonical"): "canonical cache\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	unknown := filepath.Join(legacyConfig, "unknown")
	err := migrateLegacyUserPathsWithHooks(context.Background(), run.New(false), home, legacyUserMigrationHooks{
		cache: func(_ *migrationPinnedDirectory, _ string) error {
			if err := os.MkdirAll(legacyConfig, 0o700); err != nil {
				return err
			}
			return os.WriteFile(unknown, []byte("concurrent legacy namespace\n"), 0o600)
		},
	})
	if err == nil || !strings.Contains(err.Error(), "legacy source remains after migration") {
		t.Fatalf("late legacy namespace error = %v, want final ownership collision", err)
	}
	if got := readTestFile(t, unknown); got != "concurrent legacy namespace\n" {
		t.Fatalf("late legacy namespace changed: %q", got)
	}
	if got := readTestFile(t, filepath.Join(oldCache, "legacy")); got != "legacy cache\n" {
		t.Fatalf("legacy cache changed before final validation: %q", got)
	}
	if got := readTestFile(t, filepath.Join(newCache, "canonical")); got != "canonical cache\n" {
		t.Fatalf("canonical cache changed before final validation: %q", got)
	}
}

func TestMigrateLegacyUserPathsRejectsCanonicalNamespaceReplacementDuringCacheCommit(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	cacheHome := filepath.Join(home, ".cache")
	legacyConfig := filepath.Join(configHome, "mysetup")
	canonicalConfig := filepath.Join(configHome, "wahrwelt")
	displacedConfig := filepath.Join(configHome, "migrated-config-recovery")
	oldCache := filepath.Join(cacheHome, "mysetup")
	newCache := filepath.Join(cacheHome, "wahrwelt")
	for path, content := range map[string]string{
		filepath.Join(legacyConfig, "original"): "original config\n",
		filepath.Join(oldCache, "legacy"):       "legacy cache\n",
		filepath.Join(newCache, "canonical"):    "canonical cache\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	err := migrateLegacyUserPathsWithHooks(context.Background(), run.New(false), home, legacyUserMigrationHooks{
		cache: func(_ *migrationPinnedDirectory, _ string) error {
			if err := os.Rename(canonicalConfig, displacedConfig); err != nil {
				return err
			}
			if err := os.Mkdir(canonicalConfig, 0o700); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(canonicalConfig, "attacker"), []byte("attacker config\n"), 0o600)
		},
	})
	if err == nil || !strings.Contains(err.Error(), "canonical migration target changed before final validation") {
		t.Fatalf("canonical replacement error = %v, want final target collision", err)
	}
	if got := readTestFile(t, filepath.Join(canonicalConfig, "attacker")); got != "attacker config\n" {
		t.Fatalf("canonical replacement winner changed: %q", got)
	}
	if got := readTestFile(t, filepath.Join(displacedConfig, "original")); got != "original config\n" {
		t.Fatalf("migrated config recovery changed: %q", got)
	}
	if got := readTestFile(t, filepath.Join(oldCache, "legacy")); got != "legacy cache\n" {
		t.Fatalf("legacy cache changed before target validation: %q", got)
	}
}

func TestHomeManagerMigrationQuarantinesOnlyLegacyCacheConflicts(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/home/migrations/v1_to_v2/user-paths.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`move_tree "${configHome}/mysetup" "${configHome}/wahrwelt"`,
		`preflight_hypr_user_tree`,
		`commit_hypr_user_tree`,
		`user="${configHome}/hypr/user"`,
		`old_wahrwelt="${configHome}/hypr/wahrwelt"`,
		`old_mysetup="${configHome}/hypr/mysetup"`,
		`move_tree "${stateHome}/mysetup" "${stateHome}/wahrwelt"`,
		`merge_cache "${cacheHome}/mysetup" "${cacheHome}/wahrwelt"`,
		`legacy-cache-merge`,
		`"merge" "$old" "$new" "${cacheHome}"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Home Manager migration contract is missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, `move_tree "${cacheHome}/mysetup" "${cacheHome}/wahrwelt"`) {
		t.Fatalf("cache migration must retain legacy recovery without replacing canonical cache\n%s", text)
	}
	if strings.Contains(text, `${pkgs.coreutils}/bin/rm -rf -- "$old"`) {
		t.Fatalf("cache migration must retain a recovery instead of recursively deleting the legacy path\n%s", text)
	}
}

func TestHomeManagerHyprMigrationCommitsCollisionSensitiveTreeBeforeOtherMutations(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/home/migrations/v1_to_v2/user-paths.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`preflight_hypr_user_tree()`,
		`for tree in "$user" "$hypr_user_source"; do`,
		`leaf_path="$tree/default.lua"`,
		`"check" "$leaf_path"`,
		`commit_hypr_user_tree()`,
		`preflight_hypr_user_tree`,
		`commit_hypr_user_tree`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Home Manager Hypr migration contract is missing %q\n%s", want, text)
		}
	}
	preflightCall := strings.Index(text, "    preflight_hypr_user_tree\n")
	firstMove := strings.Index(text, `    move_tree "${configHome}/mysetup" "${configHome}/wahrwelt"`)
	commitCall := strings.Index(text, "    commit_hypr_user_tree\n")
	if preflightCall < 0 || firstMove < 0 || commitCall < 0 || preflightCall > commitCall || commitCall > firstMove {
		t.Fatalf("Home Manager Hypr commit must follow all preflights and precede other mutations\n%s", text)
	}
}

func TestHomeManagerHyprFinalCommitUsesFailClosedRename(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/home/migrations/v1_to_v2/user-paths.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`hypr_wahrwelt_namespace_token="$("${legacyNamespaceMove}/bin/legacy-namespace-move"`,
		`check "$old_wahrwelt" "$user")"`,
		`hypr_mysetup_namespace_token="$("${legacyNamespaceMove}/bin/legacy-namespace-move"`,
		`check "$old_mysetup" "$user")"`,
		`hypr_namespace_token="$hypr_wahrwelt_namespace_token"`,
		`hypr_namespace_token="$hypr_mysetup_namespace_token"`,
		`"${legacyNamespaceMove}/bin/legacy-namespace-move"`,
		`move "$hypr_user_source" "$user" "$hypr_namespace_token"`,
		`verify "${configHome}/hypr/wahrwelt" "$user" "$hypr_wahrwelt_namespace_token"`,
		`verify "${configHome}/hypr/mysetup" "$user" "$hypr_mysetup_namespace_token"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Home Manager final Hypr migration must use the token-bound pinned namespace helper %q\n%s", want, text)
		}
	}
	if strings.Contains(text, `${pkgs.coreutils}/bin/mv`) {
		t.Fatalf("Home Manager final Hypr migration retained an unpinned raw rename\n%s", text)
	}
}

func TestHomeManagerHyprFinalCommitNoCopyRenamePreservesCrossDeviceSource(t *testing.T) {
	mv, err := exec.LookPath("mv")
	if err != nil {
		t.Skipf("GNU mv is unavailable: %v", err)
	}
	help, err := exec.Command(mv, "--help").CombinedOutput()
	if err != nil || !strings.Contains(string(help), "--no-copy") {
		t.Skip("installed mv does not support --no-copy")
	}

	sourceParent := t.TempDir()
	targetParent, err := os.MkdirTemp("/dev/shm", "wahrwelt-hm-hypr-migration-")
	if err != nil {
		t.Skipf("an alternate writable filesystem is unavailable: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(targetParent); err != nil {
			t.Errorf("remove alternate-filesystem test directory: %v", err)
		}
	})

	var sourceStat, targetStat unix.Stat_t
	if err := unix.Stat(sourceParent, &sourceStat); err != nil {
		t.Fatal(err)
	}
	if err := unix.Stat(targetParent, &targetStat); err != nil {
		t.Fatal(err)
	}
	if sourceStat.Dev == targetStat.Dev {
		t.Skip("test paths are on the same filesystem")
	}

	source := filepath.Join(sourceParent, "wahrwelt")
	target := filepath.Join(targetParent, "user")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal(err)
	}
	legacyFile := filepath.Join(source, "unknown.lua")
	if err := os.WriteFile(legacyFile, []byte("-- preserve me\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	output, err := exec.Command(mv, "-T", "--no-copy", "--update=none-fail", "--", source, target).CombinedOutput()
	if err == nil {
		t.Fatalf("cross-device no-copy rename unexpectedly succeeded: %s", output)
	}
	if got := readTestFile(t, legacyFile); got != "-- preserve me\n" {
		t.Fatalf("legacy Hypr file changed after cross-device failure: %q", got)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("canonical Hypr target appeared after cross-device failure: %v", err)
	}
}

func TestHomeManagerMigrationPreflightsWholeSequenceAndDoesNotRewriteLegacyHyprTree(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/home/migrations/v1_to_v2/user-paths.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`preflight_move_tree()`,
		`preflight_merge_cache()`,
		`preflight_old_links()`,
		`preflight_move_tree "${configHome}/mysetup" "${configHome}/wahrwelt"`,
		`preflight_move_tree "${stateHome}/mysetup" "${stateHome}/wahrwelt"`,
		`preflight_merge_cache "${cacheHome}/mysetup" "${cacheHome}/wahrwelt"`,
		`preflight_old_links`,
		`"$old_wahrwelt"/* | "$old_mysetup"/* | "$user"/*`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Home Manager migration is missing full preflight/preservation contract %q\n%s", want, text)
		}
	}
	if strings.Contains(text, `"''${hypr_user_source:-${configHome}/hypr/user}/keybinds.lua"`) {
		t.Fatalf("Home Manager migration must not rewrite a legacy Hypr user module before final commit\n%s", text)
	}
	if strings.Contains(text, `${configHome}/hypr/mysetup/hyprland.lua.backup`) {
		t.Fatalf("Home Manager migration must not delete a legacy Hypr user file before final commit\n%s", text)
	}
	firstMove := strings.Index(text, `    move_tree "${configHome}/mysetup" "${configHome}/wahrwelt"`)
	lastPreflight := strings.LastIndex(text, "    preflight_old_links\n")
	if firstMove < 0 || lastPreflight < 0 || lastPreflight > firstMove {
		t.Fatalf("Home Manager migration must finish whole-sequence preflight before any move\n%s", text)
	}
}
