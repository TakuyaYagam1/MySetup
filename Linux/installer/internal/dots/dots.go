package dots

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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
	shellKeybinds := filepath.Join(hyprDir, "shell-keybinds.conf")

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
	if err := writeShellProfileConfig(shellProfile, state); err != nil {
		return err
	}
	return writeShellKeybindsConfig(shellKeybinds, state)
}

func writeShellProfileConfig(path string, state config.State) error {
	profile := state.Shell.Profile
	if profile == "" {
		profile = "caelestia"
	}
	if err := prepareWritableConfigFile(path); err != nil {
		return err
	}
	script := filepath.Join(filepath.Dir(path), "scripts", "start-shell.sh")
	content := fmt.Sprintf("# Active shell profile: %s\nexec-once = %s %s\n", profile, script, profile)
	return os.WriteFile(path, []byte(content), 0o644)
}

func writeShellKeybindsConfig(path string, state config.State) error {
	profile := state.Shell.Profile
	if profile == "" {
		profile = "caelestia"
	}
	profilePath := filepath.Join(filepath.Dir(path), profile, "keybinds.conf")
	if _, err := os.Stat(profilePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("shell keybind profile missing: %s", profilePath)
		}
		return err
	}
	if err := prepareWritableConfigFile(path); err != nil {
		return err
	}
	content := fmt.Sprintf("# Active shell keybind profile: %s\nsource = %s\n", profile, profilePath)
	return os.WriteFile(path, []byte(content), 0o644)
}

func prepareWritableConfigFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return os.Remove(path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing to overwrite non-regular config file: %s", path)
	}
	return nil
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
	profileZip := filepath.Join(tmp, "profile.zip")
	engineZip := filepath.Join(tmp, "engine.zip")
	if downloadErr := runner.Command(ctx, "curl", "-fsSL", bootloaderURL, "-o", profileZip); downloadErr != nil {
		fmt.Println("Sine profile download failed; skipping")
		return nil
	}
	if downloadErr := runner.Command(ctx, "curl", "-fsSL", engineURL, "-o", engineZip); downloadErr != nil {
		fmt.Println("Sine engine download failed; skipping")
		return nil
	}
	if err := runner.Command(ctx, "mkdir", "-p", chrome); err != nil {
		return err
	}
	if runner.DryRun {
		fmt.Printf("verify sha256 %s %s\n", profileZip, sineProfileSHA256)
		fmt.Printf("verify sha256 %s %s\n", engineZip, sineEngineSHA256)
		fmt.Printf("safe extract %s -> %s\n", profileZip, chrome)
		fmt.Printf("safe extract %s -> %s\n", engineZip, chrome)
		return nil
	}
	if err := verifyFileSHA256(profileZip, sineProfileSHA256); err != nil {
		return err
	}
	if err := verifyFileSHA256(engineZip, sineEngineSHA256); err != nil {
		return err
	}
	if err := safeExtractZip(profileZip, chrome); err != nil {
		return err
	}
	return safeExtractZip(engineZip, chrome)
}

const (
	sineProfileSHA256 = "285b3d589cc979f11f01c9c77410b717694ccc4f32cc1cb08bd6d8909fb98e00"
	sineEngineSHA256  = "5892add04ab4cf808018d8982495d53029de0b5cd62d80ea6905a741cf897bfd"
	maxZipEntryBytes  = 128 * 1024 * 1024
)

func verifyFileSHA256(path, want string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return err
	}
	got := hex.EncodeToString(sum.Sum(nil))
	if got != want {
		return fmt.Errorf("sha256 mismatch for %s: got %s want %s", path, got, want)
	}
	return nil
}

func safeExtractZip(zipPath, dest string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = reader.Close()
	}()
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	for _, file := range reader.File {
		if err := safeExtractZipFile(file, destAbs); err != nil {
			return err
		}
	}
	return nil
}

func safeExtractZipFile(file *zip.File, destAbs string) error {
	if file.FileInfo().Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to extract symlink from zip: %s", file.Name)
	}
	if file.UncompressedSize64 > maxZipEntryBytes {
		return fmt.Errorf("refusing oversized zip entry: %s", file.Name)
	}
	target, err := safeZipTarget(destAbs, file.Name)
	if err != nil {
		return err
	}
	if err := ensureNoSymlinkInPath(destAbs, target); err != nil {
		return err
	}
	if file.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	source, err := file.Open()
	if err != nil {
		return err
	}
	defer func() {
		_ = source.Close()
	}()
	mode := file.FileInfo().Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	targetFile, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer func() {
		_ = targetFile.Close()
	}()
	written, err := io.Copy(targetFile, io.LimitReader(source, maxZipEntryBytes+1))
	if err != nil {
		return err
	}
	if written > maxZipEntryBytes {
		return fmt.Errorf("refusing oversized zip entry: %s", file.Name)
	}
	return nil
}

func ensureNoSymlinkInPath(destAbs, targetAbs string) error {
	rel, err := filepath.Rel(destAbs, targetAbs)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	current := destAbs
	parts := strings.Split(rel, string(os.PathSeparator))
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to extract through existing symlink: %s", current)
		}
		if i < len(parts)-1 && !info.IsDir() {
			return fmt.Errorf("refusing to extract through non-directory path: %s", current)
		}
	}
	return nil
}

func safeZipTarget(destAbs, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) {
		return "", fmt.Errorf("refusing unsafe zip path: %s", name)
	}
	target := filepath.Join(destAbs, filepath.FromSlash(name))
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(destAbs, targetAbs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("refusing zip path outside destination: %s", name)
	}
	return targetAbs, nil
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
