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
		"noctalia msg",
		"app2unit -- $terminal",
		`wahrwelt.hypr .. "/scripts/screenshot.sh full"`,
		"$hypr/scripts/record-toggle.sh",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("shared Hypr keybinds must stay shell-neutral; found %q\n%s", forbidden, text)
		}
	}
	if !strings.Contains(text, `wahrwelt.load_runtime("shell-keybinds.lua")`) {
		t.Fatalf("shared Hypr keybinds must load the runtime profile layer\n%s", text)
	}
	if !strings.Contains(text, `wahrwelt.load_runtime("shell-launcher.lua")`) {
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
		`"app2unit -- " .. v.zed`,
		`wahrwelt.hypr .. "/scripts/screenshot.sh full"`,
		"caelestia clipboard",
	} {
		if !strings.Contains(caelestiaProfile, want) {
			t.Fatalf("caelestia profile missing %q\n%s", want, caelestiaProfile)
		}
	}
	for _, want := range []string{
		`wahrwelt.bind_dispatch("SUPER + SUPER_L", "global caelestia:launcher")`,
		`wahrwelt.bind_dispatch("SUPER + mouse:272", "global caelestia:launcherInterrupt", { non_consuming = true })`,
		`wahrwelt.bind_dispatch("SUPER + mouse_down", "global caelestia:launcherInterrupt", { non_consuming = true })`,
	} {
		if !strings.Contains(string(caelestiaLauncher), want) {
			t.Fatalf("caelestia launcher profile missing %q\n%s", want, string(caelestiaLauncher))
		}
	}
	for _, want := range []string{
		"noctalia-msg.sh",
		"restore-lock.sh noctalia",
		"shell-selector.sh toggle",
		`"app2unit -- " .. v.terminal`,
		`"app2unit -- " .. v.zed`,
		`wahrwelt.hypr .. "/scripts/screenshot.sh full"`,
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
	if !strings.Contains(runtime, `wahrwelt_default_shell_profile="`+manifest.DefaultProfile+`"`) {
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

func TestFreshHyprAndSelectorAssetsUseOnlyCanonicalBrand(t *testing.T) {
	for _, root := range []string{
		"../../../dots/hypr",
		"../../../NixOS/home/shells/quickshell/wahrwelt-shell-selector",
	} {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if strings.Contains(strings.ToLower(entry.Name()), "mysetup") {
				t.Errorf("fresh managed asset keeps legacy name: %s", path)
			}
			if entry.IsDir() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(strings.ToLower(string(data)), "mysetup") {
				t.Errorf("fresh managed asset keeps legacy text: %s", path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
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
		"wahrwelt-noctalia-launcher",
		"noctalia-launcher\\.sh",
		"acquire_lock",
		"wahrwelt_noctalia_action launcher-toggle",
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
		`state_dir="$runtime_dir/wahrwelt-shell-selector"`,
		`lock_dir="$state_dir/lock"`,
		"selector_name=\"wahrwelt-shell-selector\"",
		"WAHRWELT_SHELL_SELECTOR_MONITOR",
		"WAHRWELT_ACTIVE_SHELL",
		"WAHRWELT_SHELL_SELECTOR_MONITOR",
		"WAHRWELT_ACTIVE_SHELL",
		"acquire_lock()",
		"wait_for_selector_spawn()",
		"detect_shell_from_processes()",
		"detect_shell_from_entrypoint()",
		"wahrwelt_noctalia_running",
		"quickshell/ii([/[:space:]]|$)",
		"wahrwelt/hyprland.lua",
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
		"wahrwelt_acquire_lock",
		"wahrwelt-record-toggle",
		"record-toggle\\.sh",
		`ps -p "$pid" -o args=`,
		"gpu-screen-recorder",
		"WAHRWELT_RECORD_TARGET",
		"WAHRWELT_RECORD_TARGET",
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
		"shell-runtime.sh",
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
		"wahrwelt-start-shell",
		"shell-runtime.sh",
		"shell-process.sh",
		"shell-profile-sync.sh",
		"shell-runtime-env.sh",
		"shell-end4-overrides.sh",
		"wahrwelt_pid_matches",
		"wahrwelt_noctalia_pids()",
		"wahrwelt_noctalia_daemon_flag()",
		`pgrep -u "$wahrwelt_user_name" -x noctalia`,
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
		"hypr_supports_lua_runtime()",
		"running Hyprland is older than 0.55",
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
	if strings.Contains(text, `(^|[ /])noctalia([[:space:]]|$)`) {
		t.Fatalf("noctalia process detection must not match bare profile arguments\n%s", text)
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
		"wahrwelt_valid_shell_profile",
		`"$script_dir/start-shell.sh" "$profile"`,
		"wait_for_profile()",
		`hyprctl dispatch 'hl.dsp.global("caelestia:lock")'`,
		"wahrwelt_noctalia_action lock",
		`hyprlock -c "$wahrwelt_hypr_runtime_dir/hyprlock.conf"`,
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
		"Wahrwelt start-shell owns end4 QuickShell",
		"Wahrwelt start-shell owns end4 hypridle",
		`/^    hl\.exec_cmd("qs -c \$qsConfig")$/`,
		`/^    hl\.exec_cmd("hypridle")$/`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("end4 dotfile patch missing lifecycle guard %q\n%s", want, text)
		}
	}
}

func TestEnd4RuntimeOverrideParserPreservesKeyboardLayoutCommas(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not installed")
	}
	sourceData, err := os.ReadFile("../../../dots/hypr/scripts/shell-end4-overrides.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sourceData), `hyprctl switchxkblayout all 0`) {
		t.Fatalf("end4 runtime override must reset active XKB layout to the first configured layout\n%s", string(sourceData))
	}

	configPath := filepath.Join(t.TempDir(), "general.lua")
	if err := os.WriteFile(configPath, []byte(`
hl.config({
    input = {
        kb_layout = "us, ru",
        kb_options = "grp:alt_shift_toggle",
    },
})
`), 0o644); err != nil {
		t.Fatal(err)
	}

	script, err := filepath.Abs("../../../dots/hypr/scripts/shell-end4-overrides.sh")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-c", `
set -eu
. "$1"
hypr_config_value "$2" kb_layout fallback
printf '\n'
hypr_config_value "$2" kb_options fallback
`, "bash", script, configPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hypr_config_value failed: %v\n%s", err, string(output))
	}
	if got, want := strings.TrimSpace(string(output)), "us,ru\ngrp:alt_shift_toggle"; got != want {
		t.Fatalf("unexpected parsed end4 keyboard values: got %q want %q", got, want)
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
	selector, err := os.ReadFile("../../../NixOS/home/shells/quickshell/wahrwelt-shell-selector/shell.qml")
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

func TestCaelestiaActivationPreservesDisabledTransparency(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/home/caelestia/default.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)

	if strings.Contains(text, `.appearance.transparency.enabled //= true`) {
		t.Fatalf("caelestia transparency seed must not treat false as missing\n%s", text)
	}
	for _, want := range []string{
		`${dotfilesLib.mkBoolDefault ".appearance.transparency.enabled" true} |`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("caelestia activation missing transparency false-preserving seed %q\n%s", want, text)
		}
	}
}

func TestEnd4ActivationPreservesDisabledTransparency(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/home/end4/seed/config.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)

	if strings.Contains(text, `.appearance.transparency.enable //= true`) {
		t.Fatalf("end4 transparency seed must not treat false as missing\n%s", text)
	}
	for _, want := range []string{
		`${dotfilesLib.mkBoolDefault ".appearance.transparency.enable" true} |`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("end4 activation missing transparency false-preserving seed %q\n%s", want, text)
		}
	}
}

func TestShellSeedFiltersDoNotUseJQTrueFallback(t *testing.T) {
	for _, path := range []string{
		"../../../NixOS/home/caelestia/default.nix",
		"../../../NixOS/home/end4/seed/config.nix",
		"../../../NixOS/home/noctalia/default.nix",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), `//= true`) {
			t.Fatalf("seed filters must use mkBoolDefault for true boolean defaults, found //= true in %s\n%s", path, string(data))
		}
	}
}

func TestWindowOpacityIsNotMultipliedByFoot(t *testing.T) {
	footData, err := os.ReadFile("../../../NixOS/home/programs/foot.nix")
	if err != nil {
		t.Fatal(err)
	}
	rulesData, err := os.ReadFile("../../../dots/hypr/hyprland/rules.lua")
	if err != nil {
		t.Fatal(err)
	}
	variablesData, err := os.ReadFile("../../../dots/hypr/variables.lua")
	if err != nil {
		t.Fatal(err)
	}

	foot := string(footData)
	rules := string(rulesData)
	variables := string(variablesData)
	if !strings.Contains(foot, `alpha = lib.mkForce 1.0;`) {
		t.Fatalf("Foot must leave window opacity to Hyprland instead of multiplying it\n%s", foot)
	}
	if !strings.Contains(variables, `windowOpacity = 0.75,`) {
		t.Fatalf("Hyprland application opacity must stay at the tuned value\n%s", variables)
	}
	if !strings.Contains(variables, `footWindowOpacity = 0.85,`) {
		t.Fatalf("Foot compositor opacity must stay independently tunable\n%s", variables)
	}
	if !strings.Contains(rules, `opacity = tostring(v.windowOpacity) .. " override"`) {
		t.Fatalf("Hyprland global window opacity rule is missing\n%s", rules)
	}
	if !strings.Contains(rules, `match = { class = "foot" }, opacity = tostring(v.footWindowOpacity) .. " override"`) {
		t.Fatalf("Foot must retain its dedicated compositor opacity\n%s", rules)
	}
	opaqueFootRule := regexp.MustCompile(`class = "[^"]*\bfoot\b[^"]*".*opaque = true`)
	if opaqueFootRule.MatchString(rules) {
		t.Fatalf("Foot must use the same blur and opacity path as other windows\n%s", rules)
	}
}

func TestObservabilityGrafanaUsesPersistentSecretFile(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/services/observability.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`grafanaSecretKeyPath = "/var/lib/grafana/secret_key";`,
		`pkgs.writeShellScript "wahrwelt-grafana-secret-key"`,
		`${pkgs.openssl}/bin/openssl rand -hex 32 > "$secret_key"`,
		`${pkgs.coreutils}/bin/chmod 0600 "$secret_key"`,
		`security.secret_key = "$__file{${grafanaSecretKeyPath}}";`,
		`before = [ "grafana.service" ];`,
		`requiredBy = [ "grafana.service" ];`,
		`Type = "oneshot";`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("observability Grafana config missing persistent secret contract %q\n%s", want, text)
		}
	}
}

func TestInstallerPackageProvidesXKeyboardRules(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/lib/flake-packages.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`--set WAHRWELT_XKB_RULES_DIR ${flakePkgs.xkeyboard_config}/share/X11/xkb/rules`,
		`--set WAHRWELT_XKB_RULES_DIR ${flakePkgs.xkeyboard_config}/share/X11/xkb/rules`,
		`xkeyboard_config`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("wahrwelt package missing XKB rules runtime contract %q\n%s", want, text)
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

			cmd := exec.Command("fish", "--no-config", script, "-g", "workspace", "1")
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
	bindRe := regexp.MustCompile(`^(?:wahrwelt\.)?(?:bind_[a-z]+|bind)\("([^"]+)"`)
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
