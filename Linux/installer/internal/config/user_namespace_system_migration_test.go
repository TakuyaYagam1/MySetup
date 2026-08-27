package config

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

const (
	systemUserNamespaceHelper  = "../../../NixOS/system/migrations/v1_to_v2/user-namespace.py"
	systemBrandMigrationHelper = "../../../NixOS/system/migrations/v1_to_v2/brand.py"
)

func TestSystemUserNamespaceMigrationRewritesOnlyNixPathTokens(t *testing.T) {
	root := t.TempDir()
	mustWriteSystemMigrationFixture(t, filepath.Join(root, "private", "custom.nix"), "custom\n")
	mustWriteSystemMigrationFixture(t, filepath.Join(root, "private", "nested.nix"), "imports = [ ./private ];\n")
	statePayload := systemMigrationStatePayload(7)
	mustWriteSystemMigrationFixture(t, filepath.Join(root, "wahrwelt", "state.json"), statePayload)
	mustWriteSystemMigrationFixture(t, filepath.Join(root, "wahrwelt", "keep.json"), "keep\n")

	input := "imports = [ ./private ./private/custom.nix ./private/nested/module.nix ./private-network.nix ];\n" +
		"operators = [ {}//./private 2*./private 2<./private true&&./private false||./private 2==./private 2!=./private ];\n" +
		"lambdas = [ ({}:./private) ({ x }:./private) (foo_https:./private) (foo'bar:./private) ];\n" +
		"trailingOperators = [ (./private?x) (./private*2) (./private<2) (./private>2) (./private==./other) (./private!=./other) (./private&&true) (./private||false) ];\n" +
		"uri = value:./private;\n" +
		"schemeUris = [ https://./private file://./private ];\n" +
		"colonUris = [ scheme:a:./private http://host:8080/./private ];\n" +
		"nestedUri = ssh://host//./private;\n" +
		"queryUri = https://host/path?x=./private;\n" +
		"tripleSlash = defaults///./private;\n" +
		"pathSuffixes = [ ./private+foo ./private.foo ];\n" +
		"searchPaths = [ <./private> <foo/./private> ];\n" +
		"pathTokens = [ x+./private x-./private x/./private ];\n" +
		"absolute = /tmp/./private;\n" +
		"interpolatedPath = ./base/${foo}./private;\n" +
		"interpolatedHomePath = ~/${foo}./private;\n" +
		"interpolatedPathAdjacent = ./base${foo}./private;\n" +
		"interpolatedHomePathAdjacent = ~/base${foo}./private;\n" +
		"interpolatedPathPreSeparators = ./base//${foo}////./private;\n" +
		"interpolatedHomePathPreSeparators = ~/base//${foo}////./private;\n" +
		"interpolatedPathTriple = ./base/${foo}///./private;\n" +
		"interpolatedHomePathTriple = ~/${foo}///./private;\n" +
		"interpolatedPathExpression = ./base/${./private}/asset;\n" +
		"quoted = \"./private ./private/custom.nix\";\n" +
		"interpolated = \"${./private}/asset\";\n" +
		"indented = '' ./private ./private/custom.nix '';\n" +
		"indentedInterpolated = '' ${./private/custom.nix} '';\n" +
		"# ./private ./private/custom.nix\n" +
		"/* ./private ./private/custom.nix */\n"
	mustWriteSystemMigrationFixture(t, filepath.Join(root, "configuration.nix"), input)

	runSystemUserNamespaceHelper(t, 0, "migrate-stage", root)
	runSystemUserNamespaceHelper(t, 0, "validate-migrated", root)

	if got := mustReadSystemMigrationFixture(t, filepath.Join(root, "configuration.nix")); got !=
		"imports = [ ./user ./user/custom.nix ./user/nested/module.nix ./private-network.nix ];\n"+
			"operators = [ {}//./user 2*./user 2<./user true&&./user false||./user 2==./user 2!=./user ];\n"+
			"lambdas = [ ({}:./user) ({ x }:./user) (foo_https:./user) (foo'bar:./user) ];\n"+
			"trailingOperators = [ (./user?x) (./user*2) (./user<2) (./user>2) (./user==./other) (./user!=./other) (./user&&true) (./user||false) ];\n"+
			"uri = value:./private;\n"+
			"schemeUris = [ https://./private file://./private ];\n"+
			"colonUris = [ scheme:a:./private http://host:8080/./private ];\n"+
			"nestedUri = ssh://host//./private;\n"+
			"queryUri = https://host/path?x=./private;\n"+
			"tripleSlash = defaults///./private;\n"+
			"pathSuffixes = [ ./private+foo ./private.foo ];\n"+
			"searchPaths = [ <./private> <foo/./private> ];\n"+
			"pathTokens = [ x+./private x-./private x/./private ];\n"+
			"absolute = /tmp/./private;\n"+
			"interpolatedPath = ./base/${foo}./private;\n"+
			"interpolatedHomePath = ~/${foo}./private;\n"+
			"interpolatedPathAdjacent = ./base${foo}./private;\n"+
			"interpolatedHomePathAdjacent = ~/base${foo}./private;\n"+
			"interpolatedPathPreSeparators = ./base//${foo}////./private;\n"+
			"interpolatedHomePathPreSeparators = ~/base//${foo}////./private;\n"+
			"interpolatedPathTriple = ./base/${foo}///./private;\n"+
			"interpolatedHomePathTriple = ~/${foo}///./private;\n"+
			"interpolatedPathExpression = ./base/${./user}/asset;\n"+
			"quoted = \"./private ./private/custom.nix\";\n"+
			"interpolated = \"${./user}/asset\";\n"+
			"indented = '' ./private ./private/custom.nix '';\n"+
			"indentedInterpolated = '' ${./user/custom.nix} '';\n"+
			"# ./private ./private/custom.nix\n"+
			"/* ./private ./private/custom.nix */\n" {
		t.Fatalf("unexpected rewritten Nix source:\n%s", got)
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(root, "user", "custom.nix")); got != "custom\n" {
		t.Fatalf("migrated user content = %q", got)
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(root, "user", "nested.nix")); got != "imports = [ ./private ];\n" {
		t.Fatalf("nested module path must keep its file-relative meaning, got %q", got)
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(root, "installer-state.json")); got != statePayload {
		t.Fatalf("migrated installer state = %q", got)
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(root, "wahrwelt", "keep.json")); got != "keep\n" {
		t.Fatalf("unknown wahrwelt content = %q", got)
	}
	for _, legacy := range []string{
		filepath.Join(root, "private"),
		filepath.Join(root, "wahrwelt", "state.json"),
	} {
		if _, err := os.Lstat(legacy); !os.IsNotExist(err) {
			t.Fatalf("legacy path %s was not removed: %v", legacy, err)
		}
	}
}

func TestSystemUserNamespaceMigrationRejectsUnownedTopLevelPrivateReference(t *testing.T) {
	root := t.TempDir()
	configuration := filepath.Join(root, "configuration.nix")
	custom := filepath.Join(root, "custom.nix")
	privateFile := filepath.Join(root, "private", "default.nix")
	state := filepath.Join(root, "wahrwelt", "state.json")
	mustWriteSystemMigrationFixture(t, configuration, "imports = [ ./private ];\n")
	mustWriteSystemMigrationFixture(t, custom, "# user-owned module\nimports = [ ./private ];\n")
	mustWriteSystemMigrationFixture(t, privateFile, "{ ... }: {}\n")
	mustWriteSystemMigrationFixture(t, state, systemMigrationStatePayload(7))
	if err := os.Chmod(custom, 0o640); err != nil {
		t.Fatal(err)
	}
	beforeInfo, err := os.Stat(custom)
	if err != nil {
		t.Fatal(err)
	}
	beforeCustom := mustReadSystemMigrationFixture(t, custom)
	beforeConfiguration := mustReadSystemMigrationFixture(t, configuration)
	beforeState := mustReadSystemMigrationFixture(t, state)

	output := runSystemUserNamespaceHelperOutput(t, 2, "migrate-stage", root)
	if !strings.Contains(output, "unowned top-level module") ||
		!strings.Contains(output, "move that reference manually") {
		t.Fatalf("unowned module rejection was not actionable:\n%s", output)
	}
	afterInfo, err := os.Stat(custom)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(beforeInfo, afterInfo) || afterInfo.Mode().Perm() != 0o640 {
		t.Fatalf("unowned module identity/mode changed: before=%v after=%v", beforeInfo, afterInfo)
	}
	if got := mustReadSystemMigrationFixture(t, custom); got != beforeCustom {
		t.Fatalf("unowned module bytes changed: %q", got)
	}
	if got := mustReadSystemMigrationFixture(t, configuration); got != beforeConfiguration {
		t.Fatalf("owned configuration changed before collision was rejected: %q", got)
	}
	if got := mustReadSystemMigrationFixture(t, state); got != beforeState {
		t.Fatalf("legacy state changed before collision was rejected: %q", got)
	}
	if got := mustReadSystemMigrationFixture(t, privateFile); got != "{ ... }: {}\n" {
		t.Fatalf("legacy private tree changed before collision was rejected: %q", got)
	}
}

func TestSystemUserNamespaceMigrationCompletionMarkerIsExclusive(t *testing.T) {
	root := t.TempDir()
	if output := runSystemUserNamespaceHelperOutput(t, 0, "completion-status", root); output != "absent\n" {
		t.Fatalf("initial completion status = %q", output)
	}
	runSystemUserNamespaceHelper(t, 0, "publish-completion", root)
	marker := filepath.Join(root, "v1_to_v2.complete")
	before, err := os.Stat(marker)
	if err != nil {
		t.Fatal(err)
	}
	if before.Mode().Perm() != 0o600 || !before.Mode().IsRegular() {
		t.Fatalf("completion marker metadata = %v", before.Mode())
	}
	if output := runSystemUserNamespaceHelperOutput(t, 0, "completion-status", root); output != "complete\n" {
		t.Fatalf("published completion status = %q", output)
	}
	output := runSystemUserNamespaceHelperOutput(t, 2, "publish-completion", root)
	if !strings.Contains(output, "completion marker already exists") {
		t.Fatalf("duplicate publication error was unclear:\n%s", output)
	}
	after, err := os.Stat(marker)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || mustReadSystemMigrationFixture(t, marker) != "wahrwelt-v1-to-v2-complete\n" {
		t.Fatal("duplicate completion publication changed the exact marker")
	}
}

func TestSystemUserNamespaceMigrationCompletionMarkerPreservesCollision(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "foreign")
	mustWriteSystemMigrationFixture(t, target, "foreign\n")
	marker := filepath.Join(root, "v1_to_v2.complete")
	if err := os.Symlink(target, marker); err != nil {
		t.Fatal(err)
	}
	output := runSystemUserNamespaceHelperOutput(t, 2, "completion-status", root)
	if !strings.Contains(output, "ownership collision") {
		t.Fatalf("completion collision was not explicit:\n%s", output)
	}
	if link, err := os.Readlink(marker); err != nil || link != target {
		t.Fatalf("completion collision changed: link=%q err=%v", link, err)
	}
	if got := mustReadSystemMigrationFixture(t, target); got != "foreign\n" {
		t.Fatalf("completion collision target changed: %q", got)
	}
}

func TestSystemUserNamespaceMigrationRejectsSymlinkConfigRoot(t *testing.T) {
	target := t.TempDir()
	mustWriteSystemMigrationFixture(t, filepath.Join(target, "configuration.nix"), "imports = [ ./private ];\n")
	mustWriteSystemMigrationFixture(t, filepath.Join(target, "private", "custom.nix"), "custom\n")
	link := filepath.Join(t.TempDir(), "config-root")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("python3", "-I", "-S", systemUserNamespaceHelper, "needs-migration", link)
	output, err := cmd.CombinedOutput()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("symlink config root must fail closed with exit 2: err=%v\n%s", err, output)
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(target, "configuration.nix")); got != "imports = [ ./private ];\n" {
		t.Fatalf("symlink root rejection changed target config: %q", got)
	}
}

func TestSystemUserNamespaceMigrationRejectsManagedMountpoint(t *testing.T) {
	if _, err := exec.LookPath("unshare"); err != nil {
		t.Skip("unshare is unavailable")
	}
	root := t.TempDir()
	outside := t.TempDir()
	private := filepath.Join(root, "private")
	if err := os.Mkdir(private, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteSystemMigrationFixture(t, filepath.Join(outside, "custom.nix"), "outside\n")

	cmd := exec.Command(
		"unshare", "--user", "--map-root-user", "--mount", "bash", "-c",
		`mount --bind "$1" "$2" && exec python3 -I -S "$3" needs-migration "$4"`,
		"namespace-mount-test", outside, private, systemUserNamespaceHelper, root,
	)
	output, err := cmd.CombinedOutput()
	if err != nil && strings.Contains(string(output), "Operation not permitted") {
		t.Skipf("unprivileged bind mounts are unavailable: %s", output)
	}
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("managed mountpoint must fail closed with exit 2: err=%v\n%s", err, output)
	}
	if !strings.Contains(string(output), "is a mountpoint") {
		t.Fatalf("managed mountpoint rejection was unclear:\n%s", output)
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(outside, "custom.nix")); got != "outside\n" {
		t.Fatalf("mountpoint rejection changed outside content: %q", got)
	}
}

func TestSystemUserNamespacePrecommitAcceptsUnchangedLiveTree(t *testing.T) {
	live := t.TempDir()
	stage := t.TempDir()
	for _, root := range []string{live, stage} {
		mustWriteSystemMigrationFixture(t, filepath.Join(root, "configuration.nix"), "imports = [ ./private ];\n")
		mustWriteSystemMigrationFixture(t, filepath.Join(root, "private", "custom.nix"), "custom\n")
	}
	snapshot := filepath.Join(t.TempDir(), "namespace.json")
	mustWriteSystemMigrationFixture(t, snapshot, "")
	runSystemUserNamespaceHelper(t, 0, "snapshot-live", live, snapshot)
	runSystemUserNamespaceHelper(t, 0, "migrate-stage", stage)

	runSystemUserNamespaceHelper(t, 0, "precommit", live, stage, snapshot)
}

func TestSystemUserNamespaceSnapshotRejectsExternalHardlinks(t *testing.T) {
	live := t.TempDir()
	liveFile := filepath.Join(live, "private", "custom.nix")
	mustWriteSystemMigrationFixture(t, filepath.Join(live, "configuration.nix"), "imports = [ ./private ];\n")
	mustWriteSystemMigrationFixture(t, liveFile, "custom\n")
	outside := filepath.Join(t.TempDir(), "outside-link")
	if err := os.Link(liveFile, outside); err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(t.TempDir(), "namespace.json")
	mustWriteSystemMigrationFixture(t, snapshot, "")

	cmd := exec.Command("python3", "-I", "-S", systemUserNamespaceHelper, "snapshot-live", live, snapshot)
	output, err := cmd.CombinedOutput()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("external hardlink topology must fail closed with exit 2: err=%v\n%s", err, output)
	}
	if !strings.Contains(string(output), "external hardlink") {
		t.Fatalf("external hardlink rejection was unclear:\n%s", output)
	}
	if got := mustReadSystemMigrationFixture(t, liveFile); got != "custom\n" {
		t.Fatalf("external hardlink rejection changed live content: %q", got)
	}
	if got := mustReadSystemMigrationFixture(t, outside); got != "custom\n" {
		t.Fatalf("external hardlink rejection changed outside content: %q", got)
	}
}

func TestSystemUserNamespaceAtomicPublishExchangesCanonicalAndLegacyTrees(t *testing.T) {
	parent := t.TempDir()
	live := filepath.Join(parent, "nixos")
	candidate := filepath.Join(parent, ".nixos.migration.candidate")
	for _, root := range []string{live, candidate} {
		mustWriteSystemMigrationFixture(t, filepath.Join(root, "configuration.nix"), "imports = [ ./private ];\n")
		mustWriteSystemMigrationFixture(t, filepath.Join(root, "private", "custom.nix"), "custom\n")
	}
	snapshot := filepath.Join(t.TempDir(), "namespace.json")
	mustWriteSystemMigrationFixture(t, snapshot, "")
	runSystemUserNamespaceHelper(t, 0, "snapshot-live", live, snapshot)
	runSystemUserNamespaceHelper(t, 0, "migrate-stage", candidate)

	runSystemUserNamespaceHelper(t, 0, "publish", live, candidate, snapshot)

	if got := mustReadSystemMigrationFixture(t, filepath.Join(live, "configuration.nix")); got != "imports = [ ./user ];\n" {
		t.Fatalf("published config = %q", got)
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(live, "user", "custom.nix")); got != "custom\n" {
		t.Fatalf("published user content = %q", got)
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(candidate, "configuration.nix")); got != "imports = [ ./private ];\n" {
		t.Fatalf("displaced legacy config = %q", got)
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(candidate, "private", "custom.nix")); got != "custom\n" {
		t.Fatalf("displaced legacy user content = %q", got)
	}
}

func TestSystemUserNamespaceAtomicPublishRollsBackPostSnapshotOwnerEdit(t *testing.T) {
	parent := t.TempDir()
	live := filepath.Join(parent, "nixos")
	candidate := filepath.Join(parent, ".nixos.migration.candidate")
	for _, root := range []string{live, candidate} {
		mustWriteSystemMigrationFixture(t, filepath.Join(root, "configuration.nix"), "imports = [ ./private ];\n")
		mustWriteSystemMigrationFixture(t, filepath.Join(root, "private", "custom.nix"), "custom\n")
	}
	snapshot := filepath.Join(t.TempDir(), "namespace.json")
	mustWriteSystemMigrationFixture(t, snapshot, "")
	runSystemUserNamespaceHelper(t, 0, "snapshot-live", live, snapshot)
	runSystemUserNamespaceHelper(t, 0, "migrate-stage", candidate)
	candidateBefore := snapshotSystemMigrationTree(t, candidate)
	liveInfoBefore, err := os.Lstat(live)
	if err != nil {
		t.Fatal(err)
	}
	candidateInfoBefore, err := os.Lstat(candidate)
	if err != nil {
		t.Fatal(err)
	}

	readyR, readyW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readyR.Close()
	continueR, continueW, err := os.Pipe()
	if err != nil {
		_ = readyW.Close()
		t.Fatal(err)
	}
	defer continueW.Close()

	cmd := exec.Command("python3", "-I", "-S", systemUserNamespaceHelper, "publish", live, candidate, snapshot)
	cmd.Env = append(
		os.Environ(),
		"WAHRWELT_TEST_NAMESPACE_PUBLISH_READY_FD=3",
		"WAHRWELT_TEST_NAMESPACE_PUBLISH_CONTINUE_FD=4",
	)
	cmd.ExtraFiles = []*os.File{readyW, continueR}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		_ = readyW.Close()
		_ = continueR.Close()
		t.Fatal(err)
	}
	_ = readyW.Close()
	_ = continueR.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	defer func() {
		if finished {
			return
		}
		_, _ = continueW.Write([]byte{'1'})
		_ = cmd.Process.Kill()
		<-done
	}()

	ready := make(chan error, 1)
	go func() {
		marker := make([]byte, len("ready\n"))
		_, err := io.ReadFull(readyR, marker)
		if err == nil && string(marker) != "ready\n" {
			err = fmt.Errorf("unexpected barrier marker %q", marker)
		}
		ready <- err
	}()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatalf("publish barrier failed: %v\n%s", err, output.String())
		}
	case err := <-done:
		finished = true
		t.Fatalf("publish exited before barrier: %v\n%s", err, output.String())
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for publish barrier\n%s", output.String())
	}

	racedConfig := "# exact post-snapshot owner edit\nimports = [ ./private ];\n"
	mustWriteSystemMigrationFixture(t, filepath.Join(live, "configuration.nix"), racedConfig)
	racedLive := snapshotSystemMigrationTree(t, live)
	if _, err := continueW.Write([]byte{'1'}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		finished = true
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
			t.Fatalf("raced publish must fail after one rollback: err=%v\n%s", err, output.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for raced publish rollback\n%s", output.String())
	}
	if !strings.Contains(output.String(), "rolled back through exact pinned entries") {
		t.Fatalf("publish did not report the pinned rollback\n%s", output.String())
	}

	if after := snapshotSystemMigrationTree(t, live); after != racedLive {
		t.Fatalf("rollback did not preserve exact raced live tree:\nwant:\n%s\ngot:\n%s\n%s", racedLive, after, output.String())
	}
	if after := snapshotSystemMigrationTree(t, candidate); after != candidateBefore {
		t.Fatalf("rollback did not restore exact canonical candidate:\nwant:\n%s\ngot:\n%s\n%s", candidateBefore, after, output.String())
	}
	liveInfoAfter, err := os.Lstat(live)
	if err != nil {
		t.Fatal(err)
	}
	candidateInfoAfter, err := os.Lstat(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(liveInfoBefore, liveInfoAfter) || !os.SameFile(candidateInfoBefore, candidateInfoAfter) {
		t.Fatalf("pinned rollback did not restore the exact directory identities")
	}
}

func TestSystemUserNamespaceAtomicPublishRejectsLiveReplacementBeforeExchange(t *testing.T) {
	parent := t.TempDir()
	live := filepath.Join(parent, "nixos")
	candidate := filepath.Join(parent, ".nixos.migration.candidate")
	originalLive := filepath.Join(parent, "nixos-before-race")
	for _, root := range []string{live, candidate} {
		mustWriteSystemMigrationFixture(t, filepath.Join(root, "configuration.nix"), "imports = [ ./private ];\n")
		mustWriteSystemMigrationFixture(t, filepath.Join(root, "private", "custom.nix"), "custom\n")
	}
	snapshot := filepath.Join(t.TempDir(), "namespace.json")
	mustWriteSystemMigrationFixture(t, snapshot, "")
	runSystemUserNamespaceHelper(t, 0, "snapshot-live", live, snapshot)
	runSystemUserNamespaceHelper(t, 0, "migrate-stage", candidate)
	candidateBefore := snapshotSystemMigrationTree(t, candidate)

	readyR, readyW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readyR.Close()
	continueR, continueW, err := os.Pipe()
	if err != nil {
		_ = readyW.Close()
		t.Fatal(err)
	}
	defer continueW.Close()
	cmd := exec.Command("python3", "-I", "-S", systemUserNamespaceHelper, "publish", live, candidate, snapshot)
	cmd.Env = append(os.Environ(),
		"WAHRWELT_TEST_NAMESPACE_PUBLISH_READY_FD=3",
		"WAHRWELT_TEST_NAMESPACE_PUBLISH_CONTINUE_FD=4",
	)
	cmd.ExtraFiles = []*os.File{readyW, continueR}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		_ = readyW.Close()
		_ = continueR.Close()
		t.Fatal(err)
	}
	_ = readyW.Close()
	_ = continueR.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	defer func() {
		if finished {
			return
		}
		_, _ = continueW.Write([]byte{'1'})
		_ = cmd.Process.Kill()
		<-done
	}()
	marker := make([]byte, len("ready\n"))
	ready := make(chan error, 1)
	go func() {
		_, readErr := io.ReadFull(readyR, marker)
		ready <- readErr
	}()
	select {
	case readErr := <-ready:
		if readErr != nil || string(marker) != "ready\n" {
			t.Fatalf("publish barrier failed: marker=%q err=%v\n%s", marker, readErr, output.String())
		}
	case waitErr := <-done:
		finished = true
		t.Fatalf("publish exited before barrier: %v\n%s", waitErr, output.String())
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for publish barrier\n%s", output.String())
	}

	if err := os.Rename(live, originalLive); err != nil {
		t.Fatal(err)
	}
	mustWriteSystemMigrationFixture(t, filepath.Join(live, "winner"), "second owner\n")
	if _, err := continueW.Write([]byte{'1'}); err != nil {
		t.Fatal(err)
	}
	waitErr := <-done
	finished = true
	var exitError *exec.ExitError
	if !errors.As(waitErr, &exitError) || exitError.ExitCode() != 2 ||
		!strings.Contains(output.String(), "pre-exchange revalidation: directory identity changed") {
		t.Fatalf("live replacement was not rejected before no-replace quarantine: err=%v\n%s", waitErr, output.String())
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(live, "winner")); got != "second owner\n" {
		t.Fatalf("second owner changed: %q", got)
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(originalLive, "configuration.nix")); got != "imports = [ ./private ];\n" {
		t.Fatalf("original live tree changed: %q", got)
	}
	if after := snapshotSystemMigrationTree(t, candidate); after != candidateBefore {
		t.Fatalf("candidate changed before refused exchange:\nwant:\n%s\ngot:\n%s", candidateBefore, after)
	}
}

func TestSystemUserNamespaceAtomicPublishPreservesPostRenameTargetReplacement(t *testing.T) {
	parent := t.TempDir()
	live := filepath.Join(parent, "nixos")
	candidate := filepath.Join(parent, ".nixos.migration.candidate")
	expectedRecovery := filepath.Join(parent, "canonical-after-exchange-race")
	for _, root := range []string{live, candidate} {
		mustWriteSystemMigrationFixture(t, filepath.Join(root, "configuration.nix"), "imports = [ ./private ];\n")
		mustWriteSystemMigrationFixture(t, filepath.Join(root, "private", "custom.nix"), "custom\n")
	}
	snapshot := filepath.Join(t.TempDir(), "namespace.json")
	mustWriteSystemMigrationFixture(t, snapshot, "")
	runSystemUserNamespaceHelper(t, 0, "snapshot-live", live, snapshot)
	runSystemUserNamespaceHelper(t, 0, "migrate-stage", candidate)
	candidateBefore := snapshotSystemMigrationTree(t, candidate)
	liveBefore := snapshotSystemMigrationTree(t, live)

	readyR, readyW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readyR.Close()
	continueR, continueW, err := os.Pipe()
	if err != nil {
		_ = readyW.Close()
		t.Fatal(err)
	}
	defer continueW.Close()
	cmd := exec.Command("python3", "-I", "-S", systemUserNamespaceHelper, "publish", live, candidate, snapshot)
	cmd.Env = append(os.Environ(),
		"WAHRWELT_TEST_NAMESPACE_EXCHANGE_READY_FD=3",
		"WAHRWELT_TEST_NAMESPACE_EXCHANGE_CONTINUE_FD=4",
	)
	cmd.ExtraFiles = []*os.File{readyW, continueR}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		_ = readyW.Close()
		_ = continueR.Close()
		t.Fatal(err)
	}
	_ = readyW.Close()
	_ = continueR.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	defer func() {
		if finished {
			return
		}
		_, _ = continueW.Write([]byte{'1'})
		_ = cmd.Process.Kill()
		<-done
	}()

	marker := make([]byte, len("ready\n"))
	ready := make(chan error, 1)
	go func() {
		_, readErr := io.ReadFull(readyR, marker)
		ready <- readErr
	}()
	select {
	case readErr := <-ready:
		if readErr != nil || string(marker) != "ready\n" {
			t.Fatalf("move barrier failed: marker=%q err=%v\n%s", marker, readErr, output.String())
		}
	case waitErr := <-done:
		finished = true
		t.Fatalf("publish exited before exchange barrier: %v\n%s", waitErr, output.String())
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for exchange barrier\n%s", output.String())
	}

	if err := os.Rename(live, expectedRecovery); err != nil {
		t.Fatal(err)
	}
	mustWriteSystemMigrationFixture(t, filepath.Join(live, "winner"), "post-exchange target winner\n")
	if _, err := continueW.Write([]byte{'1'}); err != nil {
		t.Fatal(err)
	}
	waitErr := <-done
	finished = true
	var exitError *exec.ExitError
	if !errors.As(waitErr, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("post-rename target replacement must fail closed: err=%v\n%s", waitErr, output.String())
	}
	for _, want := range []string{live, expectedRecovery} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("exact recovery location %q was not reported:\n%s", want, output.String())
		}
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(live, "winner")); got != "post-exchange target winner\n" {
		t.Fatalf("post-exchange target winner changed: %q", got)
	}
	if after := snapshotSystemMigrationTree(t, expectedRecovery); after != candidateBefore {
		t.Fatalf("canonical recovery changed:\nwant:\n%s\ngot:\n%s", candidateBefore, after)
	}
	if after := snapshotSystemMigrationTree(t, candidate); after != liveBefore {
		t.Fatalf("displaced live recovery changed:\nwant:\n%s\ngot:\n%s", liveBefore, after)
	}
}

func TestSystemUserNamespaceAtomicPublishKillAfterExchangeLeavesCanonicalLive(t *testing.T) {
	parent := t.TempDir()
	live := filepath.Join(parent, "nixos")
	candidate := filepath.Join(parent, ".nixos.migration.candidate")
	for _, root := range []string{live, candidate} {
		mustWriteSystemMigrationFixture(t, filepath.Join(root, "configuration.nix"), "imports = [ ./private ];\n")
		mustWriteSystemMigrationFixture(t, filepath.Join(root, "private", "custom.nix"), "custom\n")
	}
	snapshot := filepath.Join(t.TempDir(), "namespace.json")
	mustWriteSystemMigrationFixture(t, snapshot, "")
	runSystemUserNamespaceHelper(t, 0, "snapshot-live", live, snapshot)
	runSystemUserNamespaceHelper(t, 0, "migrate-stage", candidate)
	liveBefore := snapshotSystemMigrationTree(t, live)
	candidateBefore := snapshotSystemMigrationTree(t, candidate)

	readyR, readyW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readyR.Close()
	continueR, continueW, err := os.Pipe()
	if err != nil {
		_ = readyW.Close()
		t.Fatal(err)
	}
	defer continueW.Close()
	cmd := exec.Command("python3", "-I", "-S", systemUserNamespaceHelper, "publish", live, candidate, snapshot)
	cmd.Env = append(os.Environ(),
		"WAHRWELT_TEST_NAMESPACE_EXCHANGE_READY_FD=3",
		"WAHRWELT_TEST_NAMESPACE_EXCHANGE_CONTINUE_FD=4",
	)
	cmd.ExtraFiles = []*os.File{readyW, continueR}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		_ = readyW.Close()
		_ = continueR.Close()
		t.Fatal(err)
	}
	_ = readyW.Close()
	_ = continueR.Close()

	marker := make([]byte, len("ready\n"))
	ready := make(chan error, 1)
	go func() {
		_, readErr := io.ReadFull(readyR, marker)
		ready <- readErr
	}()
	select {
	case readErr := <-ready:
		if readErr != nil || string(marker) != "ready\n" {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			t.Fatalf("exchange barrier failed: marker=%q err=%v\n%s", marker, readErr, output.String())
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("timed out waiting for exchange barrier\n%s", output.String())
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err == nil {
		t.Fatal("killed publisher unexpectedly exited successfully")
	}

	if after := snapshotSystemMigrationTree(t, live); after != candidateBefore {
		t.Fatalf("kill after atomic exchange did not leave canonical live tree:\nwant:\n%s\ngot:\n%s", candidateBefore, after)
	}
	if after := snapshotSystemMigrationTree(t, candidate); after != liveBefore {
		t.Fatalf("kill after atomic exchange changed displaced live recovery:\nwant:\n%s\ngot:\n%s", liveBefore, after)
	}
	runSystemUserNamespaceHelper(t, 0, "validate-migrated", live)
}

func TestSystemUserNamespaceAtomicPublishReportsExactPostRollbackRecovery(t *testing.T) {
	parent := t.TempDir()
	live := filepath.Join(parent, "nixos")
	candidate := filepath.Join(parent, ".nixos.migration.candidate")
	liveRecovery := filepath.Join(parent, "live-after-rollback-race")
	for _, root := range []string{live, candidate} {
		mustWriteSystemMigrationFixture(t, filepath.Join(root, "configuration.nix"), "imports = [ ./private ];\n")
		mustWriteSystemMigrationFixture(t, filepath.Join(root, "private", "custom.nix"), "custom\n")
	}
	snapshot := filepath.Join(t.TempDir(), "namespace.json")
	mustWriteSystemMigrationFixture(t, snapshot, "")
	runSystemUserNamespaceHelper(t, 0, "snapshot-live", live, snapshot)
	runSystemUserNamespaceHelper(t, 0, "migrate-stage", candidate)
	candidateBefore := snapshotSystemMigrationTree(t, candidate)

	publishReadyR, publishReadyW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer publishReadyR.Close()
	publishContinueR, publishContinueW, err := os.Pipe()
	if err != nil {
		_ = publishReadyW.Close()
		t.Fatal(err)
	}
	defer publishContinueW.Close()
	postReadyR, postReadyW, err := os.Pipe()
	if err != nil {
		_ = publishReadyW.Close()
		_ = publishContinueR.Close()
		t.Fatal(err)
	}
	defer postReadyR.Close()
	postContinueR, postContinueW, err := os.Pipe()
	if err != nil {
		_ = publishReadyW.Close()
		_ = publishContinueR.Close()
		_ = postReadyW.Close()
		t.Fatal(err)
	}
	defer postContinueW.Close()

	cmd := exec.Command("python3", "-I", "-S", systemUserNamespaceHelper, "publish", live, candidate, snapshot)
	cmd.Env = append(os.Environ(),
		"WAHRWELT_TEST_NAMESPACE_PUBLISH_READY_FD=3",
		"WAHRWELT_TEST_NAMESPACE_PUBLISH_CONTINUE_FD=4",
		"WAHRWELT_TEST_NAMESPACE_POST_ROLLBACK_READY_FD=5",
		"WAHRWELT_TEST_NAMESPACE_POST_ROLLBACK_CONTINUE_FD=6",
	)
	cmd.ExtraFiles = []*os.File{publishReadyW, publishContinueR, postReadyW, postContinueR}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		_ = publishReadyW.Close()
		_ = publishContinueR.Close()
		_ = postReadyW.Close()
		_ = postContinueR.Close()
		t.Fatal(err)
	}
	_ = publishReadyW.Close()
	_ = publishContinueR.Close()
	_ = postReadyW.Close()
	_ = postContinueR.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	defer func() {
		if finished {
			return
		}
		_, _ = publishContinueW.Write([]byte{'1'})
		_, _ = postContinueW.Write([]byte{'1'})
		_ = cmd.Process.Kill()
		<-done
	}()
	waitBarrier := func(name string, reader *os.File) {
		t.Helper()
		marker := make([]byte, len("ready\n"))
		ready := make(chan error, 1)
		go func() {
			_, readErr := io.ReadFull(reader, marker)
			ready <- readErr
		}()
		select {
		case readErr := <-ready:
			if readErr != nil || string(marker) != "ready\n" {
				t.Fatalf("%s barrier failed: marker=%q err=%v\n%s", name, marker, readErr, output.String())
			}
		case waitErr := <-done:
			finished = true
			t.Fatalf("publish exited before %s barrier: %v\n%s", name, waitErr, output.String())
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for %s barrier\n%s", name, output.String())
		}
	}

	waitBarrier("publish", publishReadyR)
	racedConfig := "# owner edit before exchange\nimports = [ ./private ];\n"
	mustWriteSystemMigrationFixture(t, filepath.Join(live, "configuration.nix"), racedConfig)
	racedLive := snapshotSystemMigrationTree(t, live)
	if _, err := publishContinueW.Write([]byte{'1'}); err != nil {
		t.Fatal(err)
	}
	waitBarrier("post-rollback", postReadyR)
	if err := os.Rename(live, liveRecovery); err != nil {
		t.Fatal(err)
	}
	mustWriteSystemMigrationFixture(t, filepath.Join(live, "winner"), "post-rollback winner\n")
	if _, err := postContinueW.Write([]byte{'1'}); err != nil {
		t.Fatal(err)
	}
	waitErr := <-done
	finished = true
	var exitError *exec.ExitError
	if !errors.As(waitErr, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("post-rollback replacement must fail closed: err=%v\n%s", waitErr, output.String())
	}
	for _, want := range []string{liveRecovery, candidate} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("exact recovery location %q was not reported:\n%s", want, output.String())
		}
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(live, "winner")); got != "post-rollback winner\n" {
		t.Fatalf("post-rollback winner changed: %q", got)
	}
	if after := snapshotSystemMigrationTree(t, liveRecovery); after != racedLive {
		t.Fatalf("original live recovery changed:\nwant:\n%s\ngot:\n%s", racedLive, after)
	}
	if after := snapshotSystemMigrationTree(t, candidate); after != candidateBefore {
		t.Fatalf("canonical candidate recovery changed:\nwant:\n%s\ngot:\n%s", candidateBefore, after)
	}
}

func TestSystemUserNamespaceAtomicPublishRefusesRollbackOverSecondWinner(t *testing.T) {
	parent := t.TempDir()
	live := filepath.Join(parent, "nixos")
	candidate := filepath.Join(parent, ".nixos.migration.candidate")
	canonicalRecovery := filepath.Join(parent, "canonical-before-second-winner")
	for _, root := range []string{live, candidate} {
		mustWriteSystemMigrationFixture(t, filepath.Join(root, "configuration.nix"), "imports = [ ./private ];\n")
		mustWriteSystemMigrationFixture(t, filepath.Join(root, "private", "custom.nix"), "custom\n")
	}
	snapshot := filepath.Join(t.TempDir(), "namespace.json")
	mustWriteSystemMigrationFixture(t, snapshot, "")
	runSystemUserNamespaceHelper(t, 0, "snapshot-live", live, snapshot)
	runSystemUserNamespaceHelper(t, 0, "migrate-stage", candidate)
	candidateBefore := snapshotSystemMigrationTree(t, candidate)

	publishReadyR, publishReadyW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer publishReadyR.Close()
	publishContinueR, publishContinueW, err := os.Pipe()
	if err != nil {
		_ = publishReadyW.Close()
		t.Fatal(err)
	}
	defer publishContinueW.Close()
	rollbackReadyR, rollbackReadyW, err := os.Pipe()
	if err != nil {
		_ = publishReadyW.Close()
		_ = publishContinueR.Close()
		t.Fatal(err)
	}
	defer rollbackReadyR.Close()
	rollbackContinueR, rollbackContinueW, err := os.Pipe()
	if err != nil {
		_ = publishReadyW.Close()
		_ = publishContinueR.Close()
		_ = rollbackReadyW.Close()
		t.Fatal(err)
	}
	defer rollbackContinueW.Close()

	cmd := exec.Command("python3", "-I", "-S", systemUserNamespaceHelper, "publish", live, candidate, snapshot)
	cmd.Env = append(os.Environ(),
		"WAHRWELT_TEST_NAMESPACE_PUBLISH_READY_FD=3",
		"WAHRWELT_TEST_NAMESPACE_PUBLISH_CONTINUE_FD=4",
		"WAHRWELT_TEST_NAMESPACE_ROLLBACK_READY_FD=5",
		"WAHRWELT_TEST_NAMESPACE_ROLLBACK_CONTINUE_FD=6",
	)
	cmd.ExtraFiles = []*os.File{publishReadyW, publishContinueR, rollbackReadyW, rollbackContinueR}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		_ = publishReadyW.Close()
		_ = publishContinueR.Close()
		_ = rollbackReadyW.Close()
		_ = rollbackContinueR.Close()
		t.Fatal(err)
	}
	_ = publishReadyW.Close()
	_ = publishContinueR.Close()
	_ = rollbackReadyW.Close()
	_ = rollbackContinueR.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	defer func() {
		if finished {
			return
		}
		_, _ = publishContinueW.Write([]byte{'1'})
		_, _ = rollbackContinueW.Write([]byte{'1'})
		_ = cmd.Process.Kill()
		<-done
	}()

	waitBarrier := func(name string, reader *os.File) {
		t.Helper()
		marker := make([]byte, len("ready\n"))
		ready := make(chan error, 1)
		go func() {
			_, readErr := io.ReadFull(reader, marker)
			ready <- readErr
		}()
		select {
		case readErr := <-ready:
			if readErr != nil || string(marker) != "ready\n" {
				t.Fatalf("%s barrier failed: marker=%q err=%v\n%s", name, marker, readErr, output.String())
			}
		case waitErr := <-done:
			finished = true
			t.Fatalf("publish exited before %s barrier: %v\n%s", name, waitErr, output.String())
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for %s barrier\n%s", name, output.String())
		}
	}

	waitBarrier("publish", publishReadyR)
	racedConfig := "# owner edit before exchange\nimports = [ ./private ];\n"
	mustWriteSystemMigrationFixture(t, filepath.Join(live, "configuration.nix"), racedConfig)
	racedLive := snapshotSystemMigrationTree(t, live)
	if _, err := publishContinueW.Write([]byte{'1'}); err != nil {
		t.Fatal(err)
	}
	waitBarrier("rollback", rollbackReadyR)
	if err := os.Rename(live, canonicalRecovery); err != nil {
		t.Fatal(err)
	}
	mustWriteSystemMigrationFixture(t, filepath.Join(live, "winner"), "second owner\n")
	if _, err := rollbackContinueW.Write([]byte{'1'}); err != nil {
		t.Fatal(err)
	}
	waitErr := <-done
	finished = true
	var exitError *exec.ExitError
	if !errors.As(waitErr, &exitError) || exitError.ExitCode() != 2 ||
		!strings.Contains(output.String(), "rollback was refused") {
		t.Fatalf("rollback did not refuse second winner: err=%v\n%s", waitErr, output.String())
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(live, "winner")); got != "second owner\n" {
		t.Fatalf("second live winner changed: %q", got)
	}
	if after := snapshotSystemMigrationTree(t, canonicalRecovery); after != candidateBefore {
		t.Fatalf("published canonical recovery changed:\nwant:\n%s\ngot:\n%s", candidateBefore, after)
	}
	if after := snapshotSystemMigrationTree(t, candidate); after != racedLive {
		t.Fatalf("displaced raced live tree changed:\nwant:\n%s\ngot:\n%s", racedLive, after)
	}
}

func TestSystemUserNamespaceMigrationQuarantinesExactEmptyOrdinaryStateParent(t *testing.T) {
	root := t.TempDir()
	statePayload := systemMigrationStatePayload(7)
	legacyParent := filepath.Join(root, "wahrwelt")
	mustWriteSystemMigrationFixture(t, filepath.Join(legacyParent, "state.json"), statePayload)
	parentBefore, err := os.Lstat(legacyParent)
	if err != nil {
		t.Fatal(err)
	}

	runSystemUserNamespaceHelper(t, 0, "migrate-stage", root)

	if _, err := os.Lstat(legacyParent); !os.IsNotExist(err) {
		t.Fatalf("exact empty legacy state parent remains at its old path: %v", err)
	}
	quarantine := findSystemMigrationParentQuarantine(t, root, "wahrwelt", parentBefore)
	if entries, err := os.ReadDir(quarantine); err != nil || len(entries) != 0 {
		t.Fatalf("quarantined legacy parent is not empty: entries=%v err=%v", entries, err)
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(root, "installer-state.json")); got != statePayload {
		t.Fatalf("migrated installer state = %q", got)
	}
}

func TestSystemUserNamespaceMigrationAcceptsGeneratedStateSchemas(t *testing.T) {
	for schema := 0; schema <= 7; schema++ {
		t.Run(fmt.Sprintf("schema-%d", schema), func(t *testing.T) {
			root := t.TempDir()
			brand := "wahrwelt"
			if schema%2 == 1 {
				brand = "mysetup"
			}
			payload := systemMigrationStatePayload(schema)
			legacyState := filepath.Join(root, brand, "state.json")
			mustWriteSystemMigrationFixture(t, legacyState, payload)
			goParserState := filepath.Join(t.TempDir(), "state.json")
			mustWriteSystemMigrationFixture(t, goParserState, payload)
			if _, err := LoadExisting(goParserState); err != nil {
				t.Fatalf("Go state parser rejected accepted schema %d payload: %v", schema, err)
			}
			if _, err := LoadGeneratedExisting(goParserState); err != nil {
				t.Fatalf("Go strict ownership parser rejected accepted schema %d payload: %v", schema, err)
			}

			runSystemUserNamespaceHelper(t, 0, "migrate-stage", root)

			if got := mustReadSystemMigrationFixture(t, filepath.Join(root, "installer-state.json")); got != payload {
				t.Fatalf("schema %d payload changed during migration", schema)
			}
			if _, err := os.Lstat(legacyState); !os.IsNotExist(err) {
				t.Fatalf("schema %d legacy state remains: %v", schema, err)
			}
		})
	}
}

func TestSystemUserNamespaceMigrationAcceptsHistoricalGeneratedVariants(t *testing.T) {
	tests := []struct {
		name   string
		schema int
		mutate func(map[string]any)
		brand  string
	}{
		{
			name:   "explicit schema 0",
			schema: 0,
			brand:  "mysetup",
			mutate: func(state map[string]any) { state["schemaVersion"] = 0 },
		},
		{
			name:   "schema 3 after shell selector removal",
			schema: 3,
			brand:  "wahrwelt",
			mutate: func(state map[string]any) { delete(state, "shell") },
		},
		{
			name:   "schema 5 after services removal",
			schema: 5,
			brand:  "mysetup",
			mutate: func(state map[string]any) {
				delete(state, "services")
				delete(state["display"].(map[string]any), "extraMonitors")
			},
		},
		{
			name:   "schema 5 after Russia mode removal",
			schema: 5,
			brand:  "mysetup",
			mutate: func(state map[string]any) {
				delete(state, "services")
				delete(state["features"].(map[string]any), "russiaMode")
			},
		},
		{
			name:   "schema 6 after Neovim cleanup removal",
			schema: 6,
			brand:  "wahrwelt",
			mutate: func(state map[string]any) {
				delete(state["dots"].(map[string]any), "neovimCleanState")
			},
		},
		{
			name:   "schema 7 with Zapret before removal",
			schema: 7,
			brand:  "mysetup",
			mutate: func(state map[string]any) {
				delete(state["features"].(map[string]any), "portainer")
				state["zapret"] = map[string]any{
					"enable": false,
					"config": "general (FAKE_TLS_AUTO_ALT3)",
				}
			},
		},
		{
			name:   "schema 7 after Zapret removal before Portainer",
			schema: 7,
			brand:  "wahrwelt",
			mutate: func(state map[string]any) {
				delete(state["features"].(map[string]any), "portainer")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			payload := mutateSystemMigrationStatePayload(test.schema, test.mutate)
			legacyState := filepath.Join(root, test.brand, "state.json")
			mustWriteSystemMigrationFixture(t, legacyState, payload)
			goParserState := filepath.Join(t.TempDir(), "state.json")
			mustWriteSystemMigrationFixture(t, goParserState, payload)
			if _, err := LoadExisting(goParserState); err != nil {
				t.Fatalf("Go state parser rejected accepted historical payload: %v", err)
			}
			if _, err := LoadGeneratedExisting(goParserState); err != nil {
				t.Fatalf("Go strict ownership parser rejected accepted historical payload: %v", err)
			}

			runSystemUserNamespaceHelper(t, 0, "migrate-stage", root)

			if got := mustReadSystemMigrationFixture(t, filepath.Join(root, "installer-state.json")); got != payload {
				t.Fatalf("historical state payload changed during migration")
			}
		})
	}
}

func TestSystemUserNamespaceMigrationRejectsUnownedLegacyStateContent(t *testing.T) {
	validWithUnknown := strings.TrimSuffix(systemMigrationStatePayload(7), "}\n") +
		",\n  \"owner\": \"human\"\n}\n"
	unknownNested := mutateSystemMigrationStatePayload(7, func(state map[string]any) {
		state["features"].(map[string]any)["owner"] = "human"
	})
	missingRequired := mutateSystemMigrationStatePayload(7, func(state map[string]any) {
		delete(state, "host")
	})
	wrongType := mutateSystemMigrationStatePayload(7, func(state map[string]any) {
		state["features"].(map[string]any)["secureBoot"] = "false"
	})
	duplicateField := strings.Replace(
		systemMigrationStatePayload(7),
		`"schemaVersion": 7,`,
		`"schemaVersion": 7, "schemaVersion": 7,`,
		1,
	)
	impossibleSchema5 := mutateSystemMigrationStatePayload(5, func(state map[string]any) {
		delete(state["features"].(map[string]any), "russiaMode")
	})
	impossibleSchema7 := mutateSystemMigrationStatePayload(7, func(state map[string]any) {
		state["zapret"] = map[string]any{
			"enable": false,
			"config": "general (FAKE_TLS_AUTO_ALT3)",
		}
	})
	tests := []struct {
		name    string
		brand   string
		payload string
	}{
		{name: "arbitrary Wahrwelt bytes", brand: "wahrwelt", payload: "state\n"},
		{name: "arbitrary MySetup bytes", brand: "mysetup", payload: "human file\n"},
		{name: "malformed JSON", brand: "wahrwelt", payload: `{"schemaVersion":7` + "\n"},
		{name: "empty object", brand: "mysetup", payload: "{}\n"},
		{name: "future schema", brand: "wahrwelt", payload: `{"schemaVersion":8}` + "\n"},
		{name: "negative schema", brand: "mysetup", payload: `{"schemaVersion":-1}` + "\n"},
		{name: "non-integer schema", brand: "wahrwelt", payload: `{"schemaVersion":"7"}` + "\n"},
		{name: "unknown field", brand: "mysetup", payload: validWithUnknown},
		{name: "unknown nested field", brand: "wahrwelt", payload: unknownNested},
		{name: "missing required object", brand: "mysetup", payload: missingRequired},
		{name: "wrong nested type", brand: "wahrwelt", payload: wrongType},
		{name: "duplicate field", brand: "mysetup", payload: duplicateField},
		{name: "impossible schema 5 field combination", brand: "wahrwelt", payload: impossibleSchema5},
		{name: "impossible schema 7 field combination", brand: "mysetup", payload: impossibleSchema7},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			legacyState := filepath.Join(root, test.brand, "state.json")
			mustWriteSystemMigrationFixture(t, legacyState, test.payload)
			mustWriteSystemMigrationFixture(t, filepath.Join(root, "private", "custom.nix"), "owner config\n")
			mustWriteSystemMigrationFixture(t, filepath.Join(root, "configuration.nix"), "imports = [ ./private ];\n")
			before := snapshotSystemMigrationTree(t, root)

			output := runSystemUserNamespaceHelperOutput(t, 2, "migrate-stage", root)

			if !strings.Contains(output, "ownership collision") {
				t.Fatalf("state rejection did not identify an ownership collision:\n%s", output)
			}
			if after := snapshotSystemMigrationTree(t, root); after != before {
				t.Fatalf("invalid legacy state changed the tree:\nwant:\n%s\ngot:\n%s", before, after)
			}
			if _, err := os.Lstat(filepath.Join(root, "installer-state.json")); !os.IsNotExist(err) {
				t.Fatalf("canonical state was created for invalid legacy content: %v", err)
			}
		})
	}
}

func TestSystemUserNamespaceMigrationRevalidatesLegacyStateBeforeRename(t *testing.T) {
	tests := []struct {
		name    string
		replace bool
	}{
		{name: "content edit"},
		{name: "inode replacement", replace: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			legacyState := filepath.Join(root, "wahrwelt", "state.json")
			originalPayload := systemMigrationStatePayload(7)
			mustWriteSystemMigrationFixture(t, legacyState, originalPayload)

			readyR, readyW, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer readyR.Close()
			continueR, continueW, err := os.Pipe()
			if err != nil {
				_ = readyW.Close()
				t.Fatal(err)
			}
			defer continueW.Close()

			cmd := exec.Command("python3", "-I", "-S", systemUserNamespaceHelper, "migrate-stage", root)
			cmd.Env = append(
				os.Environ(),
				"WAHRWELT_TEST_LEGACY_STATE_VALIDATED_READY_FD=3",
				"WAHRWELT_TEST_LEGACY_STATE_VALIDATED_CONTINUE_FD=4",
			)
			cmd.ExtraFiles = []*os.File{readyW, continueR}
			var output bytes.Buffer
			cmd.Stdout = &output
			cmd.Stderr = &output
			if err := cmd.Start(); err != nil {
				_ = readyW.Close()
				_ = continueR.Close()
				t.Fatal(err)
			}
			_ = readyW.Close()
			_ = continueR.Close()
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			finished := false
			defer func() {
				if finished {
					return
				}
				_, _ = continueW.Write([]byte{'1'})
				_ = cmd.Process.Kill()
				<-done
			}()

			marker := make([]byte, len("ready\n"))
			if err := readyR.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, err := io.ReadFull(readyR, marker); err != nil || string(marker) != "ready\n" {
				t.Fatalf("state validation barrier failed: marker=%q err=%v\n%s", marker, err, output.String())
			}

			racedPayload := systemMigrationStatePayload(6)
			if test.replace {
				retained := legacyState + ".before-race"
				if err := os.Rename(legacyState, retained); err != nil {
					t.Fatal(err)
				}
				mustWriteSystemMigrationFixture(t, legacyState, racedPayload)
			} else {
				mustWriteSystemMigrationFixture(t, legacyState, racedPayload)
			}
			if _, err := continueW.Write([]byte{'1'}); err != nil {
				t.Fatal(err)
			}
			waitErr := <-done
			finished = true
			var exitError *exec.ExitError
			if !errors.As(waitErr, &exitError) || exitError.ExitCode() != 2 {
				t.Fatalf("raced legacy state must fail closed: err=%v\n%s", waitErr, output.String())
			}
			if !strings.Contains(output.String(), "ownership collision") ||
				!strings.Contains(output.String(), "changed after validation") {
				t.Fatalf("raced state rejection was unclear:\n%s", output.String())
			}
			if got := mustReadSystemMigrationFixture(t, legacyState); got != racedPayload {
				t.Fatalf("raced legacy state changed: got %q want %q", got, racedPayload)
			}
			if _, err := os.Lstat(filepath.Join(root, "installer-state.json")); !os.IsNotExist(err) {
				t.Fatalf("canonical state was created after race: %v", err)
			}
		})
	}
}

func TestSystemUserNamespaceMigrationRefusesCanonicalTargetRace(t *testing.T) {
	root := t.TempDir()
	legacyState := filepath.Join(root, "mysetup", "state.json")
	originalPayload := systemMigrationStatePayload(7)
	mustWriteSystemMigrationFixture(t, legacyState, originalPayload)

	readyR, readyW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readyR.Close()
	continueR, continueW, err := os.Pipe()
	if err != nil {
		_ = readyW.Close()
		t.Fatal(err)
	}
	defer continueW.Close()

	cmd := exec.Command("python3", "-I", "-S", systemUserNamespaceHelper, "migrate-stage", root)
	cmd.Env = append(
		os.Environ(),
		"WAHRWELT_TEST_LEGACY_STATE_VALIDATED_READY_FD=3",
		"WAHRWELT_TEST_LEGACY_STATE_VALIDATED_CONTINUE_FD=4",
	)
	cmd.ExtraFiles = []*os.File{readyW, continueR}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		_ = readyW.Close()
		_ = continueR.Close()
		t.Fatal(err)
	}
	_ = readyW.Close()
	_ = continueR.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	defer func() {
		if finished {
			return
		}
		_, _ = continueW.Write([]byte{'1'})
		_ = cmd.Process.Kill()
		<-done
	}()

	marker := make([]byte, len("ready\n"))
	if err := readyR.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(readyR, marker); err != nil || string(marker) != "ready\n" {
		t.Fatalf("state validation barrier failed: marker=%q err=%v\n%s", marker, err, output.String())
	}

	canonicalState := filepath.Join(root, "installer-state.json")
	unknownCanonical := "unknown canonical winner\n"
	mustWriteSystemMigrationFixture(t, canonicalState, unknownCanonical)
	if _, err := continueW.Write([]byte{'1'}); err != nil {
		t.Fatal(err)
	}
	waitErr := <-done
	finished = true
	var exitError *exec.ExitError
	if !errors.As(waitErr, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("canonical target race must fail closed: err=%v\n%s", waitErr, output.String())
	}
	if !strings.Contains(output.String(), "canonical installer state appeared") {
		t.Fatalf("canonical target race rejection was unclear:\n%s", output.String())
	}
	if got := mustReadSystemMigrationFixture(t, legacyState); got != originalPayload {
		t.Fatalf("legacy state changed after target race")
	}
	if got := mustReadSystemMigrationFixture(t, canonicalState); got != unknownCanonical {
		t.Fatalf("canonical race winner changed: got %q want %q", got, unknownCanonical)
	}
}

func TestSystemUserNamespaceMigrationPreservesPostPublishCanonicalReplacement(t *testing.T) {
	root := t.TempDir()
	legacyState := filepath.Join(root, "wahrwelt", "state.json")
	originalPayload := systemMigrationStatePayload(7)
	mustWriteSystemMigrationFixture(t, legacyState, originalPayload)

	readyR, readyW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readyR.Close()
	continueR, continueW, err := os.Pipe()
	if err != nil {
		_ = readyW.Close()
		t.Fatal(err)
	}
	defer continueW.Close()

	cmd := exec.Command("python3", "-I", "-S", systemUserNamespaceHelper, "migrate-stage", root)
	cmd.Env = append(
		os.Environ(),
		"WAHRWELT_TEST_LEGACY_STATE_PUBLISHED_READY_FD=3",
		"WAHRWELT_TEST_LEGACY_STATE_PUBLISHED_CONTINUE_FD=4",
	)
	cmd.ExtraFiles = []*os.File{readyW, continueR}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		_ = readyW.Close()
		_ = continueR.Close()
		t.Fatal(err)
	}
	_ = readyW.Close()
	_ = continueR.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	defer func() {
		if finished {
			return
		}
		_, _ = continueW.Write([]byte{'1'})
		_ = cmd.Process.Kill()
		<-done
	}()

	marker := make([]byte, len("ready\n"))
	if err := readyR.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(readyR, marker); err != nil || string(marker) != "ready\n" {
		t.Fatalf("state publication barrier failed: marker=%q err=%v\n%s", marker, err, output.String())
	}

	canonicalState := filepath.Join(root, "installer-state.json")
	recoveryState := filepath.Join(root, "published-state.before-race")
	if err := os.Rename(canonicalState, recoveryState); err != nil {
		t.Fatal(err)
	}
	unknownCanonical := "unknown post-publish winner\n"
	mustWriteSystemMigrationFixture(t, canonicalState, unknownCanonical)
	if _, err := continueW.Write([]byte{'1'}); err != nil {
		t.Fatal(err)
	}
	waitErr := <-done
	finished = true
	var exitError *exec.ExitError
	if !errors.As(waitErr, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("post-publish replacement must fail closed: err=%v\n%s", waitErr, output.String())
	}
	if !strings.Contains(output.String(), "replacement retained at") {
		t.Fatalf("post-publish replacement rejection was unclear:\n%s", output.String())
	}
	if got := mustReadSystemMigrationFixture(t, canonicalState); got != unknownCanonical {
		t.Fatalf("post-publish canonical winner changed: got %q want %q", got, unknownCanonical)
	}
	if got := mustReadSystemMigrationFixture(t, recoveryState); got != originalPayload {
		t.Fatalf("published state recovery changed")
	}
	if _, err := os.Lstat(legacyState); !os.IsNotExist(err) {
		t.Fatalf("canonical winner was relocated to legacy path: %v", err)
	}
}

func TestSystemUserNamespaceMigrationRetainsSameInodePostPublishRecovery(t *testing.T) {
	root := t.TempDir()
	legacyState := filepath.Join(root, "wahrwelt", "state.json")
	mustWriteSystemMigrationFixture(t, legacyState, systemMigrationStatePayload(7))

	readyR, readyW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readyR.Close()
	continueR, continueW, err := os.Pipe()
	if err != nil {
		_ = readyW.Close()
		t.Fatal(err)
	}
	defer continueW.Close()
	cmd := exec.Command("python3", "-I", "-S", systemUserNamespaceHelper, "migrate-stage", root)
	cmd.Env = append(
		os.Environ(),
		"WAHRWELT_TEST_LEGACY_STATE_PUBLISHED_READY_FD=3",
		"WAHRWELT_TEST_LEGACY_STATE_PUBLISHED_CONTINUE_FD=4",
	)
	cmd.ExtraFiles = []*os.File{readyW, continueR}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		_ = readyW.Close()
		_ = continueR.Close()
		t.Fatal(err)
	}
	_ = readyW.Close()
	_ = continueR.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	defer func() {
		if finished {
			return
		}
		_, _ = continueW.Write([]byte{'1'})
		_ = cmd.Process.Kill()
		<-done
	}()

	marker := make([]byte, len("ready\n"))
	if err := readyR.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(readyR, marker); err != nil || string(marker) != "ready\n" {
		t.Fatalf("state publication barrier failed: marker=%q err=%v\n%s", marker, err, output.String())
	}

	canonicalState := filepath.Join(root, "installer-state.json")
	racedPayload := systemMigrationStatePayload(6)
	mustWriteSystemMigrationFixture(t, canonicalState, racedPayload)
	canonicalBefore, err := os.Lstat(canonicalState)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := continueW.Write([]byte{'1'}); err != nil {
		t.Fatal(err)
	}
	waitErr := <-done
	finished = true
	var exitError *exec.ExitError
	if !errors.As(waitErr, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("same-inode state mutation must fail closed: err=%v\n%s", waitErr, output.String())
	}
	if !strings.Contains(output.String(), "exact recovery retained at") {
		t.Fatalf("same-inode recovery was not reported:\n%s", output.String())
	}
	canonicalAfter, err := os.Lstat(canonicalState)
	if err != nil {
		t.Fatalf("canonical state disappeared: %v", err)
	}
	legacyAfter, err := os.Lstat(legacyState)
	if err != nil {
		t.Fatalf("legacy recovery was not retained: %v", err)
	}
	if !os.SameFile(canonicalBefore, canonicalAfter) || !os.SameFile(canonicalAfter, legacyAfter) {
		t.Fatalf("same-inode recovery did not retain the exact published inode")
	}
	if got := mustReadSystemMigrationFixture(t, canonicalState); got != racedPayload {
		t.Fatalf("canonical same-inode mutation changed: got %q want %q", got, racedPayload)
	}
}

func TestSystemUserNamespaceMigrationPreservesPostPublishParentReplacement(t *testing.T) {
	root := t.TempDir()
	legacyParent := filepath.Join(root, "mysetup")
	legacyState := filepath.Join(legacyParent, "state.json")
	mustWriteSystemMigrationFixture(t, legacyState, systemMigrationStatePayload(7))

	readyR, readyW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readyR.Close()
	continueR, continueW, err := os.Pipe()
	if err != nil {
		_ = readyW.Close()
		t.Fatal(err)
	}
	defer continueW.Close()
	cmd := exec.Command("python3", "-I", "-S", systemUserNamespaceHelper, "migrate-stage", root)
	cmd.Env = append(
		os.Environ(),
		"WAHRWELT_TEST_LEGACY_STATE_PUBLISHED_READY_FD=3",
		"WAHRWELT_TEST_LEGACY_STATE_PUBLISHED_CONTINUE_FD=4",
	)
	cmd.ExtraFiles = []*os.File{readyW, continueR}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		_ = readyW.Close()
		_ = continueR.Close()
		t.Fatal(err)
	}
	_ = readyW.Close()
	_ = continueR.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	defer func() {
		if finished {
			return
		}
		_, _ = continueW.Write([]byte{'1'})
		_ = cmd.Process.Kill()
		<-done
	}()

	marker := make([]byte, len("ready\n"))
	if err := readyR.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(readyR, marker); err != nil || string(marker) != "ready\n" {
		t.Fatalf("state publication barrier failed: marker=%q err=%v\n%s", marker, err, output.String())
	}

	retainedParent := legacyParent + ".before-race"
	if err := os.Rename(legacyParent, retainedParent); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(legacyParent, 0o755); err != nil {
		t.Fatal(err)
	}
	replacementInfo, err := os.Lstat(legacyParent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := continueW.Write([]byte{'1'}); err != nil {
		t.Fatal(err)
	}
	waitErr := <-done
	finished = true
	var exitError *exec.ExitError
	if !errors.As(waitErr, &exitError) || exitError.ExitCode() != 2 {
		t.Fatalf("parent replacement must fail closed: err=%v\n%s", waitErr, output.String())
	}
	if !strings.Contains(output.String(), "legacy state parent changed before quarantine") {
		t.Fatalf("parent replacement collision was unclear:\n%s", output.String())
	}
	afterInfo, err := os.Lstat(legacyParent)
	if err != nil {
		t.Fatalf("replacement parent was removed: %v", err)
	}
	if !os.SameFile(replacementInfo, afterInfo) {
		t.Fatalf("replacement parent identity changed")
	}
	if _, err := os.Lstat(retainedParent); err != nil {
		t.Fatalf("original empty parent recovery changed: %v", err)
	}
}

func TestSystemUserNamespaceMigrationMovesLegacyMySetupState(t *testing.T) {
	root := t.TempDir()
	legacyState := filepath.Join(root, "mysetup", "state.json")
	compatibility := filepath.Join(root, "mysetup", "compatibility.txt")
	statePayload := systemMigrationStatePayload(0)
	mustWriteSystemMigrationFixture(t, legacyState, statePayload)
	mustWriteSystemMigrationFixture(t, compatibility, "keep compatibility\n")

	runSystemUserNamespaceHelper(t, 0, "needs-migration", root)
	runSystemUserNamespaceHelper(t, 0, "migrate-stage", root)
	runSystemUserNamespaceHelper(t, 0, "validate-migrated", root)

	if got := mustReadSystemMigrationFixture(t, filepath.Join(root, "installer-state.json")); got != statePayload {
		t.Fatalf("migrated MySetup installer state = %q", got)
	}
	if _, err := os.Lstat(legacyState); !os.IsNotExist(err) {
		t.Fatalf("legacy MySetup state was not removed: %v", err)
	}
	if got := mustReadSystemMigrationFixture(t, compatibility); got != "keep compatibility\n" {
		t.Fatalf("MySetup compatibility sibling changed: %q", got)
	}
}

func TestSystemUserNamespaceMigrationQuarantinesExactEmptyMySetupStateParent(t *testing.T) {
	root := t.TempDir()
	statePayload := systemMigrationStatePayload(7)
	legacyParent := filepath.Join(root, "mysetup")
	mustWriteSystemMigrationFixture(t, filepath.Join(legacyParent, "state.json"), statePayload)
	parentBefore, err := os.Lstat(legacyParent)
	if err != nil {
		t.Fatal(err)
	}

	runSystemUserNamespaceHelper(t, 0, "migrate-stage", root)

	if _, err := os.Lstat(legacyParent); !os.IsNotExist(err) {
		t.Fatalf("exact empty MySetup state parent remains at its old path: %v", err)
	}
	quarantine := findSystemMigrationParentQuarantine(t, root, "mysetup", parentBefore)
	if entries, err := os.ReadDir(quarantine); err != nil || len(entries) != 0 {
		t.Fatalf("quarantined MySetup parent is not empty: entries=%v err=%v", entries, err)
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(root, "installer-state.json")); got != statePayload {
		t.Fatalf("migrated installer state = %q", got)
	}
}

func TestSystemUserNamespaceMigrationFreshTreeIsNoOp(t *testing.T) {
	root := t.TempDir()
	mustWriteSystemMigrationFixture(t, filepath.Join(root, "configuration.nix"), "imports = [ ./user ];\n")
	before := mustReadSystemMigrationFixture(t, filepath.Join(root, "configuration.nix"))

	runSystemUserNamespaceHelper(t, 1, "needs-migration", root)
	runSystemUserNamespaceHelper(t, 0, "migrate-stage", root)

	if got := mustReadSystemMigrationFixture(t, filepath.Join(root, "configuration.nix")); got != before {
		t.Fatalf("fresh tree changed: got %q want %q", got, before)
	}
}

func TestSystemUserNamespaceMigrationFailsClosedOnCollisions(t *testing.T) {
	tests := map[string]func(*testing.T, string){
		"coexisting user directories": func(t *testing.T, root string) {
			t.Helper()
			mustWriteSystemMigrationFixture(t, filepath.Join(root, "private", "legacy.nix"), "legacy\n")
			mustWriteSystemMigrationFixture(t, filepath.Join(root, "user", "canonical.nix"), "canonical\n")
		},
		"coexisting state files": func(t *testing.T, root string) {
			t.Helper()
			mustWriteSystemMigrationFixture(t, filepath.Join(root, "wahrwelt", "state.json"), "legacy\n")
			mustWriteSystemMigrationFixture(t, filepath.Join(root, "installer-state.json"), "canonical\n")
		},
		"coexisting legacy state sources": func(t *testing.T, root string) {
			t.Helper()
			mustWriteSystemMigrationFixture(t, filepath.Join(root, "wahrwelt", "state.json"), "wahrwelt\n")
			mustWriteSystemMigrationFixture(t, filepath.Join(root, "mysetup", "state.json"), "mysetup\n")
		},
		"MySetup state and canonical state": func(t *testing.T, root string) {
			t.Helper()
			mustWriteSystemMigrationFixture(t, filepath.Join(root, "mysetup", "state.json"), "legacy\n")
			mustWriteSystemMigrationFixture(t, filepath.Join(root, "installer-state.json"), "canonical\n")
		},
		"legacy user symlink": func(t *testing.T, root string) {
			t.Helper()
			if err := os.Symlink("missing-user", filepath.Join(root, "private")); err != nil {
				t.Fatal(err)
			}
		},
		"canonical state symlink": func(t *testing.T, root string) {
			t.Helper()
			if err := os.Symlink("missing-state", filepath.Join(root, "installer-state.json")); err != nil {
				t.Fatal(err)
			}
		},
		"canonical user regular file": func(t *testing.T, root string) {
			t.Helper()
			mustWriteSystemMigrationFixture(t, filepath.Join(root, "user"), "unknown\n")
		},
		"legacy state fifo": func(t *testing.T, root string) {
			t.Helper()
			parent := filepath.Join(root, "wahrwelt")
			if err := os.Mkdir(parent, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := unix.Mkfifo(filepath.Join(parent, "state.json"), 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"legacy state parent symlink": func(t *testing.T, root string) {
			t.Helper()
			outside := t.TempDir()
			mustWriteSystemMigrationFixture(t, filepath.Join(outside, "state.json"), "outside\n")
			if err := os.Symlink(outside, filepath.Join(root, "wahrwelt")); err != nil {
				t.Fatal(err)
			}
		},
		"MySetup state fifo": func(t *testing.T, root string) {
			t.Helper()
			parent := filepath.Join(root, "mysetup")
			if err := os.Mkdir(parent, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := unix.Mkfifo(filepath.Join(parent, "state.json"), 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"MySetup state parent symlink": func(t *testing.T, root string) {
			t.Helper()
			outside := t.TempDir()
			mustWriteSystemMigrationFixture(t, filepath.Join(outside, "state.json"), "outside\n")
			if err := os.Symlink(outside, filepath.Join(root, "mysetup")); err != nil {
				t.Fatal(err)
			}
		},
	}

	for name, arrange := range tests {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			arrange(t, root)
			before := snapshotSystemMigrationTree(t, root)

			runSystemUserNamespaceHelper(t, 2, "migrate-stage", root)

			if after := snapshotSystemMigrationTree(t, root); after != before {
				t.Fatalf("collision changed the tree:\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}
}

func TestSystemUserNamespacePrecommitRejectsLiveCollision(t *testing.T) {
	live := t.TempDir()
	stage := t.TempDir()
	for _, root := range []string{live, stage} {
		mustWriteSystemMigrationFixture(t, filepath.Join(root, "private", "custom.nix"), "custom\n")
		mustWriteSystemMigrationFixture(t, filepath.Join(root, "wahrwelt", "state.json"), systemMigrationStatePayload(7))
	}
	snapshot := filepath.Join(t.TempDir(), "namespace.json")
	mustWriteSystemMigrationFixture(t, snapshot, "")
	runSystemUserNamespaceHelper(t, 0, "snapshot-live", live, snapshot)
	runSystemUserNamespaceHelper(t, 0, "migrate-stage", stage)
	mustWriteSystemMigrationFixture(t, filepath.Join(live, "user", "raced.nix"), "unknown\n")
	before := snapshotSystemMigrationTree(t, live)

	runSystemUserNamespaceHelper(t, 2, "precommit", live, stage, snapshot)

	if after := snapshotSystemMigrationTree(t, live); after != before {
		t.Fatalf("precommit collision check changed live data:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestSystemUserNamespacePrecommitRejectsConcurrentNixEdit(t *testing.T) {
	live := t.TempDir()
	stage := t.TempDir()
	for _, root := range []string{live, stage} {
		mustWriteSystemMigrationFixture(t, filepath.Join(root, "configuration.nix"), "imports = [ ./private ];\n")
		mustWriteSystemMigrationFixture(t, filepath.Join(root, "private", "custom.nix"), "custom\n")
	}
	snapshot := filepath.Join(t.TempDir(), "namespace.json")
	mustWriteSystemMigrationFixture(t, snapshot, "")
	runSystemUserNamespaceHelper(t, 0, "snapshot-live", live, snapshot)
	runSystemUserNamespaceHelper(t, 0, "migrate-stage", stage)
	mustWriteSystemMigrationFixture(t, filepath.Join(live, "configuration.nix"), "# concurrent owner edit\nimports = [ ./private ];\n")
	before := snapshotSystemMigrationTree(t, live)

	runSystemUserNamespaceHelper(t, 2, "precommit", live, stage, snapshot)

	if after := snapshotSystemMigrationTree(t, live); after != before {
		t.Fatalf("precommit check changed concurrent owner data:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestSystemUserNamespacePrecommitRejectsMetadataOnlyChanges(t *testing.T) {
	tests := map[string]func(*testing.T, string){
		"hardlink count": func(t *testing.T, path string) {
			t.Helper()
			outside := filepath.Join(t.TempDir(), "outside-link")
			if err := os.Link(path, outside); err != nil {
				t.Fatal(err)
			}
		},
		"extended attribute": func(t *testing.T, path string) {
			t.Helper()
			if err := unix.Setxattr(path, "user.wahrwelt-test", []byte("changed"), 0); err != nil {
				if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.EPERM) {
					t.Skipf("user xattrs are unavailable: %v", err)
				}
				t.Fatal(err)
			}
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			live := t.TempDir()
			stage := t.TempDir()
			for _, root := range []string{live, stage} {
				mustWriteSystemMigrationFixture(t, filepath.Join(root, "configuration.nix"), "imports = [ ./private ];\n")
				mustWriteSystemMigrationFixture(t, filepath.Join(root, "private", "custom.nix"), "custom\n")
			}
			snapshot := filepath.Join(t.TempDir(), "namespace.json")
			mustWriteSystemMigrationFixture(t, snapshot, "")
			runSystemUserNamespaceHelper(t, 0, "snapshot-live", live, snapshot)
			runSystemUserNamespaceHelper(t, 0, "migrate-stage", stage)

			liveFile := filepath.Join(live, "private", "custom.nix")
			mutate(t, liveFile)
			runSystemUserNamespaceHelper(t, 2, "precommit", live, stage, snapshot)

			if got := mustReadSystemMigrationFixture(t, liveFile); got != "custom\n" {
				t.Fatalf("metadata race changed live content: %q", got)
			}
		})
	}
}

func TestSystemUserNamespaceCandidateSyncPreservesHardlinksAndXattrs(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync is unavailable")
	}
	source := t.TempDir()
	candidate := t.TempDir()
	first := filepath.Join(source, "first")
	second := filepath.Join(source, "second")
	mustWriteSystemMigrationFixture(t, first, "preserved\n")
	if err := os.Link(first, second); err != nil {
		t.Fatal(err)
	}
	if err := unix.Setxattr(first, "user.wahrwelt-test", []byte("preserved"), 0); err != nil {
		if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.EPERM) {
			t.Skipf("user xattrs are unavailable: %v", err)
		}
		t.Fatal(err)
	}

	output, err := exec.Command("rsync", "-aHAX", "--delete", source+"/", candidate+"/").CombinedOutput()
	if err != nil {
		t.Fatalf("candidate rsync failed: %v\n%s", err, output)
	}
	firstInfo, err := os.Stat(filepath.Join(candidate, "first"))
	if err != nil {
		t.Fatal(err)
	}
	secondInfo, err := os.Stat(filepath.Join(candidate, "second"))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(firstInfo, secondInfo) {
		t.Fatal("candidate sync did not preserve hardlink identity")
	}
	value := make([]byte, 64)
	size, err := unix.Getxattr(filepath.Join(candidate, "first"), "user.wahrwelt-test", value)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(value[:size]); got != "preserved" {
		t.Fatalf("candidate xattr = %q, want preserved", got)
	}
}

func TestSystemUserNamespaceMigrationServiceContract(t *testing.T) {
	module := readContractFile(t, "../../../NixOS/system/wahrwelt-migration.nix")
	for _, want := range []string{
		`v1ToV2NamespaceSource = pkgs.writeText`,
		`builtins.readFile ./migrations/v1_to_v2/user-namespace.py`,
		`v1ToV2BrandSource = pkgs.writeText`,
		`builtins.readFile ./migrations/v1_to_v2/brand.py`,
		`password-evidence "$destination" "$password_hash_target"`,
		`name = "wahrwelt-v1-to-v2-migration"`,
		`systemd.services.wahrwelt-v1-to-v2-migration`,
		`fs_helper=${lib.escapeShellArg "${inputs.wahrwelt.packages.${pkgs.stdenv.hostPlatform.system}.wahrwelt}/bin/wahrwelt-fs-helper"}`,
		`"completion-status" "$work_root_fd"`,
		`"publish-completion" "$work_root_fd"`,
		`"needs-migration"`,
		`"migrate-stage"`,
		`"snapshot-live"`,
		`"precommit"`,
		`"publish"`,
		`previous live tree retained at $candidate`,
		`"create-owned-temp" "$kind" "$parent_pinned" "$prefix"`,
		`"find-owned-temp" "$parent_pinned" "$identity"`,
		`creation_barrier`,
		`pin_created_directory`,
		`pin_created_file`,
		`require_private_created_directory`,
		`namespace_snapshot="/proc/self/fd/$namespace_snapshot_fd"`,
		`restartTriggers = [`,
		`v1ToV2NamespaceSource`,
		`v1ToV2BrandSource`,
		`ConditionPathExists = [`,
		`"|${destination}/private"`,
		`"|${destination}/wahrwelt/state.json"`,
		`"|${destination}/mysetup/state.json"`,
		`"|${destination}/hashed-password.nix"`,
		`"|${destination}/hosts/NixOS/hashed-password.nix"`,
		`rsync -aHAX "$stage_pinned/" "$candidate_pinned/"`,
		`--no-overwrite-dir --sparse -xpf -`,
		`--offline`,
		`exec 9<"$work_root"`,
		`work_root_fd="/proc/self/fd/9"`,
		`destination_pinned="/proc/self/fd/$destination_fd"`,
		`flock 9`,
		`stage_pinned="/proc/self/fd/$stage_fd"`,
		`tar --acls --xattrs --xattrs-include='*' --numeric-owner --no-recursion -cpf - .`,
		`"$fs_helper" remove-migration-temporary \
        --kind staging --name "$stage_name" --expected "$stage_token"`,
		`"$fs_helper" remove-migration-temporary \
        --kind namespace --name "$namespace_snapshot_name" --expected "$namespace_snapshot_token"`,
	} {
		if !strings.Contains(module, want) {
			t.Fatalf("system migration service missing %q\n%s", want, module)
		}
	}
	for _, forbidden := range []string{
		`system/user-namespace-migration.py`,
		`systemd.services.wahrwelt-migration`,
		`"|${destination}/flake.nix"`,
		`rsync -a --delete "$stage/" "$destination/"`,
		`rsync -a --delete "$rollback/" "$destination/"`,
		`[ "$status" -eq 0 ] || [ "$publish_started" -eq 0 ]`,
		`-name "$(basename "$destination").bak.*"`,
		`rm -rf "$candidate"`,
		`lock="$work_root/lock"`,
		`exec 9>"$lock"`,
		`migration-v2.done`,
		`touch "$marker"`,
		`mktemp -d`,
		`mktemp "$work_root_fd/namespace.XXXXXX"`,
		`rsync -aHAX --delete`,
		`rm -rf -- "$stage_pinned"`,
		`rm -f "$namespace_snapshot"`,
		`cp -a "$destination/." "$stage_pinned/"`,
		`nix flake update`,
		`grep -RIl`,
		`-iname '*mysetup*'`,
	} {
		if strings.Contains(module, forbidden) {
			t.Fatalf("live tree must never be a broad rsync destination: found %q\n%s", forbidden, module)
		}
	}
	candidateRsync := strings.Index(module, `rsync -aHAX "$stage_pinned/" "$candidate_pinned/"`)
	candidateCreate := strings.Index(module, `create_owned_temp \
        directory "$destination_parent_pinned"`)
	build := strings.Index(module, `nix build \`)
	publish := strings.Index(module, `"publish" "$destination" "$candidate" "$namespace_snapshot"`)
	if build < 0 || candidateCreate < build || candidateRsync < candidateCreate || publish < candidateRsync {
		t.Fatalf("rsync must prepare only the same-parent candidate before atomic publish\n%s", module)
	}

	base := readContractFile(t, "../../../NixOS/profiles/base.nix")
	if !strings.Contains(base, "../system/wahrwelt-migration.nix") {
		t.Fatalf("base profile must activate system namespace migration\n%s", base)
	}
	fish := readContractFile(t, "../../../NixOS/home/programs/fish.nix")
	if !strings.Contains(fish, "nixos-rebuild switch") {
		t.Fatalf("normal nixos-update must activate the migration service\n%s", fish)
	}
	if strings.Contains(module, "<<EOF") || strings.Contains(module, "<<'EOF'") || strings.Contains(module, "<<\"EOF\"") {
		t.Fatalf("system migration module must not generate helpers with heredocs\n%s", module)
	}
	helper := readContractFile(t, systemUserNamespaceHelper)
	if !strings.Contains(helper, "/proc/self/mountinfo") {
		t.Fatalf("system migration helper must reject mountpoint ownership collisions\n%s", helper)
	}
}

func TestBootThemeUsesOnlyCanonicalWahrweltNamespace(t *testing.T) {
	module := readContractFile(t, "../../../NixOS/lib/boot-theme.nix")
	if !strings.Contains(module, `dir = "${homeDirectory}/.config/wahrwelt/boot-theme";`) {
		t.Fatalf("boot theme must resolve only the canonical namespace\n%s", module)
	}
	for _, forbidden := range []string{
		`.config/mysetup/boot-theme`,
		`legacyDir`,
	} {
		if strings.Contains(module, forbidden) {
			t.Fatalf("boot theme retained v1 runtime fallback %q\n%s", forbidden, module)
		}
	}
}

func TestSystemMigrationServiceMigratesNamespaceWithoutRewritingCanonicalCompatibilityAliases(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "nixos")
	writeSystemMigrationServiceFixture(t, destination, "# Generated by Wahrwelt installer.", true)
	beforeAlias := mustReadSystemMigrationFixture(t, filepath.Join(destination, "mysetup", "compatibility.txt"))
	beforeText := mustReadSystemMigrationFixture(t, filepath.Join(destination, "mysetup-compat.nix"))

	output, err := runRenderedSystemMigrationService(t, destination)
	if err != nil {
		t.Fatalf("canonical namespace migration failed: %v\n%s", err, output)
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(destination, "configuration.nix")); got != "imports = [ ./user ./mysetup-compat.nix ];\n" {
		t.Fatalf("namespace path was not migrated independently: %q", got)
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(destination, "user", "custom.nix")); got != "custom\n" {
		t.Fatalf("private namespace content was not migrated: %q", got)
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(destination, "installer-state.json")); got != systemMigrationStatePayload(7) {
		t.Fatalf("legacy state was not migrated: %q", got)
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(destination, "mysetup", "compatibility.txt")); got != beforeAlias {
		t.Fatalf("canonical compatibility alias changed: got %q want %q", got, beforeAlias)
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(destination, "mysetup-compat.nix")); got != beforeText {
		t.Fatalf("canonical compatibility text changed: got %q want %q", got, beforeText)
	}
	if _, err := os.Lstat(filepath.Join(destination, "wahrwelt", "canonical.txt")); err != nil {
		t.Fatalf("canonical Wahrwelt tree changed: %v", err)
	}
}

func TestSystemMigrationServicePreservesDestinationRootMetadata(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "nixos")
	writeSystemMigrationServiceFixture(t, destination, "# Generated by Wahrwelt installer.", true)
	if err := os.Chmod(destination, 0o751); err != nil {
		t.Fatal(err)
	}
	xattrSupported := true
	if err := unix.Setxattr(destination, "user.wahrwelt-root", []byte("preserved"), 0); err != nil {
		if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.EPERM) {
			xattrSupported = false
		} else {
			t.Fatal(err)
		}
	}
	before, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	beforeStat, ok := before.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("destination stat does not expose Unix metadata")
	}

	output, err := runRenderedSystemMigrationService(t, destination)
	if err != nil {
		t.Fatalf("metadata-preserving namespace migration failed: %v\n%s", err, output)
	}
	after, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	afterStat, ok := after.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("published destination stat does not expose Unix metadata")
	}
	if after.Mode().Perm() != before.Mode().Perm() ||
		afterStat.Uid != beforeStat.Uid || afterStat.Gid != beforeStat.Gid {
		t.Fatalf(
			"published root metadata = mode %04o uid %d gid %d, want mode %04o uid %d gid %d",
			after.Mode().Perm(), afterStat.Uid, afterStat.Gid,
			before.Mode().Perm(), beforeStat.Uid, beforeStat.Gid,
		)
	}
	if xattrSupported {
		value := make([]byte, 64)
		size, getErr := unix.Getxattr(destination, "user.wahrwelt-root", value)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if got := string(value[:size]); got != "preserved" {
			t.Fatalf("published root xattr = %q, want preserved", got)
		}
	}
}

func TestSystemMigrationServicePreservesUnownedLegacyState(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "nixos")
	writeSystemMigrationServiceFixture(t, destination, "# Generated by Wahrwelt installer.", true)
	mustWriteSystemMigrationFixture(
		t,
		filepath.Join(destination, "wahrwelt", "state.json"),
		"human-owned state\n",
	)
	before := snapshotSystemMigrationTree(t, destination)

	output, err := runRenderedSystemMigrationService(t, destination)
	if err == nil {
		t.Fatalf("service accepted unowned legacy state:\n%s", output)
	}
	if !strings.Contains(output, "ownership collision") {
		t.Fatalf("service rejection did not identify an ownership collision:\n%s", output)
	}
	if after := snapshotSystemMigrationTree(t, destination); after != before {
		t.Fatalf("service changed the tree after state collision:\nwant:\n%s\ngot:\n%s\n%s", before, after, output)
	}
}

func TestSystemMigrationServiceCanonicalCompatibilityTreeWithoutNamespaceIsNoOp(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "nixos")
	writeSystemMigrationServiceFixture(t, destination, "# Generated by Wahrwelt installer.", false)
	before := snapshotSystemMigrationTree(t, destination)

	output, err := runRenderedSystemMigrationService(t, destination)
	if err != nil {
		t.Fatalf("canonical compatibility no-op failed: %v\n%s", err, output)
	}
	if after := snapshotSystemMigrationTree(t, destination); after != before {
		t.Fatalf("canonical compatibility tree changed without namespace migration:\nwant:\n%s\ngot:\n%s\n%s", before, after, output)
	}
}

func TestSystemMigrationServiceFreshCanonicalTreeDoesNotLoadV1Recognizers(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "nixos")
	writeSystemMigrationServiceFixture(t, destination, "# Generated by Wahrwelt installer.", false)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	harness := prepareRenderedSystemMigrationService(ctx, t, destination)
	if err := harness.cmd.Run(); err != nil {
		t.Fatalf("fresh canonical migration prefilter failed: %v\n%s", err, harness.output.String())
	}
	if _, err := os.Lstat(harness.workRoot); !os.IsNotExist(err) {
		t.Fatalf("fresh canonical tree entered the v1 migration runtime: %v\n%s", err, harness.output.String())
	}
}

func TestSystemMigrationServiceUnknownPasswordModuleIsNotV1Evidence(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "nixos")
	writeSystemMigrationServiceFixture(t, destination, "# Generated by Wahrwelt installer.", false)
	module := filepath.Join(destination, "hashed-password.nix")
	mustWriteSystemMigrationFixture(t, module, "user-owned password module\n")
	if err := os.Chmod(module, 0o600); err != nil {
		t.Fatal(err)
	}
	before := snapshotSystemMigrationTree(t, destination)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	harness := prepareRenderedSystemMigrationService(ctx, t, destination)
	if err := harness.cmd.Run(); err != nil {
		t.Fatalf("unknown password module prefilter failed: %v\n%s", err, harness.output.String())
	}
	if _, err := os.Lstat(harness.workRoot); !os.IsNotExist(err) {
		t.Fatalf("unknown password module entered the v1 migration runtime: %v\n%s", err, harness.output.String())
	}
	if after := snapshotSystemMigrationTree(t, destination); after != before {
		t.Fatalf("unknown password module was changed by the v1 prefilter:\nwant:\n%s\ngot:\n%s", before, after)
	}
}

func TestSystemMigrationServiceFunctionalPasswordStubWithExternalHashIsNoOp(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "nixos")
	writeSystemMigrationServiceFixture(t, destination, "# Generated by Wahrwelt installer.", false)
	module := filepath.Join(destination, "hashed-password.nix")
	const stub = `{ config, ... }:

{
  users.users.${config.mysetup.user.username}.hashedPasswordFile = "/etc/wahrwelt/hashed-password";
}
`
	mustWriteSystemMigrationFixture(t, module, stub)
	if err := os.Chmod(module, 0o644); err != nil {
		t.Fatal(err)
	}
	externalParent := filepath.Join(root, "wahrwelt")
	if err := os.MkdirAll(externalParent, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(externalParent, 0o700); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(externalParent, "hashed-password")
	passwordHash := "$6$rounds=5000$testsalt$" + strings.Repeat("A", 86)
	mustWriteSystemMigrationFixture(t, external, passwordHash+"\n")
	if err := os.Chmod(external, 0o600); err != nil {
		t.Fatal(err)
	}
	beforeTree := snapshotSystemMigrationTree(t, destination)
	beforeExternal := mustReadSystemMigrationFixture(t, external)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	harness := prepareRenderedSystemMigrationService(ctx, t, destination)
	if err := os.MkdirAll(harness.workRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	completion := filepath.Join(harness.workRoot, "v1_to_v2.complete")
	mustWriteSystemMigrationFixture(t, completion, "wahrwelt-v1-to-v2-complete\n")
	if err := os.Chmod(completion, 0o600); err != nil {
		t.Fatal(err)
	}
	completionBefore, err := os.Stat(completion)
	if err != nil {
		t.Fatal(err)
	}
	if err := harness.cmd.Run(); err != nil {
		t.Fatalf("functional password stub no-op failed: %v\n%s", err, harness.output.String())
	}
	completionAfter, err := os.Stat(completion)
	if err != nil || !os.SameFile(completionBefore, completionAfter) || completionAfter.Mode().Perm() != 0o600 {
		t.Fatalf("completed password stub changed its completion marker: before=%v after=%v err=%v", completionBefore, completionAfter, err)
	}
	if entries, err := os.ReadDir(harness.workRoot); err != nil || len(entries) != 1 || entries[0].Name() != "v1_to_v2.complete" {
		t.Fatalf("completed password stub entered the v1 migration runtime: entries=%v err=%v\n%s", entries, err, harness.output.String())
	}
	if _, err := os.Lstat(filepath.Join(harness.testRoot, "fs-helper.log")); !os.IsNotExist(err) {
		t.Fatalf("completed password stub invoked cleanup helper: %v", err)
	}
	if after := snapshotSystemMigrationTree(t, destination); after != beforeTree {
		t.Fatalf("completed password stub changed the live tree:\nwant:\n%s\ngot:\n%s", beforeTree, after)
	}
	if got := mustReadSystemMigrationFixture(t, external); got != beforeExternal {
		t.Fatal("completed password stub changed the external password hash")
	}
}

func TestSystemMigrationServiceMigratesMySetupStateUnderCanonicalMarker(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "nixos")
	writeSystemMigrationServiceFixture(t, destination, "# Generated by Wahrwelt installer.", false)
	legacyState := filepath.Join(destination, "mysetup", "state.json")
	statePayload := systemMigrationStatePayload(0)
	mustWriteSystemMigrationFixture(t, legacyState, statePayload)

	output, err := runRenderedSystemMigrationService(t, destination)
	if err != nil {
		t.Fatalf("canonical-marker MySetup state migration failed: %v\n%s", err, output)
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(destination, "installer-state.json")); got != statePayload {
		t.Fatalf("migrated MySetup state = %q", got)
	}
	if _, statErr := os.Lstat(legacyState); !os.IsNotExist(statErr) {
		t.Fatalf("legacy MySetup state remains after migration: %v", statErr)
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(destination, "mysetup", "compatibility.txt")); got != "canonical mysetup compatibility alias\n" {
		t.Fatalf("MySetup compatibility sibling changed: %q", got)
	}
}

func TestSystemMigrationServiceLegacyMySetupMarkerStillRunsBrandMigration(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "nixos")
	writeSystemMigrationServiceFixture(t, destination, "# Generated by MySetup installer.", false)
	mustWriteSystemMigrationFixture(t, filepath.Join(destination, "mysetup-module.nix"), "# MySetup mysetup compatibility\n")

	output, err := runRenderedSystemMigrationService(t, destination)
	if err != nil {
		t.Fatalf("recognized legacy brand migration failed: %v\n%s", err, output)
	}
	if got := mustReadSystemMigrationFixture(t, filepath.Join(destination, "mysetup-module.nix")); got != "# MySetup mysetup compatibility\n" {
		t.Fatalf("supported compatibility module was rewritten: %q", got)
	}
	if _, err := os.Lstat(filepath.Join(destination, "wahrwelt-module.nix")); !os.IsNotExist(err) {
		t.Fatalf("migration invented a renamed user module: %v", err)
	}
	if flake := mustReadSystemMigrationFixture(t, filepath.Join(destination, "flake.nix")); !strings.Contains(flake, "# Generated by Wahrwelt installer.") {
		t.Fatalf("legacy installer marker was not migrated:\n%s", flake)
	}
	lock := mustReadSystemMigrationFixture(t, filepath.Join(destination, "flake.lock"))
	if strings.Contains(lock, `"mysetup"`) ||
		!strings.Contains(lock, `"wahrwelt"`) ||
		!strings.Contains(lock, `"rev": "0123456789abcdef0123456789abcdef01234567"`) ||
		!strings.Contains(lock, `"narHash": "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="`) ||
		strings.Count(lock, `"repo": "wahrwelt"`) != 2 ||
		!strings.Contains(lock, `"lastModified": 1234567890`) ||
		!strings.Contains(lock, `"ref": "main"`) ||
		strings.Count(lock, `"dir": "Linux/NixOS/presets/personal"`) != 2 {
		t.Fatalf("brand migration did not preserve the locked source identity:\n%s", lock)
	}
}

func TestSystemBrandMigrationRejectsLockSourceDriftBeforeWriting(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any, map[string]any)
	}{
		{
			name: "locked owner",
			mutate: func(locked, _ map[string]any) {
				locked["owner"] = "someone-else"
			},
		},
		{
			name: "locked repo",
			mutate: func(locked, _ map[string]any) {
				locked["repo"] = "wahrwelt"
			},
		},
		{
			name: "original repo",
			mutate: func(_, original map[string]any) {
				original["repo"] = "wahrwelt"
			},
		},
		{
			name: "locked dir",
			mutate: func(locked, _ map[string]any) {
				locked["dir"] = "Linux/NixOS/presets/developer"
			},
		},
		{
			name: "original ref",
			mutate: func(_, original map[string]any) {
				original["ref"] = "dev"
			},
		},
		{
			name: "missing lastModified",
			mutate: func(locked, _ map[string]any) {
				delete(locked, "lastModified")
			},
		},
		{
			name: "short rev",
			mutate: func(locked, _ map[string]any) {
				locked["rev"] = "fixture-rev"
			},
		},
		{
			name: "invalid narHash",
			mutate: func(locked, _ map[string]any) {
				locked["narHash"] = "sha256-fixture"
			},
		},
		{
			name: "unknown original field",
			mutate: func(_, original map[string]any) {
				original["unexpected"] = true
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			destination := filepath.Join(t.TempDir(), "nixos")
			writeSystemMigrationServiceFixture(t, destination, "# Generated by MySetup installer.", false)
			lockPath := filepath.Join(destination, "flake.lock")
			var lock map[string]any
			if err := json.Unmarshal([]byte(mustReadSystemMigrationFixture(t, lockPath)), &lock); err != nil {
				t.Fatal(err)
			}
			nodes := lock["nodes"].(map[string]any)
			node := nodes["mysetup"].(map[string]any)
			locked := node["locked"].(map[string]any)
			original := node["original"].(map[string]any)
			test.mutate(locked, original)
			payload, err := json.MarshalIndent(lock, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			mustWriteSystemMigrationFixture(t, lockPath, string(payload)+"\n")
			flakePath := filepath.Join(destination, "flake.nix")
			beforeFlake := mustReadSystemMigrationFixture(t, flakePath)
			beforeLock := mustReadSystemMigrationFixture(t, lockPath)

			output, commandErr := exec.Command(
				"python3", systemBrandMigrationHelper, "migrate", destination,
			).CombinedOutput()
			if commandErr == nil || !strings.Contains(string(output), "ownership collision") {
				t.Fatalf("invalid lock source was accepted: err=%v\n%s", commandErr, output)
			}
			if got := mustReadSystemMigrationFixture(t, flakePath); got != beforeFlake {
				t.Fatal("invalid lock source changed flake.nix before rejection")
			}
			if got := mustReadSystemMigrationFixture(t, lockPath); got != beforeLock {
				t.Fatal("invalid lock source changed flake.lock before rejection")
			}
		})
	}
}

func TestSystemMigrationServiceExternalizesGeneratedLegacyPassword(t *testing.T) {
	passwordHash := "$6$rounds=5000$testsalt$" + strings.Repeat("A", 86)
	tests := []struct {
		name            string
		marker          string
		moduleNamespace string
		moduleRelative  string
	}{
		{
			name:            "MySetup root module",
			marker:          "# Generated by MySetup installer.",
			moduleNamespace: "mysetup",
			moduleRelative:  "hashed-password.nix",
		},
		{
			name:            "Wahrwelt historical host module",
			marker:          "# Generated by Wahrwelt installer.",
			moduleNamespace: "wahrwelt",
			moduleRelative:  "hosts/NixOS/hashed-password.nix",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			destination := filepath.Join(root, "nixos")
			writeSystemMigrationServiceFixture(t, destination, test.marker, false)
			writeSystemMigrationLegacyPasswordFixture(
				t,
				destination,
				test.moduleRelative,
				test.moduleNamespace,
				passwordHash,
			)
			lockBefore := mustReadSystemMigrationFixture(t, filepath.Join(destination, "flake.lock"))

			output, err := runRenderedSystemMigrationService(t, destination)
			if err != nil {
				t.Fatalf("legacy password migration failed: %v\n%s", err, output)
			}
			if strings.Contains(output, passwordHash) {
				t.Fatal("legacy password migration leaked the hash in command output")
			}

			external := filepath.Join(root, "wahrwelt", "hashed-password")
			if got := mustReadSystemMigrationFixture(t, external); got != passwordHash+"\n" {
				t.Fatalf("external password payload = %q", got)
			}
			var externalStat unix.Stat_t
			if statErr := unix.Lstat(external, &externalStat); statErr != nil || externalStat.Mode&0o777 != 0o600 || externalStat.Nlink != 1 || externalStat.Uid != uint32(os.Geteuid()) {
				t.Fatalf("external password metadata is not private, owned, and singly linked: stat=%+v err=%v", externalStat, statErr)
			}

			flake := mustReadSystemMigrationFixture(t, filepath.Join(destination, "flake.nix"))
			if !strings.Contains(flake, "# Generated by Wahrwelt installer.") ||
				!strings.Contains(flake, "hashedPasswordFile =") ||
				!strings.Contains(flake, `"/etc/wahrwelt/hashed-password"`) ||
				strings.Contains(flake, "hashedPassword =") {
				t.Fatalf("legacy password wrapper was not migrated exactly:\n%s", flake)
			}
			marker := filepath.Join(destination, ".wahrwelt-password-hash-enabled")
			if info, statErr := os.Stat(marker); statErr != nil || info.Mode().Perm() != 0o644 || info.Size() != 0 {
				t.Fatalf("password marker metadata is invalid: info=%v err=%v", info, statErr)
			}
			if _, statErr := os.Lstat(filepath.Join(destination, test.moduleRelative)); !os.IsNotExist(statErr) {
				t.Fatalf("generated password module remains in migrated candidate: %v", statErr)
			}
			if strings.Contains(snapshotSystemMigrationTree(t, destination), passwordHash) {
				t.Fatal("migrated candidate still contains the password hash")
			}
			if test.marker == "# Generated by Wahrwelt installer." {
				if lockAfter := mustReadSystemMigrationFixture(t, filepath.Join(destination, "flake.lock")); lockAfter != lockBefore {
					t.Fatal("password-only migration changed the canonical lock file")
				}
			}
		})
	}
}

func TestSystemMigrationServiceNamespaceCollisionFailsWithoutBrandRewrite(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "nixos")
	writeSystemMigrationServiceFixture(t, destination, "# Generated by Wahrwelt installer.", true)
	mustWriteSystemMigrationFixture(t, filepath.Join(destination, "user", "winner.nix"), "canonical winner\n")
	before := snapshotSystemMigrationTree(t, destination)

	output, err := runRenderedSystemMigrationService(t, destination)
	if err == nil {
		t.Fatalf("namespace collision unexpectedly succeeded\n%s", output)
	}
	if after := snapshotSystemMigrationTree(t, destination); after != before {
		t.Fatalf("namespace collision changed live tree:\nwant:\n%s\ngot:\n%s\n%s", before, after, output)
	}
}

func TestSystemMigrationServiceUnknownFlakeMarkerFailsWhenNamespaceMigrationIsNeeded(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "nixos")
	writeSystemMigrationServiceFixture(t, destination, "# Generated by Unknown installer.", true)
	before := snapshotSystemMigrationTree(t, destination)

	output, err := runRenderedSystemMigrationService(t, destination)
	if err == nil || !strings.Contains(output, "unrecognized") {
		t.Fatalf("unknown marker namespace migration did not fail closed: err=%v\n%s", err, output)
	}
	if after := snapshotSystemMigrationTree(t, destination); after != before {
		t.Fatalf("unknown marker failure changed live tree:\nwant:\n%s\ngot:\n%s\n%s", before, after, output)
	}
}

func TestSystemMigrationServiceRejectsPostCreatorReplacement(t *testing.T) {
	tests := []struct {
		label     string
		kind      string
		readyEnv  string
		resumeEnv string
	}{
		{
			label:     "stage",
			kind:      "directory",
			readyEnv:  "WAHRWELT_TEST_MIGRATION_STAGE_CREATED_READY_FD",
			resumeEnv: "WAHRWELT_TEST_MIGRATION_STAGE_CREATED_CONTINUE_FD",
		},
		{
			label:     "snapshot",
			kind:      "file",
			readyEnv:  "WAHRWELT_TEST_MIGRATION_SNAPSHOT_CREATED_READY_FD",
			resumeEnv: "WAHRWELT_TEST_MIGRATION_SNAPSHOT_CREATED_CONTINUE_FD",
		},
		{
			label:     "candidate",
			kind:      "directory",
			readyEnv:  "WAHRWELT_TEST_MIGRATION_CANDIDATE_CREATED_READY_FD",
			resumeEnv: "WAHRWELT_TEST_MIGRATION_CANDIDATE_CREATED_CONTINUE_FD",
		},
	}

	for _, test := range tests {
		t.Run(test.label, func(t *testing.T) {
			root := t.TempDir()
			destination := filepath.Join(root, "nixos")
			writeSystemMigrationServiceFixture(t, destination, "# Generated by Wahrwelt installer.", true)
			liveBefore := snapshotSystemMigrationTree(t, destination)

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			harness := prepareRenderedSystemMigrationService(ctx, t, destination)
			readyR, readyW, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer readyR.Close()
			continueR, continueW, err := os.Pipe()
			if err != nil {
				_ = readyW.Close()
				t.Fatal(err)
			}
			defer continueW.Close()
			harness.cmd.Env = append(
				harness.cmd.Env,
				test.readyEnv+"=3",
				test.resumeEnv+"=4",
			)
			harness.cmd.ExtraFiles = []*os.File{readyW, continueR}
			if err := harness.cmd.Start(); err != nil {
				_ = readyW.Close()
				_ = continueR.Close()
				t.Fatal(err)
			}
			_ = readyW.Close()
			_ = continueR.Close()
			done := make(chan error, 1)
			go func() { done <- harness.cmd.Wait() }()
			finished := false
			t.Cleanup(func() {
				if finished {
					return
				}
				_ = harness.cmd.Process.Kill()
				<-done
			})

			lineResult := make(chan struct {
				line string
				err  error
			}, 1)
			go func() {
				line, err := bufio.NewReader(readyR).ReadString('\n')
				lineResult <- struct {
					line string
					err  error
				}{line: line, err: err}
			}()
			var creationLine string
			select {
			case result := <-lineResult:
				if result.err != nil {
					t.Fatalf("%s creator barrier failed: %v\n%s", test.label, result.err, harness.output.String())
				}
				creationLine = strings.TrimSuffix(result.line, "\n")
			case err := <-done:
				finished = true
				t.Fatalf("migration exited before %s creator barrier: %v\n%s", test.label, err, harness.output.String())
			case <-ctx.Done():
				t.Fatalf("timed out waiting for %s creator barrier\n%s", test.label, harness.output.String())
			}

			fields := strings.Split(creationLine, "\t")
			if len(fields) != 3 || fields[0] != test.label {
				t.Fatalf("unexpected %s creator barrier %q", test.label, creationLine)
			}
			name, identity := fields[1], fields[2]
			parent := harness.workRoot
			if test.label == "candidate" {
				parent = filepath.Dir(destination)
			}
			created := filepath.Join(parent, name)
			recovery := created + ".expected-recovery"
			if got := systemMigrationIdentity(t, created); got != identity {
				t.Fatalf("creator identity = %q, visible identity = %q", identity, got)
			}
			if err := os.Rename(created, recovery); err != nil {
				t.Fatal(err)
			}
			if test.kind == "directory" {
				mustWriteSystemMigrationFixture(t, filepath.Join(created, "unknown-winner"), "unknown winner\n")
			} else {
				mustWriteSystemMigrationFixture(t, created, "unknown winner\n")
			}
			if _, err := continueW.Write([]byte{'1'}); err != nil {
				t.Fatal(err)
			}

			select {
			case err := <-done:
				finished = true
				if err == nil {
					t.Fatalf("%s replacement unexpectedly succeeded\n%s", test.label, harness.output.String())
				}
			case <-ctx.Done():
				t.Fatalf("timed out waiting for %s replacement rejection\n%s", test.label, harness.output.String())
			}
			if got := systemMigrationIdentity(t, recovery); got != identity {
				t.Fatalf("%s exact recovery identity = %q, want %q", test.label, got, identity)
			}
			if !strings.Contains(harness.output.String(), "retained at "+recovery) {
				t.Fatalf("%s replacement did not report exact recovery %s\n%s", test.label, recovery, harness.output.String())
			}
			if !strings.Contains(harness.output.String(), "unknown collision preserved at "+created) {
				t.Fatalf("%s replacement did not report preserved unknown winner\n%s", test.label, harness.output.String())
			}
			unknown := created
			if test.kind == "directory" {
				unknown = filepath.Join(created, "unknown-winner")
			}
			if got := mustReadSystemMigrationFixture(t, unknown); got != "unknown winner\n" {
				t.Fatalf("%s unknown winner changed: %q", test.label, got)
			}
			if after := snapshotSystemMigrationTree(t, destination); after != liveBefore {
				t.Fatalf("%s replacement changed live tree:\nwant:\n%s\ngot:\n%s\n%s", test.label, liveBefore, after, harness.output.String())
			}
		})
	}
}

func systemMigrationIdentity(t *testing.T, path string) string {
	t.Helper()
	var info unix.Stat_t
	if err := unix.Lstat(path, &info); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%d:%d", info.Dev, info.Ino)
}

func TestSystemUserNamespaceCanonicalStagePathWorksWithNix(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix is unavailable")
	}
	workRoot := t.TempDir()
	source := t.TempDir()
	mustWriteSystemMigrationFixture(t, filepath.Join(source, "flake.nix"), `{ outputs = { self }: { }; }`+"\n")

	script := `
set -euo pipefail
work_root=$1
source=$2
exec 9<"$work_root"
work_root_fd=/proc/self/fd/9
stage="$work_root/staging"
mkdir "$stage"
stage_id="$(stat -c '%d:%i' -- "$stage")"
exec {stage_fd}<"$work_root_fd/staging"
stage_pinned="/proc/self/fd/$stage_fd"
[ "$stage_id" = "$(stat -Lc '%d:%i' -- "$stage_pinned")" ]
cp -a "$source/." "$stage/"
(
	cd "$stage_pinned"
	nix flake metadata --offline --no-write-lock-file path:. >/dev/null
)
`
	cmd := exec.Command("bash", "-c", script, "canonical-stage-test", workRoot, source)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("identity-verified canonical stage path failed Nix resolution: %v\n%s", err, output)
	}
}

func writeSystemMigrationServiceFixture(t *testing.T, destination, marker string, withNamespace bool) {
	t.Helper()
	brand := "wahrwelt"
	repo := "wahrwelt"
	description := "Host-local Wahrwelt NixOS wrapper"
	constructor := "mkWahrweltHost"
	if marker == "# Generated by MySetup installer." {
		brand = "mysetup"
		repo = "MySetup"
		description = "Host-local MySetup NixOS wrapper"
		constructor = "mkMySetupHost"
	}
	if marker == "# Generated by Wahrwelt installer." || marker == "# Generated by MySetup installer." {
		wrapper := fmt.Sprintf(`{
  %s
  description = %q;
  inputs = {
    %s = {
      url = "github:TakuyaYagam1/%s/main?dir=Linux/NixOS/presets/personal";
    };
  };
  outputs = inputs@{ %s, ... }:
    let
      hostVars = import ./host-vars.nix;
      hostname = hostVars.host.hostname;
    in {
      nixosConfigurations.${hostname} = %s.lib.%s {
        hostVars = ./host-vars.nix;
        hardware = ./hardware-configuration.nix;
        extraModules = [ ./configuration.nix ];
        homeExtraModules = if builtins.pathExists ./home.nix then [ ./home.nix ] else [ ];
      };
    };
}
`, marker, description, brand, repo, brand, brand, constructor)
		mustWriteSystemMigrationFixture(t, filepath.Join(destination, "flake.nix"), wrapper)
		lock := fmt.Sprintf(`{
  "nodes": {
    "root": {
      "inputs": {
        %q: %q
      }
    },
    %q: {
      "locked": {
        "dir": "Linux/NixOS/presets/personal",
        "lastModified": 1234567890,
        "narHash": "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
        "owner": "TakuyaYagam1",
        "repo": %q,
        "rev": "0123456789abcdef0123456789abcdef01234567",
        "type": "github"
      },
      "original": {
        "dir": "Linux/NixOS/presets/personal",
        "owner": "TakuyaYagam1",
        "ref": "main",
        "repo": %q,
        "type": "github"
      }
    }
  },
  "root": "root",
  "version": 7
}
`, brand, brand, brand, repo, repo)
		mustWriteSystemMigrationFixture(t, filepath.Join(destination, "flake.lock"), lock)
	} else {
		mustWriteSystemMigrationFixture(t, filepath.Join(destination, "flake.nix"), "{\n  "+marker+"\n}\n")
		mustWriteSystemMigrationFixture(t, filepath.Join(destination, "flake.lock"), "{}\n")
	}
	mustWriteSystemMigrationFixture(t, filepath.Join(destination, "wahrwelt", "canonical.txt"), "canonical Wahrwelt tree\n")
	mustWriteSystemMigrationFixture(t, filepath.Join(destination, "mysetup", "compatibility.txt"), "canonical mysetup compatibility alias\n")
	mustWriteSystemMigrationFixture(t, filepath.Join(destination, "mysetup-compat.nix"), "# canonical mysetup compatibility text\n")
	if !withNamespace {
		mustWriteSystemMigrationFixture(t, filepath.Join(destination, "configuration.nix"), "imports = [ ./mysetup-compat.nix ];\n")
		return
	}
	mustWriteSystemMigrationFixture(t, filepath.Join(destination, "configuration.nix"), "imports = [ ./private ./mysetup-compat.nix ];\n")
	mustWriteSystemMigrationFixture(t, filepath.Join(destination, "private", "custom.nix"), "custom\n")
	mustWriteSystemMigrationFixture(t, filepath.Join(destination, "wahrwelt", "state.json"), systemMigrationStatePayload(7))
}

func writeSystemMigrationLegacyPasswordFixture(
	t *testing.T,
	destination string,
	moduleRelative string,
	moduleNamespace string,
	passwordHash string,
) {
	t.Helper()
	flakePath := filepath.Join(destination, "flake.nix")
	flake := mustReadSystemMigrationFixture(t, flakePath)
	const hardwareLine = "        hardware = ./hardware-configuration.nix;\n"
	if strings.Count(flake, hardwareLine) != 1 {
		t.Fatalf("generated wrapper fixture lacks one hardware argument:\n%s", flake)
	}
	legacyPassword := hardwareLine + `        hashedPassword =
          if builtins.pathExists ./hashed-password.nix then ./hashed-password.nix else null;
`
	mustWriteSystemMigrationFixture(t, flakePath, strings.Replace(flake, hardwareLine, legacyPassword, 1))

	modulePath := filepath.Join(destination, filepath.FromSlash(moduleRelative))
	module := fmt.Sprintf(`{ config, ... }:

{
  users.users.${config.%s.user.username}.initialHashedPassword = %q;
}
`, moduleNamespace, passwordHash)
	mustWriteSystemMigrationFixture(t, modulePath, module)
	if err := os.Chmod(modulePath, 0o600); err != nil {
		t.Fatal(err)
	}
}

type renderedSystemMigrationService struct {
	cmd      *exec.Cmd
	output   bytes.Buffer
	testRoot string
	workRoot string
}

func prepareRenderedSystemMigrationService(
	ctx context.Context,
	t *testing.T,
	destination string,
) *renderedSystemMigrationService {
	t.Helper()
	module := readContractFile(t, "../../../NixOS/system/wahrwelt-migration.nix")
	const textStart = "    text = ''\n"
	start := strings.Index(module, textStart)
	if start < 0 {
		t.Fatal("system migration shell application text is missing")
	}
	start += len(textStart)
	end := strings.Index(module[start:], "\n    '';\n  };")
	if end < 0 {
		t.Fatal("system migration shell application terminator is missing")
	}
	script := module[start : start+end]
	helperPath, err := filepath.Abs(systemUserNamespaceHelper)
	if err != nil {
		t.Fatal(err)
	}
	brandHelperPath, err := filepath.Abs(systemBrandMigrationHelper)
	if err != nil {
		t.Fatal(err)
	}
	testRoot := t.TempDir()
	workRoot := filepath.Join(testRoot, "work")
	fakeBin := filepath.Join(testRoot, "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeFSHelper := filepath.Join(fakeBin, "wahrwelt-fs-helper")
	writeRenderedSystemMigrationFSHelper(t, fakeFSHelper)
	passwordHashTarget := filepath.Join(filepath.Dir(destination), "wahrwelt", "hashed-password")
	replacements := map[string]string{
		"${lib.escapeShellArg destination}":                     shellQuoteSystemMigrationTest(destination),
		"${lib.escapeShellArg hostname}":                        shellQuoteSystemMigrationTest("test-host"),
		"${lib.escapeShellArg \"${pkgs.python3}/bin/python3\"}": shellQuoteSystemMigrationTest("python3"),
		"${lib.escapeShellArg v1ToV2NamespaceSource}":           shellQuoteSystemMigrationTest(helperPath),
		"${lib.escapeShellArg v1ToV2BrandSource}":               shellQuoteSystemMigrationTest(brandHelperPath),
		"${lib.escapeShellArg \"${inputs.wahrwelt.packages.${pkgs.stdenv.hostPlatform.system}.wahrwelt}/bin/wahrwelt-fs-helper\"}": shellQuoteSystemMigrationTest(fakeFSHelper),
		"${lib.escapeShellArg \"/etc/wahrwelt/hashed-password\"}":                                                                  shellQuoteSystemMigrationTest(passwordHashTarget),
		`if [ "$owner" != 0 ] || [ "$((8#$mode & 0022))" -ne 0 ]; then`:                                                            `if false; then`,
	}
	for old, replacement := range replacements {
		if strings.Count(script, old) != 1 {
			t.Fatalf("system migration harness expected one %q replacement", old)
		}
		script = strings.Replace(script, old, replacement, 1)
	}
	script = strings.ReplaceAll(script, "''${", "${")

	fakeNix := filepath.Join(fakeBin, "nix")
	mustWriteSystemMigrationFixture(t, fakeNix, "#!/usr/bin/env bash\nset -euo pipefail\nprintf '%s\\n' \"$*\" >> \"$WAHRWELT_TEST_NIX_LOG\"\n")
	if err := os.Chmod(fakeNix, 0o755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(testRoot, "migration.sh")
	mustWriteSystemMigrationFixture(t, scriptPath, script)
	if err := os.Chmod(scriptPath, 0o700); err != nil {
		t.Fatal(err)
	}

	cmd := exec.CommandContext(ctx, "bash", scriptPath)
	cmd.Env = append(
		os.Environ(),
		"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"WAHRWELT_MIGRATION_WORK_ROOT="+workRoot,
		"WAHRWELT_TEST_FS_HELPER_ROOT="+workRoot,
		"WAHRWELT_TEST_FS_HELPER_LOG="+filepath.Join(testRoot, "fs-helper.log"),
		"WAHRWELT_TEST_NIX_LOG="+filepath.Join(testRoot, "nix.log"),
	)
	harness := &renderedSystemMigrationService{
		cmd:      cmd,
		testRoot: testRoot,
		workRoot: workRoot,
	}
	cmd.Stdout = &harness.output
	cmd.Stderr = &harness.output
	return harness
}

func writeRenderedSystemMigrationFSHelper(t *testing.T, path string) {
	t.Helper()
	const helper = `#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 7 ] ||
  [ "$1" != remove-migration-temporary ] ||
  [ "$2" != --kind ] ||
  [ "$4" != --name ] ||
  [ "$6" != --expected ]; then
  printf '%s\n' 'fake fs helper received an unsupported command shape' >&2
  exit 2
fi

kind=$3
name=$5
expected=$7
case "$kind" in
  staging | namespace) ;;
  *) printf '%s\n' 'fake fs helper received an unsupported kind' >&2; exit 2 ;;
esac
name_pattern="^${kind}\\.[0-9a-f]{16}$"
if [[ ! "$name" =~ $name_pattern ]] || [[ ! "$expected" =~ ^[0-9]+:[1-9][0-9]*$ ]]; then
  printf '%s\n' 'fake fs helper received an invalid name or identity' >&2
  exit 2
fi

root="${WAHRWELT_TEST_FS_HELPER_ROOT:?}"
case "$root" in
  /*) ;;
  *) printf '%s\n' 'fake fs helper root must be absolute' >&2; exit 2 ;;
esac
if [ -L "$root" ] || [ ! -d "$root" ] || [ "$(realpath -e -- "$root")" != "$root" ]; then
  printf '%s\n' 'fake fs helper root must be a canonical directory' >&2
  exit 2
fi
target="$root/$name"
if [ "$(dirname -- "$target")" != "$root" ] || [ -L "$target" ]; then
  printf '%s\n' 'fake fs helper target escaped its work root' >&2
  exit 2
fi
if [ "$(stat -c '%d:%i' -- "$target")" != "$expected" ] ||
  [ "$(stat -c '%u' -- "$target")" != "$(id -u)" ]; then
  printf '%s\n' 'fake fs helper target identity changed' >&2
  exit 2
fi

case "$kind" in
  staging)
    if [ ! -d "$target" ] || [ "$(stat -c '%a' -- "$target")" != 700 ]; then
      printf '%s\n' 'fake fs helper staging target metadata is invalid' >&2
      exit 2
    fi
    rm -rf -- "$target"
    ;;
  namespace)
    if [ ! -f "$target" ] ||
      [ "$(stat -c '%a' -- "$target")" != 600 ] ||
      [ "$(stat -c '%h' -- "$target")" != 1 ]; then
      printf '%s\n' 'fake fs helper namespace target metadata is invalid' >&2
      exit 2
    fi
    rm -f -- "$target"
    ;;
esac
printf '%s\t%s\t%s\n' "$kind" "$name" "$expected" >> "${WAHRWELT_TEST_FS_HELPER_LOG:?}"
`
	mustWriteSystemMigrationFixture(t, path, helper)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func runRenderedSystemMigrationService(t *testing.T, destination string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	harness := prepareRenderedSystemMigrationService(ctx, t, destination)
	err := harness.cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("system migration service timed out: %v\n%s", ctx.Err(), harness.output.String())
	}
	if err == nil {
		assertRenderedSystemMigrationCleanup(t, harness)
	}
	return harness.output.String(), err
}

func assertRenderedSystemMigrationCleanup(t *testing.T, harness *renderedSystemMigrationService) {
	t.Helper()
	entries, err := os.ReadDir(harness.workRoot)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "v1_to_v2.complete" {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("successful system migration work root must contain only its completion marker: %v", names)
	}
	completion := filepath.Join(harness.workRoot, "v1_to_v2.complete")
	completionInfo, err := os.Stat(completion)
	if err != nil || completionInfo.Mode().Perm() != 0o600 || !completionInfo.Mode().IsRegular() {
		t.Fatalf("system migration completion marker metadata is invalid: info=%v err=%v", completionInfo, err)
	}
	if payload := mustReadSystemMigrationFixture(t, completion); payload != "wahrwelt-v1-to-v2-complete\n" {
		t.Fatalf("system migration completion marker payload = %q", payload)
	}

	logPath := filepath.Join(harness.testRoot, "fs-helper.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("successful system migration did not record exact helper cleanup: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	wantKinds := []string{"staging", "namespace"}
	if len(lines) != len(wantKinds) {
		t.Fatalf("successful system migration cleanup calls = %q, want staging and namespace", lines)
	}
	identityPattern := regexp.MustCompile(`^[0-9]+:[1-9][0-9]*$`)
	for index, kind := range wantKinds {
		fields := strings.Split(lines[index], "\t")
		namePattern := regexp.MustCompile(`^` + kind + `\.[0-9a-f]{16}$`)
		if len(fields) != 3 || fields[0] != kind || !namePattern.MatchString(fields[1]) || !identityPattern.MatchString(fields[2]) {
			t.Fatalf("successful system migration cleanup call %d = %q, want exact %s name and identity", index, lines[index], kind)
		}
	}
}

func shellQuoteSystemMigrationTest(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func runSystemUserNamespaceHelper(t *testing.T, wantExit int, args ...string) {
	t.Helper()
	_ = runSystemUserNamespaceHelperOutput(t, wantExit, args...)
}

func runSystemUserNamespaceHelperOutput(t *testing.T, wantExit int, args ...string) string {
	t.Helper()
	cmd := exec.Command("python3", append([]string{"-I", "-S", systemUserNamespaceHelper}, args...)...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	gotExit := 0
	if err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			t.Fatalf("helper %v could not run: %v\n%s", args, err, output.String())
		}
		gotExit = exitError.ExitCode()
	}
	if gotExit != wantExit {
		t.Fatalf("helper %v exit = %d, want %d: %v\n%s", args, gotExit, wantExit, err, output.String())
	}
	return output.String()
}

func mustWriteSystemMigrationFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func systemMigrationStatePayload(schema int) string {
	state := map[string]any{
		"host": map[string]any{
			"hostname":     "NixOS",
			"stateVersion": "26.05",
		},
		"user": map[string]any{
			"username":      "alice",
			"fullName":      "Alice",
			"homeDirectory": "/home/alice",
		},
		"locale": map[string]any{
			"timeZone":        "Europe/Moscow",
			"defaultLocale":   "en_US.UTF-8",
			"extraLocale":     "ru_RU.UTF-8",
			"consoleKeyMap":   "us",
			"weatherLocation": "Moscow",
			"keyboardLayouts": "us,ru",
			"keyboardToggle":  "grp:alt_shift_toggle",
		},
		"git":      map[string]any{"username": "alice", "email": "alice@example.com"},
		"packages": map[string]any{"preset": "personal"},
		"display": map[string]any{
			"monitorName":     "eDP-1",
			"monitorMode":     "preferred",
			"monitorPosition": "0x0",
			"monitorScale":    "1",
		},
		"hardware": map[string]any{"gpu": "amd"},
		"features": map[string]any{
			"secureBoot":    false,
			"ctfTools":      false,
			"omniRouter":    false,
			"observability": false,
		},
		"dots": map[string]any{
			"hypr":       true,
			"zenTheme":   true,
			"sine":       true,
			"neovim":     true,
			"v2rayN":     true,
			"wallpapers": true,
		},
	}
	if schema > 0 {
		state["schemaVersion"] = schema
	}
	if schema <= 3 {
		state["shell"] = map[string]any{"profile": "caelestia"}
	}
	if schema <= 5 {
		state["services"] = map[string]any{"pgAdminEmail": "admin@localhost.local"}
	}
	if schema <= 6 {
		state["zapret"] = map[string]any{
			"enable": false,
			"config": "general (FAKE_TLS_AUTO_ALT3)",
		}
	}
	features := state["features"].(map[string]any)
	if schema <= 5 {
		features["russiaMode"] = false
	}
	if schema <= 3 {
		delete(features, "observability")
	}
	if schema >= 7 {
		features["portainer"] = false
	}
	dots := state["dots"].(map[string]any)
	if schema >= 4 && schema <= 6 {
		dots["neovimCleanState"] = false
	}
	if schema >= 5 {
		display := state["display"].(map[string]any)
		display["extraMonitors"] = []any{
			map[string]any{"name": "DP-1", "mode": "preferred", "position": "auto", "scale": "1"},
		}
	}
	if schema >= 6 {
		state["source"] = map[string]any{"channel": "stable"}
	}
	if schema >= 7 {
		state["noctalia"] = map[string]any{"version": "v5"}
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(data) + "\n"
}

func mutateSystemMigrationStatePayload(schema int, mutate func(map[string]any)) string {
	var state map[string]any
	if err := json.Unmarshal([]byte(systemMigrationStatePayload(schema)), &state); err != nil {
		panic(err)
	}
	mutate(state)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(data) + "\n"
}

func findSystemMigrationParentQuarantine(
	t *testing.T,
	root string,
	brand string,
	expected os.FileInfo,
) string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	prefix := "." + brand + ".installer-state-parent."
	matches := make([]string, 0, 1)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		path := filepath.Join(root, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if os.SameFile(expected, info) {
			matches = append(matches, path)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("exact %s parent quarantine matches = %v, want one", brand, matches)
	}
	return matches[0]
}

func mustReadSystemMigrationFixture(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func snapshotSystemMigrationTree(t *testing.T, root string) string {
	t.Helper()
	var out strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out.WriteString(rel)
		out.WriteByte(' ')
		out.WriteString(info.Mode().String())
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			out.WriteByte(' ')
			out.Write(data)
		}
		out.WriteByte('\n')
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return out.String()
}
