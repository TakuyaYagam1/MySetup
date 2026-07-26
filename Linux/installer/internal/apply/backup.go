package apply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
)

func backupExisting(ctx context.Context, runner run.CommandRunner, dest string) (string, error) {
	if _, err := os.Stat(dest); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	backup, err := uniqueBackupPath(dest)
	if err != nil {
		return "", err
	}
	return backup, runner.Command(ctx, "sudo", "cp", "-a", dest, backup)
}

func uniqueBackupPath(target string) (string, error) {
	dir := filepath.Dir(target)
	base := filepath.Base(target)
	for attempt := 0; attempt < 100; attempt++ {
		backup := filepath.Join(dir, fmt.Sprintf("%s.bak.%d.%d.%d", base, time.Now().UnixNano(), os.Getpid(), attempt))
		if _, err := os.Stat(backup); err != nil {
			if os.IsNotExist(err) {
				return backup, nil
			}
			return "", err
		}
	}
	return "", fmt.Errorf("could not allocate unique backup path for %s", target)
}

func handlePreSwitchError(ctx context.Context, runner run.CommandRunner, dest, backupPath string, cause error) error {
	if backupPath == "" {
		return cause
	}
	if err := restoreBackup(ctx, runner, dest, backupPath); err != nil {
		return fmt.Errorf("%w; additionally failed to restore /etc/nixos from %s: %w; restore manually with: sudo rsync -a --delete %s/ %s/", cause, backupPath, err, backupPath, dest)
	}
	return fmt.Errorf("%w; restored /etc/nixos from %s", cause, backupPath)
}

func restoreBackup(ctx context.Context, runner run.CommandRunner, dest, backupPath string) error {
	fmt.Printf("WARN pre-switch apply failed; restoring /etc/nixos from %s\n", backupPath)
	if err := runner.Command(ctx, "sudo", "mkdir", "-p", dest); err != nil {
		return err
	}
	return runner.Command(ctx, "sudo", "rsync", "-a", "--delete", backupPath+"/", dest+"/")
}

func printRollbackHint(backupPath, dest string) {
	if backupPath == "" {
		return
	}
	fmt.Printf("WARN switch failed; state was not written. Restore manually with: sudo rsync -a --delete %s/ %s/\n", backupPath, dest)
}
