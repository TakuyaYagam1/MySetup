package apply

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

type fakeRunner struct {
	dryRun bool
	calls  []fakeCall
	failOn func(name string, args []string) error
}

type fakeCall struct {
	name string
	args []string
}

func (f *fakeRunner) Command(_ context.Context, name string, args ...string) error {
	f.calls = append(f.calls, fakeCall{name: name, args: append([]string(nil), args...)})
	if f.failOn != nil {
		return f.failOn(name, args)
	}
	return nil
}

func (f *fakeRunner) Output(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, fakeCall{name: name, args: append([]string(nil), args...)})
	return "", nil
}

func (f *fakeRunner) IsDryRun() bool { return f.dryRun }

var _ run.CommandRunner = (*fakeRunner)(nil)

func validState() config.State {
	state := config.Default()
	state.Host.Hostname = "TestHost"
	state.User.Username = "tester"
	state.User.FullName = "tester"
	state.User.HomeDirectory = "/home/tester"
	state.Git.Username = "tester"
	state.Git.Email = "tester@example.com"
	return state
}

func TestCreateStagingDirUsesUserCacheOutsideTmp(t *testing.T) {
	tmp := t.TempDir()
	cache := filepath.Join(t.TempDir(), "cache")
	t.Setenv("TMPDIR", tmp)
	t.Setenv("XDG_CACHE_HOME", cache)

	staging, err := createStagingDir()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(staging)
	})

	wantPrefix := filepath.Join(cache, "mysetup", "staging") + string(filepath.Separator)
	if !strings.HasPrefix(staging, wantPrefix) {
		t.Fatalf("staging dir must use user cache, got %q want prefix %q", staging, wantPrefix)
	}
}

func TestStagingBaseDirAvoidsTempBackedCacheAndHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmp, "cache"))
	t.Setenv("HOME", filepath.Join(tmp, "home"))

	got := stagingBaseDir()
	wantPrefix := filepath.Join("/var/tmp", "mysetup-")
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("temp-backed cache/home must fall back to /var/tmp, got %q", got)
	}
}

func fakeRepo(t *testing.T) (repo, dest string) {
	t.Helper()
	repo = t.TempDir()
	for _, sub := range []string{"Linux/NixOS", "Linux/dots", "Linux/installer"} {
		if err := os.MkdirAll(filepath.Join(repo, sub), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}
	flake := filepath.Join(repo, "Linux", "NixOS", "flake.nix")
	if err := os.WriteFile(flake, []byte("# minimal\n"), 0o644); err != nil {
		t.Fatalf("write flake.nix: %v", err)
	}
	dest = t.TempDir()
	hw := filepath.Join(dest, "hardware-configuration.nix")
	if err := os.WriteFile(hw, []byte("{ }\n"), 0o644); err != nil {
		t.Fatalf("write hardware-configuration.nix: %v", err)
	}
	return repo, dest
}

func commandSummary(calls []fakeCall) string {
	lines := make([]string, 0, len(calls))
	for _, c := range calls {
		if len(c.args) == 0 {
			lines = append(lines, c.name)
			continue
		}
		lines = append(lines, c.name+" "+strings.Join(c.args, " "))
	}
	return strings.Join(lines, "\n")
}

func TestRunDryRunSkipSwitchHonoursInjectedRunner(t *testing.T) {
	repo, dest := fakeRepo(t)
	fake := &fakeRunner{dryRun: true}

	opts := Options{
		Paths: paths.Options{
			RepoRoot:  repo,
			NixOSDest: dest,
			StatePath: filepath.Join(t.TempDir(), "state.json"),
		},
		State:      validState(),
		DryRun:     true,
		SkipSwitch: true,
		Runner:     fake,
	}

	if err := Run(context.Background(), opts); err != nil {
		t.Fatalf("Run() returned %v", err)
	}

	commands := commandSummary(fake.calls)
	if strings.Contains(commands, "rsync") {
		t.Errorf("thin --no-switch should not mirror sources before dry-build; got:\n%s", commands)
	}
	if !strings.Contains(commands, "nix --extra-experimental-features nix-command flakes flake update --flake") {
		t.Errorf("expected independent thin layout to update the full local lock before dry-build; got:\n%s", commands)
	}
	if strings.Contains(commands, "flake update mysetup --flake") {
		t.Errorf("independent lock mode must not update only mysetup; got:\n%s", commands)
	}
	if !strings.Contains(commands, "nixos-rebuild dry-build") {
		t.Errorf("expected nixos-rebuild dry-build call; got:\n%s", commands)
	}
	if strings.Contains(commands, "nixos-rebuild switch") {
		t.Errorf("SkipSwitch must skip activation; got:\n%s", commands)
	}
}

func TestRunManagedThinLockUpdatesOnlyMySetup(t *testing.T) {
	repo, dest := fakeRepo(t)
	fake := &fakeRunner{dryRun: true}

	opts := Options{
		Paths: paths.Options{
			RepoRoot:  repo,
			NixOSDest: dest,
			StatePath: filepath.Join(t.TempDir(), "state.json"),
		},
		State:      validState(),
		DryRun:     true,
		SkipSwitch: true,
		LockMode:   LockModeManaged,
		Runner:     fake,
	}

	if err := Run(context.Background(), opts); err != nil {
		t.Fatalf("Run() returned %v", err)
	}

	commands := commandSummary(fake.calls)
	if !strings.Contains(commands, "nix --extra-experimental-features nix-command flakes flake update mysetup --flake") {
		t.Errorf("expected managed thin layout to update only mysetup before dry-build; got:\n%s", commands)
	}
}

func TestSwitchSystemSkipsWhenSkipSwitchSet(t *testing.T) {
	t.Parallel()
	fake := &fakeRunner{}
	opts := Options{
		Paths:      paths.Options{NixOSDest: "/etc/nixos"},
		State:      validState(),
		SkipSwitch: true,
	}

	switched, err := switchSystem(context.Background(), fake, opts)
	if err != nil {
		t.Fatalf("switchSystem() returned %v", err)
	}
	if switched {
		t.Errorf("switched = true; want false")
	}
	if len(fake.calls) != 0 {
		t.Errorf("fake recorded %d calls; want 0: %v", len(fake.calls), fake.calls)
	}
}

func TestSwitchSystemAssumeYesIssuesSwitch(t *testing.T) {
	t.Parallel()
	fake := &fakeRunner{}
	opts := Options{
		Paths:     paths.Options{NixOSDest: "/etc/nixos"},
		State:     validState(),
		AssumeYes: true,
	}

	switched, err := switchSystem(context.Background(), fake, opts)
	if err != nil {
		t.Fatalf("switchSystem() returned %v", err)
	}
	if !switched {
		t.Errorf("switched = false; want true")
	}
	commands := commandSummary(fake.calls)
	if !strings.Contains(commands, "nixos-rebuild switch") {
		t.Errorf("expected nixos-rebuild switch; got:\n%s", commands)
	}
}

func TestSwitchSystemAssumeYesPropagatesError(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("rebuild failed")
	fake := &fakeRunner{
		failOn: func(_ string, _ []string) error { return wantErr },
	}
	opts := Options{
		Paths:     paths.Options{NixOSDest: "/etc/nixos"},
		State:     validState(),
		AssumeYes: true,
	}

	switched, err := switchSystem(context.Background(), fake, opts)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v; want wraps %v", err, wantErr)
	}
	if switched {
		t.Errorf("switched = true on failure; want false")
	}
}

func TestDryBuildSystemInvokesNixosRebuild(t *testing.T) {
	t.Parallel()
	fake := &fakeRunner{}
	if err := dryBuildSystem(context.Background(), fake, "/tmp/staging", "TestHost"); err != nil {
		t.Fatalf("dryBuildSystem() returned %v", err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("calls = %d; want 1", len(fake.calls))
	}
	got := fake.calls[0]
	if got.name != "sudo" {
		t.Errorf("name = %q; want %q", got.name, "sudo")
	}
	wantPrefix := "nixos-rebuild dry-build --impure --flake path:/tmp/staging#TestHost"
	if !strings.HasPrefix(strings.Join(got.args, " "), wantPrefix) {
		t.Errorf("args = %q; want prefix %q", strings.Join(got.args, " "), wantPrefix)
	}
	for _, want := range []string{
		"--option max-jobs 1",
		"--option cores 2",
	} {
		if !strings.Contains(strings.Join(got.args, " "), want) {
			t.Errorf("args = %q; missing %q", strings.Join(got.args, " "), want)
		}
	}
}

func TestHandlePreSwitchErrorWithoutBackupReturnsCause(t *testing.T) {
	t.Parallel()
	cause := errors.New("write failed")
	fake := &fakeRunner{}

	err := handlePreSwitchError(context.Background(), fake, "/etc/nixos", "", cause)
	if !errors.Is(err, cause) {
		t.Errorf("err = %v; want %v", err, cause)
	}
	if len(fake.calls) != 0 {
		t.Errorf("fake recorded %d calls; want 0", len(fake.calls))
	}
}

func TestHandlePreSwitchErrorRestoresFromBackup(t *testing.T) {
	t.Parallel()
	cause := errors.New("write failed")
	fake := &fakeRunner{}

	err := handlePreSwitchError(context.Background(), fake, "/etc/nixos", "/etc/nixos.bak", cause)
	if !errors.Is(err, cause) {
		t.Errorf("err = %v; want wraps %v", err, cause)
	}
	if !strings.Contains(err.Error(), "/etc/nixos.bak") {
		t.Errorf("err message missing backup path: %v", err)
	}
	commands := commandSummary(fake.calls)
	if !strings.Contains(commands, "sudo mkdir") {
		t.Errorf("expected sudo mkdir; got:\n%s", commands)
	}
	if !strings.Contains(commands, "sudo rsync") {
		t.Errorf("expected sudo rsync; got:\n%s", commands)
	}
}
