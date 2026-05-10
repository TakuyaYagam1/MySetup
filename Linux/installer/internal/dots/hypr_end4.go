package dots

import (
	"context"
	"os"
	"path/filepath"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/shellruntime"
)

func restoreActiveEnd4Profile(ctx context.Context, runner run.CommandRunner, configDir, hyprDir, activeProfile string) error {
	if activeProfile != "end4" {
		return nil
	}
	return restoreEnd4ProfileLinkFromHomeManager(ctx, runner, configDir, hyprDir)
}

func pruneStaleEnd4ProfileDir(ctx context.Context, runner run.CommandRunner, hyprDir, activeProfile string) error {
	if activeProfile == "end4" {
		return nil
	}
	end4Dir := filepath.Join(hyprDir, "end4")
	info, err := os.Lstat(end4Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	if !info.IsDir() {
		return nil
	}
	if ok, err := shellruntime.RuntimeConfigExists(filepath.Join(end4Dir, "hyprland.conf")); err != nil || ok {
		return err
	}
	return runner.Command(ctx, "rm", "-rf", "--", end4Dir)
}

func restoreEnd4ProfileLinkFromHomeManager(ctx context.Context, runner run.CommandRunner, configDir, hyprDir string) error {
	target := filepath.Join(hyprDir, "end4")
	if ok, err := shellruntime.RuntimeConfigExists(filepath.Join(target, "hyprland.conf")); err != nil || ok {
		return err
	}
	source, err := shellruntime.End4SourceFromHomeManager(configDir)
	if err != nil || source == "" {
		return err
	}
	if err := removeInvalidEnd4Target(ctx, runner, target); err != nil {
		return err
	}
	return runner.Command(ctx, "ln", "-sfn", source, target)
}

func removeInvalidEnd4Target(ctx context.Context, runner run.CommandRunner, target string) error {
	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return runner.Command(ctx, "rm", "-f", "--", target)
	}
	if info.IsDir() {
		return runner.Command(ctx, "rm", "-rf", "--", target)
	}
	return nil
}
