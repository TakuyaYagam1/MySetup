//go:build linux

package config

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const currentHyprUserAdapterDigest = "3666c398dbba460e9b3dac54f396a7f53ad2093f49967c05e4588e66c41f08eb"

func TestHyprUserAdapterGuardCurrentDigestMatchesGoAndShellAllowlists(t *testing.T) {
	current := readContractFile(t, "../../../dots/hypr/hyprland.lua")
	if got := fmt.Sprintf("%x", sha256.Sum256([]byte(current))); got != currentHyprUserAdapterDigest {
		t.Fatalf("current adapter digest = %s, want %s", got, currentHyprUserAdapterDigest)
	}
	for name, source := range map[string]string{
		"activation": readContractFile(t, "../../../NixOS/home/shells/runtime-activation.sh"),
		"shell":      readContractFile(t, hyprUserAdapterGuard),
		"go":         readContractFile(t, "../dots/hypr_files.go"),
	} {
		if !strings.Contains(source, currentHyprUserAdapterDigest) {
			t.Fatalf("%s adapter allowlist is missing current digest %s", name, currentHyprUserAdapterDigest)
		}
	}
}

func TestHomeManagerDefaultSeedDoesNotInvokeReplaceablePathTools(t *testing.T) {
	helper := "../../../NixOS/home/shells/runtime-activation.sh"
	source := filepath.Join(t.TempDir(), "default-source.lua")
	canonical := "-- canonical default\n"
	if err := os.WriteFile(source, []byte(canonical), 0o600); err != nil {
		t.Fatal(err)
	}
	realLn, err := exec.LookPath("ln")
	if err != nil {
		t.Fatal(err)
	}
	realMv, err := exec.LookPath("mv")
	if err != nil {
		t.Fatal(err)
	}

	for _, mode := range []string{"publish", "error-cleanup"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "default.lua")
			winner := filepath.Join(dir, "unknown-winner.lua")
			unknown := "-- unknown temp replacement\n"
			if err := os.WriteFile(winner, []byte(unknown), 0o600); err != nil {
				t.Fatal(err)
			}
			fakeBin := filepath.Join(dir, "fake-bin")
			if err := os.Mkdir(fakeBin, 0o755); err != nil {
				t.Fatal(err)
			}
			fakeLn := `#!/usr/bin/env bash
set -euo pipefail
args=("$@")
temp_index=$((${#args[@]} - 2))
temp=${args[$temp_index]}
"$WAHRWELT_REAL_MV" -T -- "$WAHRWELT_UNKNOWN_WINNER" "$temp"
if [ "$WAHRWELT_TEMP_RACE_MODE" = publish ]; then
  exec "$WAHRWELT_REAL_LN" "$@"
fi
exit 1
`
			if err := os.WriteFile(filepath.Join(fakeBin, "ln"), []byte(fakeLn), 0o755); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("bash", helper, "seed-exclusive", source, target, "Wahrwelt user config")
			cmd.Env = append(
				os.Environ(),
				"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
				"WAHRWELT_REAL_LN="+realLn,
				"WAHRWELT_REAL_MV="+realMv,
				"WAHRWELT_UNKNOWN_WINNER="+winner,
				"WAHRWELT_TEMP_RACE_MODE="+mode,
			)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("secure seed failed after a public-temp substitution attempt: %v\n%s", err, output)
			}
			if got := readContractFile(t, target); got != canonical {
				t.Fatalf("seed published replaceable temp content: %q", got)
			}
			if got := readContractFile(t, winner); got != unknown {
				t.Fatalf("unknown temp replacement was moved or deleted: %q", got)
			}
		})
	}
}

func TestHomeManagerDefaultSeedFailsClosedWhenParentIsReplaced(t *testing.T) {
	helper := "../../../NixOS/home/shells/runtime-activation.sh"
	root := t.TempDir()
	source := filepath.Join(root, "default-source.lua")
	liveDir := filepath.Join(root, "user")
	displacedDir := filepath.Join(root, "user-displaced")
	victimDir := filepath.Join(root, "victim")
	if err := os.WriteFile(source, []byte("-- canonical default\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{liveDir, victimDir} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(liveDir, "default.lua")

	runSeedExclusiveAtBarrier(t, helper, source, target, false, func() {
		if err := os.Rename(liveDir, displacedDir); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(victimDir, liveDir); err != nil {
			t.Fatal(err)
		}
	})

	if got, err := os.Readlink(liveDir); err != nil || got != victimDir {
		t.Fatalf("unknown replacement parent changed: target=%q err=%v", got, err)
	}
	for _, unexpected := range []string{
		filepath.Join(victimDir, "default.lua"),
		filepath.Join(displacedDir, "default.lua"),
	} {
		if _, err := os.Lstat(unexpected); !os.IsNotExist(err) {
			t.Fatalf("seed published after parent replacement at %s: err=%v", unexpected, err)
		}
	}
}

func TestHyprUserAdapterGuardRetainsHistoricalRecovery(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "hyprland.lua")
	legacy := readContractFile(t, "../../../NixOS/home/shells/legacy-hypr-runtime/user-adapter-wahrwelt-v1.lua")
	if err := os.WriteFile(target, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", hyprUserAdapterGuard, "prepare", target, "../../../dots/hypr/hyprland.lua", "")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("guard prepare failed: %v\n%s", err, output)
	}
	recovery := singleHistoricalAdapterRecovery(t, dir)
	if got := readContractFile(t, recovery); got != legacy {
		t.Fatalf("historical recovery content = %q", got)
	}
	if !strings.Contains(string(output), recovery) || strings.Contains(string(output), "/proc/self/fd/") {
		t.Fatalf("guard did not report the durable recovery path %q\n%s", recovery, output)
	}
}

func TestHyprUserAdapterGuardFailsClosedOnRecoveryReplacement(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "hyprland.lua")
	legacy := readContractFile(t, "../../../NixOS/home/shells/legacy-hypr-runtime/user-adapter-wahrwelt-v1.lua")
	current := readContractFile(t, "../../../dots/hypr/hyprland.lua")
	if err := os.WriteFile(target, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
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
	cmd := exec.Command("bash", hyprUserAdapterGuard, "prepare", target, "../../../dots/hypr/hyprland.lua", "")
	cmd.Env = append(
		os.Environ(),
		"WAHRWELT_TEST_ADAPTER_READY_FD=3",
		"WAHRWELT_TEST_ADAPTER_CONTINUE_FD=4",
	)
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

	ready := make(chan error, 1)
	go func() {
		marker := make([]byte, len("ready\n"))
		_, err := io.ReadFull(readyR, marker)
		if err == nil && string(marker) != "ready\n" {
			err = fmt.Errorf("unexpected adapter barrier marker %q", marker)
		}
		ready <- err
	}()
	select {
	case err := <-ready:
		if err != nil {
			childErr := stopAndWait()
			t.Fatalf("guard did not reach recovery barrier: %v (child: %v)\n%s", err, childErr, output.String())
		}
	case err := <-done:
		waitErr = err
		finished = true
		t.Fatalf("guard exited before recovery barrier: %v\n%s", err, output.String())
	case <-time.After(5 * time.Second):
		childErr := stopAndWait()
		t.Fatalf("timed out waiting for recovery barrier (child: %v)\n%s", childErr, output.String())
	}

	recovery := singleHistoricalAdapterRecovery(t, dir)
	unknown := "-- unknown recovery replacement\n"
	winner := filepath.Join(dir, "unknown-winner.lua")
	if err := os.WriteFile(winner, []byte(unknown), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(winner, recovery); err != nil {
		t.Fatal(err)
	}
	if _, err := continueW.Write([]byte{'1'}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		waitErr = err
		finished = true
		if err == nil {
			t.Fatalf("guard accepted replaced recovery\n%s", output.String())
		}
	case <-time.After(5 * time.Second):
		childErr := stopAndWait()
		t.Fatalf("guard did not finish after recovery barrier (child: %v)\n%s", childErr, output.String())
	}
	if got := readContractFile(t, recovery); got != unknown {
		t.Fatalf("unknown recovery replacement changed: %q", got)
	}
	if got := readContractFile(t, target); got != current {
		t.Fatalf("published current adapter changed after recovery collision: %q", got)
	}
}

func TestHyprUserAdapterGuardPinsParentDuringHistoricalPublication(t *testing.T) {
	root := t.TempDir()
	liveDir := filepath.Join(root, "user")
	displacedDir := filepath.Join(root, "user-displaced")
	victimDir := filepath.Join(root, "victim")
	for _, dir := range []string{liveDir, victimDir} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(liveDir, "hyprland.lua")
	victimTarget := filepath.Join(victimDir, "hyprland.lua")
	legacy := readContractFile(t, "../../../NixOS/home/shells/legacy-hypr-runtime/user-adapter-wahrwelt-v1.lua")
	if err := os.WriteFile(target, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	realCp, err := exec.LookPath("cp")
	if err != nil {
		t.Fatal(err)
	}
	realLn, err := exec.LookPath("ln")
	if err != nil {
		t.Fatal(err)
	}
	realMv, err := exec.LookPath("mv")
	if err != nil {
		t.Fatal(err)
	}
	fakeBin := filepath.Join(root, "fake-bin")
	if err := os.Mkdir(fakeBin, 0o700); err != nil {
		t.Fatal(err)
	}
	fakeCp := `#!/usr/bin/env bash
set -euo pipefail
"$WAHRWELT_REAL_MV" -T -- "$WAHRWELT_LIVE_DIR" "$WAHRWELT_DISPLACED_DIR"
"$WAHRWELT_REAL_LN" -s -- "$WAHRWELT_VICTIM_DIR" "$WAHRWELT_LIVE_DIR"
exec "$WAHRWELT_REAL_CP" "$@"
`
	if err := os.WriteFile(filepath.Join(fakeBin, "cp"), []byte(fakeCp), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", hyprUserAdapterGuard, "prepare", target, "../../../dots/hypr/hyprland.lua", "")
	cmd.Env = append(
		os.Environ(),
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"WAHRWELT_REAL_CP="+realCp,
		"WAHRWELT_REAL_LN="+realLn,
		"WAHRWELT_REAL_MV="+realMv,
		"WAHRWELT_LIVE_DIR="+liveDir,
		"WAHRWELT_DISPLACED_DIR="+displacedDir,
		"WAHRWELT_VICTIM_DIR="+victimDir,
	)
	if output, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("guard accepted a replaced adapter parent\n%s", output)
	}
	if got, err := os.Readlink(liveDir); err != nil || got != victimDir {
		t.Fatalf("unknown replacement parent changed: target=%q err=%v", got, err)
	}
	if _, err := os.Lstat(victimTarget); !os.IsNotExist(err) {
		t.Fatalf("guard published through replacement parent: err=%v", err)
	}
	if got := readContractFile(t, singleHistoricalAdapterRecovery(t, displacedDir)); got != legacy {
		t.Fatalf("historical recovery content = %q", got)
	}
}

func TestHyprUserAdapterGuardPublicationFailureRollsBackWithoutReplacingWinner(t *testing.T) {
	legacy := readContractFile(t, "../../../NixOS/home/shells/legacy-hypr-runtime/user-adapter-wahrwelt-v1.lua")
	realCurrent := "../../../dots/hypr/hyprland.lua"

	for _, mode := range []string{"target-absent", "race-regular", "race-symlink"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "hyprland.lua")
			if err := os.WriteFile(target, []byte(legacy), 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(target)
			if err != nil {
				t.Fatal(err)
			}
			fakeBin := filepath.Join(dir, "fake-bin")
			if err := os.Mkdir(fakeBin, 0o700); err != nil {
				t.Fatal(err)
			}
			fakeCp := `#!/usr/bin/env bash
set -euo pipefail
if [ "$WAHRWELT_CP_FAILURE_MODE" = race-regular ]; then
  printf '%s\n' '-- unknown publication winner' >"$WAHRWELT_ADAPTER_TARGET"
  chmod 0600 "$WAHRWELT_ADAPTER_TARGET"
elif [ "$WAHRWELT_CP_FAILURE_MODE" = race-symlink ]; then
  printf '%s\n' '-- unknown symlink destination' >"$WAHRWELT_SYMLINK_DESTINATION"
  ln -s -- "$WAHRWELT_SYMLINK_DESTINATION" "$WAHRWELT_ADAPTER_TARGET"
fi
exit 1
`
			if err := os.WriteFile(filepath.Join(fakeBin, "cp"), []byte(fakeCp), 0o755); err != nil {
				t.Fatal(err)
			}

			cmd := exec.Command("bash", hyprUserAdapterGuard, "prepare", target, realCurrent, "")
			cmd.Env = append(
				os.Environ(),
				"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
				"WAHRWELT_CP_FAILURE_MODE="+mode,
				"WAHRWELT_ADAPTER_TARGET="+target,
				"WAHRWELT_SYMLINK_DESTINATION="+filepath.Join(dir, "unknown.lua"),
			)
			if output, err := cmd.CombinedOutput(); err == nil {
				t.Fatalf("guard accepted failed adapter publication\n%s", output)
			}

			if mode == "target-absent" {
				if got := readContractFile(t, target); got != legacy {
					t.Fatalf("historical adapter was not restored: %q", got)
				}
				after, err := os.Stat(target)
				if err != nil || !os.SameFile(before, after) {
					t.Fatalf("historical adapter inode was not restored: before=%v after=%v err=%v", before, after, err)
				}
				return
			}
			if mode == "race-symlink" {
				want := filepath.Join(dir, "unknown.lua")
				if got, err := os.Readlink(target); err != nil || got != want {
					t.Fatalf("race-winning symlink changed: target=%q err=%v", got, err)
				}
				if got := readContractFile(t, want); got != "-- unknown symlink destination\n" {
					t.Fatalf("race-winning symlink destination changed: %q", got)
				}
				if got := readContractFile(t, singleHistoricalAdapterRecovery(t, dir)); got != legacy {
					t.Fatalf("retained historical recovery content = %q", got)
				}
				return
			}
			if got := readContractFile(t, target); got != "-- unknown publication winner\n" {
				t.Fatalf("race-winning adapter changed: %q", got)
			}
			if got := readContractFile(t, singleHistoricalAdapterRecovery(t, dir)); got != legacy {
				t.Fatalf("retained historical recovery content = %q", got)
			}
		})
	}
}

func TestHyprUserAdapterGuardRestoresReplacementMovedDuringPreparation(t *testing.T) {
	legacy := readContractFile(t, "../../../NixOS/home/shells/legacy-hypr-runtime/user-adapter-wahrwelt-v1.lua")
	realMv, err := exec.LookPath("mv")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "hyprland.lua")
	saved := filepath.Join(dir, "expected-hyprland.lua")
	if err := os.WriteFile(target, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	fakeBin := filepath.Join(dir, "fake-bin")
	if err := os.Mkdir(fakeBin, 0o700); err != nil {
		t.Fatal(err)
	}
	fakeMv := `#!/usr/bin/env bash
set -euo pipefail
if [ ! -e "$WAHRWELT_MV_RACE_MARKER" ]; then
  : >"$WAHRWELT_MV_RACE_MARKER"
  "$WAHRWELT_REAL_MV" -T -- "$WAHRWELT_ADAPTER_TARGET" "$WAHRWELT_ADAPTER_SAVED"
  printf '%s\n' '-- unrelated replacement' >"$WAHRWELT_ADAPTER_TARGET"
  chmod 0600 -- "$WAHRWELT_ADAPTER_TARGET"
fi
exec "$WAHRWELT_REAL_MV" "$@"
`
	if err := os.WriteFile(filepath.Join(fakeBin, "mv"), []byte(fakeMv), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(
		"bash", hyprUserAdapterGuard, "prepare", target,
		"../../../dots/hypr/hyprland.lua", "",
	)
	cmd.Env = append(
		os.Environ(),
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"WAHRWELT_REAL_MV="+realMv,
		"WAHRWELT_MV_RACE_MARKER="+filepath.Join(dir, "race-fired"),
		"WAHRWELT_ADAPTER_TARGET="+target,
		"WAHRWELT_ADAPTER_SAVED="+saved,
	)
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "concurrent replacement restored") {
		t.Fatalf("guard accepted a replacement moved during preparation: err=%v\n%s", err, output)
	}
	if got := readContractFile(t, target); got != "-- unrelated replacement\n" {
		t.Fatalf("replacement adapter changed: %q", got)
	}
	if got := readContractFile(t, saved); got != legacy {
		t.Fatalf("expected historical adapter changed: %q", got)
	}
}

func TestRuntimeSeedUsesAnonymousPinnedPublication(t *testing.T) {
	helper := readContractFile(t, "../../../NixOS/home/shells/runtime-activation.sh")
	for _, want := range []string{"os.O_TMPFILE", "AT_EMPTY_PATH", "linkat"} {
		if !strings.Contains(helper, want) {
			t.Fatalf("seed helper is missing anonymous publication primitive %q", want)
		}
	}
	for _, forbidden := range []string{`mktemp "$target_dir/.${target_name}.seed.XXXXXX"`, `trap 'rm -f -- "$tmp"' RETURN`} {
		if strings.Contains(helper, forbidden) {
			t.Fatalf("seed helper retains replaceable public temp cleanup %q", forbidden)
		}
	}
}

func singleHistoricalAdapterRecovery(t *testing.T, dir string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".wahrwelt-hyprland-recovery.*", "previous.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("historical recovery count = %d, want 1: %v", len(matches), matches)
	}
	return matches[0]
}

func runSeedExclusiveAtBarrier(t *testing.T, helper, source, target string, wantSuccess bool, mutate func()) {
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
	cmd := exec.Command("bash", helper, "seed-exclusive", source, target, "Wahrwelt user config")
	cmd.Env = append(
		os.Environ(),
		"WAHRWELT_TEST_SEED_READY_FD=3",
		"WAHRWELT_TEST_SEED_CONTINUE_FD=4",
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
		_, err := io.ReadFull(readyR, marker)
		if err == nil && string(marker) != "ready\n" {
			err = fmt.Errorf("unexpected seed barrier marker %q", marker)
		}
		ready <- err
	}()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatalf("seed did not reach publication barrier: %v\n%s", err, output.String())
		}
	case err := <-done:
		finished = true
		t.Fatalf("seed exited before publication barrier: %v\n%s", err, output.String())
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for seed barrier\n%s", output.String())
	}

	mutate()
	if _, err := continueW.Write([]byte{'1'}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		finished = true
		if wantSuccess && err != nil {
			t.Fatalf("seed failed after preserving race winner: %v\n%s", err, output.String())
		}
		if !wantSuccess && err == nil {
			t.Fatalf("seed accepted a replaced parent\n%s", output.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("seed did not finish after publication barrier\n%s", output.String())
	}
}
