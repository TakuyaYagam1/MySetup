package apply

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/dots"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

type Options struct {
	Paths      paths.Options
	State      config.State
	Secrets    config.Secrets
	DryRun     bool
	AssumeYes  bool
	SkipSwitch bool
}

func Run(ctx context.Context, opts Options) error {
	if err := config.Validate(opts.State); err != nil {
		return err
	}
	src, err := paths.ResolveSources(opts.Paths.RepoRoot)
	if err != nil {
		return err
	}

	runner := run.New(opts.DryRun)
	fmt.Println("== MySetup apply ==")
	fmt.Printf("source: %s\n", src.RepoRoot)
	fmt.Printf("target: %s\n", opts.Paths.NixOSDest)

	staging, err := os.MkdirTemp("", "mysetup-nixos-*")
	if err != nil {
		return fmt.Errorf("create staging: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(staging)
	}()

	if err := stageConfiguration(ctx, runner, src, staging, opts.State); err != nil {
		return err
	}
	if err := writeSystemConfiguration(ctx, runner, src, staging, opts); err != nil {
		return err
	}
	if err := dots.Apply(ctx, dots.Options{
		Sources: src,
		State:   opts.State,
		Secrets: opts.Secrets,
		DryRun:  opts.DryRun,
		Yes:     opts.AssumeYes,
	}); err != nil {
		return err
	}
	return rebuildSystem(ctx, runner, opts)
}

func stageConfiguration(ctx context.Context, runner run.Runner, src paths.Sources, staging string, state config.State) error {
	if err := copyNixOS(ctx, runner, src.NixOS, staging, false); err != nil {
		return err
	}
	return writeGenerated(staging, state)
}

func writeSystemConfiguration(ctx context.Context, runner run.Runner, src paths.Sources, staging string, opts Options) error {
	dest := opts.Paths.NixOSDest
	if err := backupExisting(ctx, runner, dest); err != nil {
		return err
	}
	if err := preserveHardware(ctx, runner, dest); err != nil {
		return err
	}
	if err := syncToEtc(ctx, runner, staging, dest); err != nil {
		return err
	}
	if err := syncDotsToEtc(ctx, runner, src.Dots, dest); err != nil {
		return err
	}
	if err := seedFlakeLock(ctx, runner, staging, dest); err != nil {
		return err
	}
	if err := preserveHardware(ctx, runner, dest); err != nil {
		return err
	}
	if err := writeHashedPassword(ctx, runner, dest, opts.Secrets); err != nil {
		return err
	}
	if err := writeSecrets(ctx, runner, dest, opts.Secrets); err != nil {
		return err
	}
	return writeState(ctx, runner, opts.Paths.StatePath, opts.State)
}

func rebuildSystem(ctx context.Context, runner run.Runner, opts Options) error {
	target := fmt.Sprintf("%s#%s", opts.Paths.NixOSDest, opts.State.Host.Hostname)
	if err := runner.Command(ctx, "sudo", "nixos-rebuild", "dry-build", "--flake", target); err != nil {
		return fmt.Errorf("dry-build failed; /etc/nixos was written but not activated: %w", err)
	}
	if opts.SkipSwitch {
		fmt.Println("dry-build passed; --no-switch set, stopping before activation")
		return nil
	}
	if opts.AssumeYes {
		return runner.Command(ctx, "sudo", "nixos-rebuild", "switch", "--flake", target)
	}
	confirm, err := confirmSwitch()
	if err != nil {
		return err
	}
	if !confirm {
		fmt.Println("switch skipped")
		return nil
	}
	return runner.Command(ctx, "sudo", "nixos-rebuild", "switch", "--flake", target)
}

func confirmSwitch() (bool, error) {
	confirm := false
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title("Dry-build passed. Run nixos-rebuild switch now?").
			Value(&confirm),
	))
	if err := form.Run(); err != nil {
		return false, err
	}
	return confirm, nil
}

func copyNixOS(ctx context.Context, runner run.Runner, src, dst string, deleteExtra bool) error {
	args := []string{"-a"}
	if deleteExtra {
		args = append(args, "--delete")
	}
	args = append(args,
		"--exclude=/mysetup/",
		"--exclude=/secrets/",
		"--exclude=hosts/NixOS/hardware-configuration.nix",
		"--exclude=hosts/NixOS/hashed-password.nix",
		"--exclude=hosts/NixOS/secrets/secrets.yaml",
		"--exclude=home/secrets/secrets.yaml",
		src+"/",
		dst+"/",
	)
	return runner.Command(ctx, "rsync", args...)
}

func writeGenerated(staging string, state config.State) error {
	files := map[string]string{
		"hosts/NixOS/variables.nix": VariablesNix(state),
		"hosts/NixOS/default.nix":   HostDefaultNix(),
	}
	for rel, content := range files {
		path := filepath.Join(staging, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", rel, err)
		}
	}
	return nil
}

func hashPassword(ctx context.Context, password string) (string, error) {
	cmd := exec.CommandContext(ctx, "mkpasswd", "-sm", "sha-512", "--rounds=656000")
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

func backupExisting(ctx context.Context, runner run.Runner, dest string) error {
	if _, err := os.Stat(dest); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	backup := fmt.Sprintf("%s.bak.%d", dest, time.Now().Unix())
	return runner.Command(ctx, "sudo", "cp", "-a", dest, backup)
}

func syncToEtc(ctx context.Context, runner run.Runner, staging, dest string) error {
	return runner.Command(ctx, "sudo", syncToEtcArgs(staging, dest)...)
}

func syncToEtcArgs(staging, dest string) []string {
	return []string{
		"rsync", "-a", "--delete",
		"--exclude=/mysetup/",
		"--exclude=/secrets/",
		"--exclude=flake.lock",
		"--exclude=hardware-configuration.nix",
		"--exclude=hosts/NixOS/hardware-configuration.nix",
		"--exclude=hosts/NixOS/hashed-password.nix",
		"--exclude=hosts/NixOS/secrets/secrets.yaml",
		"--exclude=home/secrets/secrets.yaml",
		staging + "/",
		dest + "/",
	}
}

func preserveHardware(ctx context.Context, runner run.Runner, dest string) error {
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

func seedFlakeLock(ctx context.Context, runner run.Runner, staging, dest string) error {
	target := filepath.Join(dest, "flake.lock")
	if _, err := os.Stat(target); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	src := filepath.Join(staging, "flake.lock")
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return runner.Command(ctx, "sudo", "install", "-m", "644", src, target)
}

func syncDotsToEtc(ctx context.Context, runner run.Runner, srcDots, dest string) error {
	if _, err := os.Stat(srcDots); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	dst := filepath.Join(dest, "dots")
	srcAbs, err := filepath.Abs(srcDots)
	if err != nil {
		return err
	}
	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return err
	}
	if srcAbs == dstAbs {
		return nil
	}
	if err := runner.Command(ctx, "sudo", "mkdir", "-p", dst); err != nil {
		return err
	}
	return runner.Command(ctx, "sudo", "rsync", "-a", "--delete", srcDots+"/", dst+"/")
}

func writeSecrets(ctx context.Context, runner run.Runner, dest string, secrets config.Secrets) error {
	if secrets.PgAdminPassword == "" {
		return nil
	}
	secretDir := filepath.Join(dest, "secrets")
	if err := runner.Command(ctx, "sudo", "mkdir", "-p", secretDir); err != nil {
		return err
	}
	tmpPath, cleanup, err := writeTempString("mysetup-pgadmin-*", secrets.PgAdminPassword)
	if err != nil {
		return err
	}
	defer cleanup()
	target := filepath.Join(secretDir, "pgadmin-password")
	if err := runner.Command(ctx, "sudo", "install", "-m", "600", tmpPath, target); err != nil {
		return err
	}
	return runner.Command(ctx, "sudo", "chown", "-R", "root:root", secretDir)
}

func writeHashedPassword(ctx context.Context, runner run.Runner, dest string, secrets config.Secrets) error {
	if secrets.UserPassword == "" {
		return nil
	}
	hash, err := hashPassword(ctx, secrets.UserPassword)
	if err != nil {
		return err
	}
	tmpPath, cleanup, err := writeTempString("mysetup-hashed-password-*", HashedPasswordNix(hash))
	if err != nil {
		return err
	}
	defer cleanup()
	target := filepath.Join(dest, "hosts", "NixOS", "hashed-password.nix")
	if err := runner.Command(ctx, "sudo", "install", "-D", "-m", "600", tmpPath, target); err != nil {
		return err
	}
	return runner.Command(ctx, "sudo", "chown", "root:root", target)
}

func writeState(ctx context.Context, runner run.Runner, path string, state config.State) error {
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

func writeTempString(pattern, content string) (string, func(), error) {
	tmp, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, err
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = os.Remove(tmpPath)
	}
	if _, err := tmp.WriteString(content); err != nil {
		closeErr := tmp.Close()
		cleanup()
		if closeErr != nil {
			return "", nil, fmt.Errorf("write temp file: %w; close temp file: %w", err, closeErr)
		}
		return "", nil, fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("close temp file: %w", err)
	}
	return tmpPath, cleanup, nil
}
