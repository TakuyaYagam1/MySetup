//go:build linux

package config

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const hyprUserAdapterGuard = "../../../NixOS/home/shells/hypr-user-adapter-guard.sh"

var legacyHyprUserAdapterFixtures = map[string]string{
	"user-adapter-mysetup-v1.lua":  "cecf44b96c7afd4886d498abe0de382b2574c66281a5cf78bbac06586c1b071c",
	"user-adapter-mysetup-v2.lua":  "e28d16bde1d68fa2fa43c755630284f00b3c6a14f75656e89cfb5514f8633263",
	"user-adapter-mysetup-v3.lua":  "18c3eb7f48101e0bd0b57918a683778784c74c833a215af7f7b0f1d416a0a5df",
	"user-adapter-wahrwelt-v1.lua": "24229642cd871aa3eb3d27c44b0d72357395951aec076a09d173b45ca17231a0",
}

func TestHyprUserAdapterLegacyFixturesAreExact(t *testing.T) {
	fixtureDir := "../../../NixOS/home/migrations/v1_to_v2/hypr-runtime"
	for name, wantDigest := range legacyHyprUserAdapterFixtures {
		data, err := os.ReadFile(filepath.Join(fixtureDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != wantDigest {
			t.Fatalf("%s digest = %s, want %s", name, got, wantDigest)
		}
	}
}

func TestHyprUserAdapterGuardAcceptsOnlyActiveGenerationSymlink(t *testing.T) {
	current := "../../../dots/hypr/hyprland.lua"
	legacyData := readContractFile(t, "../../../NixOS/home/migrations/v1_to_v2/hypr-runtime/user-adapter-wahrwelt-v1.lua")
	oldGeneration := t.TempDir()
	expected := filepath.Join(oldGeneration, "home-files", ".config", "hypr", "wahrwelt", "hyprland.lua")
	if err := os.MkdirAll(filepath.Dir(expected), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(expected, []byte(legacyData), 0o444); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name    string
		target  string
		wantOK  bool
		prepare func(string) error
	}{
		{
			name:   "active-generation",
			target: expected,
			wantOK: true,
		},
		{
			name:   "arbitrary-symlink",
			target: filepath.Join(t.TempDir(), "adapter.lua"),
			prepare: func(path string) error {
				return os.WriteFile(path, []byte(legacyData), 0o444)
			},
		},
		{
			name:   "broken-symlink",
			target: filepath.Join(t.TempDir(), "missing.lua"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.prepare != nil {
				if err := tc.prepare(tc.target); err != nil {
					t.Fatal(err)
				}
			}
			leaf := filepath.Join(t.TempDir(), "hyprland.lua")
			if err := os.Symlink(tc.target, leaf); err != nil {
				t.Fatal(err)
			}
			output, err := exec.Command("bash", hyprUserAdapterGuard, "check", leaf, current, oldGeneration).CombinedOutput()
			if tc.wantOK && err != nil {
				t.Fatalf("active-generation link rejected: %v\n%s", err, output)
			}
			if !tc.wantOK && (err == nil || !strings.Contains(string(output), "ownership collision")) {
				t.Fatalf("unowned link accepted: err=%v\n%s", err, output)
			}
			if got, readErr := os.Readlink(leaf); readErr != nil || got != tc.target {
				t.Fatalf("link changed: target=%q err=%v", got, readErr)
			}
		})
	}
}

func TestHyprUserAdapterGuardPreparesOnlyExactRegularFixture(t *testing.T) {
	current := "../../../dots/hypr/hyprland.lua"
	legacy := "../../../NixOS/home/migrations/v1_to_v2/hypr-runtime/user-adapter-wahrwelt-v1.lua"
	currentData := readContractFile(t, current)

	for _, tc := range []struct {
		name    string
		content string
		wantOK  bool
	}{
		{name: "exact-legacy", content: readContractFile(t, legacy), wantOK: true},
		{name: "unknown", content: "-- private user adapter\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "hyprland.lua")
			if err := os.WriteFile(target, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			output, err := exec.Command("bash", hyprUserAdapterGuard, "prepare", target, current, "").CombinedOutput()
			if tc.wantOK {
				if err != nil {
					t.Fatalf("known fixture rejected: %v\n%s", err, output)
				}
				if got := readContractFile(t, target); got != currentData {
					t.Fatalf("prepared content differs from current source")
				}
				info, statErr := os.Lstat(target)
				if statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o644 {
					t.Fatalf("prepared mode = %v err=%v", info, statErr)
				}
				return
			}
			if err == nil || !strings.Contains(string(output), "ownership collision") {
				t.Fatalf("unknown regular accepted: err=%v\n%s", err, output)
			}
			if got := readContractFile(t, target); got != tc.content {
				t.Fatalf("unknown regular changed: %q", got)
			}
		})
	}
}

func TestHyprUserAdapterGuardRejectsPostPublicationTargetSwap(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "hyprland.lua")
	legacy := readContractFile(t, "../../../NixOS/home/migrations/v1_to_v2/hypr-runtime/user-adapter-wahrwelt-v1.lua")
	current := readContractFile(t, "../../../dots/hypr/hyprland.lua")
	if err := os.WriteFile(target, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	process := startScriptBarrier(
		t,
		hyprUserAdapterGuard,
		"ADAPTER",
		"prepare",
		target,
		"../../../dots/hypr/hyprland.lua",
		"",
	)
	savedCurrent := filepath.Join(dir, "saved-current.lua")
	if err := os.Rename(target, savedCurrent); err != nil {
		t.Fatal(err)
	}
	unknown := "-- unknown adapter winner\n"
	if err := os.WriteFile(target, []byte(unknown), 0o600); err != nil {
		t.Fatal(err)
	}
	output := process.releaseExpectFailure(t)
	if !strings.Contains(output, "changed after guarded publication") {
		t.Fatalf("post-publication swap did not fail the exact target token\n%s", output)
	}
	if got := readContractFile(t, target); got != unknown {
		t.Fatalf("unknown adapter winner changed: %q", got)
	}
	if got := readContractFile(t, savedCurrent); got != current {
		t.Fatalf("displaced current adapter changed: %q", got)
	}
	if got := readContractFile(t, singleHistoricalAdapterRecovery(t, dir)); got != legacy {
		t.Fatalf("historical recovery changed: %q", got)
	}
}

func TestHyprUserAdapterGuardRejectsRecoveryDirectorySwapBeforePin(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "hyprland.lua")
	legacy := readContractFile(t, "../../../NixOS/home/migrations/v1_to_v2/hypr-runtime/user-adapter-wahrwelt-v1.lua")
	if err := os.WriteFile(target, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	process := startScriptBarrier(
		t,
		hyprUserAdapterGuard,
		"ADAPTER_RECOVERY",
		"prepare",
		target,
		"../../../dots/hypr/hyprland.lua",
		"",
	)
	matches, err := filepath.Glob(filepath.Join(dir, ".wahrwelt-hyprland-recovery.*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("created recovery directory count = %d, want 1: %v", len(matches), matches)
	}
	recovery := matches[0]
	savedRecovery := recovery + ".saved"
	if err := os.Rename(recovery, savedRecovery); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(recovery, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(recovery, "unknown")
	if err := os.WriteFile(sentinel, []byte("unknown replacement\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output := process.releaseExpectFailure(t)
	if !strings.Contains(output, "recovery changed before preparation") {
		t.Fatalf("recovery directory swap did not fail the identity bridge\n%s", output)
	}
	if got := readContractFile(t, sentinel); got != "unknown replacement\n" {
		t.Fatalf("unknown recovery replacement changed: %q", got)
	}
	entries, err := os.ReadDir(savedRecovery)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("displaced created recovery was mutated: %v", entries)
	}
	if got := readContractFile(t, target); got != legacy {
		t.Fatalf("legacy adapter changed after recovery swap: %q", got)
	}
}

func TestHomeManagerHyprUserAdapterPublicationIsGuardedAndNonForced(t *testing.T) {
	migration := readContractFile(t, "../../../NixOS/home/migrations/v1_to_v2/user-paths.nix")
	shells := readContractFile(t, "../../../NixOS/home/shells/default.nix")
	for _, want := range []string{
		`hypr-user-adapter-guard`,
		`"check" "$leaf_path"`,
	} {
		if !strings.Contains(migration, want) {
			t.Fatalf("migration is missing %q\n%s", want, migration)
		}
	}
	for _, want := range []string{
		`wahrwelt-runtime-activation`,
		`activate-user-dir`,
	} {
		if !strings.Contains(shells, want) {
			t.Fatalf("Home Manager publication is missing %q\n%s", want, shells)
		}
	}
	entryStart := strings.Index(shells, `"hypr/user/hyprland.lua" = {`)
	if entryStart < 0 {
		t.Fatalf("managed user adapter declaration is missing\n%s", shells)
	}
	entryEnd := strings.Index(shells[entryStart:], "    };")
	if entryEnd < 0 {
		t.Fatalf("managed user adapter declaration is malformed\n%s", shells)
	}
	if strings.Contains(shells[entryStart:entryStart+entryEnd], "force = true;") {
		t.Fatalf("managed user adapter must not bypass Home Manager collision checks\n%s", shells)
	}
}

func TestHomeManagerInternalNamespaceMovesCannotNestConcurrentTargets(t *testing.T) {
	migration := readContractFile(t, "../../../NixOS/home/migrations/v1_to_v2/user-paths.nix")
	helper := readContractFile(t, "../../../NixOS/home/migrations/v1_to_v2/namespace-move.py")
	for _, want := range []string{
		`legacy-namespace-move`,
		`check "$old" "$new"`,
		`move "$old" "$new" "$namespace_token"`,
	} {
		if !strings.Contains(migration, want) {
			t.Fatalf("internal namespace move is missing pinned helper contract %q\n%s", want, migration)
		}
	}
	for _, want := range []string{"RENAME_NOREPLACE", "expected_parent", "rolled back through pinned parent"} {
		if !strings.Contains(helper, want) {
			t.Fatalf("namespace helper is missing ownership invariant %q\n%s", want, helper)
		}
	}
	if strings.Contains(migration, `${pkgs.coreutils}/bin/mv -- "$old" "$new"`) {
		t.Fatalf("internal namespace migration retains direct nesting-prone mv\n%s", migration)
	}
}
