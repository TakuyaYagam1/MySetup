package cleanup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
	migrationv1tov2 "github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/migrations/v1_to_v2"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/shellruntime"
)

type Options struct {
	Paths  paths.Options
	State  config.State
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
	home, err := Home(opts)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprint(out, ReportForHome(home)); err != nil {
		return err
	}
	if !opts.Yes {
		_, err := fmt.Fprintln(out, "Run with --yes to clean the safe managed candidates.")
		return err
	}
	if err := removePaths(ctx, runner, safeRemovablePaths(home)); err != nil {
		return err
	}
	if err := validateActiveEnd4ProfileLink(home); err != nil {
		return err
	}
	return nil
}

func Home(opts Options) (string, error) {
	if opts.State.User.HomeDirectory != "" {
		return opts.State.User.HomeDirectory, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return home, nil
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
		filepath.Join(home, ".cache/noctalia"),
		filepath.Join(home, ".cache/nvim/treesitter"),
		filepath.Join(home, ".local/share/nvim/treesitter"),
	}
}

func safeRemovablePaths(home string) []string {
	return []string{
		filepath.Join(home, ".cache/noctalia"),
		filepath.Join(home, ".cache/nvim/treesitter"),
		filepath.Join(home, ".local/share/nvim/treesitter"),
	}
}

func removePaths(ctx context.Context, runner run.CommandRunner, paths []string) error {
	for _, p := range paths {
		if _, err := os.Lstat(p); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if err := runner.Command(ctx, "rm", "-rf", "--", p); err != nil {
			return err
		}
	}
	return nil
}

func validateActiveEnd4ProfileLink(home string) error {
	profile := shellruntime.ReadActiveShell(shellruntime.ActiveShellStatePath(home))
	if profile == "" {
		profile = shellruntime.ReadActiveShell(
			migrationv1tov2.LegacyActiveShellStatePath(filepath.Join(home, ".local", "state")),
		)
	}
	if !shellruntime.IsEnd4Profile(profile) {
		return nil
	}
	configDir := filepath.Join(home, ".config")
	target := filepath.Join(configDir, "hypr", "end4")
	if _, err := os.Lstat(target); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	sources, err := shellruntime.ProvenEnd4SourcesFromHomeManager(configDir)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return fmt.Errorf("refusing to mutate unowned End4 profile collision without an exact Home Manager source: %s", target)
	}
	return shellruntime.ValidateEnd4TargetOwnership(target, sources)
}
