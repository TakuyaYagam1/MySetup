package config

import (
	"strings"
	"testing"
)

func TestHappIsPersonalOnlyAndDoesNotEnableDaemon(t *testing.T) {
	presetInputs := readClaudeDesktopContractFile(t, "../../../NixOS/lib/preset-inputs.nix")
	for _, want := range []string{
		"personalInputs = {",
		"happ-nix = {",
		`url = "github:DaHL-gh/happ-nix";`,
		"if personalOrFull then personalInputs",
	} {
		if !strings.Contains(presetInputs, want) {
			t.Fatalf("personal Happ input contract missing %q\n%s", want, presetInputs)
		}
	}

	overlay := readClaudeDesktopContractFile(t, "../../../NixOS/lib/flake-overlays.nix")
	if !strings.Contains(overlay, "happ = inputs.happ-nix.packages.${system}.happ;") {
		t.Fatalf("personal overlay must expose the Happ package\n%s", overlay)
	}

	homePackages := readClaudeDesktopContractFile(t, "../../../NixOS/lib/package-sets/home.nix")
	personalStart := strings.Index(homePackages, "personal = with pkgs; [")
	gamesStart := strings.Index(homePackages, "games = with pkgs; [")
	if personalStart < 0 || gamesStart < 0 || personalStart >= gamesStart {
		t.Fatalf("could not isolate the personal package set\n%s", homePackages)
	}
	if !strings.Contains(homePackages[personalStart:gamesStart], "happ") {
		t.Fatalf("Happ must be installed by the personal package set\n%s", homePackages)
	}

	for _, path := range []string{
		"../../../NixOS/presets/minimal/flake.nix",
		"../../../NixOS/presets/desktop/flake.nix",
		"../../../NixOS/presets/developer/flake.nix",
	} {
		flake := readClaudeDesktopContractFile(t, path)
		if strings.Contains(flake, "happ-nix") {
			t.Fatalf("Happ must not be present in %s", path)
		}
	}

	stack := readClaudeDesktopContractFile(t, "../../../NixOS/modules/mysetup-stack.nix")
	if strings.Contains(stack, "happ") {
		t.Fatalf("Happ daemon module must not be imported\n%s", stack)
	}
}
