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

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

func TestHostVarsNixContainsFeatureFlags(t *testing.T) {
	state := config.Default()
	state.Host.Hostname = "workstation"
	state.Features.CTFTools = true
	state.Zapret.Enable = true
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
		`ctfTools = true;`,
		`enable = true;`,
		`consoleKeyMap = "us";`,
		`keyboardToggle = "grp:alt_shift_toggle";`,
		`display = {`,
		`monitorName = "eDP-1";`,
		`monitorMode = "1920x1080@144";`,
		`monitorPosition = "0x0";`,
		`monitorScale = "1";`,
		`windowOpacity = "0.8";`,
		`wallpapers = {`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("host-vars.nix missing %q\n%s", want, out)
		}
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

func TestHomeWallpaperActivationHonorsDryRun(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/home/home.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`$DRY_RUN_CMD ${pkgs.findutils}/bin/find "$WALLS_DST" -maxdepth 1 -type f -name 'preview-*' -delete`,
		`$DRY_RUN_CMD ${pkgs.coreutils}/bin/cp -n "$wall" "$WALLS_DST/"`,
		`[ -e "$wall" ] || continue`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("home wallpaper activation missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, `cp -n "$WALLS_SRC"/* "$WALLS_DST" 2>/dev/null || true`) {
		t.Fatalf("home wallpaper activation must not hide copy failures\n%s", text)
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
	args := strings.Join(syncToEtcArgs("/tmp/staging", "/etc/nixos"), " ")
	if !strings.Contains(args, "--exclude=hosts/NixOS/hashed-password.nix") {
		t.Fatalf("syncToEtc must preserve host-local hashed-password.nix, got args: %s", args)
	}
	if strings.Contains(args, "--exclude=flake.lock") {
		t.Fatalf("syncToEtc must sync flake.lock so switch uses the same lock graph as dry-build, got args: %s", args)
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

	dryBuild := strings.Index(out, "sudo nixos-rebuild dry-build --flake ")
	backup := strings.Index(out, "sudo cp -a")
	dotsApply := strings.Index(out, "write hypr local config")
	stateWrite := strings.LastIndex(out, filepath.Join(dest, "mysetup/state.json"))
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
		"Linux/dots/hypr/hyprland.conf":                 "monitor = eDP-1, 2560x1600@120, 0x0, 1\nsource = ~/.config/hypr/hyprland/input.conf\n",
		"Linux/dots/hypr/hyprland/input.conf":           "input {\n    kb_layout = us\n    kb_options = grp:alt_shift_toggle\n}\n",
		"Linux/dots/hypr/hyprland/keybinds.conf":        "source = $hypr/shell-keybinds.conf\n",
		"Linux/dots/hypr/scripts/start-shell.sh":        "#!/usr/bin/env bash\n",
		"Linux/dots/hypr/caelestia/keybinds.conf":       "# binds\n",
		"Linux/dots/hypr/caelestia/launcher.conf":       "# launcher\n",
		"Linux/dots/hypr/shell-common-keybinds.conf":    "# common\n",
		"Linux/dots/hypr/shell-workspace-keybinds.conf": "# workspace\n",
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
	if err := os.WriteFile(filepath.Join(repo, "Linux/dots/hypr/caelestia/keybinds.conf"), []byte("# binds\n"), 0o644); err != nil {
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
	if _, statErr := os.Stat(filepath.Join(dest, "mysetup/state.json")); !os.IsNotExist(statErr) {
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
		filepath.Join(nixos, "flake.nix"):                         "{}\n",
		filepath.Join(dots, "hypr", "caelestia", "keybinds.conf"): "# caelestia\n",
		filepath.Join(dots, "hypr", "noctalia", "keybinds.conf"):  "# noctalia\n",
		filepath.Join(dots, "hypr", "scripts", "start-shell.sh"):  "#!/usr/bin/env bash\n",
		filepath.Join(installer, "go.mod"):                        "module test\n",
		filepath.Join(installer, "cmd", "mysetup", "main.go"):     "package main\n",
		filepath.Join(installer, "bin", "mysetup"):                "ignored\n",
		filepath.Join(installer, "coverage.out"):                  "ignored\n",
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
	}, staging, config.Default()); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{
		"dots/hypr/caelestia/keybinds.conf",
		"dots/hypr/noctalia/keybinds.conf",
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
		filepath.Join(dots, "hypr", "keybinds.conf"):            "# binds\n",
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
	}, staging, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"dots/hypr/keybinds.conf",
		"installer/cmd/mysetup/main.go",
		"hosts/NixOS/host-vars.nix",
	} {
		if _, err := os.Stat(filepath.Join(staging, rel)); err != nil {
			t.Fatalf("staging missing %s after readonly NixOS copy: %v", rel, err)
		}
	}
}

func TestFlakeCanUseInstalledInstallerSource(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/flake.nix")
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
	text := string(data) + string(layout) + string(packages)
	for _, want := range []string{
		"layout = import ./lib/layout.nix",
		"installerSource = layout.installer",
		`(nixosRoot + "/installer")`,
		`(nixosRoot + "/../installer")`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("flake mysetup package must support installed /etc/nixos/installer source; missing %q\n%s", want, text)
		}
	}
}

func TestFlakeMySetupWrapperCanRunFromRemoteSource(t *testing.T) {
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
		"mysetupRuntimeSource",
		`cp -a ${nixosSource} "$out/NixOS"`,
		`cp -a ${dotsSource} "$out/dots"`,
		`cp -a ${installerSource} "$out/installer"`,
		"--set MYSETUP_REPO_ROOT ${mysetupRuntimeSource}/NixOS",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("flake mysetup wrapper missing remote source support %q\n%s", want, text)
		}
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

	if err := prepareStagingHostLocal(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, staging, dest, config.Secrets{}); err != nil {
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
	err := prepareStagingHostLocal(context.Background(), runner, staging, dest, config.Secrets{UserPassword: "secret"})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(staging, "hosts", "NixOS", "hashed-password.nix"))
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
	hash := filepath.Join(dest, "hosts", "NixOS", "hashed-password.nix")
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
	if err := prepareStagingHostLocal(context.Background(), runner, staging, dest, config.Secrets{}); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	for _, want := range []string{
		"sudo install -D -m 600",
		"-o",
		"-g",
		hash,
		filepath.Join(staging, "hosts", "NixOS", "hashed-password.nix"),
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("permission denied hash should be staged with sudo install, missing %q\n%s", want, got)
		}
	}
}

func TestWriteStagedHashedPasswordInstallsStagingArtifact(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	source := filepath.Join(staging, "hosts", "NixOS", "hashed-password.nix")
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
	err := writeStagedHashedPassword(context.Background(), runner, staging, dest, config.Secrets{UserPassword: "secret"})
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
		"mysetup/active-shell",
		"mysetup/hypr-runtime",
		`"quickshell/mysetup-shell-selector"`,
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
		StatePath: filepath.Join(dest, "mysetup", "state.json"),
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
