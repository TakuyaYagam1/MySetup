package config

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestCodexDesktopUpdaterVerifiesBeforePublishing(t *testing.T) {
	workflow, err := os.ReadFile("../../../../.github/workflows/update-codex-desktop.yml")
	if err != nil {
		t.Fatal(err)
	}
	source := string(workflow)

	for _, want := range []string{
		"name: Update Codex Desktop",
		`- cron: "50 * * * *"`,
		"CODEX_DESKTOP_REPOSITORY: https://github.com/ilysenko/codex-desktop-linux.git",
		`git ls-remote "$CODEX_DESKTOP_REPOSITORY" HEAD`,
		"nix flake update codex-desktop-linux",
		"flake.lock",
		"Linux/NixOS/flake.lock",
		"Linux/NixOS/presets/developer/flake.lock",
		"Linux/NixOS/presets/personal/flake.lock",
		"git+file://${GITHUB_WORKSPACE}?dir=Linux/NixOS/presets/developer",
		"#checks.x86_64-linux.preset-host.drvPath",
		"nix eval --raw --impure --no-write-lock-file",
		"Build Codex Desktop package with retry",
		"for attempt in 1 2 3",
		`github:ilysenko/codex-desktop-linux/${TARGET_REVISION}#packages.x86_64-linux.codex-desktop`,
		"hash mismatch in fixed-output derivation",
		"peter-evans/create-pull-request@",
		"secrets.WAHRWELT_AUTOMATION_TOKEN",
		`bash .github/scripts/merge-automation-pr.sh "$PR_NUMBER"`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("Codex Desktop updater contract missing %q\n%s", want, source)
		}
	}

	if strings.Contains(source, "Build updated developer preset host") {
		t.Fatal("Codex Desktop updater must not build the whole developer host")
	}
	if strings.Contains(source, "nix flake check") {
		t.Fatal("Codex Desktop updater must not run flake check because --no-build cannot realise Stylix palette derivations on a clean runner")
	}
}

func TestGenericFlakeUpdaterExcludesCodexDesktop(t *testing.T) {
	workflow, err := os.ReadFile("../../../../.github/workflows/update-flake.yml")
	if err != nil {
		t.Fatal(err)
	}
	source := string(workflow)

	for _, want := range []string{
		"update_flake_without_codex",
		`select(. != "codex-desktop-linux")`,
		"update_flake_without_codex .",
		"update_flake_without_codex Linux/NixOS",
		"update_flake_without_codex \"Linux/NixOS/presets/${preset}\"",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("generic flake updater must exclude Codex Desktop via %q\n%s", want, source)
		}
	}
}

func TestAutomationMergeWaitsForUpdatedBranchHead(t *testing.T) {
	script, err := os.ReadFile("../../../../.github/scripts/merge-automation-pr.sh")
	if err != nil {
		t.Fatal(err)
	}
	source := string(script)

	for _, want := range []string{
		"wait_for_updated_head()",
		"expected head sha didn't match current head ref",
		"wait_for_updated_head \"$head_sha\"",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("automation merge script must wait for GitHub to apply update-branch via %q\\n%s", want, source)
		}
	}
}

func TestCaelestiaDefaultsUseCurrentBarSchema(t *testing.T) {
	defaults, err := os.ReadFile("../../../NixOS/home/caelestia/default-settings.json")
	if err != nil {
		t.Fatal(err)
	}

	var settings map[string]any
	if err := json.Unmarshal(defaults, &settings); err != nil {
		t.Fatalf("invalid Caelestia default settings JSON: %v", err)
	}

	bar, ok := settings["bar"].(map[string]any)
	if !ok {
		t.Fatal("Caelestia defaults must contain a bar object")
	}
	if _, ok := bar["status"]; ok {
		t.Fatal("Caelestia defaults must not contain the removed bar.status option")
	}
	statusIcons, ok := bar["statusIcons"].([]any)
	if !ok || len(statusIcons) == 0 {
		t.Fatal("Caelestia defaults must contain non-empty bar.statusIcons")
	}

	module, err := os.ReadFile("../../../NixOS/home/caelestia/default.nix")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(module), "del(.status)") {
		t.Fatal("Caelestia seed migration must remove the obsolete bar.status option")
	}
}
