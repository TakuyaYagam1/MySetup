package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSourcesFromRepositoryRoot(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "Linux", "NixOS"))
	mkdir(t, filepath.Join(root, "Linux", "dots"))
	mkdir(t, filepath.Join(root, "Linux", "installer"))
	write(t, filepath.Join(root, "Linux", "NixOS", "flake.nix"))

	src, err := ResolveSources(root)
	if err != nil {
		t.Fatal(err)
	}
	if src.RepoRoot != root ||
		src.NixOS != filepath.Join(root, "Linux", "NixOS") ||
		src.Dots != filepath.Join(root, "Linux", "dots") ||
		src.Installer != filepath.Join(root, "Linux", "installer") {
		t.Fatalf("unexpected repo sources: %#v", src)
	}
}

func TestResolveSourcesFromRepositoryNixOSDir(t *testing.T) {
	root := t.TempDir()
	nixos := filepath.Join(root, "NixOS")
	dots := filepath.Join(root, "dots")
	installer := filepath.Join(root, "installer")
	mkdir(t, nixos)
	mkdir(t, dots)
	mkdir(t, installer)
	write(t, filepath.Join(nixos, "flake.nix"))

	src, err := ResolveSources(nixos)
	if err != nil {
		t.Fatal(err)
	}
	if src.NixOS != nixos || src.Dots != dots || src.Installer != installer {
		t.Fatalf("expected sibling dots for repo NixOS dir, got: %#v", src)
	}
}

func TestResolveSourcesFromInstalledMirror(t *testing.T) {
	nixos := t.TempDir()
	dots := filepath.Join(nixos, "dots")
	installer := filepath.Join(nixos, "installer")
	mkdir(t, dots)
	mkdir(t, installer)
	write(t, filepath.Join(nixos, "flake.nix"))

	src, err := ResolveSources(nixos)
	if err != nil {
		t.Fatal(err)
	}
	if src.NixOS != nixos || src.Dots != dots || src.Installer != installer {
		t.Fatalf("expected in-tree dots for installed mirror, got: %#v", src)
	}
}

func TestResolveSourcesExplicitRepoDoesNotFallback(t *testing.T) {
	root := t.TempDir()
	valid := filepath.Join(root, "valid")
	mkdir(t, filepath.Join(valid, "Linux", "NixOS"))
	mkdir(t, filepath.Join(valid, "Linux", "dots"))
	mkdir(t, filepath.Join(valid, "Linux", "installer"))
	write(t, filepath.Join(valid, "Linux", "NixOS", "flake.nix"))
	t.Setenv("MYSETUP_REPO_ROOT", valid)

	if _, err := ResolveSources(filepath.Join(root, "missing")); err == nil {
		t.Fatal("explicit invalid repo should fail instead of falling back")
	}
}

func TestXDGStateHomeUsesExplicitHomeOverProcessEnv(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))

	home := filepath.Join(t.TempDir(), "managed-user")
	if got, want := XDGStateHome(home), filepath.Join(home, ".local", "state"); got != want {
		t.Fatalf("expected explicit home state path %q, got %q", want, got)
	}
}

func TestDefaultOptionsStillHonorsProcessXDGStateHome(t *testing.T) {
	stateHome := filepath.Join(t.TempDir(), "state")
	t.Setenv("XDG_STATE_HOME", stateHome)

	opts := DefaultOptions()
	if got, want := opts.DraftPath, filepath.Join(stateHome, "mysetup", "draft.json"); got != want {
		t.Fatalf("expected process XDG state home in default options %q, got %q", want, got)
	}
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
