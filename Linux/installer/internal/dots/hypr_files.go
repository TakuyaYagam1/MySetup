package dots

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

func writeHyprLocalConfig(ctx context.Context, runner run.CommandRunner, state config.State, hyprSourceDir, hyprDir string) error {
	if err := writeHyprLocalConfigFiles(state, hyprSourceDir, hyprDir); err != nil {
		if !os.IsPermission(err) || state.User.Username == "" {
			return err
		}
		if repairErr := repairUserWritableTree(ctx, runner, hyprDir, state.User.Username); repairErr != nil {
			return repairErr
		}
		return writeHyprLocalConfigFiles(state, hyprSourceDir, hyprDir)
	}
	return nil
}

func writeHyprLocalConfigFiles(state config.State, hyprSourceDir, hyprDir string) error {
	sourceHyprland := filepath.Join(hyprSourceDir, "hyprland.conf")
	mysetupHyprland := filepath.Join(hyprDir, "mysetup", "hyprland.conf")
	input := filepath.Join(hyprDir, "hyprland", "input.conf")

	if err := os.MkdirAll(filepath.Dir(mysetupHyprland), 0o755); err != nil {
		return err
	}
	if err := prepareWritableConfigFile(mysetupHyprland); err != nil {
		return err
	}
	if err := copyRegularFile(sourceHyprland, mysetupHyprland); err != nil {
		return err
	}

	monitorLine := fmt.Sprintf("monitor = %s, %s, %s, %s", state.Display.MonitorName, state.Display.MonitorMode, state.Display.MonitorPosition, state.Display.MonitorScale)
	if err := replaceFirstLine(mysetupHyprland, "monitor =", monitorLine); err != nil {
		return err
	}
	if err := replaceFirstLine(input, "    kb_layout =", "    kb_layout = "+state.Locale.KeyboardLayouts); err != nil {
		return err
	}
	if err := replaceFirstLine(input, "    kb_options =", "    kb_options = "+state.Locale.KeyboardToggle); err != nil {
		return err
	}
	return nil
}

func writeRuntimeConfigFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := prepareWritableConfigFile(path); err != nil {
		return err
	}
	return atomicWriteFile(path, []byte(content), 0o644)
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := file.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmp)
		}
	}()

	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func copyRegularFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
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
