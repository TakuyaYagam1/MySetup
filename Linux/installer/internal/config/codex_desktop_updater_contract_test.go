package config

import (
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
		"nix flake check --impure --no-build --no-write-lock-file",
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
