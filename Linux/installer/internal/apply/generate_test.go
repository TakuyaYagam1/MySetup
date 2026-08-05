package apply

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
)

func TestHostVarsNixContainsFeatureFlags(t *testing.T) {
	state := config.Default()
	state.Host.Hostname = "workstation"
	state.Features.CTFTools = true
	state.Features.Portainer = true
	state.Dots.Wallpapers = true
	state.Display.MonitorMode = "1920x1080@144"

	out, err := HostVarsNix(state)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`host = {`,
		`hostname = "workstation";`,
		`packages = {`,
		`preset = "personal";`,
		`noctalia = {`,
		`version = "v5";`,
		`ctfTools = true;`,
		`portainer = true;`,
		`enable = true;`,
		`nix = {`,
		`maxJobs = 1;`,
		`cores = 2;`,
		`swapSizeMiB = 32 * 1024;`,
		`memoryPercent = 50;`,
		`consoleKeyMap = "us";`,
		`keyboardToggle = "grp:alt_shift_toggle";`,
		`display = {`,
		`monitorName = "eDP-1";`,
		`monitorMode = "1920x1080@144";`,
		`monitorPosition = "0x0";`,
		`monitorScale = "1";`,
		`windowOpacity = "0.85";`,
		`wallpapers = {`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("host-vars.nix missing %q\n%s", want, out)
		}
	}
}

func TestHostVarsNixEmitsEmptyExtraMonitorsByDefault(t *testing.T) {
	out, err := HostVarsNix(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "extraMonitors = [ ];") {
		t.Fatalf("default state must emit empty extraMonitors list\n%s", out)
	}
}

func TestHostVarsNixRendersExtraMonitorsList(t *testing.T) {
	state := config.Default()
	state.Display.ExtraMonitors = []config.Monitor{
		{Name: "HDMI-A-1", Mode: "preferred", Position: "2560x0", Scale: "1"},
		{Name: "DP-2", Mode: "1920x1080@144", Position: "auto", Scale: "1.25"},
	}
	out, err := HostVarsNix(state)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`name = "HDMI-A-1";`,
		`mode = "preferred";`,
		`position = "2560x0";`,
		`scale = "1";`,
		`name = "DP-2";`,
		`mode = "1920x1080@144";`,
		`position = "auto";`,
		`scale = "1.25";`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("host-vars.nix missing %q in extraMonitors list\n%s", want, out)
		}
	}
	if !strings.Contains(out, "extraMonitors = [\n") {
		t.Fatalf("non-empty extraMonitors must render as multi-line list\n%s", out)
	}
}

func TestHostVarsNixContainsWallpaperFlag(t *testing.T) {
	state := config.Default()
	state.Dots.Wallpapers = false

	out, err := HostVarsNix(state)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "wallpapers = {\n    enable = false;\n  };") {
		t.Fatalf("host-vars.nix must include disabled wallpapers flag\n%s", out)
	}
}

func TestFlakeNixUsesIndependentThinWahrweltWrapper(t *testing.T) {
	state := config.Default()
	state.Host.Hostname = "workstation"

	out, err := FlakeNix(state, LockModeIndependent)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`# lock mode: independent`,
		`nixpkgs.url = "github:NixOS/nixpkgs?rev=643809054d65fdd466a63e3155b8c498cb483c04";`,
		`url = "github:nix-community/neovim-nightly-overlay?rev=5522fc3be8969569a980f3d14b86600a55e713fc";`,
		`home-manager = {`,
		`nix-index-database = {`,
		`quickshell = {`,
		`wahrwelt = {`,
		`url = "github:TakuyaYagam1/wahrwelt/main?dir=Linux/NixOS/presets/personal";`,
		`inputs.nixpkgs.follows = "nixpkgs";`,
		`inputs.home-manager.follows = "home-manager";`,
		`inputs.nix-index-database.follows = "nix-index-database";`,
		`inputs.quickshell.follows = "quickshell";`,
		`inputs.stylix.follows = "stylix";`,
		`hostname = "workstation";`,
		`wahrwelt.lib.mkWahrweltHost`,
		`hostVars = ./host-vars.nix;`,
		`hardware = ./hardware-configuration.nix;`,
		`extraModules = [ ./configuration.nix ];`,
		`if builtins.pathExists ./home.nix then [ ./home.nix ] else [ ];`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("thin flake missing %q\n%s", want, out)
		}
	}
	for _, forbidden := range []string{
		`github:a-h/templ`,
		`inputs.templ.follows`,
		`caelestia-shell = {`,
		`caelestia-cli = {`,
		`noctalia = {`,
		`noctalia-shell = {`,
		`inputs.caelestia-shell.follows`,
		`inputs.caelestia-cli.follows`,
		`inputs.noctalia.follows`,
		`inputs.noctalia-shell.follows`,
	} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("thin flake must not keep legacy input %q\n%s", forbidden, out)
		}
	}
}

func TestFlakeNixSupportsManagedThinWahrweltWrapper(t *testing.T) {
	state := config.Default()
	state.Host.Hostname = "workstation"

	out, err := FlakeNix(state, LockModeManaged)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`# lock mode: managed`,
		`wahrwelt.url = "github:TakuyaYagam1/wahrwelt/main?dir=Linux/NixOS/presets/personal";`,
		`hostname = "workstation";`,
		`wahrwelt.lib.mkWahrweltHost`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("managed thin flake missing %q\n%s", want, out)
		}
	}
	for _, forbidden := range []string{
		`inputs.nixpkgs.follows = "nixpkgs";`,
		`nixpkgs.url = "github:NixOS/nixpkgs?rev=643809054d65fdd466a63e3155b8c498cb483c04";`,
		`url = "github:nix-community/neovim-nightly-overlay?rev=5522fc3be8969569a980f3d14b86600a55e713fc";`,
	} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("managed thin flake must not expose host-owned input %q\n%s", forbidden, out)
		}
	}
}

func TestGeneratedWahrweltWrapperIsRecognizedForInPlaceUpdates(t *testing.T) {
	state := config.Default()
	state.Host.Hostname = "workstation"

	text, err := FlakeNix(state, LockModeIndependent)
	if err != nil {
		t.Fatal(err)
	}
	if !isThinWrapperFlake(text) {
		t.Fatal("generated Wahrwelt flake should be recognized as a thin wrapper")
	}
	if !isGeneratedMySetupWrapperFlake(text) {
		t.Fatal("generated Wahrwelt flake should be recognized as an installer-managed wrapper")
	}
}

func TestFlakeNixUsesDevelopmentWahrweltChannel(t *testing.T) {
	state := config.Default()
	state.Source.Channel = config.SourceChannelDevelopment
	state.Host.Hostname = "workstation"

	for _, lockMode := range []LockMode{LockModeIndependent, LockModeManaged} {
		t.Run(string(lockMode), func(t *testing.T) {
			out, err := FlakeNix(state, lockMode)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, `github:TakuyaYagam1/wahrwelt/dev?dir=Linux/NixOS/presets/personal`) {
				t.Fatalf("development channel must point at dev branch\n%s", out)
			}
			if strings.Contains(out, `github:TakuyaYagam1/wahrwelt/main?dir=Linux/NixOS/presets/personal`) {
				t.Fatalf("development channel must not keep stable Wahrwelt URL\n%s", out)
			}
		})
	}
}

func TestFlakeNixUsesPresetEntrypointAndHostOwnedSecureBootInput(t *testing.T) {
	state := config.Default()
	state.Source.Channel = config.SourceChannelDevelopment
	state.Packages.Preset = "developer"
	state.Features.SecureBoot = true

	out, err := FlakeNix(state, LockModeIndependent)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`# preset: developer`,
		`url = "github:TakuyaYagam1/wahrwelt/dev?dir=Linux/NixOS/presets/developer";`,
		`outputs = inputs@{ wahrwelt, ... }:`,
		`lanzaboote = {`,
		`hostInputs = { inherit (inputs) lanzaboote; };`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("preset wrapper missing %q\n%s", want, out)
		}
	}
	for _, forbidden := range []string{
		`claude-code = {`,
		`codex = {`,
		`codex-desktop-linux = {`,
		`inputs.lanzaboote.follows = "lanzaboote";`,
	} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("preset wrapper must delegate %q to the preset flake\n%s", forbidden, out)
		}
	}
}

func TestFlakeNixDelegatesNoctaliaInputsForV4Selection(t *testing.T) {
	state := config.Default()
	state.Noctalia.Version = config.NoctaliaVersionV4
	state.Host.Hostname = "workstation"

	out, err := FlakeNix(state, LockModeIndependent)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `url = "github:TakuyaYagam1/wahrwelt/main?dir=Linux/NixOS/presets/personal";`) {
		t.Fatalf("independent wrapper must keep its Wahrwelt source\n%s", out)
	}
	for _, forbidden := range []string{
		`noctalia = {`,
		`noctalia-shell = {`,
		`inputs.noctalia.follows`,
		`inputs.noctalia-shell.follows`,
	} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("independent wrapper must delegate release-managed input %q to Wahrwelt\n%s", forbidden, out)
		}
	}
}

func TestFlakeNixScopesHostOwnedInputsByPreset(t *testing.T) {
	for _, preset := range []string{"minimal", "desktop", "developer"} {
		t.Run(preset, func(t *testing.T) {
			state := config.Default()
			state.Packages.Preset = preset
			state.Host.Hostname = "workstation"

			out, err := FlakeNix(state, LockModeIndependent)
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{
				`claude-code = {`,
				`codex = {`,
				`codex-desktop-linux = {`,
				`lanzaboote = {`,
			} {
				if strings.Contains(out, forbidden) {
					t.Fatalf("%s preset must not include %q\n%s", preset, forbidden, out)
				}
			}
			for _, want := range []string{
				`nixpkgs.url = "github:NixOS/nixpkgs?rev=643809054d65fdd466a63e3155b8c498cb483c04";`,
				`home-manager = {`,
				`neovim-nightly-overlay = {`,
				`url = "github:nix-community/neovim-nightly-overlay?rev=5522fc3be8969569a980f3d14b86600a55e713fc";`,
				`stylix = {`,
				`nix-index-database = {`,
				`# preset: ` + preset,
				`?dir=Linux/NixOS/presets/` + preset,
			} {
				if !strings.Contains(out, want) {
					t.Fatalf("%s preset must keep baseline input %q\n%s", preset, want, out)
				}
			}
			for _, desktopInput := range []string{`quickshell = {`, `zen-browser = {`} {
				got := strings.Contains(out, desktopInput)
				want := preset != "minimal"
				if got != want {
					t.Fatalf("%s preset: desktop input %q present=%v, want=%v\n%s", preset, desktopInput, got, want, out)
				}
			}
		})
	}
}

func TestFlakeNixDelegatesAIInputsToDeveloperOrPersonalPresetFlake(t *testing.T) {
	for _, preset := range config.PackagePresets {
		t.Run(preset, func(t *testing.T) {
			state := config.Default()
			state.Packages.Preset = preset

			out, err := FlakeNix(state, LockModeIndependent)
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{
				`claude-code = {`,
				`codex = {`,
				`codex-desktop-linux = {`,
				`inputs.claude-code.follows`,
				`inputs.codex.follows`,
				`inputs.codex-desktop-linux.follows`,
			} {
				if strings.Contains(out, forbidden) {
					t.Fatalf("preset %q must delegate AI input %q to its child flake\n%s", preset, forbidden, out)
				}
			}
		})
	}
}

func TestFlakeNixIncludesLanzabooteOnlyWhenSecureBootEnabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		state := config.Default()
		state.Features.SecureBoot = enabled

		out, err := FlakeNix(state, LockModeIndependent)
		if err != nil {
			t.Fatal(err)
		}
		got := strings.Contains(out, `lanzaboote = {`) &&
			strings.Contains(out, `hostInputs = { inherit (inputs) lanzaboote; };`)
		if got != enabled {
			t.Fatalf("SecureBoot=%v: lanzaboote present=%v, want=%v\n%s", enabled, got, enabled, out)
		}
		if strings.Contains(out, `inputs.lanzaboote.follows = "lanzaboote";`) {
			t.Fatalf("Lanzaboote is host-owned and must not be followed by Wahrwelt\n%s", out)
		}
	}
}

func TestFlakeNixManagedModeKeepsOnlyWahrweltAndRequiredSecureBootInput(t *testing.T) {
	state := config.Default()
	state.Packages.Preset = "personal"
	state.Features.SecureBoot = true
	state.Host.Hostname = "workstation"

	out, err := FlakeNix(state, LockModeManaged)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`claude-code`, `codex`, `nixpkgs.url`} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("managed mode must stay a single Wahrwelt input regardless of preset/toggles, found %q\n%s", forbidden, out)
		}
	}
	for _, want := range []string{
		`wahrwelt.url = "github:TakuyaYagam1/wahrwelt/main?dir=Linux/NixOS/presets/personal";`,
		`lanzaboote = {`,
		`hostInputs = { inherit (inputs) lanzaboote; };`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("managed mode missing %q\n%s", want, out)
		}
	}
}

func TestThinTemplatesExposeSystemAndHomeOverrides(t *testing.T) {
	if !strings.Contains(ConfigurationNix(), "environment.systemPackages") {
		t.Fatalf("configuration.nix template must expose system packages\n%s", ConfigurationNix())
	}
	if strings.Contains(ConfigurationNix(), "Edit this configuration file") ||
		strings.Contains(ConfigurationNix(), "services.desktopManager.plasma6") {
		t.Fatalf("configuration.nix template must stay a clean Wahrwelt override\n%s", ConfigurationNix())
	}
	if !strings.Contains(ConfigurationNix(), "./private") {
		t.Fatalf("configuration.nix template must import private defaults\n%s", ConfigurationNix())
	}
	for _, want := range []string{"./ida-pro.nix", "./ida-mcp.nix", "./ida-plugins.nix"} {
		if !strings.Contains(PrivateDefaultNix(), want) {
			t.Fatalf("private/default.nix template missing %q\n%s", want, PrivateDefaultNix())
		}
	}
	if !strings.Contains(HomeNix(), "home.packages") {
		t.Fatalf("home.nix template must expose home packages\n%s", HomeNix())
	}
}

func TestHomeWallpaperActivationHonorsDryRun(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/home/home.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`$DRY_RUN_CMD ${pkgs.findutils}/bin/find "$WALLS_DST" -maxdepth 1 -type f -name 'preview-*' -delete`,
		`$DRY_RUN_CMD ${pkgs.coreutils}/bin/cp -n --no-preserve=mode "$wall" "$WALLS_DST/"`,
		`[ -e "$wall" ] || continue`,
		`$DRY_RUN_CMD ${pkgs.coreutils}/bin/chmod -R u+w "$WALLS_DST"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("home wallpaper activation missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, `cp -n "$WALLS_SRC"/* "$WALLS_DST" 2>/dev/null || true`) {
		t.Fatalf("home wallpaper activation must not hide copy failures\n%s", text)
	}
	if strings.Contains(text, `${pkgs.coreutils}/bin/cp -n "$wall"`) {
		t.Fatalf("home wallpaper cp must keep the --no-preserve=mode flag\n%s", text)
	}
}

func TestHomeFaceAvatarHasTrackedFallback(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/home/home.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"avatarSource =",
		"builtins.pathExists ./avatar.jpg",
		"../themes/sddm-theme/icons/logo.png",
		`file.".face".source = avatarSource;`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("home face avatar fallback missing %q\n%s", want, text)
		}
	}
}

func TestHostDefaultUsesPathExistsForGeneratedPassword(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/hosts/NixOS/default.nix")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "builtins.pathExists ./hashed-password.nix") {
		t.Fatalf("host default should import hashed-password.nix conditionally")
	}
}

func TestHostDefaultKeepsIDAPackagesOutOfDefaultImports(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/hosts/NixOS/default.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{
		"ida-mcp.nix",
		"ida-plugins.nix",
		"ida-pro.nix",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("host default must not import quarantined IDA package %q\n%s", forbidden, text)
		}
	}
}

func TestSyncToEtcPreservesLocalHashedPassword(t *testing.T) {
	args := strings.Join(syncToEtcArgs("/tmp/staging", "/etc/nixos", LayoutFull), " ")
	if !strings.Contains(args, "--exclude=hosts/NixOS/hashed-password.nix") {
		t.Fatalf("syncToEtc must preserve host-local hashed-password.nix, got args: %s", args)
	}
	if strings.Contains(args, "--exclude=flake.lock") {
		t.Fatalf("syncToEtc must sync flake.lock so switch uses the same lock graph as dry-build, got args: %s", args)
	}
}

func TestThinSyncDoesNotDeleteLegacyMirrorFiles(t *testing.T) {
	args := strings.Join(syncToEtcArgs("/tmp/staging", "/etc/nixos", LayoutThin), " ")
	if strings.Contains(args, "--delete") {
		t.Fatalf("thin sync must not delete existing full-mirror files on migration, got args: %s", args)
	}
	if strings.Contains(args, "hosts/NixOS/hashed-password.nix") {
		t.Fatalf("thin sync should use root-level host-local paths, got args: %s", args)
	}
	if !strings.Contains(args, "--exclude=/hashed-password.nix") {
		t.Fatalf("thin sync must preserve hashed-password.nix ownership, got args: %s", args)
	}
}

func TestRunDryRunNoSwitchStopsAfterDryBuildWithoutWritingEtcDotsOrState(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "Linux/NixOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "Linux/NixOS/flake.nix"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeMinimalHyprDots(t, repo)
	if err := os.MkdirAll(filepath.Join(repo, "Linux/installer"), 0o755); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "hardware-configuration.nix"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := config.Default()
	state.User.Username = "tester"
	state.User.HomeDirectory = "/home/tester"
	state.Dots = config.Dots{Hypr: true}

	out := captureStdout(t, func() {
		err := Run(context.Background(), Options{
			Paths:      pathsForTest(repo, dest),
			State:      state,
			DryRun:     true,
			SkipSwitch: true,
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	dryBuild := strings.Index(out, "sudo nixos-rebuild dry-build --impure --flake ")
	backup := strings.Index(out, "sudo cp -a")
	dotsApply := strings.Index(out, "write hypr local config")
	stateWrite := strings.LastIndex(out, filepath.Join(dest, "wahrwelt/state.json"))
	noSwitch := strings.Index(out, "dry-build passed; --no-switch set")
	if dryBuild == -1 || noSwitch == -1 {
		t.Fatalf("expected dry-build and no-switch output, got:\n%s", out)
	}
	if backup != -1 {
		t.Fatalf("--no-switch must stop before /etc backup/sync\n%s", out)
	}
	if dotsApply != -1 {
		t.Fatalf("--no-switch must stop before user dotfile apply\n%s", out)
	}
	if stateWrite != -1 {
		t.Fatalf("state must not be written when --no-switch skips activation\n%s", out)
	}
}

func writeMinimalHyprDots(t *testing.T, repo string) {
	t.Helper()

	files := map[string]string{
		"Linux/dots/hypr/hyprland.lua":                 "require(\"hyprland.input\")\nrequire(\"hyprland.keybinds\")\n",
		"Linux/dots/hypr/hyprland/input.lua":           "hl.config({ input = { kb_layout = \"us\", kb_options = \"grp:alt_shift_toggle\" } })\n",
		"Linux/dots/hypr/hyprland/keybinds.lua":        "wahrwelt.load_runtime(\"shell-keybinds.lua\")\n",
		"Linux/dots/hypr/scripts/start-shell.sh":       "#!/usr/bin/env bash\n",
		"Linux/dots/hypr/caelestia/keybinds.lua":       "-- binds\n",
		"Linux/dots/hypr/caelestia/launcher.lua":       "-- launcher\n",
		"Linux/dots/hypr/shell-common-keybinds.lua":    "-- common\n",
		"Linux/dots/hypr/shell-workspace-keybinds.lua": "-- workspace\n",
		"Linux/dots/hypr/lib/wahrwelt.lua":             "return {}\n",
		"Linux/dots/hypr/variables.lua":                "return {}\n",
		"Linux/dots/hypr/scheme/default.lua":           "return {}\n",
	}
	for rel, content := range files {
		path := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunDoesNotWriteStateWhenDryBuildFails(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "Linux/NixOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "Linux/NixOS/flake.nix"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "Linux/dots/hypr/caelestia"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "Linux/dots/hypr/caelestia/keybinds.lua"), []byte("-- binds\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "Linux/installer"), 0o755); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "hardware-configuration.nix"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "rsync"), `#!/bin/sh
last=
prev=
for arg do
  prev=$last
  last=$arg
done
mkdir -p "$last"
cp -a "$prev"/. "$last"/
`)
	writeExecutable(t, filepath.Join(bin, "nix"), `#!/bin/sh
exit 0
`)
	writeExecutable(t, filepath.Join(bin, "sudo"), `#!/bin/sh
if [ "$1" = "nixos-rebuild" ]; then
  echo "nix says no" >&2
  exit 42
fi
exec "$@"
`)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	state := config.Default()
	state.User.Username = "tester"
	state.User.HomeDirectory = "/home/tester"
	state.Dots = config.Dots{}
	err := Run(context.Background(), Options{
		Paths: pathsForTest(repo, dest),
		State: state,
	})
	if err == nil {
		t.Fatal("expected dry-build failure")
	}
	if !strings.Contains(err.Error(), "nix says no") {
		t.Fatalf("expected dry-build error details, got:\n%s", err)
	}
	if _, statErr := os.Stat(filepath.Join(dest, "wahrwelt/state.json")); !os.IsNotExist(statErr) {
		t.Fatalf("state must not be written after failed dry-build, stat err: %v", statErr)
	}
}

func TestStageConfigurationIncludesDotsAndInstaller(t *testing.T) {
	repo := t.TempDir()
	nixos := filepath.Join(repo, "Linux", "NixOS")
	dots := filepath.Join(repo, "Linux", "dots")
	installer := filepath.Join(repo, "Linux", "installer")
	for _, dir := range []string{
		nixos,
		filepath.Join(dots, "hypr", "caelestia"),
		filepath.Join(dots, "hypr", "noctalia"),
		filepath.Join(dots, "hypr", "scripts"),
		installer,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join(nixos, "flake.nix"):                        "{}\n",
		filepath.Join(dots, "hypr", "caelestia", "keybinds.lua"): "-- caelestia\n",
		filepath.Join(dots, "hypr", "noctalia", "keybinds.lua"):  "-- noctalia\n",
		filepath.Join(dots, "hypr", "scripts", "start-shell.sh"): "#!/usr/bin/env bash\n",
		filepath.Join(installer, "go.mod"):                       "module test\n",
		filepath.Join(installer, "cmd", "mysetup", "main.go"):    "package main\n",
		filepath.Join(installer, "bin", "mysetup"):               "ignored\n",
		filepath.Join(installer, "coverage.out"):                 "ignored\n",
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	staging := t.TempDir()
	if err := stageConfiguration(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, paths.Sources{
		RepoRoot:  repo,
		NixOS:     nixos,
		Dots:      dots,
		Installer: installer,
	}, staging, config.Default(), LayoutFull, LockModeIndependent); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{
		"dots/hypr/caelestia/keybinds.lua",
		"dots/hypr/noctalia/keybinds.lua",
		"dots/hypr/scripts/start-shell.sh",
		"installer/go.mod",
		"installer/cmd/mysetup/main.go",
		"hosts/NixOS/host-vars.nix",
	} {
		if _, err := os.Stat(filepath.Join(staging, rel)); err != nil {
			t.Fatalf("staging missing %s: %v", rel, err)
		}
	}
	for _, rel := range []string{"installer/bin/mysetup", "installer/coverage.out"} {
		if _, err := os.Stat(filepath.Join(staging, rel)); !os.IsNotExist(err) {
			t.Fatalf("staging should exclude %s, stat err: %v", rel, err)
		}
	}
}

func TestStageConfigurationKeepsStagingWritableAfterReadonlyNixOSRoot(t *testing.T) {
	repo := t.TempDir()
	nixos := filepath.Join(repo, "Linux", "NixOS")
	dots := filepath.Join(repo, "Linux", "dots")
	installer := filepath.Join(repo, "Linux", "installer")
	for _, dir := range []string{
		nixos,
		filepath.Join(nixos, "hosts", "NixOS"),
		filepath.Join(dots, "hypr"),
		installer,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		filepath.Join(nixos, "flake.nix"):                       "{}\n",
		filepath.Join(nixos, "hosts", "NixOS", "host-vars.nix"): "stale\n",
		filepath.Join(dots, "hypr", "keybinds.lua"):             "-- binds\n",
		filepath.Join(installer, "cmd", "mysetup", "main.go"):   "package main\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(nixos, 0o555); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(nixos, "hosts", "NixOS", "host-vars.nix"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(nixos, "hosts", "NixOS"), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(filepath.Join(nixos, "hosts", "NixOS"), 0o755)
		_ = os.Chmod(nixos, 0o755)
	})

	staging := t.TempDir()
	err := stageConfiguration(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, paths.Sources{
		RepoRoot:  repo,
		NixOS:     nixos,
		Dots:      dots,
		Installer: installer,
	}, staging, config.Default(), LayoutFull, LockModeIndependent)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"dots/hypr/keybinds.lua",
		"installer/cmd/mysetup/main.go",
		"hosts/NixOS/host-vars.nix",
	} {
		if _, err := os.Stat(filepath.Join(staging, rel)); err != nil {
			t.Fatalf("staging missing %s after readonly NixOS copy: %v", rel, err)
		}
	}
}

func TestStageThinConfigurationWritesWrapperAndTemplates(t *testing.T) {
	repo := t.TempDir()
	nixos := filepath.Join(repo, "Linux", "NixOS")
	if err := os.MkdirAll(nixos, 0o755); err != nil {
		t.Fatal(err)
	}
	state := config.Default()
	state.Host.Hostname = "ThinHost"
	staging := t.TempDir()
	if err := stageConfiguration(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, paths.Sources{
		RepoRoot:  repo,
		NixOS:     nixos,
		Dots:      filepath.Join(repo, "Linux", "dots"),
		Installer: filepath.Join(repo, "Linux", "installer"),
	}, staging, state, LayoutThin, LockModeIndependent); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{
		"flake.nix",
		"host-vars.nix",
		"configuration.nix",
		"home.nix",
		"private",
		"private/default.nix",
	} {
		info, err := os.Stat(filepath.Join(staging, rel))
		if err != nil {
			t.Fatalf("thin staging missing %s: %v", rel, err)
		}
		if rel == "private" && !info.IsDir() {
			t.Fatalf("thin staging private should be a directory")
		}
	}
	privateDefault, err := os.ReadFile(filepath.Join(staging, "private", "default.nix"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(privateDefault), "./ida-pro.nix") {
		t.Fatalf("thin staging private/default.nix missing IDA example imports\n%s", privateDefault)
	}
	for _, rel := range []string{"flake.lock", "dots", "installer", "hosts/NixOS/host-vars.nix"} {
		if _, err := os.Stat(filepath.Join(staging, rel)); !os.IsNotExist(err) {
			t.Fatalf("thin staging should not include %s, stat err: %v", rel, err)
		}
	}
}

func TestPrepareThinHostLocalPreservesOverridesLockAndSecrets(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	for path, content := range map[string]string{
		filepath.Join(dest, "hardware-configuration.nix"): "hardware\n",
		filepath.Join(dest, "flake.nix"):                  "{ outputs = { mysetup, ... }: { nixosConfigurations.NixOS = mysetup.lib.mkMySetupHost { }; }; }\n",
		filepath.Join(dest, "flake.lock"):                 "existing-lock\n",
		filepath.Join(dest, "configuration.nix"):          "{ config, ... }: { }\n",
		filepath.Join(dest, "home.nix"):                   "{ pkgs, ... }: { }\n",
		filepath.Join(dest, "private", "ida-pro.nix"):     "{ pkgs, ... }: { }\n",
		filepath.Join(dest, "private", "ida.run"):         "binary payload\n",
		filepath.Join(dest, "secrets", "secrets.yaml"):    "secret: ENC\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := prepareStagingHostLocal(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, staging, dest, config.Default(), config.Secrets{}, LayoutThin); err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]string{
		filepath.Join(staging, "hardware-configuration.nix"): "hardware\n",
		filepath.Join(staging, "flake.nix"):                  "{ outputs = { mysetup, ... }: { nixosConfigurations.NixOS = mysetup.lib.mkMySetupHost { }; }; }\n",
		filepath.Join(staging, "flake.lock"):                 "existing-lock\n",
		filepath.Join(staging, "configuration.nix"):          "{ config, ... }: { }\n",
		filepath.Join(staging, "home.nix"):                   "{ pkgs, ... }: { }\n",
		filepath.Join(staging, "private", "default.nix"):     PrivateDefaultNix(),
		filepath.Join(staging, "private", "ida-pro.nix"):     "{ pkgs, ... }: { }\n",
		filepath.Join(staging, "private", "ida.run"):         "binary payload\n",
		filepath.Join(staging, "secrets", "secrets.yaml"):    "secret: ENC\n",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("unexpected content for %s: %q", path, string(data))
		}
	}
}

func TestGeneratedWrapperDetectionAcceptsDevelopmentURL(t *testing.T) {
	text := `{
  description = "Host-local MySetup NixOS wrapper";

  inputs = {
    mysetup.url = "github:TakuyaYagam1/MySetup/dev?dir=Linux/NixOS";
  };

  outputs = { mysetup, ... }:
    let
      hostname = "NixOS";
    in
    {
      nixosConfigurations.${hostname} = mysetup.lib.mkMySetupHost {
        hostVars = ./host-vars.nix;
        hardware = ./hardware-configuration.nix;
        extraModules = [ ./configuration.nix ];
        homeExtraModules =
          if builtins.pathExists ./home.nix then [ ./home.nix ] else [ ];
      };
    };
}
`
	if !isThinWrapperFlake(text) {
		t.Fatal("dev MySetup URL should be recognised as a thin wrapper flake")
	}
	if !isGeneratedMySetupWrapperFlake(text) {
		t.Fatal("dev MySetup URL should be recognised as generated MySetup wrapper")
	}
}

func TestGeneratedWrapperDetectionAcceptsNoctaliaV4URL(t *testing.T) {
	text := `{
  description = "Host-local MySetup NixOS wrapper";

  inputs = {
    mysetup.url = "github:TakuyaYagam1/MySetup/noctalia-v4?dir=Linux/NixOS";
  };

  outputs = { mysetup, ... }:
    let
      hostname = "NixOS";
    in
    {
      nixosConfigurations.${hostname} = mysetup.lib.mkMySetupHost {
        hostVars = ./host-vars.nix;
        hardware = ./hardware-configuration.nix;
        extraModules = [ ./configuration.nix ];
        homeExtraModules =
          if builtins.pathExists ./home.nix then [ ./home.nix ] else [ ];
      };
    };
}
`
	if !isThinWrapperFlake(text) {
		t.Fatal("noctalia-v4 MySetup URL should be recognised as a thin wrapper flake")
	}
	if !isGeneratedMySetupWrapperFlake(text) {
		t.Fatal("noctalia-v4 MySetup URL should be recognised as generated MySetup wrapper")
	}
}

func mustMigrateGeneratedThinFlake(t *testing.T, text string, state config.State) (string, bool) {
	t.Helper()
	migrated, changed, err := migrateGeneratedThinFlake(text, state)
	if err != nil {
		t.Fatal(err)
	}
	return migrated, changed
}

func TestMigrateGeneratedThinFlakeCanonicallyRegeneratesInstallerOwnedWrapper(t *testing.T) {
	old := `{
  description = "Host-local MySetup NixOS wrapper";

  inputs = {
    quickshell = {
      url = "github:outfoxxed/quickshell";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    noctalia-shell = {
      url = "github:noctalia-dev/noctalia-shell/v4.7.7";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    custom-overlay = {
      url = "github:example/custom-overlay";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    templ = {
      url = "github:a-h/templ";
      inputs = {
        nixpkgs.follows = "nixpkgs-stable";
        nixpkgs-unstable.follows = "nixpkgs";
      };
    };

    mysetup = {
      url = "github:TakuyaYagam1/MySetup?dir=Linux/NixOS";
      inputs.noctalia-shell.follows = "noctalia-shell";
      inputs.templ.follows = "templ";
      inputs.custom-overlay.follows = "custom-overlay";
    };
  };

  outputs = { mysetup, ... }:
    let
      hostname = "NixOS";
    in
    {
      nixosConfigurations.${hostname} = mysetup.lib.mkMySetupHost {
        hostVars = ./host-vars.nix;
        hardware = ./hardware-configuration.nix;
        extraModules = [ ./configuration.nix ];
        homeExtraModules =
          if builtins.pathExists ./home.nix then [ ./home.nix ] else [ ];
      };
    };
}
`

	migrated, changed := mustMigrateGeneratedThinFlake(t, old, config.Default())
	if !changed {
		t.Fatal("expected generated thin flake migration to report a change")
	}
	want, err := FlakeNix(config.Default(), LockModeIndependent)
	if err != nil {
		t.Fatal(err)
	}
	if migrated != want {
		t.Fatalf("installer-owned wrapper must be canonically regenerated\nwant:\n%s\ngot:\n%s", want, migrated)
	}
}

func TestMigrateGeneratedThinFlakeRemovesLegacyZapretDiscordYoutubeInput(t *testing.T) {
	old := `{
  description = "Host-local MySetup NixOS wrapper";

  inputs = {
    quickshell = {
      url = "github:outfoxxed/quickshell";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    noctalia = {
      url = "github:noctalia-dev/noctalia";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    noctalia-shell = {
      url = "github:noctalia-dev/noctalia-shell/v4.7.7";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    zapret-discord-youtube = {
      url = "github:kartavkun/zapret-discord-youtube";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    mysetup = {
      url = "github:TakuyaYagam1/MySetup?dir=Linux/NixOS";
      inputs.noctalia.follows = "noctalia";
      inputs.noctalia-shell.follows = "noctalia-shell";
      inputs.zapret-discord-youtube.follows = "zapret-discord-youtube";
    };
  };

  outputs = { mysetup, ... }:
    let
      hostname = "NixOS";
    in
    {
      nixosConfigurations.${hostname} = mysetup.lib.mkMySetupHost {
        hostVars = ./host-vars.nix;
        hardware = ./hardware-configuration.nix;
        extraModules = [ ./configuration.nix ];
        homeExtraModules =
          if builtins.pathExists ./home.nix then [ ./home.nix ] else [ ];
      };
    };
}
`

	migrated, changed := mustMigrateGeneratedThinFlake(t, old, config.Default())
	if !changed {
		t.Fatal("expected generated thin flake migration to report a change")
	}
	for _, forbidden := range []string{
		`zapret-discord-youtube = {`,
		`github:kartavkun/zapret-discord-youtube`,
		`inputs.zapret-discord-youtube.follows`,
		`noctalia = {`,
		`noctalia-shell = {`,
		`inputs.noctalia.follows`,
		`inputs.noctalia-shell.follows`,
	} {
		if strings.Contains(migrated, forbidden) {
			t.Fatalf("migrated flake kept retired input %q\n%s", forbidden, migrated)
		}
	}
}

func TestMigrateGeneratedThinFlakeRewritesLegacyNoctaliaV4MySetupURL(t *testing.T) {
	old := `{
  description = "Host-local MySetup NixOS wrapper";

  inputs = {
    noctalia = {
      url = "github:noctalia-dev/noctalia";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    mysetup = {
	      url = "github:TakuyaYagam1/MySetup/noctalia-v4?dir=Linux/NixOS";
      inputs.noctalia.follows = "noctalia";
    };
  };
}
`

	migrated, changed := mustMigrateGeneratedThinFlake(t, old, config.Default())
	if !changed {
		t.Fatal("expected generated thin flake migration to rewrite v4-selected wrapper")
	}
	if !strings.Contains(migrated, `github:TakuyaYagam1/wahrwelt/main?dir=Linux/NixOS`) {
		t.Fatalf("migrated flake missing current Wahrwelt source\n%s", migrated)
	}
	for _, forbidden := range []string{
		`github:TakuyaYagam1/MySetup/noctalia-v4?dir=Linux/NixOS`,
		`mysetup.lib.mkMySetupHost`,
		`github:noctalia-dev/noctalia`,
		`inputs.noctalia.follows`,
	} {
		if strings.Contains(migrated, forbidden) {
			t.Fatalf("migrated flake kept stale value %q\n%s", forbidden, migrated)
		}
	}
}

func TestMigrateGeneratedThinFlakeRemovesHostOwnedNoctaliaInputs(t *testing.T) {
	old := `{
  description = "Host-local MySetup NixOS wrapper";

  inputs = {
    quickshell = {
      url = "github:outfoxxed/quickshell";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    noctalia = {
      url = "github:noctalia-dev/noctalia/v5.0.0-beta1";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    noctalia-shell = {
      url = "github:noctalia-dev/noctalia-shell/v4.7.7";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    mysetup = {
      url = "github:TakuyaYagam1/MySetup?dir=Linux/NixOS";
      inputs.noctalia.follows = "noctalia";
      inputs.noctalia-shell.follows = "noctalia-shell";
    };
  };
}
`

	migrated, changed := mustMigrateGeneratedThinFlake(t, old, config.Default())
	if !changed {
		t.Fatal("expected generated thin flake migration to report a change")
	}
	for _, forbidden := range []string{
		`noctalia = {`,
		`noctalia-shell = {`,
		`inputs.noctalia.follows`,
		`inputs.noctalia-shell.follows`,
	} {
		if strings.Contains(migrated, forbidden) {
			t.Fatalf("migrated flake kept host-owned Noctalia input %q\n%s", forbidden, migrated)
		}
	}
}

func TestMigrateGeneratedThinFlakeRemovesHostOwnedCaelestiaInputs(t *testing.T) {
	old := `{
  description = "Host-local MySetup NixOS wrapper";

  inputs = {
    caelestia-shell = {
      url = "github:caelestia-dots/shell";
      inputs = {
        caelestia-cli.follows = "caelestia-cli";
        nixpkgs.follows = "nixpkgs";
        quickshell.follows = "quickshell";
      };
    };
    caelestia-cli = {
      url = "github:caelestia-dots/cli";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    quickshell = {
      url = "github:outfoxxed/quickshell";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    mysetup = {
      url = "github:TakuyaYagam1/MySetup?dir=Linux/NixOS";
      inputs.caelestia-shell.follows = "caelestia-shell";
      inputs.caelestia-cli.follows = "caelestia-cli";
    };
  };
}
`

	migrated, changed := mustMigrateGeneratedThinFlake(t, old, config.Default())
	if !changed {
		t.Fatal("expected generated thin flake migration to report a change")
	}
	for _, forbidden := range []string{
		`caelestia-shell = {`,
		`caelestia-cli = {`,
		`inputs.caelestia-shell.follows`,
		`inputs.caelestia-cli.follows`,
	} {
		if strings.Contains(migrated, forbidden) {
			t.Fatalf("migrated flake kept host-owned Caelestia input %q\n%s", forbidden, migrated)
		}
	}
}

func TestMigrateGeneratedThinFlakeRemovesPersonalAIInputsWhenPresetNoLongerPersonal(t *testing.T) {
	before := config.Default()
	before.Packages.Preset = "personal"
	old, err := FlakeNix(before, LockModeIndependent)
	if err != nil {
		t.Fatal(err)
	}

	after := config.Default()
	after.Packages.Preset = "minimal"
	migrated, changed := mustMigrateGeneratedThinFlake(t, old, after)
	if !changed {
		t.Fatal("expected migration to report a change when preset drops out of personal")
	}
	for _, forbidden := range []string{
		`claude-code = {`,
		`inputs.claude-code.follows = "claude-code";`,
		`codex = {`,
		`inputs.codex.follows = "codex";`,
		`codex-desktop-linux = {`,
		`inputs.codex-desktop-linux.follows = "codex-desktop-linux";`,
	} {
		if strings.Contains(migrated, forbidden) {
			t.Fatalf("migrated flake kept %q after preset dropped out of personal\n%s", forbidden, migrated)
		}
	}
}

func TestMigrateGeneratedThinFlakeSwitchesToPersonalEntrypointWithoutHostOwnedAIInputs(t *testing.T) {
	before := config.Default()
	before.Packages.Preset = "minimal"
	old, err := FlakeNix(before, LockModeIndependent)
	if err != nil {
		t.Fatal(err)
	}

	after := config.Default()
	after.Packages.Preset = "personal"
	migrated, changed := mustMigrateGeneratedThinFlake(t, old, after)
	if !changed {
		t.Fatal("expected migration to report a change when preset becomes personal")
	}
	for _, want := range []string{
		`# preset: personal`,
		`github:TakuyaYagam1/wahrwelt/main?dir=Linux/NixOS/presets/personal`,
	} {
		if !strings.Contains(migrated, want) {
			t.Fatalf("migrated flake missing %q after preset became personal\n%s", want, migrated)
		}
	}
	for _, forbidden := range []string{
		`claude-code = {`,
		`codex = {`,
		`codex-desktop-linux = {`,
	} {
		if strings.Contains(migrated, forbidden) {
			t.Fatalf("migrated flake must delegate %q to the personal entrypoint\n%s", forbidden, migrated)
		}
	}
}

func TestMigrateGeneratedThinFlakeAddsAndRemovesLanzabooteWithSecureBootToggle(t *testing.T) {
	t.Run("off to on", func(t *testing.T) {
		before := config.Default()
		before.Features.SecureBoot = false
		old, err := FlakeNix(before, LockModeIndependent)
		if err != nil {
			t.Fatal(err)
		}

		after := config.Default()
		after.Features.SecureBoot = true
		migrated, changed := mustMigrateGeneratedThinFlake(t, old, after)
		if !changed {
			t.Fatal("expected migration to report a change when Secure Boot is enabled")
		}
		if !strings.Contains(migrated, `lanzaboote = {`) ||
			!strings.Contains(migrated, `hostInputs = { inherit (inputs) lanzaboote; };`) {
			t.Fatalf("migrated flake missing lanzaboote after enabling Secure Boot\n%s", migrated)
		}
		if strings.Contains(migrated, `inputs.lanzaboote.follows = "lanzaboote";`) {
			t.Fatalf("migrated flake must keep Lanzaboote host-owned\n%s", migrated)
		}
	})

	t.Run("on to off", func(t *testing.T) {
		before := config.Default()
		before.Features.SecureBoot = true
		old, err := FlakeNix(before, LockModeIndependent)
		if err != nil {
			t.Fatal(err)
		}

		after := config.Default()
		after.Features.SecureBoot = false
		migrated, changed := mustMigrateGeneratedThinFlake(t, old, after)
		if !changed {
			t.Fatal("expected migration to report a change when Secure Boot is disabled")
		}
		if strings.Contains(migrated, `lanzaboote = {`) ||
			strings.Contains(migrated, `hostInputs = { inherit (inputs) lanzaboote; };`) {
			t.Fatalf("migrated flake kept lanzaboote after disabling Secure Boot\n%s", migrated)
		}
	})
}

func TestMigrateGeneratedThinFlakeIsIdempotentWhenDesiredStateAlreadyMatches(t *testing.T) {
	state := config.Default()
	state.Packages.Preset = "personal"
	state.Features.SecureBoot = true

	old, err := FlakeNix(state, LockModeIndependent)
	if err != nil {
		t.Fatal(err)
	}

	firstPass, changed := mustMigrateGeneratedThinFlake(t, old, state)
	if changed {
		t.Fatalf("expected no change migrating a freshly-generated flake against the same state it was generated for\n%s", firstPass)
	}

	secondPass, changedAgain := mustMigrateGeneratedThinFlake(t, firstPass, state)
	if changedAgain {
		t.Fatalf("expected second migration pass to be a no-op\n%s", secondPass)
	}
	if firstPass != secondPass {
		t.Fatalf("expected repeated migration with unchanged state to be byte-identical\nfirst:\n%s\nsecond:\n%s", firstPass, secondPass)
	}
	if count := strings.Count(secondPass, `lanzaboote = {`); count != 1 {
		t.Fatalf("expected exactly one Lanzaboote block after repeated migration, got %d\n%s", count, secondPass)
	}
	for _, forbidden := range []string{`claude-code = {`, `codex = {`, `codex-desktop-linux = {`} {
		if strings.Contains(secondPass, forbidden) {
			t.Fatalf("expected AI input %q to stay delegated after repeated migration\n%s", forbidden, secondPass)
		}
	}
}

func TestPrepareThinHostLocalPreservesGeneratedThinWrapper(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	existingFlake := `{
  description = "Host-local MySetup NixOS wrapper";

  inputs = {
    mysetup.url = "github:TakuyaYagam1/MySetup?dir=Linux/NixOS";
  };

  outputs = { mysetup, ... }:
    let
      system = "x86_64-linux";
      hostname = "NixOS";
    in
    {
      nixosConfigurations.${hostname} = mysetup.lib.mkMySetupHost {
        inherit system hostname;

        hostVars = ./host-vars.nix;
        hardware = ./hardware-configuration.nix;
        extraModules = [ ./configuration.nix ];
        homeExtraModules =
          if builtins.pathExists ./home.nix then [ ./home.nix ] else [ ];
      };
    };
}
`
	wantFlake, changed := mustMigrateGeneratedThinFlake(t, existingFlake, config.Default())
	if !changed {
		t.Fatal("expected legacy generated wrapper to migrate to Wahrwelt")
	}
	for path, content := range map[string]string{
		filepath.Join(staging, "flake.nix"):               "new generated wrapper\n",
		filepath.Join(dest, "hardware-configuration.nix"): "hardware\n",
		filepath.Join(dest, "flake.nix"):                  existingFlake,
		filepath.Join(dest, "flake.lock"):                 "existing-lock\n",
		filepath.Join(dest, "configuration.nix"):          "{ config, ... }: { }\n",
		filepath.Join(dest, "home.nix"):                   "{ pkgs, ... }: { }\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := prepareStagingHostLocal(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, staging, dest, config.Default(), config.Secrets{}, LayoutThin); err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]string{
		filepath.Join(staging, "flake.nix"):         wantFlake,
		filepath.Join(staging, "flake.lock"):        "existing-lock\n",
		filepath.Join(staging, "configuration.nix"): "{ config, ... }: { }\n",
		filepath.Join(staging, "home.nix"):          "{ pkgs, ... }: { }\n",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("unexpected content for %s: %q", path, string(data))
		}
	}
}

func TestPrepareThinHostLocalOverwritesFreshNixOSOverrides(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	for path, content := range map[string]string{
		filepath.Join(staging, "configuration.nix"):       ConfigurationNix(),
		filepath.Join(staging, "home.nix"):                HomeNix(),
		filepath.Join(dest, "hardware-configuration.nix"): "hardware\n",
		filepath.Join(dest, "configuration.nix"): `# Edit this configuration file to define what should be installed on
{ pkgs, ... }:

{
  imports = [ ./hardware-configuration.nix ];
  boot.loader.systemd-boot.enable = true;
  services.desktopManager.plasma6.enable = true;
}
`,
		filepath.Join(dest, "home.nix"): "{ pkgs, ... }: { home.packages = [ pkgs.kate ]; }\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := prepareStagingHostLocal(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, staging, dest, config.Default(), config.Secrets{}, LayoutThin); err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]string{
		filepath.Join(staging, "configuration.nix"): ConfigurationNix(),
		filepath.Join(staging, "home.nix"):          HomeNix(),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("fresh NixOS adoption must keep generated %s, got:\n%s", filepath.Base(path), data)
		}
	}
}

func TestPrepareThinHostLocalPreservesPrivateDefault(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	for path, content := range map[string]string{
		filepath.Join(dest, "hardware-configuration.nix"): "hardware\n",
		filepath.Join(dest, "private", "default.nix"):     "{ ... }: { imports = [ ./custom.nix ]; }\n",
		filepath.Join(dest, "private", "custom.nix"):      "{ ... }: { }\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := prepareStagingHostLocal(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, staging, dest, config.Default(), config.Secrets{}, LayoutThin); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(staging, "private", "default.nix"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "{ ... }: { imports = [ ./custom.nix ]; }\n"; got != want {
		t.Fatalf("private/default.nix should be preserved, got %q", got)
	}
}

func TestPrepareThinHostLocalRejectsPrivateFile(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	for path, content := range map[string]string{
		filepath.Join(dest, "hardware-configuration.nix"): "hardware\n",
		filepath.Join(dest, "private"):                    "not a directory\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	err := prepareStagingHostLocal(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, staging, dest, config.Default(), config.Secrets{}, LayoutThin)
	if err == nil || !strings.Contains(err.Error(), "private path is not a directory") {
		t.Fatalf("expected private file target error, got %v", err)
	}
}

func TestPrepareThinHostLocalReplacesLegacyFullFlake(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	for path, content := range map[string]string{
		filepath.Join(staging, "flake.nix"):                    "generated thin wrapper\n",
		filepath.Join(staging, "configuration.nix"):            ConfigurationNix(),
		filepath.Join(staging, "home.nix"):                     HomeNix(),
		filepath.Join(dest, "flake.nix"):                       "{ description = \"NixOS + Caelestia\"; }\n",
		filepath.Join(dest, "flake.lock"):                      "legacy full lock\n",
		filepath.Join(dest, "hardware-configuration.nix"):      "hardware\n",
		filepath.Join(dest, "configuration.nix"):               "{ pkgs, ... }: { services.desktopManager.plasma6.enable = true; }\n",
		filepath.Join(dest, "home.nix"):                        "{ pkgs, ... }: { home.packages = [ pkgs.kate ]; }\n",
		filepath.Join(dest, "hosts", "NixOS", "host-vars.nix"): "{ }\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := prepareStagingHostLocal(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, staging, dest, config.Default(), config.Secrets{}, LayoutThin); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(staging, "flake.nix"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "generated thin wrapper\n" {
		t.Fatalf("legacy full flake should not overwrite generated thin wrapper, got:\n%s", data)
	}
	if _, err := os.Stat(filepath.Join(staging, "flake.lock")); !os.IsNotExist(err) {
		t.Fatalf("legacy full flake.lock should not be copied into thin staging, stat err: %v", err)
	}
	for path, want := range map[string]string{
		filepath.Join(staging, "configuration.nix"): ConfigurationNix(),
		filepath.Join(staging, "home.nix"):          HomeNix(),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("legacy non-thin overrides should not overwrite generated %s, got:\n%s", filepath.Base(path), data)
		}
	}
}

func TestPrepareThinHostLocalStagesLegacySecretsWhenRootMissing(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	for path, content := range map[string]string{
		filepath.Join(dest, "hardware-configuration.nix"):                "hardware\n",
		filepath.Join(dest, "hosts", "NixOS", "secrets", "secrets.yaml"): "secret: legacy\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := prepareStagingHostLocal(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, staging, dest, config.Default(), config.Secrets{}, LayoutThin); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(staging, "secrets", "secrets.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "secret: legacy\n" {
		t.Fatalf("expected legacy secrets to be staged for thin migration, got %q", data)
	}
}

func TestCopyThinSecretsFallsBackToSudoRsync(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	secrets := filepath.Join(dest, "secrets")
	if err := os.MkdirAll(secrets, 0o755); err != nil {
		t.Fatal(err)
	}

	fake := &fakeRunner{
		failOn: func(name string, _ []string) error {
			if name == "rsync" {
				return os.ErrPermission
			}
			return nil
		},
	}
	if err := copyExistingThinHostLocal(context.Background(), fake, staging, dest, config.Default()); err != nil {
		t.Fatal(err)
	}

	commands := commandSummary(fake.calls)
	if !strings.Contains(commands, "rsync -a --delete --checksum") {
		t.Fatalf("expected normal rsync attempt, got:\n%s", commands)
	}
	if !strings.Contains(commands, "sudo rsync -a --delete --checksum --chown") {
		t.Fatalf("expected sudo rsync fallback, got:\n%s", commands)
	}
}

func TestWriteStagedSecretsMigratesMissingThinSecrets(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	source := filepath.Join(staging, "secrets", "secrets.yaml")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("secret: staged\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	fake := &fakeRunner{}
	if err := writeStagedSecrets(context.Background(), fake, staging, dest, LayoutThin); err != nil {
		t.Fatal(err)
	}

	commands := commandSummary(fake.calls)
	for _, want := range []string{
		"sudo mkdir -p " + filepath.Join(dest, "secrets"),
		"sudo rsync -a --delete --checksum --chown root:root",
		filepath.Join(staging, "secrets") + "/",
		filepath.Join(dest, "secrets") + "/",
	} {
		if !strings.Contains(commands, want) {
			t.Fatalf("expected secrets migration command %q, got:\n%s", want, commands)
		}
	}
}

func TestWriteStagedSecretsPreservesExistingThinSecrets(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	for path, content := range map[string]string{
		filepath.Join(staging, "secrets", "secrets.yaml"): "secret: staged\n",
		filepath.Join(dest, "secrets", "secrets.yaml"):    "secret: existing\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	fake := &fakeRunner{}
	if err := writeStagedSecrets(context.Background(), fake, staging, dest, LayoutThin); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("existing root secrets should be preserved, got:\n%s", commandSummary(fake.calls))
	}
}

func TestWriteStagedSecretsRejectsFileTarget(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	for path, content := range map[string]string{
		filepath.Join(staging, "secrets", "secrets.yaml"): "secret: staged\n",
		filepath.Join(dest, "secrets"):                    "not a directory\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	fake := &fakeRunner{}
	err := writeStagedSecrets(context.Background(), fake, staging, dest, LayoutThin)
	if err == nil || !strings.Contains(err.Error(), "target secrets path is not a directory") {
		t.Fatalf("expected file target error, got %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("file target should fail before sudo writes, got:\n%s", commandSummary(fake.calls))
	}
}

func TestFlakeCanUseInstalledInstallerSource(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/flake.nix")
	if err != nil {
		t.Fatal(err)
	}
	presetFlake, err := os.ReadFile("../../../NixOS/lib/preset-flake.nix")
	if err != nil {
		t.Fatal(err)
	}
	layout, err := os.ReadFile("../../../NixOS/lib/layout.nix")
	if err != nil {
		t.Fatal(err)
	}
	packages, err := os.ReadFile("../../../NixOS/lib/flake-packages.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data) + string(presetFlake) + string(layout) + string(packages)
	for _, want := range []string{
		"layout = import ./layout.nix",
		"installerSource = layout.installer",
		`(nixosRoot + "/installer")`,
		`(nixosRoot + "/../installer")`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("flake mysetup package must support installed /etc/nixos/installer source; missing %q\n%s", want, text)
		}
	}
}

func TestFlakeWahrweltWrapperCanRunFromRemoteSource(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/flake.nix")
	if err != nil {
		t.Fatal(err)
	}
	packages, err := os.ReadFile("../../../NixOS/lib/flake-packages.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data) + string(packages)
	for _, want := range []string{
		"wahrweltRuntimeSource",
		`cp -a ${nixosSource} "$out/NixOS"`,
		`cp -a ${dotsSource} "$out/dots"`,
		`cp -a ${installerSource} "$out/installer"`,
		"--set WAHRWELT_REPO_ROOT ${wahrweltRuntimeSource}/NixOS",
		"--set MYSETUP_REPO_ROOT ${wahrweltRuntimeSource}/NixOS",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("flake mysetup wrapper missing remote source support %q\n%s", want, text)
		}
	}
}

func TestInnerNixOSFlakeUsesSelfForOmniRouterOverlay(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/flake.nix")
	if err != nil {
		t.Fatal(err)
	}
	presetFlake, err := os.ReadFile("../../../NixOS/lib/preset-flake.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data) + string(presetFlake)
	for _, want := range []string{
		"inputsForModules = inputs // {",
		"wahrwelt = self;",
		"mysetup = self;",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("NixOS flake must expose the current flake revision to overlays, missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, `mysetup.url = "github:TakuyaYagam1/MySetup";`) {
		t.Fatalf("NixOS flake must not pin a nested legacy MySetup input for OmniRouter updates\n%s", text)
	}
}

func TestPrepareStagingHostLocalCopiesHardwareAndPreservesStagedLock(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(staging, "flake.lock"), []byte("staged-lock\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "hardware-configuration.nix"), []byte("hardware\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "flake.lock"), []byte("lock\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := prepareStagingHostLocal(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, staging, dest, config.Default(), config.Secrets{}, LayoutFull); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		filepath.Join(staging, "hosts/NixOS/hardware-configuration.nix"): "hardware\n",
		filepath.Join(staging, "flake.lock"):                             "staged-lock\n",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("unexpected content for %s: %q", path, string(data))
		}
	}
}

func TestPrepareStagingHostLocalDryRunDoesNotHashPassword(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "hardware-configuration.nix"), []byte("hardware\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mkpasswd := filepath.Join(bin, "mkpasswd")
	if err := os.WriteFile(mkpasswd, []byte("#!/bin/sh\nexit 42\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	runner := run.Runner{DryRun: true, Stdout: io.Discard, Stderr: io.Discard}
	err := prepareStagingHostLocal(context.Background(), runner, staging, dest, config.Default(), config.Secrets{UserPassword: "secret"}, LayoutThin)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(staging, "hashed-password.nix"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "!mysetup-dry-run-placeholder") {
		t.Fatalf("expected dry-run placeholder hash, got:\n%s", data)
	}
}

func TestPrepareStagingHostLocalCopiesPermissionDeniedHashWithSudo(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "hardware-configuration.nix"), []byte("hardware\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash := filepath.Join(dest, "hashed-password.nix")
	if err := os.MkdirAll(filepath.Dir(hash), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hash, []byte("{ hash }\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(hash, 0o600)
	})

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := prepareStagingHostLocal(context.Background(), runner, staging, dest, config.Default(), config.Secrets{}, LayoutThin); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	for _, want := range []string{
		"sudo install -D -m 600",
		"-o",
		"-g",
		hash,
		filepath.Join(staging, "hashed-password.nix"),
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("permission denied hash should be staged with sudo install, missing %q\n%s", want, got)
		}
	}
}

func TestWriteStagedHashedPasswordInstallsStagingArtifact(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	source := filepath.Join(staging, "hashed-password.nix")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte(HashedPasswordNix("hash-from-dry-build")), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "sudo"), `#!/bin/sh
printf '%s\n' "$*"
`)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	var out bytes.Buffer
	runner := run.Runner{Stdout: &out, Stderr: &out}
	err := writeStagedHashedPassword(context.Background(), runner, staging, dest, config.Secrets{UserPassword: "secret"}, LayoutThin)
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, source) {
		t.Fatalf("expected staged hashed-password.nix to be installed, got:\n%s", got)
	}
	if strings.Contains(got, "mkpasswd") {
		t.Fatalf("write phase must not hash password again, got:\n%s", got)
	}
}

func TestWriteStagedHashedPasswordMigratesMissingThinHash(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	source := filepath.Join(staging, "hashed-password.nix")
	if err := os.WriteFile(source, []byte(HashedPasswordNix("existing-hash")), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "sudo"), `#!/bin/sh
printf '%s\n' "$*"
`)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	var out bytes.Buffer
	runner := run.Runner{Stdout: &out, Stderr: &out}
	err := writeStagedHashedPassword(context.Background(), runner, staging, dest, config.Secrets{}, LayoutThin)
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, source) || !strings.Contains(got, filepath.Join(dest, "hashed-password.nix")) {
		t.Fatalf("expected missing root hash to be installed, got:\n%s", got)
	}
}

func TestWriteStagedHashedPasswordPreservesExistingThinHash(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(staging, "hashed-password.nix"), []byte(HashedPasswordNix("staged")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "hashed-password.nix"), []byte(HashedPasswordNix("existing")), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	runner := run.Runner{Stdout: &out, Stderr: &out}
	err := writeStagedHashedPassword(context.Background(), runner, staging, dest, config.Secrets{}, LayoutThin)
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != "" {
		t.Fatalf("existing hash should be preserved when no new password is provided, got:\n%s", out.String())
	}
}

func TestWriteStagedHashedPasswordRejectsDirectoryTarget(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(staging, "hashed-password.nix"), []byte(HashedPasswordNix("staged")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dest, "hashed-password.nix"), 0o755); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	runner := run.Runner{Stdout: &out, Stderr: &out}
	err := writeStagedHashedPassword(context.Background(), runner, staging, dest, config.Secrets{}, LayoutThin)
	if err == nil || !strings.Contains(err.Error(), "target hashed-password.nix is not a regular file") {
		t.Fatalf("expected non-regular hash target error, got %v", err)
	}
	if out.String() != "" {
		t.Fatalf("non-regular hash target should fail before sudo writes, got:\n%s", out.String())
	}
}

func TestHandlePreSwitchErrorRestoresBackup(t *testing.T) {
	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	err := handlePreSwitchError(context.Background(), runner, "/etc/nixos", "/etc/nixos.bak.123", os.ErrPermission)
	if err == nil {
		t.Fatal("expected wrapped pre-switch error")
	}
	if !strings.Contains(err.Error(), "restored /etc/nixos from /etc/nixos.bak.123") {
		t.Fatalf("expected restored note, got %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"sudo mkdir -p /etc/nixos",
		"sudo rsync -a --delete /etc/nixos.bak.123/ /etc/nixos/",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("restore command missing %q\n%s", want, got)
		}
	}
}

func TestHomeShellModuleInstallsAllBoundScripts(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/home/shells/default.nix")
	if err != nil {
		t.Fatal(err)
	}
	helpers, err := os.ReadFile("../../../NixOS/home/lib/dotfiles.nix")
	if err != nil {
		t.Fatal(err)
	}
	manifestData, err := os.ReadFile("../shellruntime/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		HyprScripts []string `json:"hyprScripts"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	text := string(data) + string(helpers)
	for _, script := range manifest.HyprScripts {
		scriptPath := filepath.Join("../../../dots/hypr/scripts", script)
		if _, err := os.Stat(scriptPath); err != nil {
			t.Fatalf("manifest script %s must exist: %v", script, err)
		}
	}
	if !strings.Contains(text, "inherit (shellRuntimeManifest) hyprScripts end4Scripts;") {
		t.Fatalf("home shell module must source scripts from shell runtime manifest\n%s", text)
	}
	for _, want := range []string{
		"hyprctl reload",
		"start-shell.sh >/dev/null 2>&1 || true",
		"wahrwelt/active-shell",
		"wahrwelt/hypr-runtime",
		`"quickshell/wahrwelt-shell-selector"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("home/shells/default.nix must include %q\n%s", want, text)
		}
	}
}

func pathsForTest(repo, dest string) paths.Options {
	return paths.Options{
		RepoRoot:  repo,
		NixOSDest: dest,
		StatePath: filepath.Join(dest, "wahrwelt", "state.json"),
		DraftPath: filepath.Join(dest, "draft.json"),
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = write
	defer func() {
		os.Stdout = original
	}()
	fn()
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := out.ReadFrom(read); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func writeExecutable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}
