package doctor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
)

type Options struct {
	Paths  paths.Options
	State  config.State
	Stdout io.Writer
}

func Run(ctx context.Context, opts Options) error {
	report, err := Report(ctx, opts)
	if err != nil {
		return err
	}
	out := opts.Stdout
	if out == nil {
		out = os.Stdout
	}
	_, err = fmt.Fprint(out, report)
	return err
}

func Report(ctx context.Context, opts Options) (string, error) {
	_ = ctx
	out := &reportWriter{}
	out.println("== MySetup doctor ==")
	check(out, "state", opts.Paths.StatePath)
	check(out, "hardware config", filepath.Join(opts.Paths.NixOSDest, "hosts/NixOS/hardware-configuration.nix"))
	check(out, "flake", filepath.Join(opts.Paths.NixOSDest, "flake.nix"))
	check(out, "variables", filepath.Join(opts.Paths.NixOSDest, "hosts/NixOS/variables.nix"))
	checkShellRuntime(out, opts)
	checkWallpapers(out, opts)
	if findZenProfile(opts.State.User.HomeDirectory) == "" {
		out.println("WARN Zen profile not found")
	} else {
		out.println("OK   Zen profile found")
	}
	out.println("Last-resort rollback: sudo cp -a /etc/nixos.bak.<timestamp> /etc/nixos")
	return out.String(), out.err
}

type reportWriter struct {
	bytes.Buffer
	err error
}

func (w *reportWriter) printf(format string, args ...any) {
	if w.err != nil {
		return
	}
	_, w.err = fmt.Fprintf(&w.Buffer, format, args...)
}

func (w *reportWriter) println(line string) {
	if w.err != nil {
		return
	}
	_, w.err = fmt.Fprintln(&w.Buffer, line)
}

func check(out *reportWriter, label, path string) {
	if _, err := os.Stat(path); err == nil {
		out.printf("OK   %s: %s\n", label, path)
		return
	}
	out.printf("WARN %s missing: %s\n", label, path)
}

func checkShellRuntime(out *reportWriter, opts Options) {
	home := opts.State.User.HomeDirectory
	profile := detectActiveShell(home)

	checkShellStateFile(out, paths.ActiveShellStatePath(home), profile)
	checkShellLauncher(out, filepath.Join(home, ".config/hypr/shell-profile.conf"))
	check(out, "shell selector config", filepath.Join(home, ".config/quickshell/mysetup-shell-selector/shell.qml"))
	checkHyprScripts(out, filepath.Join(home, ".config/hypr/scripts"))
	checkHyprScripts(out, filepath.Join(opts.Paths.NixOSDest, "dots/hypr/scripts"))

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
	checkShellEntrypoint(out, filepath.Join(home, ".config/hypr/hyprland.conf"), "mysetup/hyprland.conf", profile)
	check(out, "mysetup hypr config", filepath.Join(home, ".config/hypr/mysetup/hyprland.conf"))
	checkShellKeybinds(out, filepath.Join(home, ".config/hypr/shell-keybinds.conf"), profile)
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
	want := profile + "/keybinds.conf"
	if target, ok := readSymlink(path); ok {
		data, err := os.ReadFile(path)
		if err != nil {
			out.printf("WARN shell keybinds symlink unreadable: %s -> %s\n", path, target)
			return
		}
		if strings.Contains(target, want) || shellKeybindTextMatchesProfile(string(data), profile) {
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
	if strings.Contains(string(data), want) || shellKeybindTextMatchesProfile(string(data), profile) {
		out.printf("OK   shell keybinds: %s -> %s\n", path, profile)
		return
	}
	out.printf("WARN shell keybinds do not source current profile %q: %s\n", profile, path)
}

func shellKeybindTextMatchesProfile(text, profile string) bool {
	switch profile {
	case "caelestia":
		return strings.Contains(text, "bindi = Super, Super_L, global, caelestia:launcher") &&
			strings.Contains(text, "bindin = Super, catchall, global, caelestia:launcherInterrupt")
	case "noctalia":
		return strings.Contains(text, "noctalia-shell ipc call") || strings.Contains(text, "noctalia-launcher.sh")
	default:
		return false
	}
}

func readSymlink(path string) (string, bool) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return "", false
	}
	target, err := os.Readlink(path)
	if err != nil {
		return "", false
	}
	return target, true
}

func checkEnd4Profile(out *reportWriter, home string) {
	checkShellEntrypoint(out, filepath.Join(home, ".config/hypr/hyprland.conf"), "end4/hyprland.conf", "end4")
	checkRuntimeConfig(out, "end4 hyprlock entrypoint", filepath.Join(home, ".config/hypr/hyprlock.conf"), "end4/hyprlock.conf")
	checkRuntimeConfig(out, "end4 hypridle entrypoint", filepath.Join(home, ".config/hypr/hypridle.conf"), "end4/hypridle.conf")
	check(out, "end4 hypr config", filepath.Join(home, ".config/hypr/end4/hyprland.conf"))
	check(out, "end4 mysetup keybinds", filepath.Join(home, ".config/hypr/end4/mysetup/keybinds.conf"))
	check(out, "end4 quickshell shell", filepath.Join(home, ".config/quickshell/ii/shell.qml"))
	checkDirectory(out, "end4 runtime config dir", filepath.Join(home, ".config/illogical-impulse"))
	checkWritableFile(out, "end4 kdeglobals", filepath.Join(home, ".config/kdeglobals"))
	checkOptionalFile(out, "end4 runtime config", filepath.Join(home, ".config/illogical-impulse/config.json"))

	for _, rel := range requiredEnd4Scripts() {
		checkExecutable(out, "end4 script", filepath.Join(home, rel))
	}
}

func checkRuntimeConfig(out *reportWriter, label, path, wantFragment string) {
	data, err := os.ReadFile(path)
	if err != nil {
		out.printf("WARN %s missing: %s\n", label, path)
		return
	}
	if strings.Contains(string(data), wantFragment) {
		out.printf("OK   %s: %s\n", label, path)
		return
	}
	out.printf("WARN %s does not point at %q: %s\n", label, wantFragment, path)
}

func requiredEnd4Scripts() []string {
	return []string{
		".config/hypr/end4/hyprland/scripts/launch_first_available.sh",
		".config/hypr/end4/hyprland/scripts/start_geoclue_agent.sh",
		".config/hypr/end4/custom/scripts/__restore_video_wallpaper.sh",
		".config/hypr/end4/mysetup/scripts/record-toggle.sh",
		".config/hypr/end4/mysetup/scripts/screenshot.sh",
		".config/hypr/end4/mysetup/scripts/spotify-toggle.sh",
	}
}

func detectActiveShell(home string) string {
	if profile := readActiveShellState(paths.ActiveShellStatePath(home)); profile != "" {
		return profile
	}
	if profile := detectShellFromEntrypoint(filepath.Join(home, ".config/hypr/hyprland.conf")); profile != "" {
		return profile
	}
	return detectShellFromKeybinds(filepath.Join(home, ".config/hypr/shell-keybinds.conf"))
}

func readActiveShellState(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	switch profile := strings.TrimSpace(string(data)); profile {
	case "caelestia", "noctalia", "end4":
		return profile
	default:
		return ""
	}
}

func detectShellFromEntrypoint(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := string(data)
	switch {
	case strings.Contains(text, "end4/hyprland.conf"):
		return "end4"
	case strings.Contains(text, "mysetup/hyprland.conf"):
		return detectShellFromKeybinds(filepath.Join(filepath.Dir(path), "shell-keybinds.conf"))
	default:
		return ""
	}
}

func detectShellFromKeybinds(path string) string {
	if target, ok := readSymlink(path); ok {
		if strings.Contains(target, "noctalia/keybinds.conf") {
			return "noctalia"
		}
		if strings.Contains(target, "caelestia/keybinds.conf") {
			return "caelestia"
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	switch {
	case strings.Contains(string(data), "noctalia/keybinds.conf") || shellKeybindTextMatchesProfile(string(data), "noctalia"):
		return "noctalia"
	case strings.Contains(string(data), "caelestia/keybinds.conf") || shellKeybindTextMatchesProfile(string(data), "caelestia"):
		return "caelestia"
	default:
		return ""
	}
}

func checkHyprScripts(out *reportWriter, dir string) {
	for _, script := range requiredHyprScripts() {
		checkExecutable(out, "hypr script", filepath.Join(dir, script))
	}
}

func requiredHyprScripts() []string {
	return []string{
		"close-active.sh",
		"noctalia-launcher.sh",
		"record-toggle.sh",
		"shell-selector.sh",
		"screenshot.sh",
		"spotify-toggle.sh",
		"start-shell.sh",
		"wsaction.fish",
	}
}

func checkDirectory(out *reportWriter, label, path string) {
	info, err := os.Stat(path)
	if err != nil {
		out.printf("WARN %s missing: %s\n", label, path)
		return
	}
	if !info.IsDir() {
		out.printf("WARN %s is not a directory: %s\n", label, path)
		return
	}
	out.printf("OK   %s: %s\n", label, path)
}

func checkExecutable(out *reportWriter, label, path string) {
	info, err := os.Stat(path)
	if err != nil {
		out.printf("WARN %s missing: %s\n", label, path)
		return
	}
	if info.Mode().Perm()&0o111 == 0 {
		out.printf("WARN %s not executable: %s\n", label, path)
		return
	}
	out.printf("OK   %s executable: %s\n", label, path)
}

func checkWritableFile(out *reportWriter, label, path string) {
	info, err := os.Stat(path)
	if err != nil {
		out.printf("WARN %s missing: %s\n", label, path)
		return
	}
	if info.IsDir() {
		out.printf("WARN %s is a directory, expected file: %s\n", label, path)
		return
	}
	if info.Mode().Perm()&0o200 == 0 {
		out.printf("WARN %s not writable: %s\n", label, path)
		return
	}
	out.printf("OK   %s writable: %s\n", label, path)
}

func checkOptionalFile(out *reportWriter, label, path string) {
	if _, err := os.Stat(path); err == nil {
		out.printf("OK   %s: %s\n", label, path)
		return
	}
	out.printf("WARN %s missing (created on first shell run): %s\n", label, path)
}

func checkWallpapers(out *reportWriter, opts Options) {
	wallpaperDir := filepath.Join(opts.State.User.HomeDirectory, "Pictures/Wallpapers")
	variables := filepath.Join(opts.Paths.NixOSDest, "hosts/NixOS/variables.nix")
	if opts.State.Dots.Wallpapers {
		check(out, "wallpapers", wallpaperDir)
		flag := variablesWallpaperEnable(variables)
		if flag != nil && !*flag {
			out.printf("WARN wallpapers enabled in state but disabled in variables: %s\n", variables)
		}
		return
	}

	out.println("OK   wallpapers disabled in state")
	flag := variablesWallpaperEnable(variables)
	if flag == nil {
		out.printf("WARN variables missing var.wallpapers.enable; Home Manager may still seed wallpapers: %s\n", variables)
		return
	}
	if *flag {
		out.printf("WARN wallpapers disabled in state but enabled in variables: %s\n", variables)
	}
}

func variablesWallpaperEnable(path string) *bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := string(data)
	if regexp.MustCompile(`(?s)wallpapers\s*=\s*\{[^}]*enable\s*=\s*true\s*;`).MatchString(text) {
		return boolPtr(true)
	}
	if regexp.MustCompile(`(?s)wallpapers\s*=\s*\{[^}]*enable\s*=\s*false\s*;`).MatchString(text) {
		return boolPtr(false)
	}
	return nil
}

func boolPtr(value bool) *bool {
	return &value
}

func findZenProfile(home string) string {
	for _, base := range []string{filepath.Join(home, ".zen"), filepath.Join(home, ".config", "zen")} {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				return filepath.Join(base, entry.Name())
			}
		}
	}
	return ""
}
