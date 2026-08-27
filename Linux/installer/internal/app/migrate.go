package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
	migrationv1tov2 "github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/migrations/v1_to_v2"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
)

func migrateCommand(opts *Options) *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate an installed legacy layout to Wahrwelt paths",
		RunE: func(cmd *cobra.Command, _ []string) error {
			legacy, err := legacyInstallPaths(opts)
			if err != nil {
				return err
			}
			if check {
				if len(legacy) == 0 {
					fmt.Println("Wahrwelt migration check: no managed legacy paths found")
					return nil
				}
				return fmt.Errorf("wahrwelt migration required: %s", strings.Join(legacy, ", "))
			}

			return runMigration(cmd.Context(), opts, run.New(opts.DryRun))
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "report managed legacy paths without changing them")
	return cmd
}

func runMigration(ctx context.Context, opts *Options, runner run.CommandRunner) error {
	// Restart, rather than start, so an already-active oneshot is forced to
	// process namespace migrations introduced by a newer generation.
	if err := runner.Command(ctx, "sudo", "systemctl", "restart", "wahrwelt-v1-to-v2-migration.service"); err != nil {
		return fmt.Errorf("restart system migration: %w; install the current generation with nixos-update first", err)
	}

	statePath, err := opts.ExistingStatePath()
	if err != nil {
		return err
	}
	state, err := config.LoadExisting(statePath)
	if err != nil {
		return err
	}
	unit := "home-manager-" + state.User.Username + ".service"
	if err := runner.Command(ctx, "sudo", "systemctl", "restart", unit); err != nil {
		return fmt.Errorf("activate migrated user paths: %w", err)
	}
	if !runner.IsDryRun() {
		remaining, err := legacyInstallPaths(opts)
		if err != nil {
			return err
		}
		if len(remaining) != 0 {
			return fmt.Errorf("wahrwelt migration incomplete; managed legacy paths remain: %s", strings.Join(remaining, ", "))
		}
	}
	fmt.Println("Wahrwelt migration completed")
	return nil
}

func legacyInstallPaths(opts *Options) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	configHome := filepath.Join(home, ".config")
	stateHome := filepath.Join(home, ".local", "state")
	cacheHome := filepath.Join(home, ".cache")
	if value := os.Getenv("XDG_CONFIG_HOME"); value != "" {
		configHome = value
	}
	if value := os.Getenv("XDG_STATE_HOME"); value != "" {
		stateHome = value
	}
	if value := os.Getenv("XDG_CACHE_HOME"); value != "" {
		cacheHome = value
	}

	candidates := migrationv1tov2.LegacyInstallPaths(opts.NixOSDest, configHome, stateHome, cacheHome)
	found := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if _, err := os.Lstat(candidate); err == nil {
			found = append(found, candidate)
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("inspect managed migration path %s: %w", candidate, err)
		}
	}
	return found, nil
}
