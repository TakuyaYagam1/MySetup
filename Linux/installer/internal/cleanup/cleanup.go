package cleanup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

type Options struct {
	Paths  paths.Options
	DryRun bool
	Yes    bool
	Stdout io.Writer
}

func Run(ctx context.Context, opts Options) error {
	runner := run.New(opts.DryRun)
	if opts.Stdout != nil {
		runner.Stdout = opts.Stdout
		runner.Stderr = opts.Stdout
	}
	out := opts.Stdout
	if out == nil {
		out = os.Stdout
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	if _, err := fmt.Fprint(out, ReportForHome(home)); err != nil {
		return err
	}
	if !opts.Yes {
		_, err := fmt.Fprintln(out, "Run with --yes to clean the safe managed candidates.")
		return err
	}
	if err := runner.Command(ctx, "rm", "-f", filepath.Join(home, ".cache/noctalia/wallpapers.json")); err != nil {
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

func Report() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return ReportForHome(home), nil
}

func ReportForHome(home string) string {
	lines := append([]string{"== Safe cleanup candidates =="}, candidates(home)...)
	return strings.Join(lines, "\n") + "\n"
}

func candidates(home string) []string {
	return []string{
		filepath.Join(home, "Pictures/Wallpapers/preview-*"),
		filepath.Join(home, ".cache/noctalia/wallpapers.json"),
	}
}
