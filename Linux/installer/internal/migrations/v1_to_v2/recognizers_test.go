package v1_to_v2

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLegacyInstallPathsAreExactAndVersionScoped(t *testing.T) {
	got := LegacyInstallPaths("/etc/nixos", "/home/alice/.config", "/home/alice/.local/state", "/home/alice/.cache")
	want := []string{
		"/etc/nixos/mysetup",
		"/etc/nixos/private",
		"/etc/nixos/wahrwelt/state.json",
		"/home/alice/.config/mysetup",
		"/home/alice/.config/hypr/mysetup",
		"/home/alice/.config/hypr/wahrwelt",
		"/home/alice/.config/hypr/lib/mysetup.lua",
		"/home/alice/.config/quickshell/mysetup-shell-selector",
		"/home/alice/.local/state/mysetup",
		"/home/alice/.cache/mysetup",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LegacyInstallPaths() = %v, want %v", got, want)
	}
}

func TestRewritePrivateUserPathTokenIsExact(t *testing.T) {
	tests := map[string]string{
		"./private":              "./user",
		"./private/default.nix":  "./user/default.nix",
		"./private-extra/module": "./private-extra/module",
		"private/default.nix":    "private/default.nix",
		"./user/default.nix":     "./user/default.nix",
	}
	for input, want := range tests {
		if got := RewritePrivateUserPathToken(input); got != want {
			t.Errorf("RewritePrivateUserPathToken(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestEntrypointRecognizersMatchVersionedFixtures(t *testing.T) {
	fixtureDir := "../../../../NixOS/home/migrations/v1_to_v2/hypr-runtime"
	tests := []struct {
		name string
		kind EntrypointKind
	}{
		{name: "user-namespace-transition.lua", kind: EntrypointNamespaceTransition},
		{name: "end4.lua", kind: EntrypointDirectEnd4},
		{name: "end4-pc.lua", kind: EntrypointDirectEnd4PC},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(fixtureDir, test.name))
			if err != nil {
				t.Fatal(err)
			}
			if got := RecognizeEntrypoint(string(data), "caelestia"); got != test.kind {
				t.Fatalf("RecognizeEntrypoint(%s) = %v, want %v", test.name, got, test.kind)
			}
			data = append(data, '\n')
			if got := RecognizeEntrypoint(string(data), "caelestia"); got != EntrypointUnknown {
				t.Fatalf("modified %s recognized as %v", test.name, got)
			}
		})
	}
}

func TestHistoricalManagedEntrypointDigestRecognizesOnlyExactPayload(t *testing.T) {
	fixture := filepath.Join("../../../../NixOS/home/migrations/v1_to_v2/hypr-runtime", "user-adapter-mysetup-v1.lua")
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !IsHistoricalManagedHyprEntrypointDigest(sha256.Sum256(data)) {
		t.Fatal("exact historical adapter digest was not recognized")
	}
	data = append(data, '\n')
	if IsHistoricalManagedHyprEntrypointDigest(sha256.Sum256(data)) {
		t.Fatal("modified historical adapter digest was recognized")
	}
}

func TestCurrentCanonicalEntrypointIsNotAV1Recognizer(t *testing.T) {
	data, err := os.ReadFile("../../../../dots/hypr/hyprland.lua")
	if err != nil {
		t.Fatal(err)
	}
	if got := RecognizeEntrypoint(string(data), "caelestia"); got != EntrypointUnknown {
		t.Fatalf("fresh canonical entrypoint recognized as v1: %v", got)
	}
}
