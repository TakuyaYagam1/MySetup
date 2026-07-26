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

func TestResolveSourcesPrefersWahrweltEnvironmentOverLegacyMySetup(t *testing.T) {
	root := t.TempDir()
	wahrwelt := filepath.Join(root, "wahrwelt")
	legacy := filepath.Join(root, "mysetup")
	for _, source := range []string{wahrwelt, legacy} {
		mkdir(t, filepath.Join(source, "Linux", "NixOS"))
		mkdir(t, filepath.Join(source, "Linux", "dots"))
		mkdir(t, filepath.Join(source, "Linux", "installer"))
		write(t, filepath.Join(source, "Linux", "NixOS", "flake.nix"))
	}
	t.Setenv("WAHRWELT_REPO_ROOT", wahrwelt)
	t.Setenv("MYSETUP_REPO_ROOT", legacy)

	src, err := ResolveSources("")
	if err != nil {
		t.Fatal(err)
	}
	if src.RepoRoot != wahrwelt {
		t.Fatalf("WAHRWELT_REPO_ROOT must take precedence: got %q, want %q", src.RepoRoot, wahrwelt)
	}
}

func TestResolveSourcesFallsBackToLegacyMySetupEnvironment(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "mysetup")
	mkdir(t, filepath.Join(legacy, "Linux", "NixOS"))
	mkdir(t, filepath.Join(legacy, "Linux", "dots"))
	mkdir(t, filepath.Join(legacy, "Linux", "installer"))
	write(t, filepath.Join(legacy, "Linux", "NixOS", "flake.nix"))
	t.Setenv("WAHRWELT_REPO_ROOT", "")
	t.Setenv("MYSETUP_REPO_ROOT", legacy)

	src, err := ResolveSources("")
	if err != nil {
		t.Fatal(err)
	}
	if src.RepoRoot != legacy {
		t.Fatalf("legacy MYSETUP_REPO_ROOT should remain supported: got %q, want %q", src.RepoRoot, legacy)
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
	if got, want := opts.DraftPath, filepath.Join(stateHome, "wahrwelt", "draft.json"); got != want {
		t.Fatalf("expected process XDG state home in default options %q, got %q", want, got)
	}
}

func TestDefaultOptionsUseCanonicalWahrweltPaths(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	opts := DefaultOptions()
	if opts.StatePath != DefaultStatePath {
		t.Fatalf("expected canonical state path %q, got %q", DefaultStatePath, opts.StatePath)
	}
	if got, want := ActiveShellStatePath("/home/tester"), "/home/tester/.local/state/wahrwelt/active-shell"; got != want {
		t.Fatalf("expected canonical active shell path %q, got %q", want, got)
	}
}

func TestExistingStatePathFallsBackToLegacyDefault(t *testing.T) {
	dir := t.TempDir()
	canonical := filepath.Join(dir, "wahrwelt", "state.json")
	legacy := filepath.Join(dir, "mysetup", "state.json")
	mkdir(t, filepath.Dir(legacy))
	write(t, legacy)

	opts := Options{StatePath: canonical}
	if got := ExistingFile(opts.StatePath, legacy); got != legacy {
		t.Fatalf("expected legacy state fallback %q, got %q", legacy, got)
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
