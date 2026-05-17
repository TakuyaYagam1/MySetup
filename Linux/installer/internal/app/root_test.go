package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if strings.Contains(text, "nix run ./Linux/NixOS#mysetup") {
		t.Fatalf("README should not recommend local ./Linux/NixOS flake path\n%s", text)
	}
	if !strings.Contains(text, `nix run "path:$PWD?dir=Linux/NixOS#mysetup"`) {
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
		`nix build $(NIX_CACHE_ARGS) --no-link --no-write-lock-file "path:$$tmp#mysetup"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("make check should include installed mirror build evidence %q\n%s", want, text)
		}
	}
}
