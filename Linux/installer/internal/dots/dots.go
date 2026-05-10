package dots

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

type Options struct {
	Sources paths.Sources
	State   config.State
	DryRun  bool
	// Runner is the command executor used for all external invocations.
	// When nil, Apply falls back to run.New(DryRun) so callers (and tests)
	// can either inject a custom runner or rely on the default behaviour.
	Runner run.CommandRunner
}

func Apply(ctx context.Context, opts Options) error {
	runner := opts.Runner
	if runner == nil {
		runner = run.New(opts.DryRun)
	}
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
		if err := setupZen(ctx, runner, opts.Sources.Dots, home, opts.State.User.Username, opts.State.Dots); err != nil {
			return err
		}
	}
	if opts.State.Dots.Neovim {
		if err := syncNvim(ctx, runner, opts.Sources.Dots, configDir, opts.State.User.Username, opts.State.Dots.NeovimCleanState); err != nil {
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

// copyWallpapers seeds ~/Pictures/Wallpapers from the in-repo Wallpapers/ tree
// so Hyprland/Caelestia/end4 wallpaper selectors discover them on first launch
// without forcing the home-manager activation to recopy on every switch.
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
		fmt.Printf("Wallpapers already exist in %s; skipping copy\n", dst)
		return nil
	}
	return runner.Command(ctx, "rsync", "-a", "--ignore-existing", src+"/", dst+"/")
}
