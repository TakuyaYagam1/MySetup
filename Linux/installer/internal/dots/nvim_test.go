package dots

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
)

func TestSyncNvimPreservesRuntimeState(t *testing.T) {
	dotsSrc := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dotsSrc, "nvim"), 0o755); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	configDir := filepath.Join(home, ".config")
	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := syncNvim(context.Background(), runner, dotsSrc, configDir, "tester"); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(out.String(), "rm -rf --") {
		t.Fatalf("neovim sync must not wipe runtime state, got:\n%s", out.String())
	}
}

func TestSyncNvimSkipsSyncWhenSourceAlreadyInstalled(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(dst, ".wahrwelt-managed.json"), []byte(managedMarkerWithSourceHash("nvim", sourceHash)), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := syncNvim(context.Background(), runner, dotsSrc, configDir, "tester"); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, forbidden := range []string{"rsync", "rm -rf --"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("already-installed Neovim should skip %q, got:\n%s", forbidden, got)
		}
	}
}
