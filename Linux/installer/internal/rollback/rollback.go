package rollback

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

type Options struct {
	Paths  paths.Options
	Backup string
	DryRun bool
	Yes    bool
	Runner run.CommandRunner
}

func Run(ctx context.Context, opts Options) error {
	dest := opts.Paths.NixOSDest
	if dest == "" {
		return fmt.Errorf("destination path is empty; pass --dest or rely on default")
	}
	backup := opts.Backup
	if backup == "" {
		latest, err := FindLatest(dest)
		if err != nil {
			return err
		}
		backup = latest
		fmt.Printf("Selected latest backup: %s\n", backup)
	} else {
		if err := validateBackupPath(backup, dest); err != nil {
			return err
		}
	}
	if info, err := os.Stat(backup); err != nil {
		return fmt.Errorf("backup not accessible %s: %w", backup, err)
	} else if !info.IsDir() {
		return fmt.Errorf("backup is not a directory: %s", backup)
	}

	runner := opts.Runner
	if runner == nil {
		runner = run.New(opts.DryRun)
	}

	fmt.Println("== MySetup rollback ==")
	fmt.Printf("backup: %s\n", backup)
	fmt.Printf("target: %s\n", dest)

	if !opts.Yes && !opts.DryRun {
		ok, err := confirm(backup, dest)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Println("rollback skipped")
			return nil
		}
	}

	if err := runner.Command(ctx, "sudo", "rsync", "-a", "--delete", backup+"/", dest+"/"); err != nil {
		return fmt.Errorf("restore from %s: %w", backup, err)
	}
	fmt.Println("Rollback complete.")
	fmt.Println("To roll the active system generation back as well, run:")
	fmt.Println("  sudo nixos-rebuild switch --rollback")
	return nil
}

func FindLatest(dest string) (string, error) {
	dir := filepath.Dir(dest)
	base := filepath.Base(dest)
	pattern := filepath.Join(dir, base+".bak.*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("scan backups in %s: %w", dir, err)
	}
	candidates := make([]string, 0, len(matches))
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil || !info.IsDir() {
			continue
		}
		candidates = append(candidates, m)
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no backups matching %s found", pattern)
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, _ := os.Stat(candidates[i])
		b, _ := os.Stat(candidates[j])
		return a.ModTime().After(b.ModTime())
	})
	return candidates[0], nil
}

func validateBackupPath(backup, dest string) error {
	abs, err := filepath.Abs(backup)
	if err != nil {
		return err
	}
	parent := filepath.Dir(abs)
	if parent != filepath.Dir(dest) {
		return fmt.Errorf("backup %s must live in %s", backup, filepath.Dir(dest))
	}
	if !strings.HasPrefix(filepath.Base(abs), filepath.Base(dest)+".bak.") {
		return fmt.Errorf("backup %s does not look like a %s.bak.* snapshot", backup, filepath.Base(dest))
	}
	return nil
}

func confirm(backup, dest string) (bool, error) {
	ok := false
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("Restore %s from %s? This rsync --deletes %s.", dest, backup, dest)).
			Value(&ok),
	))
	if err := form.Run(); err != nil {
		return false, err
	}
	return ok, nil
}
