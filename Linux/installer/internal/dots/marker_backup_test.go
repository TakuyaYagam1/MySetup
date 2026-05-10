package dots

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

func TestBackupIfUnmanagedReturnsMarkerStatError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission mode test is Unix-specific")
	}
	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(target, 0o755)
	})

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	err := backupIfUnmanaged(context.Background(), runner, target, "hypr")
	if err == nil {
		t.Fatal("expected marker stat error")
	}
	if !strings.Contains(err.Error(), "stat managed marker") {
		t.Fatalf("expected marker stat error, got %v", err)
	}
}

func TestBackupIfUnmanagedTrustsOnlyValidMarker(t *testing.T) {
	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, ".mysetup-managed.json"), []byte(`{"manager":"other","kind":"hypr","version":2}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := backupIfUnmanaged(context.Background(), runner, target, "hypr"); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "$ mv "+target+" "+target+".bak.") {
		t.Fatalf("invalid marker should be treated as unmanaged and backed up, got:\n%s", out.String())
	}
}

func TestBackupIfUnmanagedTreatsMarkerSymlinkAsUnmanaged(t *testing.T) {
	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	externalMarker := filepath.Join(t.TempDir(), "marker.json")
	if err := os.WriteFile(externalMarker, []byte(managedMarker("hypr")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalMarker, filepath.Join(target, ".mysetup-managed.json")); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := backupIfUnmanaged(context.Background(), runner, target, "hypr"); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "$ mv "+target+" "+target+".bak.") {
		t.Fatalf("marker symlink should be treated as unmanaged and backed up, got:\n%s", out.String())
	}
}

func TestBackupIfUnmanagedTreatsRootSymlinkAsUnmanaged(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeMarker(run.Runner{}, filepath.Join(outside, ".mysetup-managed.json"), "hypr"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, target); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := backupIfUnmanaged(context.Background(), runner, target, "hypr"); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "$ mv "+target+" "+target+".bak.") {
		t.Fatalf("root symlink should be backed up without following it, got:\n%s", out.String())
	}
}

func TestBackupIfUnmanagedBacksUpMarkerWithWrongKind(t *testing.T) {
	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeMarker(run.Runner{}, filepath.Join(target, ".mysetup-managed.json"), "nvim"); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := backupIfUnmanaged(context.Background(), runner, target, "hypr"); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out.String(), "$ mv "+target+" "+target+".bak.") {
		t.Fatalf("wrong-kind marker should be treated as unmanaged and backed up, got:\n%s", out.String())
	}
}

func TestBackupIfUnmanagedSkipsValidMarker(t *testing.T) {
	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeMarker(run.Runner{}, filepath.Join(target, ".mysetup-managed.json"), "hypr"); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := backupIfUnmanaged(context.Background(), runner, target, "hypr"); err != nil {
		t.Fatal(err)
	}

	if out.Len() != 0 {
		t.Fatalf("valid marker should not trigger backup command, got:\n%s", out.String())
	}
}

func TestWriteMarkerReplacesStaleReadOnlyMarker(t *testing.T) {
	target := filepath.Join(t.TempDir(), ".mysetup-managed.json")
	if err := os.WriteFile(target, []byte("stale\n"), 0o400); err != nil {
		t.Fatal(err)
	}

	if err := writeMarker(run.Runner{}, target, "hypr"); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"kind": "hypr"`) {
		t.Fatalf("marker was not replaced:\n%s", got)
	}
}

func TestWriteMarkerReplacesSymlinkWithoutFollowingIt(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, ".mysetup-managed.json")
	if err := os.Symlink(outside, target); err != nil {
		t.Fatal(err)
	}

	if err := writeMarker(run.Runner{}, target, "hypr"); err != nil {
		t.Fatal(err)
	}

	outsideData, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	if string(outsideData) != "outside\n" {
		t.Fatalf("writeMarker followed stale symlink:\n%s", outsideData)
	}
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("marker should be a regular file, got mode %s", info.Mode())
	}
}

func TestWriteMarkerWithOwnerFallsBackToSudoInstallOnPermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission mode test needs non-root Unix user")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0o700)
	})

	logFile := filepath.Join(t.TempDir(), "sudo.log")
	bin := t.TempDir()
	writeScript(t, filepath.Join(bin, "sudo"), `printf '%s\n' "$*" >> "$SUDO_LOG"`)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("SUDO_LOG", logFile)

	var out bytes.Buffer
	runner := run.Runner{Stdout: &out, Stderr: &out}
	err := writeMarkerWithOwner(context.Background(), runner, filepath.Join(dir, ".mysetup-managed.json"), "hypr", "tester")
	if err != nil {
		t.Fatal(err)
	}

	logData, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	log := string(logData)
	for _, want := range []string{
		"rm -f -- " + filepath.Join(dir, ".mysetup-managed.json"),
		"install -D -m 644 -o tester ",
		filepath.Join(dir, ".mysetup-managed.json"),
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("expected sudo fallback command %q, got:\n%s\nstdout:\n%s", want, log, out.String())
		}
	}
}

func TestEnsureUserWritableTreeRepairsReadonlyDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission mode test is Unix-specific")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0o700)
	})

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := ensureUserWritableTree(context.Background(), runner, dir, "tester"); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	for _, want := range []string{
		"sudo chown -R tester:",
		"chmod -R u+rwX",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected writable repair command %q, got:\n%s", want, got)
		}
	}
}

func TestEnsureUserWritableTreeRepairsNestedReadonlyStoreModes(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission mode test needs non-root Unix user")
	}
	dir := t.TempDir()
	nested := filepath.Join(dir, "lua", "config")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(nested, "options.lua")
	if err := os.WriteFile(file, []byte("-- options\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(nested, 0o555); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(file, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(nested, 0o755)
		_ = os.Chmod(file, 0o644)
	})

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := ensureUserWritableTree(context.Background(), runner, dir, "tester"); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	for _, want := range []string{
		"sudo chown -R tester:",
		"chmod -R u+rwX",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected writable repair command %q, got:\n%s", want, got)
		}
	}
}
