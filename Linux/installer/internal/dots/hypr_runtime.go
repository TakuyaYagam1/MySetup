package dots

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/shellruntime"
)

func writeHyprRuntimeShellState(home, hyprDir string) error {
	profile := shellruntime.BootstrapActiveShell(home, hyprDir)
	statePath := shellruntime.ActiveShellStatePath(home)
	shellLauncher := shellruntime.RuntimeFile(home, "shell-profile.conf")
	shellLauncherBindings := shellruntime.RuntimeFile(home, "shell-launcher.conf")
	shellKeybinds := shellruntime.RuntimeFile(home, "shell-keybinds.conf")
	hyprland := shellruntime.RuntimeFile(home, "hyprland.conf")
	hyprlock := shellruntime.RuntimeFile(home, "hyprlock.conf")
	hypridle := shellruntime.RuntimeFile(home, "hypridle.conf")

	if err := writeRuntimeShellStateFile(statePath, profile); err != nil {
		return err
	}
	if err := writeStableHyprEntrypoints(home, hyprDir); err != nil {
		return err
	}
	if err := writeShellLauncherConfig(shellLauncher, hyprDir); err != nil {
		return err
	}
	if err := writeShellLauncherBindingsConfig(shellLauncherBindings, hyprDir, profile); err != nil {
		return err
	}
	if profile != "end4" {
		return writeLegacyRuntimeShellState(hyprDir, shellKeybinds, hyprland, hyprlock, hypridle, shellLauncher, profile)
	}

	return writeEnd4RuntimeShellState(hyprDir, hyprland, hyprlock, hypridle, shellLauncher)
}

func writeLegacyRuntimeShellState(hyprDir, shellKeybinds, hyprland, hyprlock, hypridle, shellLauncher, profile string) error {
	if err := writeShellKeybindsConfig(shellKeybinds, hyprDir, profile); err != nil {
		return err
	}
	if err := writeHyprEntrypointConfig(hyprland, filepath.Join(hyprDir, "mysetup", "hyprland.conf"), shellLauncher, fmt.Sprintf("mysetup (%s)", profile)); err != nil {
		return err
	}
	if err := writeShellManagedRuntimePlaceholder(hyprlock, "Hyprlock", profile, "Caelestia and Noctalia use shell-native lock flows."); err != nil {
		return err
	}
	return writeShellManagedRuntimePlaceholder(hypridle, "Hypridle", profile, "Caelestia and Noctalia use shell-native idle flows.")
}

func writeEnd4RuntimeShellState(hyprDir, hyprland, hyprlock, hypridle, shellLauncher string) error {
	end4Hyprland := filepath.Join(hyprDir, "end4", "hyprland.conf")
	ok, err := shellruntime.RuntimeConfigExists(end4Hyprland)
	if err != nil || !ok {
		return err
	}
	if err := writeHyprEntrypointConfig(hyprland, end4Hyprland, shellLauncher, "end4"); err != nil {
		return err
	}
	if err := writeOptionalRuntimeSourceConfig(hyprlock, filepath.Join(hyprDir, "end4", "hyprlock.conf"), "Active Hyprlock profile: end4"); err != nil {
		return err
	}
	return writeOptionalRuntimeSourceConfig(hypridle, filepath.Join(hyprDir, "end4", "hypridle.conf"), "Active Hypridle profile: end4")
}

func writeOptionalRuntimeSourceConfig(path, target, label string) error {
	ok, err := shellruntime.RuntimeConfigExists(target)
	if err != nil || !ok {
		return err
	}
	return writeRuntimeSourceConfig(path, target, label)
}

func writeRuntimeShellStateFile(path, profile string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := prepareWritableConfigFile(path); err != nil {
		return err
	}
	return atomicWriteFile(path, []byte(profile+"\n"), 0o644)
}

func writeStableHyprEntrypoints(home, hyprDir string) error {
	runtimeDir := shellruntime.RuntimeDir(home)
	for _, name := range shellruntime.RuntimeFiles {
		comment := ""
		if name == "hyprland.conf" {
			comment = "MySetup stable Hyprland entrypoint."
		}
		if err := writeRuntimeConfigFile(
			filepath.Join(hyprDir, name),
			stableRuntimeSourceConfig(filepath.Join(runtimeDir, name), comment),
		); err != nil {
			return err
		}
	}
	return nil
}

func stableRuntimeSourceConfig(target, comment string) string {
	if comment == "" {
		return fmt.Sprintf("source = %s\n", target)
	}
	return fmt.Sprintf("# %s\nsource = %s\n", comment, target)
}

func writeShellLauncherConfig(path, hyprDir string) error {
	script := filepath.Join(hyprDir, "scripts", "start-shell.sh")
	content := fmt.Sprintf("# Runtime shell launcher\nexec-once = %s\n", script)
	return writeRuntimeConfigFile(path, content)
}

func writeShellKeybindsConfig(path, hyprDir, profile string) error {
	profilePath := filepath.Join(hyprDir, profile, "keybinds.conf")
	if _, err := os.Stat(profilePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("shell keybind profile missing: %s", profilePath)
		}
		return err
	}
	content := fmt.Sprintf("# Active shell keybind profile: %s\nsource = %s\n", profile, profilePath)
	return writeRuntimeConfigFile(path, content)
}

func writeShellLauncherBindingsConfig(path, hyprDir, profile string) error {
	profilePath := filepath.Join(hyprDir, profile, "launcher.conf")
	if _, err := os.Stat(profilePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("shell launcher profile missing: %s", profilePath)
		}
		return err
	}
	content := fmt.Sprintf("# Active shell launcher profile: %s\nsource = %s\n", profile, profilePath)
	return writeRuntimeConfigFile(path, content)
}

func writeHyprEntrypointConfig(path, target, shellProfile, label string) error {
	content := fmt.Sprintf("# Active Hyprland profile: %s\nsource = %s\nsource = %s\n", label, target, shellProfile)
	return writeRuntimeConfigFile(path, content)
}

func writeRuntimeSourceConfig(path, target, label string) error {
	content := fmt.Sprintf("# %s\nsource = %s\n", label, target)
	return writeRuntimeConfigFile(path, content)
}

func writeShellManagedRuntimePlaceholder(path, component, profile, detail string) error {
	content := fmt.Sprintf("# Active %s profile: shell-managed (%s)\n# %s\n", component, profile, detail)
	return writeRuntimeConfigFile(path, content)
}
