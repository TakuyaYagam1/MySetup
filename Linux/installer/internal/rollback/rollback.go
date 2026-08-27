package rollback

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
	"golang.org/x/sys/unix"
)

type Options struct {
	Paths  paths.Options
	Backup string
	DryRun bool
	Yes    bool
	Runner run.CommandRunner
}

//nolint:gocyclo // Selection, confirmation, descriptor pinning, and post-restore checks form one fail-closed operation.
func Run(ctx context.Context, opts Options) error {
	dest := opts.Paths.NixOSDest
	if dest == "" {
		return fmt.Errorf("destination path is empty; pass --dest or rely on default")
	}
	dest, err := filepath.Abs(dest)
	if err != nil {
		return fmt.Errorf("resolve destination path: %w", err)
	}
	if err := validateOrdinaryDirectory(dest, "destination"); err != nil {
		return err
	}
	backup := opts.Backup
	if backup == "" {
		latest, err := FindLatest(dest)
		if err != nil {
			return err
		}
		backup = latest
		fmt.Printf("Selected latest backup: %s\n", backup)
	} else {
		if err := validateBackupPath(backup, dest); err != nil {
			return err
		}
	}
	if err := validateOrdinaryDirectory(backup, "backup"); err != nil {
		return err
	}

	runner := opts.Runner
	if runner == nil {
		runner = run.New(opts.DryRun)
	}

	fmt.Println("== Wahrwelt rollback ==")
	fmt.Printf("backup: %s\n", backup)
	fmt.Printf("target: %s\n", dest)

	if !opts.Yes && !opts.DryRun {
		ok, err := confirm(backup, dest)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Println("rollback skipped")
			return nil
		}
	}

	destinationPin, err := pinOrdinaryDirectory(dest, "destination")
	if err != nil {
		return err
	}
	defer func() { _ = destinationPin.Close() }()
	backupPin, err := pinOrdinaryDirectory(backup, "backup")
	if err != nil {
		return err
	}
	defer func() { _ = backupPin.Close() }()
	if err := destinationPin.verifyVisible("destination"); err != nil {
		return err
	}
	if err := backupPin.verifyVisible("backup"); err != nil {
		return err
	}
	if err := runner.Command(
		ctx,
		"sudo",
		"rsync",
		"-a",
		"--delete",
		"--delete-excluded",
		"--exclude=/.wahrwelt-backup-v1",
		"--",
		backupPin.procPath(),
		destinationPin.procPath(),
	); err != nil {
		return fmt.Errorf("restore from %s: %w", backup, err)
	}
	if err := destinationPin.verifyVisible("destination after restore"); err != nil {
		return fmt.Errorf("rollback used a pinned destination but its visible path changed: %w", err)
	}
	if err := backupPin.verifyVisible("backup after restore"); err != nil {
		return fmt.Errorf("rollback source changed during restore: %w", err)
	}
	fmt.Println("Rollback complete.")
	fmt.Println("To roll the active system generation back as well, run:")
	fmt.Println("  sudo nixos-rebuild switch --rollback")
	return nil
}

type pinnedDirectory struct {
	file *os.File
	info os.FileInfo
	path string
}

func pinOrdinaryDirectory(path, label string) (*pinnedDirectory, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve %s path: %w", label, err)
	}
	if err := validateOrdinaryDirectory(absolute, label); err != nil {
		return nil, err
	}
	fd, err := unix.Open(absolute, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("pin %s %s: %w", label, absolute, err)
	}
	file, wrapErr := fileFromDescriptor(fd, label)
	if wrapErr != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("pin %s %s: %w", label, absolute, wrapErr)
	}
	info, err := file.Stat()
	if err != nil || !info.IsDir() {
		_ = file.Close()
		return nil, fmt.Errorf("pin %s %s: directory changed while opening", label, absolute)
	}
	pinned := &pinnedDirectory{file: file, info: info, path: absolute}
	if err := pinned.verifyVisible(label); err != nil {
		_ = file.Close()
		return nil, err
	}
	return pinned, nil
}

func fileFromDescriptor(fd int, label string) (*os.File, error) {
	if fd < 0 {
		return nil, fmt.Errorf("negative directory descriptor for %s", label)
	}
	file := os.NewFile(uintptr(fd), label)
	if file == nil {
		return nil, fmt.Errorf("wrap directory descriptor for %s", label)
	}
	return file, nil
}

func (p *pinnedDirectory) Close() error {
	if p == nil || p.file == nil {
		return nil
	}
	return p.file.Close()
}

func (p *pinnedDirectory) procPath() string {
	return "/proc/" + strconv.Itoa(os.Getpid()) + "/fd/" + strconv.FormatUint(uint64(p.file.Fd()), 10) + "/"
}

func (p *pinnedDirectory) verifyVisible(label string) error {
	if p == nil || p.file == nil {
		return fmt.Errorf("%s is not pinned", label)
	}
	current, err := os.Lstat(p.path)
	if err != nil {
		return fmt.Errorf("%s not accessible %s: %w", label, p.path, err)
	}
	if current.Mode()&os.ModeSymlink != 0 || !current.IsDir() || !os.SameFile(current, p.info) {
		return fmt.Errorf("%s changed after it was pinned: %s", label, p.path)
	}
	return nil
}

func FindLatest(dest string) (string, error) {
	dir := filepath.Dir(dest)
	base := filepath.Base(dest)
	pattern := filepath.Join(dir, base+".bak.*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("scan backups in %s: %w", dir, err)
	}
	candidates := make([]string, 0, len(matches))
	for _, m := range matches {
		info, err := os.Lstat(m)
		if err != nil || !info.IsDir() {
			continue
		}
		candidates = append(candidates, m)
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no backups matching %s found", pattern)
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, _ := os.Stat(candidates[i])
		b, _ := os.Stat(candidates[j])
		return a.ModTime().After(b.ModTime())
	})
	return candidates[0], nil
}

func validateOrdinaryDirectory(path, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("%s not accessible %s: %w", label, path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%s must be an ordinary directory: %s", label, path)
	}
	return nil
}

func validateBackupPath(backup, dest string) error {
	abs, err := filepath.Abs(backup)
	if err != nil {
		return err
	}
	parent := filepath.Dir(abs)
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	if parent != filepath.Dir(destAbs) {
		return fmt.Errorf("backup %s must live in %s", backup, filepath.Dir(destAbs))
	}
	if !strings.HasPrefix(filepath.Base(abs), filepath.Base(dest)+".bak.") {
		return fmt.Errorf("backup %s does not look like a %s.bak.* snapshot", backup, filepath.Base(dest))
	}
	return nil
}

func confirm(backup, dest string) (bool, error) {
	ok := false
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("Restore %s from %s? This rsync --deletes %s.", dest, backup, dest)).
			Value(&ok),
	))
	if err := form.Run(); err != nil {
		return false, err
	}
	return ok, nil
}
