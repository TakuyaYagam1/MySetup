package dots

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/shellruntime"
)

func syncHypr(ctx context.Context, runner run.CommandRunner, dotsSrc, configDir string, state config.State) error {
	src := filepath.Join(dotsSrc, "hypr")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("hypr dots source not found: %s", src)
	}
	if err := validateHyprSource(src); err != nil {
		return err
	}
	dst := filepath.Join(configDir, "hypr")
	home := homeDirFromConfigDir(configDir)
	homeManagerOwnsStaticTree, err := activeHomeManagerOwnsHyprStaticTree(home, dst, state.Packages.Preset)
	if err != nil {
		return err
	}

	// Keep the active Home Manager generation coherent until linkGeneration
	// replaces its immutable links. Hyprland reloads Lua as watched files change.
	if homeManagerOwnsStaticTree {
		return nil
	}

	activeProfile := bootstrapActiveShellForUpgrade(home, dst)
	if err := prepareHyprDestination(ctx, runner, dst, state.User.Username); err != nil {
		return err
	}
	if err := pruneStaleEnd4ProfileDir(configDir, dst, activeProfile); err != nil {
		return err
	}
	if err := syncHyprDotfiles(ctx, runner, src, dst); err != nil {
		return err
	}
	if err := restoreActiveEnd4Profile(ctx, runner, configDir, dst, activeProfile); err != nil {
		return err
	}
	if err := finalizeHyprDestination(ctx, runner, src, dst, home, state); err != nil {
		return err
	}
	if err := makeHyprScriptsExecutable(ctx, runner, dst); err != nil {
		return err
	}
	return nil
}

func activeHomeManagerOwnsHyprStaticTree(home, hyprDir, preset string) (bool, error) {
	if !homeManagerOwnsHyprStaticTreeForPreset(preset) {
		return false, nil
	}

	rootInfo, err := os.Lstat(hyprDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("inspect active Hypr directory %s: %w", hyprDir, err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return false, nil
	}

	entrypoint := filepath.Join(hyprDir, "hyprland.lua")
	entrypointInfo, err := os.Lstat(entrypoint)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("inspect active Home Manager Hypr entrypoint %s: %w", entrypoint, err)
	}
	if entrypointInfo.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}
	linkTarget, err := os.Readlink(entrypoint)
	if err != nil {
		return false, fmt.Errorf("read active Home Manager Hypr entrypoint %s: %w", entrypoint, err)
	}
	managedTarget, ok := managedHomeManagerTopLevelRuntimeTarget(home, entrypoint, linkTarget)
	if !ok {
		return false, nil
	}

	expectedTarget, ok, err := activeHomeManagerHyprEntrypointTarget(home)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, fmt.Errorf("active Hypr entrypoint is Home Manager-owned but the current Home Manager generation is unavailable: %s", entrypoint)
	}
	if managedTarget != expectedTarget {
		return false, fmt.Errorf("active Hypr entrypoint does not belong to the current Home Manager generation: %s", entrypoint)
	}
	content, _, err := readRegularFileNoFollowResolved(managedTarget)
	if err != nil {
		return false, fmt.Errorf("read active Home Manager Hypr entrypoint %s: %w", managedTarget, err)
	}
	return isKnownTopLevelRuntimeEntrypoint(runtimePathSnapshot{
		kind:    runtimeSnapshotRegular,
		content: content,
	}, home), nil
}

func homeManagerOwnsHyprStaticTreeForPreset(preset string) bool {
	switch preset {
	case "desktop", "developer", "personal":
		return true
	default:
		return false
	}
}

func activeHomeManagerHyprEntrypointTarget(home string) (string, bool, error) {
	currentHome := filepath.Join(home, ".local", "state", "home-manager", "gcroots", "current-home")
	currentHomeInfo, err := os.Lstat(currentHome)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("inspect active Home Manager generation %s: %w", currentHome, err)
	}
	if currentHomeInfo.Mode()&os.ModeSymlink == 0 {
		return "", false, nil
	}
	homeFiles, err := filepath.EvalSymlinks(filepath.Join(currentHome, "home-files"))
	if err != nil {
		return "", false, fmt.Errorf("resolve active Home Manager files from %s: %w", currentHome, err)
	}
	if !filepath.IsAbs(homeFiles) {
		return "", false, nil
	}
	expectedTarget := filepath.Join(homeFiles, ".config", "hypr", "hyprland.lua")
	if !isImmutableNixStoreHomeManagerHyprTarget(expectedTarget, "hyprland.lua") {
		return "", false, nil
	}
	return expectedTarget, true, nil
}

func validateHyprSource(src string) error {
	for _, rel := range requiredHyprSourceFiles() {
		path := filepath.Join(src, rel)
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("required hypr source file missing: %s", path)
			}
			return fmt.Errorf("required hypr source file unreadable: %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("required hypr source path is not a regular file: %s", path)
		}
	}
	return nil
}

func requiredHyprSourceFiles() []string {
	files := make([]string, 0, 5+2*len(shellruntime.ProfileSpecs)+len(shellruntime.HyprScripts))
	files = append(files,
		"hyprland.lua",
		"end4-adapter.lua",
		"vm-keybinds.lua",
		filepath.Join("hyprland", "input.lua"),
		filepath.Join("hyprland", "keybinds.lua"),
		"shell-common-keybinds.lua",
		"shell-common-rules.lua",
		"shell-workspace-keybinds.lua",
	)
	for _, profile := range shellruntime.ProfileSpecs {
		files = append(files, profile.Launcher, profile.Keybinds)
	}
	for _, script := range shellruntime.HyprScripts {
		files = append(files, filepath.Join("scripts", script))
	}
	return files
}

func prepareHyprDestination(ctx context.Context, runner run.CommandRunner, dst, username string) error {
	if err := backupIfUnmanaged(ctx, runner, dst, "hypr"); err != nil {
		return err
	}
	if err := runner.Command(ctx, "mkdir", "-p", dst); err != nil {
		return err
	}
	return ensureUserWritableTree(ctx, runner, dst, username)
}

func syncHyprDotfiles(ctx context.Context, runner run.CommandRunner, src, dst string) error {
	return runner.Command(ctx, "rsync", "-a", "--delete",
		"--chmod=Du+rwX,Fu+rw,go+rX,go-w",
		"--exclude", "/hyprland.lua",
		"--exclude", "/hyprlock.conf",
		"--exclude", "/hypridle.conf",
		"--exclude", "/shell-profile.lua",
		"--exclude", "/shell-launcher.lua",
		"--exclude", "/shell-keybinds.lua",
		"--exclude", "/user/",
		"--exclude", "/wahrwelt/",
		"--exclude", "/mysetup/",
		"--exclude", "/runtime/",
		"--exclude", "/end4/",
		src+"/", dst+"/")
}

func finalizeHyprDestination(ctx context.Context, runner run.CommandRunner, src, dst, home string, state config.State) error {
	if err := ensureUserWritableTree(ctx, runner, dst, state.User.Username); err != nil {
		return err
	}
	if err := writeMarkerWithOwner(ctx, runner, filepath.Join(dst, ".wahrwelt-managed.json"), "hypr", state.User.Username); err != nil {
		return err
	}
	if runner.IsDryRun() {
		fmt.Println("write hypr local config and bootstrap active shell runtime state")
		return nil
	}
	if err := writeHyprLocalConfig(ctx, runner, state.User.Username, src, dst); err != nil {
		return err
	}
	return writeHyprRuntimeShellState(home, dst)
}

func makeHyprScriptsExecutable(ctx context.Context, runner run.CommandRunner, hyprDir string) error {
	scripts := filepath.Join(hyprDir, "scripts")
	if err := runner.Command(ctx, "chmod", "-R", "u+rwX", scripts); err != nil {
		return err
	}
	return runner.Command(ctx, "find", scripts, "-type", "f", "-exec", "chmod", "u+x", "{}", "+")
}
