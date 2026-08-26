//go:build linux

package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const homeManagerLegacyLinkGuard = "../../../NixOS/home/programs/legacy-link-guard.sh"
const homeManagerLegacyCacheMerge = "../../../NixOS/home/programs/legacy-cache-merge.sh"
const homeManagerLegacyNamespaceMove = "../../../NixOS/home/programs/legacy-namespace-move.sh"
const homeManagerLegacyMarkerMigrate = "../../../NixOS/home/programs/legacy-marker-migrate.sh"

func runAtFDBarrier(
	t *testing.T,
	cmd *exec.Cmd,
	readyEnv, continueEnv string,
	mutate func(),
) (string, error) {
	t.Helper()
	readyR, readyW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readyR.Close()
	continueR, continueW, err := os.Pipe()
	if err != nil {
		_ = readyW.Close()
		t.Fatal(err)
	}
	defer continueW.Close()
	var output bytes.Buffer
	cmd.Env = append(os.Environ(), readyEnv+"=3", continueEnv+"=4")
	cmd.ExtraFiles = []*os.File{readyW, continueR}
	cmd.Stdout = &output
	cmd.Stderr = &output
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Start(); err != nil {
		_ = readyW.Close()
		_ = continueR.Close()
		t.Fatal(err)
	}
	_ = readyW.Close()
	_ = continueR.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	var waitErr error
	waitForExit := func() error {
		if !finished {
			waitErr = <-done
			finished = true
		}
		return waitErr
	}
	stopAndWait := func() error {
		if !finished {
			_, _ = continueW.Write([]byte{'1'})
			_ = cmd.Process.Kill()
		}
		return waitForExit()
	}
	defer func() {
		_ = stopAndWait()
	}()

	marker := make([]byte, len("ready\n"))
	ready := make(chan error, 1)
	go func() {
		_, readErr := io.ReadFull(readyR, marker)
		ready <- readErr
	}()
	select {
	case readErr := <-ready:
		if readErr != nil || string(marker) != "ready\n" {
			childErr := stopAndWait()
			t.Fatalf("helper barrier failed: marker=%q err=%v child=%v\n%s", marker, readErr, childErr, output.String())
		}
	case childErr := <-done:
		waitErr = childErr
		finished = true
		t.Fatalf("helper exited before barrier: %v\n%s", childErr, output.String())
	case <-time.After(5 * time.Second):
		childErr := stopAndWait()
		t.Fatalf("timed out waiting for helper barrier (child: %v)\n%s", childErr, output.String())
	}

	mutate()
	if _, err := continueW.Write([]byte{'1'}); err != nil {
		t.Fatal(err)
	}
	waitErr = waitForExit()
	return output.String(), waitErr
}

func legacyCachePreflightToken(t *testing.T, legacy, canonical string) string {
	t.Helper()
	output, err := exec.Command("bash", homeManagerLegacyCacheMerge, "check", legacy, canonical).CombinedOutput()
	if err != nil {
		t.Fatalf("legacy cache preflight: %v\n%s", err, output)
	}
	return strings.TrimSpace(string(output))
}

func TestHomeManagerLegacyLinkGuardRejectsUnrelatedNixStoreTarget(t *testing.T) {
	configHome := filepath.Join(t.TempDir(), ".config")
	legacyLink := filepath.Join(configHome, "hypr", "lib", "mysetup.lua")
	if err := os.MkdirAll(filepath.Dir(legacyLink), 0o700); err != nil {
		t.Fatal(err)
	}
	unrelated := "/nix/store/22222222222222222222222222222222-unrelated-object"
	if err := os.Symlink(unrelated, legacyLink); err != nil {
		t.Fatal(err)
	}

	output, err := exec.Command(
		"bash",
		homeManagerLegacyLinkGuard,
		"check",
		legacyLink,
		".config/hypr/lib/mysetup.lua",
		"",
		"",
		configHome,
	).CombinedOutput()
	if err == nil || !strings.Contains(string(output), "ownership collision") {
		t.Fatalf("unrelated Nix store target accepted: err=%v\n%s", err, output)
	}
	if got, readErr := os.Readlink(legacyLink); readErr != nil || got != unrelated {
		t.Fatalf("unrelated Nix store link changed: target=%q err=%v", got, readErr)
	}
}

func TestHomeManagerLegacyLinkGuardAcceptsAndQuarantinesExactHistoricalTarget(t *testing.T) {
	configHome := filepath.Join(t.TempDir(), ".config")
	legacyLink := filepath.Join(configHome, "quickshell", "mysetup-shell-selector")
	if err := os.MkdirAll(filepath.Dir(legacyLink), 0o700); err != nil {
		t.Fatal(err)
	}
	historical := "/nix/store/11111111111111111111111111111111-home-manager-files/.config/quickshell/mysetup-shell-selector"
	if err := os.Symlink(historical, legacyLink); err != nil {
		t.Fatal(err)
	}

	tokenOutput, err := exec.Command(
		"bash",
		homeManagerLegacyLinkGuard,
		"check",
		legacyLink,
		".config/quickshell/mysetup-shell-selector",
		"",
		"",
		configHome,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("exact historical target preflight rejected: %v\n%s", err, tokenOutput)
	}
	output, err := exec.Command(
		"bash",
		homeManagerLegacyLinkGuard,
		"quarantine",
		legacyLink,
		".config/quickshell/mysetup-shell-selector",
		"",
		"",
		configHome,
		strings.TrimSpace(string(tokenOutput)),
	).CombinedOutput()
	if err != nil {
		t.Fatalf("exact historical target rejected: %v\n%s", err, output)
	}
	if _, statErr := os.Lstat(legacyLink); !os.IsNotExist(statErr) {
		t.Fatalf("legacy managed link remains after quarantine: %v", statErr)
	}
	recovery := strings.TrimSpace(string(output))
	if !strings.HasPrefix(recovery, configHome+string(filepath.Separator)+".wahrwelt-migration-recovery-links-") {
		t.Fatalf("unexpected recovery path %q", recovery)
	}
	recoveredLink := filepath.Join(recovery, "legacy-link")
	if got, readErr := os.Readlink(recoveredLink); readErr != nil || got != historical {
		t.Fatalf("recovered link changed: target=%q err=%v", got, readErr)
	}
}

func TestHomeManagerLegacyLinkGuardPinsConfigRootBeforeTargetTraversal(t *testing.T) {
	root := t.TempDir()
	configHome := filepath.Join(root, ".config")
	originalRoot := filepath.Join(root, ".config-before-race")
	legacyRelative := filepath.Join("hypr", "lib", "mysetup.lua")
	legacyLink := filepath.Join(configHome, legacyRelative)
	historical := "/nix/store/00000000000000000000000000000000-home-manager-files/.config/hypr/lib/mysetup.lua"
	if err := os.MkdirAll(filepath.Dir(legacyLink), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(historical, legacyLink); err != nil {
		t.Fatal(err)
	}
	tokenOutput, err := exec.Command(
		"bash", homeManagerLegacyLinkGuard, "check", legacyLink,
		".config/hypr/lib/mysetup.lua", "", "", configHome,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("link guard preflight: %v\n%s", err, tokenOutput)
	}
	linkToken := strings.TrimSpace(string(tokenOutput))

	readyR, readyW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readyR.Close()
	continueR, continueW, err := os.Pipe()
	if err != nil {
		_ = readyW.Close()
		t.Fatal(err)
	}
	defer continueW.Close()
	var output bytes.Buffer
	cmd := exec.Command(
		"bash", homeManagerLegacyLinkGuard, "quarantine", legacyLink,
		".config/hypr/lib/mysetup.lua", "", "", configHome, linkToken,
	)
	cmd.Env = append(os.Environ(),
		"WAHRWELT_TEST_LINK_GUARD_READY_FD=3",
		"WAHRWELT_TEST_LINK_GUARD_CONTINUE_FD=4",
	)
	cmd.ExtraFiles = []*os.File{readyW, continueR}
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		_ = readyW.Close()
		_ = continueR.Close()
		t.Fatal(err)
	}
	_ = readyW.Close()
	_ = continueR.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	defer func() {
		if finished {
			return
		}
		_, _ = continueW.Write([]byte{'1'})
		_ = cmd.Process.Kill()
		<-done
	}()

	ready := make(chan error, 1)
	go func() {
		marker := make([]byte, len("ready\n"))
		_, readErr := io.ReadFull(readyR, marker)
		if readErr == nil && string(marker) != "ready\n" {
			readErr = fmt.Errorf("unexpected link guard barrier marker %q", marker)
		}
		ready <- readErr
	}()
	select {
	case readyErr := <-ready:
		if readyErr != nil {
			t.Fatalf("link guard did not reach root barrier: %v\n%s", readyErr, output.String())
		}
	case waitErr := <-done:
		finished = true
		t.Fatalf("link guard exited before root barrier: %v\n%s", waitErr, output.String())
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for link guard root barrier\n%s", output.String())
	}

	if err := os.Rename(configHome, originalRoot); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(configHome, legacyRelative)
	if err := os.MkdirAll(filepath.Dir(replacement), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(replacement, []byte("replacement winner\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := continueW.Write([]byte{'1'}); err != nil {
		t.Fatal(err)
	}
	waitErr := <-done
	finished = true
	if waitErr == nil || !strings.Contains(output.String(), "parent changed") {
		t.Fatalf("config root replacement was accepted: err=%v\n%s", waitErr, output.String())
	}
	if got, readErr := os.Readlink(filepath.Join(originalRoot, legacyRelative)); readErr != nil || got != historical {
		t.Fatalf("original managed link was not restored: target=%q err=%v", got, readErr)
	}
	if got := readContractFile(t, replacement); got != "replacement winner\n" {
		t.Fatalf("replacement root winner changed: %q", got)
	}
	matches, globErr := filepath.Glob(filepath.Join(configHome, ".wahrwelt-migration-recovery-links-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 0 {
		t.Fatalf("recovery escaped into replacement config root: %v", matches)
	}
}

func TestHomeManagerLegacyLinkGuardRejectsConfigRootReplacementBetweenCheckAndQuarantine(t *testing.T) {
	root := t.TempDir()
	configHome := filepath.Join(root, ".config")
	originalRoot := filepath.Join(root, ".config-before-check-race")
	relative := filepath.Join("hypr", "lib", "mysetup.lua")
	legacyLink := filepath.Join(configHome, relative)
	historical := "/nix/store/00000000000000000000000000000000-home-manager-files/.config/hypr/lib/mysetup.lua"
	if err := os.MkdirAll(filepath.Dir(legacyLink), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(historical, legacyLink); err != nil {
		t.Fatal(err)
	}
	tokenOutput, err := exec.Command(
		"bash", homeManagerLegacyLinkGuard, "check", legacyLink,
		".config/hypr/lib/mysetup.lua", "", "", configHome,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("link preflight: %v\n%s", err, tokenOutput)
	}
	if err := os.Rename(configHome, originalRoot); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(configHome, relative)
	if err := os.MkdirAll(filepath.Dir(replacement), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(historical, replacement); err != nil {
		t.Fatal(err)
	}

	output, err := exec.Command(
		"bash", homeManagerLegacyLinkGuard, "quarantine", replacement,
		".config/hypr/lib/mysetup.lua", "", "", configHome,
		strings.TrimSpace(string(tokenOutput)),
	).CombinedOutput()
	if err == nil || !strings.Contains(string(output), "changed after preflight") {
		t.Fatalf("replacement config root accepted across phases: err=%v\n%s", err, output)
	}
	for path := range map[string]struct{}{
		filepath.Join(originalRoot, relative): {},
		replacement:                           {},
	} {
		if got, readErr := os.Readlink(path); readErr != nil || got != historical {
			t.Fatalf("managed link changed at %s: target=%q err=%v", path, got, readErr)
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

func TestHomeManagerLegacyLinkGuardRestoresReplacementMovedDuringQuarantine(t *testing.T) {
	configHome := filepath.Join(t.TempDir(), ".config")
	legacyLink := filepath.Join(configHome, "quickshell", "mysetup-shell-selector")
	savedLink := filepath.Join(filepath.Dir(legacyLink), "expected-link")
	historical := "/nix/store/11111111111111111111111111111111-home-manager-files/.config/quickshell/mysetup-shell-selector"
	if err := os.MkdirAll(filepath.Dir(legacyLink), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(historical, legacyLink); err != nil {
		t.Fatal(err)
	}
	tokenOutput, err := exec.Command(
		"bash", homeManagerLegacyLinkGuard, "check", legacyLink,
		".config/quickshell/mysetup-shell-selector", "", "", configHome,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("link preflight: %v\n%s", err, tokenOutput)
	}

	cmd := exec.Command(
		"bash", homeManagerLegacyLinkGuard, "quarantine", legacyLink,
		".config/quickshell/mysetup-shell-selector", "", "", configHome,
		strings.TrimSpace(string(tokenOutput)),
	)
	const replacement = "unrelated replacement\n"
	output, waitErr := runAtFDBarrier(
		t,
		cmd,
		"WAHRWELT_TEST_LINK_MOVE_READY_FD",
		"WAHRWELT_TEST_LINK_MOVE_CONTINUE_FD",
		func() {
			if err := os.Rename(legacyLink, savedLink); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(legacyLink, []byte(replacement), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	)
	if waitErr == nil || !strings.Contains(output, "concurrent replacement restored") {
		t.Fatalf("replacement link owner was accepted: err=%v\n%s", waitErr, output)
	}
	if got := readContractFile(t, legacyLink); got != replacement {
		t.Fatalf("replacement owner changed: %q", got)
	}
	if got, err := os.Readlink(savedLink); err != nil || got != historical {
		t.Fatalf("expected historical link changed: target=%q err=%v", got, err)
	}
}

func TestHomeManagerMigrationQuarantinesProvenLegacyLinksBeforeNamespaceMoves(t *testing.T) {
	migration := readContractFile(t, "../../../NixOS/home/programs/wahrwelt-migration.nix")
	for _, want := range []string{
		`legacy-link-guard`,
		`"check" "$old_link" "$expected_relative"`,
		`quarantine_old_links`,
		`"quarantine" "$old_link" "$expected_relative"`,
	} {
		if !strings.Contains(migration, want) {
			t.Fatalf("Home Manager legacy-link ownership contract is missing %q\n%s", want, migration)
		}
	}
	quarantine := strings.Index(migration, "    quarantine_old_links\n")
	firstMove := strings.Index(migration, "    commit_hypr_user_tree\n")
	if quarantine < 0 || firstMove < 0 || quarantine > firstMove {
		t.Fatalf("legacy links must be safely quarantined before namespace mutation\n%s", migration)
	}
	if strings.Contains(migration, `$DRY_RUN_CMD ${pkgs.coreutils}/bin/rm -f -- "$old_link"`) {
		t.Fatalf("Home Manager migration retained unconditional legacy-link deletion\n%s", migration)
	}
}

func TestRenderedHomeManagerMigrationRejectsUnrelatedNixStoreBeforeMovingNamespaces(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	stateHome := filepath.Join(home, ".local", "state")
	cacheHome := filepath.Join(home, ".cache")
	legacyConfig := filepath.Join(configHome, "mysetup")
	legacyLink := filepath.Join(configHome, "hypr", "lib", "mysetup.lua")
	for _, path := range []string{legacyConfig, filepath.Dir(legacyLink), stateHome, cacheHome} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(legacyConfig, "config"), []byte("legacy config\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	unrelated := "/nix/store/22222222222222222222222222222222-unrelated-object"
	if err := os.Symlink(unrelated, legacyLink); err != nil {
		t.Fatal(err)
	}

	helperRoot := filepath.Join(t.TempDir(), "legacy-link-guard-package")
	if err := os.MkdirAll(filepath.Join(helperRoot, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	helperSource, err := filepath.Abs(homeManagerLegacyLinkGuard)
	if err != nil {
		t.Fatal(err)
	}
	helperWrapper := fmt.Sprintf("#!/usr/bin/env bash\nexec bash %q \"$@\"\n", helperSource)
	if err := os.WriteFile(filepath.Join(helperRoot, "bin", "legacy-link-guard"), []byte(helperWrapper), 0o700); err != nil {
		t.Fatal(err)
	}
	cacheHelperSource, err := filepath.Abs(homeManagerLegacyCacheMerge)
	if err != nil {
		t.Fatal(err)
	}
	cacheHelperWrapper := fmt.Sprintf("#!/usr/bin/env bash\nexec bash %q \"$@\"\n", cacheHelperSource)
	if err := os.WriteFile(filepath.Join(helperRoot, "bin", "legacy-cache-merge"), []byte(cacheHelperWrapper), 0o700); err != nil {
		t.Fatal(err)
	}
	namespaceHelperSource, err := filepath.Abs(homeManagerLegacyNamespaceMove)
	if err != nil {
		t.Fatal(err)
	}
	namespaceHelperWrapper := fmt.Sprintf("#!/usr/bin/env bash\nexec bash %q \"$@\"\n", namespaceHelperSource)
	if err := os.WriteFile(filepath.Join(helperRoot, "bin", "legacy-namespace-move"), []byte(namespaceHelperWrapper), 0o700); err != nil {
		t.Fatal(err)
	}
	rendered := renderHomeManagerMigrationForTest(t, home, configHome, stateHome, cacheHome, helperRoot)
	cmd := exec.Command("bash", "-euo", "pipefail", "-c", rendered)
	cmd.Env = append(os.Environ(), "DRY_RUN_CMD=", "oldGenPath=")
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "legacy link ownership collision") {
		t.Fatalf("rendered migration accepted unrelated Nix store link: err=%v\n%s", err, output)
	}
	if target, readErr := os.Readlink(legacyLink); readErr != nil || target != unrelated {
		t.Fatalf("rendered migration changed unrelated link: target=%q err=%v", target, readErr)
	}
	if got := readContractFile(t, filepath.Join(legacyConfig, "config")); got != "legacy config\n" {
		t.Fatalf("rendered migration moved config before link ownership failure: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(configHome, "wahrwelt")); !os.IsNotExist(statErr) {
		t.Fatalf("rendered migration published canonical config before link ownership failure: %v", statErr)
	}
}

func renderHomeManagerMigrationForTest(
	t *testing.T,
	home, configHome, stateHome, cacheHome, helperRoot string,
) string {
	t.Helper()
	runtimeHelper := filepath.Join(helperRoot, "bin", "wahrwelt-runtime-activation")
	if _, err := os.Lstat(runtimeHelper); os.IsNotExist(err) {
		writeMigrationTestWrapper(t, runtimeHelper, absoluteTestPath(t, runtimeActivationHelper))
	} else if err != nil {
		t.Fatal(err)
	}
	modulePath, err := filepath.Abs("../../../NixOS/home/programs/wahrwelt-migration.nix")
	if err != nil {
		t.Fatal(err)
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
    coreutils = %q;
    findutils = %q;
    gnugrep = %q;
    gnused = %q;
    python3 = %q;
    rsync = %q;
    writeShellApplication = args:
      if args.name == "legacy-link-guard" then %q else %q;
    writeText = name: text: builtins.toFile name text;
  };
  module = import (builtins.toPath %q) { inherit config lib pkgs; };
in module.home.activation.migrateWahrweltUserPaths
`, home, configHome, stateHome, cacheHome,
		commandPackageRoot(t, "dirname"),
		commandPackageRoot(t, "find"),
		commandPackageRoot(t, "grep"),
		commandPackageRoot(t, "sed"),
		commandPackageRoot(t, "python3"),
		commandPackageRoot(t, "rsync"),
		helperRoot, helperRoot, modulePath)
	rendered, err := exec.Command("nix", "eval", "--impure", "--raw", "--expr", expression).CombinedOutput()
	if err != nil {
		t.Fatalf("render Home Manager migration: %v\n%s", err, rendered)
	}
	return string(rendered)
}

func commandPackageRoot(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("locate %s for rendered migration: %v", name, err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		t.Fatalf("resolve %s for rendered migration: %v", name, err)
	}
	return filepath.Dir(filepath.Dir(path))
}

func homeManagerMigrationHelpers(t *testing.T) string {
	t.Helper()
	helperRoot := filepath.Join(t.TempDir(), "migration-helpers")
	if err := os.MkdirAll(filepath.Join(helperRoot, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, source := range map[string]string{
		"legacy-link-guard":     homeManagerLegacyLinkGuard,
		"legacy-cache-merge":    homeManagerLegacyCacheMerge,
		"legacy-namespace-move": homeManagerLegacyNamespaceMove,
		"legacy-marker-migrate": homeManagerLegacyMarkerMigrate,
	} {
		writeMigrationTestWrapper(t, filepath.Join(helperRoot, "bin", name), absoluteTestPath(t, source))
	}
	return helperRoot
}

func TestRenderedHomeManagerMigrationLeavesFreshXDGRuntimeRootsAbsent(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	stateHome := filepath.Join(home, ".local", "state")
	cacheHome := filepath.Join(home, ".cache")
	rendered := renderHomeManagerMigrationForTest(
		t, home, configHome, stateHome, cacheHome, homeManagerMigrationHelpers(t),
	)
	cmd := exec.Command("bash", "-euo", "pipefail", "-c", rendered)
	cmd.Env = append(os.Environ(), "DRY_RUN_CMD=", "oldGenPath=")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fresh Home Manager migration failed: %v\n%s", err, output)
	}

	for _, path := range []string{
		configHome,
		stateHome,
		cacheHome,
		filepath.Join(configHome, "hypr", "hyprland.lua"),
		filepath.Join(stateHome, "wahrwelt", "hypr-runtime", "hyprland.lua"),
	} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("fresh migration created runtime path %s: %v", path, err)
		}
	}
}

func TestRenderedHomeManagerMigrationSkipsAbsentConfigRootAndPreservesPresentRoots(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	stateHome := filepath.Join(home, ".local", "state")
	cacheHome := filepath.Join(home, ".cache")
	for _, root := range []string{stateHome, cacheHome} {
		if err := os.MkdirAll(root, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "user-owned"), []byte("keep\n"), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	stateBefore, err := os.Stat(stateHome)
	if err != nil {
		t.Fatal(err)
	}
	cacheBefore, err := os.Stat(cacheHome)
	if err != nil {
		t.Fatal(err)
	}

	rendered := renderHomeManagerMigrationForTest(
		t, home, configHome, stateHome, cacheHome, homeManagerMigrationHelpers(t),
	)
	cmd := exec.Command("bash", "-euo", "pipefail", "-c", rendered)
	cmd.Env = append(os.Environ(), "DRY_RUN_CMD=", "oldGenPath=")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("partial fresh Home Manager migration failed: %v\n%s", err, output)
	}

	if _, err := os.Lstat(configHome); !os.IsNotExist(err) {
		t.Fatalf("partial fresh migration created config root: %v", err)
	}
	for path, before := range map[string]os.FileInfo{
		stateHome: stateBefore,
		cacheHome: cacheBefore,
	} {
		after, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(before, after) {
			t.Fatalf("partial fresh migration replaced present root %s", path)
		}
		owned := filepath.Join(path, "user-owned")
		if got := readContractFile(t, owned); got != "keep\n" {
			t.Fatalf("partial fresh migration changed %s: %q", owned, got)
		}
		info, err := os.Stat(owned)
		if err != nil || info.Mode().Perm() != 0o640 {
			t.Fatalf("partial fresh migration changed %s mode: info=%v err=%v", owned, info, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(stateHome, "wahrwelt", "hypr-runtime", "hyprland.lua")); !os.IsNotExist(err) {
		t.Fatalf("partial fresh migration created state runtime: %v", err)
	}
}

func TestHomeManagerLegacyLinkGuardAbsentRecoveryRootIsPinnedAndRevalidated(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	legacyLink := filepath.Join(configHome, "hypr", "lib", "mysetup.lua")
	check := exec.Command(
		"bash", homeManagerLegacyLinkGuard, "check", legacyLink,
		".config/hypr/lib/mysetup.lua", "", "", configHome,
	)
	output, err := check.CombinedOutput()
	if err != nil {
		t.Fatalf("absent recovery root preflight failed: %v\n%s", err, output)
	}
	token := strings.TrimSpace(string(output))
	if !strings.HasPrefix(token, "absent-root|") {
		t.Fatalf("absent recovery root returned unsafe token %q", token)
	}
	if _, err := os.Lstat(configHome); !os.IsNotExist(err) {
		t.Fatalf("absent recovery root preflight created %s: %v", configHome, err)
	}

	if err := os.MkdirAll(filepath.Dir(legacyLink), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyLink, []byte("race winner\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	quarantine := exec.Command(
		"bash", homeManagerLegacyLinkGuard, "quarantine", legacyLink,
		".config/hypr/lib/mysetup.lua", "", "", configHome, token,
	)
	output, err = quarantine.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "ownership collision") {
		t.Fatalf("recovery root appearance was accepted: err=%v\n%s", err, output)
	}
	if got := readContractFile(t, legacyLink); got != "race winner\n" {
		t.Fatalf("recovery root race winner changed: %q", got)
	}
	info, err := os.Stat(legacyLink)
	if err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("recovery root race winner mode changed: info=%v err=%v", info, err)
	}
}

func TestHomeManagerLegacyLinkGuardQuarantinesExactStaleEnd4Links(t *testing.T) {
	for _, name := range []string{"monitors.conf", "workspaces.conf"} {
		t.Run(name, func(t *testing.T) {
			configHome := filepath.Join(t.TempDir(), ".config")
			legacyLink := filepath.Join(configHome, "hypr", name)
			expectedTarget := filepath.Join(configHome, "hypr", "end4", name)
			if err := os.MkdirAll(filepath.Dir(legacyLink), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(expectedTarget, legacyLink); err != nil {
				t.Fatal(err)
			}
			expectedRelative := filepath.ToSlash(filepath.Join(".config", "hypr", name))
			checkOutput, err := exec.Command(
				"bash", homeManagerLegacyLinkGuard, "check", legacyLink,
				expectedRelative, "", "", configHome,
			).CombinedOutput()
			if err != nil {
				t.Fatalf("exact stale End4 link preflight failed: %v\n%s", err, checkOutput)
			}
			quarantineOutput, err := exec.Command(
				"bash", homeManagerLegacyLinkGuard, "quarantine", legacyLink,
				expectedRelative, "", "", configHome,
				strings.TrimSpace(string(checkOutput)),
			).CombinedOutput()
			if err != nil {
				t.Fatalf("exact stale End4 link quarantine failed: %v\n%s", err, quarantineOutput)
			}
			if _, err := os.Lstat(legacyLink); !os.IsNotExist(err) {
				t.Fatalf("exact stale End4 link remains public: %v", err)
			}
			recovery := strings.TrimSpace(string(quarantineOutput))
			if got, err := os.Readlink(filepath.Join(recovery, "legacy-link")); err != nil || got != expectedTarget {
				t.Fatalf("quarantined End4 link changed: target=%q err=%v", got, err)
			}
		})
	}
}

func TestHomeManagerLegacyLinkGuardRejectsNonmatchingStaleEnd4Owners(t *testing.T) {
	tests := []struct {
		name  string
		setup func(string) error
	}{
		{
			name: "regular-file",
			setup: func(path string) error {
				return os.WriteFile(path, []byte("user-owned\n"), 0o640)
			},
		},
		{
			name: "other-absolute-symlink",
			setup: func(path string) error {
				return os.Symlink(filepath.Join(filepath.Dir(path), "custom.conf"), path)
			},
		},
		{
			name: "other-broken-symlink",
			setup: func(path string) error {
				return os.Symlink("missing-user-target", path)
			},
		},
		{
			name: "directory",
			setup: func(path string) error {
				return os.Mkdir(path, 0o750)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configHome := filepath.Join(t.TempDir(), ".config")
			legacyLink := filepath.Join(configHome, "hypr", "monitors.conf")
			if err := os.MkdirAll(filepath.Dir(legacyLink), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := test.setup(legacyLink); err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(legacyLink)
			if err != nil {
				t.Fatal(err)
			}
			beforeTarget := ""
			if before.Mode()&os.ModeSymlink != 0 {
				beforeTarget, err = os.Readlink(legacyLink)
				if err != nil {
					t.Fatal(err)
				}
			}

			output, err := exec.Command(
				"bash", homeManagerLegacyLinkGuard, "check", legacyLink,
				".config/hypr/monitors.conf", "", "", configHome,
			).CombinedOutput()
			if err == nil || !strings.Contains(string(output), "ownership collision") {
				t.Fatalf("nonmatching stale End4 owner accepted: err=%v\n%s", err, output)
			}
			after, err := os.Lstat(legacyLink)
			if err != nil {
				t.Fatalf("nonmatching stale End4 owner disappeared: %v", err)
			}
			if !os.SameFile(before, after) || before.Mode() != after.Mode() {
				t.Fatalf("nonmatching stale End4 owner changed: before=%v after=%v", before, after)
			}
			if before.Mode()&os.ModeSymlink != 0 {
				if got, err := os.Readlink(legacyLink); err != nil || got != beforeTarget {
					t.Fatalf("nonmatching stale End4 link changed: target=%q err=%v", got, err)
				}
			}
		})
	}
}

func TestRenderedHomeManagerMigrationQuarantinesExactStaleEnd4Links(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	stateHome := filepath.Join(home, ".local", "state")
	cacheHome := filepath.Join(home, ".cache")
	if err := os.MkdirAll(filepath.Join(configHome, "hypr"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{stateHome, cacheHome} {
		if err := os.MkdirAll(root, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	wantTargets := make(map[string]string)
	for _, name := range []string{"monitors.conf", "workspaces.conf"} {
		public := filepath.Join(configHome, "hypr", name)
		target := filepath.Join(configHome, "hypr", "end4", name)
		if err := os.Symlink(target, public); err != nil {
			t.Fatal(err)
		}
		wantTargets[public] = target
	}

	rendered := renderHomeManagerMigrationForTest(
		t, home, configHome, stateHome, cacheHome, homeManagerMigrationHelpers(t),
	)
	cmd := exec.Command("bash", "-euo", "pipefail", "-c", rendered)
	cmd.Env = append(os.Environ(), "DRY_RUN_CMD=", "oldGenPath=")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rendered stale End4 link migration failed: %v\n%s", err, output)
	}
	for public := range wantTargets {
		if _, err := os.Lstat(public); !os.IsNotExist(err) {
			t.Fatalf("rendered migration left stale End4 link %s: %v", public, err)
		}
	}
	recoveries, err := filepath.Glob(filepath.Join(configHome, ".wahrwelt-migration-recovery-links-*", "legacy-link"))
	if err != nil {
		t.Fatal(err)
	}
	if len(recoveries) != len(wantTargets) {
		t.Fatalf("rendered migration retained %d End4 recoveries, want %d: %v", len(recoveries), len(wantTargets), recoveries)
	}
	gotTargets := make(map[string]bool)
	for _, recovery := range recoveries {
		target, err := os.Readlink(recovery)
		if err != nil {
			t.Fatal(err)
		}
		gotTargets[target] = true
	}
	for _, target := range wantTargets {
		if !gotTargets[target] {
			t.Fatalf("rendered migration did not retain exact End4 target %s: %v", target, gotTargets)
		}
	}
}

func TestRenderedHomeManagerMigrationRejectsNonmatchingStaleEnd4LinkBeforeNamespaceMove(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	stateHome := filepath.Join(home, ".local", "state")
	cacheHome := filepath.Join(home, ".cache")
	legacyConfig := filepath.Join(configHome, "mysetup")
	legacyLink := filepath.Join(configHome, "hypr", "monitors.conf")
	for _, root := range []string{legacyConfig, filepath.Dir(legacyLink), stateHome, cacheHome} {
		if err := os.MkdirAll(root, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(legacyConfig, "user-owned"), []byte("keep\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	nonmatching := filepath.Join(configHome, "hypr", "end4", "custom-monitors.conf")
	if err := os.Symlink(nonmatching, legacyLink); err != nil {
		t.Fatal(err)
	}

	rendered := renderHomeManagerMigrationForTest(
		t, home, configHome, stateHome, cacheHome, homeManagerMigrationHelpers(t),
	)
	cmd := exec.Command("bash", "-euo", "pipefail", "-c", rendered)
	cmd.Env = append(os.Environ(), "DRY_RUN_CMD=", "oldGenPath=")
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "ownership collision") {
		t.Fatalf("rendered migration accepted nonmatching stale End4 link: err=%v\n%s", err, output)
	}
	if got, err := os.Readlink(legacyLink); err != nil || got != nonmatching {
		t.Fatalf("rendered migration changed nonmatching stale End4 link: target=%q err=%v", got, err)
	}
	if got := readContractFile(t, filepath.Join(legacyConfig, "user-owned")); got != "keep\n" {
		t.Fatalf("rendered migration moved namespace before stale End4 collision: %q", got)
	}
	if _, err := os.Lstat(filepath.Join(configHome, "wahrwelt")); !os.IsNotExist(err) {
		t.Fatalf("rendered migration published namespace before stale End4 collision: %v", err)
	}
}

func TestRenderedHomeManagerMigrationMigratesExactZenChromeMarker(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	stateHome := filepath.Join(home, ".local", "state")
	cacheHome := filepath.Join(home, ".cache")
	chrome := filepath.Join(home, ".zen", "profile.Default", "chrome")
	legacyMarker := filepath.Join(chrome, ".mysetup-managed.json")
	canonicalMarker := filepath.Join(chrome, ".wahrwelt-managed.json")
	decoyMarker := filepath.Join(home, ".zen", "profile.Default", "not-chrome", ".mysetup-managed.json")
	for _, path := range []string{configHome, stateHome, cacheHome, chrome, filepath.Dir(decoyMarker)} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	legacyContent := "{\"manager\":\"mysetup\",\"kind\":\"zen-chrome\",\"version\":2}\n"
	decoyContent := "unowned decoy marker\n"
	if err := os.WriteFile(legacyMarker, []byte(legacyContent), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(decoyMarker, []byte(decoyContent), 0o600); err != nil {
		t.Fatal(err)
	}

	helperRoot := filepath.Join(t.TempDir(), "migration-helpers")
	if err := os.MkdirAll(filepath.Join(helperRoot, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, source := range map[string]string{
		"legacy-link-guard":     homeManagerLegacyLinkGuard,
		"legacy-cache-merge":    homeManagerLegacyCacheMerge,
		"legacy-namespace-move": homeManagerLegacyNamespaceMove,
		"legacy-marker-migrate": homeManagerLegacyMarkerMigrate,
	} {
		writeMigrationTestWrapper(t, filepath.Join(helperRoot, "bin", name), absoluteTestPath(t, source))
	}

	rendered := renderHomeManagerMigrationForTest(t, home, configHome, stateHome, cacheHome, helperRoot)
	cmd := exec.Command("bash", "-euo", "pipefail", "-c", rendered)
	cmd.Env = append(os.Environ(), "DRY_RUN_CMD=", "oldGenPath=")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rendered migration rejected exact Zen marker: %v\n%s", err, output)
	}
	if _, err := os.Lstat(legacyMarker); !os.IsNotExist(err) {
		t.Fatalf("legacy Zen marker remains public: %v\n%s", err, output)
	}
	canonical := readContractFile(t, canonicalMarker)
	for _, want := range []string{`"manager": "wahrwelt"`, `"kind": "zen-chrome"`, `"version": 2`} {
		if !strings.Contains(canonical, want) {
			t.Fatalf("canonical Zen marker is missing %q: %s", want, canonical)
		}
	}
	if got := readContractFile(t, decoyMarker); got != decoyContent {
		t.Fatalf("non-production Zen marker was changed: %q", got)
	}
}

func TestRenderedHomeManagerMigrationPreservesInvalidZenChromeMarker(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	stateHome := filepath.Join(home, ".local", "state")
	cacheHome := filepath.Join(home, ".cache")
	chrome := filepath.Join(home, ".zen", "profile.Default", "chrome")
	legacyMarker := filepath.Join(chrome, ".mysetup-managed.json")
	canonicalMarker := filepath.Join(chrome, ".wahrwelt-managed.json")
	for _, path := range []string{configHome, stateHome, cacheHome, chrome} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	invalidContent := "{\"manager\":\"mysetup\",\"kind\":\"zen-chrome\",\"version\":1}\n"
	if err := os.WriteFile(legacyMarker, []byte(invalidContent), 0o640); err != nil {
		t.Fatal(err)
	}

	helperRoot := filepath.Join(t.TempDir(), "migration-helpers")
	if err := os.MkdirAll(filepath.Join(helperRoot, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, source := range map[string]string{
		"legacy-link-guard":     homeManagerLegacyLinkGuard,
		"legacy-cache-merge":    homeManagerLegacyCacheMerge,
		"legacy-namespace-move": homeManagerLegacyNamespaceMove,
		"legacy-marker-migrate": homeManagerLegacyMarkerMigrate,
	} {
		writeMigrationTestWrapper(t, filepath.Join(helperRoot, "bin", name), absoluteTestPath(t, source))
	}

	rendered := renderHomeManagerMigrationForTest(t, home, configHome, stateHome, cacheHome, helperRoot)
	cmd := exec.Command("bash", "-euo", "pipefail", "-c", rendered)
	cmd.Env = append(os.Environ(), "DRY_RUN_CMD=", "oldGenPath=")
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "marker ownership changed") {
		t.Fatalf("rendered migration accepted invalid Zen marker: err=%v\n%s", err, output)
	}
	if got := readContractFile(t, legacyMarker); got != invalidContent {
		t.Fatalf("invalid Zen marker changed after collision: %q", got)
	}
	if _, err := os.Lstat(canonicalMarker); !os.IsNotExist(err) {
		t.Fatalf("invalid Zen marker was canonicalized: %v", err)
	}
}

func TestRenderedHomeManagerMigrationFinalVerifyRejectsLateLegacyNamespace(t *testing.T) {
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
	legacyConfig := filepath.Join(configHome, "mysetup")
	canonicalConfig := filepath.Join(configHome, "wahrwelt")
	helperRoot := filepath.Join(t.TempDir(), "migration-helpers")
	if err := os.MkdirAll(filepath.Join(helperRoot, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}

	linkSource, err := filepath.Abs(homeManagerLegacyLinkGuard)
	if err != nil {
		t.Fatal(err)
	}
	cacheSource, err := filepath.Abs(homeManagerLegacyCacheMerge)
	if err != nil {
		t.Fatal(err)
	}
	namespaceSource, err := filepath.Abs(homeManagerLegacyNamespaceMove)
	if err != nil {
		t.Fatal(err)
	}
	for name, source := range map[string]string{
		"legacy-link-guard":  linkSource,
		"legacy-cache-merge": cacheSource,
	} {
		wrapper := fmt.Sprintf("#!/usr/bin/env bash\nexec bash %q \"$@\"\n", source)
		if err := os.WriteFile(filepath.Join(helperRoot, "bin", name), []byte(wrapper), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	lateMarker := filepath.Join(helperRoot, "late-created")
	namespaceWrapper := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
if [ "$1" = verify ] && [ "$2" = "$WAHRWELT_LATE_NAMESPACE" ] && [ ! -e "$WAHRWELT_LATE_MARKER" ]; then
  : >"$WAHRWELT_LATE_MARKER"
  mkdir -- "$2"
  printf 'late legacy bytes\n' >"$2/late"
fi
exec bash %q "$@"
`, namespaceSource)
	if err := os.WriteFile(
		filepath.Join(helperRoot, "bin", "legacy-namespace-move"),
		[]byte(namespaceWrapper),
		0o700,
	); err != nil {
		t.Fatal(err)
	}

	rendered := renderHomeManagerMigrationForTest(t, home, configHome, stateHome, cacheHome, helperRoot)
	cmd := exec.Command("bash", "-euo", "pipefail", "-c", rendered)
	cmd.Env = append(
		os.Environ(),
		"DRY_RUN_CMD=",
		"oldGenPath=",
		"WAHRWELT_LATE_NAMESPACE="+legacyConfig,
		"WAHRWELT_LATE_MARKER="+lateMarker,
	)
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "legacy namespace remains after migration") {
		t.Fatalf("rendered final verification accepted late legacy namespace: err=%v\n%s", err, output)
	}
	if got := readContractFile(t, filepath.Join(legacyConfig, "late")); got != "late legacy bytes\n" {
		t.Fatalf("rendered final verification changed late bytes: %q", got)
	}
	if _, statErr := os.Lstat(canonicalConfig); !os.IsNotExist(statErr) {
		t.Fatalf("rendered final verification unexpectedly published canonical config: %v", statErr)
	}
}

func TestRenderedHomeManagerMigrationFinalVerifyRejectsLateLegacyLink(t *testing.T) {
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
	legacyLink := filepath.Join(configHome, "hypr", "lib", "mysetup.lua")
	helperRoot := filepath.Join(t.TempDir(), "migration-helpers")
	if err := os.MkdirAll(filepath.Join(helperRoot, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	linkSource, err := filepath.Abs(homeManagerLegacyLinkGuard)
	if err != nil {
		t.Fatal(err)
	}
	cacheSource, err := filepath.Abs(homeManagerLegacyCacheMerge)
	if err != nil {
		t.Fatal(err)
	}
	namespaceSource, err := filepath.Abs(homeManagerLegacyNamespaceMove)
	if err != nil {
		t.Fatal(err)
	}
	for name, source := range map[string]string{
		"legacy-cache-merge":    cacheSource,
		"legacy-namespace-move": namespaceSource,
	} {
		writeMigrationTestWrapper(t, filepath.Join(helperRoot, "bin", name), source)
	}
	seenMarker := filepath.Join(helperRoot, "link-preflight-seen")
	linkWrapper := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
if [ "$1" = check ] && [ "$2" = "$WAHRWELT_LATE_LINK" ]; then
  if [ -e "$WAHRWELT_LATE_LINK_SEEN" ]; then
    mkdir -p -- "$(dirname -- "$2")"
    printf 'late link collision bytes\n' >"$2"
  else
    : >"$WAHRWELT_LATE_LINK_SEEN"
  fi
fi
exec bash %q "$@"
`, linkSource)
	if err := os.WriteFile(
		filepath.Join(helperRoot, "bin", "legacy-link-guard"), []byte(linkWrapper), 0o700,
	); err != nil {
		t.Fatal(err)
	}

	rendered := renderHomeManagerMigrationForTest(t, home, configHome, stateHome, cacheHome, helperRoot)
	cmd := exec.Command("bash", "-euo", "pipefail", "-c", rendered)
	cmd.Env = append(
		os.Environ(),
		"DRY_RUN_CMD=",
		"oldGenPath=",
		"WAHRWELT_LATE_LINK="+legacyLink,
		"WAHRWELT_LATE_LINK_SEEN="+seenMarker,
	)
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "legacy link ownership collision") {
		t.Fatalf("rendered final verification accepted late legacy link: err=%v\n%s", err, output)
	}
	if got := readContractFile(t, legacyLink); got != "late link collision bytes\n" {
		t.Fatalf("rendered final link verification changed late bytes: %q", got)
	}
}

func TestRenderedHomeManagerMigrationFinalVerifyRejectsLateLegacyMarker(t *testing.T) {
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
	lateMarker := filepath.Join(configHome, "late-app", ".mysetup-managed.json")
	helperRoot := filepath.Join(t.TempDir(), "migration-helpers")
	if err := os.MkdirAll(filepath.Join(helperRoot, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	linkSource, err := filepath.Abs(homeManagerLegacyLinkGuard)
	if err != nil {
		t.Fatal(err)
	}
	cacheSource, err := filepath.Abs(homeManagerLegacyCacheMerge)
	if err != nil {
		t.Fatal(err)
	}
	namespaceSource, err := filepath.Abs(homeManagerLegacyNamespaceMove)
	if err != nil {
		t.Fatal(err)
	}
	writeMigrationTestWrapper(t, filepath.Join(helperRoot, "bin", "legacy-link-guard"), linkSource)
	writeMigrationTestWrapper(t, filepath.Join(helperRoot, "bin", "legacy-namespace-move"), namespaceSource)
	cacheWrapper := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
if [ "$1" = verify ]; then
  bash %q "$@"
  mkdir -p -- "$(dirname -- "$WAHRWELT_LATE_MARKER")"
  printf 'late marker bytes\n' >"$WAHRWELT_LATE_MARKER"
  exit 0
fi
exec bash %q "$@"
`, cacheSource, cacheSource)
	if err := os.WriteFile(
		filepath.Join(helperRoot, "bin", "legacy-cache-merge"), []byte(cacheWrapper), 0o700,
	); err != nil {
		t.Fatal(err)
	}

	rendered := renderHomeManagerMigrationForTest(t, home, configHome, stateHome, cacheHome, helperRoot)
	cmd := exec.Command("bash", "-euo", "pipefail", "-c", rendered)
	cmd.Env = append(
		os.Environ(),
		"DRY_RUN_CMD=",
		"oldGenPath=",
		"WAHRWELT_LATE_MARKER="+lateMarker,
	)
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "legacy marker appeared after preflight") {
		markerInfo, markerErr := os.Lstat(lateMarker)
		t.Fatalf(
			"rendered final verification accepted late legacy marker: err=%v marker=%v markerErr=%v\n%s",
			err, markerInfo, markerErr, output,
		)
	}
	if got := readContractFile(t, lateMarker); got != "late marker bytes\n" {
		t.Fatalf("rendered final marker verification changed late bytes: %q", got)
	}
}

func writeMigrationTestWrapper(t *testing.T, path, source string) {
	t.Helper()
	wrapper := fmt.Sprintf("#!/usr/bin/env bash\nexec bash %q \"$@\"\n", source)
	if err := os.WriteFile(path, []byte(wrapper), 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestHomeManagerLegacyCacheQuarantinePreservesCanonicalAndRecovery(t *testing.T) {
	cacheHome := t.TempDir()
	legacy := filepath.Join(cacheHome, "mysetup")
	canonical := filepath.Join(cacheHome, "wahrwelt")
	for path, content := range map[string]string{
		filepath.Join(legacy, "legacy-only"):     "legacy\n",
		filepath.Join(legacy, "shared"):          "legacy\n",
		filepath.Join(canonical, "current-only"): "current\n",
		filepath.Join(canonical, "shared"):       "current\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	token := legacyCachePreflightToken(t, legacy, canonical)
	output, err := exec.Command("bash", homeManagerLegacyCacheMerge, "merge", legacy, canonical, cacheHome, token).CombinedOutput()
	if err != nil {
		t.Fatalf("merge legacy cache: %v\n%s", err, output)
	}
	if _, statErr := os.Lstat(legacy); !os.IsNotExist(statErr) {
		t.Fatalf("legacy cache remains after recoverable quarantine: %v", statErr)
	}
	for path, want := range map[string]string{
		filepath.Join(canonical, "current-only"): "current\n",
		filepath.Join(canonical, "shared"):       "current\n",
	} {
		if got := readContractFile(t, path); got != want {
			t.Fatalf("canonical cache %s = %q, want %q", path, got, want)
		}
	}
	if _, statErr := os.Lstat(filepath.Join(canonical, "legacy-only")); !os.IsNotExist(statErr) {
		t.Fatalf("legacy-only cache data was merged into canonical cache: %v", statErr)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	recovery := lines[len(lines)-1]
	if got := readContractFile(t, filepath.Join(recovery, "legacy-original", "shared")); got != "legacy\n" {
		t.Fatalf("legacy recovery changed: %q", got)
	}
}

func TestHomeManagerLegacyCacheQuarantinePreservesConcurrentCanonicalWinner(t *testing.T) {
	realMv, err := exec.LookPath("mv")
	if err != nil {
		t.Fatal(err)
	}
	cacheHome := t.TempDir()
	legacy := filepath.Join(cacheHome, "mysetup")
	canonical := filepath.Join(cacheHome, "wahrwelt")
	for path, content := range map[string]string{
		filepath.Join(legacy, "raced"):       "legacy bytes\n",
		filepath.Join(canonical, "existing"): "canonical bytes\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fakeBin := t.TempDir()
	fakeMv := `#!/usr/bin/env bash
set -euo pipefail
if [ ! -e "$WAHRWELT_CACHE_RACE_MARKER" ]; then
  : > "$WAHRWELT_CACHE_RACE_MARKER"
  printf 'concurrent winner\n' > "$WAHRWELT_CACHE_CANONICAL/raced"
fi
exec "$WAHRWELT_REAL_MV" "$@"
`
	if err := os.WriteFile(filepath.Join(fakeBin, "mv"), []byte(fakeMv), 0o700); err != nil {
		t.Fatal(err)
	}
	token := legacyCachePreflightToken(t, legacy, canonical)
	cmd := exec.Command("bash", homeManagerLegacyCacheMerge, "merge", legacy, canonical, cacheHome, token)
	cmd.Env = append(os.Environ(),
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"WAHRWELT_CACHE_RACE_MARKER="+filepath.Join(fakeBin, "race-fired"),
		"WAHRWELT_REAL_MV="+realMv,
		"WAHRWELT_CACHE_CANONICAL="+canonical,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cache quarantine: %v\n%s", err, output)
	}
	if got := readContractFile(t, filepath.Join(canonical, "raced")); got != "concurrent winner\n" {
		t.Fatalf("concurrent canonical winner changed: %q", got)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	recovery := lines[len(lines)-1]
	if got := readContractFile(t, filepath.Join(recovery, "legacy-original", "raced")); got != "legacy bytes\n" {
		t.Fatalf("legacy raced bytes were not recoverable: %q", got)
	}
}

func TestHomeManagerLegacyCacheMergePreservesReplacementRaceWinner(t *testing.T) {
	realMv, err := exec.LookPath("mv")
	if err != nil {
		t.Fatal(err)
	}
	cacheHome := t.TempDir()
	legacy := filepath.Join(cacheHome, "mysetup")
	canonical := filepath.Join(cacheHome, "wahrwelt")
	savedLegacy := filepath.Join(cacheHome, "legacy-before-race")
	for path, content := range map[string]string{
		filepath.Join(legacy, "legacy"):       "legacy\n",
		filepath.Join(canonical, "canonical"): "canonical\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fakeBin := t.TempDir()
	raceMarker := filepath.Join(fakeBin, "race-fired")
	fakeMv := `#!/usr/bin/env bash
set -euo pipefail
if [ ! -e "$WAHRWELT_CACHE_RACE_MARKER" ]; then
  : > "$WAHRWELT_CACHE_RACE_MARKER"
  "$WAHRWELT_REAL_MV" -T -- "$WAHRWELT_CACHE_OLD" "$WAHRWELT_CACHE_SAVED"
  mkdir -- "$WAHRWELT_CACHE_OLD"
  printf 'winner\n' > "$WAHRWELT_CACHE_OLD/winner"
fi
exec "$WAHRWELT_REAL_MV" "$@"
`
	if err := os.WriteFile(filepath.Join(fakeBin, "mv"), []byte(fakeMv), 0o700); err != nil {
		t.Fatal(err)
	}
	token := legacyCachePreflightToken(t, legacy, canonical)
	cmd := exec.Command("bash", homeManagerLegacyCacheMerge, "merge", legacy, canonical, cacheHome, token)
	cmd.Env = append(os.Environ(),
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"WAHRWELT_CACHE_RACE_MARKER="+raceMarker,
		"WAHRWELT_REAL_MV="+realMv,
		"WAHRWELT_CACHE_OLD="+legacy,
		"WAHRWELT_CACHE_SAVED="+savedLegacy,
	)
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "changed during quarantine") {
		t.Fatalf("cache replacement race was accepted: err=%v\n%s", err, output)
	}
	if got := readContractFile(t, filepath.Join(legacy, "winner")); got != "winner\n" {
		t.Fatalf("cache race winner changed: %q", got)
	}
	if got := readContractFile(t, filepath.Join(savedLegacy, "legacy")); got != "legacy\n" {
		t.Fatalf("legacy cache disappeared during race: %q", got)
	}
	if got := readContractFile(t, filepath.Join(canonical, "canonical")); got != "canonical\n" {
		t.Fatalf("canonical cache changed before race rejection: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(canonical, "winner")); !os.IsNotExist(statErr) {
		t.Fatalf("race winner leaked into canonical cache: %v", statErr)
	}
}

func TestHomeManagerLegacyCacheMoveReportsReplacementRecoveryWhenRestoreIsBlocked(t *testing.T) {
	realMv, err := exec.LookPath("mv")
	if err != nil {
		t.Fatal(err)
	}
	cacheHome := t.TempDir()
	legacy := filepath.Join(cacheHome, "mysetup")
	canonical := filepath.Join(cacheHome, "wahrwelt")
	savedLegacy := filepath.Join(cacheHome, "expected-legacy")
	if err := os.Mkdir(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "legacy"), []byte("expected legacy\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fakeBin := t.TempDir()
	fakeMv := `#!/usr/bin/env bash
set -euo pipefail
if [ ! -e "$WAHRWELT_CACHE_RACE_MARKER" ]; then
  : >"$WAHRWELT_CACHE_RACE_MARKER"
  "$WAHRWELT_REAL_MV" -T -- "$WAHRWELT_CACHE_OLD" "$WAHRWELT_CACHE_SAVED"
  mkdir -- "$WAHRWELT_CACHE_OLD"
  printf '%s\n' 'moved replacement' >"$WAHRWELT_CACHE_OLD/winner"
  "$WAHRWELT_REAL_MV" "$@"
  mkdir -- "$WAHRWELT_CACHE_OLD"
  printf '%s\n' 'second source owner' >"$WAHRWELT_CACHE_OLD/second"
  exit 0
fi
exec "$WAHRWELT_REAL_MV" "$@"
`
	if err := os.WriteFile(filepath.Join(fakeBin, "mv"), []byte(fakeMv), 0o700); err != nil {
		t.Fatal(err)
	}
	token := legacyCachePreflightToken(t, legacy, canonical)
	cmd := exec.Command("bash", homeManagerLegacyCacheMerge, "merge", legacy, canonical, cacheHome, token)
	cmd.Env = append(
		os.Environ(),
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"WAHRWELT_REAL_MV="+realMv,
		"WAHRWELT_CACHE_RACE_MARKER="+filepath.Join(fakeBin, "race-fired"),
		"WAHRWELT_CACHE_OLD="+legacy,
		"WAHRWELT_CACHE_SAVED="+savedLegacy,
	)
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "recovery retained at "+canonical) {
		t.Fatalf("cache replacement recovery was not reported: err=%v\n%s", err, output)
	}
	for path, want := range map[string]string{
		filepath.Join(canonical, "winner"):   "moved replacement\n",
		filepath.Join(legacy, "second"):      "second source owner\n",
		filepath.Join(savedLegacy, "legacy"): "expected legacy\n",
	} {
		if got := readContractFile(t, path); got != want {
			t.Fatalf("cache owner changed at %s: %q", path, got)
		}
	}
}

func TestHomeManagerLegacyCacheMergeRejectsParentReplacementBetweenCheckAndMerge(t *testing.T) {
	root := t.TempDir()
	cacheHome := filepath.Join(root, "cache")
	originalCache := filepath.Join(root, "cache-before-race")
	legacy := filepath.Join(cacheHome, "mysetup")
	canonical := filepath.Join(cacheHome, "wahrwelt")
	for path, content := range map[string]string{
		filepath.Join(legacy, "legacy"):       "original legacy\n",
		filepath.Join(canonical, "canonical"): "original canonical\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	token := legacyCachePreflightToken(t, legacy, canonical)
	if err := os.Rename(cacheHome, originalCache); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(legacy, "winner"):    "replacement legacy\n",
		filepath.Join(canonical, "winner"): "replacement canonical\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	output, err := exec.Command(
		"bash", homeManagerLegacyCacheMerge, "merge", legacy, canonical, cacheHome, token,
	).CombinedOutput()
	if err == nil || !strings.Contains(string(output), "cache paths must be ordinary directories") {
		t.Fatalf("replacement cache parent accepted across phases: err=%v\n%s", err, output)
	}
	for path, want := range map[string]string{
		filepath.Join(originalCache, "mysetup", "legacy"):     "original legacy\n",
		filepath.Join(originalCache, "wahrwelt", "canonical"): "original canonical\n",
		filepath.Join(cacheHome, "mysetup", "winner"):         "replacement legacy\n",
		filepath.Join(cacheHome, "wahrwelt", "winner"):        "replacement canonical\n",
	} {
		if got := readContractFile(t, path); got != want {
			t.Fatalf("cache tree changed at %s: %q", path, got)
		}
	}
	matches, globErr := filepath.Glob(filepath.Join(cacheHome, ".wahrwelt-migration-recovery-cache-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 0 {
		t.Fatalf("recovery created in replacement cache root: %v", matches)
	}
}

func TestHomeManagerLegacyCacheRejectsRecoveryCandidateReplacement(t *testing.T) {
	cacheHome := t.TempDir()
	legacy := filepath.Join(cacheHome, "mysetup")
	canonical := filepath.Join(cacheHome, "wahrwelt")
	for path, content := range map[string]string{
		filepath.Join(legacy, "legacy"):       "legacy bytes\n",
		filepath.Join(canonical, "canonical"): "canonical bytes\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	token := legacyCachePreflightToken(t, legacy, canonical)
	cmd := exec.Command("bash", homeManagerLegacyCacheMerge, "merge", legacy, canonical, cacheHome, token)
	var created, recovery string
	output, waitErr := runAtFDBarrier(
		t,
		cmd,
		"WAHRWELT_TEST_CACHE_RECOVERY_CREATED_READY_FD",
		"WAHRWELT_TEST_CACHE_RECOVERY_CREATED_CONTINUE_FD",
		func() {
			matches, err := filepath.Glob(filepath.Join(cacheHome, ".wahrwelt-migration-recovery-cache-*"))
			if err != nil {
				t.Fatal(err)
			}
			if len(matches) != 1 {
				t.Fatalf("cache creator barrier has %d candidates, want 1: %v", len(matches), matches)
			}
			created = matches[0]
			recovery = created + ".expected-recovery"
			if err := os.Rename(created, recovery); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(created, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(created, "unknown-winner"), []byte("unknown winner\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	)
	if waitErr == nil {
		t.Fatalf("cache recovery replacement unexpectedly succeeded\n%s", output)
	}
	if !strings.Contains(output, "candidate retained at "+recovery) ||
		!strings.Contains(output, "unknown collision preserved at "+created) {
		t.Fatalf("cache recovery replacement did not report exact owners\n%s", output)
	}
	for path, want := range map[string]string{
		filepath.Join(legacy, "legacy"):          "legacy bytes\n",
		filepath.Join(canonical, "canonical"):    "canonical bytes\n",
		filepath.Join(created, "unknown-winner"): "unknown winner\n",
	} {
		if got := readContractFile(t, path); got != want {
			t.Fatalf("cache recovery race changed %s: %q", path, got)
		}
	}
	if info, err := os.Lstat(recovery); err != nil || !info.IsDir() {
		t.Fatalf("exact empty recovery candidate was not retained: info=%v err=%v", info, err)
	}
}

func TestHomeManagerLegacyCacheReportsPinnedRecoveryAfterPublicNameReplacement(t *testing.T) {
	cacheHome := t.TempDir()
	legacy := filepath.Join(cacheHome, "mysetup")
	canonical := filepath.Join(cacheHome, "wahrwelt")
	for path, content := range map[string]string{
		filepath.Join(legacy, "legacy"):       "legacy bytes\n",
		filepath.Join(canonical, "canonical"): "canonical bytes\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	token := legacyCachePreflightToken(t, legacy, canonical)
	cmd := exec.Command("bash", homeManagerLegacyCacheMerge, "merge", legacy, canonical, cacheHome, token)
	var publicName, retained string
	output, waitErr := runAtFDBarrier(
		t,
		cmd,
		"WAHRWELT_TEST_CACHE_QUARANTINED_READY_FD",
		"WAHRWELT_TEST_CACHE_QUARANTINED_CONTINUE_FD",
		func() {
			matches, err := filepath.Glob(filepath.Join(cacheHome, ".wahrwelt-migration-recovery-cache-*"))
			if err != nil {
				t.Fatal(err)
			}
			if len(matches) != 1 {
				t.Fatalf("cache quarantine barrier has %d candidates, want 1: %v", len(matches), matches)
			}
			publicName = matches[0]
			retained = publicName + ".retained"
			if got := readContractFile(t, filepath.Join(publicName, "legacy-original", "legacy")); got != "legacy bytes\n" {
				t.Fatalf("legacy bytes were not pinned before replacement: %q", got)
			}
			if err := os.Rename(publicName, retained); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(publicName, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(publicName, "winner"), []byte("unknown winner\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	)
	if waitErr == nil {
		t.Fatalf("post-quarantine cache recovery replacement unexpectedly succeeded\n%s", output)
	}
	if !strings.Contains(output, "legacy recovery retained at "+retained) ||
		!strings.Contains(output, "unknown collision preserved at "+publicName) {
		t.Fatalf("post-quarantine cache replacement did not report both exact owners\n%s", output)
	}
	for path, want := range map[string]string{
		filepath.Join(retained, "legacy-original", "legacy"): "legacy bytes\n",
		filepath.Join(publicName, "winner"):                  "unknown winner\n",
		filepath.Join(canonical, "canonical"):                "canonical bytes\n",
	} {
		if got := readContractFile(t, path); got != want {
			t.Fatalf("post-quarantine cache race changed %s: %q", path, got)
		}
	}
}

func TestHomeManagerLegacyCacheFinalVerifyRejectsLateLegacyPath(t *testing.T) {
	cacheHome := t.TempDir()
	legacy := filepath.Join(cacheHome, "mysetup")
	canonical := filepath.Join(cacheHome, "wahrwelt")
	token := legacyCachePreflightToken(t, legacy, canonical)
	if err := os.Mkdir(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "late"), []byte("late legacy bytes\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	output, err := exec.Command(
		"bash", homeManagerLegacyCacheMerge, "verify", legacy, canonical, cacheHome, token,
	).CombinedOutput()
	if err == nil || !strings.Contains(string(output), "cache paths must be ordinary directories") {
		t.Fatalf("late legacy cache path was accepted: err=%v\n%s", err, output)
	}
	if got := readContractFile(t, filepath.Join(legacy, "late")); got != "late legacy bytes\n" {
		t.Fatalf("late legacy cache bytes changed: %q", got)
	}
}

func TestHomeManagerLegacyCacheAbsentParentTokenRejectsAncestorReplacement(t *testing.T) {
	root := t.TempDir()
	anchor := filepath.Join(root, "anchor")
	originalAnchor := filepath.Join(root, "anchor-before-race")
	cacheHome := filepath.Join(anchor, "missing", "cache")
	legacy := filepath.Join(cacheHome, "mysetup")
	canonical := filepath.Join(cacheHome, "wahrwelt")
	if err := os.Mkdir(anchor, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(anchor, "original"), []byte("original anchor bytes\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	token := legacyCachePreflightToken(t, legacy, canonical)
	if !strings.HasPrefix(token, "absent-parent|") {
		t.Fatalf("unexpected absent-parent cache token %q", token)
	}
	if err := os.Rename(anchor, originalAnchor); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "winner"), []byte("replacement cache bytes\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	output, err := exec.Command(
		"bash", homeManagerLegacyCacheMerge, "verify", legacy, canonical, cacheHome, token,
	).CombinedOutput()
	if err == nil || !strings.Contains(string(output), "anchored ancestor changed after preflight") {
		t.Fatalf("replacement absent-parent cache anchor was accepted: err=%v\n%s", err, output)
	}
	for path, want := range map[string]string{
		filepath.Join(originalAnchor, "original"): "original anchor bytes\n",
		filepath.Join(legacy, "winner"):           "replacement cache bytes\n",
	} {
		if got := readContractFile(t, path); got != want {
			t.Fatalf("absent-parent cache verification changed %s: %q", path, got)
		}
	}
}

func TestHomeManagerMigrationAvoidsDestructiveLegacyCleanup(t *testing.T) {
	migration := readContractFile(t, "../../../NixOS/home/programs/wahrwelt-migration.nix")
	for _, want := range []string{
		`legacy-cache-merge`,
		`"merge" "$old" "$new" "${cacheHome}"`,
		`verify_hypr_user_tree`,
		`verify_tree "${configHome}/mysetup"`,
		`verify_tree "${stateHome}/mysetup"`,
		`verify_cache`,
		`verify_old_links`,
		`verify_no_legacy_markers`,
	} {
		if !strings.Contains(migration, want) {
			t.Fatalf("Home Manager recoverable cache migration is missing %q\n%s", want, migration)
		}
	}
	for _, forbidden := range []string{
		`${pkgs.coreutils}/bin/rm -rf -- "$old"`,
		`legacy_runtime="/tmp/mysetup-runtime-`,
		`${configHome}/hypr/lib/mysetup.lua.backup`,
		`legacy_cli="${home}/.local/bin/mysetup"`,
		`${pkgs.coreutils}/bin/rm -f -- "$old_marker"`,
		`${pkgs.gnused}/bin/sed -i`,
	} {
		if strings.Contains(migration, forbidden) {
			t.Fatalf("Home Manager migration retains destructive cleanup %q\n%s", forbidden, migration)
		}
	}
}

func TestHomeManagerLegacyNamespaceMoveRollsBackThroughPinnedParent(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "config")
	originalParent := filepath.Join(root, "config-before-race")
	legacy := filepath.Join(parent, "mysetup")
	canonical := filepath.Join(parent, "wahrwelt")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "legacy"), []byte("legacy owner\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tokenOutput, err := exec.Command("bash", homeManagerLegacyNamespaceMove, "check", legacy, canonical).CombinedOutput()
	if err != nil {
		t.Fatalf("namespace preflight: %v\n%s", err, tokenOutput)
	}
	token := strings.TrimSpace(string(tokenOutput))
	if !strings.HasPrefix(token, "present|") {
		t.Fatalf("unexpected namespace token %q", token)
	}

	readyR, readyW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readyR.Close()
	continueR, continueW, err := os.Pipe()
	if err != nil {
		_ = readyW.Close()
		t.Fatal(err)
	}
	defer continueW.Close()
	var output bytes.Buffer
	cmd := exec.Command("bash", homeManagerLegacyNamespaceMove, "move", legacy, canonical, token)
	cmd.Env = append(os.Environ(),
		"WAHRWELT_TEST_NAMESPACE_MOVE_READY_FD=3",
		"WAHRWELT_TEST_NAMESPACE_MOVE_CONTINUE_FD=4",
	)
	cmd.ExtraFiles = []*os.File{readyW, continueR}
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		_ = readyW.Close()
		_ = continueR.Close()
		t.Fatal(err)
	}
	_ = readyW.Close()
	_ = continueR.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	defer func() {
		if finished {
			return
		}
		_, _ = continueW.Write([]byte{'1'})
		_ = cmd.Process.Kill()
		<-done
	}()
	marker := make([]byte, len("ready\n"))
	ready := make(chan error, 1)
	go func() {
		_, readErr := io.ReadFull(readyR, marker)
		ready <- readErr
	}()
	select {
	case readErr := <-ready:
		if readErr != nil || string(marker) != "ready\n" {
			t.Fatalf("namespace helper barrier failed: marker=%q err=%v\n%s", marker, readErr, output.String())
		}
	case waitErr := <-done:
		finished = true
		t.Fatalf("namespace helper exited before barrier: %v\n%s", waitErr, output.String())
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for namespace helper barrier\n%s", output.String())
	}

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
	if _, err := continueW.Write([]byte{'1'}); err != nil {
		t.Fatal(err)
	}
	waitErr := <-done
	finished = true
	if waitErr == nil || !strings.Contains(output.String(), "rolled back through pinned parent") {
		t.Fatalf("namespace parent replacement was accepted: err=%v\n%s", waitErr, output.String())
	}
	if got := readContractFile(t, filepath.Join(originalParent, "mysetup", "legacy")); got != "legacy owner\n" {
		t.Fatalf("original namespace was not restored: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(originalParent, "wahrwelt")); !os.IsNotExist(statErr) {
		t.Fatalf("original canonical namespace remains after rollback: %v", statErr)
	}
	for path, want := range map[string]string{
		filepath.Join(parent, "mysetup", "winner"):  "source winner\n",
		filepath.Join(parent, "wahrwelt", "winner"): "target winner\n",
	} {
		if got := readContractFile(t, path); got != want {
			t.Fatalf("replacement winner changed at %s: %q", path, got)
		}
	}
}

func TestHomeManagerLegacyNamespaceMoveRestoresReplacementMovedDuringCommit(t *testing.T) {
	parent := t.TempDir()
	legacy := filepath.Join(parent, "mysetup")
	canonical := filepath.Join(parent, "wahrwelt")
	savedLegacy := filepath.Join(parent, "expected-legacy")
	if err := os.Mkdir(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "legacy"), []byte("expected legacy\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tokenOutput, err := exec.Command("bash", homeManagerLegacyNamespaceMove, "check", legacy, canonical).CombinedOutput()
	if err != nil {
		t.Fatalf("namespace preflight: %v\n%s", err, tokenOutput)
	}
	cmd := exec.Command(
		"bash", homeManagerLegacyNamespaceMove, "move", legacy, canonical,
		strings.TrimSpace(string(tokenOutput)),
	)
	output, waitErr := runAtFDBarrier(
		t,
		cmd,
		"WAHRWELT_TEST_NAMESPACE_MOVE_READY_FD",
		"WAHRWELT_TEST_NAMESPACE_MOVE_CONTINUE_FD",
		func() {
			if err := os.Rename(legacy, savedLegacy); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(legacy, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(legacy, "winner"), []byte("source winner\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	)
	if waitErr == nil || !strings.Contains(output, "concurrent replacement restored") {
		t.Fatalf("replacement namespace owner was accepted: err=%v\n%s", waitErr, output)
	}
	if got := readContractFile(t, filepath.Join(legacy, "winner")); got != "source winner\n" {
		t.Fatalf("replacement namespace changed: %q", got)
	}
	if got := readContractFile(t, filepath.Join(savedLegacy, "legacy")); got != "expected legacy\n" {
		t.Fatalf("expected legacy namespace changed: %q", got)
	}
	if _, statErr := os.Lstat(canonical); !os.IsNotExist(statErr) {
		t.Fatalf("replacement namespace remained at canonical path: %v", statErr)
	}
}

func TestHomeManagerLegacyNamespaceFinalVerifyRejectsLateAbsentSource(t *testing.T) {
	parent := t.TempDir()
	legacy := filepath.Join(parent, "mysetup")
	canonical := filepath.Join(parent, "wahrwelt")
	tokenOutput, err := exec.Command(
		"bash", homeManagerLegacyNamespaceMove, "check", legacy, canonical,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("namespace absent preflight: %v\n%s", err, tokenOutput)
	}
	cmd := exec.Command(
		"bash", homeManagerLegacyNamespaceMove, "verify", legacy, canonical,
		strings.TrimSpace(string(tokenOutput)),
	)
	output, waitErr := runAtFDBarrier(
		t,
		cmd,
		"WAHRWELT_TEST_NAMESPACE_VERIFY_READY_FD",
		"WAHRWELT_TEST_NAMESPACE_VERIFY_CONTINUE_FD",
		func() {
			if err := os.Mkdir(legacy, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(legacy, "late"), []byte("late legacy bytes\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	)
	if waitErr == nil || !strings.Contains(output, "legacy namespace remains after migration") {
		t.Fatalf("late absent namespace was accepted: err=%v\n%s", waitErr, output)
	}
	if got := readContractFile(t, filepath.Join(legacy, "late")); got != "late legacy bytes\n" {
		t.Fatalf("late namespace bytes changed: %q", got)
	}
}

func TestHomeManagerLegacyNamespaceAbsentParentTokenRejectsLateParentTree(t *testing.T) {
	root := t.TempDir()
	anchor := filepath.Join(root, "anchor")
	parent := filepath.Join(anchor, "missing", "config")
	legacy := filepath.Join(parent, "mysetup")
	canonical := filepath.Join(parent, "wahrwelt")
	if err := os.Mkdir(anchor, 0o700); err != nil {
		t.Fatal(err)
	}
	tokenOutput, err := exec.Command(
		"bash", homeManagerLegacyNamespaceMove, "check", legacy, canonical,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("namespace absent-parent preflight: %v\n%s", err, tokenOutput)
	}
	token := strings.TrimSpace(string(tokenOutput))
	if !strings.HasPrefix(token, "absent-parent|") {
		t.Fatalf("unexpected absent-parent namespace token %q", token)
	}
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "late"), []byte("late parent bytes\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	output, err := exec.Command(
		"bash", homeManagerLegacyNamespaceMove, "verify", legacy, canonical, token,
	).CombinedOutput()
	if err == nil || !strings.Contains(string(output), "legacy namespace appeared after absent-parent preflight") {
		t.Fatalf("late absent-parent namespace was accepted: err=%v\n%s", err, output)
	}
	if got := readContractFile(t, filepath.Join(legacy, "late")); got != "late parent bytes\n" {
		t.Fatalf("late absent-parent namespace bytes changed: %q", got)
	}
}

func TestHomeManagerLegacyNamespaceAbsentParentTokenRejectsAncestorReplacement(t *testing.T) {
	root := t.TempDir()
	anchor := filepath.Join(root, "anchor")
	originalAnchor := filepath.Join(root, "anchor-before-race")
	parent := filepath.Join(anchor, "missing", "config")
	legacy := filepath.Join(parent, "mysetup")
	canonical := filepath.Join(parent, "wahrwelt")
	if err := os.Mkdir(anchor, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(anchor, "original"), []byte("original namespace anchor\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tokenOutput, err := exec.Command(
		"bash", homeManagerLegacyNamespaceMove, "check", legacy, canonical,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("namespace absent-parent preflight: %v\n%s", err, tokenOutput)
	}
	token := strings.TrimSpace(string(tokenOutput))
	if !strings.HasPrefix(token, "absent-parent|") {
		t.Fatalf("unexpected absent-parent namespace token %q", token)
	}
	if err := os.Rename(anchor, originalAnchor); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "winner"), []byte("replacement namespace bytes\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	output, err := exec.Command(
		"bash", homeManagerLegacyNamespaceMove, "verify", legacy, canonical, token,
	).CombinedOutput()
	if err == nil || !strings.Contains(string(output), "anchored ancestor changed after preflight") {
		t.Fatalf("replacement absent-parent namespace anchor was accepted: err=%v\n%s", err, output)
	}
	for path, want := range map[string]string{
		filepath.Join(originalAnchor, "original"): "original namespace anchor\n",
		filepath.Join(legacy, "winner"):           "replacement namespace bytes\n",
	} {
		if got := readContractFile(t, path); got != want {
			t.Fatalf("absent-parent namespace verification changed %s: %q", path, got)
		}
	}
}

func TestHomeManagerLegacyNamespaceFinalVerifyRequiresPublishedIdentity(t *testing.T) {
	parent := t.TempDir()
	legacy := filepath.Join(parent, "mysetup")
	canonical := filepath.Join(parent, "wahrwelt")
	expectedRecovery := filepath.Join(parent, "expected-published")
	if err := os.Mkdir(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "legacy"), []byte("expected namespace\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tokenOutput, err := exec.Command(
		"bash", homeManagerLegacyNamespaceMove, "check", legacy, canonical,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("namespace present preflight: %v\n%s", err, tokenOutput)
	}
	token := strings.TrimSpace(string(tokenOutput))
	if output, err := exec.Command(
		"bash", homeManagerLegacyNamespaceMove, "move", legacy, canonical, token,
	).CombinedOutput(); err != nil {
		t.Fatalf("namespace move: %v\n%s", err, output)
	}
	if output, err := exec.Command(
		"bash", homeManagerLegacyNamespaceMove, "verify", legacy, canonical, token,
	).CombinedOutput(); err != nil {
		t.Fatalf("namespace final verify: %v\n%s", err, output)
	}
	if err := os.Rename(canonical, expectedRecovery); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(canonical, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(canonical, "winner"), []byte("unknown winner\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(
		"bash", homeManagerLegacyNamespaceMove, "verify", legacy, canonical, token,
	).CombinedOutput()
	if err == nil || !strings.Contains(string(output), "published namespace identity changed") {
		t.Fatalf("replacement published namespace was accepted: err=%v\n%s", err, output)
	}
	for path, want := range map[string]string{
		filepath.Join(expectedRecovery, "legacy"): "expected namespace\n",
		filepath.Join(canonical, "winner"):        "unknown winner\n",
	} {
		if got := readContractFile(t, path); got != want {
			t.Fatalf("namespace final verify changed %s: %q", path, got)
		}
	}
}

func TestHomeManagerLegacyMarkerPublicationUsesAnonymousCandidateAndPinnedRollback(t *testing.T) {
	root := t.TempDir()
	configHome := filepath.Join(root, "config")
	parent := filepath.Join(configHome, "hypr")
	originalParent := filepath.Join(configHome, "hypr-before-race")
	oldMarker := filepath.Join(parent, ".mysetup-managed.json")
	newMarker := filepath.Join(parent, ".wahrwelt-managed.json")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldMarker, []byte("{\"manager\":\"mysetup\",\"kind\":\"hypr\",\"version\":2}\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	tokenOutput, err := exec.Command(
		"bash", homeManagerLegacyMarkerMigrate,
		"check", oldMarker, newMarker, configHome, root,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("marker preflight: %v\n%s", err, tokenOutput)
	}
	token := strings.TrimSpace(string(tokenOutput))
	if !strings.HasPrefix(token, "publish|") {
		t.Fatalf("unexpected marker token %q", token)
	}

	readyR, readyW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readyR.Close()
	continueR, continueW, err := os.Pipe()
	if err != nil {
		_ = readyW.Close()
		t.Fatal(err)
	}
	defer continueW.Close()
	var output bytes.Buffer
	cmd := exec.Command(
		"bash", homeManagerLegacyMarkerMigrate,
		"migrate", oldMarker, newMarker, configHome, root, token,
	)
	cmd.Env = append(os.Environ(),
		"WAHRWELT_TEST_MARKER_READY_FD=3",
		"WAHRWELT_TEST_MARKER_CONTINUE_FD=4",
	)
	cmd.ExtraFiles = []*os.File{readyW, continueR}
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		_ = readyW.Close()
		_ = continueR.Close()
		t.Fatal(err)
	}
	_ = readyW.Close()
	_ = continueR.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	defer func() {
		if finished {
			return
		}
		_, _ = continueW.Write([]byte{'1'})
		_ = cmd.Process.Kill()
		<-done
	}()
	marker := make([]byte, len("ready\n"))
	ready := make(chan error, 1)
	go func() {
		_, readErr := io.ReadFull(readyR, marker)
		ready <- readErr
	}()
	select {
	case readErr := <-ready:
		if readErr != nil || string(marker) != "ready\n" {
			t.Fatalf("marker helper barrier failed: marker=%q err=%v\n%s", marker, readErr, output.String())
		}
	case waitErr := <-done:
		finished = true
		t.Fatalf("marker helper exited before barrier: %v\n%s", waitErr, output.String())
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for marker helper barrier\n%s", output.String())
	}

	if err := os.Rename(parent, originalParent); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		filepath.Join(parent, ".mysetup-managed.json"):  "replacement legacy winner\n",
		filepath.Join(parent, ".wahrwelt-managed.json"): "replacement canonical winner\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := continueW.Write([]byte{'1'}); err != nil {
		t.Fatal(err)
	}
	waitErr := <-done
	finished = true
	if waitErr == nil || !strings.Contains(output.String(), "generated marker retained at") {
		t.Fatalf("marker parent replacement was accepted: err=%v\n%s", waitErr, output.String())
	}
	if got := readContractFile(t, filepath.Join(originalParent, ".mysetup-managed.json")); !strings.Contains(got, `"manager":"mysetup"`) {
		t.Fatalf("original legacy marker changed: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(originalParent, ".wahrwelt-managed.json")); !os.IsNotExist(statErr) {
		t.Fatalf("anonymous candidate remains in original parent after rollback: %v", statErr)
	}
	for path, want := range map[string]string{
		filepath.Join(parent, ".mysetup-managed.json"):  "replacement legacy winner\n",
		filepath.Join(parent, ".wahrwelt-managed.json"): "replacement canonical winner\n",
	} {
		if got := readContractFile(t, path); got != want {
			t.Fatalf("replacement marker winner changed at %s: %q", path, got)
		}
	}
	matches, globErr := filepath.Glob(filepath.Join(originalParent, ".wahrwelt-managed.json.migration.*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 0 {
		t.Fatalf("predictable marker candidates were published: %v", matches)
	}
}

func TestHomeManagerLegacyMarkerMigrationRemovesVerifiedPublicLegacyName(t *testing.T) {
	for _, compatible := range []bool{false, true} {
		name := "publish"
		if compatible {
			name = "compatible"
		}
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			configHome := filepath.Join(home, ".config")
			parent := filepath.Join(configHome, "hypr")
			if err := os.MkdirAll(parent, 0o700); err != nil {
				t.Fatal(err)
			}
			oldMarker := filepath.Join(parent, ".mysetup-managed.json")
			newMarker := filepath.Join(parent, ".wahrwelt-managed.json")
			legacyContent := "{\"manager\":\"mysetup\",\"kind\":\"hypr\",\"version\":2}\n"
			if err := os.WriteFile(oldMarker, []byte(legacyContent), 0o640); err != nil {
				t.Fatal(err)
			}
			var canonicalInfo os.FileInfo
			if compatible {
				if err := os.WriteFile(newMarker, []byte("{\"manager\":\"wahrwelt\",\"kind\":\"hypr\",\"version\":2}\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				canonicalInfo, _ = os.Stat(newMarker)
			}
			tokenOutput, err := exec.Command(
				"bash", homeManagerLegacyMarkerMigrate,
				"check", oldMarker, newMarker, configHome, home,
			).CombinedOutput()
			if err != nil {
				t.Fatalf("marker preflight: %v\n%s", err, tokenOutput)
			}
			output, err := exec.Command(
				"bash", homeManagerLegacyMarkerMigrate,
				"migrate", oldMarker, newMarker, configHome, home,
				strings.TrimSpace(string(tokenOutput)),
			).CombinedOutput()
			if err != nil {
				t.Fatalf("marker migrate: %v\n%s", err, output)
			}
			if _, statErr := os.Lstat(oldMarker); !os.IsNotExist(statErr) {
				t.Fatalf("verified legacy marker remains public: %v", statErr)
			}
			if got := readContractFile(t, newMarker); !strings.Contains(got, `"manager"`) || !strings.Contains(got, "wahrwelt") {
				t.Fatalf("canonical marker content = %q", got)
			}
			if compatible {
				currentInfo, statErr := os.Stat(newMarker)
				if statErr != nil || !os.SameFile(canonicalInfo, currentInfo) {
					t.Fatalf("compatible canonical marker was replaced: before=%v after=%v err=%v", canonicalInfo, currentInfo, statErr)
				}
			}
			recoveryLine := strings.TrimSpace(string(output))
			const prefix = "Wahrwelt marker recovery retained at "
			if !strings.HasPrefix(recoveryLine, prefix) {
				t.Fatalf("marker recovery path not reported: %q", recoveryLine)
			}
			recovery := strings.TrimPrefix(recoveryLine, prefix)
			if got := readContractFile(t, filepath.Join(recovery, "legacy-marker")); got != legacyContent {
				t.Fatalf("legacy marker recovery changed: %q", got)
			}
		})
	}
}

func TestHomeManagerLegacyMarkerMigrationPreservesReplacementBeforeLegacyRemoval(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	parent := filepath.Join(configHome, "hypr")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	oldMarker := filepath.Join(parent, ".mysetup-managed.json")
	savedOld := filepath.Join(parent, "expected-legacy-marker")
	newMarker := filepath.Join(parent, ".wahrwelt-managed.json")
	legacyContent := "{\"manager\":\"mysetup\",\"kind\":\"hypr\",\"version\":2}\n"
	if err := os.WriteFile(oldMarker, []byte(legacyContent), 0o640); err != nil {
		t.Fatal(err)
	}
	tokenOutput, err := exec.Command(
		"bash", homeManagerLegacyMarkerMigrate,
		"check", oldMarker, newMarker, configHome, home,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("marker preflight: %v\n%s", err, tokenOutput)
	}

	readyR, readyW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readyR.Close()
	continueR, continueW, err := os.Pipe()
	if err != nil {
		_ = readyW.Close()
		t.Fatal(err)
	}
	defer continueW.Close()
	var output bytes.Buffer
	cmd := exec.Command(
		"bash", homeManagerLegacyMarkerMigrate,
		"migrate", oldMarker, newMarker, configHome, home,
		strings.TrimSpace(string(tokenOutput)),
	)
	cmd.Env = append(os.Environ(),
		"WAHRWELT_TEST_MARKER_REMOVE_READY_FD=3",
		"WAHRWELT_TEST_MARKER_REMOVE_CONTINUE_FD=4",
	)
	cmd.ExtraFiles = []*os.File{readyW, continueR}
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		_ = readyW.Close()
		_ = continueR.Close()
		t.Fatal(err)
	}
	_ = readyW.Close()
	_ = continueR.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	defer func() {
		if finished {
			return
		}
		_, _ = continueW.Write([]byte{'1'})
		_ = cmd.Process.Kill()
		<-done
	}()
	marker := make([]byte, len("ready\n"))
	ready := make(chan error, 1)
	go func() {
		_, readErr := io.ReadFull(readyR, marker)
		ready <- readErr
	}()
	select {
	case readErr := <-ready:
		if readErr != nil || string(marker) != "ready\n" {
			t.Fatalf("marker removal barrier failed: marker=%q err=%v\n%s", marker, readErr, output.String())
		}
	case waitErr := <-done:
		finished = true
		t.Fatalf("marker helper exited before removal barrier: %v\n%s", waitErr, output.String())
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for marker removal barrier\n%s", output.String())
	}
	if err := os.Rename(oldMarker, savedOld); err != nil {
		t.Fatal(err)
	}
	replacementContent := "{\"manager\":\"someone-else\",\"winner\":true}\n"
	if err := os.WriteFile(oldMarker, []byte(replacementContent), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := continueW.Write([]byte{'1'}); err != nil {
		t.Fatal(err)
	}
	waitErr := <-done
	finished = true
	if waitErr == nil || !strings.Contains(output.String(), "concurrent replacement restored") {
		t.Fatalf("replacement legacy marker was accepted: err=%v\n%s", waitErr, output.String())
	}
	if got := readContractFile(t, oldMarker); got != replacementContent {
		t.Fatalf("replacement legacy marker changed: %q", got)
	}
	if got := readContractFile(t, savedOld); got != legacyContent {
		t.Fatalf("expected legacy marker disappeared: %q", got)
	}
	if _, statErr := os.Lstat(newMarker); !os.IsNotExist(statErr) {
		t.Fatalf("published canonical marker was not rolled back: %v", statErr)
	}
}

func TestHomeManagerLegacyMarkerMigrationRestoresLegacyMarkerWhenParentChangesDuringRemoval(t *testing.T) {
	root := t.TempDir()
	configHome := filepath.Join(root, "config")
	parent := filepath.Join(configHome, "hypr")
	originalParent := filepath.Join(configHome, "original-hypr")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	oldMarker := filepath.Join(parent, ".mysetup-managed.json")
	newMarker := filepath.Join(parent, ".wahrwelt-managed.json")
	legacyContent := "{\"manager\":\"mysetup\",\"kind\":\"hypr\",\"version\":2}\n"
	canonicalContent := "{\"manager\":\"wahrwelt\",\"kind\":\"hypr\",\"version\":2}\n"
	if err := os.WriteFile(oldMarker, []byte(legacyContent), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newMarker, []byte(canonicalContent), 0o600); err != nil {
		t.Fatal(err)
	}
	tokenOutput, err := exec.Command(
		"bash", homeManagerLegacyMarkerMigrate,
		"check", oldMarker, newMarker, configHome, root,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("marker preflight: %v\n%s", err, tokenOutput)
	}

	readyR, readyW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readyR.Close()
	continueR, continueW, err := os.Pipe()
	if err != nil {
		_ = readyW.Close()
		t.Fatal(err)
	}
	defer continueW.Close()
	var output bytes.Buffer
	cmd := exec.Command(
		"bash", homeManagerLegacyMarkerMigrate,
		"migrate", oldMarker, newMarker, configHome, root,
		strings.TrimSpace(string(tokenOutput)),
	)
	cmd.Env = append(os.Environ(),
		"WAHRWELT_TEST_MARKER_REMOVE_READY_FD=3",
		"WAHRWELT_TEST_MARKER_REMOVE_CONTINUE_FD=4",
	)
	cmd.ExtraFiles = []*os.File{readyW, continueR}
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		_ = readyW.Close()
		_ = continueR.Close()
		t.Fatal(err)
	}
	_ = readyW.Close()
	_ = continueR.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	defer func() {
		if finished {
			return
		}
		_, _ = continueW.Write([]byte{'1'})
		_ = cmd.Process.Kill()
		<-done
	}()
	marker := make([]byte, len("ready\n"))
	ready := make(chan error, 1)
	go func() {
		_, readErr := io.ReadFull(readyR, marker)
		ready <- readErr
	}()
	select {
	case readErr := <-ready:
		if readErr != nil || string(marker) != "ready\n" {
			t.Fatalf("marker removal barrier failed: marker=%q err=%v\n%s", marker, readErr, output.String())
		}
	case waitErr := <-done:
		finished = true
		t.Fatalf("marker helper exited before removal barrier: %v\n%s", waitErr, output.String())
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for marker removal barrier\n%s", output.String())
	}

	if err := os.Rename(parent, originalParent); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	replacementLegacy := "replacement legacy winner\n"
	replacementCanonical := "replacement canonical winner\n"
	if err := os.WriteFile(filepath.Join(parent, ".mysetup-managed.json"), []byte(replacementLegacy), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, ".wahrwelt-managed.json"), []byte(replacementCanonical), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := continueW.Write([]byte{'1'}); err != nil {
		t.Fatal(err)
	}
	waitErr := <-done
	finished = true
	if waitErr == nil || !strings.Contains(output.String(), "legacy marker restored through pinned parent") {
		t.Fatalf("marker parent replacement was accepted or not restored: err=%v\n%s", waitErr, output.String())
	}
	if got := readContractFile(t, filepath.Join(originalParent, ".mysetup-managed.json")); got != legacyContent {
		t.Fatalf("original legacy marker was not restored: %q", got)
	}
	if got := readContractFile(t, filepath.Join(originalParent, ".wahrwelt-managed.json")); got != canonicalContent {
		t.Fatalf("original canonical marker changed: %q", got)
	}
	if got := readContractFile(t, filepath.Join(parent, ".mysetup-managed.json")); got != replacementLegacy {
		t.Fatalf("replacement legacy marker changed: %q", got)
	}
	if got := readContractFile(t, filepath.Join(parent, ".wahrwelt-managed.json")); got != replacementCanonical {
		t.Fatalf("replacement canonical marker changed: %q", got)
	}
}
