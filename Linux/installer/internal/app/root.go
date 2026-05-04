package app

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/apply"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/cleanup"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/doctor"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/tui"
)

func NewRootCommand() *cobra.Command {
	opts := Options{Options: paths.DefaultOptions()}

	root := &cobra.Command{
		Use:   "mysetup",
		Short: "Catppuccin TUI installer for MySetup",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return tui.Run(cmd.Context(), tui.Options{
				Paths:  opts.Options,
				DryRun: opts.DryRun,
			})
		},
	}

	root.PersistentFlags().StringVar(&opts.RepoRoot, "repo", "", "MySetup repository root")
	root.PersistentFlags().StringVar(&opts.NixOSDest, "dest", opts.NixOSDest, "NixOS destination directory")
	root.PersistentFlags().StringVar(&opts.StatePath, "state", opts.StatePath, "machine-local state file")
	root.PersistentFlags().StringVar(&opts.DraftPath, "draft", opts.DraftPath, "user draft state file")
	root.PersistentFlags().BoolVar(&opts.DryRun, "dry-run", false, "print actions without changing files")
	root.PersistentFlags().BoolVar(&opts.Yes, "yes", false, "skip confirmation prompts where safe")

	root.AddCommand(tuiCommand(&opts))
	root.AddCommand(applyCommand(&opts))
	root.AddCommand(doctorCommand(&opts))
	root.AddCommand(cleanupCommand(&opts))
	root.AddCommand(printStateCommand(&opts))

	return root
}

func tuiCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open the interactive installer",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return tui.Run(cmd.Context(), tui.Options{
				Paths:  opts.Options,
				DryRun: opts.DryRun,
			})
		},
	}
}

func applyCommand(opts *Options) *cobra.Command {
	var noSwitch bool
	var userPassword string
	var pgAdminPassword string

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply saved state to /etc/nixos and user dotfiles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			state, err := config.LoadExisting(opts.StatePath)
			if err != nil {
				return err
			}
			return apply.Run(cmd.Context(), apply.Options{
				Paths:      opts.Options,
				State:      state,
				Secrets:    config.Secrets{UserPassword: userPassword, PgAdminPassword: pgAdminPassword},
				DryRun:     opts.DryRun,
				AssumeYes:  opts.Yes,
				SkipSwitch: noSwitch,
			})
		},
	}
	cmd.Flags().BoolVar(&noSwitch, "no-switch", false, "stop after dry-build and do not run nixos-rebuild switch")
	cmd.Flags().StringVar(&userPassword, "user-password", "", "initial user password for hashed-password.nix")
	cmd.Flags().StringVar(&pgAdminPassword, "pgadmin-password", "", "pgAdmin password written to /etc/nixos/secrets")
	return cmd
}

func doctorCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check current MySetup installation health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			state, err := loadDoctorState(opts.StatePath)
			if err != nil {
				return err
			}
			return doctor.Run(cmd.Context(), doctor.Options{Paths: opts.Options, State: state})
		},
	}
}

func loadDoctorState(path string) (config.State, error) {
	state, err := config.Load(path)
	if err == nil {
		return state, nil
	}
	if _, writeErr := fmt.Fprintf(os.Stderr, "WARN could not load state %s: %v\n", path, err); writeErr != nil {
		return config.State{}, writeErr
	}
	return config.Default(), nil
}

func cleanupCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup",
		Short: "Clean safe managed leftovers",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cleanup.Run(cmd.Context(), cleanup.Options{
				Paths:  opts.Options,
				DryRun: opts.DryRun,
				Yes:    opts.Yes,
			})
		},
	}
}

func printStateCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "print-state",
		Short: "Print the saved machine state as JSON",
		RunE: func(_ *cobra.Command, _ []string) error {
			state, err := config.LoadExisting(opts.StatePath)
			if err != nil {
				return err
			}
			data, err := json.MarshalIndent(state, "", "  ")
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(os.Stdout, string(data))
			return err
		},
	}
}
