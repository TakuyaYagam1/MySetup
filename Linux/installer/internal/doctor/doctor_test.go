package doctor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
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

func TestReportReturnsDoctorOutputWithoutPrinting(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "hosts/NixOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"flake.nix",
		"hosts/NixOS/variables.nix",
		"hosts/NixOS/hardware-configuration.nix",
	} {
		fullPath := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.WriteFile(fullPath, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	state := config.Default()
	state.User.HomeDirectory = t.TempDir()
	hyprDir := filepath.Join(state.User.HomeDirectory, ".config/hypr")
	if err := os.MkdirAll(filepath.Join(hyprDir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hyprDir, "shell-profile.conf"), []byte("exec-once = /home/user/.config/hypr/scripts/start-shell.sh caelestia\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hyprDir, "shell-keybinds.conf"), []byte("source = /home/user/.config/hypr/caelestia/keybinds.conf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, script := range requiredHyprScripts() {
		if err := os.WriteFile(filepath.Join(hyprDir, "scripts", script), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	installedScripts := filepath.Join(dir, "dots/hypr/scripts")
	if err := os.MkdirAll(installedScripts, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, script := range requiredHyprScripts() {
		if err := os.WriteFile(filepath.Join(installedScripts, script), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	report, err := Report(context.Background(), Options{
		Paths: paths.Options{
			NixOSDest: dir,
			StatePath: filepath.Join(dir, "mysetup/state.json"),
		},
		State: state,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"== MySetup doctor ==",
		"OK   flake:",
		"OK   shell keybinds:",
		"OK   hypr script executable:",
		"Last-resort rollback:",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("expected report to contain %q, got:\n%s", want, report)
		}
	}
}

func TestCheckShellKeybindsWarnsOnProfileMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shell-keybinds.conf")
	if err := os.WriteFile(path, []byte("source = /tmp/hypr/caelestia/keybinds.conf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := &reportWriter{}

	checkShellKeybinds(out, path, "noctalia")

	report := out.String()
	if !strings.Contains(report, "WARN shell keybinds do not source current profile") {
		t.Fatalf("expected profile mismatch warning, got:\n%s", report)
	}
}

func TestCheckShellKeybindsAcceptsHomeManagerSymlinkContent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "store-keybinds.conf")
	path := filepath.Join(dir, "shell-keybinds.conf")
	if err := os.WriteFile(target, []byte(`$noctalia = noctalia-shell ipc call
bindi = Super, Super_L, exec, $hypr/scripts/noctalia-launcher.sh press
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	out := &reportWriter{}

	checkShellKeybinds(out, path, "noctalia")

	report := out.String()
	if !strings.Contains(report, "OK   shell keybinds:") {
		t.Fatalf("expected symlink content to identify noctalia profile, got:\n%s", report)
	}
}

func TestCheckExecutableWarnsWhenScriptNotExecutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "script.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := &reportWriter{}

	checkExecutable(out, "hypr script", path)

	report := out.String()
	if !strings.Contains(report, "WARN hypr script not executable") {
		t.Fatalf("expected executable warning, got:\n%s", report)
	}
}
