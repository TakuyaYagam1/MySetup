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
	return applyWithHooks(ctx, opts, applyHooks{})
}

type applyHooks struct {
	migration    legacyUserMigrationHooks
	finalRuntime runtimeMutationHook
}

func applyWithHooks(ctx context.Context, opts Options, hooks applyHooks) error {
	runner := opts.Runner
	if runner == nil {
		runner = run.New(opts.DryRun)
	}
	home, err := managedHome(opts.State.User.HomeDirectory)
	if err != nil {
		return err
	}
	configDir := filepath.Join(home, ".config")
	hyprDir := filepath.Join(configDir, "hypr")
	hyprSyncDeferred, err := migrateAndFinalizeHyprUserRuntime(ctx, runner, home, hyprDir, opts.Sources.Dots, hooks)
	if err != nil {
		return err
	}

	if opts.State.Dots.Wallpapers {
		if err := copyWallpapers(ctx, runner, opts.Sources.NixOS, home); err != nil {
			return err
		}
	}
	if err := applyHyprDots(ctx, runner, opts, configDir, hyprSyncDeferred); err != nil {
		return err
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

func applyHyprDots(ctx context.Context, runner run.CommandRunner, opts Options, configDir string, deferred bool) error {
	if !opts.State.Dots.Hypr || deferred {
		return nil
	}
	return syncHypr(ctx, runner, opts.Sources.Dots, configDir, opts.State)
}

func migrateAndFinalizeHyprUserRuntime(
	ctx context.Context,
	runner run.CommandRunner,
	home, hyprDir, dotsSource string,
	hooks applyHooks,
) (bool, error) {
	var transition hyprUserRuntimeTransition
	prepared := false
	migrationHooks := hooks.migration
	injectedHyprHook := migrationHooks.hypr
	migrationHooks.hypr = func(stage hyprUserMigrationCommitStage, migration hyprUserMigration) error {
		if !runner.IsDryRun() {
			var err error
			transition, err = publishHyprUserNamespaceTransition(home, hyprDir, dotsSource)
			if err != nil {
				return err
			}
			prepared = true
		}
		if injectedHyprHook != nil {
			return injectedHyprHook(stage, migration)
		}
		return nil
	}
	if err := migrateLegacyUserPathsWithHooks(ctx, runner, home, migrationHooks); err != nil {
		return false, err
	}
	if !runner.IsDryRun() {
		if !prepared {
			var err error
			transition, err = publishHyprUserNamespaceTransition(home, hyprDir, dotsSource)
			if err != nil {
				return false, err
			}
		}
		if err := finalizeHyprUserNamespaceRuntime(home, hyprDir, dotsSource, transition, hooks.finalRuntime); err != nil {
			return false, err
		}
		return transition.directEnd4Deferred, nil
	}
	return false, nil
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
