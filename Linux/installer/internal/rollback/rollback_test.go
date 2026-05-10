package rollback

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
