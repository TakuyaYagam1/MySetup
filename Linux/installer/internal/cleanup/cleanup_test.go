package cleanup

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDryRunPrintsCleanupCommands(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "Pictures/Wallpapers"), 0o755); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := Run(context.Background(), Options{DryRun: true, Yes: true}); err != nil {
			t.Fatal(err)
		}
	})

	for _, want := range []string{"rm -f", "preview-*", "wallpapers.json"} {
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
		filepath.Join(home, ".cache/noctalia/wallpapers.json"),
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("expected cleanup report to contain %q, got:\n%s", want, report)
		}
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
	if !strings.Contains(out, "rm -f") {
		t.Fatalf("cleanup should log direct rm command\n%s", out)
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
