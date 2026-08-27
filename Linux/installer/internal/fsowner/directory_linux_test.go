//go:build linux

package fsowner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
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

func TestDirectoryRemoveDirectoryRejectsSameDeviceBindMount(t *testing.T) {
	const childEnvironment = "WAHRWELT_TEST_FSOWNER_BIND_MOUNT"
	if os.Getenv(childEnvironment) != "1" {
		if _, err := exec.LookPath("unshare"); err != nil {
			t.Skip("unshare is unavailable")
		}
		cmd := exec.Command(
			"unshare", "--user", "--map-root-user", "--mount", "--propagation", "private", "--",
			os.Args[0], "-test.run=^TestDirectoryRemoveDirectoryRejectsSameDeviceBindMount$",
		)
		cmd.Env = append(os.Environ(), childEnvironment+"=1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			if strings.Contains(string(output), "Operation not permitted") {
				t.Skipf("mount namespace is unavailable: %s", strings.TrimSpace(string(output)))
			}
			t.Fatalf("bind-mount namespace test failed: %v\n%s", err, output)
		}
		return
	}

	if os.Geteuid() != 0 {
		t.Fatalf("bind-mount child must run as namespace root, euid=%d", os.Geteuid())
	}
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	tree := filepath.Join(parent, "generated")
	mountpoint := filepath.Join(tree, "mounted")
	outside := filepath.Join(root, "outside")
	for _, path := range []string{mountpoint, outside} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	payload := filepath.Join(outside, "payload")
	mustWritePrivate(t, payload, "outside\n", 0o600)
	if err := unix.Mount(outside, mountpoint, "", unix.MS_BIND, ""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unix.Unmount(mountpoint, unix.MNT_DETACH) })

	directory, err := OpenDirectory(parent)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = directory.Close() })
	entry, err := directory.Inspect(filepath.Base(tree))
	if err != nil {
		t.Fatal(err)
	}
	err = directory.RemoveDirectory(entry.Name, entry.Identity, RemoveOptions{
		UID:        uint32(os.Geteuid()),
		Recursive:  true,
		SameDevice: true,
	})
	if err == nil {
		t.Fatal("RemoveDirectory() accepted a same-device bind mount")
	}
	if got := mustRead(t, payload); got != "outside\n" {
		t.Fatalf("bind-mounted external payload changed: %q", got)
	}
}

func TestDirectoryRemoveDirectoryRejectsSameDeviceRootBindMount(t *testing.T) {
	const childEnvironment = "WAHRWELT_TEST_FSOWNER_ROOT_BIND_MOUNT"
	if os.Getenv(childEnvironment) != "1" {
		if _, err := exec.LookPath("unshare"); err != nil {
			t.Skip("unshare is unavailable")
		}
		cmd := exec.Command(
			"unshare", "--user", "--map-root-user", "--mount", "--propagation", "private", "--",
			os.Args[0], "-test.run=^TestDirectoryRemoveDirectoryRejectsSameDeviceRootBindMount$",
		)
		cmd.Env = append(os.Environ(), childEnvironment+"=1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			if strings.Contains(string(output), "Operation not permitted") {
				t.Skipf("mount namespace is unavailable: %s", strings.TrimSpace(string(output)))
			}
			t.Fatalf("root bind-mount namespace test failed: %v\n%s", err, output)
		}
		return
	}

	if os.Geteuid() != 0 {
		t.Fatalf("root bind-mount child must run as namespace root, euid=%d", os.Geteuid())
	}
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	tree := filepath.Join(parent, "generated")
	outside := filepath.Join(root, "outside")
	for _, path := range []string{tree, outside} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	payload := filepath.Join(outside, "payload")
	mustWritePrivate(t, payload, "outside\n", 0o600)
	if err := unix.Mount(outside, tree, "", unix.MS_BIND, ""); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unix.Unmount(tree, unix.MNT_DETACH) })

	directory, err := OpenDirectory(parent)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = directory.Close() })
	entry, err := directory.Inspect(filepath.Base(tree))
	if err != nil {
		t.Fatal(err)
	}
	err = directory.RemoveDirectory(entry.Name, entry.Identity, RemoveOptions{
		UID:        uint32(os.Geteuid()),
		Recursive:  true,
		SameDevice: true,
	})
	if err == nil {
		t.Fatal("RemoveDirectory() accepted a same-device root bind mount")
	}
	if got := mustRead(t, payload); got != "outside\n" {
		t.Fatalf("root bind-mounted external payload changed: %q", got)
	}
}
