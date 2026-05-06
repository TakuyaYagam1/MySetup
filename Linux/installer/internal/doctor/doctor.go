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
	checkShellProfile(out, filepath.Join(opts.State.User.HomeDirectory, ".config/hypr/shell-profile.conf"), opts.State.Shell.Profile)
	checkShellKeybinds(out, filepath.Join(opts.State.User.HomeDirectory, ".config/hypr/shell-keybinds.conf"), opts.State.Shell.Profile)
	checkHyprScripts(out, filepath.Join(opts.State.User.HomeDirectory, ".config/hypr/scripts"))
	checkHyprScripts(out, filepath.Join(opts.Paths.NixOSDest, "dots/hypr/scripts"))
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

func checkShellProfile(out *reportWriter, path, profile string) {
	data, err := os.ReadFile(path)
	if err != nil {
		out.printf("WARN shell profile missing: %s\n", path)
		return
	}
	if profile == "" {
		profile = "caelestia"
	}
	want := "start-shell.sh " + profile
	if strings.Contains(string(data), want) {
		out.printf("OK   shell profile: %s -> %s\n", path, profile)
		return
	}
	out.printf("WARN shell profile does not source current profile %q: %s\n", profile, path)
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
		"screenshot.sh",
		"spotify-toggle.sh",
		"start-shell.sh",
		"wsaction.fish",
	}
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
