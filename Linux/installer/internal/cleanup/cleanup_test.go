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

	for _, want := range []string{"rm -rf", "preview-*", ".cache/noctalia", "*.backup"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected cleanup output to contain %q\n%s", want, out)
		}
	}
}

func TestReportForHomeListsCleanupCandidates(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	report := ReportForHome(home)
	for _, want := range []string{
		"== Safe cleanup candidates ==",
		filepath.Join(home, "Pictures/Wallpapers/preview-*"),
		filepath.Join(home, ".cache/noctalia"),
		filepath.Join(home, ".cache/nvim/treesitter"),
		filepath.Join(home, ".local/share/nvim/treesitter"),
		filepath.Join(home, ".config/**/*.backup"),
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("expected cleanup report to contain %q, got:\n%s", want, report)
		}
	}
	if strings.Contains(report, ".cache/thumbnails") {
		t.Fatalf("thumbnail cache must not be a managed cleanup target, got:\n%s", report)
	}
}

func TestRunSkipsMissingWallpaperDir(t *testing.T) {
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
		t.Fatalf("missing wallpaper dir should be skipped, got %v", err)
	}
}

func TestRunUsesStateHomeWhenProvided(t *testing.T) {
	home := t.TempDir()
	otherHome := t.TempDir()
	t.Setenv("HOME", otherHome)
	if err := os.MkdirAll(filepath.Join(home, "Pictures/Wallpapers"), 0o755); err != nil {
		t.Fatal(err)
	}

	state := config.Default()
	state.User.HomeDirectory = home
	out := captureStdout(t, func() {
		if err := Run(context.Background(), Options{State: state, DryRun: true, Yes: true}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, filepath.Join(home, "Pictures/Wallpapers/preview-*")) {
		t.Fatalf("cleanup should use state home, got:\n%s", out)
	}
	if strings.Contains(out, otherHome) {
		t.Fatalf("cleanup should not use process HOME when state home is provided, got:\n%s", out)
	}
}

func TestRunReturnsCleanupCommandError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "Pictures/Wallpapers"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "rm"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "find"), []byte("#!/bin/sh\nexit 23\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	err := Run(context.Background(), Options{Yes: true})
	if err == nil {
		t.Fatal("expected cleanup command error")
	}
	if !strings.Contains(err.Error(), "find failed") {
		t.Fatalf("expected find failure, got %v", err)
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
