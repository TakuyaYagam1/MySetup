package config

import (
	"encoding/json"
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
		"kimi-code = {",
		`url = "github:MoonshotAI/kimi-code";`,
	} {
		if !strings.Contains(developerInputs, want) {
			t.Fatalf("developer preset Kimi CLI contract missing %q\n%s", want, presetInputs)
		}
	}
	for _, preset := range []string{"developer", "personal"} {
		flake := readKimiCLIContractFile(t, "../../../NixOS/presets/"+preset+"/flake.nix")
		if !strings.Contains(flake, `url = "github:MoonshotAI/kimi-code";`) {
			t.Fatalf("%s preset must include the Kimi CLI flake input\n%s", preset, flake)
		}
	}
	for _, preset := range []string{"minimal", "desktop"} {
		flake := readKimiCLIContractFile(t, "../../../NixOS/presets/"+preset+"/flake.nix")
		if strings.Contains(flake, "kimi-code") {
			t.Fatalf("%s preset must not include the Kimi CLI flake input\n%s", preset, flake)
		}
	}

	overlay := readKimiCLIContractFile(t, "../../../NixOS/lib/flake-overlays.nix")
	if !strings.Contains(overlay, "kimi-code = inputs.kimi-code.packages.${system}.default;") {
		t.Fatalf("developer package overlay must expose Kimi CLI\n%s", overlay)
	}

	homePackages := readKimiCLIContractFile(t, "../../../NixOS/lib/package-sets/home.nix")
	devStart := strings.Index(homePackages, "dev = with pkgs; [")
	personalStart := strings.Index(homePackages, "personal = with pkgs; [")
	if devStart < 0 || personalStart < 0 || devStart >= personalStart {
		t.Fatalf("could not isolate the developer package set\n%s", homePackages)
	}
	if !strings.Contains(homePackages[devStart:personalStart], "kimi-code") {
		t.Fatalf("Kimi CLI must be installed by developer and inherited by personal\n%s", homePackages)
	}
}

func TestKimiCLIUsesUpstreamNixpkgsCompatibilityPin(t *testing.T) {
	for _, path := range []string{
		"../../../../flake.nix",
		"../../../NixOS/flake.nix",
		"../../../NixOS/lib/preset-inputs.nix",
		"../../../NixOS/presets/developer/flake.nix",
		"../../../NixOS/presets/personal/flake.nix",
	} {
		source := readKimiCLIContractFile(t, path)
		if strings.Contains(kimiCLIInputBlock(t, source), `inputs.nixpkgs.follows = "nixpkgs";`) {
			t.Fatalf("%s must keep Kimi CLI on its upstream Nixpkgs compatibility pin\n%s", path, source)
		}
	}
}

func TestKimiCLILocksUseUpstreamNixpkgsCompatibilityPin(t *testing.T) {
	var expectedKimi flakeLockSource
	var expectedNixpkgs flakeLockSource
	for index, path := range []string{
		"../../../../flake.lock",
		"../../../NixOS/flake.lock",
		"../../../NixOS/presets/developer/flake.lock",
		"../../../NixOS/presets/personal/flake.lock",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var lock struct {
			Nodes map[string]flakeLockNode `json:"nodes"`
		}
		if err := json.Unmarshal(data, &lock); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		kimi, ok := lock.Nodes["kimi-code"]
		if !ok {
			t.Fatalf("%s is missing the Kimi CLI lock node", path)
		}
		kimiNixpkgsTarget := readLockInputTarget(t, path, kimi, "nixpkgs")
		rootNixpkgsTarget := readLockInputTarget(t, path, lock.Nodes["root"], "nixpkgs")
		if kimiNixpkgsTarget == rootNixpkgsTarget {
			t.Fatalf("%s must not route Kimi CLI through the root Nixpkgs node %q", path, rootNixpkgsTarget)
		}
		kimiNixpkgs, ok := lock.Nodes[kimiNixpkgsTarget]
		if !ok {
			t.Fatalf("%s Kimi CLI references missing Nixpkgs node %q", path, kimiNixpkgsTarget)
		}
		original := kimiNixpkgs.Original
		if original.Owner != "NixOS" || original.Repo != "nixpkgs" || original.Ref != "nixos-25.11" {
			t.Fatalf("%s Kimi CLI Nixpkgs target must use the upstream nixos-25.11 pin, got %s/%s/%s", path, original.Owner, original.Repo, original.Ref)
		}

		if index == 0 {
			expectedKimi = kimi.Locked
			expectedNixpkgs = kimiNixpkgs.Locked
			continue
		}
		if kimi.Locked.Rev != expectedKimi.Rev || kimi.Locked.NarHash != expectedKimi.NarHash {
			t.Fatalf("%s Kimi CLI pin differs from the root lock", path)
		}
		if kimiNixpkgs.Locked.Rev != expectedNixpkgs.Rev || kimiNixpkgs.Locked.NarHash != expectedNixpkgs.NarHash {
			t.Fatalf("%s Kimi CLI Nixpkgs pin differs from the root lock", path)
		}
	}
}

type flakeLockNode struct {
	Inputs   map[string]json.RawMessage `json:"inputs"`
	Locked   flakeLockSource            `json:"locked"`
	Original flakeLockSource            `json:"original"`
}

type flakeLockSource struct {
	NarHash string `json:"narHash"`
	Owner   string `json:"owner"`
	Ref     string `json:"ref"`
	Repo    string `json:"repo"`
	Rev     string `json:"rev"`
}

func readLockInputTarget(t *testing.T, path string, node flakeLockNode, input string) string {
	t.Helper()

	raw, ok := node.Inputs[input]
	if !ok {
		t.Fatalf("%s lock node is missing input %q", path, input)
	}
	var target string
	if err := json.Unmarshal(raw, &target); err != nil {
		t.Fatalf("%s input %q must reference an independent lock node: %v", path, input, err)
	}
	return target
}

func kimiCLIInputBlock(t *testing.T, source string) string {
	t.Helper()

	const startMarker = "kimi-code = {"
	start := strings.Index(source, startMarker)
	if start < 0 {
		t.Fatalf("Kimi CLI input block is missing\n%s", source)
	}

	rest := source[start:]
	end := strings.Index(rest, "\n    };")
	if end < 0 {
		t.Fatalf("Kimi CLI input block is malformed\n%s", source)
	}

	return rest[:end]
}

func readKimiCLIContractFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
