package config

import (
	"os"
	"strings"
	"testing"
)

func readContractFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
func TestEnd4PCInputIsOwnedByDesktopAndLargerPresets(t *testing.T) {
	for _, path := range []string{
		"../../../../flake.nix",
		"../../../NixOS/flake.nix",
		"../../../NixOS/lib/preset-inputs.nix",
		"../../../NixOS/presets/desktop/flake.nix",
		"../../../NixOS/presets/developer/flake.nix",
		"../../../NixOS/presets/personal/flake.nix",
	} {
		source := readContractFile(t, path)
		for _, want := range []string{
			"end4-pc",
			"github:pctrade/end4-pC",
			"flake = false;",
		} {
			if !strings.Contains(source, want) {
				t.Fatalf("%s must own the end4-pC non-flake input: missing %q", path, want)
			}
		}
	}

	minimal := readContractFile(t, "../../../NixOS/presets/minimal/flake.nix")
	if strings.Contains(minimal, "end4-pc") || strings.Contains(minimal, "end4-pC") {
		t.Fatalf("minimal preset must not include end4-pC\n%s", minimal)
	}
}

func TestEnd4PCQuickshellIsImmutableAndSharesEnd4Config(t *testing.T) {
	module := readContractFile(t, "../../../NixOS/home/end4/patches/quickshell-pc.nix")

	for _, want := range []string{
		`inputs.end4-pc`,
		`xdg.configFile."quickshell/end4-pC"`,
		`source = "${patchedQuickshellPC}";`,
		`~/.config/illogical-impulse/config.json`,
		`Wahrwelt manages end4-pC updates through the flake input`,
	} {
		if !strings.Contains(module, want) {
			t.Fatalf("end4-pC module must provide immutable shared-config integration: missing %q\n%s", want, module)
		}
	}

	if strings.Contains(module, "git clone https://github.com/pctrade/end4-pC") {
		t.Fatalf("the installed end4-pC source must not retain its mutable self-updater\n%s", module)
	}
}
