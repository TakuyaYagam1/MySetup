//go:build linux

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/fsowner"
	"golang.org/x/sys/unix"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && (os.Args[1] == "__runtime-lock-child" || os.Args[1] == "__runtime-lock-watchdog") {
		if err := run(os.Args[1:]); err != nil {
			var commandError commandExitError
			if errors.As(err, &commandError) {
				os.Exit(commandError.code)
			}
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestRunRuntimeTransactionCommitLeavesNoResidue(t *testing.T) {
	root := t.TempDir()
	session := filepath.Join(root, "session")
	runtime := filepath.Join(root, "runtime")
	if err := os.MkdirAll(runtime, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(runtime, "shell-profile.lua")
	if err := os.WriteFile(target, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	oldInput, oldOutput := commandInput, commandOutput
	commandOutput = &output
	t.Cleanup(func() { commandInput, commandOutput = oldInput, oldOutput })
	if err := run([]string{"runtime-begin", "--root", session, "--kind", "runtime", target}); err != nil {
		t.Fatalf("runtime-begin: %v", err)
	}
	transaction := strings.TrimSpace(output.String())
	commandInput = strings.NewReader("new\n")
	if err := run([]string{"runtime-write", "--transaction", transaction, "--target", target, "--mode", "0644"}); err != nil {
		t.Fatalf("runtime-write: %v", err)
	}
	if err := run([]string{"runtime-commit", transaction}); err != nil {
		t.Fatalf("runtime-commit: %v", err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "new\n" {
		t.Fatalf("target=%q err=%v", data, err)
	}
	entries, err := os.ReadDir(session)
	if err != nil || len(entries) != 0 {
		t.Fatalf("session residue=%v err=%v", entries, err)
	}
}

func TestRunRuntimeScavengeRejectsUnknownMatchingDirectory(t *testing.T) {
	root := t.TempDir()
	unknown := filepath.Join(root, ".runtime-rollback-v1-foreign")
	if err := os.Mkdir(unknown, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unknown, "foreign"), []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"runtime-scavenge", "--root", root, "--kind", "runtime"})
	if err == nil || !strings.Contains(err.Error(), "preserved") {
		t.Fatalf("runtime-scavenge error=%v, want preserved collision", err)
	}
	if data, readErr := os.ReadFile(filepath.Join(unknown, "foreign")); readErr != nil || string(data) != "keep\n" {
		t.Fatalf("unknown journal changed: data=%q err=%v", data, readErr)
	}
}

func TestRuntimeLockCommandsRequireExactRootNameAndIdentity(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", root)
	for _, args := range [][]string{
		{"runtime-lock-run", "--root", other, "--name", "wahrwelt-shell-v2.lock", "--", "true"},
		{"runtime-lock-run", "--root", root, "--name", "foreign.lock", "--", "true"},
		{"runtime-lock-run", "--root", root, "--name", "wahrwelt-shell-v2.lock"},
	} {
		if err := run(args); err == nil {
			t.Fatalf("run(%q) accepted an unsafe lock request", args)
		}
	}
}

func TestRuntimeLockPreservesExactChildExitStatus(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	err := runWithRuntimeLock(root, "wahrwelt-shell-v2.lock", 0, []string{"sh", "-c", "exit 37"})
	var exitError commandExitError
	if !errors.As(err, &exitError) || exitError.code != 37 {
		t.Fatalf("child exit = %v, want 37", err)
	}
}

func TestRuntimeLockNonzeroExitKillsBackgroundProcessGroup(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	backgroundPIDPath := filepath.Join(t.TempDir(), "background-pid")
	t.Setenv("WAHRWELT_TEST_NONZERO_CHILD", "1")
	t.Setenv("WAHRWELT_TEST_NONZERO_BACKGROUND_PID", backgroundPIDPath)
	err := runWithRuntimeLock(
		root,
		"wahrwelt-shell-v2.lock",
		0,
		[]string{os.Args[0], "-test.run=^TestRuntimeLockNonzeroChildProcess$"},
	)
	var exitError commandExitError
	if !errors.As(err, &exitError) || exitError.code != 37 {
		t.Fatalf("child exit = %v, want 37", err)
	}
	backgroundPID := readTestPID(t, backgroundPIDPath)
	t.Cleanup(func() { _ = unix.Kill(backgroundPID, unix.SIGKILL) })
	waitForProcessGone(t, backgroundPID)
	if err := runWithRuntimeLock(root, "wahrwelt-shell-v2.lock", 0, []string{"true"}); err != nil {
		t.Fatalf("lock stayed held after nonzero cleanup: %v", err)
	}
}

func TestRuntimeLockNonzeroChildProcess(t *testing.T) {
	if os.Getenv("WAHRWELT_TEST_NONZERO_CHILD") != "1" {
		return
	}
	background := exec.Command("sleep", "60")
	if err := background.Start(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		os.Getenv("WAHRWELT_TEST_NONZERO_BACKGROUND_PID"),
		[]byte(strconv.Itoa(background.Process.Pid)),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	os.Exit(37)
}

func TestRuntimeLockUsesTwoProcessKernelExclusionAndLeavesNoFilesystemAnchor(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	coordination := t.TempDir()
	ready := filepath.Join(coordination, "ready")
	release := filepath.Join(coordination, "release")
	name := "wahrwelt-shell-v2.lock"

	holder := exec.Command(os.Args[0], "-test.run=^TestRuntimeLockHolderProcess$")
	holder.Env = append(os.Environ(),
		"WAHRWELT_TEST_LOCK_HOLDER=1",
		"WAHRWELT_TEST_LOCK_ROOT="+root,
		"WAHRWELT_TEST_LOCK_NAME="+name,
		"WAHRWELT_TEST_LOCK_READY="+ready,
		"WAHRWELT_TEST_LOCK_RELEASE="+release,
	)
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.WriteFile(release, nil, 0o600)
		_ = holder.Process.Kill()
		_ = holder.Wait()
	})
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("holder child did not acquire the abstract socket lock")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := runWithRuntimeLock(root, name, 30*time.Millisecond, []string{"true"}); !errors.Is(err, errRuntimeLockBusy) {
		t.Fatalf("contending helper error = %v, want busy", err)
	}
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := holder.Wait(); err != nil {
		t.Fatal(err)
	}
	if err := runWithRuntimeLock(root, name, 0, []string{"true"}); err != nil {
		t.Fatalf("lock was not released by process exit: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("abstract lock left filesystem residue: entries=%v err=%v", entries, err)
	}
}

func TestRuntimeLockHolderProcess(t *testing.T) {
	if os.Getenv("WAHRWELT_TEST_LOCK_HOLDER") != "1" {
		return
	}
	command := []string{os.Args[0], "-test.run=^TestRuntimeLockHeldChildProcess$"}
	if err := runWithRuntimeLock(
		os.Getenv("WAHRWELT_TEST_LOCK_ROOT"),
		os.Getenv("WAHRWELT_TEST_LOCK_NAME"),
		0,
		command,
	); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeLockHeldChildProcess(t *testing.T) {
	if os.Getenv("WAHRWELT_TEST_LOCK_HOLDER") != "1" {
		return
	}
	if err := os.WriteFile(os.Getenv("WAHRWELT_TEST_LOCK_READY"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(os.Getenv("WAHRWELT_TEST_LOCK_RELEASE")); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for lock release signal")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRuntimeLockForwardsTerminationAndMapsShellExit(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	coordination := t.TempDir()
	ready := filepath.Join(coordination, "ready")
	forwarded := filepath.Join(coordination, "forwarded")
	helper := exec.Command(os.Args[0], "-test.run=^TestRuntimeLockSignalForwarderProcess$")
	helper.Env = append(os.Environ(),
		"WAHRWELT_TEST_SIGNAL_HELPER=1",
		"WAHRWELT_TEST_LOCK_ROOT="+root,
		"WAHRWELT_TEST_SIGNAL_READY="+ready,
		"WAHRWELT_TEST_SIGNAL_FORWARDED="+forwarded,
	)
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = helper.Process.Kill()
		_ = helper.Wait()
	})
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("signal child did not become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := helper.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := helper.Wait(); err != nil {
		t.Fatalf("signal forwarding helper failed: %v", err)
	}
	if _, err := os.Stat(forwarded); err != nil {
		t.Fatalf("SIGTERM was not forwarded to the locked child: %v", err)
	}
}

func TestRuntimeLockSignalForwarderProcess(t *testing.T) {
	if os.Getenv("WAHRWELT_TEST_SIGNAL_HELPER") != "1" {
		return
	}
	command := []string{os.Args[0], "-test.run=^TestRuntimeLockSignalChildProcess$"}
	err := runWithRuntimeLock(
		os.Getenv("WAHRWELT_TEST_LOCK_ROOT"),
		"wahrwelt-shell-v2.lock",
		0,
		command,
	)
	var exitError commandExitError
	if !errors.As(err, &exitError) || exitError.code != 42 {
		t.Fatalf("forwarded child status = %v, want 42", err)
	}
}

func TestRuntimeLockSignalChildProcess(t *testing.T) {
	if os.Getenv("WAHRWELT_TEST_SIGNAL_HELPER") != "1" {
		return
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	if err := os.WriteFile(os.Getenv("WAHRWELT_TEST_SIGNAL_READY"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-signals:
		if err := os.WriteFile(os.Getenv("WAHRWELT_TEST_SIGNAL_FORWARDED"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		os.Exit(42)
	case <-time.After(5 * time.Second):
		t.Fatal("locked child did not receive SIGTERM")
	}
}

func TestRuntimeLockEscalatesForSignalIgnoringProcessGroup(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	coordination := t.TempDir()
	ready := filepath.Join(coordination, "ready")
	childPIDPath := filepath.Join(coordination, "child-pid")
	grandchildPIDPath := filepath.Join(coordination, "grandchild-pid")
	helper := exec.Command(os.Args[0], "-test.run=^TestRuntimeLockIgnoringSignalHolderProcess$")
	helper.Env = append(os.Environ(),
		"WAHRWELT_TEST_IGNORE_SIGNAL_HELPER=1",
		"WAHRWELT_TEST_LOCK_ROOT="+root,
		"WAHRWELT_TEST_IGNORE_SIGNAL_READY="+ready,
		"WAHRWELT_TEST_IGNORE_SIGNAL_CHILD_PID="+childPIDPath,
		"WAHRWELT_TEST_IGNORE_SIGNAL_GRANDCHILD_PID="+grandchildPIDPath,
	)
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = helper.Process.Kill()
		_ = helper.Wait()
	})
	waitForTestFile(t, ready)
	childPID := readTestPID(t, childPIDPath)
	grandchildPID := readTestPID(t, grandchildPIDPath)
	t.Cleanup(func() {
		_ = unix.Kill(childPID, unix.SIGKILL)
		_ = unix.Kill(grandchildPID, unix.SIGKILL)
	})
	started := time.Now()
	if err := helper.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := helper.Wait(); err != nil {
		t.Fatalf("signal escalation helper failed: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 4*time.Second {
		t.Fatalf("signal escalation took %s, want a bounded shutdown", elapsed)
	}
	waitForProcessGone(t, childPID)
	waitForProcessGone(t, grandchildPID)
	if err := runWithRuntimeLock(root, "wahrwelt-shell-v2.lock", 0, []string{"true"}); err != nil {
		t.Fatalf("lock stayed held after signal escalation: %v", err)
	}
}

func TestRuntimeLockIgnoringSignalHolderProcess(t *testing.T) {
	if os.Getenv("WAHRWELT_TEST_IGNORE_SIGNAL_HELPER") != "1" {
		return
	}
	err := runWithRuntimeLock(
		os.Getenv("WAHRWELT_TEST_LOCK_ROOT"),
		"wahrwelt-shell-v2.lock",
		0,
		[]string{os.Args[0], "-test.run=^TestRuntimeLockIgnoringSignalChildProcess$"},
	)
	var exitError commandExitError
	if !errors.As(err, &exitError) || exitError.code != 137 {
		t.Fatalf("escalated child status = %v, want 137", err)
	}
}

func TestRuntimeLockIgnoringSignalChildProcess(t *testing.T) {
	if os.Getenv("WAHRWELT_TEST_IGNORE_SIGNAL_HELPER") != "1" {
		return
	}
	signal.Ignore(syscall.SIGTERM)
	grandchild := exec.Command(os.Args[0], "-test.run=^TestRuntimeLockIgnoringSignalGrandchildProcess$")
	grandchild.Env = os.Environ()
	if err := grandchild.Start(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		os.Getenv("WAHRWELT_TEST_IGNORE_SIGNAL_CHILD_PID"),
		[]byte(strconv.Itoa(os.Getpid())),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	waitForTestFile(t, os.Getenv("WAHRWELT_TEST_IGNORE_SIGNAL_READY"))
	for {
		time.Sleep(time.Hour)
	}
}

func TestRuntimeLockIgnoringSignalGrandchildProcess(t *testing.T) {
	if os.Getenv("WAHRWELT_TEST_IGNORE_SIGNAL_HELPER") != "1" {
		return
	}
	signal.Ignore(syscall.SIGTERM)
	if err := os.WriteFile(
		os.Getenv("WAHRWELT_TEST_IGNORE_SIGNAL_GRANDCHILD_PID"),
		[]byte(strconv.Itoa(os.Getpid())),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("WAHRWELT_TEST_IGNORE_SIGNAL_READY"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for {
		time.Sleep(time.Hour)
	}
}

func TestRuntimeLockHelperSIGKILLKillsTransitionProcessGroup(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	coordination := t.TempDir()
	ready := filepath.Join(coordination, "ready")
	childPIDPath := filepath.Join(coordination, "child-pid")
	grandchildPIDPath := filepath.Join(coordination, "grandchild-pid")
	helper := exec.Command(os.Args[0], "-test.run=^TestRuntimeLockKillHolderProcess$")
	helper.Env = append(os.Environ(),
		"WAHRWELT_TEST_KILL_HELPER=1",
		"WAHRWELT_TEST_LOCK_ROOT="+root,
		"WAHRWELT_TEST_KILL_READY="+ready,
		"WAHRWELT_TEST_KILL_CHILD_PID="+childPIDPath,
		"WAHRWELT_TEST_KILL_GRANDCHILD_PID="+grandchildPIDPath,
	)
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			_ = helper.Process.Kill()
			_ = helper.Wait()
			t.Fatal("kill test transition child did not become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	childPID := readTestPID(t, childPIDPath)
	grandchildPID := readTestPID(t, grandchildPIDPath)
	t.Cleanup(func() {
		_ = unix.Kill(childPID, unix.SIGKILL)
		_ = unix.Kill(grandchildPID, unix.SIGKILL)
	})
	if err := helper.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := helper.Wait(); err == nil {
		t.Fatal("SIGKILLed helper exited successfully")
	}
	waitForProcessGone(t, childPID)
	waitForProcessGone(t, grandchildPID)
	if err := runWithRuntimeLock(root, "wahrwelt-shell-v2.lock", 2*time.Second, []string{"true"}); err != nil {
		t.Fatalf("kernel lock was not released after helper death: %v", err)
	}
}

func TestRuntimeLockKillHolderProcess(t *testing.T) {
	if os.Getenv("WAHRWELT_TEST_KILL_HELPER") != "1" {
		return
	}
	command := []string{os.Args[0], "-test.run=^TestRuntimeLockKillChildProcess$"}
	if err := runWithRuntimeLock(
		os.Getenv("WAHRWELT_TEST_LOCK_ROOT"),
		"wahrwelt-shell-v2.lock",
		0,
		command,
	); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeLockKillChildProcess(t *testing.T) {
	if os.Getenv("WAHRWELT_TEST_KILL_HELPER") != "1" {
		return
	}
	grandchild := exec.Command("sleep", "60")
	if err := grandchild.Start(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("WAHRWELT_TEST_KILL_CHILD_PID"), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("WAHRWELT_TEST_KILL_GRANDCHILD_PID"), []byte(strconv.Itoa(grandchild.Process.Pid)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("WAHRWELT_TEST_KILL_READY"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for {
		time.Sleep(time.Hour)
	}
}

func readTestPID(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil || pid <= 1 {
		t.Fatalf("invalid test pid %q: %v", data, err)
	}
	return pid
}

func waitForTestFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForProcessGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := unix.Kill(pid, 0); errors.Is(err, unix.ESRCH) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("process %d survived helper death", pid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRemoveMigrationTemporaryUsesExactKindAndIdentity(t *testing.T) {
	parent := t.TempDir()
	stage := "staging.0123456789abcdef"
	stagePath := filepath.Join(parent, stage)
	if err := os.Mkdir(stagePath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagePath, "payload"), []byte("owned\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	directory, err := fsowner.OpenDirectory(parent)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := directory.Inspect(stage)
	_ = directory.Close()
	if err != nil {
		t.Fatal(err)
	}
	token := migrationIdentity{Device: entry.Identity.Device, Inode: entry.Identity.Inode}
	if err := removeMigrationTemporary(parent, "staging", stage, token, uint32(os.Geteuid())); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(stagePath); !os.IsNotExist(err) {
		t.Fatalf("stage retained: %v", err)
	}
}

func TestRemoveMigrationTemporaryPreservesReplacement(t *testing.T) {
	parent := t.TempDir()
	name := "namespace.0123456789abcdef"
	path := filepath.Join(parent, name)
	if err := os.WriteFile(path, []byte("snapshot\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	directory, err := fsowner.OpenDirectory(parent)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := directory.Inspect(name)
	_ = directory.Close()
	if err != nil {
		t.Fatal(err)
	}
	token := migrationIdentity{Device: entry.Identity.Device, Inode: entry.Identity.Inode}
	replacement := path + ".replacement"
	if err := os.WriteFile(replacement, []byte("foreign\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	err = removeMigrationTemporary(parent, "namespace", name, token, uint32(os.Geteuid()))
	if err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatalf("replacement error=%v", err)
	}
	if data, readErr := os.ReadFile(path); readErr != nil || string(data) != "foreign\n" {
		t.Fatalf("replacement changed: data=%q err=%v", data, readErr)
	}
}

func TestPruneBackupsKeepsNewestThreeExactOwnedDirectories(t *testing.T) {
	parent := t.TempDir()
	for timestamp := 1; timestamp <= 5; timestamp++ {
		path := filepath.Join(parent, backupName(timestamp))
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "configuration.nix"), []byte("owned\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := markBackup(path, uint32(os.Geteuid()), uint32(os.Geteuid())); err != nil {
			t.Fatal(err)
		}
	}

	if err := pruneBackups(parent, 3, uint32(os.Geteuid())); err != nil {
		t.Fatal(err)
	}
	for timestamp := 1; timestamp <= 5; timestamp++ {
		_, err := os.Lstat(filepath.Join(parent, backupName(timestamp)))
		if timestamp <= 2 && !os.IsNotExist(err) {
			t.Fatalf("old backup %d retained: %v", timestamp, err)
		}
		if timestamp > 2 && err != nil {
			t.Fatalf("new backup %d removed: %v", timestamp, err)
		}
	}
}

func TestPruneBackupsRemovesMetadataPreservingMixedOwnerTree(t *testing.T) {
	const childEnvironment = "WAHRWELT_TEST_MIXED_OWNER_BACKUP"
	if os.Getenv(childEnvironment) != "1" {
		if _, err := exec.LookPath("unshare"); err != nil {
			t.Skip("unshare is unavailable")
		}
		cmd := exec.Command(
			"unshare", "--map-auto", "--map-root-user", "--",
			os.Args[0], "-test.run=^TestPruneBackupsRemovesMetadataPreservingMixedOwnerTree$",
		)
		cmd.Env = append(os.Environ(), childEnvironment+"=1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			if strings.Contains(string(output), "no line matching user") ||
				strings.Contains(string(output), "Operation not permitted") {
				t.Skipf("multi-uid user namespace is unavailable: %s", strings.TrimSpace(string(output)))
			}
			t.Fatalf("mixed-owner namespace test failed: %v\n%s", err, output)
		}
		return
	}

	if os.Geteuid() != 0 {
		t.Fatalf("mixed-owner child must run as namespace root, euid=%d", os.Geteuid())
	}
	parent := t.TempDir()
	for timestamp := 1; timestamp <= 4; timestamp++ {
		path := filepath.Join(parent, backupName(timestamp))
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		userTree := filepath.Join(path, "user")
		if err := os.Mkdir(userTree, 0o755); err != nil {
			t.Fatal(err)
		}
		payload := filepath.Join(userTree, "default.nix")
		if err := os.WriteFile(payload, []byte("{ }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, ownedPath := range []string{payload, userTree} {
			if err := os.Chown(ownedPath, 1, 1); err != nil {
				t.Fatal(err)
			}
		}
		if err := markBackup(path, 0, 0); err != nil {
			t.Fatal(err)
		}
	}

	if err := pruneBackups(parent, 3, 0); err != nil {
		t.Fatalf("metadata-preserving mixed-owner backup was not pruned: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(parent, backupName(1))); !os.IsNotExist(err) {
		t.Fatalf("oldest mixed-owner backup retained: %v", err)
	}
}

func TestPruneBackupsPreservesMatchingRootOwnedDirectoryWithoutOwnershipMarker(t *testing.T) {
	parent := t.TempDir()
	unknown := filepath.Join(parent, backupName(1))
	if err := os.Mkdir(unknown, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unknown, "foreign"), []byte("unknown payload\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for timestamp := 2; timestamp <= 5; timestamp++ {
		path := filepath.Join(parent, backupName(timestamp))
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := markBackup(path, uint32(os.Geteuid()), uint32(os.Geteuid())); err != nil {
			t.Fatal(err)
		}
	}

	if err := pruneBackups(parent, 3, uint32(os.Geteuid())); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(unknown, "foreign")); err != nil || string(got) != "unknown payload\n" {
		t.Fatalf("unknown matching backup was changed: data=%q err=%v", got, err)
	}
	if _, err := os.Lstat(filepath.Join(parent, backupName(2))); !os.IsNotExist(err) {
		t.Fatalf("oldest marked backup retained: %v", err)
	}
}

func TestPruneBackupsPreservesUnmarkedDirectoryOwnedBySomeoneOtherThanMarker(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires an unprivileged test uid")
	}
	parent := t.TempDir()
	unknown := filepath.Join(parent, backupName(1))
	if err := os.Mkdir(unknown, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(unknown, "foreign")
	if err := os.WriteFile(sentinel, []byte("unknown payload\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := pruneBackups(parent, 3, 0); err != nil {
		t.Fatalf("unmarked backup must be preserved regardless of owner: %v", err)
	}
	if got, err := os.ReadFile(sentinel); err != nil || string(got) != "unknown payload\n" {
		t.Fatalf("unknown matching backup was changed: data=%q err=%v", got, err)
	}
}

func TestBackupRemovalOptionsSeparateMarkerAndDirectoryOwners(t *testing.T) {
	path := t.TempDir()
	directory, err := fsowner.OpenDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	expected := directory.Identity()
	if err := directory.Close(); err != nil {
		t.Fatal(err)
	}
	options, err := backupRemovalOptions(path, expected, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !options.AllowsUID(0) {
		t.Fatal("root marker owner was rejected")
	}
	if !options.AllowsUID(expected.UID) {
		t.Fatal("metadata-preserved directory owner was rejected")
	}
	if options.AllowsUID(expected.UID + 1) {
		t.Fatal("unrelated owner was accepted")
	}
}

func TestPruneBackupsPreflightsMatchingSymlinkBeforeRemovingAnything(t *testing.T) {
	parent := t.TempDir()
	for timestamp := 1; timestamp <= 4; timestamp++ {
		path := filepath.Join(parent, backupName(timestamp))
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := markBackup(path, uint32(os.Geteuid()), uint32(os.Geteuid())); err != nil {
			t.Fatal(err)
		}
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(parent, "nixos.bak.0.7.0")); err != nil {
		t.Fatal(err)
	}

	err := pruneBackups(parent, 3, uint32(os.Geteuid()))
	if err == nil || !strings.Contains(err.Error(), "ownership collision") {
		t.Fatalf("expected matching symlink collision, got %v", err)
	}
	for timestamp := 1; timestamp <= 4; timestamp++ {
		if _, err := os.Stat(filepath.Join(parent, backupName(timestamp))); err != nil {
			t.Fatalf("preflight failure removed backup %d: %v", timestamp, err)
		}
	}
	if _, err := os.Stat(filepath.Join(outside, "sentinel")); !os.IsNotExist(err) {
		t.Fatalf("matching symlink target was mutated: %v", err)
	}
}

func TestMarkBackupRefusesExistingMarker(t *testing.T) {
	backup := filepath.Join(t.TempDir(), backupName(1))
	if err := os.Mkdir(backup, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(backup, backupMarkerName)
	if err := os.WriteFile(marker, []byte("foreign\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := markBackup(backup, uint32(os.Geteuid()), uint32(os.Geteuid()))
	if err == nil {
		t.Fatal("markBackup() accepted an existing unknown marker")
	}
	data, readErr := os.ReadFile(marker)
	if readErr != nil || string(data) != "foreign\n" {
		t.Fatalf("existing marker changed: data=%q err=%v", data, readErr)
	}
}

func TestRemoveManagedBackupMarkerRejectsFIFONonBlocking(t *testing.T) {
	backup := filepath.Join(t.TempDir(), backupName(1))
	if err := os.Mkdir(backup, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(backup, backupMarkerName)
	if err := unix.Mkfifo(marker, 0o600); err != nil {
		t.Fatal(err)
	}
	directory, err := fsowner.OpenDirectory(backup)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = directory.Close() })
	done := make(chan error, 1)
	go func() {
		done <- removeManagedBackupMarker(directory, backup, uint32(os.Geteuid()))
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO marker was accepted")
		}
	case <-time.After(250 * time.Millisecond):
		writer, openErr := unix.Open(marker, unix.O_WRONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if openErr == nil {
			_ = unix.Close(writer)
		}
		<-done
		t.Fatal("FIFO marker blocked the ownership check")
	}
}

func TestMarkBackupRejectsDirectoryOwnedDifferentlyFromSource(t *testing.T) {
	backup := filepath.Join(t.TempDir(), backupName(1))
	if err := os.Mkdir(backup, 0o700); err != nil {
		t.Fatal(err)
	}
	actualUID := uint32(os.Geteuid())

	err := markBackup(backup, actualUID+1, actualUID)
	if err == nil {
		t.Fatal("backup with a source-owner mismatch was accepted")
	}
	if _, statErr := os.Lstat(filepath.Join(backup, backupMarkerName)); !os.IsNotExist(statErr) {
		t.Fatalf("owner-mismatched backup was mutated: %v", statErr)
	}
}

func TestMarkBackupFromSourceAcceptsMetadataPreservingDirectoryOwnership(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "nixos")
	backup := filepath.Join(parent, backupName(1))
	if err := os.Mkdir(source, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(backup, 0o750); err != nil {
		t.Fatal(err)
	}

	if err := markBackupFromSource(source, backup, uint32(os.Geteuid())); err != nil {
		t.Fatalf("metadata-preserving backup was rejected: %v", err)
	}
	if _, err := os.Stat(filepath.Join(backup, backupMarkerName)); err != nil {
		t.Fatalf("ownership marker missing: %v", err)
	}
}

func TestMarkBackupFromSourceDoesNotConflateDirectoryAndMarkerOwners(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "nixos")
	backup := filepath.Join(parent, backupName(1))
	for _, path := range []string{source, backup} {
		if err := os.Mkdir(path, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	markerUID := uint32(os.Geteuid()) + 1

	err := markBackupFromSource(source, backup, markerUID)
	if err == nil {
		t.Fatal("marker with the wrong actual owner was accepted")
	}
	if _, statErr := os.Lstat(filepath.Join(backup, backupMarkerName)); statErr != nil {
		t.Fatalf("backup was rejected before marker ownership was checked: %v", statErr)
	}
}

func TestMarkBackupFromSourceReplacesCopiedManagedMarker(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "nixos")
	backup := filepath.Join(parent, backupName(1))
	for _, path := range []string{source, backup} {
		if err := os.Mkdir(path, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	sourceDirectory, err := fsowner.OpenDirectory(source)
	if err != nil {
		t.Fatal(err)
	}
	stalePayload := []byte(backupMarkerVersion + "\n" + sourceDirectory.Identity().String() + "\n")
	if err := sourceDirectory.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{source, backup} {
		if err := os.WriteFile(filepath.Join(path, backupMarkerName), stalePayload, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	markerUID := uint32(os.Geteuid())
	if err := markBackupFromSource(source, backup, markerUID); err != nil {
		t.Fatalf("copied managed marker was not replaced: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(source, backupMarkerName)); !os.IsNotExist(err) {
		t.Fatalf("stale marker remained in live source: %v", err)
	}
	backupDirectory, err := fsowner.OpenDirectory(backup)
	if err != nil {
		t.Fatal(err)
	}
	backupIdentity := backupDirectory.Identity()
	if err := backupDirectory.Close(); err != nil {
		t.Fatal(err)
	}
	owned, err := hasOwnedBackupMarker(backup, backupIdentity, markerUID)
	if err != nil || !owned {
		t.Fatalf("replacement marker is not bound to backup: owned=%v err=%v", owned, err)
	}
}

func TestMigrateGeneratedPasswordModulesPublishesHashAndWritesFunctionalStubs(t *testing.T) {
	root := t.TempDir()
	sources := []string{
		filepath.Join(root, "nixos", "hashed-password.nix"),
		filepath.Join(root, "nixos", "hosts", "NixOS", "hashed-password.nix"),
	}
	destination := filepath.Join(root, "wahrwelt", "hashed-password")
	journal := filepath.Join(filepath.Dir(destination), passwordMigrationRecord)
	for _, parent := range []string{filepath.Dir(sources[0]), filepath.Dir(sources[1]), filepath.Dir(destination)} {
		if err := os.MkdirAll(parent, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	hash := "$6$rounds=5000$testsalt$" + strings.Repeat("A", 86)
	for index, source := range sources {
		namespace := "wahrwelt"
		if index == 1 {
			namespace = "mysetup"
		}
		if err := os.WriteFile(source, []byte(generatedPasswordModuleForNamespace(namespace, hash)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	status, err := migrateGeneratedPasswordModules(sources, destination, journal, uint32(os.Geteuid()), passwordMigrationHooks{})
	if err != nil {
		t.Fatal(err)
	}
	if status != "migrated" {
		t.Fatalf("migration status = %q, want migrated", status)
	}
	if data, err := os.ReadFile(destination); err != nil || string(data) != hash+"\n" {
		t.Fatalf("external password hash = %q, err=%v", data, err)
	}
	for index, source := range sources {
		namespace := "wahrwelt"
		if index == 1 {
			namespace = "mysetup"
		}
		data, readErr := os.ReadFile(source)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if got, want := string(data), functionalPasswordModule(namespace); got != want || strings.Contains(got, hash) {
			t.Fatalf("functional %s stub = %q, want %q without hash", namespace, got, want)
		}
		info, statErr := os.Stat(source)
		if statErr != nil || info.Mode().Perm() != 0o644 {
			t.Fatalf("functional stub metadata = %v, err=%v", info, statErr)
		}
	}
	if _, err := os.Lstat(journal); !os.IsNotExist(err) {
		t.Fatalf("successful migration retained journal: %v", err)
	}
}

func TestMigrateGeneratedPasswordModulesRecoversAfterRollbackRestoresGeneratedModuleAtNewInode(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "nixos", "hashed-password.nix")
	destination := filepath.Join(root, "wahrwelt", "hashed-password")
	journal := filepath.Join(filepath.Dir(destination), passwordMigrationRecord)
	for _, parent := range []string{filepath.Dir(source), filepath.Dir(destination)} {
		if err := os.MkdirAll(parent, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	hash := "$6$rounds=5000$testsalt$" + strings.Repeat("A", 86)
	generated := []byte(generatedPasswordModuleForNamespace("wahrwelt", hash))
	if err := os.WriteFile(source, generated, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(source)
	if err != nil {
		t.Fatal(err)
	}
	injected := errors.New("injected crash before sanitize")
	_, err = migrateGeneratedPasswordModules(
		[]string{source}, destination, journal, uint32(os.Geteuid()),
		passwordMigrationHooks{BeforeSanitize: func() error { return injected }},
	)
	if !errors.Is(err, injected) {
		t.Fatalf("pre-sanitize crash error = %v", err)
	}
	if _, err := os.Stat(journal); err != nil {
		t.Fatalf("crash recovery journal missing: %v", err)
	}

	retained := source + ".pre-rollback"
	if err := os.Rename(source, retained); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, generated, 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := os.Lstat(source)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Fatal("rollback fixture did not replace the generated module inode")
	}

	status, err := migrateGeneratedPasswordModules(
		[]string{source}, destination, journal, uint32(os.Geteuid()), passwordMigrationHooks{},
	)
	if err != nil || status != "migrated" {
		t.Fatalf("rollback recovery status = %q, err=%v", status, err)
	}
	if got := string(mustReadFile(t, source)); got != functionalPasswordModule("wahrwelt") {
		t.Fatalf("recovered functional stub = %q", got)
	}
	if data, err := os.ReadFile(destination); err != nil || string(data) != hash+"\n" {
		t.Fatalf("external password hash changed: bytes=%d err=%v", len(data), err)
	}
	if _, err := os.Lstat(journal); !os.IsNotExist(err) {
		t.Fatalf("successful migration retained journal: %v", err)
	}
}

func TestMigrateGeneratedPasswordModulesRejectsScrubbedModuleRestoredAtNewInode(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "nixos", "hashed-password.nix")
	destination := filepath.Join(root, "wahrwelt", "hashed-password")
	journal := filepath.Join(filepath.Dir(destination), passwordMigrationRecord)
	for _, parent := range []string{filepath.Dir(source), filepath.Dir(destination)} {
		if err := os.MkdirAll(parent, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	hash := "$6$rounds=5000$testsalt$" + strings.Repeat("A", 86)
	if err := os.WriteFile(source, []byte(generatedPasswordModuleForNamespace("wahrwelt", hash)), 0o600); err != nil {
		t.Fatal(err)
	}
	injected := errors.New("injected crash after scrub")
	_, err := migrateGeneratedPasswordModules(
		[]string{source}, destination, journal, uint32(os.Geteuid()),
		passwordMigrationHooks{AfterScrub: func(string) error { return injected }},
	)
	if !errors.Is(err, injected) {
		t.Fatalf("scrub crash error = %v", err)
	}
	scrubbed := mustReadFile(t, source)
	journalBefore, err := os.Lstat(journal)
	if err != nil {
		t.Fatal(err)
	}
	journalPayload := mustReadFile(t, journal)
	retained := source + ".pre-rollback"
	if err := os.Rename(source, retained); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, scrubbed, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = migrateGeneratedPasswordModules(
		[]string{source}, destination, journal, uint32(os.Geteuid()), passwordMigrationHooks{},
	)
	if err == nil || !strings.Contains(err.Error(), "ownership collision") {
		t.Fatalf("replacement scrubbed module was accepted: %v", err)
	}
	if got := mustReadFile(t, source); !bytes.Equal(got, scrubbed) {
		t.Fatal("replacement scrubbed module changed after rejection")
	}
	journalAfter, statErr := os.Lstat(journal)
	if statErr != nil || !os.SameFile(journalBefore, journalAfter) || !bytes.Equal(journalPayload, mustReadFile(t, journal)) {
		t.Fatalf("scrub recovery journal changed after rejection: %v", statErr)
	}
}

func TestMigrateGeneratedPasswordModulesPreservesJournalWhenRecordedSourceIsAbsent(t *testing.T) {
	root := t.TempDir()
	sources := []string{
		filepath.Join(root, "nixos", "hashed-password.nix"),
		filepath.Join(root, "nixos", "hosts", "NixOS", "hashed-password.nix"),
	}
	destination := filepath.Join(root, "wahrwelt", "hashed-password")
	journalPath := filepath.Join(filepath.Dir(destination), passwordMigrationRecord)
	for _, parent := range []string{filepath.Dir(sources[0]), filepath.Dir(sources[1]), filepath.Dir(destination)} {
		if err := os.MkdirAll(parent, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	hash := "$6$rounds=5000$testsalt$" + strings.Repeat("A", 86)
	generated := []byte(generatedPasswordModuleForNamespace("wahrwelt", hash))
	for _, source := range sources {
		if err := os.WriteFile(source, generated, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	injected := errors.New("injected crash before sanitize")
	_, err := migrateGeneratedPasswordModules(
		sources, destination, journalPath, uint32(os.Geteuid()),
		passwordMigrationHooks{BeforeSanitize: func() error { return injected }},
	)
	if !errors.Is(err, injected) {
		t.Fatalf("pre-sanitize crash error = %v", err)
	}
	journalBefore, err := os.Lstat(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	payloadBefore := mustReadFile(t, journalPath)

	if err := os.Rename(sources[0], sources[0]+".pre-rollback"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sources[0], generated, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(sources[1], sources[1]+".temporarily-absent"); err != nil {
		t.Fatal(err)
	}

	_, err = migrateGeneratedPasswordModules(
		sources, destination, journalPath, uint32(os.Geteuid()), passwordMigrationHooks{},
	)
	if err == nil || !strings.Contains(err.Error(), "ownership collision") {
		t.Fatalf("partial stale journal was accepted: %v", err)
	}
	journalAfter, statErr := os.Lstat(journalPath)
	if statErr != nil {
		t.Fatalf("protected journal missing: %v", statErr)
	}
	if !os.SameFile(journalBefore, journalAfter) || !bytes.Equal(payloadBefore, mustReadFile(t, journalPath)) {
		t.Fatal("protected journal changed after partial recovery rejection")
	}
}

func TestMigrateGeneratedPasswordModulesPreservesJournalWhenAllRecordedSourcesAreAbsent(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "nixos", "hashed-password.nix")
	destination := filepath.Join(root, "wahrwelt", "hashed-password")
	journalPath := filepath.Join(filepath.Dir(destination), passwordMigrationRecord)
	for _, parent := range []string{filepath.Dir(source), filepath.Dir(destination)} {
		if err := os.MkdirAll(parent, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	hash := "$6$rounds=5000$testsalt$" + strings.Repeat("A", 86)
	if err := os.WriteFile(source, []byte(generatedPasswordModuleForNamespace("wahrwelt", hash)), 0o600); err != nil {
		t.Fatal(err)
	}
	injected := errors.New("injected crash before sanitize")
	_, err := migrateGeneratedPasswordModules(
		[]string{source}, destination, journalPath, uint32(os.Geteuid()),
		passwordMigrationHooks{BeforeSanitize: func() error { return injected }},
	)
	if !errors.Is(err, injected) {
		t.Fatalf("pre-sanitize crash error = %v", err)
	}
	journalBefore, err := os.Lstat(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	payloadBefore := mustReadFile(t, journalPath)
	if err := os.Rename(source, source+".temporarily-absent"); err != nil {
		t.Fatal(err)
	}

	_, err = migrateGeneratedPasswordModules(
		[]string{source}, destination, journalPath, uint32(os.Geteuid()), passwordMigrationHooks{},
	)
	if err == nil || !strings.Contains(err.Error(), "ownership collision") {
		t.Fatalf("missing journal sources were accepted: %v", err)
	}
	journalAfter, statErr := os.Lstat(journalPath)
	if statErr != nil || !os.SameFile(journalBefore, journalAfter) || !bytes.Equal(payloadBefore, mustReadFile(t, journalPath)) {
		t.Fatalf("journal changed after missing-source rejection: %v", statErr)
	}
}

func TestMigrateGeneratedPasswordModulesRejectsNoncanonicalStaleJournal(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "nixos", "hashed-password.nix")
	destination := filepath.Join(root, "wahrwelt", "hashed-password")
	journalPath := filepath.Join(filepath.Dir(destination), passwordMigrationRecord)
	for _, parent := range []string{filepath.Dir(source), filepath.Dir(destination)} {
		if err := os.MkdirAll(parent, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	hash := "$6$rounds=5000$testsalt$" + strings.Repeat("A", 86)
	generated := []byte(generatedPasswordModuleForNamespace("wahrwelt", hash))
	if err := os.WriteFile(source, generated, 0o600); err != nil {
		t.Fatal(err)
	}
	injected := errors.New("injected crash before sanitize")
	_, err := migrateGeneratedPasswordModules(
		[]string{source}, destination, journalPath, uint32(os.Geteuid()),
		passwordMigrationHooks{BeforeSanitize: func() error { return injected }},
	)
	if !errors.Is(err, injected) {
		t.Fatalf("pre-sanitize crash error = %v", err)
	}
	canonical := mustReadFile(t, journalPath)
	foreign := append([]byte(`{"owner":"foreign",`), canonical[1:]...)
	if err := os.WriteFile(journalPath, foreign, 0o600); err != nil {
		t.Fatal(err)
	}
	journalBefore, err := os.Lstat(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(source, source+".pre-rollback"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, generated, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = migrateGeneratedPasswordModules(
		[]string{source}, destination, journalPath, uint32(os.Geteuid()), passwordMigrationHooks{},
	)
	if err == nil || !strings.Contains(err.Error(), "ownership collision") {
		t.Fatalf("noncanonical stale journal was accepted: %v", err)
	}
	journalAfter, statErr := os.Lstat(journalPath)
	if statErr != nil {
		t.Fatalf("foreign journal missing: %v", statErr)
	}
	if !os.SameFile(journalBefore, journalAfter) || !bytes.Equal(foreign, mustReadFile(t, journalPath)) {
		t.Fatal("foreign journal changed after rejection")
	}
}

func TestRemovePasswordMigrationJournalPreservesChangedOrReplacedFile(t *testing.T) {
	root := t.TempDir()
	journalPath := filepath.Join(root, passwordMigrationRecord)
	requiredUID := uint32(os.Geteuid())
	journal := passwordMigrationJournal{
		Version: passwordMigrationV1,
		Entries: []passwordMigrationEntry{{
			Path:      "/etc/nixos/hashed-password.nix",
			Device:    1,
			Inode:     1,
			UID:       requiredUID,
			Namespace: "wahrwelt",
		}},
	}
	payload, err := passwordMigrationJournalPayload(journal)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name   string
		change func(t *testing.T)
	}{
		{
			name: "content",
			change: func(t *testing.T) {
				t.Helper()
				if err := os.WriteFile(journalPath, []byte("{}\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "inode",
			change: func(t *testing.T) {
				t.Helper()
				if err := os.Rename(journalPath, journalPath+".retained"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(journalPath, payload, 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(journalPath, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			loaded, err := loadPasswordMigrationJournal(journalPath, requiredUID)
			if err != nil {
				t.Fatal(err)
			}
			test.change(t)
			if err := removePasswordMigrationJournal(journalPath, loaded, requiredUID); err == nil ||
				!strings.Contains(err.Error(), "ownership collision") {
				t.Fatalf("changed journal removal error = %v", err)
			}
			if _, err := os.Lstat(journalPath); err != nil {
				t.Fatalf("changed journal was removed: %v", err)
			}
		})
	}
}

func TestMigrateGeneratedPasswordModulesSanitizesPinnedOriginalAndPreservesReplacement(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "nixos", "hashed-password.nix")
	destination := filepath.Join(root, "wahrwelt", "hashed-password")
	journal := filepath.Join(filepath.Dir(destination), passwordMigrationRecord)
	for _, parent := range []string{filepath.Dir(source), filepath.Dir(destination)} {
		if err := os.MkdirAll(parent, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	hash := "$6$rounds=5000$testsalt$" + strings.Repeat("A", 86)
	if err := os.WriteFile(source, []byte(generatedPasswordModuleForNamespace("wahrwelt", hash)), 0o600); err != nil {
		t.Fatal(err)
	}
	retained := source + ".retained"
	_, err := migrateGeneratedPasswordModules(
		[]string{source}, destination, journal, uint32(os.Geteuid()),
		passwordMigrationHooks{BeforeSanitize: func() error {
			if err := os.Rename(source, retained); err != nil {
				return err
			}
			return os.WriteFile(source, []byte("foreign replacement\n"), 0o600)
		}},
	)
	if err == nil {
		t.Fatal("expected replacement collision")
	}
	data, readErr := os.ReadFile(source)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "foreign replacement\n" {
		t.Fatalf("replacement changed: %q", data)
	}
	retainedData, readErr := os.ReadFile(retained)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got, want := string(retainedData), functionalPasswordModule("wahrwelt"); got != want || strings.Contains(got, hash) {
		t.Fatalf("pinned original was not sanitized: %q", got)
	}
}

func TestMigrateGeneratedPasswordModulesRetriesAfterScrubbedHashCrash(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "nixos", "hashed-password.nix")
	destination := filepath.Join(root, "wahrwelt", "hashed-password")
	journal := filepath.Join(filepath.Dir(destination), passwordMigrationRecord)
	for _, parent := range []string{filepath.Dir(source), filepath.Dir(destination)} {
		if err := os.MkdirAll(parent, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	hash := "$6$rounds=5000$testsalt$" + strings.Repeat("A", 86)
	if err := os.WriteFile(source, []byte(generatedPasswordModuleForNamespace("mysetup", hash)), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := migrateGeneratedPasswordModules(
		[]string{source}, destination, journal, uint32(os.Geteuid()),
		passwordMigrationHooks{AfterScrub: func(string) error { return errors.New("injected crash after scrub") }},
	)
	if err == nil || !strings.Contains(err.Error(), "injected crash") {
		t.Fatalf("scrub crash error = %v", err)
	}
	interrupted, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(interrupted, []byte(hash)) || !bytes.Contains(interrupted, bytes.Repeat([]byte{'x'}, len(hash))) {
		t.Fatalf("interrupted scrubbed module still contains hash or lacks exact scrub span")
	}
	status, err := migrateGeneratedPasswordModules(
		[]string{source}, destination, journal, uint32(os.Geteuid()), passwordMigrationHooks{},
	)
	if err != nil || status != "migrated" {
		t.Fatalf("retry status = %q, err=%v", status, err)
	}
	if got := string(mustReadFile(t, source)); got != functionalPasswordModule("mysetup") {
		t.Fatalf("recovered functional stub = %q", got)
	}
	if _, err := os.Lstat(journal); !os.IsNotExist(err) {
		t.Fatalf("successful scrub recovery retained journal: %v", err)
	}
}

func TestMigrateGeneratedPasswordModulesRetriesAfterFunctionalStubCrash(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "nixos", "hashed-password.nix")
	destination := filepath.Join(root, "wahrwelt", "hashed-password")
	journal := filepath.Join(filepath.Dir(destination), passwordMigrationRecord)
	for _, parent := range []string{filepath.Dir(source), filepath.Dir(destination)} {
		if err := os.MkdirAll(parent, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	hash := "$6$rounds=5000$testsalt$" + strings.Repeat("A", 86)
	if err := os.WriteFile(source, []byte(generatedPasswordModuleForNamespace("wahrwelt", hash)), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := migrateGeneratedPasswordModules(
		[]string{source}, destination, journal, uint32(os.Geteuid()),
		passwordMigrationHooks{AfterStub: func(string) error { return errors.New("injected crash after stub") }},
	)
	if err == nil || !strings.Contains(err.Error(), "injected crash") {
		t.Fatalf("stub crash error = %v", err)
	}
	if got := string(mustReadFile(t, source)); got != functionalPasswordModule("wahrwelt") {
		t.Fatalf("crash-time functional stub = %q", got)
	}
	status, err := migrateGeneratedPasswordModules(
		[]string{source}, destination, journal, uint32(os.Geteuid()), passwordMigrationHooks{},
	)
	if err != nil || status != "migrated" {
		t.Fatalf("retry status = %q, err=%v", status, err)
	}
	if _, err := os.Lstat(journal); !os.IsNotExist(err) {
		t.Fatalf("successful stub recovery retained journal: %v", err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestValidateExternalPasswordHashRejectsInvalidRootPrivatePayload(t *testing.T) {
	target := filepath.Join(t.TempDir(), "hashed-password")
	if err := os.WriteFile(target, []byte("not-a-password-hash\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateExternalPasswordHash(target, uint32(os.Geteuid())); err == nil {
		t.Fatal("validateExternalPasswordHash() accepted invalid payload")
	}
	hash := "$6$rounds=5000$testsalt$" + strings.Repeat("A", 86)
	if err := os.WriteFile(target, []byte(hash+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateExternalPasswordHash(target, uint32(os.Geteuid())); err != nil {
		t.Fatal(err)
	}
}

func TestPasswordHashPublicationUsesSealedDescriptorAndLeavesNoTransactionResidue(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "wahrwelt", "hashed-password")
	transactionRoot := filepath.Join(filepath.Dir(target), passwordTransactionDir)
	requiredUID := uint32(os.Geteuid())
	firstHash := "$6$rounds=5000$testsalt$" + strings.Repeat("A", 86)
	secondHash := "$6$rounds=5000$testsalt$" + strings.Repeat("B", 86)

	status, err := passwordHashStatus(target, requiredUID)
	if err != nil || status != "absent" {
		t.Fatalf("initial status = %q, err=%v, want absent", status, err)
	}
	first, firstPath := sealedPasswordSource(t, firstHash+"\n")
	defer first.Close()
	if err := publishPasswordHash(target, firstPath, status, requiredUID, transactionRoot); err != nil {
		t.Fatalf("fresh publish: %v", err)
	}
	status, err = passwordHashStatus(target, requiredUID)
	if err != nil || status == "absent" {
		t.Fatalf("published status = %q, err=%v", status, err)
	}
	second, secondPath := sealedPasswordSource(t, secondHash+"\n")
	defer second.Close()
	if err := publishPasswordHash(target, secondPath, status, requiredUID, transactionRoot); err != nil {
		t.Fatalf("replacement publish: %v", err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != secondHash+"\n" {
		t.Fatalf("published hash differs: bytes=%d err=%v", len(data), err)
	}
	if info, err := os.Stat(target); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("published metadata = %v err=%v", info, err)
	}
	entries, err := os.ReadDir(transactionRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("successful password publication retained transactions: %v", entries)
	}
}

func TestPasswordHashPublicationRejectsChangedExpectedTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "wahrwelt", "hashed-password")
	transactionRoot := filepath.Join(filepath.Dir(target), passwordTransactionDir)
	requiredUID := uint32(os.Geteuid())
	firstHash := "$6$rounds=5000$testsalt$" + strings.Repeat("A", 86)
	foreignHash := "$6$rounds=5000$testsalt$" + strings.Repeat("B", 86)
	replacementHash := "$6$rounds=5000$testsalt$" + strings.Repeat("C", 86)
	first, firstPath := sealedPasswordSource(t, firstHash+"\n")
	defer first.Close()
	if err := publishPasswordHash(target, firstPath, "absent", requiredUID, transactionRoot); err != nil {
		t.Fatal(err)
	}
	status, err := passwordHashStatus(target, requiredUID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(foreignHash+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	replacement, replacementPath := sealedPasswordSource(t, replacementHash+"\n")
	defer replacement.Close()
	if err := publishPasswordHash(target, replacementPath, status, requiredUID, transactionRoot); err == nil || !strings.Contains(err.Error(), "changed before publication") {
		t.Fatalf("replacement error = %v, want ownership collision", err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != foreignHash+"\n" {
		t.Fatalf("concurrent winner changed: bytes=%d err=%v", len(data), err)
	}
}

func TestPasswordHashPublicationRejectsUnsealedSource(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "wahrwelt", "hashed-password")
	source := filepath.Join(root, "source")
	hash := "$6$rounds=5000$testsalt$" + strings.Repeat("A", 86)
	if err := os.WriteFile(source, []byte(hash+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	procPath := fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), file.Fd())
	err = publishPasswordHash(
		target,
		procPath,
		"absent",
		uint32(os.Geteuid()),
		filepath.Join(filepath.Dir(target), passwordTransactionDir),
	)
	if err == nil || !strings.Contains(err.Error(), "not sealed") {
		t.Fatalf("unsealed source error = %v", err)
	}
}

func sealedPasswordSource(t *testing.T, payload string) (*os.File, string) {
	t.Helper()
	fd, err := unix.MemfdCreate("password-test", unix.MFD_CLOEXEC|unix.MFD_ALLOW_SEALING)
	if err != nil {
		t.Fatal(err)
	}
	file := os.NewFile(uintptr(fd), "password-test")
	if file == nil {
		_ = unix.Close(fd)
		t.Fatal("create memfd file")
	}
	if _, err := file.WriteString(payload); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	seals := unix.F_SEAL_WRITE | unix.F_SEAL_GROW | unix.F_SEAL_SHRINK | unix.F_SEAL_SEAL
	if _, err := unix.FcntlInt(file.Fd(), unix.F_ADD_SEALS, seals); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	return file, fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), file.Fd())
}

func backupName(timestamp int) string {
	return "nixos.bak." + string(rune('0'+timestamp)) + ".7.0"
}

func generatedPasswordModuleForNamespace(namespace, hash string) string {
	return `{ config, ... }:

{
  users.users.${config.` + namespace + `.user.username}.initialHashedPassword = "` + hash + `";
}
`
}
