package config

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestOmniRouterUsesConstrainedBuildSettings(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/pkgs/omnirouter.nix")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)

	for _, want := range []string{
		`OMNIROUTE_USE_TURBOPACK = "0";`,
		`OMNIROUTE_BUILD_MEMORY_MB = "4096";`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("OmniRouter must declare constrained build setting %q\n%s", want, source)
		}
	}
}

func TestGlancesSkipsSandboxIncompatibleUpstreamChecks(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/lib/package-sets/system.nix")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)

	glancesOverride := regexp.MustCompile(`glances\.overridePythonAttrs\s*\(_:\s*\{\s*doCheck\s*=\s*false;\s*\}\)`)
	if !glancesOverride.MatchString(source) {
		t.Fatalf("glances must skip its sandbox-incompatible upstream integration checks\n%s", source)
	}
}
