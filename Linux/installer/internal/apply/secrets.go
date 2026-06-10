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

func prepareStagingHostLocal(ctx context.Context, runner run.CommandRunner, staging, dest string, secrets config.Secrets, layout Layout) error {
	if layout == LayoutThin {
		if err := copyExistingThinHostLocal(ctx, runner, staging, dest); err != nil {
			return err
		}
	}
	if err := copyHostHardware(staging, dest, layout); err != nil {
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
		target := filepath.Join(staging, hashedPasswordRel(layout))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, []byte(HashedPasswordNix(hash)), 0o600)
	}
	return copyExistingHashedPassword(ctx, runner, existingHashedPasswordPaths(dest), filepath.Join(staging, hashedPasswordRel(layout)))
}

func copyExistingThinHostLocal(ctx context.Context, runner run.CommandRunner, staging, dest string) error {
	thinFlake, err := copyExistingThinFlake(dest, filepath.Join(staging, "flake.nix"))
	if err != nil {
		return err
	}

	preservedFiles := []string{}
	if thinFlake {
		preservedFiles = []string{"flake.lock", "configuration.nix", "home.nix"}
	}
	if err := copyExistingThinHostLocalFiles(dest, staging, preservedFiles); err != nil {
		return err
	}

	if err := copyExistingThinHostLocalDir(ctx, runner, dest, staging, "private"); err != nil {
		return err
	}
	if err := writePrivateDefaultTemplate(staging); err != nil {
		return err
	}

	return copyExistingThinSecrets(ctx, runner, dest, staging)
}

func copyExistingThinHostLocalFiles(dest, staging string, names []string) error {
	for _, name := range names {
		source := filepath.Join(dest, name)
		if _, err := os.Stat(source); err == nil {
			if err := copyFile(source, filepath.Join(staging, name)); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func copyExistingThinSecrets(ctx context.Context, runner run.CommandRunner, dest, staging string) error {
	for _, secretsDir := range existingSecretsDirs(dest) {
		if info, err := os.Stat(secretsDir); err == nil {
			if !info.IsDir() {
				return fmt.Errorf("secrets path is not a directory: %s", secretsDir)
			}
			if err := copyHostLocalTree(ctx, runner, secretsDir, filepath.Join(staging, "secrets")); err != nil {
				return fmt.Errorf("stage secrets: %w", err)
			}
			return nil
		} else if os.IsPermission(err) {
			if err := sudoCopyHostLocalTree(ctx, runner, secretsDir, filepath.Join(staging, "secrets")); err != nil {
				return fmt.Errorf("stage secrets: %w", err)
			}
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func copyExistingThinHostLocalDir(ctx context.Context, runner run.CommandRunner, dest, staging, name string) error {
	source := filepath.Join(dest, name)
	info, err := os.Stat(source)
	if err != nil {
		if os.IsPermission(err) {
			if err := sudoCopyHostLocalTree(ctx, runner, source, filepath.Join(staging, name)); err != nil {
				return fmt.Errorf("stage %s/: %w", name, err)
			}
			return nil
		}
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s path is not a directory: %s", name, source)
	}
	if err := copyHostLocalTree(ctx, runner, source, filepath.Join(staging, name)); err != nil {
		return fmt.Errorf("stage %s/: %w", name, err)
	}
	return nil
}

func copyExistingThinFlake(dest, target string) (bool, error) {
	source := filepath.Join(dest, "flake.nix")
	data, err := os.ReadFile(source)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !isThinWrapperFlake(string(data)) {
		return false, nil
	}
	return true, copyFile(source, target)
}

func isThinWrapperFlake(text string) bool {
	return strings.Contains(text, "mysetup.lib.mkMySetupHost") ||
		strings.Contains(text, "github:TakuyaYagam1/MySetup?dir=Linux/NixOS")
}

func copyHostLocalTree(ctx context.Context, runner run.CommandRunner, source, target string) error {
	if err := copyTree(ctx, runner, source, target); err != nil {
		if sudoErr := sudoCopyHostLocalTree(ctx, runner, source, target); sudoErr != nil {
			return err
		}
	}
	return nil
}

func sudoCopyHostLocalTree(ctx context.Context, runner run.CommandRunner, source, target string) error {
	uid := strconv.Itoa(os.Getuid())
	gid := strconv.Itoa(os.Getgid())
	if err := runner.Command(ctx, "sudo", "mkdir", "-p", target); err != nil {
		return err
	}
	return runner.Command(
		ctx,
		"sudo",
		"rsync",
		"-a",
		"--delete",
		"--checksum",
		"--chown",
		uid+":"+gid,
		source+"/",
		target+"/",
	)
}

func existingHashedPasswordPaths(dest string) []string {
	return []string{
		filepath.Join(dest, "hashed-password.nix"),
		filepath.Join(dest, "hosts", "NixOS", "hashed-password.nix"),
	}
}

func existingSecretsDirs(dest string) []string {
	return []string{
		filepath.Join(dest, "secrets"),
		filepath.Join(dest, "hosts", "NixOS", "secrets"),
	}
}

func copyExistingHashedPassword(ctx context.Context, runner run.CommandRunner, sources []string, target string) error {
	for _, source := range sources {
		copied, err := copyExistingHashedPasswordFile(ctx, runner, source, target)
		if err != nil || copied {
			return err
		}
	}
	return nil
}

func copyExistingHashedPasswordFile(ctx context.Context, runner run.CommandRunner, source, target string) (bool, error) {
	if _, err := os.Stat(source); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		if os.IsPermission(err) {
			return true, sudoCopyHostLocalFile(ctx, runner, source, target)
		}
		return false, err
	}
	if err := copyFile(source, target); err != nil {
		if os.IsPermission(err) {
			return true, sudoCopyHostLocalFile(ctx, runner, source, target)
		}
		return false, err
	}
	return true, nil
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

func writeStagedHashedPassword(ctx context.Context, runner run.CommandRunner, staging, dest string, secrets config.Secrets, layout Layout) error {
	source := filepath.Join(staging, hashedPasswordRel(layout))
	if _, err := os.Stat(source); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	target := filepath.Join(dest, hashedPasswordRel(layout))
	if secrets.UserPassword == "" {
		if info, err := os.Stat(target); err == nil {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("target %s is not a regular file", hashedPasswordRel(layout))
			}
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if runner.IsDryRun() {
		fmt.Printf("write %s\n", hashedPasswordRel(layout))
		return nil
	}
	if err := runner.Command(ctx, "sudo", "install", "-D", "-m", "600", source, target); err != nil {
		return err
	}
	return runner.Command(ctx, "sudo", "chown", "root:root", target)
}

func writeStagedSecrets(ctx context.Context, runner run.CommandRunner, staging, dest string, layout Layout) error {
	if layout != LayoutThin {
		return nil
	}
	source := filepath.Join(staging, "secrets")
	info, err := os.Stat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("staged secrets path is not a directory: %s", source)
	}
	target := filepath.Join(dest, "secrets")
	if targetInfo, err := os.Stat(target); err == nil {
		if !targetInfo.IsDir() {
			return fmt.Errorf("target secrets path is not a directory: %s", target)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if runner.IsDryRun() {
		fmt.Println("write secrets/")
		return nil
	}
	if err := runner.Command(ctx, "sudo", "mkdir", "-p", target); err != nil {
		return err
	}
	return runner.Command(
		ctx,
		"sudo",
		"rsync",
		"-a",
		"--delete",
		"--checksum",
		"--chown",
		"root:root",
		source+"/",
		target+"/",
	)
}
