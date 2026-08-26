package dots

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/shellruntime"
)

func restoreActiveEnd4Profile(ctx context.Context, runner run.CommandRunner, configDir, hyprDir, activeProfile string) error {
	if !shellruntime.IsEnd4Profile(activeProfile) {
		return nil
	}
	return restoreEnd4ProfileLinkFromHomeManager(ctx, runner, configDir, hyprDir, activeProfile)
}

func pruneStaleEnd4ProfileDir(configDir, hyprDir, activeProfile string) error {
	if shellruntime.IsEnd4Profile(activeProfile) {
		return nil
	}
	target := filepath.Join(hyprDir, "end4")
	if _, err := os.Lstat(target); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	sources, err := shellruntime.ProvenEnd4SourcesFromHomeManager(configDir)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return fmt.Errorf("refusing to mutate unowned End4 profile collision without an exact Home Manager source: %s", target)
	}
	return validateEnd4TargetOwnership(target, sources)
}

func validateEnd4TargetOwnership(target string, sources []string) error {
	return shellruntime.ValidateEnd4TargetOwnership(target, sources)
}

func restoreEnd4ProfileLinkFromHomeManager(ctx context.Context, runner run.CommandRunner, configDir, hyprDir, activeProfile string) error {
	target := filepath.Join(hyprDir, "end4")
	source, err := shellruntime.End4SourceForProfileFromHomeManager(configDir, activeProfile)
	if err != nil {
		return err
	}
	_, err = os.Lstat(target)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if source == "" {
			return fmt.Errorf("exact Home Manager End4 source is unavailable for active profile %s", activeProfile)
		}
		return runner.Command(ctx, "ln", "-s", "--", source, target)
	}
	sources, err := shellruntime.ProvenEnd4SourcesFromHomeManager(configDir)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return fmt.Errorf("refusing to mutate unowned End4 profile collision without an exact Home Manager source: %s", target)
	}
	return validateEnd4TargetOwnership(target, sources)
}
