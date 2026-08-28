package dots

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	migrationv1tov2 "github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/migrations/v1_to_v2"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/shellruntime"
	"golang.org/x/sys/unix"
)

var historicalManagedHyprEntrypointFixtures = []struct {
	digest  string
	content string
}{
	{
		digest: "cecf44b96c7afd4886d498abe0de382b2574c66281a5cf78bbac06586c1b071c",
		content: `local mysetup = require("lib.mysetup")

require("hyprland.env")
require("hyprland.general")
require("hyprland.input")
mysetup.optional_require("mysetup.local")
require("hyprland.misc")
require("hyprland.animations")
require("hyprland.decoration")
require("hyprland.group")
require("hyprland.execs")
require("hyprland.rules")
require("hyprland.gestures")
require("hyprland.scrolling")
require("hyprland.keybinds")
`,
	},
	{
		digest: "e28d16bde1d68fa2fa43c755630284f00b3c6a14f75656e89cfb5514f8633263",
		content: `local mysetup = require("lib.mysetup")

require("hyprland.env")
require("hyprland.general")
require("hyprland.input")
mysetup.optional_require("mysetup.local")
require("hyprland.misc")
require("hyprland.animations")
require("hyprland.decoration")
require("hyprland.group")
require("hyprland.execs")
require("hyprland.rules")
require("hyprland.gestures")
require("hyprland.scrolling")
require("hyprland.keybinds")
require("vm-keybinds")
`,
	},
	{
		digest: "18c3eb7f48101e0bd0b57918a683778784c74c833a215af7f7b0f1d416a0a5df",
		content: `local mysetup = require("lib.mysetup")

require("hyprland.env")
mysetup.optional_require("mysetup.env")
require("hyprland.general")
require("hyprland.input")
require("hyprland.misc")
require("hyprland.animations")
require("hyprland.decoration")
require("hyprland.group")
require("hyprland.execs")
require("hyprland.rules")
require("hyprland.gestures")
require("hyprland.scrolling")
require("hyprland.keybinds")
require("vm-keybinds")

-- Loaded last, after every bind above is already registered, so overrides in
-- these files can hl.unbind() a default before re-binding it. Each is
-- optional and independently guarded - create only the ones you need.
mysetup.optional_require("mysetup.execs")
mysetup.optional_require("mysetup.general")
mysetup.optional_require("mysetup.rules")
mysetup.optional_require("mysetup.keybinds")
`,
	},
	{
		digest: "24229642cd871aa3eb3d27c44b0d72357395951aec076a09d173b45ca17231a0",
		content: `local wahrwelt = require("lib.wahrwelt")

require("hyprland.env")
wahrwelt.optional_require("wahrwelt.env")
require("hyprland.general")
require("hyprland.input")
require("hyprland.misc")
require("hyprland.animations")
require("hyprland.decoration")
require("hyprland.group")
require("hyprland.execs")
require("hyprland.rules")
require("hyprland.gestures")
require("hyprland.scrolling")
require("hyprland.keybinds")
require("vm-keybinds")

-- Loaded last, after every bind above is already registered, so overrides in
-- these files can hl.unbind() a default before re-binding it. Each is
-- optional and independently guarded - create only the ones you need.
wahrwelt.optional_require("wahrwelt.execs")
wahrwelt.optional_require("wahrwelt.general")
wahrwelt.optional_require("wahrwelt.rules")
wahrwelt.optional_require("wahrwelt.keybinds")
`,
	},
}

func TestManagedHyprEntrypointDigestAllowlistMatchesShippedAdapters(t *testing.T) {
	for _, fixture := range historicalManagedHyprEntrypointFixtures {
		hash := sha256.Sum256([]byte(fixture.content))
		if got := fmt.Sprintf("%x", hash); got != fixture.digest {
			t.Fatalf("fixture digest = %s, want %s", got, fixture.digest)
		}
		if !recognizedManagedHyprEntrypointHash(hash, nil) {
			t.Fatalf("historical managed adapter %s is missing from the production allowlist", fixture.digest)
		}
	}
	current := readTestFile(t, "../../../dots/hypr/hyprland.lua")
	currentHash := sha256.Sum256([]byte(current))
	if !recognizedManagedHyprEntrypointHash(currentHash, nil) {
		t.Fatalf("current managed adapter %x is missing from the static migration allowlist", currentHash)
	}
}

func TestWriteHyprLocalConfigFilesMigratesOneLegacyUserTree(t *testing.T) {
	source := t.TempDir()
	hyprDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "hyprland.lua"), []byte("-- managed adapter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(hyprDir, "wahrwelt")
	if err := os.MkdirAll(filepath.Join(legacy, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "default.lua"), []byte("-- preserved user config\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(legacy, "nested", "broken.lua")
	if err := os.Symlink("missing.lua", link); err != nil {
		t.Fatal(err)
	}

	if err := writeHyprLocalConfigFiles(source, hyprDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy tree remained after migration: %v", err)
	}
	user := filepath.Join(hyprDir, "user")
	if got := readTestFile(t, filepath.Join(user, "default.lua")); got != "-- preserved user config\n" {
		t.Fatalf("migrated default changed: %q", got)
	}
	if target, err := os.Readlink(filepath.Join(user, "nested", "broken.lua")); err != nil || target != "missing.lua" {
		t.Fatalf("nested broken symlink not preserved: target=%q err=%v", target, err)
	}
	if got := readTestFile(t, filepath.Join(user, "hyprland.lua")); got != "-- managed adapter\n" {
		t.Fatalf("managed adapter not refreshed: %q", got)
	}
}

func TestWriteHyprLocalConfigFilesAcceptsExactHistoricalManagedEntrypoints(t *testing.T) {
	for _, fixture := range historicalManagedHyprEntrypointFixtures {
		t.Run(fixture.digest[:12], func(t *testing.T) {
			source := t.TempDir()
			hyprDir := t.TempDir()
			current := "-- current managed adapter\n"
			if err := os.WriteFile(filepath.Join(source, "hyprland.lua"), []byte(current), 0o644); err != nil {
				t.Fatal(err)
			}
			legacy := filepath.Join(hyprDir, "wahrwelt")
			if err := os.MkdirAll(legacy, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(legacy, "hyprland.lua"), []byte(fixture.content), 0o600); err != nil {
				t.Fatal(err)
			}

			if err := writeHyprLocalConfigFiles(source, hyprDir); err != nil {
				t.Fatal(err)
			}
			if got := readTestFile(t, filepath.Join(hyprDir, "user", "hyprland.lua")); got != current {
				t.Fatalf("managed adapter = %q, want current", got)
			}
		})
	}
}

func TestWriteHyprLocalConfigFilesRejectsUnownedLegacyEntrypointsBeforeRename(t *testing.T) {
	for _, kind := range []string{"regular", "symlink", "broken-symlink"} {
		t.Run(kind, func(t *testing.T) {
			source := t.TempDir()
			hyprDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(source, "hyprland.lua"), []byte("-- current managed adapter\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			legacy := filepath.Join(hyprDir, "wahrwelt")
			if err := os.MkdirAll(legacy, 0o755); err != nil {
				t.Fatal(err)
			}
			entrypoint := filepath.Join(legacy, "hyprland.lua")
			switch kind {
			case "regular":
				if err := os.WriteFile(entrypoint, []byte("-- private user adapter\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				target := filepath.Join(t.TempDir(), "known-content.lua")
				if err := os.WriteFile(target, []byte(historicalManagedHyprEntrypointFixtures[0].content), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, entrypoint); err != nil {
					t.Fatal(err)
				}
			case "broken-symlink":
				if err := os.Symlink(filepath.Join(t.TempDir(), "missing.lua"), entrypoint); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.Lstat(entrypoint)
			if err != nil {
				t.Fatal(err)
			}

			err = writeHyprLocalConfigFiles(source, hyprDir)
			if err == nil || !strings.Contains(err.Error(), "unowned managed Hypr user adapter collision") {
				t.Fatalf("expected ownership collision, got %v", err)
			}
			after, statErr := os.Lstat(entrypoint)
			if statErr != nil || !os.SameFile(before, after) {
				t.Fatalf("legacy entrypoint changed: before=%v after=%v err=%v", before, after, statErr)
			}
			if _, statErr := os.Lstat(filepath.Join(hyprDir, "user")); !os.IsNotExist(statErr) {
				t.Fatalf("legacy directory was renamed after collision: %v", statErr)
			}
		})
	}
}

func TestWriteHyprLocalConfigFilesRejectsUnownedCanonicalEntrypoint(t *testing.T) {
	source := t.TempDir()
	hyprDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "hyprland.lua"), []byte("-- current managed adapter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	user := filepath.Join(hyprDir, "user")
	if err := os.MkdirAll(user, 0o755); err != nil {
		t.Fatal(err)
	}
	entrypoint := filepath.Join(user, "hyprland.lua")
	if err := os.WriteFile(entrypoint, []byte("-- private user adapter\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := writeHyprLocalConfigFiles(source, hyprDir)
	if err == nil || !strings.Contains(err.Error(), "unowned managed Hypr user adapter collision") {
		t.Fatalf("expected ownership collision, got %v", err)
	}
	if got := readTestFile(t, entrypoint); got != "-- private user adapter\n" {
		t.Fatalf("canonical collision changed: %q", got)
	}
}

func TestInspectManagedHyprUserEntrypointRejectsSymlinkToFIFOWithoutBlocking(t *testing.T) {
	if entrypoint := os.Getenv("WAHRWELT_TEST_HYPR_FIFO_ENTRYPOINT"); entrypoint != "" {
		fifo := os.Getenv("WAHRWELT_TEST_HYPR_FIFO_TARGET")
		targets := managedHyprEntrypointTargets{fifo: {}}
		_, err := inspectManagedHyprUserEntrypoint(entrypoint, nil, targets)
		if err == nil || !strings.Contains(err.Error(), "unowned managed Hypr user adapter collision") {
			t.Fatalf("expected FIFO ownership collision, got %v", err)
		}
		return
	}

	dir := t.TempDir()
	fifo := filepath.Join(dir, "adapter.fifo")
	if err := unix.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	entrypoint := filepath.Join(dir, "hyprland.lua")
	if err := os.Symlink(fifo, entrypoint); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestInspectManagedHyprUserEntrypointRejectsSymlinkToFIFOWithoutBlocking$")
	command.Env = append(
		os.Environ(),
		"WAHRWELT_TEST_HYPR_FIFO_ENTRYPOINT="+entrypoint,
		"WAHRWELT_TEST_HYPR_FIFO_TARGET="+fifo,
	)
	if output, err := command.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			t.Fatalf("inspection blocked while following a symlink to a FIFO: %v", ctx.Err())
		}
		t.Fatalf("FIFO inspection subprocess failed: %v\n%s", err, output)
	}
	if target, err := os.Readlink(entrypoint); err != nil || target != fifo {
		t.Fatalf("FIFO symlink changed: target=%q err=%v", target, err)
	}
}

func TestWriteHyprLocalConfigAcceptsOnlyExactActiveHomeManagerEntrypoint(t *testing.T) {
	home := t.TempDir()
	hyprDir := filepath.Join(home, ".config", "hypr")
	source := t.TempDir()
	current := readTestFile(t, "../../../dots/hypr/hyprland.lua")
	if err := os.WriteFile(filepath.Join(source, "hyprland.lua"), []byte(current), 0o644); err != nil {
		t.Fatal(err)
	}

	makeGeneration := func(name string) (string, string) {
		t.Helper()
		generation := filepath.Join(home, name)
		leaf := filepath.Join(generation, "home-files", ".config", "hypr", "wahrwelt", "hyprland.lua")
		if err := os.MkdirAll(filepath.Dir(leaf), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(leaf, []byte(current), 0o444); err != nil {
			t.Fatal(err)
		}
		return generation, leaf
	}
	activeGeneration, activeLeaf := makeGeneration("active-generation")
	_, staleLeaf := makeGeneration("stale-generation")
	gcroot := filepath.Join(home, ".local", "state", "home-manager", "gcroots", "current-home")
	if err := os.MkdirAll(filepath.Dir(gcroot), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(activeGeneration, gcroot); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name       string
		linkTarget string
		wantError  bool
	}{
		{name: "active", linkTarget: activeLeaf},
		{name: "stale", linkTarget: staleLeaf, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			caseHyprDir := filepath.Join(hyprDir, tc.name)
			legacy := filepath.Join(caseHyprDir, "wahrwelt")
			if err := os.MkdirAll(legacy, 0o755); err != nil {
				t.Fatal(err)
			}
			entrypoint := filepath.Join(legacy, "hyprland.lua")
			if err := os.Symlink(tc.linkTarget, entrypoint); err != nil {
				t.Fatal(err)
			}

			err := writeHyprLocalConfigFilesForHome(source, caseHyprDir, home)
			if tc.wantError {
				if err == nil || !strings.Contains(err.Error(), "unowned managed Hypr user adapter collision") {
					t.Fatalf("expected stale-generation collision, got %v", err)
				}
				if target, readErr := os.Readlink(entrypoint); readErr != nil || target != tc.linkTarget {
					t.Fatalf("stale link changed: target=%q err=%v", target, readErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			migrated := filepath.Join(caseHyprDir, "user", "hyprland.lua")
			if target, readErr := os.Readlink(migrated); readErr != nil || target != tc.linkTarget {
				t.Fatalf("active Home Manager link was not preserved: target=%q err=%v", target, readErr)
			}
		})
	}
}

func TestManagedHomeManagerHyprEntrypointTargetRejectsMutableAliasToActiveManagedInode(t *testing.T) {
	dir := t.TempDir()
	managed := filepath.Join(dir, "managed-hyprland.lua")
	if err := os.WriteFile(managed, []byte("-- managed\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(dir, "active-hyprland.lua")
	mutableAlias := filepath.Join(dir, "failed-hyprland.lua")
	for _, target := range []string{active, mutableAlias} {
		if err := os.Symlink(managed, target); err != nil {
			t.Fatal(err)
		}
	}
	if isManagedHomeManagerHyprEntrypointTarget(mutableAlias, managedHyprEntrypointTargets{active: {}}) {
		t.Fatalf("mutable alias to active managed inode was accepted: %s", mutableAlias)
	}
}

func TestNixStoreHomeManagerHyprEntrypointTargetRequiresExactManagedShape(t *testing.T) {
	validHash := "0123456789abcdfghijklmnpqrsvwxyz"
	for _, target := range []string{
		"/nix/store/" + validHash + "-home-manager-files/.config/hypr/user/hyprland.lua",
		"/nix/store/" + validHash + "-home-manager-files/.config/hypr/wahrwelt/hyprland.lua",
		"/nix/store/" + validHash + "-home-manager-files/.config/hypr/mysetup/hyprland.lua",
	} {
		if !isNixStoreHomeManagerHyprEntrypointTarget(target) {
			t.Fatalf("exact Home Manager adapter target was rejected: %s", target)
		}
	}
	for _, target := range []string{
		"nix/store/" + validHash + "-home-manager-files/.config/hypr/user/hyprland.lua",
		"/nix/store/0123456789abcdefghijklmnopqrstuv-home-manager-files/.config/hypr/user/hyprland.lua",
		"/nix/store/" + validHash + "-home-manager-files/.config/hypr/custom/hyprland.lua",
		"/nix/store/" + validHash + "-home-manager-files/.config/hypr/user/nested/hyprland.lua",
		"/nix/store/" + validHash + "-other/.config/hypr/user/hyprland.lua",
	} {
		if isNixStoreHomeManagerHyprEntrypointTarget(target) {
			t.Fatalf("non-managed Home Manager adapter target was accepted: %s", target)
		}
	}
}

func TestWriteHyprLocalConfigPreservesHistoricalActiveHomeManagerEntrypoint(t *testing.T) {
	home := t.TempDir()
	hyprDir := filepath.Join(home, ".config", "hypr")
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "hyprland.lua"), []byte("-- newer managed adapter\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	generation := filepath.Join(home, "home-manager-generation")
	managedTarget := filepath.Join(generation, "home-files", ".config", "hypr", "wahrwelt", "hyprland.lua")
	if err := os.MkdirAll(filepath.Dir(managedTarget), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managedTarget, []byte(historicalManagedHyprEntrypointFixtures[0].content), 0o644); err != nil {
		t.Fatal(err)
	}
	gcroot := filepath.Join(home, ".local", "state", "home-manager", "gcroots", "current-home")
	if err := os.MkdirAll(filepath.Dir(gcroot), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(generation, gcroot); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(hyprDir, "wahrwelt")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(managedTarget, filepath.Join(legacy, "hyprland.lua")); err != nil {
		t.Fatal(err)
	}

	if err := writeHyprLocalConfigFilesForHome(source, hyprDir, home); err != nil {
		t.Fatalf("preserve historical active Home Manager adapter: %v", err)
	}
	migrated := filepath.Join(hyprDir, "user", "hyprland.lua")
	if got, err := os.Readlink(migrated); err != nil || got != managedTarget {
		t.Fatalf("historical active Home Manager link changed: target=%q want=%q err=%v", got, managedTarget, err)
	}
}

func TestMigrateLegacyHyprUserTreeRejectsEntrypointReplacementAfterPreflight(t *testing.T) {
	hyprDir := t.TempDir()
	legacy := filepath.Join(hyprDir, "wahrwelt")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	entrypoint := filepath.Join(legacy, "hyprland.lua")
	if err := os.WriteFile(entrypoint, []byte(historicalManagedHyprEntrypointFixtures[3].content), 0o600); err != nil {
		t.Fatal(err)
	}
	current := []byte("-- current managed adapter\n")

	err := migrateLegacyHyprUserTreeWithSourceAndHook(hyprDir, current, func(stage hyprUserMigrationCommitStage, _ hyprUserMigration) error {
		if stage != hyprUserMigrationBeforeRename {
			t.Fatalf("unexpected migration stage %q", stage)
		}
		return os.WriteFile(entrypoint, []byte("-- concurrent private replacement\n"), 0o600)
	})
	if err == nil || !strings.Contains(err.Error(), "changed during commit") {
		t.Fatalf("expected replacement collision, got %v", err)
	}
	if got := readTestFile(t, entrypoint); got != "-- concurrent private replacement\n" {
		t.Fatalf("replacement changed: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(hyprDir, "user")); !os.IsNotExist(statErr) {
		t.Fatalf("legacy directory was renamed after replacement: %v", statErr)
	}
}

func TestMigrateLegacyHyprUserTreeRetainsExactRecoveryAfterSourceReplacement(t *testing.T) {
	hyprDir := t.TempDir()
	legacy := filepath.Join(hyprDir, "wahrwelt")
	saved := filepath.Join(hyprDir, "wahrwelt-before-race")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "hyprland.lua"), []byte(historicalManagedHyprEntrypointFixtures[3].content), 0o600); err != nil {
		t.Fatal(err)
	}
	current := []byte("-- current managed adapter\n")

	err := migrateLegacyHyprUserTreeWithSourceAndHook(hyprDir, current, func(stage hyprUserMigrationCommitStage, _ hyprUserMigration) error {
		if stage != hyprUserMigrationBeforeRename {
			t.Fatalf("unexpected migration stage %q", stage)
		}
		if err := os.Rename(legacy, saved); err != nil {
			return err
		}
		if err := os.Mkdir(legacy, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(legacy, "winner"), []byte("replacement directory\n"), 0o600)
	})
	if err == nil || !strings.Contains(err.Error(), "recovery retained at "+saved) {
		t.Fatalf("directory replacement was accepted: %v", err)
	}
	if got := readTestFile(t, filepath.Join(filepath.Join(hyprDir, "user"), "winner")); got != "replacement directory\n" {
		t.Fatalf("replacement directory was not restored: %q", got)
	}
	if got := readTestFile(t, filepath.Join(saved, "hyprland.lua")); got != historicalManagedHyprEntrypointFixtures[3].content {
		t.Fatalf("expected legacy directory changed: %q", got)
	}
	if _, statErr := os.Lstat(legacy); !os.IsNotExist(statErr) {
		t.Fatalf("legacy basename unexpectedly reclaimed after replacement: %v", statErr)
	}
}

func TestHyprUserMigrationReportsExactRecoveryAfterPostRenameTargetReplacement(t *testing.T) {
	hyprDir := t.TempDir()
	legacy := filepath.Join(hyprDir, "wahrwelt")
	target := filepath.Join(hyprDir, "user")
	saved := filepath.Join(hyprDir, "user-moved-after-rename")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "hyprland.lua"), []byte(historicalManagedHyprEntrypointFixtures[3].content), 0o600); err != nil {
		t.Fatal(err)
	}
	migration, err := preflightHyprUserMigrationWithTargets(hyprDir, []byte("-- current adapter\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := commitHyprUserMigrationRetainingParentWithAfterHook(migration, nil, func(_ hyprUserMigration) error {
		if err := os.Rename(target, saved); err != nil {
			return err
		}
		if err := os.Mkdir(target, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(target, "winner"), []byte("replacement directory\n"), 0o600)
	})
	if parent != nil {
		parent.close()
	}
	if err == nil || !strings.Contains(err.Error(), "recovery retained at "+saved) {
		t.Fatalf("post-rename target replacement error = %v, want exact recovery path %s", err, saved)
	}
	if got := readTestFile(t, filepath.Join(saved, "hyprland.lua")); got != historicalManagedHyprEntrypointFixtures[3].content {
		t.Fatalf("migrated recovery payload = %q", got)
	}
	if got := readTestFile(t, filepath.Join(target, "winner")); got != "replacement directory\n" {
		t.Fatalf("replacement target payload = %q", got)
	}
}

func TestPublishManagedHyprUserEntrypointPreservesLateTarget(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.lua")
	target := filepath.Join(dir, "hyprland.lua")
	if err := os.WriteFile(source, []byte("-- current managed adapter\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := publishManagedHyprUserEntrypointWithHook(source, target, func() error {
		return os.WriteFile(target, []byte("-- late private target\n"), 0o600)
	})
	if err == nil || !strings.Contains(err.Error(), "appeared before publication") {
		t.Fatalf("expected publication collision, got %v", err)
	}
	if got := readTestFile(t, target); got != "-- late private target\n" {
		t.Fatalf("late target changed: %q", got)
	}
}

func TestPublishManagedHyprUserEntrypointRollsBackChangedExistingTarget(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.lua")
	target := filepath.Join(dir, "hyprland.lua")
	current := "-- current managed adapter\n"
	if err := os.WriteFile(source, []byte(current), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(historicalManagedHyprEntrypointFixtures[3].content), 0o600); err != nil {
		t.Fatal(err)
	}

	err := publishManagedHyprUserEntrypointWithHook(source, target, func() error {
		return os.WriteFile(target, []byte("-- concurrent private replacement\n"), 0o600)
	})
	if err == nil || !strings.Contains(err.Error(), "changed before publication") {
		t.Fatalf("expected exchange rollback collision, got %v", err)
	}
	if got := readTestFile(t, target); got != "-- concurrent private replacement\n" {
		t.Fatalf("concurrent replacement changed: %q", got)
	}
	matches, globErr := filepath.Glob(filepath.Join(dir, ".hyprland.lua.managed-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 0 {
		t.Fatalf("managed publication temp files leaked after rollback: %v", matches)
	}
}

func TestPublishManagedHyprUserEntrypointDoesNotMutateExternalHardlink(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.lua")
	target := filepath.Join(dir, "hyprland.lua")
	outside := filepath.Join(t.TempDir(), "external-hardlink.lua")
	current := "-- current managed adapter\n"
	historical := historicalManagedHyprEntrypointFixtures[3].content
	if err := os.WriteFile(source, []byte(current), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(historical), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(target, outside); err != nil {
		t.Fatal(err)
	}
	outsideInfo, err := os.Stat(outside)
	if err != nil {
		t.Fatal(err)
	}

	if err := publishManagedHyprUserEntrypointWithHook(source, target, nil); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, outside); got != historical {
		t.Fatalf("external hardlink content changed: %q", got)
	}
	outsideAfter, err := os.Stat(outside)
	if err != nil || !os.SameFile(outsideInfo, outsideAfter) {
		t.Fatalf("external hardlink identity changed: before=%v after=%v err=%v", outsideInfo, outsideAfter, err)
	}
	if got := readTestFile(t, target); got != current {
		t.Fatalf("managed target was not atomically replaced: %q", got)
	}
	recoveries, err := filepath.Glob(filepath.Join(dir, ".wahrwelt-migration-recovery-hypr-adapter-*", "previous-entrypoint"))
	if err != nil || len(recoveries) != 1 {
		t.Fatalf("managed adapter recovery = %v, err=%v", recoveries, err)
	}
	recoveryInfo, err := os.Stat(recoveries[0])
	if err != nil || !os.SameFile(outsideInfo, recoveryInfo) {
		t.Fatalf("recovery does not retain original hard-linked inode: info=%v err=%v", recoveryInfo, err)
	}
}

func TestManagedHyprEntrypointExchangeRestoresTargetAfterCandidateReplacement(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "hyprland.lua")
	historical := historicalManagedHyprEntrypointFixtures[3].content
	current := []byte("-- current managed adapter\n")
	if err := os.WriteFile(target, []byte(historical), 0o600); err != nil {
		t.Fatal(err)
	}
	directory, err := openPinnedDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.close()
	initial, err := inspectManagedHyprUserEntrypoint(target, current, nil)
	if err != nil {
		t.Fatal(err)
	}
	unknown := "-- candidate basename replacement\n"
	var recoveryPath string
	err = updateManagedHyprEntrypointWithExchangeHook(
		directory,
		filepath.Base(target),
		target,
		initial,
		current,
		func(path string) error {
			recoveryPath = path
			if err := os.Remove(path); err != nil {
				return err
			}
			return os.WriteFile(path, []byte(unknown), 0o640)
		},
	)
	if err == nil || !strings.Contains(err.Error(), "exact managed adapter entry restored") {
		t.Fatalf("candidate replacement error = %v, want exact rollback", err)
	}
	if got := readTestFile(t, target); got != historical {
		t.Fatalf("managed target after rollback = %q, want historical payload", got)
	}
	if got := readTestFile(t, recoveryPath); got != unknown {
		t.Fatalf("candidate basename replacement = %q, want %q", got, unknown)
	}
}

func TestPublishManagedHyprUserEntrypointUsesAnonymousTempAndPinsParent(t *testing.T) {
	dir := t.TempDir()
	originalDir := filepath.Join(dir, "user")
	outsideDir := filepath.Join(dir, "outside")
	if err := os.MkdirAll(originalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "source.lua")
	target := filepath.Join(originalDir, "hyprland.lua")
	if err := os.WriteFile(source, []byte("-- current managed adapter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	movedDir := filepath.Join(dir, "user-moved")

	err := publishManagedHyprUserEntrypointWithHook(source, target, func() error {
		matches, globErr := filepath.Glob(filepath.Join(originalDir, ".hyprland.lua.managed-*"))
		if globErr != nil {
			return globErr
		}
		if len(matches) != 0 {
			return fmt.Errorf("publisher exposed mutable temp paths: %v", matches)
		}
		if err := os.Rename(originalDir, movedDir); err != nil {
			return err
		}
		return os.Symlink(outsideDir, originalDir)
	})
	if err == nil || !strings.Contains(err.Error(), "parent directory changed") {
		t.Fatalf("expected pinned-parent collision, got %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(outsideDir, "hyprland.lua")); !os.IsNotExist(statErr) {
		t.Fatalf("publisher followed replacement parent: %v", statErr)
	}
	if info, statErr := os.Lstat(originalDir); statErr != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("replacement parent changed: info=%v err=%v", info, statErr)
	}
}

func TestWriteHyprLocalConfigFilesRejectsConflictingLegacyTreesBeforeMutation(t *testing.T) {
	source := t.TempDir()
	hyprDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "hyprland.lua"), []byte("-- managed adapter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"wahrwelt", "mysetup"} {
		path := filepath.Join(hyprDir, name)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "default.lua"), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	err := writeHyprLocalConfigFiles(source, hyprDir)
	if err == nil || !strings.Contains(err.Error(), "both legacy Hypr user config directories exist") {
		t.Fatalf("expected all-source preflight failure, got %v", err)
	}
	for _, name := range []string{"wahrwelt", "mysetup"} {
		if got := readTestFile(t, filepath.Join(hyprDir, name, "default.lua")); got != name {
			t.Fatalf("legacy %s changed before preflight completed: %q", name, got)
		}
	}
	if _, err := os.Lstat(filepath.Join(hyprDir, "user")); !os.IsNotExist(err) {
		t.Fatalf("canonical user tree created after failed preflight: %v", err)
	}
}

func TestWriteHyprLocalConfigFilesKeepsOnePinnedUserDirectoryAcrossPublications(t *testing.T) {
	for _, initial := range []string{"fresh", "existing"} {
		for _, stage := range []hyprUserWriteStage{hyprUserWriteAfterPin, hyprUserWriteBetweenPublications} {
			t.Run(initial+"-"+string(stage), func(t *testing.T) {
				source := t.TempDir()
				hyprDir := t.TempDir()
				userDir := filepath.Join(hyprDir, "user")
				retained := filepath.Join(hyprDir, "user-before-race")
				if err := os.WriteFile(filepath.Join(source, "hyprland.lua"), []byte("-- managed adapter\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				if initial == "existing" {
					if err := os.Mkdir(userDir, 0o755); err != nil {
						t.Fatal(err)
					}
				}
				swapped := false
				err := writeHyprLocalConfigFilesForHomeWithHook(source, hyprDir, "", func(got hyprUserWriteStage, _ *pinnedDirectory) error {
					if got != stage || swapped {
						return nil
					}
					swapped = true
					if err := os.Rename(userDir, retained); err != nil {
						return err
					}
					if err := os.Mkdir(userDir, 0o755); err != nil {
						return err
					}
					return os.WriteFile(filepath.Join(userDir, "winner"), []byte("replacement directory\n"), 0o600)
				})
				if err == nil || !strings.Contains(err.Error(), "parent directory changed") {
					t.Fatalf("user directory replacement error = %v, want pinned identity rejection", err)
				}
				if got := readTestFile(t, filepath.Join(userDir, "winner")); got != "replacement directory\n" {
					t.Fatalf("replacement marker = %q", got)
				}
				for _, name := range []string{"hyprland.lua", "default.lua"} {
					if _, statErr := os.Lstat(filepath.Join(userDir, name)); !os.IsNotExist(statErr) {
						t.Fatalf("replacement user directory received %s: %v", name, statErr)
					}
				}
				_, adapterErr := os.Lstat(filepath.Join(retained, "hyprland.lua"))
				if stage == hyprUserWriteBetweenPublications && adapterErr != nil {
					t.Fatalf("pinned adapter publication was not retained: %v", adapterErr)
				}
				if stage == hyprUserWriteAfterPin && !os.IsNotExist(adapterErr) {
					t.Fatalf("adapter was published after the first identity barrier: %v", adapterErr)
				}
				if _, statErr := os.Lstat(filepath.Join(retained, "default.lua")); !os.IsNotExist(statErr) {
					t.Fatalf("default.lua was seeded after user directory replacement: %v", statErr)
				}
			})
		}
	}
}

func TestWriteHyprLocalConfigFilesLeavesLegacyTreeOnMigratedDefaultCollision(t *testing.T) {
	source := t.TempDir()
	hyprDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "hyprland.lua"), []byte("-- managed adapter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(hyprDir, "wahrwelt")
	if err := os.MkdirAll(filepath.Join(legacy, "default.lua"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := writeHyprLocalConfigFiles(source, hyprDir)
	if err == nil || !strings.Contains(err.Error(), "non-regular Wahrwelt user config collision") {
		t.Fatalf("expected migrated default collision, got %v", err)
	}
	info, err := os.Lstat(filepath.Join(legacy, "default.lua"))
	if err != nil || !info.IsDir() {
		t.Fatalf("legacy default collision changed before rejection: info=%v err=%v", info, err)
	}
	if _, err := os.Lstat(filepath.Join(hyprDir, "user")); !os.IsNotExist(err) {
		t.Fatalf("canonical tree created before collision was rejected: %v", err)
	}
}

func TestMigrateLegacyHyprUserTreeRejectsTargetCreatedAfterPreflightWithoutOverwrite(t *testing.T) {
	hyprDir := t.TempDir()
	legacy := filepath.Join(hyprDir, "wahrwelt")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "default.lua"), []byte("-- legacy user config\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := migrateLegacyHyprUserTreeWithHook(hyprDir, func(stage hyprUserMigrationCommitStage, migration hyprUserMigration) error {
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
	user := filepath.Join(hyprDir, "user")
	if got := readTestFile(t, filepath.Join(user, "concurrent-owner.lua")); got != "-- concurrent owner\n" {
		t.Fatalf("concurrent canonical tree changed: %q", got)
	}
	if _, err := os.Lstat(filepath.Join(user, "wahrwelt")); !os.IsNotExist(err) {
		t.Fatalf("legacy tree was nested into concurrent canonical target: %v", err)
	}
}

func TestMigrateLegacyHyprUserTreeRejectsHyprParentSwapWithoutMovingVictimTree(t *testing.T) {
	root := t.TempDir()
	hyprDir := filepath.Join(root, "hypr")
	legacy := filepath.Join(hyprDir, "wahrwelt")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "default.lua"), []byte("-- legacy user config\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	movedParent := filepath.Join(root, "hypr-original")
	victimParent := filepath.Join(root, "victim")

	err := migrateLegacyHyprUserTreeWithHook(hyprDir, func(stage hyprUserMigrationCommitStage, _ hyprUserMigration) error {
		if stage != hyprUserMigrationBeforeRename {
			t.Fatalf("unexpected migration stage %q", stage)
		}
		if err := os.Rename(hyprDir, movedParent); err != nil {
			return err
		}
		if err := os.Mkdir(victimParent, 0o755); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(movedParent, "wahrwelt"), filepath.Join(victimParent, "wahrwelt")); err != nil {
			return err
		}
		return os.Symlink(victimParent, hyprDir)
	})
	if err == nil || !strings.Contains(err.Error(), "parent directory changed") {
		t.Fatalf("expected pinned Hypr parent collision, got %v", err)
	}
	if got := readTestFile(t, filepath.Join(victimParent, "wahrwelt", "default.lua")); got != "-- legacy user config\n" {
		t.Fatalf("victim legacy tree changed: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(victimParent, "user")); !os.IsNotExist(statErr) {
		t.Fatalf("victim legacy tree was renamed through swapped parent: %v", statErr)
	}
	if target, readErr := os.Readlink(hyprDir); readErr != nil || target != victimParent {
		t.Fatalf("swapped Hypr parent changed: target=%q err=%v", target, readErr)
	}
}

func TestCanonicalRuntimeUsesUserAdapterAndRecognizesOnlyExactLegacyAdapter(t *testing.T) {
	if !strings.Contains(shellruntime.CanonicalEntrypoint(), `dofile(hypr_root .. "/user/hyprland.lua")`) {
		t.Fatalf("canonical runtime must load hypr/user/hyprland.lua:\n%s", shellruntime.CanonicalEntrypoint())
	}
	if !strings.Contains(shellruntime.HomeManagerInitialCanonicalEntrypoint(), `dofile(hypr_root .. "/user/hyprland.lua")`) {
		t.Fatalf("Home Manager initial runtime must load hypr/user/hyprland.lua:\n%s", shellruntime.HomeManagerInitialCanonicalEntrypoint())
	}
	home := t.TempDir()
	legacy := filepath.Join(home, "hyprland.lua")
	legacyContent := migrationv1tov2.LegacyUserEntrypoint()
	keybinds := filepath.Join(home, "shell-keybinds.lua")
	if err := os.WriteFile(keybinds, []byte(shellruntime.AdapterMarker(shellruntime.End4PC)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	legacyHomeManagerContent := migrationv1tov2.LegacyHomeManagerUserEntrypoint(shellruntime.DefaultProfile)
	for fixtureName, fixture := range map[string]string{
		"runtime":      legacyContent,
		"home-manager": legacyHomeManagerContent,
	} {
		t.Run(fixtureName, func(t *testing.T) {
			if err := os.WriteFile(legacy, []byte(fixture), 0o644); err != nil {
				t.Fatal(err)
			}
			if migrationv1tov2.RecognizeEntrypoint(fixture, shellruntime.DefaultProfile) != migrationv1tov2.EntrypointLegacyUser {
				t.Fatal("exact generated Wahrwelt user adapter was not recognized as legacy")
			}
			if got := shellruntime.DetectShellFromEntrypoint(legacy, keybinds); got != "" {
				t.Fatalf("fresh runtime detected legacy generated adapter as %q", got)
			}
			for form, content := range map[string]string{
				"prefix":          "-- prefix\n" + fixture,
				"suffix":          fixture + "-- suffix\n",
				"missing-newline": strings.TrimSuffix(fixture, "\n"),
				"arbitrary-root":  `dofile("/tmp/arbitrary/wahrwelt/hyprland.lua")` + "\n",
			} {
				t.Run(form, func(t *testing.T) {
					if err := os.WriteFile(legacy, []byte(content), 0o644); err != nil {
						t.Fatal(err)
					}
					if migrationv1tov2.RecognizeEntrypoint(content, shellruntime.DefaultProfile) != migrationv1tov2.EntrypointUnknown {
						t.Fatalf("%s legacy fixture lookalike was accepted", form)
					}
					if got := shellruntime.DetectShellFromEntrypoint(legacy, keybinds); got != "" {
						t.Fatalf("%s legacy fixture lookalike profile = %q, want empty", form, got)
					}
				})
			}
		})
	}
}
