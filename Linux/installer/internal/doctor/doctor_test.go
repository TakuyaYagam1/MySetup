package doctor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/shellruntime"
)

func TestVariablesWallpaperEnable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "host-vars.nix")
	if err := os.WriteFile(path, []byte(`{
  wallpapers = {
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
  wallpapers = {
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
	path := filepath.Join(dir, "host-vars.nix")
	if err := os.WriteFile(path, []byte(`{ }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := variablesWallpaperEnable(path); got != nil {
		t.Fatalf("expected missing wallpapers flag, got %#v", got)
	}
}

func TestHostVarsPathFallsBackToLegacyLayout(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "hosts", "NixOS", "host-vars.nix")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := hostVarsPath(dir); got != legacy {
		t.Fatalf("expected legacy host-vars path, got %q", got)
	}

	root := filepath.Join(dir, "host-vars.nix")
	if err := os.WriteFile(root, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := hostVarsPath(dir); got != root {
		t.Fatalf("expected root host-vars path to win, got %q", got)
	}
}

func TestReportReturnsDoctorOutputWithoutPrinting(t *testing.T) {
	dir := t.TempDir()
	for _, path := range []string{
		"flake.nix",
		"host-vars.nix",
		"hardware-configuration.nix",
		"configuration.nix",
	} {
		fullPath := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.WriteFile(fullPath, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	state := config.Default()
	state.User.HomeDirectory = t.TempDir()
	hyprDir := filepath.Join(state.User.HomeDirectory, ".config/hypr")
	runtimeDir := shellruntime.RuntimeDir(state.User.HomeDirectory)
	if err := os.MkdirAll(filepath.Dir(paths.ActiveShellStatePath(state.User.HomeDirectory)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(hyprDir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(hyprDir, "wahrwelt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.ActiveShellStatePath(state.User.HomeDirectory), []byte("caelestia\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hyprDir, "shell-profile.lua"), []byte(`dofile("`+filepath.Join(runtimeDir, "shell-profile.lua")+`")`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hyprDir, "hyprland.lua"), []byte(`dofile("`+filepath.Join(runtimeDir, "hyprland.lua")+`")`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, "shell-profile.lua"), []byte(`hl.on("hyprland.start", function() hl.exec_cmd("/home/user/.config/hypr/scripts/start-shell.sh") end)`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, "hyprland.lua"), []byte(`dofile("/home/user/.config/hypr/wahrwelt/hyprland.lua")`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hyprDir, "wahrwelt", "hyprland.lua"), []byte("hl.monitor({ output = \"eDP-1\", mode = \"2560x1600@120\", position = \"0x0\", scale = \"1\" })\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, "shell-keybinds.lua"), []byte(`require("caelestia.keybinds")`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(state.User.HomeDirectory, ".config/quickshell", "wahrwelt-shell-selector"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state.User.HomeDirectory, ".config/quickshell", "wahrwelt-shell-selector", "shell.qml"), []byte("ShellRoot {}\n"), 0o644); err != nil {
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
			StatePath: filepath.Join(dir, "wahrwelt/state.json"),
		},
		State: state,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"== Wahrwelt doctor ==",
		"OK   flake:",
		"OK   shell state:",
		"OK   shell entrypoint:",
		"OK   shell keybinds:",
		"OK   shell selector config:",
		"OK   hypr script executable:",
		"Last-resort system rollback:",
		"User dotfiles are not fully transactional",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("expected report to contain %q, got:\n%s", want, report)
		}
	}
}

func TestReportUsesEnd4ProfileChecks(t *testing.T) {
	dir := t.TempDir()
	for _, path := range []string{
		"flake.nix",
		"host-vars.nix",
		"hardware-configuration.nix",
		"configuration.nix",
	} {
		fullPath := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.WriteFile(fullPath, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	state := config.Default()
	state.User.HomeDirectory = t.TempDir()
	runtimeDir := shellruntime.RuntimeDir(state.User.HomeDirectory)
	if err := os.MkdirAll(filepath.Dir(paths.ActiveShellStatePath(state.User.HomeDirectory)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.ActiveShellStatePath(state.User.HomeDirectory), []byte("end4\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for path, content := range map[string]string{
		filepath.Join(state.User.HomeDirectory, ".config/hypr/shell-profile.lua"):                                `dofile("` + filepath.Join(runtimeDir, "shell-profile.lua") + "\")\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/hyprland.lua"):                                     `dofile("` + filepath.Join(runtimeDir, "hyprland.lua") + "\")\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/hyprlock.conf"):                                    "source=" + filepath.Join(runtimeDir, "hyprlock.conf") + "\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/hypridle.conf"):                                    "source=" + filepath.Join(runtimeDir, "hypridle.conf") + "\n",
		filepath.Join(runtimeDir, "shell-profile.lua"):                                                           `hl.on("hyprland.start", function() hl.exec_cmd("/home/user/.config/hypr/scripts/start-shell.sh") end)` + "\n",
		filepath.Join(runtimeDir, "hyprland.lua"):                                                                "dofile(os.getenv(\"HOME\") .. \"/.config/hypr/end4/hyprland.lua\")\n",
		filepath.Join(runtimeDir, "hyprlock.conf"):                                                               "source=~/.config/hypr/end4/hyprlock.conf\n",
		filepath.Join(runtimeDir, "hypridle.conf"):                                                               "source=~/.config/hypr/end4/hypridle.conf\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/end4/hyprland.lua"):                                "require(\"hyprland.env\")\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/end4/wahrwelt/keybinds.lua"):                       "wahrwelt.bind_exec(\"SUPER + R\", \"~/.config/hypr/scripts/record-toggle.sh\")\n",
		filepath.Join(state.User.HomeDirectory, ".config/quickshell/ii/shell.qml"):                               "ShellRoot {}\n",
		filepath.Join(state.User.HomeDirectory, ".config/quickshell/wahrwelt-shell-selector/shell.qml"):          "ShellRoot {}\n",
		filepath.Join(state.User.HomeDirectory, ".config/illogical-impulse/config.json"):                         "{}\n",
		filepath.Join(state.User.HomeDirectory, ".config/kdeglobals"):                                            "[General]\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/end4/hyprland/scripts/launch_first_available.sh"):  "#!/bin/sh\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/end4/hyprland/scripts/start_geoclue_agent.sh"):     "#!/bin/sh\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/end4/custom/scripts/__restore_video_wallpaper.sh"): "#!/bin/sh\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/scripts/start-shell.sh"):                           "#!/bin/sh\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/scripts/close-active.sh"):                          "#!/bin/sh\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/scripts/noctalia-launcher.sh"):                     "#!/bin/sh\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/scripts/record-toggle.sh"):                         "#!/bin/sh\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/scripts/shell-end4-overrides.sh"):                  "#!/bin/sh\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/scripts/shell-process.sh"):                         "#!/bin/sh\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/scripts/shell-profile-sync.sh"):                    "#!/bin/sh\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/scripts/shell-runtime-env.sh"):                     "#!/bin/sh\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/scripts/shell-selector.sh"):                        "#!/bin/sh\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/scripts/shell-runtime.sh"):                         "#!/bin/sh\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/scripts/screenshot.sh"):                            "#!/bin/sh\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/scripts/spotify-toggle.sh"):                        "#!/bin/sh\n",
		filepath.Join(state.User.HomeDirectory, ".config/hypr/scripts/wsaction.fish"):                            "#!/bin/sh\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(path, ".sh") || strings.HasSuffix(path, ".fish") {
			mode = 0o755
		}
		if err := os.WriteFile(path, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}

	report, err := Report(context.Background(), Options{
		Paths: paths.Options{
			NixOSDest: dir,
			StatePath: filepath.Join(dir, "wahrwelt/state.json"),
		},
		State: state,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"OK   shell state:",
		"OK   shell launcher:",
		"OK   shell entrypoint:",
		"OK   end4 hyprlock entrypoint:",
		"OK   end4 hypridle entrypoint:",
		"OK   end4 hypr config:",
		"OK   end4 wahrwelt keybinds:",
		"OK   end4 quickshell shell:",
		"OK   end4 runtime config dir:",
		"OK   end4 runtime config:",
		"OK   end4 kdeglobals writable:",
		"OK   end4 script executable:",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("expected end4 report to contain %q, got:\n%s", want, report)
		}
	}
	if strings.Contains(report, "shell keybinds") {
		t.Fatalf("end4 doctor path must not run shell-keybind checks, got:\n%s", report)
	}
}

func TestCheckShellKeybindsWarnsOnProfileMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shell-keybinds.lua")
	if err := os.WriteFile(path, []byte(`require("caelestia.keybinds")`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := &reportWriter{}

	checkShellKeybinds(out, path, "noctalia")

	report := out.String()
	if !strings.Contains(report, "WARN shell keybinds do not source current profile") {
		t.Fatalf("expected profile mismatch warning, got:\n%s", report)
	}
}

func TestDetectActiveShellUsesRememberedEnd4PCVariant(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	home := t.TempDir()
	runtimeDir := shellruntime.RuntimeDir(home)
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, "hyprland.lua"), []byte(`dofile("/home/user/.config/hypr/end4/hyprland.lua")`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	variantPath := filepath.Join(home, ".local", "state", "wahrwelt", "end4-variant")
	if err := os.WriteFile(variantPath, []byte("end4-pc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := detectActiveShell(home); got != "end4-pc" {
		t.Fatalf("expected remembered end4-pc profile, got %q", got)
	}
}

func TestCheckShellKeybindsAcceptsHomeManagerSymlinkContent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "store-keybinds.lua")
	path := filepath.Join(dir, "shell-keybinds.lua")
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
