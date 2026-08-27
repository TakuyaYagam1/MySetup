package rollback

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
)

type recordingRunner struct {
	calls   int
	command string
	args    []string
}

func (r *recordingRunner) Command(_ context.Context, command string, args ...string) error {
	r.calls++
	r.command = command
	r.args = append([]string(nil), args...)
	return nil
}

func (*recordingRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (*recordingRunner) IsDryRun() bool { return false }

func TestFindLatestPicksMostRecentBackup(t *testing.T) {
	parent := t.TempDir()
	dest := filepath.Join(parent, "nixos")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	older := filepath.Join(parent, "nixos.bak.111.0.0")
	newer := filepath.Join(parent, "nixos.bak.222.0.0")
	for _, p := range []string{older, newer} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(older, past, past); err != nil {
		t.Fatal(err)
	}

	got, err := FindLatest(dest)
	if err != nil {
		t.Fatalf("FindLatest returned error: %v", err)
	}
	if got != newer {
		t.Fatalf("FindLatest = %s; want %s", got, newer)
	}
}

func TestFindLatestErrorsWhenNoBackup(t *testing.T) {
	parent := t.TempDir()
	dest := filepath.Join(parent, "nixos")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := FindLatest(dest); err == nil {
		t.Fatal("expected error when no backups present")
	}
}

func TestValidateBackupPath(t *testing.T) {
	parent := t.TempDir()
	dest := filepath.Join(parent, "nixos")
	good := filepath.Join(parent, "nixos.bak.42.0.0")
	bad := filepath.Join(parent, "unrelated")
	siblingWrongPrefix := filepath.Join(parent, "nixos-old")
	wrongDir := filepath.Join(t.TempDir(), "nixos.bak.42.0.0")

	if err := validateBackupPath(good, dest); err != nil {
		t.Fatalf("good path rejected: %v", err)
	}
	if err := validateBackupPath(bad, dest); err == nil {
		t.Fatal("expected rejection of unrelated path")
	}
	if err := validateBackupPath(siblingWrongPrefix, dest); err == nil {
		t.Fatal("expected rejection of sibling lacking .bak. prefix")
	}
	if err := validateBackupPath(wrongDir, dest); err == nil || !strings.Contains(err.Error(), "must live in") {
		t.Fatalf("expected directory-mismatch error, got %v", err)
	}
}

func TestRunRejectsSymlinkDestinationBeforePrivilegedDelete(t *testing.T) {
	parent := t.TempDir()
	realDest := filepath.Join(parent, "real-nixos")
	if err := os.Mkdir(realDest, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(parent, "nixos")
	if err := os.Symlink(realDest, dest); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(parent, "nixos.bak.42.0.0")
	if err := os.Mkdir(backup, 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{}

	err := Run(context.Background(), Options{
		Paths:  paths.Options{NixOSDest: dest},
		Backup: backup,
		Yes:    true,
		Runner: runner,
	})
	if err == nil || !strings.Contains(err.Error(), "destination must be an ordinary directory") {
		t.Fatalf("expected symlink destination rejection, got %v", err)
	}
	if runner.calls != 0 {
		t.Fatalf("privileged command ran for symlink destination: %d calls", runner.calls)
	}
	if _, err := os.Stat(filepath.Join(realDest, "sentinel")); !os.IsNotExist(err) {
		t.Fatalf("symlink destination target was mutated: %v", err)
	}
}

func TestRunRejectsSymlinkBackupBeforePrivilegedRead(t *testing.T) {
	parent := t.TempDir()
	dest := filepath.Join(parent, "nixos")
	realBackup := filepath.Join(parent, "real-backup")
	for _, directory := range []string{dest, realBackup} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	backup := filepath.Join(parent, "nixos.bak.42.0.0")
	if err := os.Symlink(realBackup, backup); err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{}

	err := Run(context.Background(), Options{
		Paths:  paths.Options{NixOSDest: dest},
		Backup: backup,
		Yes:    true,
		Runner: runner,
	})
	if err == nil || !strings.Contains(err.Error(), "backup must be an ordinary directory") {
		t.Fatalf("expected symlink backup rejection, got %v", err)
	}
	if runner.calls != 0 {
		t.Fatalf("privileged command ran for symlink backup: %d calls", runner.calls)
	}
}

func TestRunPassesPinnedDirectoryDescriptorsToPrivilegedRsync(t *testing.T) {
	parent := t.TempDir()
	dest := filepath.Join(parent, "nixos")
	backup := filepath.Join(parent, "nixos.bak.42.0.0")
	for _, directory := range []string{dest, backup} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	runner := &recordingRunner{}

	if err := Run(context.Background(), Options{
		Paths:  paths.Options{NixOSDest: dest},
		Backup: backup,
		Yes:    true,
		Runner: runner,
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if runner.command != "sudo" || len(runner.args) != 6 {
		t.Fatalf("privileged call = %q %q", runner.command, runner.args)
	}
	if got := strings.Join(runner.args[:4], " "); got != "rsync -a --delete --" {
		t.Fatalf("rsync prefix = %q", got)
	}
	for _, argument := range runner.args[4:] {
		if !strings.HasPrefix(argument, "/proc/") || !strings.Contains(argument, "/fd/") || !strings.HasSuffix(argument, "/") {
			t.Fatalf("rsync received an unpinned filesystem path: %q", argument)
		}
	}
}

type swappingPinnedRunner struct {
	dest      string
	displaced string
	outside   string
}

func (r *swappingPinnedRunner) Command(_ context.Context, command string, args ...string) error {
	if command != "sudo" || len(args) != 6 {
		return os.ErrInvalid
	}
	if err := os.Rename(r.dest, r.displaced); err != nil {
		return err
	}
	if err := os.Symlink(r.outside, r.dest); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(args[5], "restored"), []byte("pinned\n"), 0o600)
}

func (*swappingPinnedRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (*swappingPinnedRunner) IsDryRun() bool { return false }

func TestRunDoesNotFollowDestinationSwappedAfterValidation(t *testing.T) {
	parent := t.TempDir()
	dest := filepath.Join(parent, "nixos")
	backup := filepath.Join(parent, "nixos.bak.42.0.0")
	displaced := filepath.Join(parent, "displaced")
	outside := filepath.Join(parent, "outside")
	for _, directory := range []string{dest, backup, outside} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(outside, "sentinel"), []byte("outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &swappingPinnedRunner{dest: dest, displaced: displaced, outside: outside}

	err := Run(context.Background(), Options{
		Paths:  paths.Options{NixOSDest: dest},
		Backup: backup,
		Yes:    true,
		Runner: runner,
	})
	if err == nil || !strings.Contains(err.Error(), "visible path changed") {
		t.Fatalf("Run() error = %v, want post-restore collision", err)
	}
	if got, err := os.ReadFile(filepath.Join(outside, "sentinel")); err != nil || string(got) != "outside\n" {
		t.Fatalf("symlink target changed: %q, err=%v", got, err)
	}
	if _, err := os.Stat(filepath.Join(outside, "restored")); !os.IsNotExist(err) {
		t.Fatalf("restore escaped into swapped symlink target: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(displaced, "restored")); err != nil || string(got) != "pinned\n" {
		t.Fatalf("pinned destination was not retained: %q, err=%v", got, err)
	}
}
