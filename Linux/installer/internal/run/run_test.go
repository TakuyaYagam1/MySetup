package run

import (
	"bytes"
	"context"
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
