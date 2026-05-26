package apply

import (
	"context"
	"fmt"
	"os"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/defaults"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/dots"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

type Options struct {
	Paths      paths.Options
	State      config.State
	Secrets    config.Secrets
	DryRun     bool
	AssumeYes  bool
	SkipSwitch bool
	Layout     Layout
	// Runner is the command executor used during apply. When nil, Run falls
	// back to run.New(DryRun) so production callers keep the prior default
	// and tests can inject a recorder without spawning real processes.
	Runner run.CommandRunner
}

func Run(ctx context.Context, opts Options) error {
	if err := config.Validate(opts.State); err != nil {
		return err
	}
	layout, err := normalizeLayout(opts.Layout)
	if err != nil {
		return err
	}
	src, err := paths.ResolveSources(opts.Paths.RepoRoot)
	if err != nil {
		return err
	}

	runner := opts.Runner
	if runner == nil {
		runner = run.New(opts.DryRun)
	}
	fmt.Println("== MySetup apply ==")
	fmt.Printf("source: %s\n", src.RepoRoot)
	fmt.Printf("target: %s\n", opts.Paths.NixOSDest)
	fmt.Printf("layout: %s\n", layout)

	staging, err := os.MkdirTemp("", defaults.StagingTempPattern)
	if err != nil {
		return fmt.Errorf("create staging: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(staging)
	}()

	if err := stageConfiguration(ctx, runner, src, staging, opts.State, layout); err != nil {
		return err
	}
	if err := prepareStagingHostLocal(ctx, runner, staging, opts.Paths.NixOSDest, opts.Secrets, layout); err != nil {
		return err
	}
	if err := lockStagingFlake(ctx, runner, staging, layout); err != nil {
		return err
	}
	if err := dryBuildSystem(ctx, runner, staging, opts.State.Host.Hostname); err != nil {
		return fmt.Errorf("dry-build failed; /etc/nixos was not modified: %w", err)
	}
	if opts.SkipSwitch {
		fmt.Println("dry-build passed; --no-switch set, stopping before /etc/nixos or dotfile writes")
		return nil
	}
	result, err := writeSystemConfiguration(ctx, runner, staging, opts, layout)
	if err != nil {
		return handlePreSwitchError(ctx, runner, opts.Paths.NixOSDest, result.BackupPath, err)
	}
	if err := dots.Apply(ctx, dots.Options{
		Sources: src,
		State:   opts.State,
		DryRun:  opts.DryRun,
		Runner:  runner,
	}); err != nil {
		return handlePreSwitchError(ctx, runner, opts.Paths.NixOSDest, result.BackupPath, err)
	}
	switched, err := switchSystem(ctx, runner, opts)
	if err != nil {
		printRollbackHint(result.BackupPath, opts.Paths.NixOSDest)
		return err
	}
	if !switched {
		fmt.Println("state not written because system was not activated")
		return nil
	}
	return writeState(ctx, runner, opts.Paths.StatePath, opts.State)
}
