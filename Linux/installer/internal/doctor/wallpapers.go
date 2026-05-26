package doctor

import (
	"os"
	"path/filepath"
	"regexp"
)

func checkWallpapers(out *reportWriter, opts Options) {
	wallpaperDir := filepath.Join(opts.State.User.HomeDirectory, "Pictures/Wallpapers")
	hostVars := hostVarsPath(opts.Paths.NixOSDest)
	if opts.State.Dots.Wallpapers {
		check(out, "wallpapers", wallpaperDir)
		flag := variablesWallpaperEnable(hostVars)
		if flag != nil && !*flag {
			out.printf("WARN wallpapers enabled in state but disabled in host vars: %s\n", hostVars)
		}
		return
	}

	out.println("OK   wallpapers disabled in state")
	flag := variablesWallpaperEnable(hostVars)
	if flag == nil {
		out.printf("WARN host vars missing wallpapers.enable; Home Manager may still seed wallpapers: %s\n", hostVars)
		return
	}
	if *flag {
		out.printf("WARN wallpapers disabled in state but enabled in host vars: %s\n", hostVars)
	}
}

func hostVarsPath(nixosDest string) string {
	root := filepath.Join(nixosDest, "host-vars.nix")
	if _, err := os.Stat(root); err == nil {
		return root
	}
	return filepath.Join(nixosDest, "hosts/NixOS/host-vars.nix")
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
