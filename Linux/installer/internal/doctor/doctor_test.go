package doctor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
	migrationv1tov2 "github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/migrations/v1_to_v2"
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
		"installer-state.json",
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
	if err := os.MkdirAll(filepath.Join(hyprDir, "user"), 0o755); err != nil {
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
	if err := os.WriteFile(filepath.Join(runtimeDir, "hyprland.lua"), []byte(shellruntime.CanonicalEntrypoint()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hyprDir, "user", "hyprland.lua"), []byte("hl.monitor({ output = \"eDP-1\", mode = \"2560x1600@120\", position = \"0x0\", scale = \"1\" })\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, "shell-launcher.lua"), []byte(`require("caelestia.launcher")`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDir, "shell-keybinds.lua"), []byte(shellruntime.AdapterMarker(shellruntime.Caelestia)+"\n"+`require("caelestia.keybinds")`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(state.User.HomeDirectory, ".config/quickshell", "wahrwelt-shell-selector"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state.User.HomeDirectory, ".config/quickshell", "wahrwelt-shell-selector", "shell.qml"), []byte("ShellRoot {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, script := range requiredHyprScripts() {
		target := filepath.Join(hyprDir, "scripts", script)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	installedScripts := filepath.Join(dir, "dots/hypr/scripts")
	if err := os.MkdirAll(installedScripts, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, script := range requiredHyprScripts() {
		target := filepath.Join(installedScripts, script)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	report, err := Report(context.Background(), Options{
		Paths: paths.Options{
			NixOSDest: dir,
			StatePath: filepath.Join(dir, "installer-state.json"),
		},
		State: state,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"== Wahrwelt doctor ==",
		"OK   state:",
		"OK   flake:",
		"OK   shell state:",
		"OK   shell entrypoint:",
		"OK   shell launcher bindings:",
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
		"installer-state.json",
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
		filepath.Join(runtimeDir, "hyprland.lua"):                                                                shellruntime.CanonicalEntrypoint(),
		filepath.Join(runtimeDir, "shell-launcher.lua"):                                                          "require(\"end4.launcher\")\n",
		filepath.Join(runtimeDir, "shell-keybinds.lua"):                                                          shellruntime.AdapterMarker(shellruntime.End4) + "\nrequire(\"end4-adapter\").load({ profile = \"end4\", quickshell_config = \"/home/user/.config/quickshell/ii\" })\n",
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
		filepath.Join(state.User.HomeDirectory, ".config/hypr/scripts/restore-lock.sh"):                          "#!/bin/sh\n",
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
			StatePath: filepath.Join(dir, "installer-state.json"),
		},
		State: state,
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"OK   state:",
		"OK   shell state:",
		"OK   shell launcher:",
		"OK   shell entrypoint:",
		"OK   shell launcher bindings:",
		"OK   shell keybinds:",
		"WARN user hypr config missing:",
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
}

func TestCheckShellRuntimeChecksRequiredUserHyprConfigForEveryProfile(t *testing.T) {
	profiles := []string{
		shellruntime.Caelestia,
		shellruntime.Noctalia,
		shellruntime.End4,
		shellruntime.End4PC,
	}
	for _, profile := range profiles {
		for _, present := range []bool{false, true} {
			name := "missing"
			if present {
				name = "present"
			}
			t.Run(profile+"/"+name, func(t *testing.T) {
				state := config.Default()
				state.User.HomeDirectory = t.TempDir()
				statePath := shellruntime.ActiveShellStatePath(state.User.HomeDirectory)
				if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(statePath, []byte(profile+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}

				userHypr := hyprConfigPath(state.User.HomeDirectory, "user/hyprland.lua")
				want := "WARN user hypr config missing: " + userHypr
				if present {
					if err := os.MkdirAll(filepath.Dir(userHypr), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(userHypr, []byte("-- managed user runtime\n"), 0o644); err != nil {
						t.Fatal(err)
					}
					want = "OK   user hypr config: " + userHypr
				}

				out := &reportWriter{}
				checkShellRuntime(out, Options{State: state})
				report := out.String()
				if !strings.Contains(report, want) {
					t.Fatalf("profile %q did not report required user Hypr config:\n%s", profile, report)
				}
				if got := strings.Count(report, "user hypr config"); got != 1 {
					t.Fatalf("profile %q checked required user Hypr config %d times, want 1:\n%s", profile, got, report)
				}
			})
		}
	}
}

func TestCheckShellRuntimeRejectsUserHyprDirectoryForEveryProfile(t *testing.T) {
	for _, profile := range []string{
		shellruntime.Caelestia,
		shellruntime.Noctalia,
		shellruntime.End4,
		shellruntime.End4PC,
	} {
		t.Run(profile, func(t *testing.T) {
			state := config.Default()
			state.User.HomeDirectory = t.TempDir()
			statePath := shellruntime.ActiveShellStatePath(state.User.HomeDirectory)
			if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(statePath, []byte(profile+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			userHypr := hyprConfigPath(state.User.HomeDirectory, "user/hyprland.lua")
			if err := os.MkdirAll(userHypr, 0o755); err != nil {
				t.Fatal(err)
			}

			out := &reportWriter{}
			checkShellRuntime(out, Options{State: state})
			want := "WARN user hypr config is not a regular file: " + userHypr
			if report := out.String(); !strings.Contains(report, want) {
				t.Fatalf("profile %q false-green for user Hypr directory:\n%s", profile, report)
			}
		})
	}
}

func TestCheckShellKeybindsWarnsOnProfileMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shell-keybinds.lua")
	if err := os.WriteFile(path, []byte(shellruntime.AdapterMarker(shellruntime.Caelestia)+"\n"+`require("caelestia.keybinds")`+"\n"), 0o644); err != nil {
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
	if err := os.WriteFile(filepath.Join(runtimeDir, "hyprland.lua"), doctorLegacyEnd4Fixture(t, shellruntime.End4), 0o644); err != nil {
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

func TestCheckShellEntrypointWarnsForLegacyDirectEnd4Migration(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "hyprland.lua")
	if err := os.WriteFile(path, doctorLegacyEnd4Fixture(t, shellruntime.End4), 0o644); err != nil {
		t.Fatal(err)
	}
	out := &reportWriter{}

	checkShellEntrypoint(out, path, shellruntime.End4)

	if report := out.String(); !strings.Contains(report, "WARN shell entrypoint uses legacy direct End4 runtime and requires migration") {
		t.Fatalf("expected legacy migration warning, got:\n%s", report)
	}
}

func TestCheckShellEntrypointWarnsForLegacyUserNamespaceMigration(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "hyprland.lua")
	legacy := migrationv1tov2.LegacyUserEntrypoint()
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	out := &reportWriter{}

	checkShellEntrypoint(out, path, shellruntime.End4PC)

	if report := out.String(); !strings.Contains(report, "WARN shell entrypoint uses legacy Wahrwelt user namespace and requires migration") {
		t.Fatalf("expected legacy user namespace migration warning, got:\n%s", report)
	}
}

func TestCheckShellEntrypointWarnsForUserNamespaceTransition(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hyprland.lua")
	if err := os.WriteFile(path, []byte(migrationv1tov2.UserNamespaceTransitionEntrypoint()), 0o644); err != nil {
		t.Fatal(err)
	}
	out := &reportWriter{}

	checkShellEntrypoint(out, path, shellruntime.Noctalia)

	report := out.String()
	if !strings.Contains(report, "WARN shell entrypoint uses temporary user namespace transition and requires migration") {
		t.Fatalf("expected user namespace transition warning, got:\n%s", report)
	}
	if strings.Contains(report, "OK   shell entrypoint") {
		t.Fatalf("transition runtime must not be reported as canonical:\n%s", report)
	}
}

func TestCheckShellEntrypointWarnsForHistoricalHomeManagerSeededRuntime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hyprland.lua")
	payload := migrationv1tov2.HistoricalHomeManagerSeededUserEntrypoint(
		shellruntime.DefaultProfile,
		migrationv1tov2.LegacyWahrweltNamespace,
	)
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}
	out := &reportWriter{}

	checkShellEntrypoint(out, path, shellruntime.Caelestia)

	if report := out.String(); !strings.Contains(report, "WARN shell entrypoint uses legacy Home Manager seeded runtime and requires migration") {
		t.Fatalf("expected historical seeded runtime warning, got:\n%s", report)
	}
}

func TestCheckShellEntrypointDoesNotTreatCustomEnd4SuffixAsLegacy(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "hyprland.lua")
	if err := os.WriteFile(path, []byte(`dofile("/tmp/custom/end4/hyprland.lua")`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := &reportWriter{}

	checkShellEntrypoint(out, path, shellruntime.End4)

	report := out.String()
	if strings.Contains(report, "legacy direct End4") {
		t.Fatalf("custom suffix-only entrypoint must not receive migration warning:\n%s", report)
	}
	if !strings.Contains(report, "is not the canonical Wahrwelt runtime") {
		t.Fatalf("custom suffix-only entrypoint should be reported as unknown:\n%s", report)
	}
}

func TestCheckShellEntrypointRejectsCanonicalLookalikes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hyprland.lua")
	for name, content := range map[string]string{
		"arbitrary suffix": `dofile("/tmp/arbitrary/wahrwelt/hyprland.lua")` + "\n",
		"extra content":    shellruntime.CanonicalEntrypoint() + "-- user suffix\n",
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			out := &reportWriter{}
			checkShellEntrypoint(out, path, shellruntime.Caelestia)
			if report := out.String(); !strings.Contains(report, "is not the canonical Wahrwelt runtime") {
				t.Fatalf("canonical lookalike was accepted:\n%s", report)
			}
		})
	}
}

func doctorLegacyEnd4Fixture(t *testing.T, profile string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("../../../NixOS/home/migrations/v1_to_v2/hypr-runtime", profile+".lua"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCheckShellKeybindsAcceptsHomeManagerSymlinkContent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "store-keybinds.lua")
	path := filepath.Join(dir, "shell-keybinds.lua")
	if err := os.WriteFile(target, []byte(shellruntime.AdapterMarker(shellruntime.Noctalia)+"\n"+`require("noctalia.keybinds")`+"\n"), 0o644); err != nil {
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
