package apply

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

func TestVariablesNixContainsFeatureFlags(t *testing.T) {
	state := config.Default()
	state.Host.Hostname = "workstation"
	state.Features.CTFTools = true
	state.Zapret.Enable = true
	state.Dots.Wallpapers = true

	out := VariablesNix(state)
	for _, want := range []string{
		`hostname = "workstation";`,
		`packagePreset = "personal";`,
		`ctfTools = true;`,
		`enable = true;`,
		`consoleKeyMap = "us";`,
		`keyboardToggle = "grp:alt_shift_toggle";`,
		`wallpapers = {`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("variables.nix missing %q\n%s", want, out)
		}
	}
}

func TestVariablesNixContainsWallpaperFlag(t *testing.T) {
	state := config.Default()
	state.Dots.Wallpapers = false

	out := VariablesNix(state)
	if !strings.Contains(out, "wallpapers = {\n      enable = false;\n    };") {
		t.Fatalf("variables.nix must include disabled wallpapers flag\n%s", out)
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

func TestHostDefaultUsesPathExistsForGeneratedPassword(t *testing.T) {
	out := HostDefaultNix()
	if !strings.Contains(out, "builtins.pathExists ./hashed-password.nix") {
		t.Fatalf("host default should import hashed-password.nix conditionally")
	}
}

func TestHostDefaultPreservesOptionalIDAPackageImports(t *testing.T) {
	out := HostDefaultNix()
	for _, want := range []string{
		"# ../../packages/ida-mcp.nix",
		"# ../../packages/ida-plugins.nix",
		"# ../../packages/ida-pro.nix",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("host default should preserve optional IDA import %q\n%s", want, out)
		}
	}
}

func TestSyncToEtcPreservesLocalHashedPassword(t *testing.T) {
	args := strings.Join(syncToEtcArgs("/tmp/staging", "/etc/nixos"), " ")
	if !strings.Contains(args, "--exclude=hosts/NixOS/hashed-password.nix") {
		t.Fatalf("syncToEtc must preserve host-local hashed-password.nix, got args: %s", args)
	}
}

func TestSeedFlakeLockCopiesOnlyWhenMissing(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(staging, "flake.lock"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := seedFlakeLock(context.Background(), runner, staging, dest); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, "sudo install -m 644") || !strings.Contains(got, "flake.lock") {
		t.Fatalf("expected dry-run install of missing flake.lock, got: %s", got)
	}

	out.Reset()
	if err := os.WriteFile(filepath.Join(dest, "flake.lock"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := seedFlakeLock(context.Background(), runner, staging, dest); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "" {
		t.Fatalf("existing flake.lock must be preserved without command output, got: %s", got)
	}
}

func TestSyncDotsToEtcCopiesExternalDots(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "hypr"), 0o755); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := syncDotsToEtc(context.Background(), runner, src, dest); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "sudo mkdir -p") || !strings.Contains(got, "sudo rsync -a --delete") || !strings.Contains(got, "sudo find") {
		t.Fatalf("expected dots mirror commands, got: %s", got)
	}
}

func TestSyncDotsToEtcSkipsInstalledMirrorSource(t *testing.T) {
	dest := t.TempDir()
	src := filepath.Join(dest, "dots")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := syncDotsToEtc(context.Background(), runner, src, dest); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "" {
		t.Fatalf("same source/destination dots mirror should be a no-op, got: %s", got)
	}
}

func TestRunDryRunDryBuildsStagingBeforeWritingEtcAndStateLast(t *testing.T) {
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

	dryBuild := strings.Index(out, "sudo nixos-rebuild dry-build --flake /tmp/mysetup-nixos-")
	backup := strings.Index(out, "sudo cp -a")
	stateWrite := strings.LastIndex(out, filepath.Join(dest, "mysetup/state.json"))
	noSwitch := strings.Index(out, "dry-build passed; --no-switch set")
	if dryBuild == -1 || backup == -1 || stateWrite == -1 || noSwitch == -1 {
		t.Fatalf("expected dry-build, backup, no-switch and state write output, got:\n%s", out)
	}
	if dryBuild > backup {
		t.Fatalf("dry-build must happen before /etc backup/sync\n%s", out)
	}
	if stateWrite < noSwitch {
		t.Fatalf("state must be written after dots and switch/no-switch success\n%s", out)
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
		"hosts/NixOS/variables.nix",
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

func TestFlakeCanUseInstalledInstallerSource(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/flake.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "if builtins.pathExists ./installer then ./installer else ../installer") {
		t.Fatalf("flake mysetup package must support installed /etc/nixos/installer source\n%s", text)
	}
}

func TestPrepareStagingHostLocalCopiesHardwareAndExistingLock(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
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
		filepath.Join(staging, "flake.lock"):                             "lock\n",
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
	text := string(data)
	for _, script := range []string{
		"close-active.sh",
		"noctalia-launcher.sh",
		"record-toggle.sh",
		"screenshot.sh",
		"spotify-toggle.sh",
		"start-shell.sh",
		"wsaction.fish",
	} {
		if !strings.Contains(text, `"`+script+`"`) {
			t.Fatalf("home/shells/default.nix must install %s\n%s", script, text)
		}
	}
	for _, want := range []string{
		"hyprctl reload",
		"start-shell.sh ${profile}",
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
