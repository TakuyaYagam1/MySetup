package apply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

type systemWriteResult struct {
	BackupPath string
}

func writeSystemConfiguration(ctx context.Context, runner run.CommandRunner, staging string, opts Options) (systemWriteResult, error) {
	var result systemWriteResult
	dest := opts.Paths.NixOSDest
	backupPath, err := backupExisting(ctx, runner, dest)
	if err != nil {
		return result, err
	}
	result.BackupPath = backupPath
	if err := preserveHardware(ctx, runner, dest); err != nil {
		return result, err
	}
	if err := syncToEtc(ctx, runner, staging, dest); err != nil {
		return result, err
	}
	if err := writeStagedHashedPassword(ctx, runner, staging, dest, opts.Secrets); err != nil {
		return result, err
	}
	if err := writeSecrets(ctx, runner, dest, opts.Secrets); err != nil {
		return result, err
	}
	return result, nil
}

func syncToEtc(ctx context.Context, runner run.CommandRunner, staging, dest string) error {
	return runner.Command(ctx, "sudo", syncToEtcArgs(staging, dest)...)
}

func syncToEtcArgs(staging, dest string) []string {
	return []string{
		"rsync", "-a", "--delete", "--checksum",
		"--exclude=/mysetup/",
		"--exclude=/secrets/",
		"--exclude=hardware-configuration.nix",
		"--exclude=hosts/NixOS/hardware-configuration.nix",
		"--exclude=hosts/NixOS/hashed-password.nix",
		"--exclude=hosts/NixOS/secrets/secrets.yaml",
		"--exclude=home/secrets/secrets.yaml",
		staging + "/",
		dest + "/",
	}
}

func preserveHardware(ctx context.Context, runner run.CommandRunner, dest string) error {
	rootHW := filepath.Join(dest, "hardware-configuration.nix")
	hostHW := filepath.Join(dest, "hosts/NixOS/hardware-configuration.nix")
	if _, err := os.Stat(rootHW); err == nil {
		return runner.Command(ctx, "sudo", "install", "-D", "-m", "644", rootHW, hostHW)
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Stat(hostHW); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return fmt.Errorf("hardware-configuration.nix not found; run sudo nixos-generate-config --root / first")
}

func writeState(ctx context.Context, runner run.CommandRunner, path string, state config.State) error {
	tmp, err := os.CreateTemp("", "mysetup-state-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp state file: %w", err)
	}
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if err := config.Save(tmpPath, state); err != nil {
		return err
	}
	if err := runner.Command(ctx, "sudo", "mkdir", "-p", filepath.Dir(path)); err != nil {
		return err
	}
	return runner.Command(ctx, "sudo", "install", "-m", "644", tmpPath, path)
}
