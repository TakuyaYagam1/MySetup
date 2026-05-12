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

func TestSetupV2rayNSkipsWhenTargetRootMissing(t *testing.T) {
	home := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := setupV2rayN(context.Background(), run.Runner{Stdout: &stdout, Stderr: &stderr}, home)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String(), "command -v sing-box") {
		t.Fatalf("sing-box lookup should not run without v2rayN root, got %q", stdout.String())
	}
}

func TestSetupV2rayNSeedsSingBoxWhenTargetRootExists(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".local", "share", "v2rayN")
	bin := t.TempDir()
	singbox := filepath.Join(bin, "sing-box")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(singbox, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := setupV2rayN(context.Background(), run.Runner{Stdout: &stdout, Stderr: &stderr}, home)
	if err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(root, "bin", "sing_box", "sing-box")
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("seeded sing-box mode = %v, want 0755", info.Mode().Perm())
	}
}

func TestRefreshThumbnailDaemonsRestartsHelpers(t *testing.T) {
	var stdout, stderr bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &stdout, Stderr: &stderr}

	refreshThumbnailDaemons(context.Background(), runner, "alice", "/home/alice")

	text := stdout.String()
	for _, want := range []string{
		"pkill -u alice -x gvfsd",
		"pkill -u alice -x gvfsd-fuse",
		"pkill -u alice -x Thunar",
		"pkill -u alice -x thunar",
		"pkill -u alice -f gvfs-udisks2-volume-monitor",
		"pkill -u alice -f tumbler-1/tumblerd",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("refreshThumbnailDaemons missing %q in command log:\n%s", want, text)
		}
	}
	if strings.Contains(text, ".cache/thumbnails") {
		t.Fatalf("thumbnail cache must not be wiped; got:\n%s", text)
	}
}

func TestRefreshThumbnailDaemonsSkipsWithoutUsername(t *testing.T) {
	var stdout bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &stdout}

	refreshThumbnailDaemons(context.Background(), runner, "", "/home/alice")

	if stdout.Len() != 0 {
		t.Fatalf("no commands expected without username, got %q", stdout.String())
	}
}
