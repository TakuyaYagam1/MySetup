package config

import (
	"os"
	"strings"
	"testing"
)

func TestBrowserDebugFishFunctionsArePersonalOnly(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/home/programs/fish.nix")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)

	for _, want := range []string{
		"personal = wahrweltLib.presets.personal wahrwelt;",
		"personalDebugFunctions = {",
		"lib.optionalAttrs personal personalDebugFunctions",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("personal-only browser debug launcher contract missing %q\n%s", want, source)
		}
	}

	personalStart := strings.Index(source, "personalDebugFunctions = {")
	configStart := strings.Index(source, "programs.fish = {")
	if personalStart < 0 || configStart < 0 || personalStart >= configStart {
		t.Fatalf("browser debug functions must be defined outside the common Fish configuration\n%s", source)
	}

	personalFunctions := source[personalStart:configStart]
	for _, name := range []string{
		"chrome-9222 = ''",
		"chrome-9223 = ''",
	} {
		if !strings.Contains(personalFunctions, name) {
			t.Fatalf("%s must only exist in personalDebugFunctions\n%s", name, source)
		}
		if strings.Contains(source[configStart:], name) {
			t.Fatalf("%s leaked into the common Fish function set\n%s", name, source)
		}
	}

	for _, legacyName := range []string{
		"chromium-debug = ''",
		"chrome-debug = ''",
		"chromium-9222 = ''",
		"chromium-9223 = ''",
	} {
		if strings.Contains(source, legacyName) {
			t.Fatalf("legacy browser debug alias %s must not be generated\n%s", legacyName, source)
		}
	}
}

func TestBrowserDebugFishFunctionsDoNotLoadChatGPTExtension(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/home/programs/fish.nix")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)

	for _, forbidden := range []string{
		`$HOME/.config/google-chrome/Default/Extensions/hehggadaopoacecdllhhajmbjkdcmajg`,
		`find "$chatgpt_root" -mindepth 1 -maxdepth 1 -type d`,
		"sort -V",
		`test -f "$chatgpt_extension/manifest.json"`,
		`set extension_args "--load-extension=$chatgpt_extension"`,
		"$extension_args $argv",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("browser debug launchers must not inject the ChatGPT extension via %q\n%s", forbidden, source)
		}
	}
}

func TestBrowserDebugFishFunctionsUseGoogleChromeOnBothPorts(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/home/programs/fish.nix")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)

	for _, want := range []string{
		"chrome-9222 = ''",
		"set -l port 9222",
		`--user-data-dir="$HOME/.chromium-debug-9222-profile"`,
		"chrome-9223 = ''",
		"set -l port 9223",
		`--user-data-dir="$HOME/.chromium-debug-9223-profile"`,
		"google-chrome-stable \\",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("dual Chromium launcher contract missing %q\n%s", want, source)
		}
	}

	for _, forbidden := range []string{
		"chromium-9222 = ''",
		"chromium-9223 = ''",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("legacy Chromium launcher %q must not be generated\n%s", forbidden, source)
		}
	}
}
