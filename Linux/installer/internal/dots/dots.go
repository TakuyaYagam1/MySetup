package dots

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

type Options struct {
	Sources paths.Sources
	State   config.State
	Secrets config.Secrets
	DryRun  bool
	Yes     bool
}

func Apply(ctx context.Context, opts Options) error {
	runner := run.New(opts.DryRun)
	home := opts.State.User.HomeDirectory
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return err
		}
	}
	configDir := filepath.Join(home, ".config")

	if opts.State.Dots.Wallpapers {
		if err := copyWallpapers(ctx, runner, opts.Sources.NixOS, home); err != nil {
			return err
		}
	}
	if opts.State.Dots.Hypr {
		if err := syncHypr(ctx, runner, opts.Sources.Dots, configDir, opts.State); err != nil {
			return err
		}
	}
	if opts.State.Dots.ZenTheme || opts.State.Dots.Sine {
		if err := setupZen(ctx, runner, opts.Sources.Dots, home, opts.State.Dots); err != nil {
			return err
		}
	}
	if opts.State.Dots.Neovim {
		if err := syncNvim(ctx, runner, opts.Sources.Dots, configDir); err != nil {
			return err
		}
	}
	if opts.State.Dots.V2rayN {
		if err := setupV2rayN(ctx, runner, home); err != nil {
			return err
		}
	}
	return nil
}

func copyWallpapers(ctx context.Context, runner run.Runner, nixosSrc, home string) error {
	src := filepath.Join(nixosSrc, "Wallpapers")
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	dst := filepath.Join(home, "Pictures", "Wallpapers")
	if err := runner.Command(ctx, "mkdir", "-p", dst); err != nil {
		return err
	}
	if err := runner.Command(ctx, "find", dst, "-maxdepth", "1", "-type", "f", "-name", "preview-*", "-delete"); err != nil {
		return err
	}
	return runner.Command(ctx, "rsync", "-a", "--ignore-existing", src+"/", dst+"/")
}

func syncHypr(ctx context.Context, runner run.Runner, dotsSrc, configDir string, state config.State) error {
	src := filepath.Join(dotsSrc, "hypr")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("hypr dots source not found: %s", src)
	}
	dst := filepath.Join(configDir, "hypr")
	if err := backupIfUnmanaged(ctx, runner, dst); err != nil {
		return err
	}
	if err := runner.Command(ctx, "mkdir", "-p", dst); err != nil {
		return err
	}
	if err := runner.Command(ctx, "rsync", "-a", "--delete", "--exclude", "/shell-profile.conf", src+"/", dst+"/"); err != nil {
		return err
	}
	if err := writeMarker(runner, filepath.Join(dst, ".mysetup-managed.json"), "hypr"); err != nil {
		return err
	}
	if !runner.DryRun {
		if err := writeHyprLocalConfig(state, dst); err != nil {
			return err
		}
	} else {
		fmt.Printf("write hypr local config for profile %s\n", state.Shell.Profile)
	}
	if err := runner.Command(ctx, "chmod", "-R", "u+rwX", filepath.Join(dst, "scripts")); err != nil {
		return err
	}
	if err := runner.Command(ctx, "find", filepath.Join(dst, "scripts"), "-type", "f", "-exec", "chmod", "u+x", "{}", "+"); err != nil {
		return err
	}
	if err := runner.Command(ctx, "sh", "-c", "if command -v hyprctl >/dev/null 2>&1 && hyprctl monitors >/dev/null 2>&1; then hyprctl reload >/dev/null 2>&1; fi"); err != nil {
		fmt.Printf("WARN hyprctl reload skipped or failed: %v\n", err)
	}
	return nil
}

func writeHyprLocalConfig(state config.State, hyprDir string) error {
	hyprland := filepath.Join(hyprDir, "hyprland.conf")
	input := filepath.Join(hyprDir, "hyprland", "input.conf")
	shellProfile := filepath.Join(hyprDir, "shell-profile.conf")

	monitorLine := fmt.Sprintf("monitor = %s, %s, %s, %s", state.Display.MonitorName, state.Display.MonitorMode, state.Display.MonitorPosition, state.Display.MonitorScale)
	if err := replaceFirstLine(hyprland, "monitor =", monitorLine); err != nil {
		return err
	}
	if err := replaceFirstLine(input, "    kb_layout =", "    kb_layout = "+state.Locale.KeyboardLayouts); err != nil {
		return err
	}
	if err := replaceFirstLine(input, "    kb_options =", "    kb_options = "+state.Locale.KeyboardToggle); err != nil {
		return err
	}
	return writeShellProfileConfig(shellProfile, state)
}

func writeShellProfileConfig(path string, state config.State) error {
	profile := state.Shell.Profile
	if profile == "" {
		profile = "caelestia"
	}
	script := filepath.Join(filepath.Dir(path), "scripts", "start-shell.sh")
	content := fmt.Sprintf("# Active shell profile: %s\nexec-once = %s %s\n", profile, script, profile)
	return os.WriteFile(path, []byte(content), 0o644)
}

func replaceFirstLine(path, prefix, replacement string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	lines := strings.Split(string(data), "\n")
	replaced := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), strings.TrimSpace(prefix)) {
			lines[i] = replacement
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, replacement)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

func setupZen(ctx context.Context, runner run.Runner, dotsSrc, home string, cfg config.Dots) error {
	profile := findZenProfile(home)
	if profile == "" {
		fmt.Println("Zen Browser profile not found; launch Zen once and rerun mysetup dots/apply")
		return nil
	}
	chrome := filepath.Join(profile, "chrome")
	if cfg.ZenTheme {
		if err := setupZenTheme(ctx, runner, dotsSrc, chrome); err != nil {
			return err
		}
	}
	if cfg.Sine {
		if err := setupSineProfile(ctx, runner, chrome); err != nil {
			return err
		}
	}
	return nil
}

func setupZenTheme(ctx context.Context, runner run.Runner, dotsSrc, chrome string) error {
	src := filepath.Join(dotsSrc, "zen", "chrome")
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("zen browser theme source missing: %s", src)
		}
		return fmt.Errorf("zen browser theme source unreadable: %w", err)
	}
	if err := backupIfUnmanaged(ctx, runner, chrome); err != nil {
		return err
	}
	if err := runner.Command(ctx, "mkdir", "-p", chrome); err != nil {
		return err
	}
	if err := runner.Command(ctx, "rsync", "-a", "--delete", src+"/", chrome+"/"); err != nil {
		return err
	}
	return writeMarker(runner, filepath.Join(chrome, ".mysetup-managed.json"), "zen-chrome")
}

func setupSineProfile(ctx context.Context, runner run.Runner, chrome string) error {
	fmt.Println("Sine profile install is intentionally pinned and best-effort")
	tmp, err := os.MkdirTemp("", "mysetup-sine-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(tmp)
	}()
	bootloaderURL := "https://github.com/sineorg/bootloader/releases/download/v0.1.4/profile.zip"
	engineURL := "https://github.com/CosmoCreeper/Sine/releases/download/v2.3/engine.zip"
	if downloadErr := runner.Command(ctx, "curl", "-fsSL", bootloaderURL, "-o", filepath.Join(tmp, "profile.zip")); downloadErr != nil {
		fmt.Println("Sine profile download failed; skipping")
		return nil
	}
	if downloadErr := runner.Command(ctx, "curl", "-fsSL", engineURL, "-o", filepath.Join(tmp, "engine.zip")); downloadErr != nil {
		fmt.Println("Sine engine download failed; skipping")
		return nil
	}
	if err := runner.Command(ctx, "mkdir", "-p", chrome); err != nil {
		return err
	}
	if err := runner.Command(ctx, "bsdtar", "-xf", filepath.Join(tmp, "profile.zip"), "-C", chrome); err != nil {
		return err
	}
	return runner.Command(ctx, "bsdtar", "-xf", filepath.Join(tmp, "engine.zip"), "-C", chrome)
}

func syncNvim(ctx context.Context, runner run.Runner, dotsSrc, configDir string) error {
	src := filepath.Join(dotsSrc, "nvim")
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	dst := filepath.Join(configDir, "nvim")
	if err := backupIfUnmanaged(ctx, runner, dst); err != nil {
		return err
	}
	if err := runner.Command(ctx, "rsync", "-a", "--delete", src+"/", dst+"/"); err != nil {
		return err
	}
	return writeMarker(runner, filepath.Join(dst, ".mysetup-managed.json"), "nvim")
}

func setupV2rayN(ctx context.Context, runner run.Runner, home string) error {
	singbox, err := runner.Output(ctx, "sh", "-c", "command -v sing-box || true")
	if err != nil {
		return err
	}
	if singbox == "" {
		fmt.Println("sing-box not found in PATH; skipping v2rayN binary seed")
		return nil
	}
	dst := filepath.Join(home, ".local", "share", "v2rayN", "bin", "sing_box", "sing-box")
	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := runner.Command(ctx, "mkdir", "-p", filepath.Dir(dst)); err != nil {
		return err
	}
	return runner.Command(ctx, "install", "-m", "755", singbox, dst)
}

func findZenProfile(home string) string {
	for _, base := range []string{filepath.Join(home, ".zen"), filepath.Join(home, ".config", "zen")} {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		var fallback string
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			path := filepath.Join(base, entry.Name())
			name := strings.ToLower(entry.Name())
			if strings.Contains(name, "default") {
				return path
			}
			if fallback == "" {
				fallback = path
			}
		}
		if fallback != "" {
			return fallback
		}
	}
	return ""
}

func backupIfUnmanaged(ctx context.Context, runner run.Runner, target string) error {
	if _, err := os.Stat(target); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if _, err := os.Stat(filepath.Join(target, ".mysetup-managed.json")); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat managed marker for %s: %w", target, err)
	}
	backup := fmt.Sprintf("%s.bak.%d", target, os.Getpid())
	fmt.Printf("Backing up unmanaged %s -> %s\n", target, backup)
	return runner.Command(ctx, "mv", target, backup)
}

func writeMarker(runner run.Runner, target, kind string) error {
	if runner.DryRun {
		fmt.Printf("write marker %s\n", target)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, []byte(managedMarker(kind)), 0o644)
}

func managedMarker(kind string) string {
	return fmt.Sprintf(`{
  "manager": "mysetup",
  "kind": "%s",
  "version": 1
}
`, kind)
}
