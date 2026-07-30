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
		"chromium-9222 = ''",
		"chromium-9223 = ''",
		"chromium-debug = ''",
		"chrome-debug = ''",
	} {
		if !strings.Contains(personalFunctions, name) {
			t.Fatalf("%s must only exist in personalDebugFunctions\n%s", name, source)
		}
		if strings.Contains(source[configStart:], name) {
			t.Fatalf("%s leaked into the common Fish function set\n%s", name, source)
		}
	}
}

func TestBrowserDebugFishFunctionsLoadLatestChatGPTExtension(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/home/programs/fish.nix")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)

	for _, want := range []string{
		`$HOME/.config/google-chrome/Default/Extensions/hehggadaopoacecdllhhajmbjkdcmajg`,
		`find "$chatgpt_root" -mindepth 1 -maxdepth 1 -type d`,
		"sort -V",
		`test -f "$chatgpt_extension/manifest.json"`,
		`set extension_args "--load-extension=$chatgpt_extension"`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("dynamic ChatGPT extension loading contract missing %q\n%s", want, source)
		}
	}

	if strings.Count(source, "$extension_args $argv") != 2 {
		t.Fatalf("both canonical Chromium launchers must pass the resolved extension argument\n%s", source)
	}
}

func TestBrowserDebugFishFunctionsUseChromiumOnBothPorts(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/home/programs/fish.nix")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)

	for _, want := range []string{
		"chromium-9222 = ''",
		"set -l port 9222",
		`--user-data-dir="$HOME/.chromium-debug-9222-profile"`,
		"chromium-9223 = ''",
		"set -l port 9223",
		`--user-data-dir="$HOME/.chromium-debug-9223-profile"`,
		"chromium-debug = ''",
		"chromium-9222 $argv",
		"chrome-debug = ''",
		"chromium-9223 $argv",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("dual Chromium launcher contract missing %q\n%s", want, source)
		}
	}

	if strings.Contains(source, "google-chrome-stable") {
		t.Fatalf("browser debug launchers must not use branded Google Chrome\n%s", source)
	}
}
