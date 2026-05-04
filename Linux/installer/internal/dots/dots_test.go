package dots

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

func TestWriteShellProfileConfigUsesCurrentProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shell-profile.conf")
	state := config.Default()
	state.Shell.Profile = "noctalia"

	if err := writeShellProfileConfig(path, state); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"# Active shell profile: noctalia",
		"exec-once = " + filepath.Join(dir, "scripts", "start-shell.sh") + " noctalia",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("shell profile missing %q\n%s", want, got)
		}
	}
}

func TestWriteShellProfileConfigDefaultsToCaelestia(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shell-profile.conf")
	state := config.Default()
	state.Shell.Profile = ""

	if err := writeShellProfileConfig(path, state); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "start-shell.sh caelestia") {
		t.Fatalf("shell profile should default to caelestia\n%s", string(data))
	}
}

func TestCopyWallpapersReturnsPreviewCleanupError(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "Wallpapers"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeScript(t, filepath.Join(bin, "mkdir"), "exit 0")
	writeScript(t, filepath.Join(bin, "find"), "exit 17")
	writeScript(t, filepath.Join(bin, "rsync"), "exit 0")
	t.Setenv("PATH", bin)

	var out bytes.Buffer
	runner := run.Runner{Stdout: &out, Stderr: &out}
	err := copyWallpapers(context.Background(), runner, src, t.TempDir())
	if err == nil {
		t.Fatal("expected preview cleanup error")
	}
	if !strings.Contains(err.Error(), "find failed") {
		t.Fatalf("expected find failure, got %v", err)
	}
}

func TestSetupZenMissingProfileWarnsAndSkips(t *testing.T) {
	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	err := setupZen(context.Background(), runner, t.TempDir(), t.TempDir(), config.Dots{ZenTheme: true})
	if err != nil {
		t.Fatalf("missing Zen profile should be a warning, got %v", err)
	}
}

func TestSetupZenErrorsWhenThemeSourceMissing(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".zen", "profile.Default"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	err := setupZen(context.Background(), runner, t.TempDir(), home, config.Dots{ZenTheme: true})
	if err == nil {
		t.Fatal("expected missing Zen theme source error")
	}
	if !strings.Contains(err.Error(), "theme source missing") {
		t.Fatalf("expected theme source error, got %v", err)
	}
}

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
	err := backupIfUnmanaged(context.Background(), runner, target)
	if err == nil {
		t.Fatal("expected marker stat error")
	}
	if !strings.Contains(err.Error(), "stat managed marker") {
		t.Fatalf("expected marker stat error, got %v", err)
	}
}

func TestSyncHyprReturnsExecutableCommandError(t *testing.T) {
	dotsSrc := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dotsSrc, "hypr"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeScript(t, filepath.Join(bin, "rsync"), `last=
for arg do last=$arg; done
mkdir -p "$last/scripts"`)
	writeScript(t, filepath.Join(bin, "chmod"), "exit 0")
	writeScript(t, filepath.Join(bin, "find"), "exit 27")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	var out bytes.Buffer
	runner := run.Runner{Stdout: &out, Stderr: &out}
	err := syncHypr(context.Background(), runner, dotsSrc, t.TempDir(), config.Default())
	if err == nil {
		t.Fatal("expected scripts executable command error")
	}
	if !strings.Contains(err.Error(), "find failed") {
		t.Fatalf("expected find failure, got %v", err)
	}
}

func TestSyncHyprIgnoresHyprctlReloadFailure(t *testing.T) {
	dotsSrc := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dotsSrc, "hypr"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeScript(t, filepath.Join(bin, "rsync"), `last=
for arg do last=$arg; done
mkdir -p "$last/scripts"`)
	writeScript(t, filepath.Join(bin, "chmod"), "exit 0")
	writeScript(t, filepath.Join(bin, "find"), "exit 0")
	writeScript(t, filepath.Join(bin, "sh"), "exit 42")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	var out bytes.Buffer
	runner := run.Runner{Stdout: &out, Stderr: &out}
	if err := syncHypr(context.Background(), runner, dotsSrc, t.TempDir(), config.Default()); err != nil {
		t.Fatalf("hyprctl reload failure should stay best-effort, got %v", err)
	}
}

func writeScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}
