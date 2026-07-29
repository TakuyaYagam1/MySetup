package config

import (
	"os"
	"strings"
	"testing"
)

func TestDeveloperPresetIncludesCodexAndClaudeDesktopApps(t *testing.T) {
	presetInputs := readClaudeDesktopContractFile(t, "../../../NixOS/lib/preset-inputs.nix")
	for _, want := range []string{
		"developerInputs = {",
		"codex-desktop-linux = {",
		`url = "github:ilysenko/codex-desktop-linux";`,
	} {
		if !strings.Contains(presetInputs, want) {
			t.Fatalf("developer preset Codex App contract missing %q\n%s", want, presetInputs)
		}
	}

	homeModule := readClaudeDesktopContractFile(t, "../../../NixOS/home/home.nix")
	for _, want := range []string{
		"developerImports = [",
		"inputs.codex-desktop-linux.homeManagerModules.default",
		"./programs/codex-desktop.nix",
		"lib.optionals developerOrMore developerImports",
	} {
		if !strings.Contains(homeModule, want) {
			t.Fatalf("developer preset Codex App module contract missing %q\n%s", want, homeModule)
		}
	}

	codexModule := readClaudeDesktopContractFile(t, "../../../NixOS/home/programs/codex-desktop.nix")
	if !strings.Contains(codexModule, "wahrweltLib.presets.developerOrMore wahrwelt") {
		t.Fatalf("Codex App must remain enabled for developer and personal presets\n%s", codexModule)
	}

	overlay := readClaudeDesktopContractFile(t, "../../../NixOS/lib/flake-overlays.nix")
	if !strings.Contains(overlay, "claude-desktop = prev.callPackage ../pkgs/claude-desktop.nix { };") {
		t.Fatalf("developer package overlay must expose the Wahrwelt Claude Desktop package\n%s", overlay)
	}

	homePackages := readClaudeDesktopContractFile(t, "../../../NixOS/lib/package-sets/home.nix")
	devStart := strings.Index(homePackages, "dev = with pkgs; [")
	personalStart := strings.Index(homePackages, "personal = with pkgs; [")
	if devStart < 0 || personalStart < 0 || devStart >= personalStart {
		t.Fatalf("could not isolate the developer package set\n%s", homePackages)
	}
	if !strings.Contains(homePackages[devStart:personalStart], "claude-desktop") {
		t.Fatalf("Claude Desktop must be installed by developer and inherited by personal\n%s", homePackages)
	}
}

func TestClaudeDesktopPackageUsesPinnedOfficialDeb(t *testing.T) {
	source := readClaudeDesktopContractFile(t, "../../../NixOS/pkgs/claude-desktop-source.nix")
	for _, want := range []string{
		`version = "`,
		`url = "https://downloads.claude.ai/claude-desktop/apt/stable/pool/`,
		`hash = "sha256-`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("Claude Desktop source metadata missing %q\n%s", want, source)
		}
	}

	pkg := readClaudeDesktopContractFile(t, "../../../NixOS/pkgs/claude-desktop.nix")
	for _, want := range []string{
		"fetchurl",
		"dpkg-deb --fsys-tarfile",
		"autoPatchelfHook",
		"wrapGAppsHook3",
		"rm -f $out/lib/claude-desktop/chrome-sandbox",
		`--add-flags "--ozone-platform-hint=auto --password-store=gnome-libsecret"`,
		"sourceProvenance = [ lib.sourceTypes.binaryNativeCode ];",
	} {
		if !strings.Contains(pkg, want) {
			t.Fatalf("Claude Desktop package contract missing %q\n%s", want, pkg)
		}
	}
	if strings.Contains(pkg, "--no-sandbox") {
		t.Fatalf("Claude Desktop wrapper must not disable Chromium sandboxing\n%s", pkg)
	}
}

func TestClaudeDesktopCoworkIntegrationIsOptIn(t *testing.T) {
	options := readClaudeDesktopContractFile(t, "../../../NixOS/modules/mysetup-options.nix")
	if !strings.Contains(options, "claudeDesktopCowork = boolOption false;") {
		t.Fatalf("Claude Desktop Cowork must default to disabled\n%s", options)
	}

	profile := readClaudeDesktopContractFile(t, "../../../NixOS/profiles/developer.nix")
	if !strings.Contains(profile, "../services/claude-desktop.nix") {
		t.Fatalf("developer profile must import the Claude Desktop host integration\n%s", profile)
	}

	service := readClaudeDesktopContractFile(t, "../../../NixOS/services/claude-desktop.nix")
	for _, want := range []string{
		"config.wahrwelt.features.claudeDesktopCowork",
		"wahrweltLib.presets.developerOrMore config.wahrwelt",
		"pkgs.qemu_kvm",
		"pkgs.OVMF.fd",
		"pkgs.virtiofsd",
		"/usr/share/OVMF/OVMF_CODE.fd",
		"/usr/share/OVMF/OVMF_VARS.fd",
		"/usr/libexec/virtiofsd",
	} {
		if !strings.Contains(service, want) {
			t.Fatalf("Claude Desktop Cowork integration missing %q\n%s", want, service)
		}
	}
}

func TestClaudeDesktopUpdaterVerifiesSignedAptMetadata(t *testing.T) {
	workflow := readClaudeDesktopContractFile(t, "../../../../.github/workflows/update-claude-desktop.yml")
	for _, want := range []string{
		"name: Update Claude Desktop",
		"https://downloads.claude.ai/claude-desktop/key.asc",
		"https://downloads.claude.ai/claude-desktop/apt/stable/dists/stable/InRelease",
		"31DDDE24DDFAB679F42D7BD2BAA929FF1A7ECACE",
		"gpgv",
		"main/binary-amd64/Packages",
		"sha256sum --check",
		"nix build --no-link --print-build-logs",
		"peter-evans/create-pull-request@",
		"secrets.WAHRWELT_AUTOMATION_TOKEN",
		"Linux/NixOS/pkgs/claude-desktop-source.nix",
		`bash .github/scripts/merge-automation-pr.sh "$PR_NUMBER"`,
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("Claude Desktop updater contract missing %q\n%s", want, workflow)
		}
	}
}

func readClaudeDesktopContractFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
