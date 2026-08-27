package apply

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/defaults"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
	"golang.org/x/sys/unix"
)

type publicationRaceRunner struct {
	before func() error
	raced  bool
}

func (r *publicationRaceRunner) race() error {
	if r.raced || r.before == nil {
		return nil
	}
	r.raced = true
	return r.before()
}

func (r *publicationRaceRunner) Command(ctx context.Context, name string, args ...string) error {
	if name == "sudo" {
		if err := r.race(); err != nil {
			return err
		}
		if len(args) == 0 {
			return fmt.Errorf("sudo command is empty")
		}
		return exec.CommandContext(ctx, args[0], args[1:]...).Run()
	}
	return exec.CommandContext(ctx, name, args...).Run()
}

func (r *publicationRaceRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	if name == "sudo" {
		if err := r.race(); err != nil {
			return "", err
		}
		if len(args) == 0 {
			return "", fmt.Errorf("sudo command is empty")
		}
		name, args = args[0], args[1:]
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s failed: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (r *publicationRaceRunner) OutputInPinnedDirectory(ctx context.Context, directory *os.File, name string, args ...string) (string, error) {
	if err := r.race(); err != nil {
		return "", err
	}
	return runValidatedStagingPinnedOutput(ctx, directory, name, args...)
}

func (*publicationRaceRunner) IsDryRun() bool { return false }

type validatedStagingMutationRunner struct {
	original      string
	dryBuildFlake string
}

func (r *validatedStagingMutationRunner) Command(ctx context.Context, name string, args ...string) error {
	if name == "sudo" && len(args) > 1 && args[0] == "nixos-rebuild" && args[1] == "dry-build" {
		for index := range args {
			if args[index] == "--flake" && index+1 < len(args) {
				r.dryBuildFlake = strings.TrimPrefix(strings.SplitN(args[index+1], "#", 2)[0], "path:")
				break
			}
		}
		if r.dryBuildFlake == "" {
			return fmt.Errorf("dry-build did not contain a flake path")
		}
		mutations := map[string]string{
			"configuration.nix":    "# raced after validation\n",
			"user/default.nix":     "raced user template\n",
			"hashed-password.nix":  "raced password hash\n",
			"secrets/secrets.yaml": "raced secret\n",
		}
		for rel, content := range mutations {
			if err := os.WriteFile(filepath.Join(r.original, rel), []byte(content), 0o644); err != nil {
				return err
			}
		}
		return nil
	}
	if filepath.Base(name) == "nix-store" {
		return exec.CommandContext(ctx, name, args...).Run()
	}
	if name == "rsync" {
		return exec.CommandContext(ctx, name, args...).Run()
	}
	// Flake locking is not relevant to this unit regression.
	return nil
}

func (r *validatedStagingMutationRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	if filepath.Base(name) != "nix" {
		return "", fmt.Errorf("unexpected output command %s %s", name, strings.Join(args, " "))
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s failed: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (r *validatedStagingMutationRunner) OutputInPinnedDirectory(ctx context.Context, directory *os.File, name string, args ...string) (string, error) {
	return run.Runner{Stdout: io.Discard, Stderr: io.Discard}.OutputInPinnedDirectory(ctx, directory, name, args...)
}

func (*validatedStagingMutationRunner) IsDryRun() bool { return false }

type userNamespaceCleanupRunner struct{}

func (*userNamespaceCleanupRunner) Command(ctx context.Context, name string, args ...string) error {
	return runUserNamespaceCleanup(ctx, name, args, nil)
}

func runUserNamespaceCleanup(ctx context.Context, name string, args []string, transform func(string) string) error {
	if name != "sudo" || len(args) < 11 || args[2] != privilegedStagingCleanupPython {
		return fmt.Errorf("unexpected cleanup command %s %s", name, strings.Join(args, " "))
	}
	commandArgs := append([]string(nil), args...)
	if transform != nil {
		commandArgs[2] = transform(commandArgs[2])
	}
	commandArgs[len(commandArgs)-3] = "0"
	commandArgs[len(commandArgs)-2] = "0"
	parent, err := os.Open(commandArgs[3])
	if err != nil {
		return err
	}
	defer parent.Close()
	expected, err := os.Open(commandArgs[5])
	if err != nil {
		return err
	}
	defer expected.Close()
	commandArgs[3] = "/proc/self/fd/3"
	commandArgs[5] = "/proc/self/fd/4"
	cmd := exec.CommandContext(ctx, "unshare", append([]string{"-Ur", commandArgs[0]}, commandArgs[1:]...)...)
	cmd.ExtraFiles = []*os.File{parent, expected}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("user-namespace cleanup failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (*userNamespaceCleanupRunner) Output(context.Context, string, ...string) (string, error) {
	return "", fmt.Errorf("unexpected cleanup output command")
}

func (*userNamespaceCleanupRunner) IsDryRun() bool { return false }

type rootOnlySecretsRunner struct{ calls int }

func (r *rootOnlySecretsRunner) Command(ctx context.Context, name string, args ...string) error {
	if name != "sudo" || len(args) != 7 || args[1] != "-c" || args[2] != privilegedCopyEncryptedSecretsPython {
		return fmt.Errorf("unexpected root-only secrets command %s %s", name, strings.Join(args, " "))
	}
	r.calls++
	commandArgs := append([]string(nil), args...)
	// The test process cannot gain host root. Temporarily expose the exact
	// allowlisted source only inside this sudo stand-in after the caller has
	// already selected the permission fallback.
	if err := os.Chmod(commandArgs[3], 0o700); err != nil {
		return err
	}
	defer os.Chmod(commandArgs[3], 0o000)
	cmd := exec.CommandContext(ctx, commandArgs[0], commandArgs[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("root-only secrets helper failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (*rootOnlySecretsRunner) Output(context.Context, string, ...string) (string, error) {
	return "", fmt.Errorf("unexpected root-only secrets output command")
}

func (*rootOnlySecretsRunner) IsDryRun() bool { return false }

type crashingUserNamespaceCleanupRunner struct{}

func (*crashingUserNamespaceCleanupRunner) Command(ctx context.Context, name string, args ...string) error {
	return runUserNamespaceCleanup(ctx, name, args, func(script string) string {
		needle := "    require_public_expected(parent)\n\n    tree = os.open"
		replacement := "    require_public_expected(parent)\n    os._exit(73)\n\n    tree = os.open"
		return strings.Replace(script, needle, replacement, 1)
	})
}

func (*crashingUserNamespaceCleanupRunner) Output(context.Context, string, ...string) (string, error) {
	return "", fmt.Errorf("unexpected cleanup output command")
}

func (*crashingUserNamespaceCleanupRunner) IsDryRun() bool { return false }

func TestPrepareStagedApplyDryBuildsImmutablePublicationCandidate(t *testing.T) {
	repo, dest := fakeRepo(t)
	seed := map[string]string{
		"user/default.nix":     "expected user template\n",
		"hashed-password.nix":  "expected password hash\n",
		"secrets/secrets.yaml": testEncryptedSecretsYAML,
	}
	for rel, content := range seed {
		writePublicationSource(t, dest, rel, content)
	}
	staging := t.TempDir()
	runner := &validatedStagingMutationRunner{original: staging}
	opts := Options{
		Paths: paths.Options{RepoRoot: repo, NixOSDest: dest},
		State: validState(),
	}
	sources, err := paths.ResolveSources(repo)
	if err != nil {
		t.Fatal(err)
	}
	modes, err := normalizeApplyModes(LayoutThin, LockModeIndependent)
	if err != nil {
		t.Fatal(err)
	}

	validated, err := prepareStagedApply(context.Background(), runner, sources, staging, opts, modes)
	if err != nil {
		t.Fatal(err)
	}
	defer validated.close()
	if validated.path == staging || filepath.Dir(validated.path) != "/nix/store" {
		t.Fatalf("validated staging = %q, want immutable direct /nix/store child", validated.path)
	}
	if runner.dryBuildFlake != validated.path {
		t.Fatalf("dry-build flake = %q, want exact publication candidate %q", runner.dryBuildFlake, validated.path)
	}
	for rel, raced := range map[string]string{
		"configuration.nix":    "# raced after validation\n",
		"user/default.nix":     "raced user template\n",
		"secrets/secrets.yaml": "raced secret\n",
	} {
		validatedBytes, readErr := os.ReadFile(filepath.Join(validated.path, rel))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(validatedBytes) == raced {
			t.Fatalf("immutable publication candidate accepted post-snapshot mutation for %s: %q", rel, validatedBytes)
		}
	}
	if _, err := os.Stat(filepath.Join(validated.path, paths.PasswordHashMarkerName)); err != nil {
		t.Fatalf("immutable publication candidate missing non-secret password marker: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(validated.path, "hashed-password.nix")); !os.IsNotExist(err) {
		t.Fatalf("immutable publication candidate contains a password hash: %v", err)
	}
	originalBytes, err := os.ReadFile(filepath.Join(staging, "configuration.nix"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(originalBytes), "# raced after validation\n"; got != want {
		t.Fatalf("race hook did not mutate original staging: got %q want %q", got, want)
	}
}

func TestPinnedStagingCreatorRejectsPreOpenReplacement(t *testing.T) {
	for _, level := range []string{"outer container", "staging tree"} {
		t.Run(level, func(t *testing.T) {
			rootPath := t.TempDir()
			root, err := os.Open(rootPath)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()

			parent := root
			displayParent := rootPath
			pattern := ".wahrwelt-workspace-"
			if level == "staging tree" {
				containerName, container, _, createErr := createPinnedStagingChild(root, rootPath, ".wahrwelt-workspace-")
				if createErr != nil {
					t.Fatal(createErr)
				}
				defer container.Close()
				parent = container
				displayParent = filepath.Join(rootPath, containerName)
				pattern = defaults.StagingTempPattern
			}

			var createdToken unix.Stat_t
			var retainedPath string
			var replacementPath string
			_, opened, _, err := createPinnedStagingChildWithBarrier(parent, displayParent, pattern, func(name string, created unix.Stat_t) error {
				createdToken = created
				pinnedParent := fileDescriptorPath(parent)
				publicPath := filepath.Join(pinnedParent, name)
				retainedPath = filepath.Join(pinnedParent, name+".created")
				replacementPath = publicPath
				if err := os.Rename(publicPath, retainedPath); err != nil {
					return err
				}
				if err := os.Mkdir(publicPath, 0o700); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(publicPath, "unknown.txt"), []byte("unknown replacement\n"), 0o600)
			})
			if opened != nil {
				opened.Close()
			}
			if err == nil || !strings.Contains(err.Error(), "changed before exact open") {
				t.Fatalf("pre-open replacement error = %v", err)
			}
			if got := readLegacyUserMigrationFile(t, filepath.Join(replacementPath, "unknown.txt")); got != "unknown replacement\n" {
				t.Fatalf("replacement bytes changed: %q", got)
			}
			var retained unix.Stat_t
			if err := unix.Fstatat(int(parent.Fd()), filepath.Base(retainedPath), &retained, unix.AT_SYMLINK_NOFOLLOW); err != nil {
				t.Fatal(err)
			}
			if retained.Dev != createdToken.Dev || retained.Ino != createdToken.Ino {
				t.Fatalf("retained created directory identity = %d:%d, want %d:%d", retained.Dev, retained.Ino, createdToken.Dev, createdToken.Ino)
			}
		})
	}
}

func TestCreateStagingWorkspaceScavengesOnlyExactEmptyHistoricalContainers(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", cache)
	base := filepath.Join(cache, "wahrwelt", "staging")
	if err := os.MkdirAll(base, 0o700); err != nil {
		t.Fatal(err)
	}
	empty := []string{
		".wahrwelt-workspace-123456789",
		".wahrwelt-workspace-1234567890",
		".wahrwelt-workspace-0123456789abcdef0123456789abcdef",
	}
	for _, name := range empty {
		if !isKnownStagingContainerName(name) {
			t.Fatalf("fixture is not recognized as a historical container: %s", name)
		}
		if err := os.Mkdir(filepath.Join(base, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	nonempty := filepath.Join(base, ".wahrwelt-workspace-fedcba9876543210fedcba9876543210")
	if err := os.Mkdir(nonempty, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nonempty, "recovery"), []byte("preserve\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	unknown := filepath.Join(base, ".wahrwelt-workspace-user-owned")
	if err := os.Mkdir(unknown, 0o700); err != nil {
		t.Fatal(err)
	}

	workspace, err := createStagingWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	defer workspace.close()
	if got := filepath.Dir(filepath.Dir(workspace.path)); got != base {
		t.Fatalf("workspace base = %s, want %s", got, base)
	}
	for _, name := range empty {
		if _, err := os.Lstat(filepath.Join(base, name)); !os.IsNotExist(err) {
			t.Fatalf("empty historical container remains: %s: %v", name, err)
		}
	}
	if got := readLegacyUserMigrationFile(t, filepath.Join(nonempty, "recovery")); got != "preserve\n" {
		t.Fatalf("nonempty recovery changed: %q", got)
	}
	if _, err := os.Stat(unknown); err != nil {
		t.Fatalf("unknown workspace-like directory changed: %v", err)
	}
}

func TestStagingCleanupPreservesPublicReplacementAndDisplacedTree(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", cache)
	workspace, err := createStagingWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	defer workspace.close()
	displaced := filepath.Join(filepath.Dir(workspace.path), ".displaced-"+workspace.name)
	if err := os.WriteFile(filepath.Join(workspace.path, "owned.txt"), []byte("owned staging\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(workspace.path, displaced); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(workspace.path, 0o700); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(workspace.path, "victim.txt")
	if err := os.WriteFile(victim, []byte("unknown victim\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nexec \"$@\"\n")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	err = workspace.cleanup(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "staging tree changed") {
		t.Fatalf("staging replacement cleanup error = %v", err)
	}
	if got := readLegacyUserMigrationFile(t, victim); got != "unknown victim\n" {
		t.Fatalf("public replacement changed: %q", got)
	}
	if got := readLegacyUserMigrationFile(t, filepath.Join(displaced, "owned.txt")); got != "owned staging\n" {
		t.Fatalf("displaced staging changed: %q", got)
	}
}

func TestStagingRuntimePathPinsExactTreeAcrossPublicReplacementForRealChild(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	workspace, err := createStagingWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	defer workspace.close()
	if err := os.WriteFile(filepath.Join(workspace.runtimePath, "before.txt"), []byte("pinned before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	displaced := filepath.Join(filepath.Dir(workspace.path), ".displaced-"+workspace.name)
	if err := os.Rename(workspace.path, displaced); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(workspace.path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace.path, "winner.txt"), []byte("public winner\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	command := exec.Command("sh", "-c", `set -eu; test "$(cat -- "$1/before.txt")" = "pinned before"; printf '%s\n' 'child write' >"$1/child.txt"`, "sh", workspace.runtimePath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("real child could not reopen pinned staging tree: %v: %s", err, output)
	}
	if got := readLegacyUserMigrationFile(t, filepath.Join(displaced, "child.txt")); got != "child write\n" {
		t.Fatalf("real child did not write displaced pinned tree: %q", got)
	}
	if got := readLegacyUserMigrationFile(t, filepath.Join(workspace.path, "winner.txt")); got != "public winner\n" {
		t.Fatalf("public replacement changed: %q", got)
	}
	if _, err := os.Lstat(filepath.Join(workspace.path, "child.txt")); !os.IsNotExist(err) {
		t.Fatalf("real child wrote through public replacement: %v", err)
	}
}

func TestValidatedStagingArchivesPinnedDirectoryInsteadOfProcDescriptorSymlink(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix is unavailable")
	}
	if _, err := exec.LookPath("nix-store"); err != nil {
		t.Skip("nix-store is unavailable")
	}
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	workspace, err := createStagingWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	defer workspace.close()
	if err := os.WriteFile(filepath.Join(workspace.runtimePath, "pinned.txt"), []byte("pinned tree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	displaced := filepath.Join(filepath.Dir(workspace.path), ".displaced-"+workspace.name)
	if err := os.Rename(workspace.path, displaced); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(workspace.path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace.path, "winner.txt"), []byte("public winner\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	validated, err := createValidatedStaging(
		context.Background(),
		run.Runner{Stdout: io.Discard, Stderr: io.Discard},
		workspace.runtimePath,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer validated.close()
	info, err := os.Lstat(validated.path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		t.Fatalf("validated store object mode = %v, want immutable directory", info.Mode())
	}
	if got := readLegacyUserMigrationFile(t, filepath.Join(validated.path, "pinned.txt")); got != "pinned tree\n" {
		t.Fatalf("validated pinned payload = %q", got)
	}
	entries, err := os.ReadDir(validated.path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "pinned.txt" {
		t.Fatalf("validated store tree entries = %v, want only pinned.txt", entries)
	}
	if _, err := os.Lstat(filepath.Join(validated.path, "winner.txt")); !os.IsNotExist(err) {
		t.Fatalf("public replacement entered validated store tree: %v", err)
	}
}

func TestRootOnlySecretsFallbackWritesPinnedDisplacedStagingTree(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission fallback requires an unprivileged installer process")
	}
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	workspace, err := createStagingWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	defer workspace.close()

	sourceRoot := t.TempDir()
	secretsDir := filepath.Join(sourceRoot, "secrets")
	if err := os.Mkdir(secretsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secretsDir, "secrets.yaml"), []byte(testEncryptedSecretsYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(secretsDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(secretsDir, 0o700) })

	displaced := filepath.Join(filepath.Dir(workspace.path), ".displaced-"+workspace.name)
	if err := os.Rename(workspace.path, displaced); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(workspace.path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace.path, "winner.txt"), []byte("public winner\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	runner := &rootOnlySecretsRunner{}
	if err := copyExistingThinSecrets(context.Background(), runner, sourceRoot, workspace.runtimePath); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 {
		t.Fatalf("privileged SOPS fallback calls = %d, want 1", runner.calls)
	}
	if got := readLegacyUserMigrationFile(t, filepath.Join(displaced, "secrets", "secrets.yaml")); got != testEncryptedSecretsYAML {
		t.Fatalf("pinned staged SOPS payload = %q", got)
	}
	if _, err := os.Lstat(filepath.Join(workspace.path, "secrets")); !os.IsNotExist(err) {
		t.Fatalf("privileged SOPS fallback wrote into public replacement: %v", err)
	}
	if err := workspace.verifyVisible(); err == nil || !strings.Contains(err.Error(), "staging tree changed") {
		t.Fatalf("public replacement was not blocked before publication: %v", err)
	}
}

func TestStagingCleanupFreezesAndRemovesExactTree(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	workspace, err := createStagingWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	defer workspace.close()
	nested := filepath.Join(workspace.path, "nested", "deeper")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "config.nix"), []byte("validated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("config.nix", filepath.Join(nested, "config-link")); err != nil {
		t.Fatal(err)
	}

	if err := workspace.cleanup(context.Background(), &userNamespaceCleanupRunner{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(workspace.path); !os.IsNotExist(err) {
		t.Fatalf("exact staging tree remained after cleanup: %v", err)
	}
	if _, err := os.Lstat(filepath.Dir(workspace.path)); !os.IsNotExist(err) {
		t.Fatalf("empty outer staging container remained after cleanup: %v", err)
	}
	parentInfo, err := workspace.parent.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if parentInfo.Mode().Perm() != os.FileMode(workspace.parentStat.Mode&0o777) {
		t.Fatalf("staging parent mode = %04o, want restored %04o", parentInfo.Mode().Perm(), workspace.parentStat.Mode&0o777)
	}
}

func TestStagingCleanupCrashCannotBlockNextWorkspace(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	first, err := createStagingWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	defer first.close()
	if err := os.WriteFile(filepath.Join(first.path, "retained.txt"), []byte("retained after crash\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err = first.cleanup(context.Background(), &crashingUserNamespaceCleanupRunner{})
	if err == nil {
		t.Fatal("injected cleanup crash was accepted as success")
	}
	parentPath := filepath.Dir(first.path)
	parentInfo, statErr := os.Stat(parentPath)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if parentInfo.Mode().Perm()&0o200 != 0 {
		t.Fatalf("crash hook did not strand the dedicated parent read-only: %04o", parentInfo.Mode().Perm())
	}
	if got := readLegacyUserMigrationFile(t, filepath.Join(first.path, "retained.txt")); got != "retained after crash\n" {
		t.Fatalf("crashed workspace bytes changed: %q", got)
	}

	second, err := createStagingWorkspace()
	if err != nil {
		t.Fatalf("next workspace was blocked by crashed sibling: %v", err)
	}
	defer second.close()
	if filepath.Dir(second.path) == parentPath {
		t.Fatal("workspaces unexpectedly share the crash-sensitive lock parent")
	}
	if err := os.WriteFile(filepath.Join(second.path, "next.txt"), []byte("next workspace\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := second.cleanup(context.Background(), &userNamespaceCleanupRunner{}); err != nil {
		t.Fatalf("next workspace cleanup failed after sibling crash: %v", err)
	}

	// User-namespace root maps back to the test UID, so restore this deliberate
	// crash artifact for testing.TempDir's own controlled cleanup.
	if err := os.Chmod(parentPath, 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestOuterWorkspaceReplacementRefusesValidationAndCleanup(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	workspace, err := createStagingWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	defer workspace.close()
	if err := os.WriteFile(filepath.Join(workspace.runtimePath, "owned.txt"), []byte("pinned workspace\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	containerPath := filepath.Dir(workspace.path)
	displaced := containerPath + ".displaced"
	if err := os.Rename(containerPath, displaced); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(containerPath, 0o700); err != nil {
		t.Fatal(err)
	}
	replacementTree := filepath.Join(containerPath, workspace.name)
	if err := os.Mkdir(replacementTree, 0o700); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(replacementTree, "victim.txt")
	if err := os.WriteFile(victim, []byte("unknown outer replacement\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	repo, dest := fakeRepo(t)
	sources, err := paths.ResolveSources(repo)
	if err != nil {
		t.Fatal(err)
	}
	modes, err := normalizeApplyModes(LayoutThin, LockModeIndependent)
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{dryRun: true}
	_, err = prepareStagedApply(context.Background(), runner, sources, workspace.runtimePath, Options{
		Paths: paths.Options{RepoRoot: repo, NixOSDest: dest},
		State: validState(),
	}, modes, workspace.verifyVisible)
	if err == nil || !strings.Contains(err.Error(), "staging outer container changed") {
		t.Fatalf("outer replacement validation error = %v", err)
	}
	if err := workspace.cleanup(context.Background(), runner); err == nil || !strings.Contains(err.Error(), "staging outer container changed") {
		t.Fatalf("outer replacement cleanup error = %v", err)
	}
	recoveryPath := workspace.ownedRecoveryPath()
	if !strings.HasPrefix(recoveryPath, displaced+string(filepath.Separator)) || recoveryPath == workspace.path {
		t.Fatalf("FD-resolved recovery path = %q, want displaced owned tree below %q and not replacement %q", recoveryPath, displaced, workspace.path)
	}
	reported := workspace.retainedCleanupError(errors.New("injected cleanup refusal")).Error()
	if !strings.Contains(reported, recoveryPath) || strings.Contains(reported, "retained at FD-resolved owned path "+workspace.path+" ") {
		t.Fatalf("cleanup report = %q, want owned recovery %q and not replacement %q", reported, recoveryPath, workspace.path)
	}
	if got := readLegacyUserMigrationFile(t, victim); got != "unknown outer replacement\n" {
		t.Fatalf("outer replacement bytes changed: %q", got)
	}
	entries, err := os.ReadDir(replacementTree)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "victim.txt" {
		t.Fatalf("generated staging entries escaped into public replacement: %v", entries)
	}
	if got := readLegacyUserMigrationFile(t, filepath.Join(displaced, workspace.name, "owned.txt")); got != "pinned workspace\n" {
		t.Fatalf("displaced pinned workspace changed: %q", got)
	}
}

func installLegacyRecoveryCreatorRace(t *testing.T, legacy, saved string) string {
	t.Helper()
	realMV, err := exec.LookPath("mv")
	if err != nil {
		t.Fatal(err)
	}
	realStat, err := exec.LookPath("stat")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "legacy-recovery-name")
	counter := filepath.Join(t.TempDir(), "legacy-mv-count")
	writeExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nexec \"$@\"\n")
	writeExecutable(t, filepath.Join(bin, "mv"), `#!/bin/sh
set -eu
count=0
if [ -f "$WAHRWELT_TEST_LEGACY_MV_COUNTER" ]; then count=$(cat "$WAHRWELT_TEST_LEGACY_MV_COUNTER"); fi
count=$((count + 1))
printf '%s\n' "$count" > "$WAHRWELT_TEST_LEGACY_MV_COUNTER"
"$WAHRWELT_TEST_REAL_MV" "$@"
if [ "$count" = 1 ]; then
  mkdir -- "$WAHRWELT_TEST_LEGACY_SOURCE"
  printf '%s\n' 'concurrent private owner' > "$WAHRWELT_TEST_LEGACY_SOURCE/winner.nix"
fi
`)
	writeExecutable(t, filepath.Join(bin, "stat"), `#!/bin/sh
set -eu
last=
for arg do last=$arg; done
parent=${last%/*}
name=${last##*/}
case "$name" in
  .wahrwelt-installer-recovery-user-*)
    if [ ! -e "$WAHRWELT_TEST_LEGACY_RECOVERY_MARKER" ]; then
      printf '%s\n' "$name" > "$WAHRWELT_TEST_LEGACY_RECOVERY_MARKER"
      "$WAHRWELT_TEST_REAL_MV" -- "$last" "$WAHRWELT_TEST_LEGACY_RECOVERY_SAVED"
      mkdir -- "$last"
      printf '%s\n' 'unknown recovery owner' > "$last/unknown.txt"
    fi
    ;;
esac
exec "$WAHRWELT_TEST_REAL_STAT" "$@"
`)
	t.Setenv("WAHRWELT_TEST_REAL_MV", realMV)
	t.Setenv("WAHRWELT_TEST_REAL_STAT", realStat)
	t.Setenv("WAHRWELT_TEST_LEGACY_MV_COUNTER", counter)
	t.Setenv("WAHRWELT_TEST_LEGACY_SOURCE", legacy)
	t.Setenv("WAHRWELT_TEST_LEGACY_RECOVERY_MARKER", marker)
	t.Setenv("WAHRWELT_TEST_LEGACY_RECOVERY_SAVED", saved)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	return marker
}

func TestLegacyUserRecoveryRejectsCreatorReplacementBeforeOpen(t *testing.T) {
	dest := t.TempDir()
	legacy := filepath.Join(dest, "private")
	user := filepath.Join(dest, "user")
	savedRecovery := filepath.Join(dest, "created-recovery")
	if err := os.Mkdir(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "custom.nix"), []byte("expected legacy data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := installLegacyRecoveryCreatorRace(t, legacy, savedRecovery)
	var stderr strings.Builder

	err := migrateLegacyUserDirectory(context.Background(), run.Runner{Stdout: io.Discard, Stderr: &stderr}, dest)
	if err == nil {
		t.Fatal("legacy recovery replacement before open was accepted")
	}
	markerBytes, readErr := os.ReadFile(marker)
	if readErr != nil {
		t.Fatalf("legacy recovery creator barrier was not reached: %v; helper stderr: %s; error: %v", readErr, stderr.String(), err)
	}
	recoveryName := strings.TrimSpace(string(markerBytes))
	unknown, readErr := os.ReadFile(filepath.Join(dest, recoveryName, "unknown.txt"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got, want := string(unknown), "unknown recovery owner\n"; got != want {
		t.Fatalf("unknown recovery bytes = %q, want %q", got, want)
	}
	entries, readErr := os.ReadDir(savedRecovery)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("created recovery was mutated before exact reopen: %v", entries)
	}
	if got := readLegacyUserMigrationFile(t, filepath.Join(user, "custom.nix")); got != "expected legacy data\n" {
		t.Fatalf("expected migrated target changed: %q", got)
	}
	if got := readLegacyUserMigrationFile(t, filepath.Join(legacy, "winner.nix")); got != "concurrent private owner\n" {
		t.Fatalf("concurrent source owner changed: %q", got)
	}
}

func writePublicationSource(t *testing.T, staging, rel, content string) string {
	t.Helper()
	path := filepath.Join(staging, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWriteStagedSecretsPreservesConcurrentDirectoryWinner(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	writePublicationSource(t, staging, "secrets/secrets.yaml", "secret: staged\n")
	target := filepath.Join(dest, "secrets")
	unknown := filepath.Join(target, "unknown.txt")
	var expected os.FileInfo
	runner := &publicationRaceRunner{before: func() error {
		if err := os.Mkdir(target, 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(unknown, []byte("concurrent owner\n"), 0o600); err != nil {
			return err
		}
		var err error
		expected, err = os.Lstat(target)
		return err
	}}

	err := writeStagedSecrets(context.Background(), runner, staging, dest, LayoutThin)
	if err == nil {
		t.Fatal("concurrent secrets directory winner was accepted")
	}
	actual, statErr := os.Lstat(target)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if expected == nil || !os.SameFile(expected, actual) {
		t.Fatal("concurrent secrets directory inode was replaced")
	}
	data, readErr := os.ReadFile(unknown)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got, want := string(data), "concurrent owner\n"; got != want {
		t.Fatalf("concurrent secrets bytes = %q, want %q", got, want)
	}
	if _, statErr := os.Lstat(filepath.Join(target, "secrets.yaml")); !os.IsNotExist(statErr) {
		t.Fatalf("staged secrets leaked into concurrent directory: %v", statErr)
	}
}

func TestWriteStagedSecretsCopiesPinnedSourceDirectory(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	source := filepath.Join(staging, "secrets")
	writePublicationSource(t, staging, "secrets/secrets.yaml", "secret: expected\n")
	retained := filepath.Join(staging, "secrets-retained")
	runner := &publicationRaceRunner{before: func() error {
		if err := os.Rename(source, retained); err != nil {
			return err
		}
		if err := os.Mkdir(source, 0o700); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(source, "secrets.yaml"), []byte("secret: replacement\n"), 0o600)
	}}

	if err := writeStagedSecrets(context.Background(), runner, staging, dest, LayoutThin); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "secrets", "secrets.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "secret: expected\n"; got != want {
		t.Fatalf("published secrets = %q, want pinned source %q", got, want)
	}
	replacement, err := os.ReadFile(filepath.Join(source, "secrets.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(replacement), "secret: replacement\n"; got != want {
		t.Fatalf("replacement source changed to %q", got)
	}
}

func TestWriteStagedHashedPasswordPreservesConcurrentRegularWinner(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	writePasswordHashMarker(t, staging)
	if err := os.WriteFile(filepath.Join(dest, "hashed-password.nix"), []byte(legacyGeneratedPasswordModule(testSHA512Hash)), 0o600); err != nil {
		t.Fatal(err)
	}
	target := passwordHashTarget(dest)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	var expected os.FileInfo
	runner := &publicationRaceRunner{before: func() error {
		if err := os.WriteFile(target, []byte("concurrent owner\n"), 0o600); err != nil {
			return err
		}
		var err error
		expected, err = os.Lstat(target)
		return err
	}}

	err := writeStagedHashedPassword(context.Background(), runner, staging, dest, config.Secrets{}, LayoutThin)
	if err == nil {
		t.Fatal("concurrent hashed-password winner was accepted")
	}
	actual, statErr := os.Lstat(target)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if expected == nil || !os.SameFile(expected, actual) {
		t.Fatal("concurrent hashed-password inode was replaced")
	}
	data, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got, want := string(data), "concurrent owner\n"; got != want {
		t.Fatalf("concurrent hashed-password bytes = %q, want %q", got, want)
	}
}

func TestWriteStagedHashedPasswordPreservesReplacementOfPinnedTarget(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	writePasswordHashMarker(t, staging)
	target := passwordHashTarget(dest)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(testSHA512Hash+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	retained := filepath.Join(filepath.Dir(target), "hashed-password.expected")
	var replacement os.FileInfo
	runner := &publicationRaceRunner{before: func() error {
		if err := os.Rename(target, retained); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte("replacement owner\n"), 0o600); err != nil {
			return err
		}
		var err error
		replacement, err = os.Lstat(target)
		return err
	}}
	bin := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "mkpasswd"), "#!/bin/sh\nprintf '%s\\n' '"+strings.Replace(testSHA512Hash, "A", "B", 1)+"'\n")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	err := writeStagedHashedPassword(context.Background(), runner, staging, dest, config.Secrets{UserPassword: "replace"}, LayoutThin)
	if err == nil {
		t.Fatal("replacement of pinned hashed-password target was accepted")
	}
	actual, statErr := os.Lstat(target)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if replacement == nil || !os.SameFile(replacement, actual) {
		t.Fatal("replacement hashed-password inode was overwritten")
	}
	data, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got, want := string(data), "replacement owner\n"; got != want {
		t.Fatalf("replacement hashed-password bytes = %q, want %q", got, want)
	}
	if _, statErr := os.Lstat(retained); statErr != nil {
		t.Fatalf("pinned expected target was not retained: %v", statErr)
	}
}

func TestWriteStagedHashedPasswordSnapshotsLegacySourceBeforePublication(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	writePasswordHashMarker(t, staging)
	source := writePublicationSource(t, dest, "hashed-password.nix", legacyGeneratedPasswordModule(testSHA512Hash))
	retained := filepath.Join(dest, "hashed-password.expected")
	target := passwordHashTarget(dest)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &publicationRaceRunner{before: func() error {
		if err := os.Rename(source, retained); err != nil {
			return err
		}
		return os.WriteFile(source, []byte("foreign replacement\n"), 0o600)
	}}

	err := writeStagedHashedPassword(context.Background(), runner, staging, dest, config.Secrets{}, LayoutThin)
	if err == nil || !strings.Contains(err.Error(), "ownership collision") {
		t.Fatalf("legacy source replacement must fail closed after publishing the external hash, got %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), testSHA512Hash+"\n"; got != want {
		t.Fatalf("published hash differs from pinned legacy source")
	}
	replacement, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(replacement), "foreign replacement\n"; got != want {
		t.Fatalf("legacy replacement changed: got %q want %q", got, want)
	}
	retainedData, err := os.ReadFile(retained)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(retainedData), functionalLegacyPasswordModule("wahrwelt"); got != want || strings.Contains(got, testSHA512Hash) {
		t.Fatalf("pinned legacy source was not sanitized after replacement: got %q want %q", got, want)
	}
}

func TestWriteStagedUserDefaultSnapshotsPinnedSourceBytes(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	if err := os.Mkdir(filepath.Join(dest, "user"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := writePublicationSource(t, staging, "user/default.nix", "expected template\n")
	runner := &publicationRaceRunner{before: func() error {
		return os.WriteFile(source, []byte("mutated template\n"), 0o644)
	}}

	if err := writeStagedUserDefault(context.Background(), runner, staging, dest, LayoutThin); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "user", "default.nix"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "expected template\n"; got != want {
		t.Fatalf("published user/default.nix = %q, want immutable snapshot %q", got, want)
	}
}

func installDirectoryCreatorRace(t *testing.T, parent, saved string) string {
	t.Helper()
	realStat, err := exec.LookPath("stat")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "candidate-name")
	writeExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nexec \"$@\"\n")
	writeExecutable(t, filepath.Join(bin, "stat"), `#!/bin/sh
set -eu
last=
for arg do last=$arg; done
case "$last" in
  */.wahrwelt-*)
    if [ ! -e "$WAHRWELT_TEST_CANDIDATE_MARKER" ]; then
      name=${last##*/}
      printf '%s\n' "$name" > "$WAHRWELT_TEST_CANDIDATE_MARKER"
      mv -- "$last" "$WAHRWELT_TEST_CANDIDATE_SAVED"
      mkdir -- "$last"
      printf '%s\n' 'unknown candidate owner' > "$last/unknown.txt"
    fi
    ;;
esac
exec "$WAHRWELT_TEST_REAL_STAT" "$@"
`)
	t.Setenv("WAHRWELT_TEST_REAL_STAT", realStat)
	t.Setenv("WAHRWELT_TEST_CANDIDATE_PARENT", parent)
	t.Setenv("WAHRWELT_TEST_CANDIDATE_SAVED", saved)
	t.Setenv("WAHRWELT_TEST_CANDIDATE_MARKER", marker)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	return marker
}

func TestPrivilegedPythonPathRejectsUntrustedExecutable(t *testing.T) {
	python := filepath.Join(t.TempDir(), "python3")
	writeExecutable(t, python, "#!/bin/sh\nexit 0\n")
	t.Setenv("WAHRWELT_PRIVILEGED_PYTHON", python)
	err := writeState(context.Background(), &fakeRunner{}, filepath.Join(t.TempDir(), "installer-state.json"), config.Default())
	if err == nil || !strings.Contains(err.Error(), "untrusted privileged python") {
		t.Fatalf("untrusted privileged Python error = %v", err)
	}
}

func TestPrivilegedPythonPathRejectsUserWritableSymlinkToTrustedExecutable(t *testing.T) {
	trusted, err := privilegedPythonPath()
	if err != nil {
		t.Fatal(err)
	}
	python := filepath.Join(t.TempDir(), "python3")
	if err := os.Symlink(trusted, python); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WAHRWELT_PRIVILEGED_PYTHON", python)
	_, err = privilegedPythonPath()
	if err == nil || !strings.Contains(err.Error(), "outside trusted system executable locations") {
		t.Fatalf("user-writable symlink to trusted Python error = %v", err)
	}
}

func TestPrivilegedFSHelperPathRejectsUntrustedExecutable(t *testing.T) {
	helper := filepath.Join(t.TempDir(), "wahrwelt-fs-helper")
	writeExecutable(t, helper, "#!/bin/sh\nexit 0\n")
	t.Setenv("WAHRWELT_PRIVILEGED_FS_HELPER", helper)
	_, err := privilegedFSHelperPath()
	if err == nil || !strings.Contains(err.Error(), "untrusted privileged filesystem helper") {
		t.Fatalf("untrusted privileged filesystem helper error = %v", err)
	}
}

func TestValidatedStagingRejectsUntrustedNixExecutable(t *testing.T) {
	nix := filepath.Join(t.TempDir(), "nix")
	writeExecutable(t, nix, "#!/bin/sh\nprintf '/nix/store/attacker-controlled\\n'\n")
	t.Setenv("WAHRWELT_VALIDATION_NIX", nix)
	_, err := createValidatedStaging(context.Background(), &publicationRaceRunner{}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "untrusted nix for staging validation") {
		t.Fatalf("untrusted staging Nix error = %v", err)
	}
}

func assertDirectoryCreatorRacePreserved(t *testing.T, parent, saved, marker string) {
	t.Helper()
	nameData, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("directory creator barrier was not reached: %v", err)
	}
	name := strings.TrimSpace(string(nameData))
	data, err := os.ReadFile(filepath.Join(parent, name, "unknown.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "unknown candidate owner\n"; got != want {
		t.Fatalf("unknown candidate bytes = %q, want %q", got, want)
	}
	entries, err := os.ReadDir(saved)
	if err != nil {
		t.Fatalf("created candidate recovery was not retained: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("created candidate was mutated before identity verification: %v", entries)
	}
}

func TestPrivilegedStatePublishRejectsQuarantineReplacementBeforeOpen(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "installer-state.json")
	saved := filepath.Join(parent, "created-quarantine")
	marker := installDirectoryCreatorRace(t, parent, saved)
	err := writeState(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, target, config.Default())
	if err == nil {
		t.Fatal("quarantine replacement before open was accepted")
	}
	if _, statErr := os.Lstat(target); !os.IsNotExist(statErr) {
		t.Fatalf("state target was published after quarantine replacement: %v", statErr)
	}
	assertDirectoryCreatorRacePreserved(t, parent, saved, marker)
}

func TestPrivilegedUserDefaultCreateRejectsCandidateReplacementBeforeOpen(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	writePublicationSource(t, staging, "user/default.nix", "generated\n")
	saved := filepath.Join(dest, "created-user-candidate")
	marker := installDirectoryCreatorRace(t, dest, saved)
	err := writeStagedUserDefault(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, staging, dest, LayoutThin)
	if err == nil {
		t.Fatal("user/default.nix candidate replacement before open was accepted")
	}
	if _, statErr := os.Lstat(filepath.Join(dest, "user")); !os.IsNotExist(statErr) {
		t.Fatalf("user directory was published after candidate replacement: %v", statErr)
	}
	assertDirectoryCreatorRacePreserved(t, dest, saved, marker)
}

func installDirectoryMoveRace(t *testing.T, _ string, prefix, saved string) string {
	t.Helper()
	realMV, err := exec.LookPath("mv")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "directory-mv-called")
	writeExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nexec \"$@\"\n")
	writeExecutable(t, filepath.Join(bin, "mv"), `#!/bin/sh
set -eu
previous=
last=
for arg do previous=$last; last=$arg; done
source=$previous
name=${source##*/}
case "$name" in
  "$WAHRWELT_TEST_DIRECTORY_PREFIX"*)
    if [ ! -e "$WAHRWELT_TEST_DIRECTORY_MV_MARKER" ]; then
      printf '%s\n' "$name" > "$WAHRWELT_TEST_DIRECTORY_MV_MARKER"
      "$WAHRWELT_TEST_REAL_MV" -- "$source" "$WAHRWELT_TEST_DIRECTORY_SAVED"
      mkdir -- "$source"
      printf '%s\n' 'unknown directory owner' > "$source/unknown.txt"
    fi
    ;;
esac
exec "$WAHRWELT_TEST_REAL_MV" "$@"
`)
	t.Setenv("WAHRWELT_TEST_REAL_MV", realMV)
	t.Setenv("WAHRWELT_TEST_DIRECTORY_PREFIX", prefix)
	t.Setenv("WAHRWELT_TEST_DIRECTORY_SAVED", saved)
	t.Setenv("WAHRWELT_TEST_DIRECTORY_MV_MARKER", marker)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	return marker
}

func assertDirectoryMoveRaceRestored(t *testing.T, parent, saved, marker string) {
	t.Helper()
	nameData, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("directory move barrier was not reached: %v", err)
	}
	name := strings.TrimSpace(string(nameData))
	data, err := os.ReadFile(filepath.Join(parent, name, "unknown.txt"))
	if err != nil {
		t.Fatalf("unknown directory was not restored to its original name: %v", err)
	}
	if got, want := string(data), "unknown directory owner\n"; got != want {
		t.Fatalf("restored unknown directory bytes = %q, want %q", got, want)
	}
	if _, err := os.Stat(saved); err != nil {
		t.Fatalf("exact created directory recovery was lost: %v", err)
	}
}

func TestSecretsPublicationRestoresCandidateReplacementAfterMoveRace(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	writePublicationSource(t, staging, "secrets/secrets.yaml", "secret: expected\n")
	saved := filepath.Join(dest, "expected-secrets-candidate")
	marker := installDirectoryMoveRace(t, dest, ".wahrwelt-secrets-recovery-", saved)
	err := writeStagedSecrets(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, staging, dest, LayoutThin)
	if err == nil {
		t.Fatal("secrets candidate replacement during move was accepted")
	}
	if _, statErr := os.Lstat(filepath.Join(dest, "secrets")); !os.IsNotExist(statErr) {
		t.Fatalf("unknown secrets candidate remained at public target: %v", statErr)
	}
	assertDirectoryMoveRaceRestored(t, dest, saved, marker)
	data, readErr := os.ReadFile(filepath.Join(saved, "secrets.yaml"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got, want := string(data), "secret: expected\n"; got != want {
		t.Fatalf("created candidate recovery = %q, want %q", got, want)
	}
}

func TestUserDefaultCreateRestoresCandidateReplacementAfterMoveRace(t *testing.T) {
	staging := t.TempDir()
	dest := t.TempDir()
	writePublicationSource(t, staging, "user/default.nix", "generated\n")
	saved := filepath.Join(dest, "expected-user-candidate")
	marker := installDirectoryMoveRace(t, dest, ".wahrwelt-user-default-recovery-", saved)
	err := writeStagedUserDefault(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, staging, dest, LayoutThin)
	if err == nil {
		t.Fatal("user directory candidate replacement during move was accepted")
	}
	if _, statErr := os.Lstat(filepath.Join(dest, "user")); !os.IsNotExist(statErr) {
		t.Fatalf("unknown user candidate remained at public target: %v", statErr)
	}
	assertDirectoryMoveRaceRestored(t, dest, saved, marker)
	data, readErr := os.ReadFile(filepath.Join(saved, "default.nix"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got, want := string(data), "generated\n"; got != want {
		t.Fatalf("created user candidate recovery = %q, want %q", got, want)
	}
}

func TestCreatePinnedStateSourceIsSealedAndHasNoCleanupPath(t *testing.T) {
	state := config.Default()
	state.Host.Hostname = "sealed-source"
	source, expected, cleanup, err := createPinnedStateSource(state)
	if err != nil {
		t.Fatal(err)
	}
	cleaned := false
	t.Cleanup(func() {
		if !cleaned {
			cleanup()
		}
	})
	if !bytes.Contains(expected, []byte(`"hostname": "sealed-source"`)) {
		t.Fatalf("serialized state does not contain expected hostname:\n%s", expected)
	}
	if _, err := unix.Write(int(source.file.Fd()), []byte("mutated")); err == nil {
		t.Fatal("pinned state source remained writable")
	}
	link, err := os.Readlink(fileDescriptorPath(source.file))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(link, " (deleted)") || strings.HasPrefix(link, "/memfd:") {
		cleanup()
		cleaned = true
		return
	}

	retained := link + ".retained"
	if err := os.Rename(link, retained); err != nil {
		t.Fatalf("state source remained named but could not be raced: %v", err)
	}
	if err := os.WriteFile(link, []byte("unknown cleanup replacement\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cleanup()
	cleaned = true
	data, readErr := os.ReadFile(link)
	if readErr != nil {
		t.Fatalf("state source cleanup deleted a replacement: %v", readErr)
	}
	if got, want := string(data), "unknown cleanup replacement\n"; got != want {
		t.Fatalf("cleanup replacement bytes = %q, want %q", got, want)
	}
	t.Fatalf("state source remained publicly named at %s", link)
}

func installCleanupCommandBarriers(t *testing.T) (rmMarker, rmdirMarker string) {
	t.Helper()
	realRM, err := exec.LookPath("rm")
	if err != nil {
		t.Fatal(err)
	}
	realRmdir, err := exec.LookPath("rmdir")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	rmMarker = filepath.Join(t.TempDir(), "rm-called")
	rmdirMarker = filepath.Join(t.TempDir(), "rmdir-called")
	writeExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nexec \"$@\"\n")
	writeExecutable(t, filepath.Join(bin, "rm"), `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$WAHRWELT_TEST_RM_MARKER"
last=
for arg do last=$arg; done
if [ "$last" = payload ] && [ -e payload ]; then
  mv -- payload payload.expected
  printf '%s\n' 'unknown cleanup replacement' > payload
fi
exec "$WAHRWELT_TEST_REAL_RM" "$@"
`)
	writeExecutable(t, filepath.Join(bin, "rmdir"), `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$WAHRWELT_TEST_RMDIR_MARKER"
exec "$WAHRWELT_TEST_REAL_RMDIR" "$@"
`)
	t.Setenv("WAHRWELT_TEST_REAL_RM", realRM)
	t.Setenv("WAHRWELT_TEST_REAL_RMDIR", realRmdir)
	t.Setenv("WAHRWELT_TEST_RM_MARKER", rmMarker)
	t.Setenv("WAHRWELT_TEST_RMDIR_MARKER", rmdirMarker)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	return rmMarker, rmdirMarker
}

func assertCleanupCommandsNotUsed(t *testing.T, rmMarker, rmdirMarker string) {
	t.Helper()
	if data, err := os.ReadFile(rmMarker); err == nil {
		t.Fatalf("pathname rm used in actor-writable namespace: %s", data)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(rmdirMarker); err == nil {
		t.Fatalf("pathname rmdir used in actor-writable namespace: %s", data)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestPrivilegedPublishRetainsCleanupPayloadInWritableParent(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "installer-state.json")
	old := config.Default()
	old.Host.Hostname = "old-state"
	if err := config.Save(target, old); err != nil {
		t.Fatal(err)
	}
	rmMarker, rmdirMarker := installCleanupCommandBarriers(t)
	next := config.Default()
	next.Host.Hostname = "new-state"
	if err := writeState(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, target, next); err != nil {
		t.Fatal(err)
	}
	assertCleanupCommandsNotUsed(t, rmMarker, rmdirMarker)
	payload := recoveryPayload(t, parent)
	recovered, err := config.LoadExisting(payload)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := recovered.Host.Hostname, "old-state"; got != want {
		t.Fatalf("retained displaced state = %q, want %q", got, want)
	}
}

func installPayloadMoveRace(t *testing.T, exchange bool) string {
	t.Helper()
	realMV, err := exec.LookPath("mv")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "mv-called")
	writeExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nexec \"$@\"\n")
	writeExecutable(t, filepath.Join(bin, "mv"), `#!/bin/sh
set -eu
should_race=0
for arg do
  if [ "$arg" = --exchange ]; then should_race=1; fi
done
if [ "$WAHRWELT_TEST_EXCHANGE_RACE" = 0 ]; then should_race=1; fi
if [ "$should_race" = 1 ] && [ ! -e "$WAHRWELT_TEST_MV_MARKER" ] && [ -e payload ]; then
  printf '%s\n' raced > "$WAHRWELT_TEST_MV_MARKER"
  "$WAHRWELT_TEST_REAL_MV" -- payload payload.expected
  printf '%s\n' 'unknown payload owner' > payload
fi
exec "$WAHRWELT_TEST_REAL_MV" "$@"
`)
	t.Setenv("WAHRWELT_TEST_REAL_MV", realMV)
	t.Setenv("WAHRWELT_TEST_MV_MARKER", marker)
	if exchange {
		t.Setenv("WAHRWELT_TEST_EXCHANGE_RACE", "1")
	} else {
		t.Setenv("WAHRWELT_TEST_EXCHANGE_RACE", "0")
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	return marker
}

func TestPrivilegedFreshPublishLinksExactPinnedPayload(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "installer-state.json")
	marker := installPayloadMoveRace(t, false)
	state := config.Default()
	state.Host.Hostname = "exact-payload"
	if err := writeState(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, target, state); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(marker); !os.IsNotExist(err) {
		t.Fatalf("fresh publication used mutable payload pathname: %v", err)
	}
	published, err := config.LoadExisting(target)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := published.Host.Hostname, "exact-payload"; got != want {
		t.Fatalf("published state = %q, want %q", got, want)
	}
}

func TestPrivilegedExchangeRestoresExactTargetAfterPayloadReplacement(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "installer-state.json")
	old := config.Default()
	old.Host.Hostname = "old-state"
	if err := config.Save(target, old); err != nil {
		t.Fatal(err)
	}
	marker := installPayloadMoveRace(t, true)
	next := config.Default()
	next.Host.Hostname = "new-state"
	err := writeState(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, target, next)
	if err == nil {
		t.Fatal("payload replacement during exchange was accepted")
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Fatalf("exchange barrier was not reached: %v", statErr)
	}
	current, loadErr := config.LoadExisting(target)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if got, want := current.Host.Hostname, "old-state"; got != want {
		t.Fatalf("restored state = %q, want %q", got, want)
	}
	payload := recoveryPayload(t, parent)
	data, readErr := os.ReadFile(payload)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got, want := string(data), "unknown payload owner\n"; got != want {
		t.Fatalf("retained unknown payload = %q, want %q", got, want)
	}
}

func TestLegacyStateCleanupRetainsPayloadInWritableParent(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "wahrwelt")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(parent, "state.json")
	current := config.Default()
	if err := config.Save(statePath, current); err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	rmMarker, rmdirMarker := installCleanupCommandBarriers(t)
	if err := cleanupLegacyStatePaths(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, []string{statePath}, current); err != nil {
		t.Fatal(err)
	}
	assertCleanupCommandsNotUsed(t, rmMarker, rmdirMarker)
	if _, err := os.Lstat(statePath); !os.IsNotExist(err) {
		t.Fatalf("legacy public state still exists: %v", err)
	}
	payload := recoveryPayload(t, parent)
	data, err := os.ReadFile(payload)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), string(expected); got != want {
		t.Fatalf("retained legacy state = %q, want %q", got, want)
	}
}

func TestLegacyStateCleanupDoesNotRestoreReplacementAsExpectedState(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "wahrwelt")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(parent, "state.json")
	current := config.Default()
	if err := config.Save(statePath, current); err != nil {
		t.Fatal(err)
	}
	expectedData, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	realMV, err := exec.LookPath("mv")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "legacy-payload-raced")
	saved := filepath.Join(root, "expected-state-recovery")
	writeExecutable(t, filepath.Join(bin, "sudo"), "#!/bin/sh\nexec \"$@\"\n")
	writeExecutable(t, filepath.Join(bin, "mv"), `#!/bin/sh
set -eu
last=
for arg do last=$arg; done
if [ "$last" = payload ] && [ ! -e "$WAHRWELT_TEST_LEGACY_MV_MARKER" ]; then
  "$WAHRWELT_TEST_REAL_MV" "$@"
  "$WAHRWELT_TEST_REAL_MV" -- payload "$WAHRWELT_TEST_LEGACY_EXPECTED_SAVED"
  printf '%s\n' 'unknown legacy payload' > payload
  printf '%s\n' raced > "$WAHRWELT_TEST_LEGACY_MV_MARKER"
  exit 0
fi
exec "$WAHRWELT_TEST_REAL_MV" "$@"
`)
	t.Setenv("WAHRWELT_TEST_REAL_MV", realMV)
	t.Setenv("WAHRWELT_TEST_LEGACY_MV_MARKER", marker)
	t.Setenv("WAHRWELT_TEST_LEGACY_EXPECTED_SAVED", saved)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	err = cleanupLegacyStatePaths(context.Background(), run.Runner{Stdout: io.Discard, Stderr: io.Discard}, []string{statePath}, current)
	if err == nil {
		t.Fatal("legacy payload replacement was accepted")
	}
	if _, statErr := os.Lstat(statePath); !os.IsNotExist(statErr) {
		t.Fatalf("unknown payload was restored as legacy state: %v", statErr)
	}
	data, readErr := os.ReadFile(recoveryPayload(t, parent))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got, want := string(data), "unknown legacy payload\n"; got != want {
		t.Fatalf("retained unknown legacy payload = %q, want %q", got, want)
	}
	expected, readErr := os.ReadFile(saved)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if got, want := string(expected), string(expectedData); got != want {
		t.Fatalf("exact expected legacy recovery = %q, want %q", got, want)
	}
}

func TestPackagedInstallerIncludesDirectoryCreatorRuntime(t *testing.T) {
	data, err := os.ReadFile("../../../NixOS/lib/flake-packages.nix")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "              python3\n") {
		t.Fatalf("packaged installer PATH must include python3 for pinned directory creation:\n%s", text)
	}
	if !strings.Contains(text, `--set WAHRWELT_PRIVILEGED_PYTHON ${flakePkgs.python3}/bin/python3`) {
		t.Fatalf("packaged installer must pass the exact Python store path across sudo:\n%s", text)
	}
}
