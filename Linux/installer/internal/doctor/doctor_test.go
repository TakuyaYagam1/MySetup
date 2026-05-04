package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVariablesWallpaperEnable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "variables.nix")
	if err := os.WriteFile(path, []byte(`{
  config.var.wallpapers = {
    enable = false;
  };
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got := variablesWallpaperEnable(path)
	if got == nil || *got {
		t.Fatalf("expected wallpapers flag false, got %#v", got)
	}

	if err := os.WriteFile(path, []byte(`{
  config.var.wallpapers = {
    enable = true;
  };
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got = variablesWallpaperEnable(path)
	if got == nil || !*got {
		t.Fatalf("expected wallpapers flag true, got %#v", got)
	}
}

func TestVariablesWallpaperEnableMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "variables.nix")
	if err := os.WriteFile(path, []byte(`{ config.var = {}; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := variablesWallpaperEnable(path); got != nil {
		t.Fatalf("expected missing wallpapers flag, got %#v", got)
	}
}
