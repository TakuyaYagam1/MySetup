package config

import (
	"os"
	"strings"
	"testing"
)

func TestKimiCLIIsLimitedToDeveloperAndPersonalPresets(t *testing.T) {
	presetInputs := readKimiCLIContractFile(t, "../../../NixOS/lib/preset-inputs.nix")
	developerInputsStart := strings.Index(presetInputs, "developerInputs = {")
	personalInputsStart := strings.Index(presetInputs, "personalInputs = {")
	if developerInputsStart < 0 || personalInputsStart < 0 || developerInputsStart >= personalInputsStart {
		t.Fatalf("could not isolate developer flake inputs\n%s", presetInputs)
	}
	developerInputs := presetInputs[developerInputsStart:personalInputsStart]
	for _, want := range []string{
		"kimi-cli = {",
		`url = "github:MoonshotAI/kimi-cli";`,
		`inputs.nixpkgs.follows = "nixpkgs";`,
	} {
		if !strings.Contains(developerInputs, want) {
			t.Fatalf("developer preset Kimi CLI contract missing %q\n%s", want, presetInputs)
		}
	}

	for _, preset := range []string{"developer", "personal"} {
		flake := readKimiCLIContractFile(t, "../../../NixOS/presets/"+preset+"/flake.nix")
		if !strings.Contains(flake, `url = "github:MoonshotAI/kimi-cli";`) {
			t.Fatalf("%s preset must include the Kimi CLI flake input\n%s", preset, flake)
		}
	}
	for _, preset := range []string{"minimal", "desktop"} {
		flake := readKimiCLIContractFile(t, "../../../NixOS/presets/"+preset+"/flake.nix")
		if strings.Contains(flake, "kimi-cli") {
			t.Fatalf("%s preset must not include the Kimi CLI flake input\n%s", preset, flake)
		}
	}

	overlay := readKimiCLIContractFile(t, "../../../NixOS/lib/flake-overlays.nix")
	if !strings.Contains(overlay, "kimi-cli = inputs.kimi-cli.packages.${system}.default;") {
		t.Fatalf("developer package overlay must expose Kimi CLI\n%s", overlay)
	}

	homePackages := readKimiCLIContractFile(t, "../../../NixOS/lib/package-sets/home.nix")
	devStart := strings.Index(homePackages, "dev = with pkgs; [")
	personalStart := strings.Index(homePackages, "personal = with pkgs; [")
	if devStart < 0 || personalStart < 0 || devStart >= personalStart {
		t.Fatalf("could not isolate the developer package set\n%s", homePackages)
	}
	if !strings.Contains(homePackages[devStart:personalStart], "kimi-cli") {
		t.Fatalf("Kimi CLI must be installed by developer and inherited by personal\n%s", homePackages)
	}
}

func readKimiCLIContractFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
