package doctor

import (
	"os"
	"path/filepath"
	"strings"

	migrationv1tov2 "github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/migrations/v1_to_v2"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/shellruntime"
)

func checkShellRuntime(out *reportWriter, opts Options) {
	home := opts.State.User.HomeDirectory
	profile := detectActiveShell(home)

	checkShellStateFile(out, shellruntime.ActiveShellStatePath(home), profile)
	checkRuntimeConfig(out, "stable hyprland entrypoint", hyprConfigPath(home, "hyprland.lua"), "hypr-runtime/hyprland.lua")
	checkRuntimeConfig(out, "stable shell launcher entrypoint", hyprConfigPath(home, "shell-profile.lua"), "hypr-runtime/shell-profile.lua")
	checkShellLauncher(out, hyprRuntimePath(home, "shell-profile.lua"))
	checkRegularFile(out, "user hypr config", hyprConfigPath(home, "user/hyprland.lua"))
	check(out, "shell selector config", filepath.Join(home, ".config/quickshell/wahrwelt-shell-selector/shell.qml"))
	checkHyprScripts(out, filepath.Join(home, ".config/hypr/scripts"))

	switch {
	case shellruntime.IsEnd4Profile(profile):
		checkEnd4Profile(out, home, profile)
	case profile == "caelestia" || profile == "noctalia":
		checkWahrweltProfile(out, home, profile)
	default:
		out.println("WARN active shell is unknown")
		checkWahrweltProfile(out, home, "caelestia")
		checkEnd4Profile(out, home, shellruntime.End4)
	}
}

func hyprConfigPath(home, name string) string {
	return filepath.Join(home, ".config", "hypr", name)
}

func hyprRuntimePath(home, name string) string {
	return shellruntime.RuntimeFile(home, name)
}

func checkShellStateFile(out *reportWriter, path, profile string) {
	if profile == "" {
		profile = "unknown"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		out.printf("WARN shell state missing: %s\n", path)
		return
	}
	got := strings.TrimSpace(string(data))
	if got == profile {
		out.printf("OK   shell state: %s -> %s\n", path, profile)
		return
	}
	out.printf("WARN shell state mismatch: %s -> %s (detected %s)\n", path, got, profile)
}

func checkShellLauncher(out *reportWriter, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		out.printf("WARN shell launcher missing: %s\n", path)
		return
	}
	if strings.Contains(string(data), "start-shell.sh") {
		out.printf("OK   shell launcher: %s\n", path)
		return
	}
	out.printf("WARN shell launcher does not call start-shell.sh: %s\n", path)
}

func checkWahrweltProfile(out *reportWriter, home, profile string) {
	checkShellEntrypoint(out, hyprRuntimePath(home, "hyprland.lua"), profile)
	checkShellLauncherBindings(out, hyprRuntimePath(home, "shell-launcher.lua"), profile)
	checkShellKeybinds(out, hyprRuntimePath(home, "shell-keybinds.lua"), profile)
}

func checkShellEntrypoint(out *reportWriter, path, profile string) {
	data, err := os.ReadFile(path)
	if err != nil {
		out.printf("WARN shell entrypoint missing: %s\n", path)
		return
	}
	if shellruntime.IsCanonicalEntrypoint(path) {
		out.printf("OK   shell entrypoint: %s -> %s\n", path, profile)
		return
	}
	switch migrationv1tov2.RecognizeEntrypoint(string(data), shellruntime.DefaultProfile) {
	case migrationv1tov2.EntrypointNamespaceTransition:
		out.printf("WARN shell entrypoint uses temporary user namespace transition and requires migration: %s\n", path)
		return
	case migrationv1tov2.EntrypointHomeManagerSeededUser:
		out.printf("WARN shell entrypoint uses legacy Home Manager seeded runtime and requires migration: %s\n", path)
		return
	case migrationv1tov2.EntrypointDirectEnd4, migrationv1tov2.EntrypointDirectEnd4PC:
		out.printf("WARN shell entrypoint uses legacy direct End4 runtime and requires migration: %s\n", path)
		return
	case migrationv1tov2.EntrypointLegacyUser:
		out.printf("WARN shell entrypoint uses legacy Wahrwelt user namespace and requires migration: %s\n", path)
		return
	}
	out.printf("WARN shell entrypoint is not the canonical Wahrwelt runtime for profile %q: %s\n", profile, path)
}

func checkShellLauncherBindings(out *reportWriter, path, profileID string) {
	profile, ok := shellruntime.ProfileByID(profileID)
	if !ok {
		out.printf("WARN shell launcher profile is unknown: %s\n", profileID)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		out.printf("WARN shell launcher bindings missing: %s\n", path)
		return
	}
	module := strings.TrimSuffix(filepath.ToSlash(profile.Launcher), ".lua")
	module = strings.ReplaceAll(module, "/", ".")
	if strings.Contains(string(data), `require("`+module+`")`) {
		out.printf("OK   shell launcher bindings: %s -> %s\n", path, profile.ID)
		return
	}
	out.printf("WARN shell launcher bindings do not source current profile %q: %s\n", profile.ID, path)
}

func checkShellKeybinds(out *reportWriter, path, profile string) {
	if profile == "" {
		profile = "caelestia"
	}
	if target, ok := readSymlink(path); ok {
		if _, err := os.ReadFile(path); err != nil {
			out.printf("WARN shell keybinds symlink unreadable: %s -> %s\n", path, target)
			return
		}
		if shellruntime.DetectShellFromKeybinds(path) == profile {
			out.printf("OK   shell keybinds: %s -> %s\n", path, profile)
			return
		}
		out.printf("WARN shell keybinds symlink does not point at profile %q: %s -> %s\n", profile, path, target)
		return
	}
	if _, err := os.ReadFile(path); err != nil {
		out.printf("WARN shell keybinds missing: %s\n", path)
		return
	}
	if shellruntime.DetectShellFromKeybinds(path) == profile {
		out.printf("OK   shell keybinds: %s -> %s\n", path, profile)
		return
	}
	out.printf("WARN shell keybinds do not source current profile %q: %s\n", profile, path)
}

func checkEnd4Profile(out *reportWriter, home, profileID string) {
	profile, ok := shellruntime.ProfileByID(profileID)
	if !ok || profile.Family != shellruntime.End4Family {
		profile, _ = shellruntime.ProfileByID(shellruntime.End4)
	}
	checkShellEntrypoint(out, hyprRuntimePath(home, "hyprland.lua"), profile.ID)
	checkShellLauncherBindings(out, hyprRuntimePath(home, "shell-launcher.lua"), profile.ID)
	checkShellKeybinds(out, hyprRuntimePath(home, "shell-keybinds.lua"), profile.ID)
	checkRuntimeConfig(out, "end4 hyprlock entrypoint", hyprRuntimePath(home, "hyprlock.conf"), "end4/hyprlock.conf")
	checkRuntimeConfig(out, "end4 hypridle entrypoint", hyprRuntimePath(home, "hypridle.conf"), "end4/hypridle.conf")
	check(out, "end4 hypr config", hyprConfigPath(home, "end4/hyprland.lua"))
	check(out, "end4 wahrwelt keybinds", hyprConfigPath(home, "end4/wahrwelt/keybinds.lua"))
	check(out, "end4 quickshell shell", filepath.Join(home, ".config/quickshell", profile.QuickshellConfig, "shell.qml"))
	checkDirectory(out, "end4 runtime config dir", filepath.Join(home, ".config/illogical-impulse"))
	checkWritableFile(out, "end4 kdeglobals", filepath.Join(home, ".config/kdeglobals"))
	checkOptionalFile(out, "end4 runtime config", filepath.Join(home, ".config/illogical-impulse/config.json"))

	for _, rel := range requiredEnd4Scripts() {
		checkExecutable(out, "end4 script", filepath.Join(home, rel))
	}
}

func requiredEnd4Scripts() []string {
	return shellruntime.End4Scripts
}

func checkHyprScripts(out *reportWriter, dir string) {
	for _, script := range requiredHyprScripts() {
		checkExecutable(out, "hypr script", filepath.Join(dir, script))
	}
}

func requiredHyprScripts() []string {
	return shellruntime.HyprScripts
}

func detectActiveShell(home string) string {
	if profile := shellruntime.ReadActiveShell(shellruntime.ActiveShellStatePath(home)); profile != "" {
		return profile
	}
	variantPath := shellruntime.End4VariantStatePath(home)
	for _, candidate := range []struct {
		entrypoint string
		keybinds   string
	}{
		{hyprRuntimePath(home, "hyprland.lua"), hyprRuntimePath(home, "shell-keybinds.lua")},
		{hyprConfigPath(home, "hyprland.lua"), hyprConfigPath(home, "shell-keybinds.lua")},
	} {
		if profile := detectShellFromEntrypointForUpgrade(candidate.entrypoint, candidate.keybinds, variantPath); profile != "" {
			return profile
		}
	}
	for _, path := range []string{
		hyprRuntimePath(home, "shell-keybinds.lua"),
		hyprConfigPath(home, "shell-keybinds.lua"),
	} {
		if profile := detectShellFromKeybinds(path); profile != "" {
			return profile
		}
	}
	return ""
}

func detectShellFromEntrypointForUpgrade(entrypointPath, keybindsPath, variantPath string) string {
	if profile := shellruntime.DetectShellFromEntrypointWithEnd4Variant(entrypointPath, keybindsPath, variantPath); profile != "" {
		return profile
	}
	data, err := os.ReadFile(entrypointPath)
	if err != nil {
		return ""
	}
	switch migrationv1tov2.RecognizeEntrypoint(string(data), shellruntime.DefaultProfile) {
	case migrationv1tov2.EntrypointDirectEnd4, migrationv1tov2.EntrypointDirectEnd4PC:
		return shellruntime.ReadEnd4Variant(variantPath)
	case migrationv1tov2.EntrypointLegacyUser,
		migrationv1tov2.EntrypointHomeManagerSeededUser,
		migrationv1tov2.EntrypointNamespaceTransition:
		return shellruntime.DetectShellFromKeybinds(keybindsPath)
	default:
		return ""
	}
}

func detectShellFromKeybinds(path string) string {
	return shellruntime.DetectShellFromKeybinds(path)
}
