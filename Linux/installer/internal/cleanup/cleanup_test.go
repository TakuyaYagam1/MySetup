package cleanup

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
)

func TestRunDryRunPrintsCleanupCommands(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "Pictures/Wallpapers"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".cache/noctalia"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".config/nvim"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".config/nvim/init.lua.backup"), []byte("-- backup\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := Run(context.Background(), Options{DryRun: true, Yes: true}); err != nil {
			t.Fatal(err)
		}
	})

	for _, want := range []string{"rm -rf", ".cache/noctalia"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected cleanup output to contain %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "preview-") {
		t.Fatalf("cleanup must not claim unmarked wallpaper previews:\n%s", out)
	}
}

func TestReportForHomeListsCleanupCandidates(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	report := ReportForHome(home)
	for _, want := range []string{
		"== Safe cleanup candidates ==",
		filepath.Join(home, ".cache/noctalia"),
		filepath.Join(home, ".cache/nvim/treesitter"),
		filepath.Join(home, ".local/share/nvim/treesitter"),
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("expected cleanup report to contain %q, got:\n%s", want, report)
		}
	}
	if strings.Contains(report, ".cache/thumbnails") {
		t.Fatalf("thumbnail cache must not be a managed cleanup target, got:\n%s", report)
	}
	if strings.Contains(report, "*.backup") {
		t.Fatalf("unowned Home Manager-style backups must not be cleanup targets, got:\n%s", report)
	}
	if strings.Contains(report, "preview-") {
		t.Fatalf("unmarked wallpaper previews must not be cleanup targets, got:\n%s", report)
	}
}

func TestRunDoesNotInvokeWallpaperFind(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "rm"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "find"), []byte("#!/bin/sh\nexit 23\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	if err := Run(context.Background(), Options{Yes: true}); err != nil {
		t.Fatalf("cleanup invoked the forbidden wallpaper find command: %v", err)
	}
}

func TestRunUsesStateHomeWhenProvided(t *testing.T) {
	home := t.TempDir()
	otherHome := t.TempDir()
	t.Setenv("HOME", otherHome)
	if err := os.MkdirAll(filepath.Join(home, ".cache/noctalia"), 0o755); err != nil {
		t.Fatal(err)
	}

	state := config.Default()
	state.User.HomeDirectory = home
	out := captureStdout(t, func() {
		if err := Run(context.Background(), Options{State: state, DryRun: true, Yes: true}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, filepath.Join(home, ".cache/noctalia")) {
		t.Fatalf("cleanup should use state home, got:\n%s", out)
	}
	if strings.Contains(out, otherHome) {
		t.Fatalf("cleanup should not use process HOME when state home is provided, got:\n%s", out)
	}
}

func TestRunReturnsCleanupCommandError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".cache/noctalia"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "rm"), []byte("#!/bin/sh\nexit 23\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	err := Run(context.Background(), Options{Yes: true})
	if err == nil {
		t.Fatal("expected cleanup command error")
	}
	if !strings.Contains(err.Error(), "rm failed") {
		t.Fatalf("expected rm failure, got %v", err)
	}
}

func TestRunPreservesUnmarkedWallpaperContentAndModes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	wallpapers := filepath.Join(home, "Pictures", "Wallpapers")
	nested := filepath.Join(wallpapers, "nested")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	preview := filepath.Join(wallpapers, "preview-user.png")
	readonly := filepath.Join(nested, "readonly-user.png")
	if err := os.WriteFile(preview, []byte("preview\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readonly, []byte("nested\n"), 0o440); err != nil {
		t.Fatal(err)
	}

	if err := Run(context.Background(), Options{Yes: true, Stdout: io.Discard}); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		path    string
		content string
		mode    os.FileMode
	}{
		{path: preview, content: "preview\n", mode: 0o640},
		{path: readonly, content: "nested\n", mode: 0o440},
	} {
		data, err := os.ReadFile(item.path)
		if err != nil || string(data) != item.content {
			t.Fatalf("cleanup changed %s: data=%q err=%v", item.path, data, err)
		}
		info, err := os.Stat(item.path)
		if err != nil || info.Mode().Perm() != item.mode {
			t.Fatalf("cleanup changed %s mode: info=%v err=%v", item.path, info, err)
		}
	}
}

func TestRunPreservesUnownedHomeManagerStyleBackups(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	backup := filepath.Join(home, ".config", "nvim", "init.lua.backup")
	if err := os.MkdirAll(filepath.Dir(backup), 0o755); err != nil {
		t.Fatal(err)
	}
	want := []byte("user-owned backup\n")
	if err := os.WriteFile(backup, want, 0o640); err != nil {
		t.Fatal(err)
	}

	if err := Run(context.Background(), Options{Yes: true, Stdout: io.Discard}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("cleanup removed an unowned backup: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("cleanup changed an unowned backup: got %q want %q", got, want)
	}
	info, err := os.Stat(backup)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("cleanup changed backup mode: got %04o want 0640", info.Mode().Perm())
	}
}

func TestRunUsesDirectRmInsteadOfShell(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".cache/noctalia"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "rm"), []byte("#!/bin/sh\nprintf '%s\\n' \"$0 $@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	out := captureStdout(t, func() {
		if err := Run(context.Background(), Options{Yes: true}); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(out, "sh -c") {
		t.Fatalf("cleanup must not use shell interpolation\n%s", out)
	}
	if !strings.Contains(out, "rm -rf") {
		t.Fatalf("cleanup should log direct rm command\n%s", out)
	}
}

func TestRunOnlyValidatesActiveEnd4ProfileLink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	hmGeneration := filepath.Join(t.TempDir(), "home-manager-generation")
	end4Source := filepath.Join(hmGeneration, "home-files", ".config", "hypr", "end4")
	end4Artifact := filepath.Join(t.TempDir(), "end4-artifact")
	if err := os.MkdirAll(filepath.Dir(end4Source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(end4Artifact, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(end4Artifact, "hyprland.lua"), []byte("require(\"hyprland.env\")\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(end4Artifact, end4Source); err != nil {
		t.Fatal(err)
	}
	gcroot := filepath.Join(home, ".local", "state", "home-manager", "gcroots", "current-home")
	if err := os.MkdirAll(filepath.Dir(gcroot), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(hmGeneration, gcroot); err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(home, ".local", "state", "wahrwelt")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "active-shell"), []byte("end4\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(home, ".config", "hypr", "end4")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(end4Source, target); err != nil {
		t.Fatal(err)
	}
	originalTarget, err := os.Readlink(target)
	if err != nil {
		t.Fatal(err)
	}

	captureStdout(t, func() {
		if err := Run(context.Background(), Options{DryRun: true, Yes: true}); err != nil {
			t.Fatal(err)
		}
	})
	gotTarget, err := os.Readlink(target)
	if err != nil {
		t.Fatal(err)
	}
	if gotTarget != originalTarget {
		t.Fatalf("cleanup changed Home Manager End4 link: got %q want %q", gotTarget, originalTarget)
	}
}

func TestRunRejectsUnownedActiveEnd4ProfileCollisionsWithoutMutation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	stateDir := filepath.Join(home, ".local", "state", "wahrwelt")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "active-shell"), []byte("end4-pc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, ".config", "hypr", "end4")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("foreign End4 file\n"), 0o640); err != nil {
		t.Fatal(err)
	}

	err := Run(context.Background(), Options{DryRun: true, Yes: true, Stdout: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "refusing to mutate unowned End4 profile collision") {
		t.Fatalf("cleanup must reject unowned End4 collision, err=%v", err)
	}
	data, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "foreign End4 file\n" {
		t.Fatalf("cleanup changed unowned End4 collision: %q", data)
	}
	info, statErr := os.Stat(target)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("cleanup changed unowned End4 collision mode: %o", info.Mode().Perm())
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = write
	defer func() {
		os.Stdout = original
	}()

	fn()
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := io.Copy(&out, read); err != nil {
		t.Fatal(err)
	}
	return out.String()
}
