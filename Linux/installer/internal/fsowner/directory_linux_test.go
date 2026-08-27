//go:build linux

package fsowner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveOptionsAllowsOnlyPrimaryAndExplicitAdditionalUIDs(t *testing.T) {
	t.Parallel()

	options := RemoveOptions{UID: 0, AdditionalUIDs: []uint32{1000}}
	if !options.AllowsUID(0) {
		t.Fatal("primary uid was rejected")
	}
	if !options.AllowsUID(1000) {
		t.Fatal("explicit additional uid was rejected")
	}
	if options.AllowsUID(1001) {
		t.Fatal("unknown uid was accepted")
	}
}

func TestDirectoryRemoveRegularRequiresExactIdentity(t *testing.T) {
	t.Parallel()

	parent := privateTempDir(t)
	path := filepath.Join(parent, "generated")
	mustWritePrivate(t, path, "owned\n", 0o600)
	directory, err := OpenDirectory(parent)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = directory.Close() })
	entry, err := directory.Inspect("generated")
	if err != nil {
		t.Fatal(err)
	}

	replacement := filepath.Join(parent, "replacement")
	mustWritePrivate(t, replacement, "winner\n", 0o600)
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	if err := directory.RemoveRegular("generated", entry.Identity, RemoveOptions{UID: uint32(os.Geteuid())}); err == nil {
		t.Fatal("RemoveRegular() removed or accepted a replacement")
	}
	if got := mustRead(t, path); got != "winner\n" {
		t.Fatalf("replacement changed: %q", got)
	}
}

func TestDirectoryRemoveDirectoryRejectsSymlinkCandidate(t *testing.T) {
	t.Parallel()

	parent := privateTempDir(t)
	tree := filepath.Join(parent, "elsewhere")
	if err := os.Mkdir(tree, 0o700); err != nil {
		t.Fatal(err)
	}
	mustWritePrivate(t, filepath.Join(tree, "payload"), "owned\n", 0o600)
	linkName := "nixos.bak.1.2.3"
	if err := os.Symlink(tree, filepath.Join(parent, linkName)); err != nil {
		t.Fatal(err)
	}
	directory, err := OpenDirectory(parent)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = directory.Close() })
	entry, err := directory.Inspect(linkName)
	if err != nil {
		t.Fatal(err)
	}

	if err := directory.RemoveDirectory(entry.Name, entry.Identity, RemoveOptions{
		UID:        uint32(os.Geteuid()),
		Recursive:  true,
		SameDevice: true,
	}); err == nil {
		t.Fatal("RemoveDirectory() accepted a symlink candidate")
	}
	if got := mustRead(t, filepath.Join(tree, "payload")); got != "owned\n" {
		t.Fatalf("tree changed after refusal: %q", got)
	}
}

func TestDirectoryRemoveKnownTreeAndEmptyDirectory(t *testing.T) {
	t.Parallel()

	parent := privateTempDir(t)
	tree := filepath.Join(parent, "nixos.bak.1.2.3")
	if err := os.MkdirAll(filepath.Join(tree, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	mustWritePrivate(t, filepath.Join(tree, "nested", "payload"), "owned\n", 0o600)
	empty := filepath.Join(parent, ".wahrwelt-workspace-owned")
	if err := os.Mkdir(empty, 0o700); err != nil {
		t.Fatal(err)
	}

	directory, err := OpenDirectory(parent)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = directory.Close() })
	treeEntry, err := directory.Inspect(filepath.Base(tree))
	if err != nil {
		t.Fatal(err)
	}
	emptyEntry, err := directory.Inspect(filepath.Base(empty))
	if err != nil {
		t.Fatal(err)
	}

	if err := directory.RemoveDirectory(treeEntry.Name, treeEntry.Identity, RemoveOptions{
		UID:        uint32(os.Geteuid()),
		Recursive:  true,
		SameDevice: true,
	}); err != nil {
		t.Fatalf("RemoveDirectory(recursive) error = %v", err)
	}
	if err := directory.RemoveDirectory(emptyEntry.Name, emptyEntry.Identity, RemoveOptions{
		UID:          uint32(os.Geteuid()),
		RequireEmpty: true,
	}); err != nil {
		t.Fatalf("RemoveDirectory(empty) error = %v", err)
	}
	for _, path := range []string{tree, empty} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("%s still exists: %v", path, err)
		}
	}
}
