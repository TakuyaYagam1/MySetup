package dots

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/zenutil"
)

func setupZen(ctx context.Context, runner run.CommandRunner, dotsSrc, home, username string, cfg config.Dots) error {
	profile := zenutil.FindProfile(home)
	if profile == "" {
		fmt.Println("Zen Browser profile not found; launch Zen once and rerun mysetup dots/apply")
		return nil
	}
	chrome := filepath.Join(profile, "chrome")
	if cfg.ZenTheme {
		if err := setupZenTheme(ctx, runner, dotsSrc, chrome, username); err != nil {
			return err
		}
	}
	if cfg.Sine {
		if err := setupSineProfile(ctx, runner, chrome, username); err != nil {
			return err
		}
	}
	return nil
}

func setupZenTheme(ctx context.Context, runner run.CommandRunner, dotsSrc, chrome, username string) error {
	src := filepath.Join(dotsSrc, "zen", "chrome")
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("zen browser theme source missing: %s", src)
		}
		return fmt.Errorf("zen browser theme source unreadable: %w", err)
	}
	if err := backupIfUnmanaged(ctx, runner, chrome, "zen-chrome"); err != nil {
		return err
	}
	sourceHash, alreadyInstalled, err := managedSourceAlreadyInstalled(src, chrome, "zen-chrome", nil)
	if err != nil {
		return err
	}
	if alreadyInstalled {
		if err := writeMarkerWithOwnerAndSourceHash(ctx, runner, filepath.Join(chrome, ".mysetup-managed.json"), "zen-chrome", username, sourceHash); err != nil {
			return err
		}
		fmt.Printf("Zen chrome already exists in %s; skipping sync\n", chrome)
		return nil
	}
	if err := runner.Command(ctx, "mkdir", "-p", chrome); err != nil {
		return err
	}
	if err := ensureUserWritableTree(ctx, runner, chrome, username); err != nil {
		return err
	}
	if err := runner.Command(ctx, "rsync", "-a", "--delete", src+"/", chrome+"/"); err != nil {
		return err
	}
	if err := ensureUserWritableTree(ctx, runner, chrome, username); err != nil {
		return err
	}
	return writeMarkerWithOwnerAndSourceHash(ctx, runner, filepath.Join(chrome, ".mysetup-managed.json"), "zen-chrome", username, sourceHash)
}

func setupSineProfile(ctx context.Context, runner run.CommandRunner, chrome, username string) error {
	fmt.Println("Sine profile install is intentionally pinned and best-effort")
	if sineProfileAlreadyInstalled(chrome) {
		fmt.Printf("Sine profile files already exist in %s; skipping download\n", chrome)
		return nil
	}
	tmp, err := os.MkdirTemp("", "mysetup-sine-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(tmp)
	}()

	archives, ok := downloadSineArchives(ctx, runner, tmp)
	if !ok {
		return nil
	}
	if err := runner.Command(ctx, "mkdir", "-p", chrome); err != nil {
		return err
	}
	if err := ensureUserWritableTree(ctx, runner, chrome, username); err != nil {
		return err
	}
	if runner.IsDryRun() {
		printSineArchivePlan(archives, chrome)
		return nil
	}
	if err := verifySineArchives(archives); err != nil {
		return err
	}
	if err := extractSineArchives(archives, chrome); err != nil {
		return err
	}
	if err := ensureUserWritableTree(ctx, runner, chrome, username); err != nil {
		return err
	}
	fmt.Println("Sine profile part installed; clear Zen startup cache from about:support, then restart Zen Browser")
	return nil
}
