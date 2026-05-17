package apply

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/defaults"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

func prepareStagingHostLocal(ctx context.Context, runner run.CommandRunner, staging, dest string, secrets config.Secrets) error {
	if err := copyHostHardware(staging, dest); err != nil {
		return err
	}
	if secrets.UserPassword != "" {
		hash := "!mysetup-dry-run-placeholder"
		if !runner.IsDryRun() {
			var err error
			hash, err = hashPassword(ctx, secrets.UserPassword)
			if err != nil {
				return err
			}
		}
		target := filepath.Join(staging, "hosts", "NixOS", "hashed-password.nix")
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, []byte(HashedPasswordNix(hash)), 0o600)
	}
	return copyExistingHashedPassword(
		ctx,
		runner,
		filepath.Join(dest, "hosts", "NixOS", "hashed-password.nix"),
		filepath.Join(staging, "hosts", "NixOS", "hashed-password.nix"),
	)
}

func copyExistingHashedPassword(ctx context.Context, runner run.CommandRunner, source, target string) error {
	if _, err := os.Stat(source); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		if os.IsPermission(err) {
			return sudoCopyHostLocalFile(ctx, runner, source, target)
		}
		return err
	}
	if err := copyFile(source, target); err != nil {
		if os.IsPermission(err) {
			return sudoCopyHostLocalFile(ctx, runner, source, target)
		}
		return err
	}
	return nil
}

func sudoCopyHostLocalFile(ctx context.Context, runner run.CommandRunner, source, target string) error {
	uid := strconv.Itoa(os.Getuid())
	gid := strconv.Itoa(os.Getgid())
	if err := runner.Command(ctx, "sudo", "install", "-D", "-m", "600", "-o", uid, "-g", gid, source, target); err != nil {
		return fmt.Errorf("copy existing hashed-password.nix with sudo: %w; provide --user-password-file to generate a fresh hash", err)
	}
	return nil
}

func hashPassword(ctx context.Context, password string) (string, error) {
	rounds := fmt.Sprintf("--rounds=%d", defaults.ShaCryptRounds)
	cmd := exec.CommandContext(ctx, "mkpasswd", "-sm", "sha-512", rounds)
	cmd.Stdin = strings.NewReader(password)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("mkpasswd failed: %w", err)
	}
	hash := strings.TrimSpace(string(out))
	if hash == "" {
		return "", fmt.Errorf("mkpasswd produced empty hash")
	}
	return hash, nil
}

func writeStagedHashedPassword(ctx context.Context, runner run.CommandRunner, staging, dest string, secrets config.Secrets) error {
	if secrets.UserPassword == "" {
		return nil
	}
	if runner.IsDryRun() {
		fmt.Println("write hosts/NixOS/hashed-password.nix")
		return nil
	}
	source := filepath.Join(staging, "hosts", "NixOS", "hashed-password.nix")
	target := filepath.Join(dest, "hosts", "NixOS", "hashed-password.nix")
	if err := runner.Command(ctx, "sudo", "install", "-D", "-m", "600", source, target); err != nil {
		return err
	}
	return runner.Command(ctx, "sudo", "chown", "root:root", target)
}
