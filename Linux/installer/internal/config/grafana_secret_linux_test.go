//go:build linux

package config

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

const grafanaSecretHelper = "../../../NixOS/services/grafana-secret-key.py"

func runGrafanaSecretHelper(t *testing.T, directory string, migrationSource ...string) ([]byte, error) {
	t.Helper()
	args := make([]string, 0, 4+len(migrationSource))
	args = append(args,
		grafanaSecretHelper,
		directory,
		fmt.Sprint(os.Geteuid()),
		fmt.Sprint(os.Getegid()),
	)
	args = append(args, migrationSource...)
	return exec.Command("python3", args...).CombinedOutput()
}

func makeGrafanaSecretDirectory(t *testing.T) string {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "grafana")
	if err := os.Mkdir(directory, 0o750); err != nil {
		t.Fatal(err)
	}
	return directory
}

func TestGrafanaSecretFreshPublicationIsStableAndPrivate(t *testing.T) {
	directory := makeGrafanaSecretDirectory(t)
	if output, err := runGrafanaSecretHelper(t, directory); err != nil {
		t.Fatalf("fresh secret: %v\n%s", err, output)
	}
	path := filepath.Join(directory, "secret_key")
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	beforeHash := sha256.Sum256(payload)
	if before.Mode().Perm() != 0o640 || len(payload) != 65 || payload[64] != '\n' {
		t.Fatalf("secret metadata/content = mode %04o len %d", before.Mode().Perm(), len(payload))
	}
	if output, err := runGrafanaSecretHelper(t, directory); err != nil {
		t.Fatalf("repeat secret validation: %v\n%s", err, output)
	}
	after, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	afterPayload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	beforeSys := before.Sys().(*syscall.Stat_t)
	afterSys := after.Sys().(*syscall.Stat_t)
	if beforeSys.Ino != afterSys.Ino || before.Mode() != after.Mode() || beforeHash != sha256.Sum256(afterPayload) {
		t.Fatal("repeat activation changed the existing Grafana secret")
	}
}

func TestGrafanaSecretPreservesSymlinkCollision(t *testing.T) {
	directory := makeGrafanaSecretDirectory(t)
	outside := filepath.Join(t.TempDir(), "outside")
	want := []byte("outside user data\n")
	if err := os.WriteFile(outside, want, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(directory, "secret_key")); err != nil {
		t.Fatal(err)
	}
	output, err := runGrafanaSecretHelper(t, directory)
	if err == nil || !strings.Contains(string(output), "ownership collision") {
		t.Fatalf("symlink collision accepted: err=%v\n%s", err, output)
	}
	got, readErr := os.ReadFile(outside)
	if readErr != nil || !bytes.Equal(got, want) {
		t.Fatalf("symlink target changed: %q err=%v", got, readErr)
	}
}

func TestGrafanaSecretMigratesOnlyPinnedRegularV1Source(t *testing.T) {
	directory := makeGrafanaSecretDirectory(t)
	legacyDirectory := makeGrafanaSecretDirectory(t)
	legacy := filepath.Join(legacyDirectory, "secret_key")
	want := []byte(strings.Repeat("a", 64) + "\n")
	if err := os.WriteFile(legacy, want, 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := runGrafanaSecretHelper(t, directory, legacy); err != nil {
		t.Fatalf("migrate v1 secret: %v\n%s", err, output)
	}
	got, err := os.ReadFile(filepath.Join(directory, "secret_key"))
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("migrated secret = %q err=%v", got, err)
	}
	legacyAfter, err := os.ReadFile(legacy)
	if err != nil || !bytes.Equal(legacyAfter, want) {
		t.Fatalf("v1 source changed: %q err=%v", legacyAfter, err)
	}
}

func TestGrafanaSecretRejectsV1SymlinkWithoutPublishing(t *testing.T) {
	directory := makeGrafanaSecretDirectory(t)
	legacyDirectory := makeGrafanaSecretDirectory(t)
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, []byte(strings.Repeat("b", 64)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(legacyDirectory, "secret_key")
	if err := os.Symlink(outside, legacy); err != nil {
		t.Fatal(err)
	}
	output, err := runGrafanaSecretHelper(t, directory, legacy)
	if err == nil || !strings.Contains(string(output), "ownership collision") {
		t.Fatalf("v1 symlink accepted: err=%v\n%s", err, output)
	}
	if _, statErr := os.Lstat(filepath.Join(directory, "secret_key")); !os.IsNotExist(statErr) {
		t.Fatalf("canonical secret was published after v1 collision: %v", statErr)
	}
}
