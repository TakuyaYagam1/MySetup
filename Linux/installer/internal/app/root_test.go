package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyMissingStateFailsClosed(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--state", filepath.Join(t.TempDir(), "missing.json"),
		"apply",
		"--dry-run",
		"--no-switch",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected apply to fail when state is missing")
	}
	if !strings.Contains(err.Error(), "state file does not exist") {
		t.Fatalf("expected missing state error, got %v", err)
	}
}

func TestPrintStateMissingStateFailsClosed(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--state", filepath.Join(t.TempDir(), "missing.json"),
		"print-state",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected print-state to fail when state is missing")
	}
	if !strings.Contains(err.Error(), "state file does not exist") {
		t.Fatalf("expected missing state error, got %v", err)
	}
}

func TestApplyRejectsPlainPasswordFlags(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"apply",
		"--user-password",
		"secret",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected plain password flag to be rejected")
	}
	if !strings.Contains(err.Error(), "unknown flag: --user-password") {
		t.Fatalf("expected unknown flag error, got %v", err)
	}
}

func TestLoadSecretsFromFiles(t *testing.T) {
	dir := t.TempDir()
	userPassword := filepath.Join(dir, "user-password")
	pgAdminPassword := filepath.Join(dir, "pgadmin-password")
	if err := os.WriteFile(userPassword, []byte("linux-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pgAdminPassword, []byte("pg-secret\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	secrets, err := loadSecretsFromFiles(userPassword, pgAdminPassword)
	if err != nil {
		t.Fatal(err)
	}
	if secrets.UserPassword != "linux-secret" || secrets.PgAdminPassword != "pg-secret" {
		t.Fatalf("unexpected secrets: %#v", secrets)
	}
}

func TestReadSecretFileRejectsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(path, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := readSecretFile(path)
	if err == nil {
		t.Fatal("expected empty secret file error")
	}
	if !strings.Contains(err.Error(), "secret file is empty") {
		t.Fatalf("expected empty secret error, got %v", err)
	}
}

func TestReadSecretFileRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")
	if err := os.WriteFile(target, []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	_, err := readSecretFile(link)
	if err == nil {
		t.Fatal("expected symlink secret rejection")
	}
	if !strings.Contains(err.Error(), "must not be a symlink") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
}

func TestReadSecretFileRejectsNonRegularFile(t *testing.T) {
	_, err := readSecretFile(t.TempDir())
	if err == nil {
		t.Fatal("expected non-regular secret file rejection")
	}
	if !strings.Contains(err.Error(), "must be a regular file") {
		t.Fatalf("expected non-regular rejection, got %v", err)
	}
}

func TestReadSecretFileRejectsOpenPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte("secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := readSecretFile(path)
	if err == nil {
		t.Fatal("expected permissive secret file rejection")
	}
	if !strings.Contains(err.Error(), "permissions are too open") {
		t.Fatalf("expected permission rejection, got %v", err)
	}
}

func TestReadSecretFileRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", maxSecretFileBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := readSecretFile(path)
	if err == nil {
		t.Fatal("expected oversized secret file rejection")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected size rejection, got %v", err)
	}
}

func TestLoadDoctorStateFallsBackOnInvalidState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{ nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	state, err := loadDoctorState(path)
	if err != nil {
		t.Fatal(err)
	}
	if state.Packages.Preset != "personal" {
		t.Fatalf("expected default doctor fallback state, got %#v", state)
	}
}

func TestMakeCheckCleansBuildArtifact(t *testing.T) {
	data, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "$(MAKE) clean") {
		t.Fatalf("check target should run clean after build\n%s", text)
	}
	if strings.Contains(text, "check: fmt-check lint vet test build nix-build") {
		t.Fatalf("check target should not use dependency form that leaves bin/mysetup behind\n%s", text)
	}
}

func TestMakeCheckRunsHomeManagerShellEval(t *testing.T) {
	data, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "nix-hm-eval") {
		t.Fatalf("make check should include HM shell eval target\n%s", text)
	}
	if !strings.Contains(text, "seedMySetupHyprConfig") || !strings.Contains(text, `"hypr/shell-profile.conf"`) {
		t.Fatalf("nix-hm-eval should cover the runtime shell module\n%s", text)
	}
	if !strings.Contains(text, `"hypr/end4"`) {
		t.Fatalf("nix-hm-eval should also validate the end4 Home Manager profile\n%s", text)
	}
}

func TestMakeCheckRunsInstalledMirrorBuild(t *testing.T) {
	data, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"nix-installed-mirror-build",
		`rsync -a ../NixOS/ "$$tmp"/`,
		`rsync -a ../dots/ "$$tmp"/dots/`,
		`rsync -a --exclude=/bin/ --exclude=/tmp/ --exclude=/coverage.out --exclude=/coverage.html ./ "$$tmp"/installer/`,
		`nix build --no-link --no-write-lock-file "path:$$tmp#mysetup"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("make check should include installed mirror build evidence %q\n%s", want, text)
		}
	}
}
