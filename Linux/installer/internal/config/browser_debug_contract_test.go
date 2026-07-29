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
	for _, name := range []string{"chrome-debug = ''", "chromium-debug = ''"} {
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
		t.Fatalf("both browser launchers must pass the resolved extension argument\n%s", source)
	}
}
