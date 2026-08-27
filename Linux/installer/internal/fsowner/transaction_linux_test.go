//go:build linux

package fsowner

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTransactionCommitRemovesPrivateStagesAndJournal(t *testing.T) {
	t.Parallel()

	root := privateTempDir(t)
	target := filepath.Join(root, "runtime", "shell-keybinds.lua")
	mustWritePrivate(t, target, "old\n", 0o644)

	tx, err := Begin(filepath.Join(root, "session"), ".runtime-rollback-", []string{target})
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if err := Write(tx, target, []byte("new\n"), 0o644); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := Commit(tx); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	if got := mustRead(t, target); got != "new\n" {
		t.Fatalf("target = %q, want new payload", got)
	}
	assertNoRuntimeResidue(t, root)
}

func TestBeginExpectedRejectsReplacementBeforeJournal(t *testing.T) {
	t.Parallel()

	root := privateTempDir(t)
	target := filepath.Join(root, "runtime", "password-hash")
	mustWritePrivate(t, target, "old\n", 0o600)
	parent, err := OpenDirectory(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}
	oldState, err := inspectFileState(parent.fd, filepath.Base(target))
	_ = parent.Close()
	if err != nil {
		t.Fatal(err)
	}
	replacement := target + ".replacement"
	mustWritePrivate(t, replacement, "new\n", 0o600)
	if err := os.Rename(replacement, target); err != nil {
		t.Fatal(err)
	}

	_, err = BeginExpected(
		filepath.Join(root, "session"),
		".password-hash-v1-",
		[]string{target},
		map[string]Identity{target: oldState.Identity},
	)
	if err == nil || !strings.Contains(err.Error(), "changed before transaction") {
		t.Fatalf("BeginExpected() error = %v, want replacement rejection", err)
	}
	if got := mustRead(t, target); got != "new\n" {
		t.Fatalf("replacement changed: %q", got)
	}
	assertNoRuntimeResidue(t, root)
}

func TestTransactionRollbackRestoresExactOriginalThenCleans(t *testing.T) {
	t.Parallel()

	root := privateTempDir(t)
	target := filepath.Join(root, "runtime", "shell-launcher.lua")
	mustWritePrivate(t, target, "original\n", 0o600)
	before, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}

	tx, err := Begin(filepath.Join(root, "session"), ".runtime-rollback-", []string{target})
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if err := Write(tx, target, []byte("replacement\n"), 0o644); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := Rollback(tx); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	after, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("rollback did not restore the exact original inode")
	}
	if got := mustRead(t, target); got != "original\n" {
		t.Fatalf("target = %q, want original payload", got)
	}
	if got := after.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
	if err := Write(tx, target, []byte("original\n"), 0o600); err != nil {
		t.Fatalf("unchanged Write() after rollback error = %v", err)
	}
	if err := Write(tx, target, []byte("fallback\n"), 0o644); err != nil {
		t.Fatalf("fallback Write() after rollback error = %v", err)
	}
	if err := Rollback(tx); err != nil {
		t.Fatalf("second Rollback() error = %v", err)
	}
	if err := Commit(tx); err != nil {
		t.Fatalf("Commit() after rollback error = %v", err)
	}
	assertNoRuntimeResidue(t, root)
}

func TestTransactionRollbackRestoresAbsence(t *testing.T) {
	t.Parallel()

	root := privateTempDir(t)
	target := filepath.Join(root, "runtime", "active-shell")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}

	tx, err := Begin(filepath.Join(root, "session"), ".state-switch-rollback-", []string{target})
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if err := Write(tx, target, []byte("noctalia\n"), 0o644); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := Rollback(tx); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("target after rollback error = %v, want absence", err)
	}
	if err := Commit(tx); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	assertNoRuntimeResidue(t, root)
}

func TestTransactionRemoveAndRollbackRestoresSymlinkInode(t *testing.T) {
	t.Parallel()

	root := privateTempDir(t)
	target := filepath.Join(root, "runtime", "legacy.conf")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("managed-target", target); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := Begin(filepath.Join(root, "session"), ".runtime-rollback-", []string{target})
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if err := Remove(tx, target); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed target error = %v, want absence", err)
	}
	if err := Rollback(tx); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	after, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("rollback did not restore exact symlink inode")
	}
	if link, err := os.Readlink(target); err != nil || link != "managed-target" {
		t.Fatalf("restored link = %q, err=%v", link, err)
	}
	if err := Commit(tx); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	assertNoRuntimeResidue(t, root)
}

func TestTransactionPreservesConcurrentTargetReplacement(t *testing.T) {
	t.Parallel()

	root := privateTempDir(t)
	target := filepath.Join(root, "runtime", "shell-keybinds.lua")
	mustWritePrivate(t, target, "old\n", 0o644)

	tx, err := Begin(filepath.Join(root, "session"), ".runtime-rollback-", []string{target})
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if err := Write(tx, target, []byte("managed\n"), 0o644); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	replacement := target + ".concurrent"
	mustWritePrivate(t, replacement, "concurrent\n", 0o644)
	if err := os.Rename(replacement, target); err != nil {
		t.Fatal(err)
	}

	if err := Rollback(tx); err == nil || !strings.Contains(err.Error(), "ownership collision") {
		t.Fatalf("Rollback() error = %v, want ownership collision", err)
	}
	if err := Commit(tx); err == nil || !strings.Contains(err.Error(), "ownership collision") {
		t.Fatalf("Commit() error = %v, want ownership collision", err)
	}
	if got := mustRead(t, target); got != "concurrent\n" {
		t.Fatalf("concurrent winner changed: %q", got)
	}
	if _, err := os.Lstat(tx); err != nil {
		t.Fatalf("recovery journal was not retained: %v", err)
	}
}

func TestScavengeOnlyProcessesSelfIdentifyingJournals(t *testing.T) {
	t.Parallel()

	root := privateTempDir(t)
	session := filepath.Join(root, "session")
	target := filepath.Join(root, "runtime", "shell-launcher.lua")
	mustWritePrivate(t, target, "old\n", 0o644)

	tx, err := Begin(session, ".runtime-rollback-", []string{target})
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if err := Write(tx, target, []byte("new\n"), 0o644); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	unknown := filepath.Join(session, ".runtime-rollback-unknown")
	if err := os.Mkdir(unknown, 0o700); err != nil {
		t.Fatal(err)
	}
	mustWritePrivate(t, filepath.Join(unknown, "user-data"), "keep\n", 0o600)

	result, err := Scavenge(session, []string{".runtime-rollback-"})
	if err != nil {
		t.Fatalf("Scavenge() error = %v", err)
	}
	if result.Cleaned != 1 || result.Preserved != 1 {
		t.Fatalf("Scavenge() = %+v, want one cleaned and one preserved", result)
	}
	if got := mustRead(t, target); got != "old\n" {
		t.Fatalf("active transaction was not rolled back: %q", got)
	}
	if got := mustRead(t, filepath.Join(unknown, "user-data")); got != "keep\n" {
		t.Fatalf("unknown recovery changed: %q", got)
	}
	assertNoGlob(t, root, ".wahrwelt-runtime-stage-*")
}

func TestBeginRejectsSymlinkTransactionRoot(t *testing.T) {
	t.Parallel()

	root := privateTempDir(t)
	realRoot := filepath.Join(root, "real-session")
	if err := os.Mkdir(realRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	linkedRoot := filepath.Join(root, "session")
	if err := os.Symlink(realRoot, linkedRoot); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "runtime", "active-shell")
	mustWritePrivate(t, target, "caelestia\n", 0o644)

	if _, err := Begin(linkedRoot, ".runtime-rollback-", []string{target}); err == nil {
		t.Fatal("Begin() accepted a symlink transaction root")
	}
	entries, err := os.ReadDir(realRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("symlink root gained entries: %v", entries)
	}
}
