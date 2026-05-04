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

func TestRunSkipsMissingWallpaperDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
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
	if err := os.WriteFile(filepath.Join(bin, "sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
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
