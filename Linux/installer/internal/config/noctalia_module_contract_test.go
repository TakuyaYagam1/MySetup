package config

import (
	"strings"
	"testing"
)

func TestNoctaliaUpstreamModuleOwnsItsOptionsAcrossHomeManagerVersions(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"../../../../flake.nix",
		"../../../NixOS/home/home.nix",
	} {
		source := readContractFile(t, path)
		for _, want := range []string{
			`disabledModules =`,
			`modulesPath`,
			`builtins.pathExists "${modulesPath}/programs/noctalia.nix"`,
			`"programs/noctalia.nix"`,
			`inputs.noctalia-shell.homeModules.default`,
			`inputs.noctalia.homeModules.default`,
		} {
			if !strings.Contains(source, want) {
				t.Fatalf("%s must keep the version-matched upstream Noctalia module and disable Home Manager's colliding built-in when present: missing %q", path, want)
			}
		}
	}
}
