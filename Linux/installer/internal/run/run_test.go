package run

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestCommandIncludesCapturedOutputOnFailure(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Runner{Stdout: &stdout, Stderr: &stderr}.Command(
		context.Background(),
		"sh",
		"-c",
		"printf 'out\\n'; printf 'err\\n' >&2; exit 42",
	)
	if err == nil {
		t.Fatal("expected command failure")
	}

	text := err.Error()
	for _, want := range []string{
		"sh failed: exit status 42",
		"stdout:\nout",
		"stderr:\nerr",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected error to contain %q, got:\n%s", want, text)
		}
	}
}

func TestOutputHonorsDryRun(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	out, err := Runner{DryRun: true, Stdout: &stdout, Stderr: &stderr}.Output(
		context.Background(),
		"sh",
		"-c",
		"printf real-output",
	)
	if err != nil {
		t.Fatalf("dry-run output should not fail: %v", err)
	}
	if out != "" {
		t.Fatalf("dry-run output should be empty, got %q", out)
	}
	if got := stdout.String(); !strings.Contains(got, "$ sh -c 'printf real-output'") {
		t.Fatalf("expected dry-run command log, got %q", got)
	}
}

func TestResolveCommandNamePrefersNixOSSudoWrapper(t *testing.T) {
	const sudoWrapper = "/run/wrappers/bin/sudo"

	got := resolveCommandName("sudo")
	info, err := os.Stat(sudoWrapper)
	if err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0 {
		if got != sudoWrapper {
			t.Fatalf("expected sudo wrapper %q, got %q", sudoWrapper, got)
		}
		return
	}
	if got != "sudo" {
		t.Fatalf("expected fallback sudo, got %q", got)
	}
}

func TestResolveCommandNameLeavesNonSudoCommandsUntouched(t *testing.T) {
	if got := resolveCommandName("nixos-rebuild"); got != "nixos-rebuild" {
		t.Fatalf("expected non-sudo command unchanged, got %q", got)
	}
}

func TestCommandLogQuotesAndRedacts(t *testing.T) {
	var stdout bytes.Buffer

	err := Runner{
		DryRun: true,
		Stdout: &stdout,
		Redact: func(text string) string {
			return strings.ReplaceAll(text, "secret", "REDACTED")
		},
	}.Command(context.Background(), "printf", "hello world", "secret")
	if err != nil {
		t.Fatal(err)
	}

	got := stdout.String()
	for _, want := range []string{
		"$ printf 'hello world' REDACTED",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected quoted/redacted log %q, got %q", want, got)
		}
	}
}

func TestOutputIncludesCapturedStderrOnFailure(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	_, err := Runner{Stdout: &stdout, Stderr: &stderr}.Output(
		context.Background(),
		"sh",
		"-c",
		"printf 'out\\n'; printf 'err\\n' >&2; exit 42",
	)
	if err == nil {
		t.Fatal("expected output failure")
	}

	text := err.Error()
	for _, want := range []string{
		"sh failed: exit status 42",
		"stdout:\nout",
		"stderr:\nerr",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected error to contain %q, got:\n%s", want, text)
		}
	}
}

func TestTailBufferKeepsBoundedSuffix(t *testing.T) {
	buf := newTailBuffer(5)
	if _, err := buf.Write([]byte("1234")); err != nil {
		t.Fatal(err)
	}
	if _, err := buf.Write([]byte("56789")); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "56789" {
		t.Fatalf("unexpected tail buffer contents: got %q", got)
	}
}
