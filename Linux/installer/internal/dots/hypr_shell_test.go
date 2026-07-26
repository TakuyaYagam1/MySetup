package dots

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/shellruntime"
)

func TestWriteShellLauncherConfigWritesRuntimeLauncher(t *testing.T) {
	dir := t.TempDir()
	hyprDir := filepath.Join(dir, ".config", "hypr")
	path := filepath.Join(shellruntime.RuntimeDir(dir), "shell-profile.lua")

	if err := writeShellLauncherConfig(path, hyprDir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"-- Runtime shell launcher",
		`hl.exec_cmd("` + filepath.Join(hyprDir, "scripts", "start-shell.sh") + `")`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("shell launcher missing %q\n%s", want, got)
		}
	}
}

func TestWriteShellLauncherConfigReplacesExistingSymlink(t *testing.T) {
	dir := t.TempDir()
	hyprDir := filepath.Join(dir, ".config", "hypr")
	path := filepath.Join(shellruntime.RuntimeDir(dir), "shell-profile.lua")
	target := filepath.Join(dir, "store-shell-profile.lua")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("old target\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}

	if err := writeShellLauncherConfig(path, hyprDir); err != nil {
		t.Fatal(err)
	}

	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("shell launcher should be rewritten as a regular file, got symlink %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "start-shell.sh") || strings.Contains(string(data), "start-shell.sh noctalia") {
		t.Fatalf("shell launcher should contain runtime launcher without hardcoded profile\n%s", string(data))
	}
	targetData, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(targetData) != "old target\n" {
		t.Fatalf("symlink target should not be overwritten, got %q", string(targetData))
	}
}

func TestWriteShellKeybindsConfigUsesCurrentShell(t *testing.T) {
	dir := t.TempDir()
	hyprDir := filepath.Join(dir, ".config", "hypr")
	path := filepath.Join(shellruntime.RuntimeDir(dir), "shell-keybinds.lua")
	noctaliaKeybinds := filepath.Join(hyprDir, "noctalia", "keybinds.lua")
	if err := os.MkdirAll(filepath.Dir(noctaliaKeybinds), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(noctaliaKeybinds, []byte("# noctalia binds\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeShellKeybindsConfig(path, hyprDir, "noctalia"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"-- Active shell keybind profile: noctalia",
		`require("noctalia.keybinds")`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("shell keybind layer missing %q\n%s", want, got)
		}
	}
}

func TestWriteShellKeybindsConfigReplacesExistingSymlink(t *testing.T) {
	dir := t.TempDir()
	hyprDir := filepath.Join(dir, ".config", "hypr")
	path := filepath.Join(shellruntime.RuntimeDir(dir), "shell-keybinds.lua")
	caelestiaKeybinds := filepath.Join(hyprDir, "caelestia", "keybinds.lua")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(caelestiaKeybinds), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caelestiaKeybinds, []byte("# caelestia binds\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "store-shell-keybinds.lua")
	if err := os.WriteFile(target, []byte("old target\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}

	if err := writeShellKeybindsConfig(path, hyprDir, "caelestia"); err != nil {
		t.Fatal(err)
	}

	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("shell keybinds should be rewritten as a regular file, got symlink %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `require("caelestia.keybinds")`) {
		t.Fatalf("shell keybind config should source default profile\n%s", string(data))
	}
}

func TestWriteShellKeybindsConfigErrorsWhenProfileMissing(t *testing.T) {
	dir := t.TempDir()
	hyprDir := filepath.Join(dir, ".config", "hypr")

	err := writeShellKeybindsConfig(filepath.Join(shellruntime.RuntimeDir(dir), "shell-keybinds.lua"), hyprDir, "noctalia")
	if err == nil {
		t.Fatal("expected missing shell keybind layer error")
	}
	if !strings.Contains(err.Error(), "shell keybind profile missing") {
		t.Fatalf("expected missing shell keybind layer error, got %v", err)
	}
}

func TestWriteHyprRuntimeShellStateSeedsLegacyRuntimeFiles(t *testing.T) {
	home := t.TempDir()
	hyprDir := filepath.Join(home, ".config", "hypr")
	runtimeDir := shellruntime.RuntimeDir(home)
	if err := os.MkdirAll(filepath.Join(hyprDir, "caelestia"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(hyprDir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(hyprDir, "wahrwelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hyprDir, "caelestia", "keybinds.lua"), []byte("-- binds\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hyprDir, "caelestia", "launcher.lua"), []byte("-- launcher\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hyprDir, "wahrwelt", "hyprland.lua"), []byte("hl.monitor({ output = \"eDP-1\", mode = \"2560x1600@120\", position = \"0x0\", scale = \"1\" })\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeHyprRuntimeShellState(home, hyprDir); err != nil {
		t.Fatal(err)
	}

	activeShell, err := os.ReadFile(paths.ActiveShellStatePath(home))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(activeShell)) != "caelestia" {
		t.Fatalf("expected active shell caelestia, got %q", string(activeShell))
	}

	stableEntrypoint, err := os.ReadFile(filepath.Join(hyprDir, "hyprland.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stableEntrypoint), filepath.Join(runtimeDir, "hyprland.lua")) {
		t.Fatalf("expected stable entrypoint to source runtime config\n%s", string(stableEntrypoint))
	}

	entrypoint, err := os.ReadFile(filepath.Join(runtimeDir, "hyprland.lua"))
	if err != nil {
		t.Fatal(err)
	}
	entrypointText := string(entrypoint)
	if !strings.Contains(entrypointText, "wahrwelt/hyprland.lua") {
		t.Fatalf("expected legacy entrypoint to source wahrwelt config\n%s", entrypointText)
	}
	if !strings.Contains(entrypointText, shellruntime.RuntimeFile(home, "shell-profile.lua")) {
		t.Fatalf("expected legacy entrypoint to source runtime shell profile\n%s", entrypointText)
	}
	if strings.Contains(entrypointText, filepath.Join(hyprDir, "runtime", "shell-profile.lua")) {
		t.Fatalf("legacy entrypoint should not source old hypr runtime path\n%s", entrypointText)
	}

	launcher, err := os.ReadFile(filepath.Join(runtimeDir, "shell-launcher.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(launcher), `require("caelestia.launcher")`) {
		t.Fatalf("expected launcher runtime config to source caelestia launcher layer\n%s", string(launcher))
	}

	hyprlock, err := os.ReadFile(filepath.Join(runtimeDir, "hyprlock.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(hyprlock), "shell-managed (caelestia)") {
		t.Fatalf("expected legacy hyprlock placeholder to mention caelestia\n%s", string(hyprlock))
	}

	hypridle, err := os.ReadFile(filepath.Join(runtimeDir, "hypridle.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(hypridle), "shell-managed (caelestia)") {
		t.Fatalf("expected legacy hypridle placeholder to mention caelestia\n%s", string(hypridle))
	}
}

func TestWriteHyprRuntimeShellStateBootstrapsEnd4BeforeProfileExists(t *testing.T) {
	home := t.TempDir()
	hyprDir := filepath.Join(home, ".config", "hypr")
	runtimeDir := shellruntime.RuntimeDir(home)
	if err := os.MkdirAll(filepath.Join(hyprDir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(hyprDir, "end4"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.ActiveShellStatePath(home)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.ActiveShellStatePath(home), []byte("end4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeHyprRuntimeShellState(home, hyprDir); err != nil {
		t.Fatal(err)
	}

	activeShell, err := os.ReadFile(paths.ActiveShellStatePath(home))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(activeShell)) != "end4" {
		t.Fatalf("expected active shell end4, got %q", string(activeShell))
	}

	hyprland, err := os.ReadFile(filepath.Join(runtimeDir, "hyprland.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(hyprland), `dofile(end4_root .. "/hyprland.lua")`) {
		t.Fatalf("expected end4 runtime entrypoint to point at profile installed by Home Manager\n%s", string(hyprland))
	}
	hyprlock, err := os.ReadFile(filepath.Join(runtimeDir, "hyprlock.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(hyprlock), filepath.Join(hyprDir, "end4", "hyprlock.conf")) {
		t.Fatalf("expected end4 hyprlock runtime source\n%s", string(hyprlock))
	}
	shellLauncher, err := os.ReadFile(filepath.Join(runtimeDir, "shell-launcher.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(shellLauncher), "Active shell launcher profile: end4") {
		t.Fatalf("expected end4 shell launcher placeholder\n%s", string(shellLauncher))
	}
	shellKeybinds, err := os.ReadFile(filepath.Join(runtimeDir, "shell-keybinds.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(shellKeybinds), "Active shell keybind profile: end4") {
		t.Fatalf("expected end4 shell keybind placeholder\n%s", string(shellKeybinds))
	}
	stableEntrypoint, err := os.ReadFile(filepath.Join(hyprDir, "hyprland.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stableEntrypoint), filepath.Join(runtimeDir, "hyprland.lua")) {
		t.Fatalf("expected stable entrypoint to keep pointing at runtime config\n%s", string(stableEntrypoint))
	}
}

func TestWriteHyprRuntimeShellStateDetectsNoctaliaFromEntrypointWhenStateMissing(t *testing.T) {
	home := t.TempDir()
	hyprDir := filepath.Join(home, ".config", "hypr")
	if err := os.MkdirAll(filepath.Join(hyprDir, "wahrwelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(hyprDir, "noctalia"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hyprDir, "hyprland.lua"), []byte(`dofile("`+filepath.Join(hyprDir, "wahrwelt", "hyprland.lua")+`")`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hyprDir, "shell-keybinds.lua"), []byte(`require("noctalia.keybinds")`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hyprDir, "wahrwelt", "hyprland.lua"), []byte("hl.monitor({ output = \"eDP-1\", mode = \"2560x1600@120\", position = \"0x0\", scale = \"1\" })\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hyprDir, "noctalia", "keybinds.lua"), []byte("-- noctalia binds\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hyprDir, "noctalia", "launcher.lua"), []byte("-- noctalia launcher\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeHyprRuntimeShellState(home, hyprDir); err != nil {
		t.Fatal(err)
	}

	activeShell, err := os.ReadFile(paths.ActiveShellStatePath(home))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(activeShell)) != "noctalia" {
		t.Fatalf("expected detected active shell noctalia, got %q", string(activeShell))
	}
}

func TestWriteHyprRuntimeShellStateSeedsEnd4RuntimeFilesWhenProfileExists(t *testing.T) {
	home := t.TempDir()
	hyprDir := filepath.Join(home, ".config", "hypr")
	runtimeDir := shellruntime.RuntimeDir(home)
	if err := os.MkdirAll(filepath.Join(hyprDir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(hyprDir, "end4"), 0o755); err != nil {
		t.Fatal(err)
	}
	for rel := range map[string]string{
		"end4/hyprland.lua":  "require(\"hyprland.env\")\n",
		"end4/hyprlock.conf": "background {}\n",
		"end4/hypridle.conf": "general {}\n",
	} {
		path := filepath.Join(hyprDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("placeholder\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(paths.ActiveShellStatePath(home)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.ActiveShellStatePath(home), []byte("end4\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeHyprRuntimeShellState(home, hyprDir); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		path string
		want string
	}{
		{path: filepath.Join(runtimeDir, "hyprland.lua"), want: `dofile(end4_root .. "/hyprland.lua")`},
		{path: filepath.Join(runtimeDir, "hyprlock.conf"), want: "source = " + filepath.Join(hyprDir, "end4", "hyprlock.conf")},
		{path: filepath.Join(runtimeDir, "hypridle.conf"), want: "source = " + filepath.Join(hyprDir, "end4", "hypridle.conf")},
		{path: filepath.Join(runtimeDir, "shell-launcher.lua"), want: "Active shell launcher profile: end4"},
		{path: filepath.Join(runtimeDir, "shell-keybinds.lua"), want: "Active shell keybind profile: end4"},
	} {
		data, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), tc.want) {
			t.Fatalf("expected %s to contain %q\n%s", tc.path, tc.want, string(data))
		}
	}

	entrypoint, err := os.ReadFile(filepath.Join(runtimeDir, "hyprland.lua"))
	if err != nil {
		t.Fatal(err)
	}
	entrypointText := string(entrypoint)
	if strings.Contains(entrypointText, shellruntime.RuntimeFile(home, "shell-profile.lua")) {
		t.Fatalf("end4 entrypoint should not source Wahrwelt shell profile\n%s", entrypointText)
	}
	if strings.Contains(entrypointText, filepath.Join(hyprDir, "runtime", "shell-profile.lua")) {
		t.Fatalf("end4 entrypoint should not source old hypr runtime path\n%s", entrypointText)
	}
}
