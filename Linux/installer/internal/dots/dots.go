package dots

import (
	"context"
	"os"
	"path/filepath"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
)

type Options struct {
	Sources paths.Sources
	State   config.State
	DryRun  bool
	Runner  run.CommandRunner
}

func Apply(ctx context.Context, opts Options) error {
	runner := opts.Runner
	if runner == nil {
		runner = run.New(opts.DryRun)
	}
	home, err := managedHome(opts.State.User.HomeDirectory)
	if err != nil {
		return err
	}
	configDir := filepath.Join(home, ".config")
	if err := migrateLegacyUserPaths(ctx, runner, home); err != nil {
		return err
	}

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
		if err := setupZen(ctx, runner, opts.Sources.Dots, home, opts.State.User.Username, opts.State.Dots); err != nil {
			return err
		}
	}
	if opts.State.Dots.Neovim {
		if err := syncNvim(ctx, runner, opts.Sources.Dots, configDir, opts.State.User.Username); err != nil {
			return err
		}
	}
	if opts.State.Dots.V2rayN {
		if err := setupV2rayN(ctx, runner, home); err != nil {
			return err
		}
	}
	refreshThumbnailDaemons(ctx, runner, opts.State.User.Username, home)
	return nil
}

func managedHome(configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}
	return os.UserHomeDir()
}

func refreshThumbnailDaemons(ctx context.Context, runner run.CommandRunner, username, _ string) {
	if username == "" {
		return
	}
	for _, daemon := range []string{"gvfsd", "gvfsd-fuse", "Thunar", "thunar"} {
		_ = runner.Command(ctx, "pkill", "-u", username, "-x", daemon)
	}
	_ = runner.Command(ctx, "pkill", "-u", username, "-f", "gvfs-udisks2-volume-monitor")
	_ = runner.Command(ctx, "pkill", "-u", username, "-f", "tumbler-1/tumblerd")
}

func copyWallpapers(ctx context.Context, runner run.CommandRunner, nixosSrc, home string) error {
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
	if same, err := sourceSubsetMatches(src, dst, nil); err != nil {
		return err
	} else if same {
		return runner.Command(ctx, "chmod", "-R", "u+w", dst)
	}
	if err := runner.Command(ctx,
		"rsync", "-a", "--ignore-existing",
		"--chmod=Du=rwx,Dg=rx,Do=rx,Fu=rw,Fg=r,Fo=r",
		src+"/", dst+"/",
	); err != nil {
		return err
	}
	return runner.Command(ctx, "chmod", "-R", "u+w", dst)
}
