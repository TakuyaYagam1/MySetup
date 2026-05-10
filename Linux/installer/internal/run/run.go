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

const (
	tailBufferSize      = 64 * 1024
	failureTailMaxLines = 60
	failureTailMaxBytes = 4096
)

// CommandRunner is the contract callers depend on to execute external
// processes. Tests can substitute a fake to record invocations or reshape
// dry-run behaviour without spawning real processes.
type CommandRunner interface {
	Command(ctx context.Context, name string, args ...string) error
	Output(ctx context.Context, name string, args ...string) (string, error)
	IsDryRun() bool
}

type Runner struct {
	DryRun bool
	Stdout io.Writer
	Stderr io.Writer
	Redact func(string) string
}

func New(dryRun bool) Runner {
	return Runner{DryRun: dryRun, Stdout: os.Stdout, Stderr: os.Stderr}
}

func (r Runner) IsDryRun() bool {
	return r.DryRun
}

func (r Runner) Command(ctx context.Context, name string, args ...string) error {
	stdout := writerOrDefault(r.Stdout, os.Stdout)
	stderr := writerOrDefault(r.Stderr, os.Stderr)
	if _, err := fmt.Fprintln(stdout, r.commandLog(name, args)); err != nil {
		return fmt.Errorf("write command log: %w", err)
	}
	if r.DryRun {
		return nil
	}
	cmd := exec.CommandContext(ctx, name, args...)
	stdoutBuf := newTailBuffer(tailBufferSize)
	stderrBuf := newTailBuffer(tailBufferSize)
	cmd.Stdout = io.MultiWriter(stdout, &stdoutBuf)
	cmd.Stderr = io.MultiWriter(stderr, &stderrBuf)
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w%s", name, err, formatCommandFailureOutput(stdoutBuf.String(), stderrBuf.String()))
	}
	return nil
}

func (r Runner) Output(ctx context.Context, name string, args ...string) (string, error) {
	stdout := writerOrDefault(r.Stdout, os.Stdout)
	if _, err := fmt.Fprintln(stdout, r.commandLog(name, args)); err != nil {
		return "", fmt.Errorf("write command log: %w", err)
	}
	if r.DryRun {
		return "", nil
	}
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	stderrBuf := newTailBuffer(tailBufferSize)
	cmd.Stdout = &out
	cmd.Stderr = io.MultiWriter(writerOrDefault(r.Stderr, os.Stderr), &stderrBuf)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s failed: %w%s", name, err, formatCommandFailureOutput(out.String(), stderrBuf.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

func (r Runner) commandLog(name string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, shellQuote(name))
	for _, arg := range args {
		parts = append(parts, shellQuote(arg))
	}
	line := "$ " + strings.Join(parts, " ")
	if r.Redact != nil {
		return r.Redact(line)
	}
	return line
}

func writerOrDefault(writer io.Writer, fallback io.Writer) io.Writer {
	if writer != nil {
		return writer
	}
	return fallback
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if isShellSafe(value) {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func isShellSafe(value string) bool {
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case strings.ContainsRune("_@%+=:,./-", r):
		default:
			return false
		}
	}
	return true
}

func formatCommandFailureOutput(stdout, stderr string) string {
	stdout = tailText(stdout, failureTailMaxLines, failureTailMaxBytes)
	stderr = tailText(stderr, failureTailMaxLines, failureTailMaxBytes)
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

type tailBuffer struct {
	buf      []byte
	maxBytes int
}

func newTailBuffer(maxBytes int) tailBuffer {
	return tailBuffer{maxBytes: maxBytes}
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	if b.maxBytes <= 0 {
		return len(p), nil
	}
	b.buf = append(b.buf, p...)
	if len(b.buf) > b.maxBytes {
		copy(b.buf, b.buf[len(b.buf)-b.maxBytes:])
		b.buf = b.buf[:b.maxBytes]
	}
	return len(p), nil
}

func (b tailBuffer) String() string {
	return string(b.buf)
}
