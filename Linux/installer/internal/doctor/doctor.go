package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
)

type Options struct {
	Paths paths.Options
	State config.State
}

func Run(ctx context.Context, opts Options) error {
	_ = ctx
	fmt.Println("== MySetup doctor ==")
	check("state", opts.Paths.StatePath)
	check("hardware config", filepath.Join(opts.Paths.NixOSDest, "hosts/NixOS/hardware-configuration.nix"))
	check("flake", filepath.Join(opts.Paths.NixOSDest, "flake.nix"))
	check("variables", filepath.Join(opts.Paths.NixOSDest, "hosts/NixOS/variables.nix"))
	checkShellProfile(filepath.Join(opts.State.User.HomeDirectory, ".config/hypr/shell-profile.conf"), opts.State.Shell.Profile)
	check("hypr start shell", filepath.Join(opts.State.User.HomeDirectory, ".config/hypr/scripts/start-shell.sh"))
	check("installed hypr start shell", filepath.Join(opts.Paths.NixOSDest, "dots/hypr/scripts/start-shell.sh"))
	check("hypr record toggle", filepath.Join(opts.State.User.HomeDirectory, ".config/hypr/scripts/record-toggle.sh"))
	checkWallpapers(opts)
	if findZenProfile(opts.State.User.HomeDirectory) == "" {
		fmt.Println("WARN Zen profile not found")
	} else {
		fmt.Println("OK   Zen profile found")
	}
	fmt.Println("Last-resort rollback: sudo cp -a /etc/nixos.bak.<timestamp> /etc/nixos")
	return nil
}

func check(label, path string) {
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("OK   %s: %s\n", label, path)
		return
	}
	fmt.Printf("WARN %s missing: %s\n", label, path)
}

func checkShellProfile(path, profile string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("WARN shell profile missing: %s\n", path)
		return
	}
	if profile == "" {
		profile = "caelestia"
	}
	want := "start-shell.sh " + profile
	if strings.Contains(string(data), want) {
		fmt.Printf("OK   shell profile: %s -> %s\n", path, profile)
		return
	}
	fmt.Printf("WARN shell profile does not source current profile %q: %s\n", profile, path)
}

func checkWallpapers(opts Options) {
	wallpaperDir := filepath.Join(opts.State.User.HomeDirectory, "Pictures/Wallpapers")
	variables := filepath.Join(opts.Paths.NixOSDest, "hosts/NixOS/variables.nix")
	if opts.State.Dots.Wallpapers {
		check("wallpapers", wallpaperDir)
		flag := variablesWallpaperEnable(variables)
		if flag != nil && !*flag {
			fmt.Printf("WARN wallpapers enabled in state but disabled in variables: %s\n", variables)
		}
		return
	}

	fmt.Println("OK   wallpapers disabled in state")
	flag := variablesWallpaperEnable(variables)
	if flag == nil {
		fmt.Printf("WARN variables missing var.wallpapers.enable; Home Manager may still seed wallpapers: %s\n", variables)
		return
	}
	if *flag {
		fmt.Printf("WARN wallpapers disabled in state but enabled in variables: %s\n", variables)
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
