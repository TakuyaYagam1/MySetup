package apply

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
)

func TestCanonicalThinTemplatesImportUserDirectory(t *testing.T) {
	template := ConfigurationNix()
	if !strings.Contains(template, "./user") {
		t.Fatalf("configuration.nix must import ./user\n%s", template)
	}
	if strings.Contains(template, "./private") {
		t.Fatalf("configuration.nix must not import the legacy directory\n%s", template)
	}
}

func TestThinSyncPreservesCanonicalStateAndUserDirectory(t *testing.T) {
	args := strings.Join(syncToEtcArgs("/tmp/staging", "/etc/nixos", LayoutThin), " ")
	for _, want := range []string{
		"--exclude=/installer-state.json",
		"--exclude=/user/",
		"--exclude=/wahrwelt/",
		"--exclude=/mysetup/",
		"--exclude=/secrets/",
		"--exclude=/hashed-password.nix",
	} {
		if !strings.Contains(args, want) {
			t.Fatalf("thin sync must preserve %q, got %s", want, args)
		}
	}
}

func TestSyncPreservesManagedStateRecoveryQuarantine(t *testing.T) {
	for _, layout := range []Layout{LayoutThin, LayoutFull} {
		args := strings.Join(syncToEtcArgs("/tmp/staging", "/etc/nixos", layout), " ")
		if !strings.Contains(args, "--exclude=.wahrwelt-installer-recovery-*/") {
			t.Fatalf("%s sync must preserve the managed state recovery quarantine, got %s", layout, args)
		}
	}
}

func TestSyncPreservesNestedManagedStateRecoveryQuarantine(t *testing.T) {
	for _, layout := range []Layout{LayoutThin, LayoutFull} {
		t.Run(string(layout), func(t *testing.T) {
			staging := t.TempDir()
			dest := t.TempDir()
			payload := filepath.Join(dest, "nested", ".wahrwelt-installer-recovery-probe", "payload")
			if err := os.MkdirAll(filepath.Dir(payload), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(payload, []byte("recover me\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(staging, "managed.nix"), []byte("managed\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			bin := t.TempDir()
			writeExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nexec \"$@\"\n")
			t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
			if err := syncToEtc(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, staging, dest, layout); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(payload)
			if err != nil {
				t.Fatalf("recovery payload was removed by %s sync: %v", layout, err)
			}
			if got, want := string(data), "recover me\n"; got != want {
				t.Fatalf("recovery payload = %q, want %q", got, want)
			}
		})
	}
}

func TestStageThinConfigurationSeedsUserDefault(t *testing.T) {
	staging := t.TempDir()
	if err := stageThinConfiguration(paths.Sources{}, staging, config.Default(), LockModeIndependent); err != nil {
		t.Fatal(err)
	}
	defaultPath := filepath.Join(staging, "user", "default.nix")
	info, err := os.Stat(defaultPath)
	if err != nil {
		t.Fatalf("fresh thin staging must seed user/default.nix: %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o644); got != want {
		t.Fatalf("user/default.nix mode = %o, want %o", got, want)
	}
}

func TestWriteUserDefaultTemplatePreservesExistingBrokenSymlink(t *testing.T) {
	staging := t.TempDir()
	userDir := filepath.Join(staging, "user")
	if err := os.Mkdir(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	defaultPath := filepath.Join(userDir, "default.nix")
	if err := os.Symlink(filepath.Join(staging, "missing.nix"), defaultPath); err != nil {
		t.Fatal(err)
	}

	if err := writeUserDefaultTemplate(staging); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(defaultPath)
	if err != nil {
		t.Fatalf("existing user default must remain in place: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("existing broken symlink was replaced with mode %v", info.Mode())
	}
}

func TestWriteStagedUserDefaultSeedsFreshDestination(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	source := filepath.Join(staging, "user", "default.nix")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("{ ... }: { }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nexec \"$@\"\n")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	if err := writeStagedUserDefault(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, staging, dest, LayoutThin); err != nil {
		t.Fatal(err)
	}
	defaultPath := filepath.Join(dest, "user", "default.nix")
	data, err := os.ReadFile(defaultPath)
	if err != nil {
		t.Fatalf("fresh destination must receive user/default.nix: %v", err)
	}
	if got, want := string(data), "{ ... }: { }\n"; got != want {
		t.Fatalf("fresh user/default.nix = %q, want %q", got, want)
	}
}

func TestWriteStagedUserDefaultPreservesExistingRegularAndSymlinkNodes(t *testing.T) {
	for _, kind := range []string{"regular", "symlink", "broken-symlink"} {
		t.Run(kind, func(t *testing.T) {
			staging := t.TempDir()
			dest := t.TempDir()
			source := filepath.Join(staging, "user", "default.nix")
			target := filepath.Join(dest, "user", "default.nix")
			if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(source, []byte("generated\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "regular":
				if err := os.WriteFile(target, []byte("user owned\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				userFile := filepath.Join(dest, "custom.nix")
				if err := os.WriteFile(userFile, []byte("user owned\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(userFile, target); err != nil {
					t.Fatal(err)
				}
			case "broken-symlink":
				if err := os.Symlink(filepath.Join(dest, "missing.nix"), target); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.Lstat(target)
			if err != nil {
				t.Fatal(err)
			}
			fake := &fakeRunner{}
			if err := writeStagedUserDefault(context.Background(), fake, staging, dest, LayoutThin); err != nil {
				t.Fatal(err)
			}
			after, err := os.Lstat(target)
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(before, after) {
				t.Fatal("existing user/default.nix inode was replaced")
			}
			if len(fake.calls) != 0 {
				t.Fatalf("existing user/default.nix triggered privileged writes: %#v", fake.calls)
			}
		})
	}
}

func TestWriteStagedUserDefaultRejectsNonRegularCollision(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	source := filepath.Join(staging, "user", "default.nix")
	target := filepath.Join(dest, "user", "default.nix")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := &fakeRunner{}
	err := writeStagedUserDefault(context.Background(), fake, staging, dest, LayoutThin)
	if err == nil || !strings.Contains(err.Error(), "unsupported user/default.nix") {
		t.Fatalf("non-regular user/default.nix collision error = %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("non-regular collision triggered privileged writes: %#v", fake.calls)
	}
}

type userDefaultTempCleanupReplacementRaceRunner struct {
	dir               string
	publicationFailed bool
}

func (r *userDefaultTempCleanupReplacementRaceRunner) Command(ctx context.Context, name string, args ...string) error {
	if name == "sudo" && len(args) > 0 && args[0] == "sh" && !r.publicationFailed {
		r.publicationFailed = true
		replacement := filepath.Join(r.dir, "user", ".default.nix.tmp.raced")
		if err := os.WriteFile(replacement, []byte("raced user default replacement\n"), 0o644); err != nil {
			return err
		}
		return errors.New("injected user default publication failure")
	}
	return exec.CommandContext(ctx, args[0], args[1:]...).Run()
}

func (*userDefaultTempCleanupReplacementRaceRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (*userDefaultTempCleanupReplacementRaceRunner) IsDryRun() bool { return false }

func TestWriteStagedUserDefaultPreservesReplacementCreatedBeforePrivilegedTempCleanup(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	source := filepath.Join(staging, "user", "default.nix")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("{ ... }: { }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dest, "user"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := writeStagedUserDefault(context.Background(), &userDefaultTempCleanupReplacementRaceRunner{dir: dest}, staging, dest, LayoutThin)
	if err == nil || !strings.Contains(err.Error(), "injected user default publication failure") {
		t.Fatalf("writeStagedUserDefault() error = %v, want injected publication failure", err)
	}
	replacement := filepath.Join(dest, "user", ".default.nix.tmp.raced")
	data, err := os.ReadFile(replacement)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "raced user default replacement\n"; got != want {
		t.Fatalf("raced user default replacement = %q, want %q", got, want)
	}
}

type userDefaultPublishSourceRaceRunner struct {
	source string
	raced  bool
}

func (r *userDefaultPublishSourceRaceRunner) Command(ctx context.Context, name string, args ...string) error {
	if name == "sudo" && len(args) > 0 && args[0] == "sh" && !r.raced {
		r.raced = true
		if err := os.Remove(r.source); err != nil {
			return err
		}
		if err := os.WriteFile(r.source, []byte("raced link source\n"), 0o644); err != nil {
			return err
		}
	}
	return exec.CommandContext(ctx, args[0], args[1:]...).Run()
}

func (r *userDefaultPublishSourceRaceRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	if name == "sudo" && len(args) > 0 && args[0] == "sh" && !r.raced {
		r.raced = true
		if err := os.Remove(r.source); err != nil {
			return "", err
		}
		if err := os.WriteFile(r.source, []byte("raced link source\n"), 0o644); err != nil {
			return "", err
		}
	}
	out, err := exec.CommandContext(ctx, args[0], args[1:]...).Output()
	return strings.TrimSpace(string(out)), err
}

func (*userDefaultPublishSourceRaceRunner) IsDryRun() bool { return false }

func TestWriteStagedUserDefaultLinksPinnedSourceAcrossSourceRace(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	source := filepath.Join(staging, "user", "default.nix")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("original template\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := writeStagedUserDefault(context.Background(), &userDefaultPublishSourceRaceRunner{source: source}, staging, dest, LayoutThin)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "user", "default.nix"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "original template\n"; got != want {
		t.Fatalf("published user/default.nix = %q, want pinned source %q", got, want)
	}
	raced, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(raced), "raced link source\n"; got != want {
		t.Fatalf("preserved raced link source = %q, want %q", got, want)
	}
}

func TestWriteStagedUserDefaultPreservesConcurrentRegularWinner(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	source := filepath.Join(staging, "user", "default.nix")
	target := filepath.Join(dest, "user", "default.nix")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	realLN, err := exec.LookPath("ln")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nexec \"$@\"\n")
	writeExecutable(t, filepath.Join(bin, "ln"), `#!/bin/sh
set -eu
printf '%s\n' 'concurrent user data' > "$WAHRWELT_USER_DEFAULT_RACE_TARGET"
exec "$WAHRWELT_REAL_LN" "$@"
`)
	t.Setenv("WAHRWELT_REAL_LN", realLN)
	t.Setenv("WAHRWELT_USER_DEFAULT_RACE_TARGET", target)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	if err := writeStagedUserDefault(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, staging, dest, LayoutThin); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "concurrent user data\n"; got != want {
		t.Fatalf("concurrent user default = %q, want %q", got, want)
	}
}

type userDefaultRootSwapBeforeChildOpenRunner struct {
	dest     string
	retained string
	winner   os.FileInfo
	raced    bool
}

func (r *userDefaultRootSwapBeforeChildOpenRunner) Command(ctx context.Context, name string, args ...string) error {
	if name == "sudo" && len(args) > 0 && args[0] == "mkdir" && !r.raced {
		r.raced = true
		if err := os.Rename(r.dest, r.retained); err != nil {
			return err
		}
		winner := filepath.Join(r.dest, "user", "default.nix")
		if err := os.MkdirAll(filepath.Dir(winner), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(winner, []byte("replacement winner\n"), 0o644); err != nil {
			return err
		}
		info, err := os.Stat(winner)
		if err != nil {
			return err
		}
		r.winner = info
	}
	return exec.CommandContext(ctx, args[0], args[1:]...).Run()
}

func (r *userDefaultRootSwapBeforeChildOpenRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	if name == "sudo" && len(args) > 0 && args[0] == "sh" && !r.raced {
		r.raced = true
		if err := os.Rename(r.dest, r.retained); err != nil {
			return "", err
		}
		winner := filepath.Join(r.dest, "user", "default.nix")
		if err := os.MkdirAll(filepath.Dir(winner), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(winner, []byte("replacement winner\n"), 0o644); err != nil {
			return "", err
		}
		info, err := os.Stat(winner)
		if err != nil {
			return "", err
		}
		r.winner = info
	}
	out, err := exec.CommandContext(ctx, args[0], args[1:]...).Output()
	return strings.TrimSpace(string(out)), err
}

func (*userDefaultRootSwapBeforeChildOpenRunner) IsDryRun() bool { return false }

func TestWriteStagedUserDefaultRejectsRootSwapBeforeUserChildOpen(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, "staging")
	dest := filepath.Join(root, "nixos")
	retained := filepath.Join(root, "nixos-before-race")
	source := filepath.Join(staging, "user", "default.nix")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := &userDefaultRootSwapBeforeChildOpenRunner{dest: dest, retained: retained}
	err := writeStagedUserDefault(context.Background(), runner, staging, dest, LayoutThin)
	if err == nil {
		t.Fatal("destination root replacement was accepted")
	}
	winner := filepath.Join(dest, "user", "default.nix")
	info, statErr := os.Stat(winner)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if r := runner.winner; r == nil || !os.SameFile(r, info) {
		t.Fatal("replacement winner was replaced during user/default.nix handoff")
	}
	data, readErr := os.ReadFile(winner)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got, want := string(data), "replacement winner\n"; got != want {
		t.Fatalf("replacement winner = %q, want %q", got, want)
	}
}

type userDefaultCreatedChildSwapRunner struct {
	userDir     string
	retained    string
	replacement os.FileInfo
}

func (r *userDefaultCreatedChildSwapRunner) Command(ctx context.Context, _ string, args ...string) error {
	return exec.CommandContext(ctx, args[0], args[1:]...).Run()
}

func (r *userDefaultCreatedChildSwapRunner) Output(ctx context.Context, _ string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, args[0], args[1:]...).Output()
	if err != nil {
		return "", err
	}
	if err := os.Rename(r.userDir, r.retained); err != nil {
		return "", err
	}
	if err := os.Mkdir(r.userDir, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(r.userDir, "default.nix")
	if err := os.WriteFile(target, []byte("replacement winner\n"), 0o600); err != nil {
		return "", err
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", err
	}
	r.replacement = info
	return strings.TrimSpace(string(out)), nil
}

func (*userDefaultCreatedChildSwapRunner) IsDryRun() bool { return false }

func TestWriteStagedUserDefaultRejectsCreatedChildSwapBeforePin(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, "staging")
	dest := filepath.Join(root, "nixos")
	userDir := filepath.Join(dest, "user")
	retained := filepath.Join(dest, "user-created-by-helper")
	source := filepath.Join(staging, "user", "default.nix")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &userDefaultCreatedChildSwapRunner{userDir: userDir, retained: retained}

	err := writeStagedUserDefault(context.Background(), runner, staging, dest, LayoutThin)
	if err == nil || !strings.Contains(err.Error(), "changed before pin") {
		t.Fatalf("created child replacement error = %v, want identity rejection", err)
	}
	visible, err := os.Stat(filepath.Join(userDir, "default.nix"))
	if err != nil {
		t.Fatal(err)
	}
	if runner.replacement == nil || !os.SameFile(runner.replacement, visible) {
		t.Fatal("replacement user/default.nix was modified")
	}
	data, err := os.ReadFile(filepath.Join(retained, "default.nix"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "generated\n"; got != want {
		t.Fatalf("retained helper publication = %q, want %q", got, want)
	}
}

type userDefaultForeignCollisionRunner struct {
	dest     string
	retained string
	winner   os.FileInfo
}

func (r *userDefaultForeignCollisionRunner) Command(ctx context.Context, name string, args ...string) error {
	if name != "sudo" || len(args) == 0 || args[0] != "sh" {
		return exec.CommandContext(ctx, args[0], args[1:]...).Run()
	}
	if err := os.Rename(r.dest, r.retained); err != nil {
		return err
	}
	winner := filepath.Join(r.dest, "user", "default.nix")
	if err := os.MkdirAll(filepath.Dir(winner), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(winner, []byte("foreign collision\n"), 0o644); err != nil {
		return err
	}
	info, err := os.Stat(winner)
	if err != nil {
		return err
	}
	r.winner = info
	return exec.CommandContext(ctx, "sh", "-c", "exit 17").Run()
}

func (*userDefaultForeignCollisionRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (*userDefaultForeignCollisionRunner) IsDryRun() bool { return false }

func TestWriteStagedUserDefaultRejectsCollisionFromReplacementRoot(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, "staging")
	dest := filepath.Join(root, "nixos")
	retained := filepath.Join(root, "nixos-before-race")
	source := filepath.Join(staging, "user", "default.nix")
	targetDir := filepath.Join(dest, "user")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := &userDefaultForeignCollisionRunner{dest: dest, retained: retained}
	err := writeStagedUserDefault(context.Background(), runner, staging, dest, LayoutThin)
	if err == nil {
		t.Fatal("collision in a replacement destination root was accepted")
	}
	winner := filepath.Join(dest, "user", "default.nix")
	info, statErr := os.Stat(winner)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if r := runner.winner; r == nil || !os.SameFile(r, info) {
		t.Fatal("foreign collision winner was replaced")
	}
	if _, statErr := os.Lstat(filepath.Join(retained, "user", "default.nix")); !os.IsNotExist(statErr) {
		t.Fatalf("failed foreign collision unexpectedly published in retained root: %v", statErr)
	}
}

type userDefaultChildSwapAfterPublishRunner struct {
	userDir     string
	retained    string
	replacement os.FileInfo
	raced       bool
}

func (r *userDefaultChildSwapAfterPublishRunner) Command(ctx context.Context, name string, args ...string) error {
	err := exec.CommandContext(ctx, args[0], args[1:]...).Run()
	if err != nil || name != "sudo" || len(args) == 0 || args[0] != "sh" || r.raced {
		return err
	}
	r.raced = true
	if renameErr := os.Rename(r.userDir, r.retained); renameErr != nil {
		return renameErr
	}
	if mkdirErr := os.Mkdir(r.userDir, 0o755); mkdirErr != nil {
		return mkdirErr
	}
	target := filepath.Join(r.userDir, "default.nix")
	if writeErr := os.WriteFile(target, []byte("generated\n"), 0o644); writeErr != nil {
		return writeErr
	}
	info, statErr := os.Stat(target)
	if statErr != nil {
		return statErr
	}
	r.replacement = info
	return nil
}

func (*userDefaultChildSwapAfterPublishRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (*userDefaultChildSwapAfterPublishRunner) IsDryRun() bool { return false }

func TestWriteStagedUserDefaultRejectsUserChildSwapAfterPublication(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, "staging")
	dest := filepath.Join(root, "nixos")
	userDir := filepath.Join(dest, "user")
	retained := filepath.Join(dest, "user-before-race")
	source := filepath.Join(staging, "user", "default.nix")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("generated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &userDefaultChildSwapAfterPublishRunner{userDir: userDir, retained: retained}

	err := writeStagedUserDefault(context.Background(), runner, staging, dest, LayoutThin)
	if err == nil {
		t.Fatal("post-publication user directory replacement was accepted")
	}
	visibleInfo, statErr := os.Stat(filepath.Join(userDir, "default.nix"))
	if statErr != nil {
		t.Fatal(statErr)
	}
	if runner.replacement == nil || !os.SameFile(runner.replacement, visibleInfo) {
		t.Fatal("replacement user/default.nix was modified")
	}
	retainedData, readErr := os.ReadFile(filepath.Join(retained, "default.nix"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got, want := string(retainedData), "generated\n"; got != want {
		t.Fatalf("retained pinned user/default.nix = %q, want %q", got, want)
	}
}

func TestWriteStagedUserDefaultRejectsCommandOnlyNonDryRunner(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	source := filepath.Join(staging, "user", "default.nix")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("{ ... }: { }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeStagedUserDefault(context.Background(), &fakeRunner{}, staging, dest, LayoutThin); err == nil {
		t.Fatal("command-only non-dry runner was accepted for user/default.nix publication")
	}
}

func privilegedInstallProbeToolchain(t *testing.T, publicProbe string) {
	t.Helper()
	realInstall, err := exec.LookPath("install")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nexec \"$@\"\n")
	writeExecutable(t, filepath.Join(bin, "install"), `#!/bin/sh
set -eu
"$WAHRWELT_REAL_INSTALL" "$@"
printf '%s\n' 'unknown public replacement' > "$WAHRWELT_PUBLIC_TEMP_PROBE"
`)
	t.Setenv("WAHRWELT_REAL_INSTALL", realInstall)
	t.Setenv("WAHRWELT_PUBLIC_TEMP_PROBE", publicProbe)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
}

func TestWriteStagedUserDefaultDoesNotExposePublicTempDuringPrivilegedInstall(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	source := filepath.Join(staging, "user", "default.nix")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("original template\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dest, "user"), 0o755); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(dest, "user", ".default.nix.tmp.raced")
	privilegedInstallProbeToolchain(t, probe)
	if err := writeStagedUserDefault(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, staging, dest, LayoutThin); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "user", "default.nix"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "original template\n"; got != want {
		t.Fatalf("published user/default.nix = %q, want %q", got, want)
	}
	probeData, err := os.ReadFile(probe)
	if err != nil {
		t.Fatalf("public replacement written after helper install was removed: %v", err)
	}
	if got, want := string(probeData), "unknown public replacement\n"; got != want {
		t.Fatalf("public replacement = %q, want %q", got, want)
	}
}

func TestPrepareThinHostLocalPreservesExistingUserDirectory(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	for path, content := range map[string]string{
		filepath.Join(dest, "hardware-configuration.nix"): "hardware\n",
		filepath.Join(dest, "user", "default.nix"):        "{ ... }: { imports = [ ./custom.nix ]; }\n",
		filepath.Join(dest, "user", "custom.nix"):         "{ ... }: { }\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := prepareStagingHostLocal(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, staging, dest, config.Default(), config.Secrets{}, LayoutThin); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(staging, "user", "custom.nix"))
	if err != nil {
		t.Fatalf("staging must retain existing user content: %v", err)
	}
	if got, want := string(data), "{ ... }: { }\n"; got != want {
		t.Fatalf("staged user content = %q, want %q", got, want)
	}
}

func TestPrepareThinHostLocalStagesLegacyDirectoryAsUser(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	for path, content := range map[string]string{
		filepath.Join(dest, "hardware-configuration.nix"): "hardware\n",
		filepath.Join(dest, "private", "custom.nix"):      "{ ... }: { }\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := prepareStagingHostLocal(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, staging, dest, config.Default(), config.Secrets{}, LayoutThin); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(staging, "user", "custom.nix")); err != nil {
		t.Fatalf("legacy content must be staged as user content: %v", err)
	}
}

func TestPrepareThinHostLocalRejectsUserAndLegacyDirectoryCollision(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	for _, name := range []string{"user", "private"} {
		if err := os.MkdirAll(filepath.Join(dest, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dest, "hardware-configuration.nix"), []byte("hardware\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	flake, err := FlakeNix(config.Default(), LockModeIndependent)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "flake.nix"), []byte(flake), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "configuration.nix"), []byte("destination override\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "configuration.nix"), []byte("staging seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err = prepareStagingHostLocal(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, staging, dest, config.Default(), config.Secrets{}, LayoutThin)
	if err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("expected user and legacy directory collision error, got %v", err)
	}
	data, readErr := os.ReadFile(filepath.Join(staging, "configuration.nix"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got, want := string(data), "staging seed\n"; got != want {
		t.Fatalf("collision must stop before staging copies, got %q want %q", got, want)
	}
}

func TestPrepareThinHostLocalRejectsLegacySymlink(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "hardware-configuration.nix"), []byte("hardware\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dest, "missing"), filepath.Join(dest, "private")); err != nil {
		t.Fatal(err)
	}

	err := prepareStagingHostLocal(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, staging, dest, config.Default(), config.Secrets{}, LayoutThin)
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported legacy path error, got %v", err)
	}
}

func TestRewriteLegacyUserImportChangesOnlyNixPathTokens(t *testing.T) {
	input := "imports = [ ./private ./private-network.nix ];\n" +
		"message = \"./private\";\n" +
		"# ./private\n" +
		"payload = '' ./private '';\n"
	want := "imports = [ ./user ./private-network.nix ];\n" +
		"message = \"./private\";\n" +
		"# ./private\n" +
		"payload = '' ./private '';\n"
	if got := rewriteLegacyUserImport(input); got != want {
		t.Fatalf("rewritten configuration:\n%s\nwant:\n%s", got, want)
	}
}

func TestRewriteLegacyUserImportMigratesLegacyUserPathPrefix(t *testing.T) {
	input := "imports = [ ./private/custom.nix ./private ./private-network.nix ];\n" +
		"message = \"./private/custom.nix\";\n" +
		"# ./private/custom.nix\n" +
		"payload = '' ./private/custom.nix '';\n"
	want := "imports = [ ./user/custom.nix ./user ./private-network.nix ];\n" +
		"message = \"./private/custom.nix\";\n" +
		"# ./private/custom.nix\n" +
		"payload = '' ./private/custom.nix '';\n"
	if got := rewriteLegacyUserImport(input); got != want {
		t.Fatalf("rewritten configuration:\n%s\nwant:\n%s", got, want)
	}
}

func TestRewriteLegacyUserImportMigratesPathsInsideNixInterpolations(t *testing.T) {
	input := `quoted = "${./private}";
nested = "${if true then { path = ./private/custom.nix; } else { path = ./private-network.nix; }}";
indented = '' ${./private/custom.nix} '';
escaped_quoted = "\${./private}";
escaped_indented = '' keep ''${./private} '';
`
	want := `quoted = "${./user}";
nested = "${if true then { path = ./user/custom.nix; } else { path = ./private-network.nix; }}";
indented = '' ${./user/custom.nix} '';
escaped_quoted = "\${./private}";
escaped_indented = '' keep ''${./private} '';
`
	if got := rewriteLegacyUserImport(input); got != want {
		t.Fatalf("rewritten interpolations:\n%s\nwant:\n%s", got, want)
	}
}

func TestRewriteLegacyUserImportUsesNixOperatorBoundaries(t *testing.T) {
	input := `operators = [ {}//./private 2*./private 2<./private true&&./private false||./private 2==./private 2!=./private ];
lambdas = [ ({}:./private) ({ x }:./private) (foo_https:./private) (foo'bar:./private) ];
trailing_operators = [ (./private?x) (./private*2) (./private<2) (./private>2) (./private==./other) (./private!=./other) (./private&&true) (./private||false) ];
uri = value:./private;
scheme_uris = [ https://./private file://./private ];
colon_uris = [ scheme:a:./private http://host:8080/./private ];
nested_uri = ssh://host//./private;
query_uri = https://host/path?x=./private;
triple_slash = defaults///./private;
path_suffixes = [ ./private+foo ./private.foo ];
search_paths = [ <./private> <foo/./private> ];
path_tokens = [ x+./private x-./private x/./private ];
absolute = /tmp/./private;
interpolated_path = ./base/${foo}./private;
interpolated_home_path = ~/${foo}./private;
interpolated_path_adjacent = ./base${foo}./private;
interpolated_home_path_adjacent = ~/base${foo}./private;
interpolated_path_pre_separators = ./base//${foo}////./private;
interpolated_home_path_pre_separators = ~/base//${foo}////./private;
interpolated_path_triple = ./base/${foo}///./private;
interpolated_home_path_triple = ~/${foo}///./private;
interpolated_path_expression = ./base/${./private}/asset;
`
	want := `operators = [ {}//./user 2*./user 2<./user true&&./user false||./user 2==./user 2!=./user ];
lambdas = [ ({}:./user) ({ x }:./user) (foo_https:./user) (foo'bar:./user) ];
trailing_operators = [ (./user?x) (./user*2) (./user<2) (./user>2) (./user==./other) (./user!=./other) (./user&&true) (./user||false) ];
uri = value:./private;
scheme_uris = [ https://./private file://./private ];
colon_uris = [ scheme:a:./private http://host:8080/./private ];
nested_uri = ssh://host//./private;
query_uri = https://host/path?x=./private;
triple_slash = defaults///./private;
path_suffixes = [ ./private+foo ./private.foo ];
search_paths = [ <./private> <foo/./private> ];
path_tokens = [ x+./private x-./private x/./private ];
absolute = /tmp/./private;
interpolated_path = ./base/${foo}./private;
interpolated_home_path = ~/${foo}./private;
interpolated_path_adjacent = ./base${foo}./private;
interpolated_home_path_adjacent = ~/base${foo}./private;
interpolated_path_pre_separators = ./base//${foo}////./private;
interpolated_home_path_pre_separators = ~/base//${foo}////./private;
interpolated_path_triple = ./base/${foo}///./private;
interpolated_home_path_triple = ~/${foo}///./private;
interpolated_path_expression = ./base/${./user}/asset;
`
	if got := rewriteLegacyUserImport(input); got != want {
		t.Fatalf("rewritten operator boundaries:\n%s\nwant:\n%s", got, want)
	}
}

func TestMigrateLegacyUserDirectoryRenamesOrdinaryDirectory(t *testing.T) {
	dest := t.TempDir()
	if err := os.Mkdir(filepath.Join(dest, "private"), 0o755); err != nil {
		t.Fatal(err)
	}
	fake := &fakeRunner{}
	if err := migrateLegacyUserDirectory(context.Background(), fake, dest); err != nil {
		t.Fatal(err)
	}
	commands := commandSummary(fake.calls)
	for _, want := range []string{
		"sudo sh -c",
		`mv -T --no-copy --update=none-fail -- "$parent_fd/$source_name" "$parent_fd/$target_name"`,
		" private user /proc/",
	} {
		if !strings.Contains(commands, want) {
			t.Fatalf("legacy directory must use the pinned privileged no-replace helper %q, got:\n%s", want, commands)
		}
	}
}

func TestMigrateLegacyUserDirectoryRejectsTargetCollision(t *testing.T) {
	dest := t.TempDir()
	for _, name := range []string{"private", "user"} {
		if err := os.Mkdir(filepath.Join(dest, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := migrateLegacyUserDirectory(context.Background(), &fakeRunner{}, dest); err == nil {
		t.Fatal("expected target collision error")
	}
}

type legacyUserDestinationRaceRunner struct {
	userDir string
	raced   bool
}

func (r *legacyUserDestinationRaceRunner) Command(ctx context.Context, name string, args ...string) error {
	if name == "sudo" && len(args) > 0 && args[0] == "sh" && !r.raced {
		r.raced = true
		if err := os.Mkdir(r.userDir, 0o755); err != nil {
			return err
		}
	}
	return exec.CommandContext(ctx, args[0], args[1:]...).Run()
}

func (*legacyUserDestinationRaceRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (*legacyUserDestinationRaceRunner) IsDryRun() bool { return false }

func TestMigrateLegacyUserDirectoryPreservesDestinationCreatedDuringRename(t *testing.T) {
	dest := t.TempDir()
	legacyDir := filepath.Join(dest, "private")
	userDir := filepath.Join(dest, "user")
	legacyFile := filepath.Join(legacyDir, "custom.nix")
	if err := os.Mkdir(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyFile, []byte("legacy user data\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := migrateLegacyUserDirectory(context.Background(), &legacyUserDestinationRaceRunner{userDir: userDir}, dest)
	if err == nil || !strings.Contains(err.Error(), "target appeared during migration") {
		t.Fatalf("migrateLegacyUserDirectory() error = %v, want raced target collision", err)
	}
	data, err := os.ReadFile(legacyFile)
	if err != nil {
		t.Fatalf("legacy user directory was replaced: %v", err)
	}
	if got, want := string(data), "legacy user data\n"; got != want {
		t.Fatalf("legacy user data = %q, want %q", got, want)
	}
	entries, err := os.ReadDir(userDir)
	if err != nil {
		t.Fatalf("raced user directory missing: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("raced user directory was replaced: %#v", entries)
	}
}

type legacyUserSourceReplacementRunner struct {
	legacy string
	saved  string
	raced  bool
}

func (r *legacyUserSourceReplacementRunner) Command(ctx context.Context, name string, args ...string) error {
	if name == "sudo" && len(args) > 0 && args[0] == "sh" && !r.raced {
		r.raced = true
		if err := os.Rename(r.legacy, r.saved); err != nil {
			return err
		}
		if err := os.Mkdir(r.legacy, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(r.legacy, "winner.nix"), []byte("source winner\n"), 0o644); err != nil {
			return err
		}
	}
	return exec.CommandContext(ctx, args[0], args[1:]...).Run()
}

func (*legacyUserSourceReplacementRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (*legacyUserSourceReplacementRunner) IsDryRun() bool { return false }

func readLegacyUserMigrationFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestMigrateLegacyUserDirectoryPreservesSourceReplacementBeforePrivilegedMove(t *testing.T) {
	dest := t.TempDir()
	legacy := filepath.Join(dest, "private")
	saved := filepath.Join(dest, "private-before-race")
	if err := os.Mkdir(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "custom.nix"), []byte("legacy data\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := migrateLegacyUserDirectory(
		context.Background(),
		&legacyUserSourceReplacementRunner{legacy: legacy, saved: saved},
		dest,
	)
	if err == nil || !strings.Contains(err.Error(), "migrate legacy user directory") {
		t.Fatalf("source replacement was accepted: %v", err)
	}
	if got := readLegacyUserMigrationFile(t, filepath.Join(legacy, "winner.nix")); got != "source winner\n" {
		t.Fatalf("source replacement winner changed: %q", got)
	}
	if got := readLegacyUserMigrationFile(t, filepath.Join(saved, "custom.nix")); got != "legacy data\n" {
		t.Fatalf("expected legacy source changed: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(dest, "user")); !os.IsNotExist(statErr) {
		t.Fatalf("canonical user target appeared after source replacement: %v", statErr)
	}
}

func legacyUnexpectedMoveToolchain(t *testing.T, legacy, saved string) {
	t.Helper()
	realMV, err := exec.LookPath("mv")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nexec \"$@\"\n")
	writeExecutable(t, filepath.Join(bin, "mv"), `#!/bin/sh
set -eu
count_file=${WAHRWELT_LEGACY_MOVE_COUNTER:?}
count=0
if [ -f "$count_file" ]; then
	count=$(cat "$count_file")
fi
count=$((count + 1))
printf '%s\n' "$count" > "$count_file"
if [ "$count" = 1 ]; then
	"$WAHRWELT_REAL_MV" -T -- "$WAHRWELT_LEGACY_SOURCE" "$WAHRWELT_LEGACY_SAVED"
	mkdir -- "$WAHRWELT_LEGACY_SOURCE"
	printf '%s\n' 'unexpected moved source' > "$WAHRWELT_LEGACY_SOURCE/unexpected.nix"
fi
exec "$WAHRWELT_REAL_MV" "$@"
`)
	t.Setenv("WAHRWELT_REAL_MV", realMV)
	t.Setenv("WAHRWELT_LEGACY_MOVE_COUNTER", filepath.Join(t.TempDir(), "mv-count"))
	t.Setenv("WAHRWELT_LEGACY_SOURCE", legacy)
	t.Setenv("WAHRWELT_LEGACY_SAVED", saved)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
}

func TestMigrateLegacyUserDirectoryPreservesUnexpectedMovedSourceAtTarget(t *testing.T) {
	dest := t.TempDir()
	legacy := filepath.Join(dest, "private")
	saved := filepath.Join(dest, "private-before-move-race")
	if err := os.Mkdir(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "custom.nix"), []byte("expected legacy source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	legacyUnexpectedMoveToolchain(t, legacy, saved)
	var stderr strings.Builder

	err := migrateLegacyUserDirectory(context.Background(), run.Runner{Stdout: io.Discard, Stderr: &stderr}, dest)
	if err == nil {
		t.Fatal("unexpected moved source was accepted")
	}
	if _, statErr := os.Lstat(legacy); !os.IsNotExist(statErr) {
		t.Fatalf("legacy source name was unexpectedly recreated: %v", statErr)
	}
	user := filepath.Join(dest, "user")
	if got := readLegacyUserMigrationFile(t, filepath.Join(user, "unexpected.nix")); got != "unexpected moved source\n" {
		t.Fatalf("unexpected source at canonical target = %q", got)
	}
	if got := readLegacyUserMigrationFile(t, filepath.Join(saved, "custom.nix")); got != "expected legacy source\n" {
		t.Fatalf("pinned expected source = %q", got)
	}
	matches, globErr := filepath.Glob(filepath.Join(dest, ".wahrwelt-installer-recovery-user-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 0 {
		t.Fatalf("unexpected source was needlessly displaced into recovery: %v", matches)
	}
	for _, want := range []string{user, saved} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("exact collision locations were not reported, want %q in %s", want, stderr.String())
		}
	}
}

func legacyPostRenameReplacementToolchain(t *testing.T, target, saved string) {
	t.Helper()
	realMV, err := exec.LookPath("mv")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nexec \"$@\"\n")
	writeExecutable(t, filepath.Join(bin, "mv"), `#!/bin/sh
set -eu
count_file=${WAHRWELT_LEGACY_MOVE_COUNTER:?}
count=0
if [ -f "$count_file" ]; then
	count=$(cat "$count_file")
fi
count=$((count + 1))
printf '%s\n' "$count" > "$count_file"
"$WAHRWELT_REAL_MV" "$@"
if [ "$count" = 1 ]; then
	"$WAHRWELT_REAL_MV" -T -- "$WAHRWELT_LEGACY_TARGET" "$WAHRWELT_LEGACY_SAVED"
	mkdir -- "$WAHRWELT_LEGACY_TARGET"
	printf '%s\n' 'post-rename target winner' > "$WAHRWELT_LEGACY_TARGET/winner.nix"
fi
`)
	t.Setenv("WAHRWELT_REAL_MV", realMV)
	t.Setenv("WAHRWELT_LEGACY_MOVE_COUNTER", filepath.Join(t.TempDir(), "mv-count"))
	t.Setenv("WAHRWELT_LEGACY_TARGET", target)
	t.Setenv("WAHRWELT_LEGACY_SAVED", saved)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
}

func TestMigrateLegacyUserDirectoryPreservesPostRenameTargetReplacement(t *testing.T) {
	dest := t.TempDir()
	legacy := filepath.Join(dest, "private")
	user := filepath.Join(dest, "user")
	saved := filepath.Join(dest, "user-before-target-race")
	if err := os.Mkdir(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "custom.nix"), []byte("expected legacy source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	legacyPostRenameReplacementToolchain(t, user, saved)
	var stderr strings.Builder

	err := migrateLegacyUserDirectory(context.Background(), run.Runner{Stdout: io.Discard, Stderr: &stderr}, dest)
	if err == nil {
		t.Fatal("post-rename target replacement was accepted")
	}
	if _, statErr := os.Lstat(legacy); !os.IsNotExist(statErr) {
		t.Fatalf("legacy source name was unexpectedly recreated: %v", statErr)
	}
	if got := readLegacyUserMigrationFile(t, filepath.Join(user, "winner.nix")); got != "post-rename target winner\n" {
		t.Fatalf("target replacement winner = %q", got)
	}
	if got := readLegacyUserMigrationFile(t, filepath.Join(saved, "custom.nix")); got != "expected legacy source\n" {
		t.Fatalf("pinned expected source = %q", got)
	}
	for _, want := range []string{user, saved} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("exact collision locations were not reported, want %q in %s", want, stderr.String())
		}
	}
}

type legacyUserParentReplacementRunner struct {
	dest  string
	saved string
	raced bool
}

func (r *legacyUserParentReplacementRunner) Command(ctx context.Context, name string, args ...string) error {
	if name == "sudo" && len(args) > 0 && args[0] == "sh" && !r.raced {
		r.raced = true
		if err := os.Rename(r.dest, r.saved); err != nil {
			return err
		}
		for path, content := range map[string]string{
			filepath.Join(r.dest, "private", "winner.nix"): "replacement private\n",
			filepath.Join(r.dest, "user", "winner.nix"):    "replacement user\n",
		} {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return err
			}
		}
	}
	return exec.CommandContext(ctx, args[0], args[1:]...).Run()
}

func (*legacyUserParentReplacementRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (*legacyUserParentReplacementRunner) IsDryRun() bool { return false }

func TestMigrateLegacyUserDirectoryPreservesReplacementParent(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "nixos")
	saved := filepath.Join(root, "nixos-before-race")
	if err := os.MkdirAll(filepath.Join(dest, "private"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "private", "custom.nix"), []byte("legacy data\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := migrateLegacyUserDirectory(
		context.Background(),
		&legacyUserParentReplacementRunner{dest: dest, saved: saved},
		dest,
	)
	if err == nil || !strings.Contains(err.Error(), "migrate legacy user directory") {
		t.Fatalf("parent replacement was accepted: %v", err)
	}
	if got := readLegacyUserMigrationFile(t, filepath.Join(saved, "private", "custom.nix")); got != "legacy data\n" {
		t.Fatalf("pinned legacy source changed: %q", got)
	}
	for path, want := range map[string]string{
		filepath.Join(dest, "private", "winner.nix"): "replacement private\n",
		filepath.Join(dest, "user", "winner.nix"):    "replacement user\n",
	} {
		if got := readLegacyUserMigrationFile(t, path); got != want {
			t.Fatalf("replacement parent winner changed at %s: %q", path, got)
		}
	}
}

type backupMigrationCollisionRunner struct {
	userDir string
	calls   []fakeCall
	raced   bool
}

func (r *backupMigrationCollisionRunner) Command(ctx context.Context, name string, args ...string) error {
	r.calls = append(r.calls, fakeCall{name: name, args: append([]string(nil), args...)})
	if handled, err := runValidatedStagingTestCommand(ctx, name, args...); handled {
		return err
	}
	if name != "sudo" || len(args) == 0 {
		return nil
	}
	if args[0] == "sh" && !r.raced {
		r.raced = true
		if err := os.Mkdir(r.userDir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(r.userDir, "winner.nix"), []byte("raced user winner\n"), 0o644); err != nil {
			return err
		}
	}
	switch args[0] {
	case "cp", "mkdir", "mv", "rsync", "sh":
		return exec.CommandContext(ctx, args[0], args[1:]...).Run()
	default:
		return nil
	}
}

func (r *backupMigrationCollisionRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, fakeCall{name: name, args: append([]string(nil), args...)})
	if output, handled, err := runValidatedStagingTestOutput(ctx, name, args...); handled {
		return output, err
	}
	return "", nil
}

func (*backupMigrationCollisionRunner) IsDryRun() bool { return false }

func TestRunPreservesUserWinnerWhenMigrationCollidesAfterBackup(t *testing.T) {
	repo, dest := fakeRepo(t)
	legacyFile := filepath.Join(dest, "private", "custom.nix")
	if err := os.MkdirAll(filepath.Dir(legacyFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyFile, []byte("legacy user data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &backupMigrationCollisionRunner{userDir: filepath.Join(dest, "user")}
	state := validState()
	state.Dots = config.Dots{}
	err := Run(context.Background(), Options{
		Paths: paths.Options{
			RepoRoot:  repo,
			NixOSDest: dest,
			StatePath: filepath.Join(t.TempDir(), "state.json"),
		},
		State:     state,
		AssumeYes: true,
		Runner:    runner,
	})
	if err == nil || !strings.Contains(err.Error(), "skipped automatic") {
		t.Fatalf("Run() error = %v, want preserved post-backup collision", err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "user", "winner.nix"))
	if err != nil {
		t.Fatalf("raced user winner was removed by restore: %v", err)
	}
	if got, want := string(data), "raced user winner\n"; got != want {
		t.Fatalf("raced user winner = %q, want %q", got, want)
	}
	if _, err := os.Stat(legacyFile); err != nil {
		t.Fatalf("legacy source was altered after collision: %v", err)
	}
	if commands := commandSummary(runner.calls); strings.Contains(commands, "sudo rsync -a --delete") {
		t.Fatalf("collision rollback must not run destructive rsync restore, got:\n%s", commands)
	}
}

type backupPreflightCollisionRunner struct {
	userDir string
	calls   []fakeCall
	raced   bool
}

func (r *backupPreflightCollisionRunner) Command(ctx context.Context, name string, args ...string) error {
	r.calls = append(r.calls, fakeCall{name: name, args: append([]string(nil), args...)})
	if handled, err := runValidatedStagingTestCommand(ctx, name, args...); handled {
		return err
	}
	if name != "sudo" || len(args) == 0 {
		return nil
	}
	if args[0] == "cp" {
		if err := exec.CommandContext(ctx, args[0], args[1:]...).Run(); err != nil {
			return err
		}
		if !r.raced {
			r.raced = true
			if err := os.Mkdir(r.userDir, 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(r.userDir, "winner.nix"), []byte("preflight race winner\n"), 0o644)
		}
		return nil
	}
	if args[0] == "rsync" {
		return exec.CommandContext(ctx, args[0], args[1:]...).Run()
	}
	return nil
}

func (r *backupPreflightCollisionRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, fakeCall{name: name, args: append([]string(nil), args...)})
	if output, handled, err := runValidatedStagingTestOutput(ctx, name, args...); handled {
		return output, err
	}
	return "", nil
}

func (*backupPreflightCollisionRunner) IsDryRun() bool { return false }

func TestRunPreservesUserWinnerCreatedAfterBackupBeforeMigrationPreflight(t *testing.T) {
	repo, dest := fakeRepo(t)
	legacyFile := filepath.Join(dest, "private", "custom.nix")
	if err := os.MkdirAll(filepath.Dir(legacyFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyFile, []byte("legacy user data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &backupPreflightCollisionRunner{userDir: filepath.Join(dest, "user")}
	state := validState()
	state.Dots = config.Dots{}
	err := Run(context.Background(), Options{
		Paths: paths.Options{
			RepoRoot:  repo,
			NixOSDest: dest,
			StatePath: filepath.Join(t.TempDir(), "state.json"),
		},
		State:     state,
		AssumeYes: true,
		Runner:    runner,
	})
	if err == nil || !strings.Contains(err.Error(), "skipped automatic") {
		t.Fatalf("Run() error = %v, want preserved post-backup preflight collision", err)
	}
	winnerData, readErr := os.ReadFile(filepath.Join(dest, "user", "winner.nix"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got := string(winnerData); got != "preflight race winner\n" {
		t.Fatalf("preflight race winner = %q", got)
	}
	legacyData, readErr := os.ReadFile(legacyFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got := string(legacyData); got != "legacy user data\n" {
		t.Fatalf("legacy source changed after preflight collision: %q", got)
	}
	if commands := commandSummary(runner.calls); strings.Contains(commands, "sudo rsync -a --delete") {
		t.Fatalf("preflight collision must not run destructive restore, got:\n%s", commands)
	}
}

type backupMigrationFailureRunner struct {
	calls []fakeCall
}

func (r *backupMigrationFailureRunner) Command(ctx context.Context, name string, args ...string) error {
	r.calls = append(r.calls, fakeCall{name: name, args: append([]string(nil), args...)})
	if handled, err := runValidatedStagingTestCommand(ctx, name, args...); handled {
		return err
	}
	if name != "sudo" || len(args) == 0 {
		return nil
	}
	switch args[0] {
	case "cp", "rsync":
		return exec.CommandContext(ctx, args[0], args[1:]...).Run()
	case "sh":
		return errors.New("injected atomic rename failure")
	default:
		return nil
	}
}

func (r *backupMigrationFailureRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, fakeCall{name: name, args: append([]string(nil), args...)})
	if output, handled, err := runValidatedStagingTestOutput(ctx, name, args...); handled {
		return output, err
	}
	return "", nil
}

func (*backupMigrationFailureRunner) IsDryRun() bool { return false }

type postMigrationSyncFailureRunner struct {
	userDir   string
	calls     []fakeCall
	syncCalls int
}

func (r *postMigrationSyncFailureRunner) Command(ctx context.Context, name string, args ...string) error {
	r.calls = append(r.calls, fakeCall{name: name, args: append([]string(nil), args...)})
	if handled, err := runValidatedStagingTestCommand(ctx, name, args...); handled {
		return err
	}
	if name != "sudo" || len(args) == 0 {
		return nil
	}
	switch args[0] {
	case "cp", "mkdir":
		return exec.CommandContext(ctx, args[0], args[1:]...).Run()
	case "sh":
		if err := exec.CommandContext(ctx, args[0], args[1:]...).Run(); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(r.userDir, "concurrent-winner.nix"), []byte("canonical winner\n"), 0o644)
	case "rsync":
		r.syncCalls++
		if r.syncCalls == 1 {
			return errors.New("injected post-migration sync failure")
		}
		return exec.CommandContext(ctx, args[0], args[1:]...).Run()
	default:
		return nil
	}
}

func (r *postMigrationSyncFailureRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, fakeCall{name: name, args: append([]string(nil), args...)})
	if output, handled, err := runValidatedStagingTestOutput(ctx, name, args...); handled {
		return output, err
	}
	return "", nil
}

func (*postMigrationSyncFailureRunner) IsDryRun() bool { return false }

func TestRunPreservesCanonicalUserWinnerAfterCommittedMigrationAndLaterFailure(t *testing.T) {
	repo, dest := fakeRepo(t)
	legacyFile := filepath.Join(dest, "private", "custom.nix")
	if err := os.MkdirAll(filepath.Dir(legacyFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyFile, []byte("legacy user data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &postMigrationSyncFailureRunner{userDir: filepath.Join(dest, "user")}
	state := validState()
	state.Dots = config.Dots{}

	err := Run(context.Background(), Options{
		Paths: paths.Options{
			RepoRoot:  repo,
			NixOSDest: dest,
			StatePath: filepath.Join(t.TempDir(), "state.json"),
		},
		State:     state,
		AssumeYes: true,
		Runner:    runner,
	})
	if err == nil || !strings.Contains(err.Error(), "injected post-migration sync failure") {
		t.Fatalf("Run() error = %v, want injected post-migration failure", err)
	}
	winnerData, readErr := os.ReadFile(filepath.Join(dest, "user", "concurrent-winner.nix"))
	if readErr != nil {
		t.Fatalf("canonical winner was removed after later failure: %v", readErr)
	}
	if got := string(winnerData); got != "canonical winner\n" {
		t.Fatalf("canonical winner changed after later failure: %q", got)
	}
	legacyData, readErr := os.ReadFile(filepath.Join(dest, "user", "custom.nix"))
	if readErr != nil {
		t.Fatalf("committed legacy user data was removed after later failure: %v", readErr)
	}
	if got := string(legacyData); got != "legacy user data\n" {
		t.Fatalf("committed legacy user data changed after later failure: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(dest, "private")); !os.IsNotExist(statErr) {
		t.Fatalf("committed migration was rolled back to private/: %v", statErr)
	}
	if commands := commandSummary(runner.calls); strings.Contains(commands, "sudo rsync -a --delete") {
		t.Fatalf("post-migration failure must retain the backup instead of running broad restore, got:\n%s", commands)
	}
}

type postMigrationDotsFailureRunner struct {
	userDir string
	calls   []fakeCall
}

func (r *postMigrationDotsFailureRunner) Command(ctx context.Context, name string, args ...string) error {
	r.calls = append(r.calls, fakeCall{name: name, args: append([]string(nil), args...)})
	if handled, err := runValidatedStagingTestCommand(ctx, name, args...); handled {
		return err
	}
	if name == "mkdir" && len(args) > 0 && args[len(args)-1] == "/home/tester/Pictures/Wallpapers" {
		return errors.New("injected post-migration dots failure")
	}
	if name != "sudo" || len(args) == 0 {
		return nil
	}
	switch args[0] {
	case "cp", "mkdir", "mv", "rsync", "sh", "install", "chown":
		if err := exec.CommandContext(ctx, args[0], args[1:]...).Run(); err != nil {
			return err
		}
	default:
		return nil
	}
	if args[0] == "sh" {
		return os.WriteFile(filepath.Join(r.userDir, "concurrent-winner.nix"), []byte("canonical dots winner\n"), 0o644)
	}
	return nil
}

func (r *postMigrationDotsFailureRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, fakeCall{name: name, args: append([]string(nil), args...)})
	if output, handled, err := runValidatedStagingTestOutput(ctx, name, args...); handled {
		return output, err
	}
	return "", nil
}

func (*postMigrationDotsFailureRunner) IsDryRun() bool { return false }

func TestRunPreservesCanonicalUserWinnerAfterCommittedMigrationAndDotsFailure(t *testing.T) {
	repo, dest := fakeRepo(t)
	legacyFile := filepath.Join(dest, "private", "custom.nix")
	if err := os.MkdirAll(filepath.Dir(legacyFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyFile, []byte("legacy user data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wallpaper := filepath.Join(repo, "Linux", "NixOS", "Wallpapers", "wallpaper.txt")
	if err := os.MkdirAll(filepath.Dir(wallpaper), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wallpaper, []byte("wallpaper\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &postMigrationDotsFailureRunner{userDir: filepath.Join(dest, "user")}
	state := validState()
	state.Dots = config.Dots{Wallpapers: true}

	err := Run(context.Background(), Options{
		Paths: paths.Options{
			RepoRoot:  repo,
			NixOSDest: dest,
			StatePath: filepath.Join(t.TempDir(), "state.json"),
		},
		State:     state,
		AssumeYes: true,
		Runner:    runner,
	})
	if err == nil || !strings.Contains(err.Error(), "injected post-migration dots failure") {
		t.Fatalf("Run() error = %v, want injected dots failure", err)
	}
	winnerData, readErr := os.ReadFile(filepath.Join(dest, "user", "concurrent-winner.nix"))
	if readErr != nil {
		t.Fatalf("canonical dots winner was removed after later failure: %v", readErr)
	}
	if got := string(winnerData); got != "canonical dots winner\n" {
		t.Fatalf("canonical dots winner changed after later failure: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(dest, "private")); !os.IsNotExist(statErr) {
		t.Fatalf("committed migration was rolled back after dots failure: %v", statErr)
	}
	if commands := commandSummary(runner.calls); strings.Contains(commands, "sudo rsync -a --delete") {
		t.Fatalf("post-migration dots failure must retain the backup instead of running broad restore, got:\n%s", commands)
	}
}

func TestRunSkipsBroadRestoreAfterAtomicMigrationCommandFailure(t *testing.T) {
	repo, dest := fakeRepo(t)
	legacyFile := filepath.Join(dest, "private", "custom.nix")
	if err := os.MkdirAll(filepath.Dir(legacyFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyFile, []byte("legacy user data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &backupMigrationFailureRunner{}
	state := validState()
	state.Dots = config.Dots{}
	err := Run(context.Background(), Options{
		Paths: paths.Options{
			RepoRoot:  repo,
			NixOSDest: dest,
			StatePath: filepath.Join(t.TempDir(), "state.json"),
		},
		State:     state,
		AssumeYes: true,
		Runner:    runner,
	})
	if err == nil || !strings.Contains(err.Error(), "skipped automatic") {
		t.Fatalf("Run() error = %v, want skipped automatic restore", err)
	}
	legacyData, readErr := os.ReadFile(legacyFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got := string(legacyData); got != "legacy user data\n" {
		t.Fatalf("legacy source changed after failed atomic rename: %q", got)
	}
	if _, err := os.Lstat(filepath.Join(dest, "user")); !os.IsNotExist(err) {
		t.Fatalf("canonical user path appeared after failed atomic rename: %v", err)
	}
	if commands := commandSummary(runner.calls); strings.Contains(commands, "sudo rsync -a --delete") {
		t.Fatalf("failed atomic rename must not run destructive restore, got:\n%s", commands)
	}
}

func TestCleanupLegacyStateRunsOnlyAfterCanonicalStateInstall(t *testing.T) {
	dir := t.TempDir()
	canonical := filepath.Join(dir, "installer-state.json")
	legacy := filepath.Join(dir, "wahrwelt", "state.json")
	current := config.Default()
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := config.Save(legacy, current); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nexec \"$@\"\n")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	runner := run.Runner{Stdout: io.Discard, Stderr: io.Discard}
	if err := writeState(context.Background(), runner, canonical, current); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(canonical); err != nil {
		t.Fatalf("canonical state was not installed before cleanup: %v", err)
	}
	if err := cleanupLegacyStatePathsForCurrent(context.Background(), runner, []string{legacy}, current); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy state remained after canonical state install: %v", err)
	}
}

func TestRunMigratesLoadedLegacyStateWhilePublishingValidEdit(t *testing.T) {
	repo, dest := fakeRepo(t)
	legacy := filepath.Join(dest, "wahrwelt", "state.json")
	original := validState()
	original.Dots = config.Dots{}
	if err := config.Save(legacy, original); err != nil {
		t.Fatal(err)
	}
	desired, proof, err := LoadStateWithProof(legacy)
	if err != nil {
		t.Fatalf("load legacy state with proof: %v", err)
	}
	desired.Source.Channel = config.SourceChannelDevelopment
	runner := &selectivePrivilegedRunner{}

	err = Run(context.Background(), Options{
		Paths: paths.Options{
			RepoRoot:  repo,
			NixOSDest: dest,
			StatePath: filepath.Join(dest, "installer-state.json"),
		},
		State:            desired,
		LoadedStateProof: proof,
		AssumeYes:        true,
		Runner:           runner,
	})
	if err != nil {
		t.Fatalf("Run() rejected a valid edit to loaded legacy state: %v", err)
	}
	canonical := filepath.Join(dest, "installer-state.json")
	published, err := config.LoadExisting(canonical)
	if err != nil {
		t.Fatalf("load canonical state: %v", err)
	}
	if got, want := published.Source.Channel, config.SourceChannelDevelopment; got != want {
		t.Fatalf("canonical source channel = %q, want %q", got, want)
	}
	if _, err := os.Lstat(legacy); !os.IsNotExist(err) {
		t.Fatalf("loaded legacy state remains after canonical publication: %v", err)
	}
}

func TestLoadStateWithProofRejectsUnownedGeneratedStateBytes(t *testing.T) {
	tests := []struct {
		name     string
		payload  []byte
		wantLoad bool
	}{
		{name: "partial state", payload: []byte(`{"user":{"username":"tester"}}` + "\n"), wantLoad: true},
		{name: "invalid UTF-8", payload: []byte("{\"host\":{\"hostname\":\"Nix\xffOS\"}}\n")},
		{name: "over one MiB", payload: []byte("{}" + strings.Repeat(" ", config.MaxGeneratedStateBytes))},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			statePath := filepath.Join(t.TempDir(), "wahrwelt", "state.json")
			if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(statePath, test.payload, 0o644); err != nil {
				t.Fatal(err)
			}

			_, proof, err := LoadStateWithProof(statePath)
			if proof.path != "" {
				t.Fatalf("LoadStateWithProof() proof = %#v, want no ownership proof", proof)
			}
			if test.wantLoad && err != nil {
				t.Fatalf("LoadStateWithProof() rejected supported partial state: %v", err)
			}
			if !test.wantLoad && err == nil {
				t.Fatal("LoadStateWithProof() accepted unsafe raw state bytes")
			}
			if got, err := os.ReadFile(statePath); err != nil || string(got) != string(test.payload) {
				t.Fatalf("rejected state changed: got %q err=%v", got, err)
			}
		})
	}
}

func TestLoadStateWithProofRejectsExternalHardlink(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "wahrwelt", "state.json")
	if err := config.Save(statePath, validState()); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "external-state.json")
	if err := os.Link(statePath, external); err != nil {
		t.Fatal(err)
	}

	if _, proof, err := LoadStateWithProof(statePath); err == nil || proof.path != "" {
		t.Fatalf("LoadStateWithProof() error = %v proof = %#v, want hardlink ownership error", err, proof)
	}
	for _, path := range []string{statePath, external} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("hardlink %s changed after rejection: %v", path, err)
		}
	}
}

func TestLoadStateWithProofRejectsSameInodeMutationBetweenReads(t *testing.T) {
	tests := []struct {
		name        string
		replacement []byte
	}{
		{name: "partial", replacement: []byte(`{"user":{"username":"mallory"}}` + "\n")},
		{name: "invalid", replacement: []byte("{invalid\n")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			statePath := filepath.Join(t.TempDir(), "installer-state.json")
			if err := config.Save(statePath, validState()); err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(statePath)
			if err != nil {
				t.Fatal(err)
			}

			_, proof, err := loadStateWithProof(statePath, stateProofHooks{
				beforeStrictRead: func() error {
					return os.WriteFile(statePath, test.replacement, 0o644)
				},
			})
			if err == nil || proof.path != "" {
				t.Fatalf("loadStateWithProof() error = %v proof = %#v, want same-inode mutation rejection", err, proof)
			}
			after, statErr := os.Lstat(statePath)
			if statErr != nil {
				t.Fatal(statErr)
			}
			if !os.SameFile(before, after) {
				t.Fatal("test did not mutate the original inode")
			}
			if got, readErr := os.ReadFile(statePath); readErr != nil || string(got) != string(test.replacement) {
				t.Fatalf("same-inode replacement changed after rejection: got %q err=%v", got, readErr)
			}
		})
	}
}

func TestLoadStateWithProofRevalidatesStablePartialPublicInode(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "installer-state.json")
	partial := []byte(`{"user":{"username":"tester"}}` + "\n")
	if err := os.WriteFile(statePath, partial, 0o644); err != nil {
		t.Fatal(err)
	}
	original, err := os.Lstat(statePath)
	if err != nil {
		t.Fatal(err)
	}
	replacementPath := filepath.Join(dir, "replacement.json")
	if err := os.WriteFile(replacementPath, partial, 0o644); err != nil {
		t.Fatal(err)
	}

	_, proof, err := loadStateWithProof(statePath, stateProofHooks{
		afterStrictRead: func() error {
			return os.Rename(replacementPath, statePath)
		},
	})
	if err == nil || proof.path != "" {
		t.Fatalf("loadStateWithProof() error = %v proof = %#v, want partial public-inode replacement rejection", err, proof)
	}
	visible, statErr := os.Lstat(statePath)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if os.SameFile(original, visible) {
		t.Fatal("test did not replace the public inode")
	}
	if got, readErr := os.ReadFile(statePath); readErr != nil || string(got) != string(partial) {
		t.Fatalf("public replacement changed after rejection: got %q err=%v", got, readErr)
	}
}

func TestRunRejectsPartialLoadedLegacyStateBeforeLiveMutation(t *testing.T) {
	repo, dest := fakeRepo(t)
	legacy := filepath.Join(dest, "wahrwelt", "state.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	partial := []byte(`{
  "host": {"hostname": "TestHost"},
  "user": {"username": "tester", "fullName": "tester", "homeDirectory": "/home/tester"},
  "git": {"username": "tester", "email": "tester@example.com"}
}` + "\n")
	if err := os.WriteFile(legacy, partial, 0o644); err != nil {
		t.Fatal(err)
	}
	desired, proof, err := LoadStateWithProof(legacy)
	if err != nil {
		t.Fatalf("load partial legacy draft: %v", err)
	}
	if proof.path != "" {
		t.Fatalf("partial legacy state received ownership proof: %#v", proof)
	}
	runner := &selectivePrivilegedRunner{}

	err = Run(context.Background(), Options{
		Paths: paths.Options{
			RepoRoot:  repo,
			NixOSDest: dest,
			StatePath: filepath.Join(dest, "installer-state.json"),
		},
		State:     desired,
		AssumeYes: true,
		Runner:    runner,
	})
	if err == nil || !strings.Contains(err.Error(), "no validated loaded state proof") {
		t.Fatalf("Run() error = %v, want partial legacy ownership collision", err)
	}
	if got, readErr := os.ReadFile(legacy); readErr != nil || string(got) != string(partial) {
		t.Fatalf("partial legacy state changed: got %q err=%v", got, readErr)
	}
	if _, statErr := os.Lstat(filepath.Join(dest, "installer-state.json")); !os.IsNotExist(statErr) {
		t.Fatalf("canonical state appeared after partial legacy rejection: %v", statErr)
	}
	commands := commandSummary(runner.calls)
	for _, forbidden := range []string{"sudo rsync", "nixos-rebuild switch", "systemctl --user"} {
		if strings.Contains(commands, forbidden) {
			t.Fatalf("partial legacy collision reached live mutation %q:\n%s", forbidden, commands)
		}
	}
}

func TestRunRejectsReplacedLoadedLegacyStateBeforeLiveMutation(t *testing.T) {
	repo, dest := fakeRepo(t)
	legacy := filepath.Join(dest, "wahrwelt", "state.json")
	original := validState()
	original.Dots = config.Dots{}
	if err := config.Save(legacy, original); err != nil {
		t.Fatal(err)
	}
	desired, proof, err := LoadStateWithProof(legacy)
	if err != nil {
		t.Fatalf("load legacy state with proof: %v", err)
	}
	desired.Source.Channel = config.SourceChannelDevelopment
	replacement := filepath.Join(filepath.Dir(legacy), "replacement.json")
	if err := config.Save(replacement, original); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, legacy); err != nil {
		t.Fatal(err)
	}
	runner := &selectivePrivilegedRunner{}

	err = Run(context.Background(), Options{
		Paths: paths.Options{
			RepoRoot:  repo,
			NixOSDest: dest,
			StatePath: filepath.Join(dest, "installer-state.json"),
		},
		State:            desired,
		LoadedStateProof: proof,
		AssumeYes:        true,
		Runner:           runner,
	})
	if err == nil || !strings.Contains(err.Error(), "loaded state inode changed") {
		t.Fatalf("Run() error = %v, want loaded legacy inode collision", err)
	}
	if _, statErr := os.Lstat(legacy); statErr != nil {
		t.Fatalf("replacement legacy state was not preserved: %v", statErr)
	}
	if _, statErr := os.Lstat(filepath.Join(dest, "installer-state.json")); !os.IsNotExist(statErr) {
		t.Fatalf("canonical state appeared after provenance collision: %v", statErr)
	}
	commands := commandSummary(runner.calls)
	for _, forbidden := range []string{"sudo rsync", "nixos-rebuild switch", "systemctl --user"} {
		if strings.Contains(commands, forbidden) {
			t.Fatalf("provenance collision reached live mutation %q:\n%s", forbidden, commands)
		}
	}
}

// cleanupLegacyStatePathsForCurrent keeps the expected-state contract visible
// in the tests while the production cleanup API is migrated to accept it.
func cleanupLegacyStatePathsForCurrent(ctx context.Context, runner run.CommandRunner, statePaths []string, current config.State) error {
	return cleanupLegacyStatePaths(ctx, runner, statePaths, current)
}

func TestCleanupLegacyStatePathsRejectsUnownedPayload(t *testing.T) {
	current := validState()
	divergent := current
	divergent.Host.Hostname = "OtherHost"
	tests := []struct {
		name  string
		write func(*testing.T, string)
	}{
		{
			name: "unknown json",
			write: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("{\"owner\":\"human\"}\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "malformed json",
			write: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("not installer state\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "divergent installer state",
			write: func(t *testing.T, path string) {
				t.Helper()
				if err := config.Save(path, divergent); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parent := filepath.Join(t.TempDir(), "wahrwelt")
			if err := os.Mkdir(parent, 0o755); err != nil {
				t.Fatal(err)
			}
			statePath := filepath.Join(parent, "state.json")
			test.write(t, statePath)
			before, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			bin := t.TempDir()
			writeExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nexec \"$@\"\n")
			t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

			err = cleanupLegacyStatePathsForCurrent(
				context.Background(),
				run.Runner{Stdout: io.Discard, Stderr: io.Discard},
				[]string{statePath},
				current,
			)
			if err == nil || !strings.Contains(err.Error(), "ownership collision") {
				t.Fatalf("cleanupLegacyStatePaths() error = %v, want ownership collision", err)
			}
			after, readErr := os.ReadFile(statePath)
			if readErr != nil {
				t.Fatalf("unowned legacy state was removed: %v", readErr)
			}
			if got, want := string(after), string(before); got != want {
				t.Fatalf("unowned legacy state changed to %q, want %q", got, want)
			}
		})
	}
}

func TestCleanupLegacyStatePathsRejectsPartialPayloadMatchingCurrentDefaults(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "wahrwelt")
	statePath := filepath.Join(parent, "state.json")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	current := validState()
	partial := []byte(`{
  "host": {"hostname": "TestHost"},
  "user": {"username": "tester", "fullName": "tester", "homeDirectory": "/home/tester"},
  "git": {"username": "tester", "email": "tester@example.com"}
}` + "\n")
	if err := os.WriteFile(statePath, partial, 0o644); err != nil {
		t.Fatal(err)
	}

	err := cleanupLegacyStatePathsForCurrent(
		context.Background(),
		run.Runner{Stdout: io.Discard, Stderr: io.Discard},
		[]string{statePath},
		current,
	)
	if err == nil || !strings.Contains(err.Error(), "ownership collision") {
		t.Fatalf("cleanupLegacyStatePaths() error = %v, want partial-state ownership collision", err)
	}
	if got, readErr := os.ReadFile(statePath); readErr != nil || string(got) != string(partial) {
		t.Fatalf("partial legacy state changed: got %q err=%v", got, readErr)
	}
}

func TestCleanupLegacyStatePathsRejectsUnownedRawBytesAndHardlinks(t *testing.T) {
	tests := []struct {
		name  string
		write func(*testing.T, string)
	}{
		{
			name: "invalid UTF-8",
			write: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("{\"host\":{\"hostname\":\"Nix\xffOS\"}}\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "over one MiB",
			write: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("{}"+strings.Repeat(" ", config.MaxGeneratedStateBytes)), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "external hardlink",
			write: func(t *testing.T, path string) {
				t.Helper()
				if err := config.Save(path, validState()); err != nil {
					t.Fatal(err)
				}
				if err := os.Link(path, filepath.Join(t.TempDir(), "external-state.json")); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parent := filepath.Join(t.TempDir(), "wahrwelt")
			if err := os.Mkdir(parent, 0o755); err != nil {
				t.Fatal(err)
			}
			statePath := filepath.Join(parent, "state.json")
			test.write(t, statePath)
			before, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}

			err = cleanupLegacyStatePathsForCurrent(
				context.Background(),
				run.Runner{Stdout: io.Discard, Stderr: io.Discard},
				[]string{statePath},
				validState(),
			)
			if err == nil || !strings.Contains(err.Error(), "ownership collision") {
				t.Fatalf("cleanupLegacyStatePaths() error = %v, want raw ownership collision", err)
			}
			if got, readErr := os.ReadFile(statePath); readErr != nil || string(got) != string(before) {
				t.Fatalf("rejected legacy state changed: got %q err=%v", got, readErr)
			}
		})
	}
}

func TestCleanupLegacyStatePathsPreflightsAllPayloadsBeforeRemovingAny(t *testing.T) {
	root := t.TempDir()
	current := validState()
	divergent := current
	divergent.Host.Hostname = "OtherHost"
	first := filepath.Join(root, "wahrwelt", "state.json")
	second := filepath.Join(root, "mysetup", "state.json")
	if err := config.Save(first, current); err != nil {
		t.Fatal(err)
	}
	if err := config.Save(second, divergent); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nexec \"$@\"\n")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	err := cleanupLegacyStatePathsForCurrent(
		context.Background(),
		run.Runner{Stdout: io.Discard, Stderr: io.Discard},
		[]string{first, second},
		current,
	)
	if err == nil || !strings.Contains(err.Error(), "ownership collision") {
		t.Fatalf("cleanupLegacyStatePaths() error = %v, want ownership collision", err)
	}
	for _, path := range []string{first, second} {
		if _, statErr := os.Lstat(path); statErr != nil {
			t.Fatalf("all-target preflight removed %s before reporting collision: %v", path, statErr)
		}
	}
}

func TestCleanupLegacyStatePathsAcceptsAllowlistedHistoricalPayload(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "wahrwelt")
	statePath := filepath.Join(parent, "state.json")
	current := validState()
	if err := config.Save(statePath, current); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var historical map[string]any
	if err := json.Unmarshal(data, &historical); err != nil {
		t.Fatal(err)
	}
	delete(historical["features"].(map[string]any), "portainer")
	historical["zapret"] = map[string]any{
		"enable": false,
		"config": "general (FAKE_TLS_AUTO_ALT3)",
	}
	historicalData, err := json.MarshalIndent(historical, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	historicalData = append(historicalData, '\n')
	if err := os.WriteFile(statePath, historicalData, 0o644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nexec \"$@\"\n")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	if err := cleanupLegacyStatePathsForCurrent(
		context.Background(),
		run.Runner{Stdout: io.Discard, Stderr: io.Discard},
		[]string{statePath},
		current,
	); err != nil {
		t.Fatalf("cleanupLegacyStatePaths() rejected allowlisted historical state: %v", err)
	}
	if _, err := os.Lstat(statePath); !os.IsNotExist(err) {
		t.Fatalf("allowlisted historical state remained after cleanup: %v", err)
	}
}

// selectivePrivilegedRunner models only the helper boundary: mkdir and the
// single privileged helper run against the test directory, while unrelated
// installer commands remain command-only.  This keeps state-write ordering
// tests honest without invoking Nix or mutating the host.
type selectivePrivilegedRunner struct {
	calls     []fakeCall
	failState bool
}

func (r *selectivePrivilegedRunner) Command(ctx context.Context, name string, args ...string) error {
	r.calls = append(r.calls, fakeCall{name: name, args: append([]string(nil), args...)})
	if handled, err := runValidatedStagingTestCommand(ctx, name, args...); handled {
		return err
	}
	if name != "sudo" || len(args) == 0 {
		return nil
	}
	if args[0] == "sh" && len(args) > 4 && args[4] == "state" && r.failState {
		return os.ErrPermission
	}
	if args[0] == "mkdir" || args[0] == "sh" {
		return exec.CommandContext(ctx, args[0], args[1:]...).Run()
	}
	return nil
}

func (r *selectivePrivilegedRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	r.calls = append(r.calls, fakeCall{name: name, args: append([]string(nil), args...)})
	if output, handled, err := runValidatedStagingTestOutput(ctx, name, args...); handled {
		return output, err
	}
	if name == "sudo" && len(args) > 0 && args[0] == "sh" {
		out, err := exec.CommandContext(ctx, args[0], args[1:]...).Output()
		return strings.TrimSpace(string(out)), err
	}
	return "", nil
}

func (*selectivePrivilegedRunner) IsDryRun() bool { return false }

func TestFailedCanonicalStateWriteLeavesLegacyStateUntouched(t *testing.T) {
	repo, dest := fakeRepo(t)
	runner := &selectivePrivilegedRunner{failState: true}
	state := validState()
	state.Dots = config.Dots{}
	opts := Options{
		Paths:     paths.Options{RepoRoot: repo, NixOSDest: dest, StatePath: paths.DefaultStatePath},
		State:     state,
		AssumeYes: true,
		Runner:    runner,
	}
	if err := Run(context.Background(), opts); err == nil {
		t.Fatal("expected state write error")
	}
	if commands := commandSummary(runner.calls); strings.Contains(commands, "sudo find -P "+filepath.Dir(paths.LegacyWahrweltStatePath)) {
		t.Fatalf("failed state write must not clean legacy state, got:\n%s", commands)
	}
}

func TestRunStableLegacyStateCollisionStopsBeforeLiveMutation(t *testing.T) {
	repo, dest := fakeRepo(t)
	legacy := filepath.Join(dest, "wahrwelt", "state.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	unknown := []byte("{\"owner\":\"human\"}\n")
	if err := os.WriteFile(legacy, unknown, 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &selectivePrivilegedRunner{}
	state := validState()
	state.Dots = config.Dots{}
	err := Run(context.Background(), Options{
		Paths: paths.Options{
			RepoRoot:  repo,
			NixOSDest: dest,
			StatePath: filepath.Join(dest, "installer-state.json"),
		},
		State:     state,
		AssumeYes: true,
		Runner:    runner,
	})
	if err == nil || !strings.Contains(err.Error(), "ownership collision") {
		t.Fatalf("Run() error = %v, want legacy state ownership collision", err)
	}
	data, readErr := os.ReadFile(legacy)
	if readErr != nil {
		t.Fatalf("stable legacy collision was removed: %v", readErr)
	}
	if got, want := string(data), string(unknown); got != want {
		t.Fatalf("stable legacy collision changed to %q, want %q", got, want)
	}
	commands := commandSummary(runner.calls)
	for _, forbidden := range []string{"sudo rsync", "nixos-rebuild switch", "systemctl --user"} {
		if strings.Contains(commands, forbidden) {
			t.Fatalf("stable legacy collision reached live mutation %q:\n%s", forbidden, commands)
		}
	}
	if _, statErr := os.Lstat(filepath.Join(dest, "installer-state.json")); !os.IsNotExist(statErr) {
		t.Fatalf("stable legacy collision published canonical state: %v", statErr)
	}
}

func TestCustomStatePathDoesNotCleanLegacyState(t *testing.T) {
	repo, dest := fakeRepo(t)
	runner := &selectivePrivilegedRunner{}
	state := validState()
	state.Dots = config.Dots{}
	opts := Options{
		Paths:     paths.Options{RepoRoot: repo, NixOSDest: dest, StatePath: filepath.Join(t.TempDir(), "state.json")},
		State:     state,
		AssumeYes: true,
		Runner:    runner,
	}
	if err := Run(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if commands := commandSummary(runner.calls); strings.Contains(commands, "sudo find -P "+filepath.Dir(paths.LegacyWahrweltStatePath)) {
		t.Fatalf("custom state path must not clean legacy state, got:\n%s", commands)
	}
}

func TestWriteStateRejectsCommandOnlyNonDryRunner(t *testing.T) {
	target := filepath.Join(t.TempDir(), "installer-state.json")
	if err := writeState(context.Background(), &fakeRunner{}, target, config.Default()); err == nil {
		t.Fatal("command-only non-dry runner was accepted for state publication")
	}
}

func TestWriteStateRejectsNoOpNonDryRunnerWhenExistingBytesMatch(t *testing.T) {
	target := filepath.Join(t.TempDir(), "installer-state.json")
	state := config.Default()
	if err := config.Save(target, state); err != nil {
		t.Fatal(err)
	}
	err := writeState(context.Background(), &fakeRunner{}, target, state)
	if err == nil || !strings.Contains(err.Error(), "was not replaced") {
		t.Fatalf("writeState() error = %v, want no-op replace rejection", err)
	}
}

func TestWriteStateDryRunUsesAtomicNoClobberPublication(t *testing.T) {
	fake := &fakeRunner{dryRun: true}
	if err := writeState(context.Background(), fake, filepath.Join(t.TempDir(), "installer-state.json"), config.Default()); err != nil {
		t.Fatal(err)
	}
	commands := commandSummary(fake.calls)
	if !strings.Contains(commands, "sudo sh -c") || !strings.Contains(commands, "ln -L -T -- /proc/self/fd/9") {
		t.Fatalf("dry state install must model the helper's atomic no-clobber publication, got:\n%s", commands)
	}
}

func TestOpenPinnedRegularFileRejectsFIFOWithoutBlocking(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "state.fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	// The child starts a race-instrumented test binary in the -race suite, so
	// keep this bounded but allow its normal runtime initialization overhead.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestOpenPinnedRegularFileFIFOHelper$")
	cmd.Env = append(os.Environ(), "WAHRWELT_FIFO_PROBE="+fifo)
	if output, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			t.Fatalf("opening a raced FIFO blocked instead of failing closed: %v", ctx.Err())
		}
		t.Fatalf("FIFO helper failed: %v\n%s", err, output)
	}
}

func TestOpenPinnedRegularFileFIFOHelper(t *testing.T) {
	path := os.Getenv("WAHRWELT_FIFO_PROBE")
	if path == "" {
		return
	}
	file, err := openPinnedRegularFile(path, "state")
	if file != nil {
		defer file.file.Close()
	}
	if err == nil {
		t.Fatal("FIFO was accepted as a pinned regular file")
	}
}

func TestWriteStateDoesNotExposePublicTempDuringPrivilegedInstall(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "installer-state.json")
	state := config.Default()
	state.Host.Hostname = "pinned-state"
	probe := filepath.Join(dir, ".installer-state.json.tmp.raced")
	privilegedInstallProbeToolchain(t, probe)
	if err := writeState(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, target, state); err != nil {
		t.Fatal(err)
	}
	got, err := config.LoadExisting(target)
	if err != nil {
		t.Fatal(err)
	}
	if got.Host.Hostname != state.Host.Hostname {
		t.Fatalf("published state hostname = %q, want %q", got.Host.Hostname, state.Host.Hostname)
	}
	probeData, err := os.ReadFile(probe)
	if err != nil {
		t.Fatalf("public replacement written after helper install was removed: %v", err)
	}
	if got, want := string(probeData), "unknown public replacement\n"; got != want {
		t.Fatalf("public replacement = %q, want %q", got, want)
	}
}

func TestWriteStateReplacesExistingRegularStateAndCleansOwnedTemp(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "installer-state.json")
	initial := config.Default()
	initial.Host.Hostname = "before"
	if err := config.Save(target, initial); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nexec \"$@\"\n")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	updated := config.Default()
	updated.Host.Hostname = "after"
	if err := writeState(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, target, updated); err != nil {
		t.Fatal(err)
	}
	got, err := config.LoadExisting(target)
	if err != nil {
		t.Fatal(err)
	}
	if got.Host.Hostname != updated.Host.Hostname {
		t.Fatalf("state hostname = %q, want %q", got.Host.Hostname, updated.Host.Hostname)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".installer-state.json.tmp.*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("owned temporary state file remained: %v", matches)
	}
}

type stateParentSwapBeforeOpenRunner struct {
	parent   string
	retained string
	raced    bool
}

func (r *stateParentSwapBeforeOpenRunner) Command(ctx context.Context, name string, args ...string) error {
	if name == "sudo" && len(args) > 0 && args[0] == "mkdir" && !r.raced {
		r.raced = true
		if err := os.Rename(r.parent, r.retained); err != nil {
			return err
		}
		if err := os.Mkdir(r.parent, 0o755); err != nil {
			return err
		}
	}
	return exec.CommandContext(ctx, args[0], args[1:]...).Run()
}

func (*stateParentSwapBeforeOpenRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (*stateParentSwapBeforeOpenRunner) IsDryRun() bool { return false }

func TestWriteStatePinsParentBeforePrivilegedHandoff(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "state-parent")
	retained := filepath.Join(root, "state-parent-before-race")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(parent, "installer-state.json")
	state := config.Default()
	state.Host.Hostname = "pinned-before-open"

	err := writeState(context.Background(), &stateParentSwapBeforeOpenRunner{parent: parent, retained: retained}, target, state)
	if err == nil {
		t.Fatal("state parent replacement before the old open was accepted")
	}
	if _, statErr := os.Lstat(target); !os.IsNotExist(statErr) {
		t.Fatalf("replacement state parent was modified: %v", statErr)
	}
	retainedState, loadErr := config.LoadExisting(filepath.Join(retained, "installer-state.json"))
	if loadErr != nil {
		t.Fatalf("published state was not retained under the pinned parent: %v", loadErr)
	}
	if got, want := retainedState.Host.Hostname, state.Host.Hostname; got != want {
		t.Fatalf("retained state hostname = %q, want %q", got, want)
	}
}

type stateParentSwapAfterPublishRunner struct {
	parent      string
	retained    string
	replacement os.FileInfo
	state       config.State
	raced       bool
}

func (r *stateParentSwapAfterPublishRunner) Command(ctx context.Context, name string, args ...string) error {
	err := exec.CommandContext(ctx, args[0], args[1:]...).Run()
	if err != nil || name != "sudo" || len(args) == 0 || args[0] != "sh" || r.raced {
		return err
	}
	r.raced = true
	if renameErr := os.Rename(r.parent, r.retained); renameErr != nil {
		return renameErr
	}
	if mkdirErr := os.Mkdir(r.parent, 0o755); mkdirErr != nil {
		return mkdirErr
	}
	target := filepath.Join(r.parent, "installer-state.json")
	if saveErr := config.Save(target, r.state); saveErr != nil {
		return saveErr
	}
	info, statErr := os.Stat(target)
	if statErr != nil {
		return statErr
	}
	r.replacement = info
	return nil
}

func (*stateParentSwapAfterPublishRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (*stateParentSwapAfterPublishRunner) IsDryRun() bool { return false }

func TestWriteStateRejectsParentSwapAfterPublication(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "state-parent")
	retained := filepath.Join(root, "state-parent-after-publish")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(parent, "installer-state.json")
	state := config.Default()
	state.Host.Hostname = "same-visible-bytes"
	runner := &stateParentSwapAfterPublishRunner{parent: parent, retained: retained, state: state}

	err := writeState(context.Background(), runner, target, state)
	if err == nil {
		t.Fatal("post-publication state parent replacement was accepted")
	}
	visibleInfo, statErr := os.Stat(target)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if runner.replacement == nil || !os.SameFile(runner.replacement, visibleInfo) {
		t.Fatal("replacement state was modified during pinned verification")
	}
	retainedState, loadErr := config.LoadExisting(filepath.Join(retained, "installer-state.json"))
	if loadErr != nil {
		t.Fatalf("published state was not retained under the original parent: %v", loadErr)
	}
	if got, want := retainedState.Host.Hostname, state.Host.Hostname; got != want {
		t.Fatalf("retained state hostname = %q, want %q", got, want)
	}
}

type stateSymlinkRaceRunner struct {
	target string
	victim string
}

func (r stateSymlinkRaceRunner) Command(ctx context.Context, name string, args ...string) error {
	if name == "sudo" && len(args) > 0 && args[0] == "sh" {
		if err := os.Symlink(r.victim, r.target); err != nil {
			return err
		}
	}
	return exec.CommandContext(ctx, args[0], args[1:]...).Run()
}

func (stateSymlinkRaceRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (stateSymlinkRaceRunner) IsDryRun() bool { return false }

func TestWriteStateRejectsSymlinkCreatedDuringPublication(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "installer-state.json")
	victim := filepath.Join(dir, "victim.json")
	if err := os.WriteFile(victim, []byte("user data\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := writeState(context.Background(), stateSymlinkRaceRunner{target: target, victim: victim}, target, config.Default())
	if err == nil {
		t.Fatal("expected raced symlink rejection")
	}
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("raced target was replaced with mode %v", info.Mode())
	}
}

func TestWriteStateRejectsSymlinkedStateParent(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	parent := filepath.Join(root, "state-parent")
	if err := os.Symlink(outside, parent); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(parent, "installer-state.json")
	fake := &fakeRunner{}
	if err := writeState(context.Background(), fake, target, config.Default()); err == nil {
		t.Fatal("symlinked state parent was accepted")
	}
	if _, err := os.Lstat(filepath.Join(outside, "installer-state.json")); !os.IsNotExist(err) {
		t.Fatalf("state write escaped through symlinked parent: %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("symlinked state parent must fail before privileged commands: %#v", fake.calls)
	}
}

func TestWriteStateRejectsSymlinkedStateAncestor(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	ancestor := filepath.Join(root, "namespace")
	if err := os.Symlink(outside, ancestor); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(ancestor, "nested", "installer-state.json")
	fake := &fakeRunner{}
	if err := writeState(context.Background(), fake, target, config.Default()); err == nil {
		t.Fatal("symlinked state ancestor was accepted")
	}
	if len(fake.calls) != 0 {
		t.Fatalf("symlinked state ancestor must fail before privileged commands: %#v", fake.calls)
	}
}

func TestOpenPinnedParentDirectoryRejectsSymlinkedAncestor(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(outside, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	ancestor := filepath.Join(root, "namespace")
	if err := os.Symlink(outside, ancestor); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(ancestor, "nested", "installer-state.json")
	if parent, err := openPinnedParentDirectory(path, "state"); err == nil {
		_ = parent.Close()
		t.Fatal("pinned parent walk followed a symlinked ancestor")
	}
}

type existingStateSymlinkRaceRunner struct {
	target string
	victim string
	raced  bool
}

func (r *existingStateSymlinkRaceRunner) Command(ctx context.Context, name string, args ...string) error {
	if name == "sudo" && len(args) > 0 && args[0] == "sh" && !r.raced {
		r.raced = true
		if err := os.Remove(r.target); err != nil {
			return err
		}
		if err := os.Symlink(r.victim, r.target); err != nil {
			return err
		}
	}
	return exec.CommandContext(ctx, args[0], args[1:]...).Run()
}

func (*existingStateSymlinkRaceRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (*existingStateSymlinkRaceRunner) IsDryRun() bool { return false }

func TestWriteStateRestoresExistingSymlinkRaceWinner(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "installer-state.json")
	victim := filepath.Join(dir, "victim.json")
	if err := config.Save(target, config.Default()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(victim, []byte("user data\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := writeState(context.Background(), &existingStateSymlinkRaceRunner{target: target, victim: victim}, target, config.Default())
	if err == nil {
		t.Fatal("expected raced symlink rejection")
	}
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("raced target was replaced with mode %v", info.Mode())
	}
	data, err := os.ReadFile(victim)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "user data\n"; got != want {
		t.Fatalf("raced symlink target changed to %q", got)
	}
}

func stateRaceToolchain(t *testing.T, target, first, second string, failSecond bool) {
	t.Helper()
	realMV, err := exec.LookPath("mv")
	if err != nil {
		t.Fatal(err)
	}
	realRM, err := exec.LookPath("rm")
	if err != nil {
		t.Fatal(err)
	}
	realLN, err := exec.LookPath("ln")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nexec \"$@\"\n")
	writeExecutable(t, filepath.Join(bin, "mv"), `#!/bin/sh
set -eu
count_file=${WAHRWELT_STATE_RACE_COUNTER:?}
count=0
if [ -f "$count_file" ]; then
	count=$(cat "$count_file")
fi
count=$((count + 1))
printf '%s\n' "$count" > "$count_file"
if [ "$count" = 1 ]; then
	"$WAHRWELT_REAL_RM" -f -- "$WAHRWELT_STATE_RACE_TARGET"
	"$WAHRWELT_REAL_LN" -s -- "$WAHRWELT_STATE_RACE_FIRST" "$WAHRWELT_STATE_RACE_TARGET"
fi
if [ "$count" = 2 ]; then
	if [ "${WAHRWELT_STATE_RACE_FAIL_SECOND:-}" = 1 ]; then
		exit 42
	fi
	"$WAHRWELT_REAL_RM" -f -- "$WAHRWELT_STATE_RACE_TARGET"
	"$WAHRWELT_REAL_MV" -T --no-copy -- "$WAHRWELT_STATE_RACE_SECOND" "$WAHRWELT_STATE_RACE_TARGET"
fi
exec "$WAHRWELT_REAL_MV" "$@"
`)
	t.Setenv("WAHRWELT_STATE_RACE_COUNTER", filepath.Join(t.TempDir(), "mv-count"))
	t.Setenv("WAHRWELT_STATE_RACE_TARGET", target)
	t.Setenv("WAHRWELT_STATE_RACE_FIRST", first)
	t.Setenv("WAHRWELT_STATE_RACE_SECOND", second)
	t.Setenv("WAHRWELT_STATE_RACE_FAIL_SECOND", map[bool]string{true: "1", false: ""}[failSecond])
	t.Setenv("WAHRWELT_REAL_MV", realMV)
	t.Setenv("WAHRWELT_REAL_RM", realRM)
	t.Setenv("WAHRWELT_REAL_LN", realLN)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
}

func recoveryPayload(t *testing.T, parent string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(parent, ".wahrwelt-installer-recovery-*", "payload"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("recovery payloads = %v, want exactly one", matches)
	}
	return matches[0]
}

func TestWriteStatePreservesRacedNodeWhenExchangeRollbackFails(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "installer-state.json")
	victim := filepath.Join(dir, "victim.json")
	if err := config.Save(target, config.Default()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(victim, []byte("user data\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stateRaceToolchain(t, target, victim, victim, true)
	err := writeState(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, target, config.Default())
	if err == nil {
		t.Fatal("expected injected state rollback failure")
	}
	info, err := os.Lstat(recoveryPayload(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("preserved raced node mode = %v, want symlink", info.Mode())
	}
}

func TestWriteStatePreservesSecondRaceWinnerDuringRollback(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "installer-state.json")
	first := filepath.Join(dir, "first-victim.json")
	second := filepath.Join(dir, "second-victim.json")
	if err := config.Save(target, config.Default()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(first, []byte("first race winner\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("second race winner\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stateRaceToolchain(t, target, first, second, false)
	err := writeState(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, target, config.Default())
	if err == nil {
		t.Fatal("expected state race rejection")
	}
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("first raced target was not restored: %v", info.Mode())
	}
	data, err := os.ReadFile(recoveryPayload(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "second race winner\n"; got != want {
		t.Fatalf("preserved second race winner = %q, want %q", got, want)
	}
}

type canceledStateCleanupRunner struct {
	cancel context.CancelFunc
}

func (r canceledStateCleanupRunner) Command(ctx context.Context, name string, args ...string) error {
	if name == "sudo" && len(args) > 0 && args[0] == "sh" {
		r.cancel()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return exec.CommandContext(ctx, args[0], args[1:]...).Run()
}

func (canceledStateCleanupRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (canceledStateCleanupRunner) IsDryRun() bool { return false }

func TestWriteStateCleansPrivilegedTempAfterContextCancellation(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "installer-state.json")
	ctx, cancel := context.WithCancel(context.Background())
	err := writeState(ctx, canceledStateCleanupRunner{cancel: cancel}, target, config.Default())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("writeState() error = %v, want context cancellation", err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".installer-state.json.tmp.*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("privileged state temp leaked after cancellation: %v", matches)
	}
	if _, loadErr := config.LoadExisting(target); loadErr != nil {
		t.Fatalf("safe helper must finish publication before surfacing cancellation: %v", loadErr)
	}
}

type stateTempCleanupReplacementRaceRunner struct {
	dir               string
	publicationFailed bool
}

func (r *stateTempCleanupReplacementRaceRunner) Command(ctx context.Context, name string, args ...string) error {
	if name == "sudo" && len(args) > 0 && args[0] == "sh" && !r.publicationFailed {
		r.publicationFailed = true
		replacement := filepath.Join(r.dir, ".installer-state.json.tmp.raced")
		if err := os.WriteFile(replacement, []byte("raced replacement\n"), 0o644); err != nil {
			return err
		}
		return errors.New("injected state publication failure")
	}
	return exec.CommandContext(ctx, args[0], args[1:]...).Run()
}

func (*stateTempCleanupReplacementRaceRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (*stateTempCleanupReplacementRaceRunner) IsDryRun() bool { return false }

func TestWriteStatePreservesReplacementCreatedBeforePrivilegedTempCleanup(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "installer-state.json")
	runner := &stateTempCleanupReplacementRaceRunner{dir: dir}

	err := writeState(context.Background(), runner, target, config.Default())
	if err == nil || !strings.Contains(err.Error(), "injected state publication failure") {
		t.Fatalf("writeState() error = %v, want injected publication failure", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".installer-state.json.tmp.raced"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "raced replacement\n"; got != want {
		t.Fatalf("raced replacement = %q, want %q", got, want)
	}
}

func TestPrivilegedPublishUsesSameParentRecoveryQuarantine(t *testing.T) {
	if strings.Contains(privilegedPublishScript, "/tmp") {
		t.Fatalf("privileged publish must not depend on a cross-filesystem /tmp quarantine:\n%s", privilegedPublishScript)
	}
	for _, want := range []string{
		`os.mkdir(name, 0o700, dir_fd=parent)`,
		`opened_id=$(stat -Lc '%d:%i' -- /proc/self/fd/8)`,
		`[ "$opened_id" != "$created_id" ]`,
	} {
		if !strings.Contains(privilegedPublishScript, want) {
			t.Fatalf("privileged publish must pin the created quarantine before mutation; missing %q:\n%s", want, privilegedPublishScript)
		}
	}
	if !strings.Contains(privilegedPublishScript, "ln -L -T -- /proc/self/fd/9") || !strings.Contains(privilegedPublishScript, "mv --exchange -T --no-copy") {
		t.Fatalf("privileged publish must use the exact pinned inode for fresh targets and no-copy exchange for replacements:\n%s", privilegedPublishScript)
	}
}

func TestCleanupLegacyStatePathsRejectsSymlinkedParent(t *testing.T) {
	outside := t.TempDir()
	outsideState := filepath.Join(outside, "state.json")
	if err := os.WriteFile(outsideState, []byte("external state\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(t.TempDir(), "wahrwelt")
	if err := os.Symlink(outside, parent); err != nil {
		t.Fatal(err)
	}
	fake := &fakeRunner{}
	if err := cleanupLegacyStatePaths(context.Background(), fake, []string{filepath.Join(parent, "state.json")}, validState()); err == nil {
		t.Fatal("expected symlinked legacy parent rejection")
	}
	data, err := os.ReadFile(outsideState)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "external state\n"; got != want {
		t.Fatalf("external state changed to %q", got)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("symlinked parent must fail before cleanup commands: %#v", fake.calls)
	}
}

func TestCleanupLegacyStatePathsRejectsSymlinkedLeaf(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "wahrwelt")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	outsideState := filepath.Join(t.TempDir(), "external-state.json")
	if err := os.WriteFile(outsideState, []byte("external state\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(parent, "state.json")
	if err := os.Symlink(outsideState, statePath); err != nil {
		t.Fatal(err)
	}
	fake := &fakeRunner{}
	if err := cleanupLegacyStatePaths(context.Background(), fake, []string{statePath}, validState()); err == nil {
		t.Fatal("expected symlinked legacy state rejection")
	}
	data, err := os.ReadFile(outsideState)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "external state\n"; got != want {
		t.Fatalf("external state changed to %q", got)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("symlinked state must fail before cleanup commands: %#v", fake.calls)
	}
}

type legacyParentSwapRunner struct {
	parent  string
	outside string
	swapped bool
}

func (r *legacyParentSwapRunner) Command(ctx context.Context, name string, args ...string) error {
	if name == "sudo" && len(args) > 0 && args[0] == "sh" && !r.swapped {
		r.swapped = true
		if err := os.Remove(filepath.Join(r.parent, "state.json")); err != nil {
			return err
		}
		if err := os.Remove(r.parent); err != nil {
			return err
		}
		if err := os.Symlink(r.outside, r.parent); err != nil {
			return err
		}
	}
	return exec.CommandContext(ctx, args[0], args[1:]...).Run()
}

func (*legacyParentSwapRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (*legacyParentSwapRunner) IsDryRun() bool { return false }

func TestCleanupLegacyStatePathsDoesNotTraverseRacedSymlinkedParent(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "wahrwelt")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	current := validState()
	if err := config.Save(filepath.Join(parent, "state.json"), current); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	outsideState := filepath.Join(outside, "state.json")
	if err := os.WriteFile(outsideState, []byte("external state\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := cleanupLegacyStatePaths(context.Background(), &legacyParentSwapRunner{parent: parent, outside: outside}, []string{filepath.Join(parent, "state.json")}, current)
	if err == nil {
		t.Fatal("expected raced symlinked parent rejection")
	}
	data, err := os.ReadFile(outsideState)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "external state\n"; got != want {
		t.Fatalf("external state changed to %q", got)
	}
}

type legacyStateDirectorySwapRunner struct {
	parent      string
	displaced   string
	replacement string
	swapped     bool
}

func (r *legacyStateDirectorySwapRunner) Command(ctx context.Context, name string, args ...string) error {
	if name == "sudo" && len(args) > 0 && args[0] == "sh" && !r.swapped {
		r.swapped = true
		if err := os.Rename(r.parent, r.displaced); err != nil {
			return err
		}
		if err := os.Mkdir(r.parent, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(r.parent, "state.json"), []byte(r.replacement), 0o644); err != nil {
			return err
		}
	}
	return exec.CommandContext(ctx, args[0], args[1:]...).Run()
}

func (*legacyStateDirectorySwapRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (*legacyStateDirectorySwapRunner) IsDryRun() bool { return false }

func TestCleanupLegacyStatePathsRejectsVisibleParentReplacementAfterPin(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "wahrwelt")
	displaced := filepath.Join(root, "wahrwelt-pinned")
	statePath := filepath.Join(parent, "state.json")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	current := validState()
	if err := config.Save(statePath, current); err != nil {
		t.Fatal(err)
	}
	expectedData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	runner := &legacyStateDirectorySwapRunner{
		parent:      parent,
		displaced:   displaced,
		replacement: "replacement state\n",
	}

	err = cleanupLegacyStatePaths(context.Background(), runner, []string{statePath}, current)
	if err == nil {
		t.Fatal("cleanupLegacyStatePaths() accepted a replaced visible legacy parent")
	}
	data, err := os.ReadFile(filepath.Join(displaced, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), string(expectedData); got != want {
		t.Fatalf("pinned legacy state = %q, want %q", got, want)
	}
	data, err = os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "replacement state\n"; got != want {
		t.Fatalf("visible replacement state = %q, want %q", got, want)
	}
}

func TestCleanupLegacyStatePathsDeletesExactRegularStateWithoutRemovingWritableParent(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "wahrwelt")
	statePath := filepath.Join(parent, "state.json")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	current := validState()
	if err := config.Save(statePath, current); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nexec \"$@\"\n")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	if err := cleanupLegacyStatePaths(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, []string{statePath}, current); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(statePath); !os.IsNotExist(err) {
		t.Fatalf("legacy state remained after cleanup: %v", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatalf("writable legacy parent must be preserved rather than path-racy pruned: %v", err)
	}
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), ".wahrwelt-installer-recovery-") {
		t.Fatalf("writable legacy parent entries = %#v, want only managed quarantine", entries)
	}
}

type legacyStateReplacementRunner struct {
	path  string
	raced bool
}

func (r *legacyStateReplacementRunner) Command(ctx context.Context, name string, args ...string) error {
	if name == "sudo" && len(args) > 0 && args[0] == "sh" && !r.raced {
		r.raced = true
		if err := os.Remove(r.path); err != nil {
			return err
		}
		if err := os.WriteFile(r.path, []byte("raced legacy state\n"), 0o644); err != nil {
			return err
		}
	}
	return exec.CommandContext(ctx, args[0], args[1:]...).Run()
}

func (*legacyStateReplacementRunner) Output(context.Context, string, ...string) (string, error) {
	return "", nil
}

func (*legacyStateReplacementRunner) IsDryRun() bool { return false }

func TestCleanupLegacyStatePathsPreservesRegularReplacementBeforeDelete(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "wahrwelt")
	statePath := filepath.Join(parent, "state.json")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	current := validState()
	if err := config.Save(statePath, current); err != nil {
		t.Fatal(err)
	}
	err := cleanupLegacyStatePaths(context.Background(), &legacyStateReplacementRunner{path: statePath}, []string{statePath}, current)
	if err == nil {
		t.Fatal("cleanupLegacyStatePaths() accepted a replaced legacy state")
	}
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "raced legacy state\n"; got != want {
		t.Fatalf("raced legacy state = %q, want %q", got, want)
	}
}

func TestRewriteLegacyUserImportKeepsNewlineEscapedIndentedString(t *testing.T) {
	input := "payload = '' keep ''\\\n./private '';\nimports = [ ./private ];\n"
	want := "payload = '' keep ''\\\n./private '';\nimports = [ ./user ];\n"
	if got := rewriteLegacyUserImport(input); got != want {
		t.Fatalf("rewritten configuration:\n%s\nwant:\n%s", got, want)
	}
}
