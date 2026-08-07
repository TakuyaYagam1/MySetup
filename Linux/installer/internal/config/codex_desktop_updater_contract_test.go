package config

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestAutomationMergeFollowsHeadChangesWhileWaitingForLint(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "pr-view-count")
	mergedFile := filepath.Join(tempDir, "merged")
	fakeGH := filepath.Join(tempDir, "gh")

	fakeGHSource := `#!/usr/bin/env bash
set -euo pipefail

old_head=1111111111111111111111111111111111111111
new_head=2222222222222222222222222222222222222222

if [[ "$1 $2" == "pr view" ]]; then
  count=0
  if [[ -f "$FAKE_GH_STATE" ]]; then
    count="$(<"$FAKE_GH_STATE")"
  fi
  count=$((count + 1))
  printf '%s' "$count" >"$FAKE_GH_STATE"

  if [[ " $* " == *" --jq "* ]]; then
    printf '%s\n' "$new_head"
  elif ((count == 1)); then
    printf '{"state":"OPEN","isDraft":false,"baseRefName":"main","headRefOid":"%s"}\n' "$old_head"
  else
    printf '{"state":"OPEN","isDraft":false,"baseRefName":"main","headRefOid":"%s"}\n' "$new_head"
  fi
  exit 0
fi

if [[ "$1 $2" == "pr merge" ]]; then
  : >"$FAKE_GH_MERGED"
  exit 0
fi

if [[ "$1" == "api" ]]; then
  case " $* " in
    *"/commits/$old_head/check-runs"*)
      printf '{"check_runs":[]}\n'
      ;;
    *"/commits/$new_head/check-runs"*)
      printf '{"check_runs":[{"id":1,"status":"completed","conclusion":"success","html_url":"https://example.invalid/check"}]}\n'
      ;;
    *"/git/ref/heads/main"*)
      printf '%s\n' aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
      ;;
    *"/compare/"*)
      printf '%s\n' ahead
      ;;
    *)
      printf 'unexpected gh api call: %s\n' "$*" >&2
      exit 1
      ;;
  esac
  exit 0
fi

printf 'unexpected gh call: %s\n' "$*" >&2
exit 1
`
	if err := os.WriteFile(fakeGH, []byte(fakeGHSource), 0o700); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "../../../../.github/scripts/merge-automation-pr.sh", "111")
	cmd.Env = append(os.Environ(),
		"GITHUB_REPOSITORY=test/repository",
		"FAKE_GH_STATE="+stateFile,
		"FAKE_GH_MERGED="+mergedFile,
		"PATH="+tempDir+":"+os.Getenv("PATH"),
	)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("automation merge kept polling the stale PR head:\n%s", output)
	}
	if err != nil {
		t.Fatalf("automation merge failed: %v\n%s", err, output)
	}
	if _, err := os.Stat(mergedFile); err != nil {
		t.Fatalf("automation merge did not merge the updated PR head: %v\n%s", err, output)
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
