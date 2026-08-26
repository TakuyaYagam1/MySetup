package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/apply"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
)

func TestApplyMissingStateFailsClosed(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--state", filepath.Join(t.TempDir(), "missing.json"),
		"apply",
		"--dry-run",
		"--no-switch",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected apply to fail when state is missing")
	}
	if !strings.Contains(err.Error(), "state file does not exist") {
		t.Fatalf("expected missing state error, got %v", err)
	}
}

func TestPrintStateMissingStateFailsClosed(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--state", filepath.Join(t.TempDir(), "missing.json"),
		"print-state",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected print-state to fail when state is missing")
	}
	if !strings.Contains(err.Error(), "state file does not exist") {
		t.Fatalf("expected missing state error, got %v", err)
	}
}

func TestLegacyInstallPathsReportsOnlyExistingManagedPaths(t *testing.T) {
	home := t.TempDir()
	dest := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	legacyState := filepath.Join(dest, "mysetup")
	legacyUser := filepath.Join(dest, "private")
	legacyInstallerState := filepath.Join(dest, "wahrwelt", "state.json")
	legacyConfig := filepath.Join(home, ".config", "hypr", "mysetup")
	legacyHyprUser := filepath.Join(home, ".config", "hypr", "wahrwelt")
	canonicalConfig := filepath.Join(home, ".config", "wahrwelt")
	for _, path := range []string{legacyState, legacyUser, legacyConfig, legacyHyprUser, canonicalConfig} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(legacyInstallerState), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyInstallerState, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := legacyInstallPaths(&Options{Options: paths.Options{NixOSDest: dest}})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{legacyState, legacyUser, legacyInstallerState, legacyConfig, legacyHyprUser} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected legacy path %q in result %v", want, got)
		}
	}
	if strings.Contains(joined, canonicalConfig) {
		t.Fatalf("canonical path must not be reported as legacy: %v", got)
	}
}

type migrationCommandRunner struct {
	calls    []string
	onSystem func() error
	onHome   func() error
}

func (r *migrationCommandRunner) Command(_ context.Context, name string, args ...string) error {
	r.calls = append(r.calls, strings.Join(append([]string{name}, args...), " "))
	if name == "sudo" && len(args) == 3 && args[0] == "systemctl" && args[1] == "restart" {
		switch args[2] {
		case "wahrwelt-brand-migration.service":
			if r.onSystem != nil {
				return r.onSystem()
			}
		default:
			if r.onHome != nil {
				return r.onHome()
			}
		}
	}
	return nil
}

func (*migrationCommandRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (*migrationCommandRunner) IsDryRun() bool { return false }

var _ run.CommandRunner = (*migrationCommandRunner)(nil)

func TestRunMigrationRestartsOneshotAndVerifiesNewLegacyNamespaces(t *testing.T) {
	home := t.TempDir()
	dest := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	legacyUser := filepath.Join(dest, "private")
	legacyInstallerState := filepath.Join(dest, "wahrwelt", "state.json")
	legacyHyprUser := filepath.Join(home, ".config", "hypr", "wahrwelt")
	for _, path := range []string{legacyUser, filepath.Dir(legacyInstallerState), legacyHyprUser} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(legacyInstallerState, []byte("legacy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(dest, "installer-state.json")
	state := config.Default()
	state.User.HomeDirectory = home
	if err := config.Save(statePath, state); err != nil {
		t.Fatal(err)
	}
	opts := &Options{Options: paths.Options{NixOSDest: dest, StatePath: statePath}}
	runner := &migrationCommandRunner{
		onSystem: func() error {
			if err := os.RemoveAll(legacyUser); err != nil {
				return err
			}
			if err := os.Remove(legacyInstallerState); err != nil {
				return err
			}
			return os.Remove(filepath.Dir(legacyInstallerState))
		},
		onHome: func() error {
			return os.RemoveAll(legacyHyprUser)
		},
	}

	if err := runMigration(context.Background(), opts, runner); err != nil {
		t.Fatal(err)
	}
	commands := strings.Join(runner.calls, "\n")
	if !strings.Contains(commands, "sudo systemctl restart wahrwelt-brand-migration.service") {
		t.Fatalf("migration must force the already-active oneshot to rerun:\n%s", commands)
	}
}

func TestRunMigrationRejectsFalseGreenWhenLegacyNamespaceRemains(t *testing.T) {
	home := t.TempDir()
	dest := t.TempDir()
	t.Setenv("HOME", home)
	legacyUser := filepath.Join(dest, "private")
	if err := os.Mkdir(legacyUser, 0o755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(dest, "installer-state.json")
	state := config.Default()
	state.User.HomeDirectory = home
	if err := config.Save(statePath, state); err != nil {
		t.Fatal(err)
	}

	err := runMigration(context.Background(), &Options{Options: paths.Options{
		NixOSDest: dest,
		StatePath: statePath,
	}}, &migrationCommandRunner{})
	if err == nil || !strings.Contains(err.Error(), "migration incomplete") {
		t.Fatalf("runMigration() error = %v, want remaining-legacy failure", err)
	}
}

func TestLegacyInstallPathsDoesNotTreatPermissionDeniedAsAbsent(t *testing.T) {
	home := t.TempDir()
	dest := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	restricted := filepath.Join(dest, "wahrwelt")
	if err := os.Mkdir(restricted, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(restricted, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(restricted, 0o700); err != nil && !os.IsNotExist(err) {
			t.Errorf("restore restricted directory mode: %v", err)
		}
	})

	_, err := legacyInstallPaths(&Options{Options: paths.Options{NixOSDest: dest}})
	if err == nil {
		// Root can traverse mode 000, so this executable regression is relevant
		// only when the test process is subject to ordinary directory permissions.
		if os.Geteuid() == 0 {
			t.Skip("root bypasses the permission-denied fixture")
		}
		t.Fatal("permission-denied legacy path was treated as absent")
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("legacyInstallPaths() error = %v, want permission failure", err)
	}
}

func TestApplyRejectsPlainPasswordFlags(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"apply",
		"--user-password",
		"secret",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected plain password flag to be rejected")
	}
	if !strings.Contains(err.Error(), "unknown flag: --user-password") {
		t.Fatalf("expected unknown flag error, got %v", err)
	}
}

func TestApplySourceChannelFlagIsValidated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := config.Save(path, config.Default()); err != nil {
		t.Fatal(err)
	}

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--state", path,
		"apply",
		"--source-channel", "nightly",
		"--dry-run",
		"--no-switch",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected invalid source channel to fail")
	}
	if !strings.Contains(err.Error(), "source channel") {
		t.Fatalf("expected source channel validation error, got %v", err)
	}
}

func TestLoadApplyStatePreservesProofAcrossSourceChannelEdit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := config.Save(path, config.Default()); err != nil {
		t.Fatal(err)
	}

	state, proof, err := loadApplyState(
		paths.Options{StatePath: path},
		config.SourceChannelDevelopment,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := state.Source.Channel, config.SourceChannelDevelopment; got != want {
		t.Fatalf("source channel = %q, want %q", got, want)
	}
	if reflect.DeepEqual(proof, apply.LoadedStateProof{}) {
		t.Fatal("source-channel edit discarded the loaded state ownership proof")
	}
}

func TestLoadDoctorStateFallsBackOnInvalidState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{ nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	state, err := loadDoctorState(path)
	if err != nil {
		t.Fatal(err)
	}
	if state.Packages.Preset != "personal" {
		t.Fatalf("expected default doctor fallback state, got %#v", state)
	}
}

func TestMakeCheckCleansBuildArtifact(t *testing.T) {
	data, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "$(MAKE) clean") {
		t.Fatalf("check target should run clean after build\n%s", text)
	}
	if strings.Contains(text, "check: fmt-check lint vet test build nix-build") {
		t.Fatalf("check target should not use dependency form that leaves bin/mysetup behind\n%s", text)
	}
}

func TestMakeCheckRunsHomeManagerShellEval(t *testing.T) {
	makefile, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	helpers, err := os.ReadFile("../../scripts/nix-eval-helpers.sh")
	if err != nil {
		t.Fatal(err)
	}
	makefileText := string(makefile)
	if !strings.Contains(makefileText, "nix-hm-eval") {
		t.Fatalf("make check should include HM shell eval target\n%s", makefileText)
	}
	if !strings.Contains(makefileText, "scripts/nix-eval-helpers.sh") {
		t.Fatalf("nix-hm-eval target should source the shared helper script\n%s", makefileText)
	}
	combined := makefileText + string(helpers)
	if !strings.Contains(combined, "seedHyprShellRuntime") || !strings.Contains(combined, "hypr/shell-profile.lua") {
		t.Fatalf("nix-hm-eval should cover the runtime shell module\n%s", combined)
	}
	if !strings.Contains(combined, "hypr/end4") {
		t.Fatalf("nix-hm-eval should also validate the end4 Home Manager profile\n%s", combined)
	}
}

func TestMakeCheckRunsRepoRootPackageEval(t *testing.T) {
	data, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"nix-package-eval",
		`builtins.getFlake "path:$(REPO_ROOT)?dir=Linux/NixOS"`,
		"flake.packages.x86_64-linux.wahrwelt.pname",
		"flake.packages.x86_64-linux.mysetup.pname",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("make check should eval package through repo-root flake path %q\n%s", want, text)
		}
	}
}

func TestReadmeUsesRepoRootFlakePathForLocalRun(t *testing.T) {
	data, err := os.ReadFile("../../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "nix run ./Linux/NixOS#wahrwelt") {
		t.Fatalf("README should not recommend local ./Linux/NixOS flake path\n%s", text)
	}
	if !strings.Contains(text, `nix run "path:$PWD?dir=Linux/NixOS#wahrwelt"`) {
		t.Fatalf("README should document repo-root path flake local run\n%s", text)
	}
}

//nolint:misspell // Makefile must keep Nix's real extra-substituters option.
func TestMakeCheckRunsInstalledMirrorBuild(t *testing.T) {
	data, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"nix-installed-mirror-build",
		`rsync -a ../NixOS/ "$$tmp"/`,
		`rsync -a ../dots/ "$$tmp"/dots/`,
		`rsync -a --exclude=/bin/ --exclude=/tmp/ --exclude=/coverage.out --exclude=/coverage.html ./ "$$tmp"/installer/`,
		`NIX_CACHE_ARGS := --option extra-substituters`,
		`nix build $(NIX_CACHE_ARGS) --no-link --no-write-lock-file "path:$$tmp#wahrwelt"`,
		`nix build $(NIX_CACHE_ARGS) --no-link --no-write-lock-file "path:$$tmp#mysetup"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("make check should include installed mirror build evidence %q\n%s", want, text)
		}
	}
}
