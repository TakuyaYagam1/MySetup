package doctor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/zenutil"
)

type Options struct {
	Paths  paths.Options
	State  config.State
	Stdout io.Writer
}

func Run(ctx context.Context, opts Options) error {
	report, err := Report(ctx, opts)
	if err != nil {
		return err
	}
	out := opts.Stdout
	if out == nil {
		out = os.Stdout
	}
	_, err = fmt.Fprint(out, report)
	return err
}

func Report(ctx context.Context, opts Options) (string, error) {
	_ = ctx
	out := &reportWriter{}
	out.println("== MySetup doctor ==")
	check(out, "state", opts.Paths.StatePath)
	check(out, "hardware config", filepath.Join(opts.Paths.NixOSDest, "hosts/NixOS/hardware-configuration.nix"))
	check(out, "flake", filepath.Join(opts.Paths.NixOSDest, "flake.nix"))
	check(out, "host vars", filepath.Join(opts.Paths.NixOSDest, "hosts/NixOS/host-vars.nix"))
	checkShellRuntime(out, opts)
	checkWallpapers(out, opts)
	if zenutil.FindProfile(opts.State.User.HomeDirectory) == "" {
		out.println("WARN Zen profile not found")
	} else {
		out.println("OK   Zen profile found")
	}
	out.println("Last-resort system rollback: sudo rsync -a --delete /etc/nixos.bak.<timestamp>.<pid>.<n>/ /etc/nixos/")
	out.println("User dotfiles are not fully transactional; rerun apply or cleanup after checking ~/.config.")
	return out.String(), out.err
}

type reportWriter struct {
	bytes.Buffer
	err error
}

func (w *reportWriter) printf(format string, args ...any) {
	if w.err != nil {
		return
	}
	_, w.err = fmt.Fprintf(&w.Buffer, format, args...)
}

func (w *reportWriter) println(line string) {
	if w.err != nil {
		return
	}
	_, w.err = fmt.Fprintln(&w.Buffer, line)
}
