package dots

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

func backupIfUnmanaged(ctx context.Context, runner run.CommandRunner, target, kind string) error {
	inspection, err := InspectManagedRoot(target, kind)
	if err != nil {
		return err
	}
	if inspection.Status == ManagedRootMissing || inspection.Status == ManagedRootManaged {
		return nil
	}
	return backupTree(ctx, runner, target)
}

func backupTree(ctx context.Context, runner run.CommandRunner, target string) error {
	backup, err := uniqueManagedBackupPath(target)
	if err != nil {
		return err
	}
	fmt.Printf("Backing up unmanaged %s -> %s\n", target, backup)
	return runner.Command(ctx, "mv", target, backup)
}

func uniqueManagedBackupPath(target string) (string, error) {
	for attempt := range 100 {
		backup := fmt.Sprintf("%s.bak.%d.%d.%d", target, time.Now().UnixNano(), os.Getpid(), attempt)
		if _, err := os.Stat(backup); err != nil {
			if os.IsNotExist(err) {
				return backup, nil
			}
			return "", err
		}
	}
	return "", fmt.Errorf("could not allocate unique backup path for %s", target)
}
