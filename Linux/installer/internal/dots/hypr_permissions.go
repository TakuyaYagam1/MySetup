package dots

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
)

func homeDirFromConfigDir(configDir string) string {
	return filepath.Dir(configDir)
}

func ensureUserWritableTree(ctx context.Context, runner run.CommandRunner, path, username string) error {
	if treeIsUserWritable(path) {
		return nil
	}
	if username == "" {
		return fmt.Errorf("%s is not writable and username is empty", path)
	}
	return repairUserWritableTree(ctx, runner, path, username)
}

func treeIsUserWritable(path string) bool {
	if !canWriteDir(path) {
		return false
	}
	err := filepath.WalkDir(path, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			if !canWriteDir(path) {
				return os.ErrPermission
			}
			return nil
		}
		if info.Mode().IsRegular() && !canWriteFile(path) {
			return os.ErrPermission
		}
		return nil
	})
	return err == nil
}

func repairUserWritableTree(ctx context.Context, runner run.CommandRunner, path, username string) error {
	if err := runner.Command(ctx, "sudo", "chown", "-R", username+":", path); err != nil {
		return err
	}
	return runner.Command(ctx, "chmod", "-R", "u+rwX", path)
}

func canWriteDir(path string) bool {
	file, err := os.CreateTemp(path, ".wahrwelt-write-test-*")
	if err != nil {
		return false
	}
	name := file.Name()
	closeErr := file.Close()
	removeErr := os.Remove(name)
	return closeErr == nil && removeErr == nil
}

func canWriteFile(path string) bool {
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	return file.Close() == nil
}
