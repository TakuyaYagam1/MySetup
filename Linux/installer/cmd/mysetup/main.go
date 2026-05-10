package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/app"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/shellruntime"
)

func main() {
	os.Exit(run())
}

func run() int {
	if err := shellruntime.ManifestError(); err != nil {
		fmt.Fprintln(os.Stderr, "embedded shell runtime manifest is broken:", err)
		return 1
	}
	// signal.NotifyContext lets long-running steps (rsync, nixos-rebuild) abort
	// promptly on Ctrl+C while still letting our defers (RemoveAll on staging,
	// temp file cleanup) run before exit.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := app.NewRootCommand().ExecuteContext(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "interrupted")
			return 130
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
