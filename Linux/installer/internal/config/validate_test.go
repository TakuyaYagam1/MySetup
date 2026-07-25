package config

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestValidateDefault(t *testing.T) {
	if err := Validate(Default()); err != nil {
		t.Fatalf("default state should validate: %v", err)
	}
}

func TestDefaultPrefersSudoUserOverRoot(t *testing.T) {
	t.Setenv("USER", "root")
	t.Setenv("SUDO_USER", "takuya")

	state := Default()
	if state.User.Username != "takuya" {
		t.Fatalf("expected sudo user as default username, got %q", state.User.Username)
	}
	if state.User.HomeDirectory != "/home/takuya" {
		t.Fatalf("expected sudo user home directory, got %q", state.User.HomeDirectory)
	}
}

func TestValidateRejectsBadUsername(t *testing.T) {
	state := Default()
	state.User.Username = "Bad User"
	if err := Validate(state); err == nil {
		t.Fatal("expected invalid username error")
	}
}

func TestValidateRejectsBadPackagePreset(t *testing.T) {
	state := Default()
	state.Packages.Preset = "everything"
	if err := Validate(state); err == nil {
		t.Fatal("expected invalid package preset error")
	}
}

func TestValidateRejectsBadStateVersion(t *testing.T) {
	state := Default()
	state.Host.StateVersion = "latest"
	if err := Validate(state); err == nil {
		t.Fatal("expected invalid state version error")
	}
}

func TestValidateRejectsBadSourceChannel(t *testing.T) {
	state := Default()
	state.Source.Channel = "nightly"
	if err := Validate(state); err == nil {
		t.Fatal("expected invalid source channel error")
	}
}

func TestMySetupFlakeURLChannels(t *testing.T) {
	tests := map[string]string{
		SourceChannelStable:      "github:TakuyaYagam1/MySetup/main?dir=Linux/NixOS",
		SourceChannelDevelopment: "github:TakuyaYagam1/MySetup/dev?dir=Linux/NixOS",
		"":                       "github:TakuyaYagam1/MySetup/main?dir=Linux/NixOS",
	}
	for channel, want := range tests {
		if got := MySetupFlakeURL(channel); got != want {
			t.Fatalf("MySetupFlakeURL(%q) = %q, want %q", channel, got, want)
		}
	}
}

func TestKnownMySetupFlakeURLsIncludeLegacyStableURL(t *testing.T) {
	urls := KnownMySetupFlakeURLs()
	for _, want := range []string{
		"github:TakuyaYagam1/MySetup/main?dir=Linux/NixOS",
		"github:TakuyaYagam1/MySetup?dir=Linux/NixOS",
		"github:TakuyaYagam1/MySetup/dev?dir=Linux/NixOS",
		"github:TakuyaYagam1/MySetup/noctalia-v4?dir=Linux/NixOS",
	} {
		if !containsString(urls, want) {
			t.Fatalf("known MySetup URLs must include %q, got %#v", want, urls)
		}
	}
}

func TestNoctaliaFlakeURLs(t *testing.T) {
	if got := NoctaliaV5FlakeURL(); got != "github:noctalia-dev/noctalia/v5.0.0-beta.4" {
		t.Fatalf("NoctaliaV5FlakeURL() = %q", got)
	}
	if got := NoctaliaV4FlakeURL(); got != "github:noctalia-dev/noctalia-shell/v4.7.7" {
		t.Fatalf("NoctaliaV4FlakeURL() = %q", got)
	}
}

func TestCaelestiaFlakeURLs(t *testing.T) {
	if got := CaelestiaShellFlakeURL(); got != "github:caelestia-dots/shell/v2.2.0" {
		t.Fatalf("CaelestiaShellFlakeURL() = %q", got)
	}
	if got := CaelestiaCliFlakeURL(); got != "github:caelestia-dots/cli/v1.1.2" {
		t.Fatalf("CaelestiaCliFlakeURL() = %q", got)
	}
}

func TestValidateRejectsBadNoctaliaVersion(t *testing.T) {
	state := Default()
	state.Noctalia.Version = "v6"
	if err := Validate(state); err == nil {
		t.Fatal("expected invalid noctalia version error")
	}
}

func TestValidateRejectsBadKeyboardSettings(t *testing.T) {
	tests := map[string]func(*State){
		"console keymap": func(state *State) {
			state.Locale.ConsoleKeyMap = "../us"
		},
		"layouts": func(state *State) {
			state.Locale.KeyboardLayouts = "us, ru"
		},
		"toggle": func(state *State) {
			state.Locale.KeyboardToggle = "grp:unknown_toggle"
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			state := Default()
			mutate(&state)
			if err := Validate(state); err == nil {
				t.Fatal("expected invalid keyboard setting error")
			}
		})
	}
}

func TestValidateMonitorLine(t *testing.T) {
	valid := []string{
		"eDP-1, 2560x1600@120, 0x0, 1",
		",preferred,auto,1",
		",preferred,auto,1.5",
		",auto,auto,1",
		"eDP-1, 2560x1600@120, 0x0, 1.25",
	}
	for _, line := range valid {
		if err := ValidateMonitorLine(line); err != nil {
			t.Fatalf("expected %q to validate: %v", line, err)
		}
	}
	invalid := []string{
		"eDP-1, lol, 0x0, 1",
		"eDP-1, , 0x0, 1",
		"eDP-1, 2560x1600@120, 0x0",
	}
	for _, line := range invalid {
		if err := ValidateMonitorLine(line); err == nil {
			t.Fatalf("expected %q to fail validation", line)
		}
	}
}

func TestValidateExtraMonitorsReportsIndexedErrors(t *testing.T) {
	state := Default()
	state.Display.ExtraMonitors = []Monitor{
		{Name: "HDMI-A-1", Mode: "preferred", Position: "auto", Scale: "1"},
		{Name: "DP-2", Mode: "nope", Position: "auto", Scale: "1"},
		{Name: "", Mode: "preferred", Position: "auto", Scale: "1"},
	}
	errs := ValidateDetailed(state)
	if errs.ExtraMonitors == nil {
		t.Fatal("expected per-extra-monitor error slice")
	}
	if errs.ExtraMonitors[0] != "" {
		t.Fatalf("first extra must validate cleanly, got %q", errs.ExtraMonitors[0])
	}
	if errs.ExtraMonitors[1] == "" {
		t.Fatal("second extra must report its bad mode")
	}
	if errs.ExtraMonitors[2] == "" {
		t.Fatal("third extra must report empty name")
	}
	messages := errs.Messages()
	wantFragments := []string{"extra monitor #2", "extra monitor #3"}
	for _, fragment := range wantFragments {
		found := false
		for _, message := range messages {
			if strings.Contains(message, fragment) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected messages to surface %q, got %#v", fragment, messages)
		}
	}
}

func TestPackagePresetChoicesMatchNixContracts(t *testing.T) {
	presetNamesNix, err := os.ReadFile("../../../NixOS/lib/preset-names.nix")
	if err != nil {
		t.Fatal(err)
	}
	presetsNix, err := os.ReadFile("../../../NixOS/lib/presets.nix")
	if err != nil {
		t.Fatal(err)
	}
	optionsNix, err := os.ReadFile("../../../NixOS/modules/mysetup-options.nix")
	if err != nil {
		t.Fatal(err)
	}

	presetsBlock := regexp.MustCompile(`presetNames = \[([^\]]+)\];`).FindStringSubmatch(string(presetNamesNix))
	if presetsBlock == nil {
		t.Fatalf("presetNames missing in preset-names.nix\n%s", presetNamesNix)
	}
	want := sortedMatches(regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(presetsBlock[1], -1))
	got := sortedStrings(PackagePresets)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Go package presets drifted from preset-names.nix\nGo:  %#v\nNix: %#v", got, want)
	}

	if !regexp.MustCompile(`orderedPresets = presetNames;`).MatchString(string(presetsNix)) {
		t.Fatalf("presets.nix must reuse presetNames from preset-names.nix\n%s", presetsNix)
	}
	if !regexp.MustCompile(`type = types\.enum presetNames;`).MatchString(string(optionsNix)) {
		t.Fatalf("mysetup-options.nix must reuse presetNames from preset-names.nix\n%s", optionsNix)
	}
}

func sortedMatches(matches [][]string) []string {
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = append(values, match[1])
	}
	return sortedStrings(values)
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
