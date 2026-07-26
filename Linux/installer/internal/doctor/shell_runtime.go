package doctor

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/shellruntime"
)

func checkShellRuntime(out *reportWriter, opts Options) {
	home := opts.State.User.HomeDirectory
	profile := detectActiveShell(home)

	checkShellStateFile(out, shellruntime.ActiveShellStatePath(home), profile)
	checkRuntimeConfig(out, "stable hyprland entrypoint", hyprConfigPath(home, "hyprland.lua"), "hypr-runtime/hyprland.lua")
	checkRuntimeConfig(out, "stable shell launcher entrypoint", hyprConfigPath(home, "shell-profile.lua"), "hypr-runtime/shell-profile.lua")
	checkShellLauncher(out, hyprRuntimePath(home, "shell-profile.lua"))
	check(out, "shell selector config", filepath.Join(home, ".config/quickshell/mysetup-shell-selector/shell.qml"))
	checkHyprScripts(out, filepath.Join(home, ".config/hypr/scripts"))

	switch profile {
	case "end4":
		checkEnd4Profile(out, home)
	case "caelestia", "noctalia":
		checkMySetupProfile(out, home, profile)
	default:
		out.println("WARN active shell is unknown")
		checkMySetupProfile(out, home, "caelestia")
		checkEnd4Profile(out, home)
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

func checkMySetupProfile(out *reportWriter, home, profile string) {
	checkShellEntrypoint(out, hyprRuntimePath(home, "hyprland.lua"), "mysetup/hyprland.lua", profile)
	check(out, "mysetup hypr config", hyprConfigPath(home, "mysetup/hyprland.lua"))
	checkShellKeybinds(out, hyprRuntimePath(home, "shell-keybinds.lua"), profile)
}

func checkShellEntrypoint(out *reportWriter, path, wantFragment, profile string) {
	data, err := os.ReadFile(path)
	if err != nil {
		out.printf("WARN shell entrypoint missing: %s\n", path)
		return
	}
	if strings.Contains(string(data), wantFragment) {
		out.printf("OK   shell entrypoint: %s -> %s\n", path, profile)
		return
	}
	out.printf("WARN shell entrypoint does not point at profile %q: %s\n", profile, path)
}

func checkShellKeybinds(out *reportWriter, path, profile string) {
	if profile == "" {
		profile = "caelestia"
	}
	want := profile + ".keybinds"
	if target, ok := readSymlink(path); ok {
		data, err := os.ReadFile(path)
		if err != nil {
			out.printf("WARN shell keybinds symlink unreadable: %s -> %s\n", path, target)
			return
		}
		if strings.Contains(target, profile+"/keybinds.lua") || strings.Contains(string(data), want) || shellKeybindTextMatchesProfile(string(data), profile) {
			out.printf("OK   shell keybinds: %s -> %s\n", path, profile)
			return
		}
		out.printf("WARN shell keybinds symlink does not point at profile %q: %s -> %s\n", profile, path, target)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		out.printf("WARN shell keybinds missing: %s\n", path)
		return
	}
	if strings.Contains(string(data), want) || strings.Contains(string(data), profile+"/keybinds.lua") || shellKeybindTextMatchesProfile(string(data), profile) {
		out.printf("OK   shell keybinds: %s -> %s\n", path, profile)
		return
	}
	out.printf("WARN shell keybinds do not source current profile %q: %s\n", profile, path)
}

func shellKeybindTextMatchesProfile(text, profile string) bool {
	switch profile {
	case "caelestia":
		return strings.Contains(text, "caelestia:session") || strings.Contains(text, "caelestia:launcher")
	case "noctalia":
		return strings.Contains(text, "noctalia-shell ipc call") || strings.Contains(text, "noctalia-msg.sh") || strings.Contains(text, "noctalia-launcher.sh")
	default:
		return false
	}
}

func checkEnd4Profile(out *reportWriter, home string) {
	checkShellEntrypoint(out, hyprRuntimePath(home, "hyprland.lua"), "end4/hyprland.lua", "end4")
	checkRuntimeConfig(out, "end4 hyprlock entrypoint", hyprRuntimePath(home, "hyprlock.conf"), "end4/hyprlock.conf")
	checkRuntimeConfig(out, "end4 hypridle entrypoint", hyprRuntimePath(home, "hypridle.conf"), "end4/hypridle.conf")
	check(out, "end4 hypr config", hyprConfigPath(home, "end4/hyprland.lua"))
	check(out, "end4 mysetup keybinds", hyprConfigPath(home, "end4/mysetup/keybinds.lua"))
	check(out, "end4 quickshell shell", filepath.Join(home, ".config/quickshell/ii/shell.qml"))
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
	for _, candidate := range []struct {
		entrypoint string
		keybinds   string
	}{
		{hyprRuntimePath(home, "hyprland.lua"), hyprRuntimePath(home, "shell-keybinds.lua")},
		{hyprConfigPath(home, "hyprland.lua"), hyprConfigPath(home, "shell-keybinds.lua")},
	} {
		if profile := shellruntime.DetectShellFromEntrypoint(candidate.entrypoint, candidate.keybinds); profile != "" {
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

func detectShellFromKeybinds(path string) string {
	if target, ok := readSymlink(path); ok {
		if strings.Contains(target, "noctalia/keybinds.lua") {
			return "noctalia"
		}
		if strings.Contains(target, "caelestia/keybinds.lua") {
			return "caelestia"
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	switch {
	case strings.Contains(string(data), "noctalia.keybinds") || strings.Contains(string(data), "noctalia/keybinds.lua") || shellKeybindTextMatchesProfile(string(data), "noctalia"):
		return "noctalia"
	case strings.Contains(string(data), "caelestia.keybinds") || strings.Contains(string(data), "caelestia/keybinds.lua") || shellKeybindTextMatchesProfile(string(data), "caelestia"):
		return "caelestia"
	default:
		return ""
	}
}
