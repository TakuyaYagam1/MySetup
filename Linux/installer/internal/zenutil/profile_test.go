package zenutil

import (
	"os"
	"path/filepath"
	"testing"
)

func mkdirs(t *testing.T, paths ...string) {
	t.Helper()
	for _, p := range paths {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", p, err)
		}
	}
}

func TestFindProfileReturnsEmptyWhenHomeMissing(t *testing.T) {
	home := filepath.Join(t.TempDir(), "ghost")
	if got := FindProfile(home); got != "" {
		t.Fatalf("missing home should yield empty profile, got %q", got)
	}
}

func TestFindProfileReturnsEmptyWhenZenDirsEmpty(t *testing.T) {
	home := t.TempDir()
	mkdirs(t, filepath.Join(home, ".zen"))
	if got := FindProfile(home); got != "" {
		t.Fatalf("empty .zen dir should yield empty profile, got %q", got)
	}
}

func TestFindProfilePrefersDefaultName(t *testing.T) {
	home := t.TempDir()
	zen := filepath.Join(home, ".zen")
	other := filepath.Join(zen, "abc12345.user")
	preferred := filepath.Join(zen, "default-release-xyz")
	mkdirs(t, other, preferred)

	got := FindProfile(home)
	if got != preferred {
		t.Fatalf("expected default profile, got %q", got)
	}
}

func TestFindProfileFallsBackToFirstDir(t *testing.T) {
	home := t.TempDir()
	zen := filepath.Join(home, ".zen")
	candidate := filepath.Join(zen, "abc12345.user")
	mkdirs(t, candidate)

	if got := FindProfile(home); got != candidate {
		t.Fatalf("expected fallback profile %q, got %q", candidate, got)
	}
}

func TestFindProfileSkipsRegularFiles(t *testing.T) {
	home := t.TempDir()
	zen := filepath.Join(home, ".zen")
	mkdirs(t, zen)
	if err := os.WriteFile(filepath.Join(zen, "profiles.ini"), []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := FindProfile(home); got != "" {
		t.Fatalf("regular files should be ignored, got %q", got)
	}
}

func TestFindProfileFallsBackToConfigZen(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, ".config", "zen")
	candidate := filepath.Join(cfg, "default-release-xyz")
	mkdirs(t, candidate)

	if got := FindProfile(home); got != candidate {
		t.Fatalf("expected ~/.config/zen profile %q, got %q", candidate, got)
	}
}

func TestFindProfileCaseInsensitiveDefault(t *testing.T) {
	home := t.TempDir()
	zen := filepath.Join(home, ".zen")
	other := filepath.Join(zen, "aaa.user")
	preferred := filepath.Join(zen, "Default-Release-xyz")
	mkdirs(t, other, preferred)

	if got := FindProfile(home); got != preferred {
		t.Fatalf("expected case-insensitive default match, got %q", got)
	}
}
