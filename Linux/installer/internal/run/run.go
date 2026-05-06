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
	stdout := writerOrDefault(r.Stdout, os.Stdout)
	stderr := writerOrDefault(r.Stderr, os.Stderr)
	if _, err := fmt.Fprintf(stdout, "$ %s %s\n", name, strings.Join(args, " ")); err != nil {
		return fmt.Errorf("write command log: %w", err)
	}
	if r.DryRun {
		return nil
	}
	cmd := exec.CommandContext(ctx, name, args...)
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(stdout, &stdoutBuf)
	cmd.Stderr = io.MultiWriter(stderr, &stderrBuf)
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w%s", name, err, formatCommandFailureOutput(stdoutBuf.String(), stderrBuf.String()))
	}
	return nil
}

func (r Runner) Output(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = writerOrDefault(r.Stderr, os.Stderr)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s failed: %w", name, err)
	}
	return strings.TrimSpace(out.String()), nil
}

func writerOrDefault(writer io.Writer, fallback io.Writer) io.Writer {
	if writer != nil {
		return writer
	}
	return fallback
}

func formatCommandFailureOutput(stdout, stderr string) string {
	stdout = tailText(stdout, 60, 4096)
	stderr = tailText(stderr, 60, 4096)
	switch {
	case stdout == "" && stderr == "":
		return ""
	case stdout == "" && stderr != "":
		return "\nstderr:\n" + stderr
	case stdout != "" && (stderr == "" || stderr == stdout):
		return "\noutput:\n" + stdout
	default:
		return "\nstdout:\n" + stdout + "\nstderr:\n" + stderr
	}
}

func tailText(text string, maxLines, maxBytes int) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	truncated := false
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
		truncated = true
	}
	joined := strings.Join(lines, "\n")
	if len(joined) > maxBytes {
		joined = joined[len(joined)-maxBytes:]
		truncated = true
	}
	if !truncated {
		return joined
	}
	return "[last command output]\n" + joined
}
