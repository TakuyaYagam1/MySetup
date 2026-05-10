package dots

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyFileSHA256RejectsMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload.zip")
	if err := os.WriteFile(path, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := verifyFileSHA256(path, strings.Repeat("0", 64))
	if err == nil {
		t.Fatal("expected sha256 mismatch")
	}
	if !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("expected sha256 mismatch, got %v", err)
	}
}

func TestSafeExtractZipExtractsRegularFile(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "safe.zip")
	writeZip(t, zipPath, []zipEntry{{name: "dir/file.txt", body: "hello"}})
	dest := t.TempDir()

	if err := safeExtractZip(zipPath, dest); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "dir", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected extracted content: %q", string(data))
	}
}

func TestSafeExtractZipRejectsTraversal(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "traversal.zip")
	writeZip(t, zipPath, []zipEntry{{name: "../escape.txt", body: "nope"}})

	err := safeExtractZip(zipPath, t.TempDir())
	if err == nil {
		t.Fatal("expected traversal rejection")
	}
	if !strings.Contains(err.Error(), "outside destination") {
		t.Fatalf("expected outside destination error, got %v", err)
	}
}

func TestSafeExtractZipRejectsSymlink(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "symlink.zip")
	writeZip(t, zipPath, []zipEntry{{name: "link", body: "target", mode: os.ModeSymlink | 0o777}})

	err := safeExtractZip(zipPath, t.TempDir())
	if err == nil {
		t.Fatal("expected symlink rejection")
	}
	if !strings.Contains(err.Error(), "refusing to extract symlink") {
		t.Fatalf("expected symlink error, got %v", err)
	}
}

func TestSafeExtractZipRejectsPreExistingSymlinkPath(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "payload.zip")
	writeZip(t, zipPath, []zipEntry{{name: "linked/file.txt", body: "nope"}})
	dest := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dest, "linked")); err != nil {
		t.Fatal(err)
	}

	err := safeExtractZip(zipPath, dest)
	if err == nil {
		t.Fatal("expected pre-existing symlink path rejection")
	}
	if !strings.Contains(err.Error(), "existing symlink") {
		t.Fatalf("expected existing symlink rejection, got %v", err)
	}
}

type zipEntry struct {
	name string
	body string
	mode os.FileMode
}

func writeZip(t *testing.T, path string, entries []zipEntry) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name}
		if entry.mode != 0 {
			header.SetMode(entry.mode)
		}
		part, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(entry.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
