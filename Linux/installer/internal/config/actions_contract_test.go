package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestGitHubActionsUseVersionTags(t *testing.T) {
	workflowPaths, err := filepath.Glob("../../../../.github/workflows/*.yml")
	if err != nil {
		t.Fatal(err)
	}
	if len(workflowPaths) == 0 {
		t.Fatal("no GitHub Actions workflows found")
	}

	usesAction := regexp.MustCompile(`(?m)^\s*uses:\s+([^\s@]+)@([^\s#]+)`)
	versionTag := regexp.MustCompile(`^v[0-9]+(?:\.[0-9]+){0,2}$`)

	for _, path := range workflowPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)

		for _, match := range usesAction.FindAllStringSubmatch(source, -1) {
			action, ref := match[1], match[2]
			if !versionTag.MatchString(ref) {
				t.Fatalf("%s must use a version tag for %s, got %q", path, action, ref)
			}
		}
	}
}

func TestLintingUsesNixToolchainWithoutApt(t *testing.T) {
	workflow, err := os.ReadFile("../../../../.github/workflows/linting.yml")
	if err != nil {
		t.Fatal(err)
	}
	source := string(workflow)

	for _, forbidden := range []string{"apt-get", "apt install", "sudo apt"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("linting workflow must not depend on APT mirrors: found %q\n%s", forbidden, source)
		}
	}

	for _, required := range []string{
		"timeout-minutes:",
		"nix profile add",
		"nixpkgs#fish",
		"nixpkgs#shellcheck",
		"nixpkgs#jq",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("linting workflow must provide its toolchain through Nix: missing %q\n%s", required, source)
		}
	}
}

func TestPresetFlakeUpdaterUsesDirectoryFlakeReferences(t *testing.T) {
	workflow, err := os.ReadFile("../../../../.github/workflows/update-flake.yml")
	if err != nil {
		t.Fatal(err)
	}
	source := string(workflow)

	for _, want := range []string{
		`update_flake_without_codex "Linux/NixOS/presets/${preset}"`,
		`nix flake update "${input_names[@]}"`,
		`nix eval --raw --no-write-lock-file`,
		`"path:${GITHUB_WORKSPACE}?dir=Linux/NixOS/presets/${preset}#checks.x86_64-linux.preset-host.drvPath"`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("preset updater must use directory flake reference %q\n%s", want, source)
		}
	}

	if strings.Contains(source, `nix flake check --no-build --no-write-lock-file "$preset_flake"`) {
		t.Fatalf("preset checks must evaluate preset-host.drvPath instead of checking unbuilt outputs\n%s", source)
	}
}
