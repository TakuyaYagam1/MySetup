package dots

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

func TestSyncNvimCleansRuntimeStateWhenEnabled(t *testing.T) {
	dotsSrc := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dotsSrc, "nvim"), 0o755); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	configDir := filepath.Join(home, ".config")
	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := syncNvim(context.Background(), runner, dotsSrc, configDir, "tester", true); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	for _, want := range []string{
		"rm -rf --",
		filepath.Join(home, ".local", "share", "nvim"),
		filepath.Join(home, ".local", "state", "nvim"),
		filepath.Join(home, ".cache", "nvim"),
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected neovim cleanup output to contain %q, got:\n%s", want, got)
		}
	}
}

func TestSyncNvimPreservesRuntimeStateWhenDisabled(t *testing.T) {
	dotsSrc := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dotsSrc, "nvim"), 0o755); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := syncNvim(context.Background(), runner, dotsSrc, filepath.Join(t.TempDir(), ".config"), "tester", false); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(out.String(), "rm -rf --") {
		t.Fatalf("neovim cleanup should be skipped when disabled, got:\n%s", out.String())
	}
}

func TestSyncNvimSkipsSyncAndCleanupWhenSourceAlreadyInstalled(t *testing.T) {
	dotsSrc := t.TempDir()
	src := filepath.Join(dotsSrc, "nvim")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "init.lua"), []byte("-- init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	configDir := filepath.Join(home, ".config")
	dst := filepath.Join(configDir, "nvim")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "init.lua"), []byte("-- init\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceHash, err := sourceTreeHash(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, ".mysetup-managed.json"), []byte(managedMarkerWithSourceHash("nvim", sourceHash)), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := syncNvim(context.Background(), runner, dotsSrc, configDir, "tester", true); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, forbidden := range []string{"rsync", "rm -rf --"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("already-installed Neovim should skip %q, got:\n%s", forbidden, got)
		}
	}
}
