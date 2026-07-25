package apply

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/defaults"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

var (
	nixCacheSubstituters = strings.Join(defaults.ExtraSubstituters, " ")
	nixCacheTrustedKeys  = strings.Join(defaults.ExtraTrustedPublicKeys, " ")
)

const (
	safeBuildMaxJobs = "1"
	safeBuildCores   = "2"
)

func dryBuildSystem(ctx context.Context, runner run.CommandRunner, flakePath, hostname string) error {
	target := fmt.Sprintf("path:%s#%s", flakePath, hostname)
	if err := runner.Command(ctx, "sudo", nixosRebuildArgs("dry-build", target)...); err != nil {
		return err
	}
	return nil
}

func switchSystem(ctx context.Context, runner run.CommandRunner, opts Options) (bool, error) {
	target := fmt.Sprintf("path:%s#%s", opts.Paths.NixOSDest, opts.State.Host.Hostname)
	if opts.SkipSwitch {
		fmt.Println("dry-build passed; --no-switch set, stopping before activation")
		return false, nil
	}
	if opts.AssumeYes {
		if err := runner.Command(ctx, "sudo", nixosRebuildArgs("switch", target)...); err != nil {
			return false, err
		}
		return true, nil
	}
	confirm, err := confirmSwitch()
	if err != nil {
		return false, err
	}
	if !confirm {
		fmt.Println("switch skipped")
		return false, nil
	}
	if err := runner.Command(ctx, "sudo", nixosRebuildArgs("switch", target)...); err != nil {
		return false, err
	}
	return true, nil
}

//nolint:misspell // Nix names this option extra-substituters.
func nixosRebuildArgs(action, target string) []string {
	return []string{
		"nixos-rebuild",
		action,
		"--impure",
		"--flake", target,
		"--option", "max-jobs", safeBuildMaxJobs,
		"--option", "cores", safeBuildCores,
		"--option", "extra-substituters", nixCacheSubstituters,
		"--option", "extra-trusted-public-keys", nixCacheTrustedKeys,
	}
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
