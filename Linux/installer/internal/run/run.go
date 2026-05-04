package run

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type Runner struct {
	DryRun bool
	Stdout io.Writer
	Stderr io.Writer
}

func New(dryRun bool) Runner {
	return Runner{DryRun: dryRun, Stdout: os.Stdout, Stderr: os.Stderr}
}

func (r Runner) Command(ctx context.Context, name string, args ...string) error {
	if _, err := fmt.Fprintf(r.Stdout, "$ %s %s\n", name, strings.Join(args, " ")); err != nil {
		return fmt.Errorf("write command log: %w", err)
	}
	if r.DryRun {
		return nil
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = r.Stdout
	cmd.Stderr = r.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}

func (r Runner) Output(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = r.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s failed: %w", name, err)
	}
	return strings.TrimSpace(out.String()), nil
}
