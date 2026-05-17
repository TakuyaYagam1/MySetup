package dots

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestSharedHyprKeybindsDoNotContainShellSpecificBindings(t *testing.T) {
	data, err := os.ReadFile("../../../dots/hypr/hyprland/keybinds.lua")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{
		"caelestia clipboard",
		"caelestia emoji",
		"caelestia record",
		"caelestia resizer pip",
		"noctalia-shell ipc call",
		"app2unit -- $terminal",
		`mysetup.hypr .. "/scripts/screenshot.sh full"`,
		"$hypr/scripts/record-toggle.sh",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("shared Hypr keybinds must stay shell-neutral; found %q\n%s", forbidden, text)
		}
	}
	if !strings.Contains(text, `mysetup.load_runtime("shell-keybinds.lua")`) {
		t.Fatalf("shared Hypr keybinds must load the runtime profile layer\n%s", text)
	}
	if !strings.Contains(text, `mysetup.load_runtime("shell-launcher.lua")`) {
		t.Fatalf("shared Hypr keybinds must load the runtime launcher layer\n%s", text)
	}
}

func TestShellKeybindProfilesUseExpectedLaunchers(t *testing.T) {
	caelestia, err := os.ReadFile("../../../dots/hypr/caelestia/keybinds.lua")
	if err != nil {
		t.Fatal(err)
	}
	caelestiaLauncher, err := os.ReadFile("../../../dots/hypr/caelestia/launcher.lua")
	if err != nil {
		t.Fatal(err)
	}
	noctalia, err := os.ReadFile("../../../dots/hypr/noctalia/keybinds.lua")
	if err != nil {
		t.Fatal(err)
	}
	noctaliaLauncher, err := os.ReadFile("../../../dots/hypr/noctalia/launcher.lua")
	if err != nil {
		t.Fatal(err)
	}
	common, err := os.ReadFile("../../../dots/hypr/shell-common-keybinds.lua")
	if err != nil {
		t.Fatal(err)
	}
	caelestiaProfile := string(caelestia) + string(common)
	noctaliaProfile := string(noctalia) + string(common)
	for _, want := range []string{
		"restore-lock.sh caelestia",
		"shell-selector.sh toggle",
		`"app2unit -- " .. v.terminal`,
		`mysetup.hypr .. "/scripts/screenshot.sh full"`,
		"caelestia clipboard",
	} {
		if !strings.Contains(caelestiaProfile, want) {
			t.Fatalf("caelestia profile missing %q\n%s", want, caelestiaProfile)
		}
	}
	for _, want := range []string{
		`mysetup.bind_dispatch("SUPER + SUPER_L", "global caelestia:launcher")`,
		`mysetup.bind_dispatch("SUPER + mouse:272", "global caelestia:launcherInterrupt", { non_consuming = true })`,
		`mysetup.bind_dispatch("SUPER + mouse_down", "global caelestia:launcherInterrupt", { non_consuming = true })`,
	} {
		if !strings.Contains(string(caelestiaLauncher), want) {
			t.Fatalf("caelestia launcher profile missing %q\n%s", want, string(caelestiaLauncher))
		}
	}
	for _, want := range []string{
		"noctalia-shell ipc call",
		"restore-lock.sh noctalia",
		"shell-selector.sh toggle",
		`"app2unit -- " .. v.terminal`,
		`mysetup.hypr .. "/scripts/screenshot.sh full"`,
	} {
		if !strings.Contains(noctaliaProfile, want) {
			t.Fatalf("noctalia profile missing %q\n%s", want, noctaliaProfile)
		}
	}
	for _, want := range []string{
		"noctalia-launcher.sh press",
		"noctalia-launcher.sh interrupt",
		"noctalia-launcher.sh release",
	} {
		if !strings.Contains(string(noctaliaLauncher), want) {
			t.Fatalf("noctalia launcher profile missing %q\n%s", want, string(noctaliaLauncher))
		}
	}
}

func TestHyprKeybindFilesDoNotContainUnexpectedDuplicateChords(t *testing.T) {
	allowed := map[string]bool{
		"hyprland/keybinds.lua|CTRL+SUPER+ALT+Backslash": true,
		"noctalia/launcher.lua|SUPER+SUPER_L":            true,
	}

	for _, rel := range []string{
		"hyprland/keybinds.lua",
		"caelestia/keybinds.lua",
		"caelestia/launcher.lua",
		"noctalia/keybinds.lua",
		"noctalia/launcher.lua",
		"end4/keybinds.lua",
		"shell-common-keybinds.lua",
		"shell-workspace-keybinds.lua",
	} {
		t.Run(rel, func(t *testing.T) {
			assertNoUnexpectedDuplicateHyprBinds(t, rel, allowed)
		})
	}
}

func TestShellProfilesMatchRuntimeScriptContract(t *testing.T) {
	profilesData, err := os.ReadFile("../../../NixOS/home/shells/profiles.nix")
	if err != nil {
		t.Fatal(err)
	}
	runtimeData, err := os.ReadFile("../../../dots/hypr/scripts/shell-runtime.sh")
	if err != nil {
		t.Fatal(err)
	}
	manifestData, err := os.ReadFile("../shellruntime/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		DefaultProfile string `json:"defaultProfile"`
		Profiles       []struct {
			ID string `json:"id"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}

	profiles := manifestProfileIDs(manifest.Profiles)
	if !strings.Contains(string(profilesData), "ordered = shellRuntimeManifest.profiles;") {
		t.Fatalf("profiles.nix must source ordered profile metadata from shell runtime manifest\n%s", string(profilesData))
	}
	runtime := string(runtimeData)
	for _, profile := range profiles {
		pattern := regexp.MustCompile(`(\|\s*` + regexp.QuoteMeta(profile) + `\b)|(\b` + regexp.QuoteMeta(profile) + `\s*\|)`)
		if !pattern.MatchString(runtime) {
			t.Fatalf("shell-runtime.sh valid profile case is missing %q\n%s", profile, runtime)
		}
	}

	if !strings.Contains(string(profilesData), "inherit (shellRuntimeManifest) defaultProfile;") {
		t.Fatalf("profiles.nix must source defaultProfile from shell runtime manifest\n%s", string(profilesData))
	}
	if !strings.Contains(runtime, `mysetup_default_shell_profile="`+manifest.DefaultProfile+`"`) {
		t.Fatalf("shell-runtime.sh default profile drifted from manifest default %q\n%s", manifest.DefaultProfile, runtime)
	}
}

func manifestProfileIDs(profiles []struct {
	ID string `json:"id"`
}) []string {
	ids := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		ids = append(ids, profile.ID)
	}
	return ids
}

func TestNoctaliaLauncherScriptIsGuarded(t *testing.T) {
	data, err := os.ReadFile("../../../dots/hypr/scripts/noctalia-launcher.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"active_file=",
		"interrupt_file=",
		"lock_dir=",
		"lock_owner_file=",
		"mysetup-noctalia-launcher",
		"noctalia-launcher\\.sh",
		"acquire_lock",
		"noctalia-shell ipc call launcher toggle",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("noctalia launcher wrapper missing %q\n%s", want, text)
		}
	}
}

func TestShellSelectorScriptTracksFocusedMonitorAndActiveShell(t *testing.T) {
	data, err := os.ReadFile("../../../dots/hypr/scripts/shell-selector.sh")
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.ReadFile("../../../dots/hypr/scripts/shell-runtime.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data) + string(helper)
	for _, want := range []string{
		`state_dir="$runtime_dir/mysetup-shell-selector"`,
		`lock_dir="$state_dir/lock"`,
		"selector_name=\"mysetup-shell-selector\"",
		"MYSETUP_SHELL_SELECTOR_MONITOR",
		"MYSETUP_ACTIVE_SHELL",
		"acquire_lock()",
		"wait_for_selector_spawn()",
		"detect_shell_from_processes()",
		"detect_shell_from_entrypoint()",
		"quickshell/ii([/[:space:]]|$)",
		"mysetup/hyprland.lua",
		"hyprctl monitors -j",
		"active-shell",
		"start-shell.sh",
		"qs -c \"$selector_name\"",
		"switch_shell()",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("shell selector script missing %q\n%s", want, text)
		}
	}
}

func TestRecordToggleScriptUsesLockAndPidValidation(t *testing.T) {
	data, err := os.ReadFile("../../../dots/hypr/scripts/record-toggle.sh")
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.ReadFile("../../../dots/hypr/scripts/shell-runtime.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data) + string(helper)
	for _, want := range []string{
		`mkdir "$lock_dir"`,
		`lock_pid_file=`,
		`lock_owner_file=`,
		"mysetup_acquire_lock",
		"mysetup-record-toggle",
		"record-toggle\\.sh",
		`ps -p "$pid" -o args=`,
		"gpu-screen-recorder",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("record toggle script missing %q\n%s", want, text)
		}
	}
}

func TestStartShellScriptCleansDuplicateProfiles(t *testing.T) {
	data, err := os.ReadFile("../../../dots/hypr/scripts/start-shell.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, helper := range []string{
		"shell-process.sh",
		"shell-profile-sync.sh",
		"shell-runtime-env.sh",
		"shell-end4-overrides.sh",
	} {
		data, err := os.ReadFile(filepath.Join("../../../dots/hypr/scripts", helper))
		if err != nil {
			t.Fatal(err)
		}
		text += string(data)
	}
	for _, want := range []string{
		"lock_owner_file=",
		"mysetup-start-shell",
		"shell-runtime.sh",
		"shell-process.sh",
		"shell-profile-sync.sh",
		"shell-runtime-env.sh",
		"shell-end4-overrides.sh",
		"mysetup_pid_matches",
		"start-shell\\.sh",
		"running_count()",
		"dedupe_shell()",
		"sync_runtime_shell_files()",
		"validate_profile_ready()",
		"prepare_profile_or_fallback()",
		"require_file()",
		"aborting shell switch before stopping current shell",
		"persistent_state_file=",
		"selector_pattern=",
		"caelestia_resizer_handle=",
		"end4_pattern=",
		"end4_idle_handle=",
		"stop_shell_selector()",
		"stop_caelestia_resizer()",
		"stop_end4_idle()",
		"hypr_runtime_dir=",
		"runtime_file()",
		"shell-keybinds.lua",
		`hypridle -c "$idle_config"`,
		"hyprctl reload",
		`("$@" >>"$log_file" 2>&1 &)`,
		`(caelestia resizer -d >>"$log_file" 2>&1 &)`,
		`dedupe_shell "caelestia"`,
		`dedupe_shell "caelestia resizer"`,
		`dedupe_shell "noctalia"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("start-shell script missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "qs kill --any-display") {
		t.Fatalf("start-shell script must not kill every quickshell instance\n%s", text)
	}
	if strings.Contains(text, `pkill -u "$user_name" -x hypridle`) {
		t.Fatalf("start-shell script must only stop the managed end4 hypridle instance\n%s", text)
	}

	prepareIndex := strings.Index(text, "if ! prepare_profile_or_fallback; then")
	stopIndex := strings.Index(text, `if [ "$previous" != "$profile" ] || [ -n "$requested_profile" ]; then`)
	persistIndex := strings.Index(text, "persist_profile ||")
	for name, index := range map[string]int{
		"prepare": prepareIndex,
		"stop":    stopIndex,
		"persist": persistIndex,
	} {
		if index < 0 {
			t.Fatalf("start-shell script missing %s ordering marker\n%s", name, text)
		}
	}
	if prepareIndex >= stopIndex || stopIndex >= persistIndex {
		t.Fatalf("start-shell must validate and sync before stopping shells or persisting profile")
	}
}

func TestRestoreLockScriptStartsProfileBeforeLocking(t *testing.T) {
	data, err := os.ReadFile("../../../dots/hypr/scripts/restore-lock.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"shell-runtime.sh",
		"mysetup_valid_shell_profile",
		`"$script_dir/start-shell.sh" "$profile"`,
		"wait_for_profile()",
		`hyprctl dispatch 'hl.dsp.global("caelestia:lock")'`,
		"noctalia-shell ipc call lockScreen lock",
		`hyprlock -c "$mysetup_hypr_runtime_dir/hyprlock.conf"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("restore-lock script missing %q\n%s", want, text)
		}
	}
}

func TestEnd4HyprPatchDisablesUpstreamShellLifecycle(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/home/end4/patches/hypr.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"MySetup start-shell owns end4 QuickShell",
		"MySetup start-shell owns end4 hypridle",
		`/^    hl\.exec_cmd("qs -c \$qsConfig")$/`,
		`/^    hl\.exec_cmd("hypridle")$/`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("end4 dotfile patch missing lifecycle guard %q\n%s", want, text)
		}
	}
}

func TestEnd4QuickshellPatchUsesNixOSLogo(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/home/end4/patches/quickshell.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `Quickshell.iconPath("nix-snowflake")`) {
		t.Fatalf("end4 quickshell patch missing NixOS logo replacement\n%s", text)
	}
}

func TestShellBrandingUsesNixOSLogoOutsideSelector(t *testing.T) {
	caelestia, err := os.ReadFile("../../../NixOS/home/caelestia/general.nix")
	if err != nil {
		t.Fatal(err)
	}
	noctalia, err := os.ReadFile("../../../NixOS/home/noctalia/config/settings.json")
	if err != nil {
		t.Fatal(err)
	}
	selector, err := os.ReadFile("../../../NixOS/home/shells/quickshell/mysetup-shell-selector/shell.qml")
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		`logo = "nix-snowflake";`,
	} {
		if !strings.Contains(string(caelestia), want) {
			t.Fatalf("caelestia branding missing %q\n%s", want, string(caelestia))
		}
	}
	for _, want := range []string{
		`"icon": "nix-snowflake"`,
		`"useDistroLogo": true`,
	} {
		if !strings.Contains(string(noctalia), want) {
			t.Fatalf("noctalia branding missing %q\n%s", want, string(noctalia))
		}
	}
	for _, want := range []string{
		`assets/noctalia.svg`,
		`assets/caelestia.svg`,
		`assets/illogical-impulse.svg`,
	} {
		if !strings.Contains(string(selector), want) {
			t.Fatalf("shell selector must keep upstream logos; missing %q\n%s", want, string(selector))
		}
	}
}

func TestCaelestiaActivationRepairsInvalidShellJson(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/home/caelestia/default.nix")
	if err != nil {
		t.Fatal(err)
	}
	helpers, err := os.ReadFile("../../../NixOS/home/lib/dotfiles.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data) + string(helpers)
	for _, want := range []string{
		`jq -e 'type == "object"'`,
		`$target.bak.`,
		`seed_if_missing "$target" "$source"`,
		`ensure_json_object "$target" "$source"`,
		`${pkgs.coreutils}/bin/install -m 644 "$source" "$target"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("caelestia activation missing invalid shell.json repair guard %q\n%s", want, text)
		}
	}
}

func TestWsactionGroupModeShiftsFishArgv(t *testing.T) {
	if _, err := exec.LookPath("fish"); err != nil {
		t.Skip("fish is not installed")
	}
	bin := t.TempDir()
	callFile := filepath.Join(t.TempDir(), "hyprctl-call")
	writeScript(t, filepath.Join(bin, "hyprctl"), `if [ "$1" = "activeworkspace" ]; then
  printf '{"id":%s}\n' "$ACTIVE_WS"
  exit 0
fi
printf '%s\n' "$*" > "$CALL_FILE"
`)
	writeScript(t, filepath.Join(bin, "jq"), `printf '%s\n' "$ACTIVE_WS"`)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("CALL_FILE", callFile)

	script, err := filepath.Abs("../../../dots/hypr/scripts/wsaction.fish")
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"10": `dispatch hl.dsp.focus({ workspace = "10" })`,
		"20": `dispatch hl.dsp.focus({ workspace = "10" })`,
		"23": `dispatch hl.dsp.focus({ workspace = "3" })`,
	}
	for activeWS, want := range cases {
		t.Run(activeWS, func(t *testing.T) {
			t.Setenv("ACTIVE_WS", activeWS)
			if err := os.Remove(callFile); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}

			cmd := exec.Command("fish", script, "-g", "workspace", "1")
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("wsaction.fish failed: %v\n%s", err, string(output))
			}
			data, err := os.ReadFile(callFile)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.TrimSpace(string(data)); got != want {
				t.Fatalf("unexpected hyprctl dispatch call: got %q want %q", got, want)
			}
		})
	}
}

func writeScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func assertNoUnexpectedDuplicateHyprBinds(t *testing.T, rel string, allowed map[string]bool) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("../../../dots/hypr", rel))
	if err != nil {
		t.Fatal(err)
	}

	seen := map[string]int{}
	bindRe := regexp.MustCompile(`^(?:mysetup\.)?(?:bind_[a-z]+|bind)\("([^"]+)"`)
	for lineNo, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		match := bindRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		key := strings.Join(strings.Fields(match[1]), "")
		allowKey := rel + "|" + key
		if previous, ok := seen[key]; ok && !allowed[allowKey] {
			t.Fatalf("unexpected duplicate Hypr bind in %s: line %d duplicates line %d for %s\n%s", rel, lineNo+1, previous, key, string(data))
		}
		seen[key] = lineNo + 1
	}
}
