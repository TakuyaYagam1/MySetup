package dots

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

func setupV2rayN(ctx context.Context, runner run.CommandRunner, home string) error {
	root := filepath.Join(home, ".local", "share", "v2rayN")
	if info, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("v2rayN data directory not found in %s; skipping sing-box seed\n", root)
			return nil
		}
		return err
	} else if !info.IsDir() {
		return fmt.Errorf("v2rayN data path is not a directory: %s", root)
	}
	singbox, err := exec.LookPath("sing-box")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			fmt.Println("sing-box not found in PATH; skipping v2rayN binary seed")
			return nil
		}
		return fmt.Errorf("locate sing-box: %w", err)
	}
	dst := filepath.Join(root, "bin", "sing_box", "sing-box")
	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := runner.Command(ctx, "mkdir", "-p", filepath.Dir(dst)); err != nil {
		return err
	}
	return runner.Command(ctx, "install", "-m", "755", singbox, dst)
}
