package dots

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
)

func syncNvim(ctx context.Context, runner run.CommandRunner, dotsSrc, configDir, username string) error {
	src := filepath.Join(dotsSrc, "nvim")
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	dst := filepath.Join(configDir, "nvim")
	if err := backupIfUnmanaged(ctx, runner, dst, "nvim"); err != nil {
		return err
	}
	sourceHash, alreadyInstalled, err := managedSourceAlreadyInstalled(src, dst, "nvim", nil)
	if err != nil {
		return err
	}
	if alreadyInstalled {
		if err := writeMarkerWithOwnerAndSourceHash(ctx, runner, filepath.Join(dst, ".mysetup-managed.json"), "nvim", username, sourceHash); err != nil {
			return err
		}
		fmt.Printf("Neovim config already exists in %s; skipping sync\n", dst)
		return nil
	}
	if err := runner.Command(ctx, "mkdir", "-p", dst); err != nil {
		return err
	}
	if err := ensureUserWritableTree(ctx, runner, dst, username); err != nil {
		return err
	}
	if err := runner.Command(ctx, "rsync", "-a", "--delete", src+"/", dst+"/"); err != nil {
		return err
	}
	if err := ensureUserWritableTree(ctx, runner, dst, username); err != nil {
		return err
	}
	if err := writeMarkerWithOwnerAndSourceHash(ctx, runner, filepath.Join(dst, ".mysetup-managed.json"), "nvim", username, sourceHash); err != nil {
		return err
	}
	return nil
}
