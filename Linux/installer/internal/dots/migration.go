package dots

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
)

func migrateLegacyUserPaths(ctx context.Context, runner run.CommandRunner, home string) error {
	configHome := filepath.Join(home, ".config")
	stateHome := paths.XDGStateHome(home)
	cacheHome := filepath.Join(home, ".cache")
	if value := os.Getenv("XDG_CONFIG_HOME"); value != "" {
		configHome = value
	}
	if value := os.Getenv("XDG_CACHE_HOME"); value != "" {
		cacheHome = value
	}

	for _, pair := range [][2]string{
		{filepath.Join(configHome, "mysetup"), filepath.Join(configHome, "wahrwelt")},
		{filepath.Join(configHome, "hypr", "mysetup"), filepath.Join(configHome, "hypr", "wahrwelt")},
		{filepath.Join(stateHome, "mysetup"), filepath.Join(stateHome, "wahrwelt")},
		{filepath.Join(cacheHome, "mysetup"), filepath.Join(cacheHome, "wahrwelt")},
	} {
		if err := moveLegacyPath(ctx, runner, pair[0], pair[1]); err != nil {
			return err
		}
	}

	for _, path := range []string{
		filepath.Join(configHome, "hypr", "lib", "mysetup.lua"),
		filepath.Join(configHome, "quickshell", "mysetup-shell-selector"),
	} {
		info, err := os.Lstat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("wahrwelt migration conflict: refusing to remove non-symlink %s", path)
		}
		if err := runner.Command(ctx, "rm", "-f", "--", path); err != nil {
			return err
		}
	}
	return nil
}

func moveLegacyPath(ctx context.Context, runner run.CommandRunner, oldPath, newPath string) error {
	if _, err := os.Lstat(oldPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if _, err := os.Lstat(newPath); err == nil {
		return fmt.Errorf("wahrwelt migration conflict: both %s and %s exist", oldPath, newPath)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := runner.Command(ctx, "mkdir", "-p", filepath.Dir(newPath)); err != nil {
		return err
	}
	return runner.Command(ctx, "mv", "--", oldPath, newPath)
}
