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

const runtimeActivationHelper = "../../../NixOS/home/shells/runtime-activation.sh"

type activationBarrierProcess struct {
	cmd       *exec.Cmd
	output    *bytes.Buffer
	continueW *os.File
	done      chan error
	finished  bool
}

func startActivationBarrier(t *testing.T, barrier string, args ...string) *activationBarrierProcess {
	t.Helper()
	return startScriptBarrier(t, runtimeActivationHelper, barrier, args...)
}

func startScriptBarrier(t *testing.T, script, barrier string, args ...string) *activationBarrierProcess {
	t.Helper()
	readyR, readyW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	continueR, continueW, err := os.Pipe()
	if err != nil {
		_ = readyR.Close()
		_ = readyW.Close()
		t.Fatal(err)
	}
	output := &bytes.Buffer{}
	cmd := exec.Command("bash", append([]string{script}, args...)...)
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("WAHRWELT_TEST_%s_READY_FD=3", barrier),
		fmt.Sprintf("WAHRWELT_TEST_%s_CONTINUE_FD=4", barrier),
	)
	cmd.ExtraFiles = []*os.File{readyW, continueR}
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Start(); err != nil {
		_ = readyR.Close()
		_ = readyW.Close()
		_ = continueR.Close()
		_ = continueW.Close()
		t.Fatal(err)
	}
	_ = readyW.Close()
	_ = continueR.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	ready := make(chan error, 1)
	go func() {
		defer readyR.Close()
		marker := make([]byte, len("ready\n"))
		_, err := io.ReadFull(readyR, marker)
		if err == nil && string(marker) != "ready\n" {
			err = fmt.Errorf("unexpected activation barrier marker %q", marker)
		}
		ready <- err
	}()
	select {
	case err := <-ready:
		if err != nil {
			_ = continueW.Close()
			_ = cmd.Process.Kill()
			<-done
			t.Fatalf("helper did not reach %s barrier: %v\n%s", barrier, err, output.String())
		}
	case err := <-done:
		_ = continueW.Close()
		t.Fatalf("helper exited before %s barrier: %v\n%s", barrier, err, output.String())
	case <-time.After(5 * time.Second):
		_ = continueW.Close()
		_ = cmd.Process.Kill()
		<-done
		t.Fatalf("timed out waiting for %s barrier\n%s", barrier, output.String())
	}

	process := &activationBarrierProcess{
		cmd:       cmd,
		output:    output,
		continueW: continueW,
		done:      done,
	}
	t.Cleanup(func() {
		if process.finished {
			return
		}
		_, _ = process.continueW.Write([]byte{'1'})
		_ = process.continueW.Close()
		_ = process.cmd.Process.Kill()
		<-process.done
	})
	return process
}

func TestHomeManagerRuntimeActivationAcceptsFailedGenerationStableStoreLink(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}
	stableSource := filepath.Join(t.TempDir(), "stable-hyprland.lua")
	const stable = "-- stable failed-generation delegator\n"
	if err := os.WriteFile(stableSource, []byte(stable), 0o444); err != nil {
		t.Fatal(err)
	}
	stableStore := addNixStorePath(t, "hm_hyprhyprland.lua", stableSource)
	filesTree := filepath.Join(t.TempDir(), "home-manager-files")
	managedTarget := filepath.Join(filesTree, ".config", "hypr", "hyprland.lua")
	if err := os.MkdirAll(filepath.Dir(managedTarget), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(stableStore, managedTarget); err != nil {
		t.Fatal(err)
	}
	filesStore := addNixStorePath(t, "home-manager-files", filesTree)
	failedGenerationTarget := filepath.Join(filesStore, ".config", "hypr", "hyprland.lua")
	liveTarget := filepath.Join(t.TempDir(), "hypr", "hyprland.lua")
	if err := os.MkdirAll(filepath.Dir(liveTarget), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(failedGenerationTarget, liveTarget); err != nil {
		t.Fatal(err)
	}

	output, err := exec.Command(
		"bash",
		runtimeActivationHelper,
		"classify-top-level-runtime",
		liveTarget,
		"",
		".config/hypr/hyprland.lua",
		stableStore,
		stableStore,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("failed-generation Home Manager runtime link was rejected: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(string(output)); !strings.HasPrefix(got, "stable-link|") {
		t.Fatalf("failed-generation runtime classification = %q, want stable-link", got)
	}
	if got, err := os.Readlink(liveTarget); err != nil || got != failedGenerationTarget {
		t.Fatalf("failed-generation runtime link changed: target=%q err=%v", got, err)
	}
}

func TestHomeManagerRuntimeActivationRejectsStoreLinkToMutableStablePayload(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}
	const stable = "-- stable mutable delegator\n"
	stableSource := filepath.Join(t.TempDir(), "stable-hyprland.lua")
	if err := os.WriteFile(stableSource, []byte(stable), 0o444); err != nil {
		t.Fatal(err)
	}
	stableStore := addNixStorePath(t, "hm_hyprhyprland.lua", stableSource)
	mutablePayload := filepath.Join(t.TempDir(), "mutable-hyprland.lua")
	if err := os.WriteFile(mutablePayload, []byte(stable), 0o644); err != nil {
		t.Fatal(err)
	}
	filesTree := filepath.Join(t.TempDir(), "home-manager-files")
	managedTarget := filepath.Join(filesTree, ".config", "hypr", "hyprland.lua")
	if err := os.MkdirAll(filepath.Dir(managedTarget), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(mutablePayload, managedTarget); err != nil {
		t.Fatal(err)
	}
	filesStore := addNixStorePath(t, "home-manager-files", filesTree)
	storeTarget := filepath.Join(filesStore, ".config", "hypr", "hyprland.lua")
	liveTarget := filepath.Join(t.TempDir(), "hypr", "hyprland.lua")
	if err := os.MkdirAll(filepath.Dir(liveTarget), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(storeTarget, liveTarget); err != nil {
		t.Fatal(err)
	}

	output, err := exec.Command(
		"bash",
		runtimeActivationHelper,
		"classify-top-level-runtime",
		liveTarget,
		"",
		".config/hypr/hyprland.lua",
		stableStore,
		stableStore,
	).CombinedOutput()
	if err == nil || !strings.Contains(string(output), "ownership collision") {
		t.Fatalf("store link to mutable stable payload was accepted: err=%v\n%s", err, output)
	}
	if got, err := os.Readlink(liveTarget); err != nil || got != storeTarget {
		t.Fatalf("rejected mutable runtime link changed: target=%q err=%v", got, err)
	}
}

func (process *activationBarrierProcess) releaseExpectFailure(t *testing.T) string {
	t.Helper()
	if _, err := process.continueW.Write([]byte{'1'}); err != nil {
		t.Fatal(err)
	}
	_ = process.continueW.Close()
	select {
	case err := <-process.done:
		process.finished = true
		if err == nil {
			t.Fatalf("activation helper accepted an ownership race\n%s", process.output.String())
		}
	case <-time.After(5 * time.Second):
		_ = process.cmd.Process.Kill()
		<-process.done
		process.finished = true
		t.Fatalf("activation helper did not finish\n%s", process.output.String())
	}
	return process.output.String()
}

func TestHomeManagerRuntimeActivationRejectsPostValidationWriter(t *testing.T) {
	legacyPaths, legacy := runtimeLegacyFixtures(t)
	dir := t.TempDir()
	canonical := filepath.Join(dir, "canonical.lua")
	target := filepath.Join(dir, "hyprland.lua")
	if err := os.WriteFile(canonical, []byte("-- canonical Wahrwelt runtime\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, legacy, 0o600); err != nil {
		t.Fatal(err)
	}

	process := startActivationBarrier(
		t,
		"MIGRATION",
		"migrate-known-runtime",
		target,
		canonical,
		legacyPaths[0],
		legacyPaths[1],
	)
	unknownContent := "-- post-lease separate-process writer\n"
	writer := exec.Command("python3", "-I", "-S", "-c", `
import os
import sys
fd = os.open(sys.argv[1], os.O_WRONLY | os.O_TRUNC | os.O_NOFOLLOW)
try:
    os.write(fd, sys.argv[2].encode())
    os.fchmod(fd, 0o600)
    os.fsync(fd)
finally:
    os.close(fd)
`, target, unknownContent)
	writerDone := make(chan error, 1)
	if err := writer.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { writerDone <- writer.Wait() }()
	select {
	case err := <-writerDone:
		t.Fatalf("writer was not blocked by the runtime lease: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	output := process.releaseExpectFailure(t)
	if !strings.Contains(output, "ownership collision") {
		t.Fatalf("unsafe legacy runtime failure is not an ownership collision\n%s", output)
	}
	select {
	case err := <-writerDone:
		if err != nil {
			t.Fatalf("writer failed after activation released the lease: %v", err)
		}
	case <-time.After(5 * time.Second):
		_ = writer.Process.Kill()
		<-writerDone
		t.Fatal("writer stayed blocked after activation failure")
	}
	if got := readContractFile(t, target); got != unknownContent {
		t.Fatalf("unsafe migration changed the writer-owned inode: %q", got)
	}
}

func TestHomeManagerRuntimeActivationRejectsExternalHardlinkAtBarrier(t *testing.T) {
	legacyPaths, legacy := runtimeLegacyFixtures(t)
	dir := t.TempDir()
	canonical := filepath.Join(dir, "canonical.lua")
	target := filepath.Join(dir, "hyprland.lua")
	external := filepath.Join(dir, "external-hardlink.lua")
	if err := os.WriteFile(canonical, []byte("-- canonical Wahrwelt runtime\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}

	process := startActivationBarrier(
		t,
		"MIGRATION",
		"migrate-known-runtime",
		target,
		canonical,
		legacyPaths[0],
		legacyPaths[1],
	)
	if err := os.Link(target, external); err != nil {
		t.Fatal(err)
	}
	output := process.releaseExpectFailure(t)
	if !strings.Contains(output, "ownership collision") {
		t.Fatalf("hard-linked legacy runtime failure is not an ownership collision\n%s", output)
	}
	after, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	externalInfo, err := os.Stat(external)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || !os.SameFile(after, externalInfo) {
		t.Fatal("unsafe migration replaced the externally hard-linked inode")
	}
	if got := readContractFile(t, target); got != string(legacy) {
		t.Fatalf("unsafe migration changed legacy content: %q", got)
	}
}

func TestHomeManagerRuntimeActivationRejectsUnknownManagedEntrypoint(t *testing.T) {
	legacyPaths, _ := runtimeLegacyFixtures(t)
	dir := t.TempDir()
	runtimeDir := filepath.Join(dir, "runtime")
	if err := os.Mkdir(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(runtimeDir, "hyprland.lua")
	unknown := "-- private unknown managed runtime\n"
	if err := os.WriteFile(target, []byte(unknown), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	canonical := filepath.Join(dir, "canonical.lua")
	if err := os.WriteFile(canonical, []byte("-- canonical\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(
		"bash",
		runtimeActivationHelper,
		"activate-runtime-dir",
		runtimeDir,
		canonical,
		legacyPaths[0],
		legacyPaths[1],
		legacyPaths[0],
		legacyPaths[1],
		canonical,
		canonical,
		canonical,
		canonical,
		canonical,
	)
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "unknown regular runtime entrypoint") {
		t.Fatalf("unknown managed runtime did not fail closed: err=%v\n%s", err, output)
	}
	after, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || after.Mode().Perm() != 0o600 {
		t.Fatalf("unknown managed runtime identity changed: before=%v after=%v", before, after)
	}
	if got := readContractFile(t, target); got != unknown {
		t.Fatalf("unknown managed runtime content changed: %q", got)
	}
	entries, err := os.ReadDir(runtimeDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "hyprland.lua" {
		t.Fatalf("activation seeded files after unknown runtime collision: %v", entries)
	}
}

func TestHomeManagerActivationExchangeRollbackRequiresExactPairAndContent(t *testing.T) {
	legacyPaths, runtimeLegacy := runtimeLegacyFixtures(t)
	adapterLegacy := readContractFile(t, "../../../NixOS/home/migrations/v1_to_v2/hypr-runtime/user-adapter-wahrwelt-v1.lua")
	adapterCurrent := readContractFile(t, "../../../dots/hypr/hyprland.lua")
	for _, tree := range []string{"adapter", "runtime"} {
		for _, race := range []string{"recovery-replacement", "candidate-write"} {
			t.Run(tree+"-"+race, func(t *testing.T) {
				dir := t.TempDir()
				managedDir := filepath.Join(dir, tree)
				if err := os.Mkdir(managedDir, 0o700); err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(managedDir, "hyprland.lua")
				canonical := filepath.Join(dir, "canonical.lua")
				var barrier string
				var args []string
				var legacyContent, canonicalContent string
				if tree == "adapter" {
					barrier = "ADAPTER_EXCHANGE"
					legacyContent = adapterLegacy
					canonicalContent = adapterCurrent
					if err := os.WriteFile(target, []byte(legacyContent), 0o600); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(canonical, []byte("-- managed default\n"), 0o600); err != nil {
						t.Fatal(err)
					}
					args = []string{
						"activate-user-dir",
						managedDir,
						"../../../dots/hypr/hyprland.lua",
						"",
						canonical,
					}
				} else {
					barrier = "MIGRATION_EXCHANGE"
					legacyContent = string(runtimeLegacy)
					canonicalContent = "-- canonical runtime\n"
					if err := os.WriteFile(target, []byte(legacyContent), 0o600); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(canonical, []byte(canonicalContent), 0o600); err != nil {
						t.Fatal(err)
					}
					args = []string{
						"activate-runtime-dir",
						managedDir,
						canonical,
						legacyPaths[0],
						legacyPaths[1],
						legacyPaths[0],
						legacyPaths[1],
						canonical,
						canonical,
						canonical,
						canonical,
						canonical,
					}
				}
				process := startActivationBarrier(t, barrier, args...)
				recovery := singleRuntimeAdapterRecovery(t, managedDir)
				unknown := "-- unknown exchange replacement\n"
				if race == "recovery-replacement" {
					savedRecovery := recovery + ".saved"
					if err := os.Rename(recovery, savedRecovery); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(recovery, []byte(unknown), 0o600); err != nil {
						t.Fatal(err)
					}
					output := process.releaseExpectFailure(t)
					if !strings.Contains(output, "retained both") {
						t.Fatalf("uncertain pair did not report both retained paths\n%s", output)
					}
					if got := readContractFile(t, target); got != canonicalContent {
						t.Fatalf("canonical target changed after recovery replacement: %q", got)
					}
					if got := readContractFile(t, recovery); got != unknown {
						t.Fatalf("unknown recovery replacement changed: %q", got)
					}
					if got := readContractFile(t, savedRecovery); got != legacyContent {
						t.Fatalf("displaced original recovery changed: %q", got)
					}
					return
				}
				if err := os.WriteFile(target, []byte(unknown), 0o644); err != nil {
					t.Fatal(err)
				}
				output := process.releaseExpectFailure(t)
				if !strings.Contains(output, "rolled back") {
					t.Fatalf("candidate content race was not rolled back\n%s", output)
				}
				if got := readContractFile(t, target); got != legacyContent {
					t.Fatalf("legacy target was not restored: %q", got)
				}
				if got := readContractFile(t, recovery); got != unknown {
					t.Fatalf("corrupted candidate recovery changed: %q", got)
				}
			})
		}
	}
}

func TestHomeManagerRuntimeActivationRejectsFinalManagedEntrypointWrite(t *testing.T) {
	legacyPaths, _ := runtimeLegacyFixtures(t)
	dir := t.TempDir()
	runtimeDir := filepath.Join(dir, "runtime")
	if err := os.Mkdir(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	canonical := filepath.Join(dir, "canonical.lua")
	canonicalContent := "-- canonical runtime\n"
	if err := os.WriteFile(canonical, []byte(canonicalContent), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(runtimeDir, "hyprland.lua")
	if err := os.WriteFile(target, []byte(canonicalContent), 0o644); err != nil {
		t.Fatal(err)
	}
	process := startActivationBarrier(
		t,
		"RUNTIME_FINAL",
		"activate-runtime-dir",
		runtimeDir,
		canonical,
		legacyPaths[0],
		legacyPaths[1],
		legacyPaths[0],
		legacyPaths[1],
		canonical,
		canonical,
		canonical,
		canonical,
		canonical,
	)
	unknown := "-- late managed runtime writer\n"
	if err := os.WriteFile(target, []byte(unknown), 0o644); err != nil {
		t.Fatal(err)
	}
	output := process.releaseExpectFailure(t)
	if !strings.Contains(output, "managed entrypoint content is not canonical") {
		t.Fatalf("late managed runtime writer did not fail final token\n%s", output)
	}
	if got := readContractFile(t, target); got != unknown {
		t.Fatalf("late writer content changed: %q", got)
	}
}

func TestHomeManagerActivationPinsUserAndRuntimeDirectories(t *testing.T) {
	legacyPaths, _ := runtimeLegacyFixtures(t)
	for _, tree := range []string{"user", "runtime"} {
		for _, initially := range []string{"existing", "created"} {
			for _, replacement := range []string{"directory", "symlink"} {
				t.Run(tree+"-"+initially+"-"+replacement, func(t *testing.T) {
					root := t.TempDir()
					live := filepath.Join(root, tree)
					displaced := filepath.Join(root, tree+"-displaced")
					victim := filepath.Join(root, "victim")
					if initially == "existing" {
						if err := os.Mkdir(live, 0o700); err != nil {
							t.Fatal(err)
						}
					}
					if err := os.Mkdir(victim, 0o700); err != nil {
						t.Fatal(err)
					}
					sentinel := filepath.Join(victim, "sentinel")
					if err := os.WriteFile(sentinel, []byte("unknown replacement\n"), 0o600); err != nil {
						t.Fatal(err)
					}
					canonical := filepath.Join(root, "canonical.lua")
					if err := os.WriteFile(canonical, []byte("-- canonical\n"), 0o600); err != nil {
						t.Fatal(err)
					}

					var args []string
					if tree == "user" {
						args = []string{
							"activate-user-dir",
							live,
							"../../../dots/hypr/hyprland.lua",
							"",
							canonical,
						}
					} else {
						args = []string{
							"activate-runtime-dir",
							live,
							canonical,
							legacyPaths[0],
							legacyPaths[1],
							legacyPaths[0],
							legacyPaths[1],
							canonical,
							canonical,
							canonical,
							canonical,
							canonical,
						}
					}
					process := startActivationBarrier(t, "ACTIVATION", args...)
					if err := os.Rename(live, displaced); err != nil {
						t.Fatal(err)
					}
					if replacement == "directory" {
						if err := os.Mkdir(live, 0o700); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(filepath.Join(live, "sentinel"), []byte("ordinary replacement\n"), 0o600); err != nil {
							t.Fatal(err)
						}
					} else if err := os.Symlink(victim, live); err != nil {
						t.Fatal(err)
					}
					output := process.releaseExpectFailure(t)
					if !strings.Contains(output, "ownership collision") {
						t.Fatalf("directory replacement failure is not an ownership collision\n%s", output)
					}
					if got := readContractFile(t, sentinel); got != "unknown replacement\n" {
						t.Fatalf("victim changed: %q", got)
					}
					if replacement == "directory" {
						if got := readContractFile(t, filepath.Join(live, "sentinel")); got != "ordinary replacement\n" {
							t.Fatalf("ordinary replacement changed: %q", got)
						}
					}
					entries, err := os.ReadDir(displaced)
					if err != nil {
						t.Fatal(err)
					}
					if len(entries) != 0 {
						t.Fatalf("pinned displaced tree was mutated: %v", entries)
					}
				})
			}
		}
	}
}

func TestHomeManagerActivationRejectsCreatedDirectorySwapBeforePin(t *testing.T) {
	legacyPaths, _ := runtimeLegacyFixtures(t)
	for _, tree := range []string{"user", "runtime"} {
		for _, replacement := range []string{"directory", "symlink"} {
			t.Run(tree+"-"+replacement, func(t *testing.T) {
				root := t.TempDir()
				live := filepath.Join(root, tree)
				displaced := filepath.Join(root, tree+"-created")
				victim := filepath.Join(root, "victim")
				if err := os.Mkdir(victim, 0o700); err != nil {
					t.Fatal(err)
				}
				sentinel := filepath.Join(victim, "sentinel")
				if err := os.WriteFile(sentinel, []byte("unknown victim\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				canonical := filepath.Join(root, "canonical.lua")
				if err := os.WriteFile(canonical, []byte("-- canonical\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				var args []string
				if tree == "user" {
					args = []string{
						"activate-user-dir",
						live,
						"../../../dots/hypr/hyprland.lua",
						"",
						canonical,
					}
				} else {
					args = []string{
						"activate-runtime-dir",
						live,
						canonical,
						legacyPaths[0],
						legacyPaths[1],
						legacyPaths[0],
						legacyPaths[1],
						canonical,
						canonical,
						canonical,
						canonical,
						canonical,
					}
				}
				process := startActivationBarrier(t, "DIRECTORY", args...)
				if err := os.Rename(live, displaced); err != nil {
					t.Fatal(err)
				}
				if replacement == "directory" {
					if err := os.Mkdir(live, 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(live, "unknown"), []byte("unknown replacement\n"), 0o600); err != nil {
						t.Fatal(err)
					}
				} else if err := os.Symlink(victim, live); err != nil {
					t.Fatal(err)
				}
				output := process.releaseExpectFailure(t)
				if !strings.Contains(output, "created directory changed before pin") &&
					!strings.Contains(output, "cannot pin created directory") {
					t.Fatalf("created-directory swap did not fail its identity bridge\n%s", output)
				}
				if got := readContractFile(t, sentinel); got != "unknown victim\n" {
					t.Fatalf("victim changed: %q", got)
				}
				if replacement == "directory" {
					if got := readContractFile(t, filepath.Join(live, "unknown")); got != "unknown replacement\n" {
						t.Fatalf("ordinary replacement changed: %q", got)
					}
				}
				entries, err := os.ReadDir(displaced)
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) != 0 {
					t.Fatalf("displaced created directory was mutated: %v", entries)
				}
			})
		}
	}
}

func TestHomeManagerUserActivationMigratesAdapterAndSeedsDefault(t *testing.T) {
	dir := t.TempDir()
	userDir := filepath.Join(dir, "user")
	if err := os.Mkdir(userDir, 0o700); err != nil {
		t.Fatal(err)
	}
	adapter := filepath.Join(userDir, "hyprland.lua")
	legacy := readContractFile(t, "../../../NixOS/home/migrations/v1_to_v2/hypr-runtime/user-adapter-wahrwelt-v1.lua")
	if err := os.WriteFile(adapter, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	defaultSource := filepath.Join(dir, "default-source.lua")
	defaultContent := "-- canonical default\n"
	if err := os.WriteFile(defaultSource, []byte(defaultContent), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(
		"bash",
		runtimeActivationHelper,
		"activate-user-dir",
		userDir,
		"../../../dots/hypr/hyprland.lua",
		"",
		defaultSource,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("user activation failed: %v\n%s", err, output)
	}
	if got := readContractFile(t, adapter); got != readContractFile(t, "../../../dots/hypr/hyprland.lua") {
		t.Fatal("user activation did not publish the current adapter")
	}
	defaultPath := filepath.Join(userDir, "default.lua")
	if got := readContractFile(t, defaultPath); got != defaultContent {
		t.Fatalf("user activation default content = %q", got)
	}
	defaultInfo, err := os.Lstat(defaultPath)
	if err != nil || defaultInfo.Mode().Perm() != 0o644 {
		t.Fatalf("user activation default mode: info=%v err=%v", defaultInfo, err)
	}
	recoveries, err := filepath.Glob(filepath.Join(userDir, ".hyprland.lua.wahrwelt-recovery.*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(recoveries) != 1 || readContractFile(t, recoveries[0]) != legacy {
		t.Fatalf("historical adapter recovery = %v", recoveries)
	}
}

func TestHomeManagerUserActivationRejectsAdapterSwapBeforeFinalSuccess(t *testing.T) {
	dir := t.TempDir()
	userDir := filepath.Join(dir, "user")
	if err := os.Mkdir(userDir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(userDir, "hyprland.lua")
	legacy := readContractFile(t, "../../../NixOS/home/migrations/v1_to_v2/hypr-runtime/user-adapter-wahrwelt-v1.lua")
	current := readContractFile(t, "../../../dots/hypr/hyprland.lua")
	if err := os.WriteFile(target, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	defaultSource := filepath.Join(dir, "default-source.lua")
	if err := os.WriteFile(defaultSource, []byte("-- managed default\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	process := startActivationBarrier(
		t,
		"SEED",
		"activate-user-dir",
		userDir,
		"../../../dots/hypr/hyprland.lua",
		"",
		defaultSource,
	)
	savedCurrent := filepath.Join(userDir, "saved-current.lua")
	if err := os.Rename(target, savedCurrent); err != nil {
		t.Fatal(err)
	}
	unknown := "-- unknown post-publication adapter\n"
	if err := os.WriteFile(target, []byte(unknown), 0o600); err != nil {
		t.Fatal(err)
	}
	output := process.releaseExpectFailure(t)
	if !strings.Contains(output, "adapter changed after guarded preparation") {
		t.Fatalf("adapter swap did not fail final token validation\n%s", output)
	}
	if got := readContractFile(t, target); got != unknown {
		t.Fatalf("unknown adapter winner changed: %q", got)
	}
	if got := readContractFile(t, savedCurrent); got != current {
		t.Fatalf("displaced current adapter changed: %q", got)
	}
	if got := readContractFile(t, singleRuntimeAdapterRecovery(t, userDir)); got != legacy {
		t.Fatalf("legacy adapter recovery changed: %q", got)
	}
}

func TestHomeManagerUserActivationPreservesOwnedDefaultNodes(t *testing.T) {
	for _, kind := range []string{"regular", "symlink", "broken-symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			userDir := filepath.Join(dir, "user")
			if err := os.Mkdir(userDir, 0o700); err != nil {
				t.Fatal(err)
			}
			defaultPath := filepath.Join(userDir, "default.lua")
			linkTarget := filepath.Join(userDir, "owned.lua")
			switch kind {
			case "regular":
				if err := os.WriteFile(defaultPath, []byte("-- owned regular\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.WriteFile(linkTarget, []byte("-- owned target\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(linkTarget, defaultPath); err != nil {
					t.Fatal(err)
				}
			case "broken-symlink":
				if err := os.Symlink(linkTarget, defaultPath); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(defaultPath, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			defaultSource := filepath.Join(dir, "default-source.lua")
			if err := os.WriteFile(defaultSource, []byte("-- managed default\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(
				"bash",
				runtimeActivationHelper,
				"activate-user-dir",
				userDir,
				"../../../dots/hypr/hyprland.lua",
				"",
				defaultSource,
			)
			output, err := cmd.CombinedOutput()
			if kind == "directory" {
				if err == nil || !strings.Contains(string(output), "Refusing non-regular") {
					t.Fatalf("non-regular default collision accepted: err=%v\n%s", err, output)
				}
				info, statErr := os.Lstat(defaultPath)
				if statErr != nil || !info.IsDir() {
					t.Fatalf("default directory collision changed: info=%v err=%v", info, statErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("owned default node rejected: %v\n%s", err, output)
			}
			switch kind {
			case "regular":
				if got := readContractFile(t, defaultPath); got != "-- owned regular\n" {
					t.Fatalf("owned regular default changed: %q", got)
				}
			case "symlink", "broken-symlink":
				if got, readErr := os.Readlink(defaultPath); readErr != nil || got != linkTarget {
					t.Fatalf("owned default symlink changed: target=%q err=%v", got, readErr)
				}
			}
		})
	}
}

func TestHomeManagerUserActivationAcceptsOnlyExactOldGenerationAdapterLink(t *testing.T) {
	legacy := readContractFile(t, "../../../NixOS/home/migrations/v1_to_v2/hypr-runtime/user-adapter-wahrwelt-v1.lua")
	for _, owned := range []bool{true, false} {
		name := "unowned"
		if owned {
			name = "owned"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			oldGeneration := filepath.Join(dir, "old-generation")
			expected := filepath.Join(oldGeneration, "home-files", ".config", "hypr", "wahrwelt", "hyprland.lua")
			if err := os.MkdirAll(filepath.Dir(expected), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(expected, []byte(legacy), 0o444); err != nil {
				t.Fatal(err)
			}
			userDir := filepath.Join(dir, "user")
			if err := os.Mkdir(userDir, 0o700); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(userDir, "hyprland.lua")
			linkTarget := expected
			if !owned {
				linkTarget = filepath.Join(dir, "unowned.lua")
				if err := os.WriteFile(linkTarget, []byte(legacy), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(linkTarget, target); err != nil {
				t.Fatal(err)
			}
			defaultSource := filepath.Join(dir, "default-source.lua")
			if err := os.WriteFile(defaultSource, []byte("-- managed default\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(
				"bash",
				runtimeActivationHelper,
				"activate-user-dir",
				userDir,
				"../../../dots/hypr/hyprland.lua",
				oldGeneration,
				defaultSource,
			)
			output, err := cmd.CombinedOutput()
			if owned && err != nil {
				t.Fatalf("exact old generation link rejected: %v\n%s", err, output)
			}
			if !owned && (err == nil || !strings.Contains(string(output), "ownership collision")) {
				t.Fatalf("unowned adapter link accepted: err=%v\n%s", err, output)
			}
			if got, readErr := os.Readlink(target); readErr != nil || got != linkTarget {
				t.Fatalf("adapter link changed: target=%q err=%v", got, readErr)
			}
		})
	}
}

func TestHomeManagerUserActivationAcceptsOnlyKnownNixOSManagedAdapterLink(t *testing.T) {
	currentPath := "../../../dots/hypr/hyprland.lua"
	current := readContractFile(t, currentPath)
	fixtures := map[string]struct {
		content string
		wantOK  bool
	}{
		"current": {
			content: current,
			wantOK:  true,
		},
		"historical": {
			content: readContractFile(t, "../../../NixOS/home/migrations/v1_to_v2/hypr-runtime/user-adapter-wahrwelt-v1.lua"),
			wantOK:  true,
		},
		"unknown": {
			content: "-- unknown store adapter\n",
		},
	}
	for name, fixture := range fixtures {
		t.Run(name, func(t *testing.T) {
			managed := addHomeManagerAdapterStoreFixture(t, "user", fixture.content)
			dir := t.TempDir()
			userDir := filepath.Join(dir, "user")
			if err := os.Mkdir(userDir, 0o700); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(userDir, "hyprland.lua")
			if err := os.Symlink(managed.adapter, target); err != nil {
				t.Fatal(err)
			}
			oldManaged := addHomeManagerAdapterStoreFixture(t, "wahrwelt", fixture.content)
			unrelatedGeneration := filepath.Join(dir, "unrelated-generation")
			if err := os.MkdirAll(unrelatedGeneration, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(oldManaged.root, filepath.Join(unrelatedGeneration, "home-files")); err != nil {
				t.Fatal(err)
			}
			defaultSource := filepath.Join(dir, "default-source.lua")
			if err := os.WriteFile(defaultSource, []byte("-- managed default\n"), 0o600); err != nil {
				t.Fatal(err)
			}

			output, err := exec.Command(
				"bash",
				runtimeActivationHelper,
				"activate-user-dir",
				userDir,
				currentPath,
				unrelatedGeneration,
				defaultSource,
			).CombinedOutput()
			if fixture.wantOK && err != nil {
				t.Fatalf("known NixOS-managed adapter link rejected: %v\n%s", err, output)
			}
			if !fixture.wantOK && (err == nil || !strings.Contains(string(output), "ownership collision")) {
				t.Fatalf("unknown store adapter accepted: err=%v\n%s", err, output)
			}
			if got, readErr := os.Readlink(target); readErr != nil || got != managed.adapter {
				t.Fatalf("managed adapter link changed: target=%q err=%v", got, readErr)
			}
			if fixture.wantOK {
				if got := readContractFile(t, filepath.Join(userDir, "default.lua")); got != "-- managed default\n" {
					t.Fatalf("default seed content = %q", got)
				}
			}
		})
	}
}

func runtimeLegacyFixtures(t *testing.T) ([]string, []byte) {
	t.Helper()
	legacyDir := "../../../NixOS/home/migrations/v1_to_v2/hypr-runtime"
	paths := []string{
		filepath.Join(legacyDir, "end4.lua"),
		filepath.Join(legacyDir, "end4-pc.lua"),
	}
	legacy, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	return paths, legacy
}

func singleRuntimeAdapterRecovery(t *testing.T, dir string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".hyprland.lua.wahrwelt-recovery.*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("runtime adapter recovery count = %d, want 1: %v", len(matches), matches)
	}
	return matches[0]
}
