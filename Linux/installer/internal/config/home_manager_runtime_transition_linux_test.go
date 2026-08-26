//go:build linux

package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("WAHRWELT_TEST_LEGACY_QS_PROCESS") == "1" {
		for {
			time.Sleep(time.Hour)
		}
	}
	os.Exit(m.Run())
}

const legacyWahrweltRuntimeFixture = `-- Wahrwelt canonical Hyprland runtime entrypoint
local home = os.getenv("HOME")
if home == nil then
    error("HOME is not set; cannot locate Wahrwelt Hyprland config")
end

local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
local hypr_root = config_home .. "/hypr"
package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
dofile(hypr_root .. "/wahrwelt/hyprland.lua")
`

const canonicalUserRuntimeFixture = `-- Wahrwelt canonical Hyprland runtime entrypoint
local home = os.getenv("HOME")
if home == nil then
    error("HOME is not set; cannot locate Wahrwelt Hyprland config")
end

local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
local hypr_root = config_home .. "/hypr"
package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
dofile(hypr_root .. "/user/hyprland.lua")
`

type homeManagerRuntimeFault string

const (
	noRuntimeFault         homeManagerRuntimeFault = ""
	failHyprNamespaceMove  homeManagerRuntimeFault = "hypr-move"
	failRuntimeFinalize    homeManagerRuntimeFault = "runtime-finalize"
	failAfterRuntime       homeManagerRuntimeFault = "after-runtime"
	replacePreparedRuntime homeManagerRuntimeFault = "replace-prepared-runtime"
	failMissingBundleAsset homeManagerRuntimeFault = "missing-bundle-asset"
)

func TestRenderedHomeManagerMigrationKeepsRuntimeLoadableAtFailureBoundaries(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}

	for _, tt := range []struct {
		name             string
		fault            homeManagerRuntimeFault
		wantNamespace    string
		wantRuntime      string
		wantAdapterProbe string
	}{
		{
			name:             "failure before namespace move",
			fault:            failHyprNamespaceMove,
			wantNamespace:    "wahrwelt",
			wantRuntime:      "transition",
			wantAdapterProbe: "wahrwelt",
		},
		{
			name:             "failure immediately after namespace move",
			fault:            failRuntimeFinalize,
			wantNamespace:    "user",
			wantRuntime:      "transition",
			wantAdapterProbe: "user",
		},
		{
			name:             "later activation failure",
			fault:            failAfterRuntime,
			wantNamespace:    "user",
			wantRuntime:      "canonical",
			wantAdapterProbe: "user",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newHomeManagerRuntimeFixture(t, tt.fault, "wahrwelt", legacyWahrweltRuntimeFixture)
			output, err := fixture.runMigration()
			if err == nil {
				t.Fatalf("faulted activation unexpectedly succeeded\n%s", output)
			}
			fixture.assertNamespace(t, tt.wantNamespace)
			fixture.assertRuntime(t, tt.wantRuntime)
			fixture.assertRuntimeLoads(t, tt.wantAdapterProbe)
		})
	}
}

func TestRenderedHomeManagerMigrationRepairsStrandedCanonicalNamespace(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}

	fixture := newHomeManagerRuntimeFixture(t, noRuntimeFault, "user", legacyWahrweltRuntimeFixture)
	if output, err := fixture.runMigration(); err != nil {
		t.Fatalf("repair stranded runtime: %v\n%s", err, output)
	}
	fixture.assertNamespace(t, "user")
	fixture.assertRuntime(t, "canonical")
	fixture.assertRuntimeLoads(t, "user")
}

func TestRenderedHomeManagerMigrationPreservesUnknownRuntimeBeforeNamespaceMove(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}

	const unknown = "-- private unknown runtime\n"
	fixture := newHomeManagerRuntimeFixture(t, noRuntimeFault, "wahrwelt", unknown)
	before, err := os.Stat(fixture.runtime)
	if err != nil {
		t.Fatal(err)
	}
	output, runErr := fixture.runMigration()
	if runErr == nil || !strings.Contains(output, "unknown regular runtime entrypoint") {
		t.Fatalf("unknown runtime was accepted: err=%v\n%s", runErr, output)
	}
	after, err := os.Stat(fixture.runtime)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("unknown runtime inode changed")
	}
	if got := readContractFile(t, fixture.runtime); got != unknown {
		t.Fatalf("unknown runtime content changed: %q", got)
	}
	fixture.assertNamespace(t, "wahrwelt")
}

func TestRenderedHomeManagerMigrationPreflightsEnd4BundleAssetsBeforeMutation(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}

	direct := readContractFile(t, "../../../NixOS/home/shells/legacy-hypr-runtime/end4.lua")
	fixture := newHomeManagerRuntimeFixture(t, failMissingBundleAsset, "wahrwelt", direct)
	ancillaryPaths := make([]string, 0, 5)
	for _, name := range []string{
		"shell-profile.lua",
		"hyprlock.conf",
		"hypridle.conf",
		"shell-launcher.lua",
		"shell-keybinds.lua",
	} {
		path := filepath.Join(filepath.Dir(fixture.runtime), name)
		if err := os.WriteFile(path, []byte("private "+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		ancillaryPaths = append(ancillaryPaths, path)
	}
	paths := append([]string{fixture.runtime}, ancillaryPaths...)
	before := make(map[string]os.FileInfo, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = info
	}

	output, runErr := fixture.runMigration()
	if runErr == nil || !strings.Contains(output, "cannot open managed source") {
		t.Fatalf("missing End4 bundle asset was accepted: err=%v\n%s", runErr, output)
	}
	fixture.assertNamespace(t, "wahrwelt")
	for _, path := range paths {
		after, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(before[path], after) {
			t.Fatalf("%s identity changed after bundle asset preflight failure", path)
		}
	}
	if got := readContractFile(t, fixture.runtime); got != direct {
		t.Fatalf("direct main changed after bundle asset preflight failure:\n%s", got)
	}
}

func TestRenderedHomeManagerMigrationRejectsPreparedRuntimeReplacementBeforeMove(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}

	fixture := newHomeManagerRuntimeFixture(
		t,
		replacePreparedRuntime,
		"wahrwelt",
		legacyWahrweltRuntimeFixture,
	)
	output, runErr := fixture.runMigration()
	if runErr == nil || !strings.Contains(output, "runtime changed after transition preparation") {
		t.Fatalf("prepared runtime replacement was accepted: err=%v\n%s", runErr, output)
	}
	if got := readContractFile(t, fixture.runtime); got != "-- concurrent runtime replacement\n" {
		t.Fatalf("concurrent runtime owner changed: %q", got)
	}
	fixture.assertNamespace(t, "wahrwelt")
}

func TestRenderedHomeManagerMigrationPreservesUnknownTopLevelRuntimeBeforeNamespaceMove(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}

	fixture := newHomeManagerRuntimeFixture(t, noRuntimeFault, "wahrwelt", legacyWahrweltRuntimeFixture)
	topLevel := filepath.Join(fixture.configHome, "hypr", "hyprland.lua")
	const unknown = "-- private top-level runtime\n"
	if err := os.WriteFile(topLevel, []byte(unknown), 0o600); err != nil {
		t.Fatal(err)
	}
	topBefore, err := os.Stat(topLevel)
	if err != nil {
		t.Fatal(err)
	}
	stateBefore, err := os.Stat(fixture.runtime)
	if err != nil {
		t.Fatal(err)
	}
	output, runErr := fixture.runMigration()
	if runErr == nil || !strings.Contains(output, "top-level Hyprland runtime ownership collision") {
		t.Fatalf("unknown top-level runtime was accepted: err=%v\n%s", runErr, output)
	}
	topAfter, err := os.Stat(topLevel)
	if err != nil {
		t.Fatal(err)
	}
	stateAfter, err := os.Stat(fixture.runtime)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(topBefore, topAfter) || readContractFile(t, topLevel) != unknown {
		t.Fatal("unknown top-level runtime changed")
	}
	if !os.SameFile(stateBefore, stateAfter) || readContractFile(t, fixture.runtime) != legacyWahrweltRuntimeFixture {
		t.Fatal("state runtime changed before top-level collision was rejected")
	}
	fixture.assertNamespace(t, "wahrwelt")
}

func TestRenderedHomeManagerMigrationAcceptsExactOldGenerationStableEntrypoint(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}

	fixture := newHomeManagerRuntimeFixture(t, noRuntimeFault, "wahrwelt", legacyWahrweltRuntimeFixture)
	generation := filepath.Join(t.TempDir(), "home-manager-generation")
	filesTree := filepath.Join(t.TempDir(), "home-manager-files")
	managedTarget := filepath.Join(filesTree, ".config", "hypr", "hyprland.lua")
	if err := os.MkdirAll(filepath.Dir(managedTarget), 0o700); err != nil {
		t.Fatal(err)
	}
	stableSource := filepath.Join(t.TempDir(), "stable-hyprland.lua")
	if err := os.WriteFile(stableSource, []byte(stableHyprRuntimeFixture(fixture.runtime)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(stableSource, managedTarget); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(generation, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filesTree, filepath.Join(generation, "home-files")); err != nil {
		t.Fatal(err)
	}
	topLevel := filepath.Join(fixture.configHome, "hypr", "hyprland.lua")
	if err := os.Symlink(managedTarget, topLevel); err != nil {
		t.Fatal(err)
	}
	fixture.oldGenPath = generation

	if output, err := fixture.runMigration(); err != nil {
		t.Fatalf("migrate behind exact old-generation stable entrypoint: %v\n%s", err, output)
	}
	if got, err := os.Readlink(topLevel); err != nil || got != managedTarget {
		t.Fatalf("stable entrypoint symlink changed: target=%q err=%v", got, err)
	}
	fixture.assertNamespace(t, "user")
	fixture.assertRuntime(t, "canonical")
}

func TestRenderedHomeManagerMigrationAcceptsExactRegularStableEntrypoint(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}

	fixture := newHomeManagerRuntimeFixture(t, noRuntimeFault, "wahrwelt", legacyWahrweltRuntimeFixture)
	topLevel := filepath.Join(fixture.configHome, "hypr", "hyprland.lua")
	stable := stableHyprRuntimeFixture(fixture.runtime)
	if err := os.WriteFile(topLevel, []byte(stable), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(topLevel)
	if err != nil {
		t.Fatal(err)
	}
	if output, err := fixture.runMigration(); err != nil {
		t.Fatalf("migrate behind exact regular stable entrypoint: %v\n%s", err, output)
	}
	after, err := os.Stat(topLevel)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || readContractFile(t, topLevel) != stable {
		t.Fatal("exact regular stable entrypoint changed")
	}
	fixture.assertNamespace(t, "user")
	fixture.assertRuntime(t, "canonical")
}

func TestRenderedHomeManagerMigrationPreservesUnownedTopLevelSymlink(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}

	fixture := newHomeManagerRuntimeFixture(t, noRuntimeFault, "wahrwelt", legacyWahrweltRuntimeFixture)
	unowned := filepath.Join(t.TempDir(), "private-runtime.lua")
	if err := os.WriteFile(unowned, []byte("-- private runtime\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	topLevel := filepath.Join(fixture.configHome, "hypr", "hyprland.lua")
	if err := os.Symlink(unowned, topLevel); err != nil {
		t.Fatal(err)
	}
	output, runErr := fixture.runMigration()
	if runErr == nil || !strings.Contains(output, "top-level Hyprland runtime ownership collision") {
		t.Fatalf("unowned top-level symlink was accepted: err=%v\n%s", runErr, output)
	}
	if got, err := os.Readlink(topLevel); err != nil || got != unowned {
		t.Fatalf("unowned symlink changed: target=%q err=%v", got, err)
	}
	if got := readContractFile(t, unowned); got != "-- private runtime\n" {
		t.Fatalf("unowned symlink target changed: %q", got)
	}
	if got := readContractFile(t, fixture.runtime); got != legacyWahrweltRuntimeFixture {
		t.Fatal("state runtime changed before unowned symlink collision was rejected")
	}
	fixture.assertNamespace(t, "wahrwelt")
}

func TestRenderedHomeManagerMigrationTransitionsExactDirectTopLevelRuntime(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}

	fixture := newHomeManagerRuntimeFixture(t, failRuntimeFinalize, "wahrwelt", legacyWahrweltRuntimeFixture)
	topLevel := filepath.Join(fixture.configHome, "hypr", "hyprland.lua")
	if err := os.WriteFile(topLevel, []byte(legacyWahrweltRuntimeFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := fixture.runMigration(); err == nil {
		t.Fatalf("faulted activation unexpectedly succeeded\n%s", output)
	}
	fixture.assertNamespace(t, "user")
	transition := readContractFile(t, "../../../NixOS/home/shells/legacy-hypr-runtime/user-namespace-transition.lua")
	if got := readContractFile(t, topLevel); got != transition {
		t.Fatalf("direct top-level runtime is not interruption-safe:\n%s", got)
	}
}

func TestRenderedHomeManagerMigrationPreservesDirectEnd4UntilCoherentBundleActivation(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}

	legacyDir := "../../../NixOS/home/shells/legacy-hypr-runtime"
	for _, profile := range []string{"end4", "end4-pc"} {
		t.Run(profile, func(t *testing.T) {
			direct := readContractFile(t, filepath.Join(legacyDir, profile+".lua"))
			fixture := newHomeManagerRuntimeFixture(t, noRuntimeFault, "wahrwelt", direct)
			topLevel := filepath.Join(fixture.configHome, "hypr", "hyprland.lua")
			if err := os.WriteFile(topLevel, []byte(direct), 0o644); err != nil {
				t.Fatal(err)
			}

			if output, err := fixture.runMigration(); err != nil {
				t.Fatalf("migrate namespace behind direct %s runtime: %v\n%s", profile, err, output)
			}
			fixture.assertNamespace(t, "user")
			if got := readContractFile(t, fixture.runtime); got != direct {
				t.Fatalf("state direct %s runtime changed before coherent bundle activation:\n%s", profile, got)
			}
			if got := readContractFile(t, topLevel); got != direct {
				t.Fatalf("top-level direct %s runtime changed before coherent bundle activation:\n%s", profile, got)
			}
		})
	}
}

func TestRenderedHomeManagerMigrationStagesTopOnlyDirectEnd4BeforeStableLink(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}

	legacyDir := "../../../NixOS/home/shells/legacy-hypr-runtime"
	for _, profile := range []string{"end4", "end4-pc"} {
		for _, state := range []string{"absent", "legacy"} {
			t.Run(profile+"_state_"+state, func(t *testing.T) {
				direct := readContractFile(t, filepath.Join(legacyDir, profile+".lua"))
				fixture := newHomeManagerRuntimeFixture(
					t,
					noRuntimeFault,
					"wahrwelt",
					legacyWahrweltRuntimeFixture,
				)
				if state == "absent" {
					if err := os.Remove(fixture.runtime); err != nil {
						t.Fatal(err)
					}
				}
				topLevel := filepath.Join(fixture.configHome, "hypr", "hyprland.lua")
				if err := os.WriteFile(topLevel, []byte(direct), 0o644); err != nil {
					t.Fatal(err)
				}
				if profile == "end4-pc" {
					if err := os.WriteFile(
						filepath.Join(fixture.stateHome, "wahrwelt", "end4-variant"),
						[]byte("end4-pc\n"),
						0o600,
					); err != nil {
						t.Fatal(err)
					}
				}

				if output, err := fixture.runMigration(); err != nil {
					t.Fatalf("stage top-only %s with %s state: %v\n%s", profile, state, err, output)
				}
				fixture.assertNamespace(t, "user")
				if got := readContractFile(t, fixture.runtime); got != direct {
					t.Fatalf("state was not staged from top-only %s provenance:\n%s", profile, got)
				}
				if got := readContractFile(t, topLevel); got != direct {
					t.Fatalf("top-only %s provenance changed before stable link publication:\n%s", profile, got)
				}

				dots := absoluteTestPath(t, "../../../dots")
				installEnd4RuntimeAssetsForTest(t, fixture.configHome, dots)
				rendered := renderHomeManagerShellSeedForTest(t, fixture.configHome, fixture.stateHome, dots)
				cmd := exec.Command("bash", "-euo", "pipefail", "-c", rendered)
				cmd.Env = append(os.Environ(), "DRY_RUN_CMD=")
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("finalize staged %s bundle: %v\n%s", profile, err, output)
				}
				if got := readContractFile(t, fixture.runtime); got != canonicalUserRuntimeFixture {
					t.Fatalf("staged %s main was not finalized:\n%s", profile, got)
				}
				quickshell := "ii"
				if profile == "end4-pc" {
					quickshell = "end4-pC"
				}
				wantMarker := fmt.Sprintf(
					"-- Wahrwelt shell adapter: %s\nrequire(\"end4-adapter\").load({ profile = \"%s\", quickshell_config = \"%s\" })\n",
					profile,
					profile,
					filepath.Join(fixture.configHome, "quickshell", quickshell),
				)
				if got := readContractFile(t, filepath.Join(filepath.Dir(fixture.runtime), "shell-keybinds.lua")); got != wantMarker {
					t.Fatalf("staged %s adapter runtime = %q, want %q", profile, got, wantMarker)
				}
			})
		}
	}
}

func TestRenderedHomeManagerMigrationRecognizesHistoricalSeededRuntimes(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}

	for _, tt := range []struct {
		name      string
		namespace string
		fault     homeManagerRuntimeFault
		want      string
	}{
		{name: "legacy namespace remains loadable", namespace: "wahrwelt", fault: failHyprNamespaceMove, want: "transition"},
		{name: "stranded canonical namespace is finalized", namespace: "user", fault: noRuntimeFault, want: "canonical"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newHomeManagerRuntimeFixture(
				t,
				tt.fault,
				tt.namespace,
				historicalSeededRuntimeFixture(tt.namespace),
			)
			output, err := fixture.runMigration()
			if tt.fault == noRuntimeFault && err != nil {
				t.Fatalf("migrate historical runtime: %v\n%s", err, output)
			}
			if tt.fault != noRuntimeFault && err == nil {
				t.Fatalf("faulted activation unexpectedly succeeded\n%s", output)
			}
			fixture.assertRuntime(t, tt.want)
		})
	}
}

func TestRenderedHomeManagerMigrationDoesNotPublishBridgeBeforeNamespacePreflight(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}

	fixture := newHomeManagerRuntimeFixture(t, noRuntimeFault, "wahrwelt", legacyWahrweltRuntimeFixture)
	userDir := filepath.Join(fixture.configHome, "hypr", "user")
	if err := os.MkdirAll(userDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "hyprland.lua"), []byte("-- canonical collision\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(fixture.runtime)
	if err != nil {
		t.Fatal(err)
	}
	output, runErr := fixture.runMigration()
	if runErr == nil || !strings.Contains(output, "legacy and canonical Hypr user directories coexist") {
		t.Fatalf("coexisting namespaces were accepted: err=%v\n%s", runErr, output)
	}
	after, err := os.Stat(fixture.runtime)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || readContractFile(t, fixture.runtime) != legacyWahrweltRuntimeFixture {
		t.Fatal("runtime changed before namespace preflight rejected coexistence")
	}
}

func TestRenderedHomeManagerFreshMigrationLeavesRuntimeAbsentForSeed(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}

	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	stateHome := filepath.Join(home, ".local", "state")
	cacheHome := filepath.Join(home, ".cache")
	for _, path := range []string{configHome, stateHome, cacheHome} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	helpers := buildHomeManagerMigrationHelpers(
		t,
		noRuntimeFault,
		filepath.Join(configHome, "hypr", "wahrwelt"),
		filepath.Join(stateHome, "wahrwelt", "hypr-runtime", "hyprland.lua"),
	)
	rendered := renderHomeManagerRuntimeTransitionForTest(home, configHome, stateHome, cacheHome, helpers)
	cmd := exec.Command("bash", "-euo", "pipefail", "-c", rendered)
	cmd.Env = append(os.Environ(), "DRY_RUN_CMD=", "oldGenPath=")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fresh migration: %v\n%s", err, output)
	}
	runtime := filepath.Join(stateHome, "wahrwelt", "hypr-runtime", "hyprland.lua")
	if _, err := os.Lstat(runtime); !os.IsNotExist(err) {
		t.Fatalf("fresh migration published runtime before seed: %v", err)
	}
}

func TestHomeManagerRuntimeActivationFinalizesTransitionEntrypoint(t *testing.T) {
	dir := t.TempDir()
	runtimeDir := filepath.Join(dir, "runtime")
	if err := os.Mkdir(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	transition := "../../../NixOS/home/shells/legacy-hypr-runtime/user-namespace-transition.lua"
	transitionContent := readContractFile(t, transition)
	target := filepath.Join(runtimeDir, "hyprland.lua")
	if err := os.WriteFile(target, []byte(transitionContent), 0o644); err != nil {
		t.Fatal(err)
	}

	writeSource := func(name, content string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	canonical := writeSource("canonical.lua", canonicalUserRuntimeFixture)
	legacy := writeSource("legacy.lua", legacyWahrweltRuntimeFixture)
	profile := writeSource("shell-profile.lua", "-- profile\n")
	lock := writeSource("hyprlock.conf", "")
	idle := writeSource("hypridle.conf", "")
	launcher := writeSource("shell-launcher.lua", "-- launcher\n")
	keybinds := writeSource("shell-keybinds.lua", "-- keybinds\n")
	end4Dir := "../../../NixOS/home/shells/legacy-hypr-runtime"

	output, err := exec.Command(
		"bash",
		runtimeActivationHelper,
		"activate-runtime-dir",
		runtimeDir,
		canonical,
		legacy,
		legacy,
		filepath.Join(end4Dir, "end4.lua"),
		filepath.Join(end4Dir, "end4-pc.lua"),
		transition,
		profile,
		lock,
		idle,
		launcher,
		keybinds,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("finalize transition entrypoint: %v\n%s", err, output)
	}
	if got := readContractFile(t, target); got != canonicalUserRuntimeFixture {
		t.Fatalf("transition runtime was not finalized: %q", got)
	}
}

func TestHomeManagerRuntimeActivationMigratesDirectEnd4AsCoherentBundle(t *testing.T) {
	legacyDir := "../../../NixOS/home/shells/legacy-hypr-runtime"
	tests := []struct {
		name, directMain, variant, legacyProfile, wantProfile string
	}{
		{
			name: "missing_variant_defaults_official_with_pc_placeholders", directMain: "end4-pc",
			legacyProfile: "end4-pc", wantProfile: "end4",
		},
		{
			name: "invalid_variant_defaults_official_with_pc_placeholders", directMain: "end4-pc",
			variant: "end4-pc\ntrailing", legacyProfile: "end4-pc", wantProfile: "end4",
		},
		{
			name: "exact_pc_variant_with_official_placeholders", directMain: "end4",
			variant: "end4-pc\n", legacyProfile: "end4", wantProfile: "end4-pc",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			var legacyProcess, arbitraryProcess, canonicalProcess *exec.Cmd
			if test.name == "missing_variant_defaults_official_with_pc_placeholders" {
				activationRuntime := filepath.Join(dir, "xdg-runtime")
				if err := os.Mkdir(activationRuntime, 0o700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("XDG_RUNTIME_DIR", "")
				testBinary, err := os.Executable()
				if err != nil {
					t.Fatalf("locate Go test helper executable: %v", err)
				}
				fakeQuickShell := filepath.Join(dir, "qs-end4")
				if err := os.Symlink(testBinary, fakeQuickShell); err != nil {
					t.Fatal(err)
				}
				startProcess := func(config string, environment []string) *exec.Cmd {
					t.Helper()
					process := exec.Command(fakeQuickShell, "-n", "-d", "-c", config)
					process.Env = environment
					if err := process.Start(); err != nil {
						t.Fatalf("start fake QuickShell %s: %v", config, err)
					}
					t.Cleanup(func() {
						_ = process.Process.Kill()
						_ = process.Wait()
					})
					return process
				}
				legacyProcess = startProcess("ii", []string{
					"WAHRWELT_TEST_LEGACY_QS_PROCESS=1",
					"XDG_RUNTIME_DIR=" + activationRuntime,
				})
				arbitraryProcess = startProcess("unrelated", []string{"WAHRWELT_TEST_LEGACY_QS_PROCESS=1"})
				canonicalProcess = startProcess("ii", []string{
					"WAHRWELT_TEST_LEGACY_QS_PROCESS=1",
					"WAHRWELT_END4_PROFILE=end4",
				})
			}
			runtimeDir := filepath.Join(dir, "runtime")
			if err := os.Mkdir(runtimeDir, 0o700); err != nil {
				t.Fatal(err)
			}
			main := filepath.Join(legacyDir, test.directMain+".lua")
			if err := os.WriteFile(
				filepath.Join(runtimeDir, "hyprland.lua"),
				[]byte(readContractFile(t, main)),
				0o644,
			); err != nil {
				t.Fatal(err)
			}

			canonical := writeRuntimeBundleSource(t, dir, "canonical.lua", canonicalUserRuntimeFixture)
			official := runtimeBundleSourcesForTest(t, dir, "end4")
			pc := runtimeBundleSourcesForTest(t, dir, "end4-pc")
			selected := official
			if test.wantProfile == "end4-pc" {
				selected = pc
			}
			legacy := official
			if test.legacyProfile == "end4-pc" {
				legacy = pc
			}
			current := pc
			if test.wantProfile == "end4-pc" {
				current = official
			}
			variantPath := filepath.Join(dir, "end4-variant")
			if test.variant != "" {
				if err := os.WriteFile(variantPath, []byte(test.variant), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(
				filepath.Join(runtimeDir, "hyprlock.conf"),
				[]byte(readContractFile(t, current.lock)),
				0o600,
			); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(
				filepath.Join(runtimeDir, "shell-launcher.lua"),
				[]byte(readContractFile(t, legacy.legacyLauncher)),
				0o600,
			); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(
				filepath.Join(runtimeDir, "shell-keybinds.lua"),
				[]byte(readContractFile(t, legacy.legacyKeybinds)),
				0o600,
			); err != nil {
				t.Fatal(err)
			}

			args := make([]string, 0, 34)
			args = append(args,
				"bash", runtimeActivationHelper, "migrate-direct-end4-bundle",
				runtimeDir, filepath.Join(dir, "missing-top-provenance"), variantPath, canonical,
				filepath.Join(legacyDir, "end4.lua"),
				filepath.Join(legacyDir, "end4-pc.lua"),
			)
			args = append(args, official.args()...)
			args = append(args, pc.args()...)
			args = append(args, runtimeBundleAssetArgsForTest(t, dir, main)...)
			output, err := exec.Command(args[0], args[1:]...).CombinedOutput()
			if err != nil {
				t.Fatalf("migrate %s bundle: %v\n%s", test.wantProfile, err, output)
			}
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			protocol := lines[len(lines)-1]
			if !strings.HasPrefix(protocol, "legacy-upgrade=") {
				got := protocol
				t.Fatalf("direct End4 migration did not emit one-shot upgrade provenance: %q", got)
			}
			if legacyProcess != nil {
				if !strings.Contains(protocol, fmt.Sprintf("=%d:", legacyProcess.Process.Pid)) ||
					!strings.Contains(protocol, ":ii") {
					t.Fatalf("exact active legacy End4 process was not bound into provenance: %q", protocol)
				}
				for _, excluded := range []*exec.Cmd{arbitraryProcess, canonicalProcess} {
					if strings.Contains(protocol, fmt.Sprintf("%d:", excluded.Process.Pid)) {
						t.Fatalf("unrelated or already canonical process entered upgrade provenance: %q", protocol)
					}
				}
			}
			for name, source := range selected.desiredFiles() {
				path := filepath.Join(runtimeDir, name)
				if got, want := readContractFile(t, path), readContractFile(t, source); got != want {
					t.Fatalf("%s %s = %q, want %q", test.wantProfile, name, got, want)
				}
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != 0o644 {
					t.Fatalf("%s %s mode = %s, want 0644", test.wantProfile, name, info.Mode())
				}
			}
			if got := readContractFile(t, filepath.Join(runtimeDir, "hyprland.lua")); got != canonicalUserRuntimeFixture {
				t.Fatalf("%s main runtime was not finalized last: %q", test.wantProfile, got)
			}
			currentOutput, err := exec.Command(args[0], args[1:]...).CombinedOutput()
			if err != nil {
				t.Fatalf("recheck canonical runtime after %s migration: %v\n%s", test.wantProfile, err, currentOutput)
			}
			currentLines := strings.Split(strings.TrimSpace(string(currentOutput)), "\n")
			gotCurrent := currentLines[len(currentLines)-1]
			if legacyProcess != nil {
				wantResume := "resume-upgrade-runtime-hex=" + hex.EncodeToString([]byte(os.Getenv("XDG_RUNTIME_DIR")))
				if gotCurrent == wantResume {
					t.Fatalf("resume proof unexpectedly used inherited activation XDG runtime: %q", gotCurrent)
				}
				if !strings.HasPrefix(gotCurrent, "resume-upgrade-runtime-hex=") {
					t.Fatalf("canonical runtime lost durable upgrade resume proof: %q", gotCurrent)
				}
			} else if gotCurrent != "current" {
				t.Fatalf("canonical runtime retained one-shot legacy process provenance: %q", gotCurrent)
			}
		})
	}
}

func TestHomeManagerRuntimeActivationRejectsUnknownEnd4AncillaryBeforeMutation(t *testing.T) {
	legacyDir := "../../../NixOS/home/shells/legacy-hypr-runtime"
	dir := t.TempDir()
	runtimeDir := filepath.Join(dir, "runtime")
	if err := os.Mkdir(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(runtimeDir, "hyprland.lua")
	legacyMain := readContractFile(t, filepath.Join(legacyDir, "end4.lua"))
	if err := os.WriteFile(main, []byte(legacyMain), 0o644); err != nil {
		t.Fatal(err)
	}
	keybinds := filepath.Join(runtimeDir, "shell-keybinds.lua")
	const unknown = "-- private keybind runtime\n"
	if err := os.WriteFile(keybinds, []byte(unknown), 0o600); err != nil {
		t.Fatal(err)
	}
	canonical := writeRuntimeBundleSource(t, dir, "canonical.lua", canonicalUserRuntimeFixture)
	official := runtimeBundleSourcesForTest(t, dir, "end4")
	pc := runtimeBundleSourcesForTest(t, dir, "end4-pc")
	args := make([]string, 0, 34)
	args = append(args,
		"bash", runtimeActivationHelper, "migrate-direct-end4-bundle",
		runtimeDir, filepath.Join(dir, "missing-top-provenance"),
		filepath.Join(dir, "missing-end4-variant"), canonical,
		filepath.Join(legacyDir, "end4.lua"),
		filepath.Join(legacyDir, "end4-pc.lua"),
	)
	args = append(args, official.args()...)
	args = append(args, pc.args()...)
	args = append(args, runtimeBundleAssetArgsForTest(t, dir, main)...)
	output, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err == nil || !strings.Contains(string(output), "bundle ownership collision") {
		t.Fatalf("unknown End4 ancillary was accepted: err=%v\n%s", err, output)
	}
	if got := readContractFile(t, main); got != legacyMain {
		t.Fatalf("direct End4 main changed after ancillary preflight failure: %q", got)
	}
	if got := readContractFile(t, keybinds); got != unknown {
		t.Fatalf("unknown ancillary changed: %q", got)
	}
	for _, name := range []string{"shell-profile.lua", "hyprlock.conf", "hypridle.conf", "shell-launcher.lua"} {
		if _, statErr := os.Lstat(filepath.Join(runtimeDir, name)); !os.IsNotExist(statErr) {
			t.Fatalf("bundle mutated %s before rejecting unknown ancillary: %v", name, statErr)
		}
	}
}

func TestHomeManagerRuntimeActivationKeepsDirectMainAcrossBundleInterruption(t *testing.T) {
	legacyDir := "../../../NixOS/home/shells/legacy-hypr-runtime"
	dir := t.TempDir()
	runtimeDir := filepath.Join(dir, "runtime")
	if err := os.Mkdir(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	directMain := filepath.Join(legacyDir, "end4.lua")
	directContent := readContractFile(t, directMain)
	main := filepath.Join(runtimeDir, "hyprland.lua")
	if err := os.WriteFile(main, []byte(directContent), 0o644); err != nil {
		t.Fatal(err)
	}
	canonical := writeRuntimeBundleSource(t, dir, "canonical.lua", canonicalUserRuntimeFixture)
	official := runtimeBundleSourcesForTest(t, dir, "end4")
	pc := runtimeBundleSourcesForTest(t, dir, "end4-pc")
	for name, source := range map[string]string{
		"shell-launcher.lua": official.legacyLauncher,
		"shell-keybinds.lua": official.legacyKeybinds,
	} {
		if err := os.WriteFile(
			filepath.Join(runtimeDir, name),
			[]byte(readContractFile(t, source)),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
	}
	args := make([]string, 0, 32)
	args = append(args,
		"migrate-direct-end4-bundle",
		runtimeDir,
		filepath.Join(dir, "missing-top-provenance"),
		filepath.Join(dir, "missing-end4-variant"),
		canonical,
		filepath.Join(legacyDir, "end4.lua"),
		filepath.Join(legacyDir, "end4-pc.lua"),
	)
	args = append(args, official.args()...)
	args = append(args, pc.args()...)
	args = append(args, runtimeBundleAssetArgsForTest(t, dir, main)...)
	process := startActivationBarrier(t, "BUNDLE_MAIN", args...)

	if got := readContractFile(t, main); got != directContent {
		t.Fatalf("direct main changed before bundle commit barrier:\n%s", got)
	}
	for name, source := range official.desiredFiles() {
		if got, want := readContractFile(t, filepath.Join(runtimeDir, name)), readContractFile(t, source); got != want {
			t.Fatalf("%s was not ready before main commit: got %q, want %q", name, got, want)
		}
	}
	if err := process.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = process.continueW.Close()
	<-process.done
	process.finished = true
	if got := readContractFile(t, main); got != directContent {
		t.Fatalf("direct main changed after interrupted bundle activation:\n%s", got)
	}
}

func TestHomeManagerRuntimeActivationPersistsEnd4ProcessBeforeCanonicalCommit(t *testing.T) {
	legacyDir := "../../../NixOS/home/shells/legacy-hypr-runtime"
	dir := t.TempDir()
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	fakeQuickShell := filepath.Join(dir, "qs-end4")
	if err := os.Symlink(testBinary, fakeQuickShell); err != nil {
		t.Fatal(err)
	}
	legacyProcess := exec.Command(fakeQuickShell, "-n", "-d", "-c", "ii")
	activationRuntime := filepath.Join(dir, "xdg-runtime")
	if err := os.Mkdir(activationRuntime, 0o700); err != nil {
		t.Fatal(err)
	}
	legacyProcess.Env = []string{
		"WAHRWELT_TEST_LEGACY_QS_PROCESS=1",
		"XDG_RUNTIME_DIR=" + activationRuntime,
	}
	if err := legacyProcess.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = legacyProcess.Process.Kill()
		_ = legacyProcess.Wait()
	})

	runtimeDir := filepath.Join(dir, "runtime")
	if err := os.Mkdir(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	directMain := filepath.Join(legacyDir, "end4.lua")
	main := filepath.Join(runtimeDir, "hyprland.lua")
	if err := os.WriteFile(main, []byte(readContractFile(t, directMain)), 0o644); err != nil {
		t.Fatal(err)
	}
	canonical := writeRuntimeBundleSource(t, dir, "canonical.lua", canonicalUserRuntimeFixture)
	official := runtimeBundleSourcesForTest(t, dir, "end4")
	pc := runtimeBundleSourcesForTest(t, dir, "end4-pc")
	args := make([]string, 0, 33)
	args = append(args,
		"migrate-direct-end4-bundle",
		runtimeDir,
		filepath.Join(dir, "missing-top-provenance"),
		filepath.Join(dir, "missing-end4-variant"),
		canonical,
		filepath.Join(legacyDir, "end4.lua"),
		filepath.Join(legacyDir, "end4-pc.lua"),
	)
	args = append(args, official.args()...)
	args = append(args, pc.args()...)
	args = append(args, runtimeBundleAssetArgsForTest(t, dir, main)...)
	t.Setenv("XDG_RUNTIME_DIR", "")
	process := startActivationBarrier(t, "BUNDLE_COMMIT", args...)

	if got := readContractFile(t, main); got != canonicalUserRuntimeFixture {
		t.Fatalf("canonical main was not committed before post-commit barrier: %q", got)
	}
	persisted, err := filepath.Glob(filepath.Join(
		activationRuntime,
		"wahrwelt-end4-upgrade",
		fmt.Sprintf("%d:*:ii", legacyProcess.Process.Pid),
	))
	if err != nil || len(persisted) != 1 {
		t.Fatalf("exact legacy End4 process token was not durable before canonical commit: paths=%v err=%v", persisted, err)
	}
	if err := process.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = process.continueW.Close()
	<-process.done
	process.finished = true

	retry := exec.Command("bash", append([]string{runtimeActivationHelper}, args...)...)
	retry.Env = os.Environ()
	output, err := retry.CombinedOutput()
	if err != nil {
		t.Fatalf("retry canonical direct End4 migration: %v\n%s", err, output)
	}
	wantResume := "resume-upgrade-runtime-hex=" + hex.EncodeToString([]byte(activationRuntime)) +
		";runtime-id=" + runtimeIdentityForTest(t, activationRuntime)
	if got := strings.TrimSpace(string(output)); got != wantResume {
		t.Fatalf("post-commit retry runtime proof = %q, want %q", got, wantResume)
	}
	if _, err := os.Stat(persisted[0]); err != nil {
		t.Fatalf("post-commit retry lost durable End4 process token: %v", err)
	}
}

func TestHomeManagerRuntimeActivationExecutesOnlyWithExactRuntimeIdentity(t *testing.T) {
	dir := t.TempDir()
	runtimeDir := filepath.Join(dir, "xdg-runtime")
	if err := os.Mkdir(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	result := filepath.Join(dir, "runtime-result")
	command := filepath.Join(dir, "runtime-command")
	if err := os.WriteFile(command, []byte("#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"${XDG_RUNTIME_DIR-}\" >\"$RESULT_PATH\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	runtimeHex := hex.EncodeToString([]byte(runtimeDir))
	runtimeIdentity := runtimeIdentityForTest(t, runtimeDir)
	cmd := exec.Command(
		"bash",
		runtimeActivationHelper,
		"run-with-runtime-hex",
		runtimeHex,
		runtimeIdentity,
		command,
	)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "XDG_RUNTIME_DIR=") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	cmd.Env = append(cmd.Env, "RESULT_PATH="+result)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("exec with exact process-derived runtime: %v\n%s", err, output)
	}
	if got := readContractFile(t, result); got != runtimeDir+"\n" {
		t.Fatalf("runtime command XDG_RUNTIME_DIR = %q, want %q", got, runtimeDir+"\n")
	}

	if err := os.Remove(result); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RESULT_PATH", result)
	process := startActivationBarrier(
		t,
		"RUNTIME_EXEC",
		"run-with-runtime-hex",
		runtimeHex,
		runtimeIdentity,
		command,
	)
	savedRuntime := filepath.Join(dir, "xdg-runtime-original")
	if err := os.Rename(runtimeDir, savedRuntime); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, "replacement"), []byte("replacement runtime\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output := process.releaseExpectFailure(t)
	if !strings.Contains(output, "XDG runtime changed before exec") {
		t.Fatalf("runtime replacement did not fail with exact identity proof:\n%s", output)
	}
	if _, err := os.Stat(result); !os.IsNotExist(err) {
		t.Fatalf("runtime command executed after directory replacement: %v", err)
	}
	if got := readContractFile(t, filepath.Join(runtimeDir, "replacement")); got != "replacement runtime\n" {
		t.Fatalf("replacement runtime changed: %q", got)
	}
	if info, err := os.Stat(savedRuntime); err != nil || !info.IsDir() {
		t.Fatalf("original runtime was not preserved: info=%v err=%v", info, err)
	}
}

func TestHomeManagerRuntimeActivationRejectsMixedProcessRuntimeDirectories(t *testing.T) {
	legacyDir := "../../../NixOS/home/shells/legacy-hypr-runtime"
	dir := t.TempDir()
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	fakeQuickShell := filepath.Join(dir, "qs-end4")
	if err := os.Symlink(testBinary, fakeQuickShell); err != nil {
		t.Fatal(err)
	}
	processes := make([]*exec.Cmd, 0, 2)
	for index, config := range []string{"ii", "end4-pC"} {
		processRuntime := filepath.Join(dir, fmt.Sprintf("xdg-runtime-%d", index))
		if err := os.Mkdir(processRuntime, 0o700); err != nil {
			t.Fatal(err)
		}
		process := exec.Command(fakeQuickShell, "-n", "-d", "-c", config)
		process.Env = []string{
			"WAHRWELT_TEST_LEGACY_QS_PROCESS=1",
			"XDG_RUNTIME_DIR=" + processRuntime,
		}
		if err := process.Start(); err != nil {
			t.Fatal(err)
		}
		processes = append(processes, process)
	}
	t.Cleanup(func() {
		for _, process := range processes {
			_ = process.Process.Kill()
			_ = process.Wait()
		}
	})

	runtimeDir := filepath.Join(dir, "runtime")
	if err := os.Mkdir(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	directMain := filepath.Join(legacyDir, "end4.lua")
	directContent := readContractFile(t, directMain)
	main := filepath.Join(runtimeDir, "hyprland.lua")
	if err := os.WriteFile(main, []byte(directContent), 0o644); err != nil {
		t.Fatal(err)
	}
	canonical := writeRuntimeBundleSource(t, dir, "canonical.lua", canonicalUserRuntimeFixture)
	official := runtimeBundleSourcesForTest(t, dir, "end4")
	pc := runtimeBundleSourcesForTest(t, dir, "end4-pc")
	officialArgs := official.args()
	pcArgs := pc.args()
	assetArgs := runtimeBundleAssetArgsForTest(t, dir, main)
	args := make([]string, 0, 7+len(officialArgs)+len(pcArgs)+len(assetArgs))
	args = append(args,
		"bash", runtimeActivationHelper, "migrate-direct-end4-bundle",
		runtimeDir,
		filepath.Join(dir, "missing-top-provenance"),
		filepath.Join(dir, "missing-end4-variant"),
		canonical,
		filepath.Join(legacyDir, "end4.lua"),
		filepath.Join(legacyDir, "end4-pc.lua"),
	)
	args = append(args, officialArgs...)
	args = append(args, pcArgs...)
	args = append(args, assetArgs...)
	t.Setenv("XDG_RUNTIME_DIR", "")
	output, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err == nil || !strings.Contains(string(output), "different XDG runtime directories") {
		t.Fatalf("mixed process runtime directories were accepted: err=%v\n%s", err, output)
	}
	if got := readContractFile(t, main); got != directContent {
		t.Fatalf("direct End4 main changed after mixed runtime rejection: %q", got)
	}
}

func TestRenderedHomeManagerSeedBuildsTopOnlyEnd4BundleBeforeCanonicalMain(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}

	legacyDir := "../../../NixOS/home/shells/legacy-hypr-runtime"
	for _, test := range []struct {
		name, variant, profile, quickshell string
	}{
		{name: "missing_variant_official", profile: "end4", quickshell: "ii"},
		{name: "exact_pc_variant", variant: "end4-pc\n", profile: "end4-pc", quickshell: "end4-pC"},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			configHome := filepath.Join(home, ".config")
			stateHome := filepath.Join(home, ".local", "state")
			hyprDir := filepath.Join(configHome, "hypr")
			if err := os.MkdirAll(filepath.Join(stateHome, "wahrwelt"), 0o700); err != nil {
				t.Fatal(err)
			}
			dots := absoluteTestPath(t, "../../../dots")
			installEnd4RuntimeAssetsForTest(t, configHome, dots)
			top := filepath.Join(hyprDir, "hyprland.lua")
			if err := os.WriteFile(
				top,
				[]byte(readContractFile(t, filepath.Join(legacyDir, "end4.lua"))),
				0o644,
			); err != nil {
				t.Fatal(err)
			}
			if test.variant != "" {
				if err := os.WriteFile(
					filepath.Join(stateHome, "wahrwelt", "end4-variant"),
					[]byte(test.variant),
					0o600,
				); err != nil {
					t.Fatal(err)
				}
			}

			rendered := renderHomeManagerShellSeedForTest(t, configHome, stateHome, dots)
			cmd := exec.Command("bash", "-euo", "pipefail", "-c", rendered)
			cmd.Env = append(os.Environ(), "DRY_RUN_CMD=")
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("run rendered %s seed: %v\n%s", test.profile, err, output)
			}
			runtimeDir := filepath.Join(stateHome, "wahrwelt", "hypr-runtime")
			if got := readContractFile(t, filepath.Join(runtimeDir, "hyprland.lua")); got != canonicalUserRuntimeFixture {
				t.Fatalf("canonical main was not committed for %s:\n%s", test.profile, got)
			}
			wantKeybinds := fmt.Sprintf(
				"-- Wahrwelt shell adapter: %s\nrequire(\"end4-adapter\").load({ profile = \"%s\", quickshell_config = \"%s\" })\n",
				test.profile,
				test.profile,
				filepath.Join(configHome, "quickshell", test.quickshell),
			)
			if got := readContractFile(t, filepath.Join(runtimeDir, "shell-keybinds.lua")); got != wantKeybinds {
				t.Fatalf("%s adapter runtime = %q, want %q", test.profile, got, wantKeybinds)
			}
			wantLauncher := fmt.Sprintf(
				"-- Active shell launcher profile: %s\nrequire(\"end4.launcher\")\n",
				test.profile,
			)
			if got := readContractFile(t, filepath.Join(runtimeDir, "shell-launcher.lua")); got != wantLauncher {
				t.Fatalf("%s launcher runtime = %q, want %q", test.profile, got, wantLauncher)
			}
		})
	}
}

func TestRenderedHomeManagerLiveSyncUsesProcessRuntimeWithoutInheritedEnvironment(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}
	dir := t.TempDir()
	configHome := filepath.Join(dir, "config")
	stateHome := filepath.Join(dir, "state")
	processRuntime := filepath.Join(dir, "process-runtime")
	for _, path := range []string{configHome, stateHome, processRuntime} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	dots := absoluteTestPath(t, "../../../dots")
	installEnd4RuntimeAssetsForTest(t, configHome, dots)
	runtimeDir := filepath.Join(stateHome, "wahrwelt", "hypr-runtime")
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	direct := readContractFile(t, "../../../NixOS/home/shells/legacy-hypr-runtime/end4.lua")
	if err := os.WriteFile(filepath.Join(runtimeDir, "hyprland.lua"), []byte(direct), 0o644); err != nil {
		t.Fatal(err)
	}

	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	fakeQuickShell := filepath.Join(dir, "qs-end4")
	if err := os.Symlink(testBinary, fakeQuickShell); err != nil {
		t.Fatal(err)
	}
	legacyProcess := exec.Command(fakeQuickShell, "-n", "-d", "-c", "ii")
	legacyProcess.Env = []string{
		"WAHRWELT_TEST_LEGACY_QS_PROCESS=1",
		"XDG_RUNTIME_DIR=" + processRuntime,
	}
	if err := legacyProcess.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = legacyProcess.Process.Kill()
		_ = legacyProcess.Wait()
	})

	commandLog := filepath.Join(dir, "live-commands")
	fakeBin := filepath.Join(dir, "bin")
	if err := os.Mkdir(fakeBin, 0o700); err != nil {
		t.Fatal(err)
	}
	hyprctl := filepath.Join(fakeBin, "hyprctl")
	hyprctlScript := `#!/usr/bin/env bash
set -euo pipefail
printf 'hyprctl\t%s\t%s\n' "$XDG_RUNTIME_DIR" "$*" >>"$LIVE_COMMAND_LOG"
case "${1:-}" in
  instances | reload) ;;
  version) printf '%s\n' 'Hyprland 0.55.0' ;;
  *) exit 64 ;;
esac
`
	if err := os.WriteFile(hyprctl, []byte(hyprctlScript), 0o700); err != nil {
		t.Fatal(err)
	}
	scriptsDir := filepath.Join(configHome, "hypr", "scripts")
	if err := os.MkdirAll(scriptsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	startShell := filepath.Join(scriptsDir, "start-shell.sh")
	startShellScript := `#!/usr/bin/env bash
set -euo pipefail
shopt -s nullglob
tokens=("$XDG_RUNTIME_DIR"/wahrwelt-end4-upgrade/*:*:ii)
[ "${#tokens[@]}" -eq 1 ]
printf 'start-shell\t%s\n' "$XDG_RUNTIME_DIR" >>"$LIVE_COMMAND_LOG"
`
	if err := os.WriteFile(startShell, []byte(startShellScript), 0o700); err != nil {
		t.Fatal(err)
	}

	rendered := renderHomeManagerShellActivationForTest(t, configHome, stateHome, dots)
	cmd := exec.Command("bash", "-euo", "pipefail", "-c", rendered)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "XDG_RUNTIME_DIR=") &&
			!strings.HasPrefix(value, "PATH=") &&
			!strings.HasPrefix(value, "DRY_RUN_CMD=") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	cmd.Env = append(
		cmd.Env,
		"PATH="+fakeBin+":"+os.Getenv("PATH"),
		"DRY_RUN_CMD=",
		"LIVE_COMMAND_LOG="+commandLog,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("rendered live transition without inherited XDG runtime: %v\n%s", err, output)
	}
	wantLog := strings.Join([]string{
		"hyprctl\t" + processRuntime + "\tinstances",
		"hyprctl\t" + processRuntime + "\tversion",
		"hyprctl\t" + processRuntime + "\treload",
		"start-shell\t" + processRuntime,
		"",
	}, "\n")
	if got := readContractFile(t, commandLog); got != wantLog {
		t.Fatalf("live commands did not share the process-derived XDG runtime:\ngot:\n%s\nwant:\n%s", got, wantLog)
	}
}

func TestHomeManagerSeedRunsAfterLinkGeneration(t *testing.T) {
	shells := readContractFile(t, "../../../NixOS/home/shells/default.nix")
	start := strings.Index(shells, "seedHyprShellRuntime =")
	if start < 0 {
		t.Fatal("cannot locate seedHyprShellRuntime activation block")
	}
	end := strings.Index(shells[start:], "liveSyncHyprShell =")
	if end < 0 {
		t.Fatal("cannot locate seedHyprShellRuntime activation block")
	}
	block := shells[start : start+end]
	if !strings.Contains(block, `"linkGeneration"`) {
		t.Fatalf("seedHyprShellRuntime is not explicitly ordered after linkGeneration:\n%s", block)
	}
}

func TestHomeManagerLiveSyncConsumesDirectEnd4UpgradeProvenanceOnce(t *testing.T) {
	shells := readContractFile(t, "../../../NixOS/home/shells/default.nix")
	for _, required := range []string{
		`wahrwelt_direct_end4_process_upgrade=""`,
		`wahrwelt_direct_end4_process_runtime_hex=""`,
		`wahrwelt_direct_end4_process_runtime_id=""`,
		`migration_result="$(`,
		`legacy-upgrade=*)`,
		`wahrwelt_direct_end4_process_upgrade="''${migration_payload%%;runtime-hex=*}"`,
		`wahrwelt_direct_end4_process_runtime_hex="''${migration_runtime%%;runtime-id=*}"`,
		`wahrwelt_direct_end4_process_runtime_id="''${migration_runtime#*;runtime-id=}"`,
		`resume-upgrade-runtime-hex=*)`,
		`"${dotsRoot}/hypr/scripts/start-shell.sh"`,
		`run_live_shell_command()`,
		`"$activation_helper" run-with-runtime-hex`,
		`"$wahrwelt_direct_end4_process_runtime_id"`,
		`run_live_shell_command "$hyprctl_path" instances`,
		`run_live_shell_command "$hyprctl_path" version`,
		`run_live_shell_command "$hyprctl_path" reload`,
		`run_live_shell_command "${config.xdg.configHome}/hypr/scripts/start-shell.sh"`,
		`wahrwelt_direct_end4_process_upgrade=""`,
		`wahrwelt_direct_end4_process_runtime_hex=""`,
		`wahrwelt_direct_end4_process_runtime_id=""`,
	} {
		if !strings.Contains(shells, required) {
			t.Fatalf("Home Manager live sync is missing one-shot direct End4 provenance contract %q\n%s", required, shells)
		}
	}
	if strings.Contains(shells, "legacy-end4-process-upgrade-marker") {
		t.Fatalf("direct End4 process provenance must stay activation-local\n%s", shells)
	}
	activation := readContractFile(t, "../../../NixOS/home/shells/runtime-activation.sh")
	for _, required := range []string{
		`"--persist-end4-upgrade-processes"`,
		`persisted = subprocess.run(`,
		`if upgrade_process_tokens:`,
		`validated_process_runtime_directory(callback_runtime, label)`,
		`"XDG_RUNTIME_DIR": callback_runtime`,
		`test_barrier("BUNDLE_COMMIT")`,
		`durable_upgrade_resume_runtime(label)`,
		`"resume-upgrade-runtime-hex="`,
		`elif operation == "run-with-runtime-hex":`,
		`test_barrier("RUNTIME_EXEC")`,
	} {
		if !strings.Contains(activation, required) {
			t.Fatalf("direct End4 migration must durably persist provenance before mutation: missing %q\n%s", required, activation)
		}
	}
	persistIndex := strings.Index(activation, `persisted = subprocess.run(`)
	mutationIndex := strings.Index(activation, `ancillary_tokens = {}`)
	if persistIndex < 0 || mutationIndex < 0 || persistIndex >= mutationIndex {
		t.Fatalf("direct End4 migration must persist process provenance before its first bundle mutation")
	}
}

func TestValidatedEnd4TreePublishesRuntimeContractMarker(t *testing.T) {
	hypr := readContractFile(t, "../../../NixOS/home/end4/patches/hypr.nix")
	for _, required := range []string{
		`pkgs.writeText "end4-runtime-contract" "end4-adapter-v1\n"`,
		`${pkgs.coreutils}/bin/install -m 0444`,
		`"$out/.wahrwelt-runtime-contract"`,
		`${pkgs.diffutils}/bin/cmp -s`,
	} {
		if !strings.Contains(hypr, required) {
			t.Fatalf("validated End4 builder is missing runtime contract %q\n%s", required, hypr)
		}
	}
}

type runtimeBundleSources struct {
	profile, lock, idle, launcher, keybinds, legacyLauncher, legacyKeybinds string
}

func runtimeBundleSourcesForTest(t *testing.T, dir, profile string) runtimeBundleSources {
	t.Helper()
	prefix := strings.ReplaceAll(profile, "-", "_")
	return runtimeBundleSources{
		profile:  writeRuntimeBundleSource(t, dir, prefix+"_profile.lua", "-- profile "+profile+"\n"),
		lock:     writeRuntimeBundleSource(t, dir, prefix+"_lock.conf", "# lock "+profile+"\n"),
		idle:     writeRuntimeBundleSource(t, dir, prefix+"_idle.conf", "# idle "+profile+"\n"),
		launcher: writeRuntimeBundleSource(t, dir, prefix+"_launcher.lua", "-- launcher "+profile+"\n"),
		keybinds: writeRuntimeBundleSource(t, dir, prefix+"_keybinds.lua", "-- keybinds "+profile+"\n"),
		legacyLauncher: writeRuntimeBundleSource(
			t,
			dir,
			prefix+"_legacy_launcher.lua",
			"-- Active shell launcher profile: "+profile+"\n-- end4 registers launcher bindings from its own Hyprland Lua modules.\n",
		),
		legacyKeybinds: writeRuntimeBundleSource(
			t,
			dir,
			prefix+"_legacy_keybinds.lua",
			"-- Active shell keybind profile: "+profile+"\n-- end4 registers keybinds from its own Hyprland Lua modules.\n",
		),
	}
}

func (sources runtimeBundleSources) args() []string {
	return []string{
		sources.profile,
		sources.lock,
		sources.idle,
		sources.launcher,
		sources.keybinds,
		sources.legacyLauncher,
		sources.legacyKeybinds,
	}
}

func (sources runtimeBundleSources) desiredFiles() map[string]string {
	return map[string]string{
		"shell-profile.lua":  sources.profile,
		"hyprlock.conf":      sources.lock,
		"hypridle.conf":      sources.idle,
		"shell-launcher.lua": sources.launcher,
		"shell-keybinds.lua": sources.keybinds,
	}
}

func writeRuntimeBundleSource(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func runtimeIdentityForTest(t *testing.T, path string) string {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	value, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("runtime identity for %s is unavailable", path)
	}
	return fmt.Sprintf("%d:%d:%d:700", value.Dev, value.Ino, value.Uid)
}

func runtimeBundleAssetArgsForTest(t *testing.T, dir, main string) []string {
	t.Helper()
	asset := writeRuntimeBundleSource(t, dir, "managed-asset", "-- managed asset\n")
	contractSource := writeRuntimeBundleSource(t, dir, "runtime-contract-source", "end4-adapter-v1\n")
	contractTarget := writeRuntimeBundleSource(t, dir, "runtime-contract-target", "end4-adapter-v1\n")
	persistHelper := absoluteTestPath(t, "../../../dots/hypr/scripts/start-shell.sh")
	if err := os.Chmod(contractTarget, 0o444); err != nil {
		t.Fatal(err)
	}
	return []string{
		asset, asset, asset, contractSource,
		asset, asset, asset, main, contractTarget, asset, asset, persistHelper,
	}
}

func renderHomeManagerShellSeedForTest(t *testing.T, configHome, stateHome, dots string) string {
	t.Helper()
	modulePath := absoluteTestPath(t, "../../../NixOS/home/shells/default.nix")
	helperRoot := t.TempDir()
	bin := filepath.Join(helperRoot, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	writeMigrationTestWrapper(
		t,
		filepath.Join(bin, "wahrwelt-runtime-activation"),
		absoluteTestPath(t, runtimeActivationHelper),
	)
	expression := fmt.Sprintf(`
let
  config = {
    xdg.configHome = %q;
    xdg.stateHome = %q;
  };
  homeLibs.dotfiles.dotsRoot = builtins.toPath %q;
  lib.hm.dag.entryAfter = _: text: text;
  pkgs = {
    bash = "/usr";
    coreutils = "/usr";
    python3 = "/usr";
    util-linux = "/usr";
    writeShellApplication = _: %q;
    writeText = name: text: builtins.toFile name text;
  };
  module = import (builtins.toPath %q) { inherit config homeLibs lib pkgs; };
in module.home.activation.seedHyprShellRuntime
`, configHome, stateHome, dots, helperRoot, modulePath)
	rendered, err := exec.Command("nix", "eval", "--impure", "--raw", "--expr", expression).CombinedOutput()
	if err != nil {
		t.Fatalf("render Home Manager shell seed: %v\n%s", err, rendered)
	}
	return string(rendered)
}

func renderHomeManagerShellActivationForTest(t *testing.T, configHome, stateHome, dots string) string {
	t.Helper()
	modulePath := absoluteTestPath(t, "../../../NixOS/home/shells/default.nix")
	helperRoot := t.TempDir()
	bin := filepath.Join(helperRoot, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	writeMigrationTestWrapper(
		t,
		filepath.Join(bin, "wahrwelt-runtime-activation"),
		absoluteTestPath(t, runtimeActivationHelper),
	)
	expression := fmt.Sprintf(`
let
  config = {
    xdg.configHome = %q;
    xdg.stateHome = %q;
  };
  homeLibs.dotfiles.dotsRoot = builtins.toPath %q;
  lib.hm.dag.entryAfter = _: text: text;
  pkgs = {
    bash = "/usr";
    coreutils = "/usr";
    python3 = "/usr";
    util-linux = "/usr";
    writeShellApplication = _: %q;
    writeText = name: text: builtins.toFile name text;
  };
  module = import (builtins.toPath %q) { inherit config homeLibs lib pkgs; };
in module.home.activation.seedHyprShellRuntime + "\n" + module.home.activation.liveSyncHyprShell
`, configHome, stateHome, dots, helperRoot, modulePath)
	rendered, err := exec.Command("nix", "eval", "--impure", "--raw", "--expr", expression).CombinedOutput()
	if err != nil {
		t.Fatalf("render Home Manager shell activation: %v\n%s", err, rendered)
	}
	return string(rendered)
}

func installEnd4RuntimeAssetsForTest(t *testing.T, configHome, dots string) {
	t.Helper()
	hyprDir := filepath.Join(configHome, "hypr")
	for _, path := range []string{
		filepath.Join(hyprDir, "user"),
		filepath.Join(hyprDir, "end4"),
		filepath.Join(configHome, "quickshell", "ii"),
		filepath.Join(configHome, "quickshell", "end4-pC"),
	} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for target, source := range map[string]string{
		filepath.Join(hyprDir, "user", "hyprland.lua"): filepath.Join(dots, "hypr", "hyprland.lua"),
		filepath.Join(hyprDir, "end4-adapter.lua"):     filepath.Join(dots, "hypr", "end4-adapter.lua"),
		filepath.Join(hyprDir, "end4", "launcher.lua"): filepath.Join(dots, "hypr", "end4", "launcher.lua"),
	} {
		if err := os.WriteFile(target, []byte(readContractFile(t, source)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{
		filepath.Join(hyprDir, "end4", "hyprland.lua"),
		filepath.Join(configHome, "quickshell", "ii", "shell.qml"),
		filepath.Join(configHome, "quickshell", "end4-pC", "shell.qml"),
	} {
		if err := os.WriteFile(path, []byte("// managed runtime asset\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	contract := filepath.Join(hyprDir, "end4", ".wahrwelt-runtime-contract")
	if err := os.WriteFile(contract, []byte("end4-adapter-v1\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(contract, 0o444); err != nil {
		t.Fatal(err)
	}
}

type homeManagerRuntimeFixture struct {
	home       string
	configHome string
	stateHome  string
	cacheHome  string
	runtime    string
	helpers    string
	oldGenPath string
}

func newHomeManagerRuntimeFixture(
	t *testing.T,
	fault homeManagerRuntimeFault,
	namespace, runtimeContent string,
) *homeManagerRuntimeFixture {
	t.Helper()
	home := t.TempDir()
	fixture := &homeManagerRuntimeFixture{
		home:       home,
		configHome: filepath.Join(home, ".config"),
		stateHome:  filepath.Join(home, ".local", "state"),
		cacheHome:  filepath.Join(home, ".cache"),
	}
	fixture.runtime = filepath.Join(fixture.stateHome, "wahrwelt", "hypr-runtime", "hyprland.lua")
	for _, path := range []string{
		fixture.configHome,
		fixture.stateHome,
		fixture.cacheHome,
		filepath.Dir(fixture.runtime),
		filepath.Join(fixture.configHome, "hypr", namespace),
	} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	adapter := `local output = assert(io.open(os.getenv("RESULT_PATH"), "w"))
local source = assert(debug.getinfo(1, "S").source)
output:write(source)
output:close()
`
	if err := os.WriteFile(
		filepath.Join(fixture.configHome, "hypr", namespace, "hyprland.lua"),
		[]byte(adapter), 0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.runtime, []byte(runtimeContent), 0o644); err != nil {
		t.Fatal(err)
	}
	fixture.helpers = buildHomeManagerMigrationHelpers(
		t,
		fault,
		filepath.Join(fixture.configHome, "hypr", "wahrwelt"),
		fixture.runtime,
	)
	return fixture
}

func (fixture *homeManagerRuntimeFixture) runMigration() (string, error) {
	rendered := renderHomeManagerRuntimeTransitionForTest(
		fixture.home,
		fixture.configHome,
		fixture.stateHome,
		fixture.cacheHome,
		fixture.helpers,
	)
	cmd := exec.Command("bash", "-euo", "pipefail", "-c", rendered)
	cmd.Env = append(os.Environ(), "DRY_RUN_CMD=", "oldGenPath="+fixture.oldGenPath)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func stableHyprRuntimeFixture(target string) string {
	return fmt.Sprintf(`-- Generated by Wahrwelt: stable Hyprland Lua runtime entrypoint
local home = os.getenv("HOME")
if home == nil then
    error("HOME is not set; cannot locate Wahrwelt Hyprland runtime")
end

local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
local hypr_root = config_home .. "/hypr"
package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
dofile(%q)
`, target)
}

func historicalSeededRuntimeFixture(namespace string) string {
	return fmt.Sprintf(`-- Active Hyprland profile: wahrwelt (caelestia)
local home = os.getenv("HOME")
if home == nil then
    error("HOME is not set; cannot locate Wahrwelt Hyprland config")
end

local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
local state_home = os.getenv("XDG_STATE_HOME") or (home .. "/.local/state")
local hypr_root = config_home .. "/hypr"
local runtime_root = state_home .. "/wahrwelt/hypr-runtime"
package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
dofile(hypr_root .. "/%s/hyprland.lua")
dofile(runtime_root .. "/shell-profile.lua")
`, namespace)
}

func (fixture *homeManagerRuntimeFixture) assertNamespace(t *testing.T, want string) {
	t.Helper()
	for _, namespace := range []string{"wahrwelt", "user"} {
		_, err := os.Lstat(filepath.Join(fixture.configHome, "hypr", namespace))
		if namespace == want && err != nil {
			t.Fatalf("wanted namespace %s is unavailable: %v", namespace, err)
		}
		if namespace != want && !os.IsNotExist(err) {
			t.Fatalf("unexpected namespace %s remains: %v", namespace, err)
		}
	}
}

func (fixture *homeManagerRuntimeFixture) assertRuntime(t *testing.T, want string) {
	t.Helper()
	got := readContractFile(t, fixture.runtime)
	var wantContent string
	switch want {
	case "transition":
		wantContent = readContractFile(t, "../../../NixOS/home/shells/legacy-hypr-runtime/user-namespace-transition.lua")
	case "canonical":
		wantContent = canonicalUserRuntimeFixture
	default:
		t.Fatalf("unknown runtime expectation %q", want)
	}
	if got != wantContent {
		t.Fatalf("runtime =\n%s\nwant %s =\n%s", got, want, wantContent)
	}
}

func (fixture *homeManagerRuntimeFixture) assertRuntimeLoads(t *testing.T, want string) {
	t.Helper()
	lua, err := exec.LookPath("lua")
	if err != nil {
		t.Logf("lua interpreter is unavailable; exact runtime bytes were validated: %v", err)
		return
	}
	result := filepath.Join(t.TempDir(), "adapter-result")
	cmd := exec.Command(lua, fixture.runtime)
	cmd.Env = append(
		os.Environ(),
		"HOME="+fixture.home,
		"XDG_CONFIG_HOME="+fixture.configHome,
		"RESULT_PATH="+result,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("runtime is not loadable: %v\n%s", err, output)
	}
	wantPath := "@" + filepath.Join(fixture.configHome, "hypr", want, "hyprland.lua")
	if got := readContractFile(t, result); got != wantPath {
		t.Fatalf("runtime loaded adapter path %q, want %q", got, wantPath)
	}
}

func buildHomeManagerMigrationHelpers(
	t *testing.T,
	fault homeManagerRuntimeFault,
	legacyHypr, runtimeTarget string,
) string {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}

	for name, source := range map[string]string{
		"legacy-link-guard":     homeManagerLegacyLinkGuard,
		"legacy-cache-merge":    homeManagerLegacyCacheMerge,
		"legacy-marker-migrate": homeManagerLegacyMarkerMigrate,
	} {
		writeMigrationTestWrapper(t, filepath.Join(bin, name), absoluteTestPath(t, source))
	}
	guardMarker := filepath.Join(root, "adapter-guard-seen")
	guardWrapper := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
if [ %q = replace-prepared-runtime ]; then
  if [ -e %q ]; then
    printf '%%s\n' '-- concurrent runtime replacement' >%q
  else
    : >%q
  fi
fi
exit 0
`, string(fault), guardMarker, runtimeTarget, guardMarker)
	if err := os.WriteFile(
		filepath.Join(bin, "hypr-user-adapter-guard"),
		[]byte(guardWrapper), 0o700,
	); err != nil {
		t.Fatal(err)
	}

	namespaceSource := absoluteTestPath(t, homeManagerLegacyNamespaceMove)
	namespaceWrapper := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
if [ %q = hypr-move ] && [ "$1" = move ] && [ "$2" = %q ]; then
  echo "injected failure before Hypr namespace move" >&2
  exit 70
fi
exec bash %q "$@"
`, string(fault), legacyHypr, namespaceSource)
	if err := os.WriteFile(filepath.Join(bin, "legacy-namespace-move"), []byte(namespaceWrapper), 0o700); err != nil {
		t.Fatal(err)
	}

	runtimeSource := absoluteTestPath(t, runtimeActivationHelper)
	missingBundleAsset := filepath.Join(root, "missing-bundle-asset")
	runtimeWrapper := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
if [ %q = runtime-finalize ] && [ "$1" = migrate-known-runtime ] && grep -Fq '/user/hyprland.lua' "$3"; then
  echo "injected failure before runtime finalization" >&2
  exit 71
fi
if [ %q = missing-bundle-asset ] && [ "$1" = preflight-direct-end4-bundle ]; then
  args=("$@")
  args[22]=%q
  exec bash %q "${args[@]}"
fi
exec bash %q "$@"
`, string(fault), string(fault), missingBundleAsset, runtimeSource, runtimeSource)
	if err := os.WriteFile(filepath.Join(bin, "wahrwelt-runtime-activation"), []byte(runtimeWrapper), 0o700); err != nil {
		t.Fatal(err)
	}

	if fault == failAfterRuntime {
		cacheSource := absoluteTestPath(t, homeManagerLegacyCacheMerge)
		cacheWrapper := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
if [ "$1" = merge ]; then
  echo "injected later activation failure" >&2
  exit 72
fi
exec bash %q "$@"
`, cacheSource)
		if err := os.WriteFile(filepath.Join(bin, "legacy-cache-merge"), []byte(cacheWrapper), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func absoluteTestPath(t *testing.T, path string) string {
	t.Helper()
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return absolute
}

func renderHomeManagerRuntimeTransitionForTest(home, configHome, stateHome, cacheHome, helperRoot string) string {
	modulePath, err := filepath.Abs("../../../NixOS/home/programs/wahrwelt-migration.nix")
	if err != nil {
		panic(err)
	}
	expression := fmt.Sprintf(`
let
  config = {
    home.homeDirectory = %q;
    xdg.configHome = %q;
    xdg.stateHome = %q;
    xdg.cacheHome = %q;
  };
  lib.hm.dag.entryBefore = _: text: text;
  pkgs = {
    coreutils = "/usr";
    findutils = "/usr";
    python3 = "/usr";
    writeShellApplication = _: %q;
    writeText = name: text: builtins.toFile name text;
  };
  module = import (builtins.toPath %q) { inherit config lib pkgs; };
in module.home.activation.migrateWahrweltUserPaths
`, home, configHome, stateHome, cacheHome, helperRoot, modulePath)
	rendered, evalErr := exec.Command("nix", "eval", "--impure", "--raw", "--expr", expression).CombinedOutput()
	if evalErr != nil {
		panic(fmt.Sprintf("render Home Manager runtime transition: %v\n%s", evalErr, rendered))
	}
	return string(rendered)
}
