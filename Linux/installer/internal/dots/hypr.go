package dots

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/shellruntime"
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
	activeProfile := shellruntime.BootstrapActiveShell(home, dst)

	if err := prepareHyprDestination(ctx, runner, dst, state.User.Username); err != nil {
		return err
	}
	if err := pruneStaleEnd4ProfileDir(ctx, runner, dst, activeProfile); err != nil {
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
	reloadHyprBestEffort(ctx, runner)
	return nil
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
		"hyprland.conf",
		filepath.Join("hyprland", "input.conf"),
		filepath.Join("hyprland", "keybinds.conf"),
		"shell-common-keybinds.conf",
		"shell-workspace-keybinds.conf",
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
	// --chmod overrides source perms: nix-store dirs ship 555 and would otherwise
	// propagate, locking us out of subsequent symlink/file creation in the dest.
	return runner.Command(ctx, "rsync", "-a", "--delete",
		"--chmod=Du+rwX,Fu+rw,go+rX,go-w",
		"--exclude", "/hyprland.conf",
		"--exclude", "/hyprlock.conf",
		"--exclude", "/hypridle.conf",
		"--exclude", "/shell-profile.conf",
		"--exclude", "/shell-launcher.conf",
		"--exclude", "/shell-keybinds.conf",
		"--exclude", "/runtime/",
		"--exclude", "/end4/",
		src+"/", dst+"/")
}

func finalizeHyprDestination(ctx context.Context, runner run.CommandRunner, src, dst, home string, state config.State) error {
	if err := ensureUserWritableTree(ctx, runner, dst, state.User.Username); err != nil {
		return err
	}
	if err := writeMarkerWithOwner(ctx, runner, filepath.Join(dst, ".mysetup-managed.json"), "hypr", state.User.Username); err != nil {
		return err
	}
	if runner.IsDryRun() {
		fmt.Println("write hypr local config and bootstrap active shell runtime state")
		return nil
	}
	if err := writeHyprLocalConfig(ctx, runner, state, src, dst); err != nil {
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

func reloadHyprBestEffort(ctx context.Context, runner run.CommandRunner) {
	if _, err := exec.LookPath("hyprctl"); err != nil {
		return
	}
	if !hyprlandRunning(ctx) {
		return
	}
	if err := runner.Command(ctx, "hyprctl", "reload"); err != nil {
		fmt.Printf("WARN hyprctl reload failed: %v\n", err)
	}
}

func hyprlandRunning(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "hyprctl", "monitors")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}
