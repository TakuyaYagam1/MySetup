package secrets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFromFiles(t *testing.T) {
	dir := t.TempDir()
	userPassword := filepath.Join(dir, "user-password")
	if err := os.WriteFile(userPassword, []byte("linux-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	secrets, err := LoadFromFiles(userPassword)
	if err != nil {
		t.Fatal(err)
	}
	if secrets.UserPassword != "linux-secret" {
		t.Fatalf("unexpected secrets: %#v", secrets)
	}
}

func TestReadFileRejectsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(path, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := readFile(path)
	if err == nil {
		t.Fatal("expected empty secret file error")
	}
	if !strings.Contains(err.Error(), "secret file is empty") {
		t.Fatalf("expected empty secret error, got %v", err)
	}
}

func TestReadFileRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")
	if err := os.WriteFile(target, []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	_, err := readFile(link)
	if err == nil {
		t.Fatal("expected symlink secret rejection")
	}
	if !strings.Contains(err.Error(), "must not be a symlink") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
}

func TestReadFileRejectsNonRegularFile(t *testing.T) {
	_, err := readFile(t.TempDir())
	if err == nil {
		t.Fatal("expected non-regular secret file rejection")
	}
	if !strings.Contains(err.Error(), "must be a regular file") {
		t.Fatalf("expected non-regular rejection, got %v", err)
	}
}

func TestReadFileRejectsOpenPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte("secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := readFile(path)
	if err == nil {
		t.Fatal("expected permissive secret file rejection")
	}
	if !strings.Contains(err.Error(), "permissions are too open") {
		t.Fatalf("expected permission rejection, got %v", err)
	}
}

func TestReadFileRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", MaxSecretFileBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := readFile(path)
	if err == nil {
		t.Fatal("expected oversized secret file rejection")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected size rejection, got %v", err)
	}
}
