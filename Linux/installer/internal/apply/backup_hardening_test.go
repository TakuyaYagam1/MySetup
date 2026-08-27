package apply

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestMarkNixOSBackupUsesAllowlistedPrivilegedHelperOnlyForCanonicalDestination(t *testing.T) {
	runner := &fakeRunner{}
	if err := markNixOSBackup(context.Background(), runner, "/tmp/nixos", "/tmp/nixos.bak.1.2.3", "/nix/store/test-helper"); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("custom destination must not call privileged backup marker helper: %v", runner.calls)
	}
	if err := markNixOSBackup(context.Background(), runner, "/etc/nixos", "/etc/nixos.bak.1.2.3", "/nix/store/test-helper"); err != nil {
		t.Fatal(err)
	}
	got := commandSummary(runner.calls)
	want := "sudo /nix/store/test-helper mark-nixos-backup --backup /etc/nixos.bak.1.2.3"
	if !strings.Contains(got, want) {
		t.Fatalf("canonical backup marker command missing: got\n%s\nwant %s", got, want)
	}
}

func TestInaccessibleCanonicalPasswordHashUsesPrivilegedValidator(t *testing.T) {
	runner := &fakeRunner{}
	permissionDenied := func(path string) (os.FileInfo, error) {
		return nil, &os.PathError{Op: "lstat", Path: path, Err: os.ErrPermission}
	}
	configured, err := externalPasswordHashConfigured(
		context.Background(),
		runner,
		"/etc/wahrwelt/hashed-password",
		permissionDenied,
		func() (string, error) { return "/nix/store/test-helper", nil },
	)
	if err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("privileged validator accepted the canonical hash but it was not marked configured")
	}
	got := commandSummary(runner.calls)
	want := "sudo /nix/store/test-helper validate-password-hash --path /etc/wahrwelt/hashed-password"
	if !strings.Contains(got, want) {
		t.Fatalf("canonical external hash validation command missing: got\n%s\nwant %s", got, want)
	}
}

func TestInaccessibleCanonicalPasswordHashPropagatesValidatorFailure(t *testing.T) {
	runner := &fakeRunner{failOn: func(_ string, _ []string) error { return errors.New("invalid hash") }}
	permissionDenied := func(path string) (os.FileInfo, error) {
		return nil, &os.PathError{Op: "lstat", Path: path, Err: os.ErrPermission}
	}
	configured, err := externalPasswordHashConfigured(
		context.Background(),
		runner,
		"/etc/wahrwelt/hashed-password",
		permissionDenied,
		func() (string, error) { return "/nix/store/test-helper", nil },
	)
	if err == nil || !strings.Contains(err.Error(), "validate inaccessible external password hash") {
		t.Fatalf("validator failure = %v", err)
	}
	if configured {
		t.Fatal("failed validator marked the external password hash configured")
	}
}

func TestPruneNixOSBackupsUsesFixedRetentionOnlyForCanonicalDestination(t *testing.T) {
	runner := &fakeRunner{}
	if err := pruneNixOSBackups(context.Background(), runner, "/tmp/nixos", "/nix/store/test-helper"); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("custom destination must not call privileged backup prune helper: %v", runner.calls)
	}
	if err := pruneNixOSBackups(context.Background(), runner, "/etc/nixos", "/nix/store/test-helper"); err != nil {
		t.Fatal(err)
	}
	got := commandSummary(runner.calls)
	want := "sudo /nix/store/test-helper prune-nixos-backups --parent /etc --keep 3"
	if !strings.Contains(got, want) {
		t.Fatalf("canonical backup prune command missing: got\n%s\nwant %s", got, want)
	}
}

func TestValidateExternalPasswordHashUsesPrivilegedHelperOnlyForCanonicalTarget(t *testing.T) {
	runner := &fakeRunner{}
	if err := validateExternalPasswordHashWithHelper(context.Background(), runner, "/tmp/hashed-password", "/nix/store/test-helper"); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("custom target must not call privileged hash validator: %v", runner.calls)
	}
	if err := validateExternalPasswordHashWithHelper(context.Background(), runner, "/etc/wahrwelt/hashed-password", "/nix/store/test-helper"); err != nil {
		t.Fatal(err)
	}
	got := commandSummary(runner.calls)
	want := "sudo /nix/store/test-helper validate-password-hash --path /etc/wahrwelt/hashed-password"
	if !strings.Contains(got, want) {
		t.Fatalf("canonical external hash validation command missing: got\n%s\nwant %s", got, want)
	}
}

func TestCanonicalPasswordHashHelperUsesStatusAndSealedDescriptorWithoutHashArguments(t *testing.T) {
	hash := "$6$rounds=5000$testsalt$" + strings.Repeat("A", 86)
	identity := "1:2:33152:1:0:regular"
	runner := &fakeRunner{output: identity}
	status, err := privilegedPasswordHashStatus(
		context.Background(),
		runner,
		"/nix/store/test-helper",
		"/etc/wahrwelt/hashed-password",
	)
	if err != nil || status != identity {
		t.Fatalf("password hash status = %q, err=%v", status, err)
	}
	if err := publishPrivilegedExternalPasswordHash(
		context.Background(),
		runner,
		"/nix/store/test-helper",
		"/etc/wahrwelt/hashed-password",
		hash,
		status,
	); err != nil {
		t.Fatal(err)
	}
	summary := commandSummary(runner.calls)
	for _, want := range []string{
		"sudo /nix/store/test-helper password-hash-status --path /etc/wahrwelt/hashed-password",
		"sudo /nix/store/test-helper publish-password-hash --path /etc/wahrwelt/hashed-password --source /proc/",
		"--expected " + identity,
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("canonical helper command missing %q:\n%s", want, summary)
		}
	}
	if strings.Contains(summary, hash) {
		t.Fatal("raw password hash leaked into privileged helper arguments")
	}
}
