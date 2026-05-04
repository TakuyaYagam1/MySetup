package apply

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
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
	if !strings.Contains(got, "sudo mkdir -p") || !strings.Contains(got, "sudo rsync -a --delete") {
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
