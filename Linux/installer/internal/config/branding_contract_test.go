package config

import (
	"os"
	"strings"
	"testing"
)

func TestWahrweltDoomBannerIsSharedByFishAndNeovim(t *testing.T) {
	fish := readBrandingContractFile(t, "../../../NixOS/home/programs/fish.nix")
	foot := readBrandingContractFile(t, "../../../NixOS/home/programs/foot.nix")
	dashboard := readBrandingContractFile(t, "../../../dots/nvim/lua/plugins/dashboard.lua")

	bannerLines := []string{
		" _    _   ___   _   _ ______  _    _  _____  _      _____",
		`| |  | | / _ \ | | | || ___ \| |  | ||  ___|| |    |_   _|`,
		`| |  | |/ /_\ \| |_| || |_/ /| |  | || |__  | |      | |`,
		"| |/\\| ||  _  ||  _  ||    / | |/\\| ||  __| | |      | |",
		`\  /\  /| | | || | | || |\ \ \  /\  /| |___ | |____  | |`,
		` \/  \/ \_| |_/\_| |_/\_| \_| \/  \/ \____/ \_____/  \_/`,
	}
	for _, line := range bannerLines {
		if !strings.Contains(fish, line) {
			t.Fatalf("Fish greeting is missing Wahrwelt Doom banner line %q\n%s", line, fish)
		}
		if !strings.Contains(dashboard, line) {
			t.Fatalf("Neovim dashboard is missing Wahrwelt Doom banner line %q\n%s", line, dashboard)
		}
	}

	if !strings.Contains(fish, "TAAG font: Doom") {
		t.Fatalf("Fish greeting must document the shared FIGlet font\n%s", fish)
	}
	if !strings.Contains(dashboard, "TAAG font: Doom") {
		t.Fatalf("Neovim dashboard must document the shared FIGlet font\n%s", dashboard)
	}
	if !strings.Contains(foot, `shell = "fish";`) {
		t.Fatalf("Foot must continue to render the shared Fish greeting\n%s", foot)
	}

	for name, source := range map[string]string{
		"Fish":   fish,
		"Neovim": dashboard,
	} {
		if strings.Contains(source, "|_     _|.---.-.|  |--.") {
			t.Fatalf("%s still contains the legacy Takuya Chunky banner\n%s", name, source)
		}
	}
}

func TestFishGreetingCentersFastfetchUnderWahrweltBanner(t *testing.T) {
	fish := readBrandingContractFile(t, "../../../NixOS/home/programs/fish.nix")

	for _, snippet := range []string{
		"set -l wahrwelt_banner_width 58",
		"set -l fastfetch_width 37",
		`set -l fastfetch_padding (math --scale=0 "($wahrwelt_banner_width - $fastfetch_width) / 2")`,
		"fastfetch --key-padding-left $fastfetch_padding",
	} {
		if !strings.Contains(fish, snippet) {
			t.Fatalf("Fish greeting must center Fastfetch under the Wahrwelt banner using %q\n%s", snippet, fish)
		}
	}
}

func readBrandingContractFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
