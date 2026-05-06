package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

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
		Short: "Catppuccin Macchiato TUI installer for MySetup",
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
	var userPasswordFile string
	var pgAdminPasswordFile string

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply saved state to /etc/nixos and user dotfiles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			state, err := config.LoadExisting(opts.StatePath)
			if err != nil {
				return err
			}
			secrets, err := loadSecretsFromFiles(userPasswordFile, pgAdminPasswordFile)
			if err != nil {
				return err
			}
			return apply.Run(cmd.Context(), apply.Options{
				Paths:      opts.Options,
				State:      state,
				Secrets:    secrets,
				DryRun:     opts.DryRun,
				AssumeYes:  opts.Yes,
				SkipSwitch: noSwitch,
			})
		},
	}
	cmd.Flags().BoolVar(&noSwitch, "no-switch", false, "stop after dry-build and do not run nixos-rebuild switch")
	cmd.Flags().StringVar(&userPasswordFile, "user-password-file", "", "read initial user password from file for hashed-password.nix")
	cmd.Flags().StringVar(&pgAdminPasswordFile, "pgadmin-password-file", "", "read pgAdmin password from file for /etc/nixos/secrets")
	return cmd
}

func loadSecretsFromFiles(userPasswordFile, pgAdminPasswordFile string) (config.Secrets, error) {
	userPassword, err := readSecretFile(userPasswordFile)
	if err != nil {
		return config.Secrets{}, fmt.Errorf("read user password file: %w", err)
	}
	pgAdminPassword, err := readSecretFile(pgAdminPasswordFile)
	if err != nil {
		return config.Secrets{}, fmt.Errorf("read pgAdmin password file: %w", err)
	}
	return config.Secrets{UserPassword: userPassword, PgAdminPassword: pgAdminPassword}, nil
}

const maxSecretFileBytes = 64 * 1024

func readSecretFile(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	file, err := openSecretFileNoFollow(path)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close()
	}()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("secret file must not be a symlink: %s", path)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("secret file must be a regular file: %s", path)
	}
	if info.Size() > maxSecretFileBytes {
		return "", fmt.Errorf("secret file is too large: %s", path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("secret file permissions are too open: %s", path)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxSecretFileBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxSecretFileBytes {
		return "", fmt.Errorf("secret file is too large: %s", path)
	}
	secret := strings.TrimRight(string(data), "\r\n")
	if secret == "" {
		return "", fmt.Errorf("secret file is empty: %s", path)
	}
	return secret, nil
}

func openSecretFileNoFollow(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		if errors.Is(err, syscall.ELOOP) {
			return nil, fmt.Errorf("secret file must not be a symlink: %s", path)
		}
		return nil, err
	}
	//nolint:gosec // syscall.Open returns a valid non-negative file descriptor here; os.NewFile requires uintptr.
	return os.NewFile(uintptr(fd), path), nil
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
