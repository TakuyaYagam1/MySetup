package apply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
	LockMode   LockMode
	// Runner is the command executor used during apply. When nil, Run falls
	// back to run.New(DryRun) so production callers keep the prior default
	// and tests can inject a recorder without spawning real processes.
	Runner run.CommandRunner
}

type applyModes struct {
	layout   Layout
	lockMode LockMode
}

func normalizeApplyModes(layout Layout, lockMode LockMode) (applyModes, error) {
	normalizedLayout, err := normalizeLayout(layout)
	if err != nil {
		return applyModes{}, err
	}
	normalizedLockMode, err := normalizeLockMode(lockMode)
	if err != nil {
		return applyModes{}, err
	}
	return applyModes{layout: normalizedLayout, lockMode: normalizedLockMode}, nil
}

func printApplyHeader(src paths.Sources, dest string, modes applyModes) {
	fmt.Println("== MySetup apply ==")
	fmt.Printf("source: %s\n", src.RepoRoot)
	fmt.Printf("target: %s\n", dest)
	fmt.Printf("layout: %s\n", modes.layout)
	if modes.layout == LayoutThin {
		fmt.Printf("lock mode: %s\n", modes.lockMode)
	}
}

func Run(ctx context.Context, opts Options) error {
	if err := config.Validate(opts.State); err != nil {
		return err
	}
	modes, err := normalizeApplyModes(opts.Layout, opts.LockMode)
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
	printApplyHeader(src, opts.Paths.NixOSDest, modes)

	staging, err := createStagingDir()
	if err != nil {
		return fmt.Errorf("create staging: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(staging)
	}()

	if err := stageConfiguration(ctx, runner, src, staging, opts.State, modes.layout, modes.lockMode); err != nil {
		return err
	}
	if err := prepareStagingHostLocal(ctx, runner, staging, opts.Paths.NixOSDest, opts.State.Source.Channel, opts.Secrets, modes.layout); err != nil {
		return err
	}
	if err := lockStagingFlake(ctx, runner, staging, modes.layout, modes.lockMode); err != nil {
		return err
	}
	if err := dryBuildSystem(ctx, runner, staging, opts.State.Host.Hostname); err != nil {
		return fmt.Errorf("dry-build failed; /etc/nixos was not modified: %w", err)
	}
	if opts.SkipSwitch {
		fmt.Println("dry-build passed; --no-switch set, stopping before /etc/nixos or dotfile writes")
		return nil
	}
	result, err := writeSystemConfiguration(ctx, runner, staging, opts, modes.layout)
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

func createStagingDir() (string, error) {
	base := stagingBaseDir()
	if err := os.MkdirAll(base, 0o700); err != nil {
		return "", err
	}
	return os.MkdirTemp(base, defaults.StagingTempPattern)
}

func stagingBaseDir() string {
	if cacheDir, err := os.UserCacheDir(); err == nil && cacheDir != "" && !isUnderPath(cacheDir, os.TempDir()) {
		return filepath.Join(cacheDir, "mysetup", "staging")
	}
	if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" && !isUnderPath(homeDir, os.TempDir()) {
		return filepath.Join(homeDir, ".cache", "mysetup", "staging")
	}
	return filepath.Join("/var/tmp", "mysetup-"+strconv.Itoa(os.Getuid()), "staging")
}

func isUnderPath(path, parent string) bool {
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		cleanPath = filepath.Clean(path)
	}
	cleanParent, err := filepath.Abs(parent)
	if err != nil {
		cleanParent = filepath.Clean(parent)
	}
	rel, err := filepath.Rel(cleanParent, cleanPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
