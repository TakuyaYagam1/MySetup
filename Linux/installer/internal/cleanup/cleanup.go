package cleanup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

type Options struct {
	Paths  paths.Options
	DryRun bool
	Yes    bool
}

func Run(ctx context.Context, opts Options) error {
	runner := run.New(opts.DryRun)
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	candidates := []string{
		filepath.Join(home, "Pictures/Wallpapers/preview-*"),
		filepath.Join(home, ".cache/noctalia/wallpapers.json"),
	}
	fmt.Println("== Safe cleanup candidates ==")
	for _, candidate := range candidates {
		fmt.Println(candidate)
	}
	if !opts.Yes {
		fmt.Println("Run with --yes to clean the safe managed candidates.")
		return nil
	}
	if err := runner.Command(ctx, "sh", "-c", fmt.Sprintf("rm -f %q", filepath.Join(home, ".cache/noctalia/wallpapers.json"))); err != nil {
		return err
	}
	wallpaperDir := filepath.Join(home, "Pictures/Wallpapers")
	if _, err := os.Stat(wallpaperDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return runner.Command(ctx, "find", wallpaperDir, "-maxdepth", "1", "-type", "f", "-name", "preview-*", "-delete")
}
