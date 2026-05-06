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
