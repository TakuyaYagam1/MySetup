//go:build linux

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/fsowner"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
	"golang.org/x/sys/unix"
)

const (
	maxPasswordModuleBytes  = 16 * 1024
	backupMarkerName        = ".wahrwelt-backup-v1"
	backupMarkerVersion     = "wahrwelt-backup-v1"
	passwordTransactionDir  = ".password-transactions"
	passwordTransactionPre  = ".password-hash-v1-"
	passwordMigrationRecord = ".password-module-v1-to-v2.json"
	passwordMigrationV1     = 1
)

var (
	generatedPasswordModuleDetailsPattern           = regexp.MustCompile(`(?s)^\{ config, \.\.\. \}:\n\n\{\n  users\.users\.\$\{config\.(wahrwelt|mysetup)\.user\.username\}\.initialHashedPassword = "([^"\r\n]+)";\n\}\n$`)
	sha512CryptPattern                              = regexp.MustCompile(`^\$6\$(?:rounds=[0-9]+\$)?[./A-Za-z0-9]{1,16}\$[./A-Za-z0-9]{86}$`)
	commandInput                          io.Reader = os.Stdin
	commandOutput                         io.Writer = os.Stdout
)

var (
	errRuntimeLockBusy = errors.New("runtime lock is busy")
)

const runtimeLockSignalGracePeriod = time.Second

type backupCandidate struct {
	entry     fsowner.Entry
	timestamp uint64
	pid       uint64
	attempt   uint64
}

type migrationIdentity struct {
	Device uint64
	Inode  uint64
}

type passwordModuleCandidate struct {
	path      string
	file      *os.File
	identity  fsowner.Identity
	namespace string
	hash      string
	stub      bool
	scrubbed  bool
}

type passwordMigrationHooks struct {
	BeforeSanitize func() error
	AfterScrub     func(path string) error
	AfterStub      func(path string) error
}

type passwordMigrationEntry struct {
	Path      string `json:"path"`
	Device    uint64 `json:"device"`
	Inode     uint64 `json:"inode"`
	UID       uint32 `json:"uid"`
	Namespace string `json:"namespace"`
}

type passwordMigrationJournal struct {
	Version int                      `json:"version"`
	Entries []passwordMigrationEntry `json:"entries"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, errRuntimeLockBusy) {
			os.Exit(75)
		}
		var commandError commandExitError
		if errors.As(err, &commandError) {
			os.Exit(commandError.code)
		}
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func closeIgnoringError(closer io.Closer) {
	_ = closer.Close()
}

func closeDescriptorIgnoringError(fd int) {
	_ = unix.Close(fd)
}

func effectiveUID() (uint32, error) {
	uid := os.Geteuid()
	if uid < 0 || int64(uid) > int64(^uint32(0)) {
		return 0, errors.New("effective user ID is outside the supported range")
	}
	return uint32(uid), nil
}

func checkedUint32(value uint64) (uint32, error) {
	if value > uint64(^uint32(0)) {
		return 0, errors.New("value is outside the uint32 range")
	}
	return uint32(value), nil
}

func newFileFromDescriptor(fd int, name string) *os.File {
	if fd < 0 {
		return nil
	}
	return os.NewFile(uintptr(fd), name)
}

func descriptorNumber(file *os.File) (int, error) {
	fd := file.Fd()
	if fd > uintptr(^uint(0)>>1) {
		return 0, errors.New("file descriptor is outside the supported range")
	}
	return int(fd), nil
}

//nolint:gocyclo // The command boundary keeps each privileged command and its validation explicit.
func run(args []string) error {
	if len(args) == 0 {
		return errors.New("missing helper command")
	}
	switch args[0] {
	case "__runtime-lock-child":
		return runRuntimeLockChild(args[1:])
	case "__runtime-lock-watchdog":
		return runRuntimeLockWatchdog(args[1:])
	case "mark-nixos-backup":
		if err := requireRoot(); err != nil {
			return err
		}
		set := flag.NewFlagSet(args[0], flag.ContinueOnError)
		backup := set.String("backup", "", "new backup path")
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		if set.NArg() != 0 || !canonicalNixOSBackupPath(*backup) {
			return errors.New("mark-nixos-backup accepts only an exact /etc/nixos.bak.<timestamp>.<pid>.<attempt> path")
		}
		return markBackup(filepath.Clean(*backup), 0)
	case "prune-nixos-backups":
		if err := requireRoot(); err != nil {
			return err
		}
		set := flag.NewFlagSet(args[0], flag.ContinueOnError)
		parent := set.String("parent", "/etc", "backup parent")
		keep := set.Int("keep", 3, "newest backup count")
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		if filepath.Clean(*parent) != "/etc" || *keep != 3 || set.NArg() != 0 {
			return errors.New("prune-nixos-backups accepts only --parent /etc --keep 3")
		}
		return pruneBackups(*parent, *keep, 0)
	case "migrate-generated-password-modules":
		if err := requireRoot(); err != nil {
			return err
		}
		set := flag.NewFlagSet(args[0], flag.ContinueOnError)
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		if set.NArg() != 0 {
			return errors.New("migrate-generated-password-modules accepts no arguments")
		}
		status, err := migrateGeneratedPasswordModules(
			sortedPasswordModuleSources(canonicalPasswordModuleSources()),
			paths.DefaultPasswordHashPath,
			filepath.Join(filepath.Dir(paths.DefaultPasswordHashPath), passwordMigrationRecord),
			0,
			passwordMigrationHooks{},
		)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(commandOutput, status)
		return err
	case "validate-password-hash":
		if err := requireRoot(); err != nil {
			return err
		}
		set := flag.NewFlagSet(args[0], flag.ContinueOnError)
		path := set.String("path", "", "external raw password hash path")
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		if set.NArg() != 0 || filepath.Clean(*path) != paths.DefaultPasswordHashPath || *path != filepath.Clean(*path) {
			return errors.New("validate-password-hash accepts only --path /etc/wahrwelt/hashed-password")
		}
		return validateExternalPasswordHash(*path, 0)
	case "password-hash-status":
		if err := requireRoot(); err != nil {
			return err
		}
		set := flag.NewFlagSet(args[0], flag.ContinueOnError)
		path := set.String("path", "", "external raw password hash path")
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		if set.NArg() != 0 || filepath.Clean(*path) != paths.DefaultPasswordHashPath || *path != filepath.Clean(*path) {
			return errors.New("password-hash-status accepts only --path /etc/wahrwelt/hashed-password")
		}
		status, err := passwordHashStatus(*path, 0)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(commandOutput, status)
		return err
	case "publish-password-hash":
		if err := requireRoot(); err != nil {
			return err
		}
		set := flag.NewFlagSet(args[0], flag.ContinueOnError)
		path := set.String("path", "", "external raw password hash path")
		source := set.String("source", "", "sealed caller descriptor")
		expected := set.String("expected", "", "absent or exact target identity")
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		if set.NArg() != 0 || filepath.Clean(*path) != paths.DefaultPasswordHashPath || *path != filepath.Clean(*path) {
			return errors.New("publish-password-hash accepts only --path /etc/wahrwelt/hashed-password")
		}
		return publishPasswordHash(*path, *source, *expected, 0, filepath.Join(filepath.Dir(*path), passwordTransactionDir))
	case "remove-migration-temporary":
		if err := requireRoot(); err != nil {
			return err
		}
		set := flag.NewFlagSet(args[0], flag.ContinueOnError)
		kind := set.String("kind", "", "staging or namespace")
		name := set.String("name", "", "allowlisted temporary basename")
		expectedText := set.String("expected", "", "expected device:inode")
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		expected, err := parseMigrationIdentity(*expectedText)
		if err != nil || set.NArg() != 0 || !validMigrationTemporaryName(*kind, *name) {
			return errors.New("remove-migration-temporary requires --kind staging|namespace, its exact 16-hex basename, and --expected device:inode")
		}
		return removeMigrationTemporary("/var/lib/wahrwelt/migration", *kind, *name, expected, 0)
	case "runtime-lock-run":
		set := flag.NewFlagSet(args[0], flag.ContinueOnError)
		root := set.String("root", "", "validated XDG runtime root")
		name := set.String("name", "", "exact v2 logical lock name")
		waitMS := set.Int("wait-ms", 0, "bounded lock wait in milliseconds")
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		command := set.Args()
		if !exactRuntimeRoot(*root) || !validRuntimeLockName(*name) || *waitMS < 0 || *waitMS > 30_000 || len(command) == 0 {
			return errors.New("runtime-lock-run requires the exact XDG runtime root, an allowlisted v2 lock name, bounded wait, and exact argv")
		}
		return runWithRuntimeLock(*root, *name, time.Duration(*waitMS)*time.Millisecond, command)
	case "runtime-begin":
		set := flag.NewFlagSet(args[0], flag.ContinueOnError)
		root := set.String("root", "", "private transaction root")
		kind := set.String("kind", "", "runtime or state")
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		prefix, err := runtimePrefix(*kind)
		if err != nil || *root == "" || len(set.Args()) == 0 {
			return errors.New("runtime-begin requires --root, --kind runtime|state, and absolute targets")
		}
		transaction, err := fsowner.Begin(*root, prefix, set.Args())
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(commandOutput, transaction)
		return err
	case "runtime-write":
		set := flag.NewFlagSet(args[0], flag.ContinueOnError)
		transaction := set.String("transaction", "", "transaction path")
		target := set.String("target", "", "tracked target")
		modeText := set.String("mode", "", "0600 or 0644")
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		mode, err := runtimeMode(*modeText)
		if err != nil || *transaction == "" || *target == "" || set.NArg() != 0 {
			return errors.New("runtime-write requires --transaction, --target, and --mode 0600|0644")
		}
		payload, err := readRuntimePayload(commandInput)
		if err != nil {
			return err
		}
		return fsowner.Write(*transaction, *target, payload, mode)
	case "runtime-remove":
		set := flag.NewFlagSet(args[0], flag.ContinueOnError)
		transaction := set.String("transaction", "", "transaction path")
		target := set.String("target", "", "tracked target")
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		if *transaction == "" || *target == "" || set.NArg() != 0 {
			return errors.New("runtime-remove requires --transaction and --target")
		}
		return fsowner.Remove(*transaction, *target)
	case "runtime-rollback", "runtime-commit":
		if len(args) != 2 {
			return fmt.Errorf("%s requires one transaction path", args[0])
		}
		if args[0] == "runtime-rollback" {
			return fsowner.Rollback(args[1])
		}
		return fsowner.Commit(args[1])
	case "runtime-matches":
		set := flag.NewFlagSet(args[0], flag.ContinueOnError)
		target := set.String("target", "", "target path")
		modeText := set.String("mode", "", "0600 or 0644")
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		mode, err := runtimeMode(*modeText)
		if err != nil || *target == "" || set.NArg() != 0 {
			return errors.New("runtime-matches requires --target and --mode 0600|0644")
		}
		payload, err := readRuntimePayload(commandInput)
		if err != nil {
			return err
		}
		matches, err := fsowner.Matches(*target, payload, mode)
		if err != nil {
			return err
		}
		if !matches {
			return errors.New("runtime target does not match")
		}
		return nil
	case "runtime-scavenge":
		set := flag.NewFlagSet(args[0], flag.ContinueOnError)
		root := set.String("root", "", "private transaction root")
		kind := set.String("kind", "", "runtime or state")
		if err := set.Parse(args[1:]); err != nil {
			return err
		}
		prefix, err := runtimePrefix(*kind)
		if err != nil || *root == "" || set.NArg() != 0 {
			return errors.New("runtime-scavenge requires --root and --kind runtime|state")
		}
		result, err := fsowner.Scavenge(*root, []string{prefix})
		if err != nil {
			return err
		}
		if len(result.Issues) != 0 {
			return fmt.Errorf("owned cleanup incomplete: %s", strings.Join(result.Issues, "; "))
		}
		return nil
	default:
		return fmt.Errorf("unsupported helper command %q", args[0])
	}
}

func exactRuntimeRoot(root string) bool {
	expected := os.Getenv("XDG_RUNTIME_DIR")
	return root != "" && expected != "" && filepath.IsAbs(root) && root == filepath.Clean(root) &&
		expected == filepath.Clean(expected) && root == expected
}

func validRuntimeLockName(name string) bool {
	switch name {
	case "wahrwelt-noctalia-launcher-v2.lock",
		"wahrwelt-record-toggle-v2.lock",
		"wahrwelt-shell-selector-v2.lock",
		"wahrwelt-shell-v2.lock":
		return true
	default:
		return false
	}
}

//nolint:gocyclo // Lock ownership, child supervision, signals, and watchdog cleanup form one linear lifecycle.
func runWithRuntimeLock(rootPath, name string, wait time.Duration, command []string) error {
	root, err := fsowner.OpenDirectory(rootPath)
	if err != nil {
		return err
	}
	identity := root.Identity()
	if closeErr := root.Close(); closeErr != nil {
		return closeErr
	}
	uid, err := effectiveUID()
	if err != nil {
		return err
	}
	if identity.Kind != fsowner.KindDirectory || identity.UID != uid || identity.Mode&0o7777 != 0o700 {
		return errors.New("ownership collision: runtime lock scope root is not exact mode 0700")
	}

	scope := sha256.Sum256([]byte(fmt.Sprintf("wahrwelt-runtime-lock-v2\x00%d\x00%d:%d\x00%s", uid, identity.Device, identity.Inode, name)))
	address := &unix.SockaddrUnix{Name: fmt.Sprintf("@wahrwelt-v2-%x", scope[:])}
	deadline := time.Now().Add(wait)
	var socketFD int
	for {
		socketFD, err = unix.Socket(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
		if err != nil {
			return fmt.Errorf("create runtime lock socket: %w", err)
		}
		err = unix.Bind(socketFD, address)
		if err == nil {
			break
		}
		_ = unix.Close(socketFD)
		if !errors.Is(err, unix.EADDRINUSE) {
			return fmt.Errorf("bind runtime lock socket: %w", err)
		}
		if wait == 0 || !time.Now().Before(deadline) {
			return errRuntimeLockBusy
		}
		pause := 10 * time.Millisecond
		if remaining := time.Until(deadline); pause > remaining {
			pause = remaining
		}
		if pause > 0 {
			time.Sleep(pause)
		}
	}
	defer closeDescriptorIgnoringError(socketFD)

	helperExecutable, err := os.Executable()
	if err != nil {
		return err
	}
	gateReader, gateWriter, err := os.Pipe()
	if err != nil {
		return err
	}
	defer closeIgnoringError(gateReader)
	defer closeIgnoringError(gateWriter)
	watchReader, watchWriter, err := os.Pipe()
	if err != nil {
		return err
	}
	defer closeIgnoringError(watchReader)
	defer closeIgnoringError(watchWriter)

	childArgs := append([]string{"__runtime-lock-child", "--"}, command...)
	// helperExecutable comes from os.Executable, and childArgs preserves exact argv boundaries.
	child := exec.Command(helperExecutable, childArgs...) //nolint:gosec // This starts the current verified helper executable.
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.ExtraFiles = []*os.File{gateReader}
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	child.Env = runtimeLockEnvironment(os.Environ(), map[string]string{
		"WAHRWELT_RUNTIME_LOCK_V2":      name,
		"WAHRWELT_RUNTIME_LOCK_V2_ROOT": rootPath,
	})
	if err := child.Start(); err != nil {
		return err
	}
	_ = gateReader.Close()

	watchSocketFD, err := unix.Dup(socketFD)
	if err != nil {
		_ = unix.Kill(-child.Process.Pid, unix.SIGKILL)
		_ = child.Wait()
		return fmt.Errorf("duplicate runtime lock socket for watchdog: %w", err)
	}
	watchSocket := newFileFromDescriptor(watchSocketFD, "runtime-lock-watchdog-socket")
	watchdog := exec.Command(helperExecutable, "__runtime-lock-watchdog", strconv.Itoa(child.Process.Pid))
	watchdog.ExtraFiles = []*os.File{watchReader, watchSocket}
	watchdog.Stdout = os.Stderr
	watchdog.Stderr = os.Stderr
	if err := watchdog.Start(); err != nil {
		_ = watchSocket.Close()
		_ = unix.Kill(-child.Process.Pid, unix.SIGKILL)
		_ = child.Wait()
		return fmt.Errorf("start runtime lock watchdog: %w", err)
	}
	_ = watchSocket.Close()
	_ = watchReader.Close()
	signals := make(chan os.Signal, 8)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	defer signal.Stop(signals)
	if _, err := gateWriter.Write([]byte{'G'}); err != nil {
		_ = unix.Kill(-child.Process.Pid, unix.SIGKILL)
		_ = child.Wait()
		_ = watchWriter.Close()
		_ = watchdog.Wait()
		return fmt.Errorf("release runtime lock child barrier: %w", err)
	}
	_ = gateWriter.Close()

	childDone := make(chan error, 1)
	watchdogDone := make(chan error, 1)
	go func() {
		childDone <- child.Wait()
	}()
	go func() {
		watchdogDone <- watchdog.Wait()
	}()
	var (
		waitErr         error
		childExited     bool
		signalForwarded bool
		escalation      *time.Timer
		escalationC     <-chan time.Time
	)
	for !childExited {
		select {
		case waitErr = <-childDone:
			childExited = true
		case watchErr := <-watchdogDone:
			_ = unix.Kill(-child.Process.Pid, unix.SIGKILL)
			childErr := <-childDone
			return errors.Join(errors.New("runtime lock watchdog exited before child"), watchErr, childErr)
		case received := <-signals:
			if value, ok := received.(syscall.Signal); ok {
				signalForwarded = true
				_ = unix.Kill(-child.Process.Pid, value)
				if escalation == nil {
					escalation = time.NewTimer(runtimeLockSignalGracePeriod)
					escalationC = escalation.C
				}
			}
		case <-escalationC:
			_ = unix.Kill(-child.Process.Pid, unix.SIGKILL)
			escalationC = nil
		}
	}
	if escalation != nil {
		escalation.Stop()
	}

	// Only a clean, uninterrupted child is allowed to release descendants.
	// Every failure or forwarded signal closes the pipe without the clean token,
	// making the watchdog kill the entire process group before it drops its copy
	// of the bound socket.
	var cleanErr error
	if waitErr == nil && !signalForwarded {
		_, cleanErr = watchWriter.Write([]byte{'C'})
	}
	_ = watchWriter.Close()
	watchErr := <-watchdogDone
	if cleanErr != nil || watchErr != nil {
		return errors.Join(waitErr, fmt.Errorf("runtime lock watchdog cleanup failed: %w", errors.Join(cleanErr, watchErr)))
	}
	if waitErr == nil {
		return nil
	}
	var exitError *exec.ExitError
	if errors.As(waitErr, &exitError) {
		if status, ok := exitError.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return commandExitError{code: 128 + int(status.Signal())}
		}
		return commandExitError{code: exitError.ExitCode()}
	}
	return waitErr
}

func runRuntimeLockChild(args []string) error {
	if len(args) < 2 || args[0] != "--" || args[1] == "" {
		return errors.New("invalid internal runtime lock child argv")
	}
	gate := os.NewFile(3, "runtime-lock-child-gate")
	if gate == nil {
		return errors.New("runtime lock child gate is unavailable")
	}
	var token [1]byte
	read, err := gate.Read(token[:])
	_ = gate.Close()
	if err != nil || read != 1 || token[0] != 'G' {
		return errors.New("runtime lock child barrier was not released")
	}
	command := args[1:]
	executable, err := exec.LookPath(command[0])
	if err != nil {
		return err
	}
	return unix.Exec(executable, command, os.Environ())
}

func runRuntimeLockWatchdog(args []string) error {
	if len(args) != 1 {
		return errors.New("invalid internal runtime lock watchdog argv")
	}
	processGroup, err := strconv.Atoi(args[0])
	if err != nil || processGroup <= 1 {
		return errors.New("invalid internal runtime lock process group")
	}
	watch := os.NewFile(3, "runtime-lock-watchdog")
	if watch == nil {
		return errors.New("runtime lock watchdog pipe is unavailable")
	}
	var token [1]byte
	read, readErr := watch.Read(token[:])
	_ = watch.Close()
	if read == 1 && readErr == nil && token[0] == 'C' {
		return nil
	}
	if err := unix.Kill(-processGroup, unix.SIGKILL); err != nil && !errors.Is(err, unix.ESRCH) {
		return fmt.Errorf("kill abandoned runtime lock process group: %w", err)
	}
	return nil
}

func runtimeLockEnvironment(current []string, replacements map[string]string) []string {
	result := make([]string, 0, len(current)+len(replacements))
	for _, value := range current {
		name, _, found := strings.Cut(value, "=")
		if found {
			if _, replaced := replacements[name]; replaced {
				continue
			}
		}
		result = append(result, value)
	}
	keys := make([]string, 0, len(replacements))
	for key := range replacements {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		result = append(result, key+"="+replacements[key])
	}
	return result
}

type commandExitError struct{ code int }

func (e commandExitError) Error() string {
	return fmt.Sprintf("locked command exited with status %d", e.code)
}

func validMigrationTemporaryName(kind, name string) bool {
	if kind != "staging" && kind != "namespace" {
		return false
	}
	return regexp.MustCompile(`^` + regexp.QuoteMeta(kind) + `\.[0-9a-f]{16}$`).MatchString(name)
}

func parseMigrationIdentity(value string) (migrationIdentity, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return migrationIdentity{}, errors.New("invalid migration identity token")
	}
	device, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return migrationIdentity{}, errors.New("invalid migration identity token")
	}
	inode, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil || inode == 0 {
		return migrationIdentity{}, errors.New("invalid migration identity token")
	}
	return migrationIdentity{Device: device, Inode: inode}, nil
}

//nolint:gocyclo // Removal validates every ownership property before a single unlink operation.
func removeMigrationTemporary(parent, kind, name string, expected migrationIdentity, requiredUID uint32) error {
	if !validMigrationTemporaryName(kind, name) {
		return errors.New("unsupported migration temporary name")
	}
	directory, err := fsowner.OpenDirectory(parent)
	if err != nil {
		return err
	}
	defer closeIgnoringError(directory)
	parentIdentity := directory.Identity()
	if parentIdentity.UID != requiredUID || parentIdentity.Mode&0o022 != 0 {
		return fmt.Errorf("ownership collision: migration parent is not owned and private")
	}
	entry, err := directory.Inspect(name)
	if err != nil {
		return err
	}
	if entry.Identity.Device != expected.Device || entry.Identity.Inode != expected.Inode {
		return fmt.Errorf("ownership collision: migration temporary identity changed")
	}
	switch kind {
	case "staging":
		if entry.Identity.Kind != fsowner.KindDirectory || entry.Identity.UID != requiredUID || entry.Identity.Mode&0o777 != 0o700 {
			return fmt.Errorf("ownership collision: migration staging path is not an owned mode 0700 directory")
		}
		return directory.RemoveDirectory(name, entry.Identity, fsowner.RemoveOptions{
			UID:        requiredUID,
			Recursive:  true,
			SameDevice: true,
		})
	case "namespace":
		if entry.Identity.Kind != fsowner.KindRegular || entry.Identity.UID != requiredUID ||
			entry.Identity.Links != 1 || entry.Identity.Mode&0o777 != 0o600 {
			return fmt.Errorf("ownership collision: migration namespace snapshot is not an owned mode 0600 regular file")
		}
		return directory.RemoveRegular(name, entry.Identity, fsowner.RemoveOptions{UID: requiredUID})
	default:
		return errors.New("unsupported migration temporary kind")
	}
}

func requireRoot() error {
	if os.Geteuid() != 0 {
		return errors.New("privileged filesystem helper must run as root")
	}
	return nil
}

func runtimePrefix(kind string) (string, error) {
	switch kind {
	case "runtime":
		return ".runtime-rollback-v1-", nil
	case "state":
		return ".state-switch-rollback-v1-", nil
	default:
		return "", errors.New("unsupported runtime transaction kind")
	}
}

func runtimeMode(value string) (os.FileMode, error) {
	switch value {
	case "0600":
		return 0o600, nil
	case "0644":
		return 0o644, nil
	default:
		return 0, errors.New("unsupported runtime file mode")
	}
}

func readRuntimePayload(reader io.Reader) ([]byte, error) {
	const limit = 16 << 20
	payload, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > limit {
		return nil, errors.New("runtime payload exceeds 16 MiB")
	}
	return payload, nil
}

func canonicalNixOSBackupPath(path string) bool {
	clean := filepath.Clean(path)
	if clean != path || filepath.Dir(clean) != "/etc" {
		return false
	}
	return regexp.MustCompile(`^nixos\.bak\.[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(filepath.Base(clean))
}

func canonicalPasswordModuleSources() map[string]struct{} {
	return map[string]struct{}{
		"/etc/nixos/hashed-password.nix":             {},
		"/etc/nixos/hosts/NixOS/hashed-password.nix": {},
	}
}

func sortedPasswordModuleSources(allowed map[string]struct{}) []string {
	sources := make([]string, 0, len(allowed))
	for source := range allowed {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	return sources
}

func functionalPasswordModule(namespace string) string {
	return fmt.Sprintf(`{ config, ... }:

{
  users.users.${config.%s.user.username}.hashedPasswordFile = "/etc/wahrwelt/hashed-password";
}
`, namespace)
}

func parseGeneratedPasswordModuleDetails(data []byte) (namespace, hash string, err error) {
	matches := generatedPasswordModuleDetailsPattern.FindSubmatch(data)
	if len(matches) != 3 || !sha512CryptPattern.Match(matches[2]) {
		return "", "", errors.New("unrecognized generated password module")
	}
	return string(matches[1]), string(matches[2]), nil
}

//nolint:gocyclo // The migration deliberately keeps journal, scrub, stub, and visibility checks in order.
func migrateGeneratedPasswordModules(
	sources []string,
	destination, journalPath string,
	requiredUID uint32,
	hooks passwordMigrationHooks,
) (string, error) {
	existingJournal, err := loadPasswordMigrationJournal(journalPath, requiredUID)
	if err != nil {
		return "", err
	}
	candidates, err := pinPasswordModuleCandidates(sources, requiredUID, existingJournal)
	if err != nil {
		return "", err
	}
	defer closePasswordModuleCandidates(candidates)
	if len(candidates) == 0 {
		return "absent", nil
	}
	hash := ""
	for _, candidate := range candidates {
		if candidate.hash == "" {
			continue
		}
		if hash != "" && hash != candidate.hash {
			return "", errors.New("ownership collision: generated password modules disagree")
		}
		hash = candidate.hash
	}
	external, externalIdentity, externalData, err := prepareExternalPasswordHash(destination, hash, requiredUID)
	if err != nil {
		return "", err
	}
	defer closeIgnoringError(external)
	journal := passwordMigrationJournal{Version: passwordMigrationV1, Entries: make([]passwordMigrationEntry, 0, len(candidates))}
	for _, candidate := range candidates {
		journal.Entries = append(journal.Entries, passwordMigrationEntry{
			Path:      candidate.path,
			Device:    candidate.identity.Device,
			Inode:     candidate.identity.Inode,
			UID:       candidate.identity.UID,
			Namespace: candidate.namespace,
		})
	}
	if err := ensurePasswordMigrationJournal(journalPath, journal, requiredUID); err != nil {
		return "", err
	}
	if hooks.BeforeSanitize != nil {
		if err := hooks.BeforeSanitize(); err != nil {
			return "", err
		}
	}
	var visibilityErrors []error
	for _, candidate := range candidates {
		if !candidate.stub && !candidate.scrubbed {
			module, readErr := readOpenRegular(candidate.file)
			if readErr != nil {
				return "", readErr
			}
			if err := sanitizeOpenGeneratedPasswordModule(
				candidate.file,
				candidate.identity,
				module,
				candidate.hash,
				[]byte(functionalPasswordModule(candidate.namespace)),
				func() error {
					if hooks.AfterScrub == nil {
						return nil
					}
					return hooks.AfterScrub(candidate.path)
				},
				func() error {
					if hooks.AfterStub == nil {
						return nil
					}
					return hooks.AfterStub(candidate.path)
				},
			); err != nil {
				return "", err
			}
		} else if candidate.scrubbed {
			if err := writeFunctionalPasswordStub(
				candidate.file,
				candidate.identity,
				[]byte(functionalPasswordModule(candidate.namespace)),
				func() error {
					if hooks.AfterStub == nil {
						return nil
					}
					return hooks.AfterStub(candidate.path)
				},
			); err != nil {
				return "", err
			}
		}
		if err := verifyVisibleFunctionalPasswordModule(candidate); err != nil {
			visibilityErrors = append(visibilityErrors, err)
		}
	}
	if err := verifyOpenRegularUnchanged(external, externalIdentity, externalData); err != nil {
		return "", fmt.Errorf("ownership collision: external password hash changed during migration: %w", err)
	}
	if err := verifyVisibleRegularIdentity(destination, externalIdentity); err != nil {
		return "", err
	}
	if len(visibilityErrors) != 0 {
		return "", errors.Join(visibilityErrors...)
	}
	return "migrated", nil
}

//nolint:gocyclo // Candidate pinning rejects every unowned or ambiguous recovery state before mutation.
func pinPasswordModuleCandidates(sources []string, requiredUID uint32, journal *passwordMigrationJournal) ([]*passwordModuleCandidate, error) {
	candidates := make([]*passwordModuleCandidate, 0, len(sources))
	for _, source := range sources {
		file, identity, err := openRegularNoFollow(filepath.Clean(source), unix.O_RDWR)
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, unix.ENOENT) {
			continue
		}
		if err != nil {
			closePasswordModuleCandidates(candidates)
			return nil, err
		}
		data, readErr := readOpenRegular(file)
		if readErr != nil {
			_ = file.Close()
			closePasswordModuleCandidates(candidates)
			return nil, readErr
		}
		candidate := &passwordModuleCandidate{path: filepath.Clean(source), file: file, identity: identity}
		namespace, hash, parseErr := parseGeneratedPasswordModuleDetails(data)
		if parseErr == nil {
			if identity.Kind != fsowner.KindRegular || identity.UID != requiredUID || identity.Links != 1 || identity.Mode&0o777 != 0o600 {
				_ = file.Close()
				closePasswordModuleCandidates(candidates)
				return nil, fmt.Errorf("ownership collision: generated password module metadata is not private at %s", source)
			}
			candidate.namespace, candidate.hash = namespace, hash
		} else {
			for _, namespace := range []string{"mysetup", "wahrwelt"} {
				if bytes.Equal(data, []byte(functionalPasswordModule(namespace))) {
					candidate.namespace, candidate.stub = namespace, true
					break
				}
			}
			if !candidate.stub {
				matches := generatedPasswordModuleDetailsPattern.FindSubmatch(data)
				entry, recorded := passwordMigrationEntryFor(journal, filepath.Clean(source))
				if len(matches) == 3 && recorded && string(matches[1]) == entry.Namespace &&
					allBytesEqual(matches[2], 'x') && len(matches[2]) >= 91 &&
					entry.Device == identity.Device && entry.Inode == identity.Inode && entry.UID == identity.UID {
					candidate.namespace, candidate.scrubbed = entry.Namespace, true
				}
			}
			validStub := candidate.stub && identity.Kind == fsowner.KindRegular && identity.UID == requiredUID &&
				identity.Links == 1 && identity.Mode&0o777 == 0o644
			validScrubbed := candidate.scrubbed && identity.Kind == fsowner.KindRegular && identity.UID == requiredUID &&
				identity.Links == 1 && identity.Mode&0o777 == 0o600
			if !validStub && !validScrubbed {
				_ = file.Close()
				closePasswordModuleCandidates(candidates)
				return nil, fmt.Errorf("ownership collision: unrecognized generated password module at %s", source)
			}
		}
		if err := verifyVisibleRegularIdentity(source, identity); err != nil {
			_ = file.Close()
			closePasswordModuleCandidates(candidates)
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func closePasswordModuleCandidates(candidates []*passwordModuleCandidate) {
	for _, candidate := range candidates {
		if candidate != nil && candidate.file != nil {
			_ = candidate.file.Close()
		}
	}
}

//nolint:gocyclo // External hash creation and validation are one fail-closed ownership check.
func prepareExternalPasswordHash(path, expectedHash string, requiredUID uint32) (*os.File, fsowner.Identity, []byte, error) {
	if err := ensurePasswordHashParent(path, requiredUID); err != nil {
		return nil, fsowner.Identity{}, nil, err
	}
	file, identity, err := openRegularNoFollow(path, unix.O_RDONLY)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, unix.ENOENT) {
		if expectedHash == "" {
			return nil, fsowner.Identity{}, nil, errors.New("functional password module exists without an external password hash")
		}
		if err := createExternalPasswordHash(path, expectedHash, requiredUID); err != nil {
			return nil, fsowner.Identity{}, nil, err
		}
		file, identity, err = openRegularNoFollow(path, unix.O_RDONLY)
	}
	if err != nil {
		return nil, fsowner.Identity{}, nil, err
	}
	data, err := readOpenRegular(file)
	if err != nil {
		_ = file.Close()
		return nil, fsowner.Identity{}, nil, err
	}
	if identity.Kind != fsowner.KindRegular || identity.UID != requiredUID || identity.Links != 1 || identity.Mode&0o777 != 0o600 ||
		!sha512CryptPattern.Match(bytes.TrimSpace(data)) {
		_ = file.Close()
		return nil, fsowner.Identity{}, nil, errors.New("ownership collision: external password hash is not an exact private raw hash")
	}
	if expectedHash != "" && !bytes.Equal(bytes.TrimSpace(data), []byte(expectedHash)) {
		_ = file.Close()
		return nil, fsowner.Identity{}, nil, errors.New("external password hash does not match generated module")
	}
	if err := verifyVisibleRegularIdentity(path, identity); err != nil {
		_ = file.Close()
		return nil, fsowner.Identity{}, nil, err
	}
	return file, identity, data, nil
}

func createExternalPasswordHash(path, hash string, requiredUID uint32) error {
	parent, err := fsowner.OpenDirectory(filepath.Dir(path))
	if err != nil {
		return err
	}
	parentIdentity := parent.Identity()
	_ = parent.Close()
	if parentIdentity.UID != requiredUID || parentIdentity.Mode&0o777 != 0o700 {
		return errors.New("ownership collision: external password hash parent is not owned mode 0700")
	}
	fd, err := unix.Open(path, unix.O_WRONLY|unix.O_CLOEXEC|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return err
	}
	file := newFileFromDescriptor(fd, path)
	if file == nil {
		_ = unix.Close(fd)
		return errors.New("invalid external password hash descriptor")
	}
	payload := []byte(hash + "\n")
	if err := writeAllAt(file, payload, 0); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return fsyncDirectory(filepath.Dir(path))
}

//nolint:gocyclo // Journal publication handles exact-match reuse and no-clobber creation in one transaction.
func ensurePasswordMigrationJournal(path string, expected passwordMigrationJournal, requiredUID uint32) error {
	data, err := json.Marshal(expected)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	existing, identity, openErr := openRegularNoFollow(path, unix.O_RDONLY)
	if openErr == nil {
		defer closeIgnoringError(existing)
		actual, readErr := readOpenRegular(existing)
		if readErr != nil {
			return readErr
		}
		if identity.UID != requiredUID || identity.Links != 1 || identity.Mode&0o777 != 0o600 || !bytes.Equal(actual, data) {
			return errors.New("ownership collision: password migration journal does not match pinned sources")
		}
		return verifyVisibleRegularIdentity(path, identity)
	}
	if !errors.Is(openErr, os.ErrNotExist) && !errors.Is(openErr, unix.ENOENT) {
		return openErr
	}
	fd, err := unix.Open(path, unix.O_WRONLY|unix.O_CLOEXEC|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return err
	}
	file := newFileFromDescriptor(fd, path)
	if file == nil {
		_ = unix.Close(fd)
		return errors.New("invalid password migration journal descriptor")
	}
	if err := writeAllAt(file, data, 0); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return fsyncDirectory(filepath.Dir(path))
}

//nolint:gocyclo // Journal recovery validation is intentionally exhaustive and fail-closed.
func loadPasswordMigrationJournal(path string, requiredUID uint32) (*passwordMigrationJournal, error) {
	file, identity, err := openRegularNoFollow(path, unix.O_RDONLY)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, unix.ENOENT) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer closeIgnoringError(file)
	if identity.UID != requiredUID || identity.Links != 1 || identity.Mode&0o777 != 0o600 {
		return nil, errors.New("ownership collision: password migration journal metadata is not private")
	}
	data, err := readOpenRegular(file)
	if err != nil {
		return nil, err
	}
	var journal passwordMigrationJournal
	if err := json.Unmarshal(data, &journal); err != nil || journal.Version != passwordMigrationV1 || len(journal.Entries) == 0 {
		return nil, errors.New("ownership collision: password migration journal is invalid")
	}
	seen := make(map[string]struct{}, len(journal.Entries))
	for _, entry := range journal.Entries {
		if entry.Path == "" || filepath.Clean(entry.Path) != entry.Path || entry.Inode == 0 ||
			entry.UID != requiredUID || entry.Namespace != "wahrwelt" && entry.Namespace != "mysetup" {
			return nil, errors.New("ownership collision: password migration journal entry is invalid")
		}
		if _, duplicate := seen[entry.Path]; duplicate {
			return nil, errors.New("ownership collision: password migration journal contains a duplicate source")
		}
		seen[entry.Path] = struct{}{}
	}
	if err := verifyVisibleRegularIdentity(path, identity); err != nil {
		return nil, err
	}
	return &journal, nil
}

func passwordMigrationEntryFor(journal *passwordMigrationJournal, path string) (passwordMigrationEntry, bool) {
	if journal == nil {
		return passwordMigrationEntry{}, false
	}
	for _, entry := range journal.Entries {
		if entry.Path == path {
			return entry, true
		}
	}
	return passwordMigrationEntry{}, false
}

func allBytesEqual(data []byte, expected byte) bool {
	if len(data) == 0 {
		return false
	}
	for _, current := range data {
		if current != expected {
			return false
		}
	}
	return true
}

func fsyncDirectory(path string) error {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer closeDescriptorIgnoringError(fd)
	return unix.Fsync(fd)
}

func verifyVisibleFunctionalPasswordModule(candidate *passwordModuleCandidate) error {
	if candidate == nil || candidate.file == nil {
		return errors.New("missing pinned password module")
	}
	actual, err := identityFromOpenFile(candidate.file)
	if err != nil {
		return err
	}
	if actual.Device != candidate.identity.Device || actual.Inode != candidate.identity.Inode || actual.UID != candidate.identity.UID ||
		actual.Kind != fsowner.KindRegular || actual.Links != 1 || actual.Mode&0o777 != 0o644 {
		return fmt.Errorf("ownership collision: password module inode changed at %s", candidate.path)
	}
	data, err := readOpenRegular(candidate.file)
	if err != nil {
		return err
	}
	if !bytes.Equal(data, []byte(functionalPasswordModule(candidate.namespace))) {
		return fmt.Errorf("ownership collision: functional password module changed at %s", candidate.path)
	}
	if err := verifyVisibleRegularObject(candidate.path, candidate.identity); err != nil {
		return fmt.Errorf("ownership collision: password module public name changed after its pinned inode was sanitized at %s: %w", candidate.path, err)
	}
	return nil
}

func pruneBackups(parent string, keep int, requiredUID uint32) error {
	if keep < 0 {
		return errors.New("invalid backup retention request")
	}
	const target = "nixos"
	pattern := regexp.MustCompile(`^` + regexp.QuoteMeta(target) + `\.bak\.([0-9]+)\.([0-9]+)\.([0-9]+)$`)
	directory, err := fsowner.OpenDirectory(parent)
	if err != nil {
		return err
	}
	defer closeIgnoringError(directory)
	entries, err := directory.List(func(name string) bool { return pattern.MatchString(name) })
	if err != nil {
		return err
	}
	candidates := make([]backupCandidate, 0, len(entries))
	for _, entry := range entries {
		matches := pattern.FindStringSubmatch(entry.Name)
		if entry.Identity.Kind != fsowner.KindDirectory || entry.Identity.UID != requiredUID {
			return fmt.Errorf("ownership collision: backup candidate %s/%s is not an owned ordinary directory", parent, entry.Name)
		}
		owned, ownershipErr := hasOwnedBackupMarker(filepath.Join(parent, entry.Name), entry.Identity, requiredUID)
		if ownershipErr != nil {
			return ownershipErr
		}
		if !owned {
			continue
		}
		values := make([]uint64, 3)
		for index := range values {
			value, parseErr := strconv.ParseUint(matches[index+1], 10, 64)
			if parseErr != nil {
				return fmt.Errorf("parse backup candidate %s: %w", entry.Name, parseErr)
			}
			values[index] = value
		}
		candidates = append(candidates, backupCandidate{entry: entry, timestamp: values[0], pid: values[1], attempt: values[2]})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].timestamp != candidates[j].timestamp {
			return candidates[i].timestamp > candidates[j].timestamp
		}
		if candidates[i].pid != candidates[j].pid {
			return candidates[i].pid > candidates[j].pid
		}
		return candidates[i].attempt > candidates[j].attempt
	})
	for _, candidate := range candidates[min(keep, len(candidates)):] {
		if err := directory.RemoveDirectory(candidate.entry.Name, candidate.entry.Identity, fsowner.RemoveOptions{
			UID:        requiredUID,
			Recursive:  true,
			SameDevice: true,
		}); err != nil {
			return err
		}
	}
	return nil
}

func markBackup(path string, requiredUID uint32) error {
	directory, err := fsowner.OpenDirectory(path)
	if err != nil {
		return err
	}
	expected := directory.Identity()
	if expected.Kind != fsowner.KindDirectory || expected.UID != requiredUID {
		_ = directory.Close()
		return fmt.Errorf("ownership collision: backup is not an owned ordinary directory: %s", path)
	}
	if err := directory.Close(); err != nil {
		return err
	}
	marker := filepath.Join(path, backupMarkerName)
	fd, err := unix.Open(marker, unix.O_WRONLY|unix.O_CLOEXEC|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return fmt.Errorf("create backup ownership marker: %w", err)
	}
	file := newFileFromDescriptor(fd, marker)
	if file == nil {
		_ = unix.Close(fd)
		return errors.New("create backup ownership marker: invalid file descriptor")
	}
	data := []byte(backupMarkerVersion + "\n" + expected.String() + "\n")
	writeErr := error(nil)
	if _, err := file.Write(data); err != nil {
		writeErr = err
	} else if err := file.Sync(); err != nil {
		writeErr = err
	}
	if err := file.Close(); writeErr == nil {
		writeErr = err
	}
	if writeErr != nil {
		return fmt.Errorf("write backup ownership marker: %w", writeErr)
	}
	opened, err := fsowner.OpenDirectory(path)
	if err != nil {
		return err
	}
	actual := opened.Identity()
	_ = opened.Close()
	if actual != expected {
		return fmt.Errorf("ownership collision: backup changed while marking: %s", path)
	}
	owned, err := hasOwnedBackupMarker(path, expected, requiredUID)
	if err != nil {
		return err
	}
	if !owned {
		return fmt.Errorf("backup ownership marker could not be verified: %s", path)
	}
	return nil
}

func hasOwnedBackupMarker(path string, expected fsowner.Identity, requiredUID uint32) (bool, error) {
	marker := filepath.Join(path, backupMarkerName)
	data, identity, err := readRegularNoFollowWithIdentity(marker)
	if err != nil {
		if errors.Is(err, unix.ENOENT) || os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("inspect backup ownership marker: %w", err)
	}
	if identity.UID != requiredUID || identity.Links != 1 || identity.Kind != fsowner.KindRegular || identity.Mode&0o777 != 0o600 {
		return false, fmt.Errorf("ownership collision: backup marker metadata is not private at %s", marker)
	}
	want := backupMarkerVersion + "\n" + expected.String() + "\n"
	if string(data) != want {
		return false, fmt.Errorf("ownership collision: backup marker does not match directory identity at %s", marker)
	}
	return true, nil
}

func openRegularNoFollow(path string, flags int) (*os.File, fsowner.Identity, error) {
	fd, err := unix.Open(path, flags|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, fsowner.Identity{}, err
	}
	file := newFileFromDescriptor(fd, path)
	if file == nil {
		_ = unix.Close(fd)
		return nil, fsowner.Identity{}, errors.New("invalid regular file descriptor")
	}
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		_ = file.Close()
		return nil, fsowner.Identity{}, err
	}
	identity := identityFromStat(stat)
	if identity.Kind != fsowner.KindRegular {
		_ = file.Close()
		return nil, identity, fmt.Errorf("ownership collision: %s is not a regular file", path)
	}
	return file, identity, nil
}

func readOpenRegular(file *os.File) ([]byte, error) {
	data, err := io.ReadAll(io.NewSectionReader(file, 0, maxPasswordModuleBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxPasswordModuleBytes {
		return nil, fmt.Errorf("file exceeds allowed size: %s", file.Name())
	}
	return data, nil
}

func sanitizeOpenGeneratedPasswordModule(
	file *os.File,
	expected fsowner.Identity,
	module []byte,
	hash string,
	stub []byte,
	afterScrub func() error,
	afterStub func() error,
) error {
	current, err := identityFromOpenFile(file)
	if err != nil {
		return err
	}
	if current != expected {
		return fmt.Errorf("ownership collision: generated password module changed before sanitization")
	}
	hashBytes := []byte(hash)
	if bytes.Count(module, hashBytes) != 1 {
		return errors.New("generated password module hash is not unique")
	}
	offset := bytes.Index(module, hashBytes)
	if err := writeAllAt(file, bytes.Repeat([]byte{'x'}, len(hashBytes)), int64(offset)); err != nil {
		return fmt.Errorf("scrub generated password hash: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync scrubbed generated password hash: %w", err)
	}
	if afterScrub != nil {
		if err := afterScrub(); err != nil {
			return err
		}
	}
	return writeFunctionalPasswordStub(file, expected, stub, afterStub)
}

//nolint:gocyclo // The scrubbed inode must complete every write, sync, mode, and identity transition in order.
func writeFunctionalPasswordStub(file *os.File, expected fsowner.Identity, stub []byte, afterStub func() error) error {
	current, err := identityFromOpenFile(file)
	if err != nil {
		return err
	}
	if current.Device != expected.Device || current.Inode != expected.Inode || current.Kind != fsowner.KindRegular ||
		current.UID != expected.UID || current.Links != 1 || current.Mode&0o777 != 0o600 {
		return fmt.Errorf("ownership collision: scrubbed password module inode changed")
	}
	if err := writeAllAt(file, stub, 0); err != nil {
		return fmt.Errorf("write sanitized password module: %w", err)
	}
	if err := file.Truncate(int64(len(stub))); err != nil {
		return fmt.Errorf("truncate sanitized password module: %w", err)
	}
	if err := file.Chmod(0o644); err != nil {
		return fmt.Errorf("chmod sanitized password module: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync sanitized password module: %w", err)
	}
	if afterStub != nil {
		if err := afterStub(); err != nil {
			return err
		}
	}
	post, err := identityFromOpenFile(file)
	if err != nil {
		return err
	}
	if post.Device != expected.Device || post.Inode != expected.Inode || post.Kind != fsowner.KindRegular ||
		post.UID != expected.UID || post.Links != 1 || post.Mode&0o777 != 0o644 {
		return fmt.Errorf("ownership collision: sanitized password module inode changed")
	}
	actual, err := readOpenRegular(file)
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, stub) {
		return errors.New("sanitized password module failed exact verification")
	}
	return nil
}

func identityFromOpenFile(file *os.File) (fsowner.Identity, error) {
	var stat unix.Stat_t
	fd, err := descriptorNumber(file)
	if err != nil {
		return fsowner.Identity{}, err
	}
	if err := unix.Fstat(fd, &stat); err != nil {
		return fsowner.Identity{}, err
	}
	return identityFromStat(stat), nil
}

func writeAllAt(file *os.File, payload []byte, offset int64) error {
	for len(payload) != 0 {
		written, err := file.WriteAt(payload, offset)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		offset += int64(written)
		payload = payload[written:]
	}
	return nil
}

func verifyOpenRegularUnchanged(file *os.File, expected fsowner.Identity, expectedData []byte) error {
	actual, err := identityFromOpenFile(file)
	if err != nil {
		return err
	}
	if actual != expected {
		return errors.New("identity changed")
	}
	data, err := readOpenRegular(file)
	if err != nil {
		return err
	}
	if !bytes.Equal(data, expectedData) {
		return errors.New("content changed")
	}
	return nil
}

func verifyVisibleRegularIdentity(path string, expected fsowner.Identity) error {
	var stat unix.Stat_t
	if err := unix.Lstat(path, &stat); err != nil {
		return err
	}
	actual := identityFromStat(stat)
	if actual != expected {
		return fmt.Errorf("ownership collision: %s identity changed", path)
	}
	return nil
}

func verifyVisibleRegularObject(path string, expected fsowner.Identity) error {
	var stat unix.Stat_t
	if err := unix.Lstat(path, &stat); err != nil {
		return err
	}
	actual := identityFromStat(stat)
	if actual.Device != expected.Device || actual.Inode != expected.Inode || actual.Kind != fsowner.KindRegular ||
		actual.UID != expected.UID || actual.Links != 1 || actual.Mode&0o777 != 0o644 {
		return fmt.Errorf("%s no longer names the sanitized inode", path)
	}
	return nil
}

func validateExternalPasswordHash(path string, requiredUID uint32) error {
	raw, identity, err := readRegularNoFollowWithIdentity(path)
	if err != nil {
		return err
	}
	return validatePasswordHashSnapshot(raw, identity, requiredUID)
}

func validatePasswordHashSnapshot(raw []byte, identity fsowner.Identity, requiredUID uint32) error {
	if identity.UID != requiredUID || identity.Links != 1 || identity.Kind != fsowner.KindRegular || identity.Mode&0o777 != 0o600 {
		return fmt.Errorf("ownership collision: external password hash metadata is not private")
	}
	if !sha512CryptPattern.Match(bytes.TrimSpace(raw)) {
		return errors.New("unrecognized raw password hash")
	}
	return nil
}

func passwordHashStatusToken(identity fsowner.Identity, raw []byte) fsowner.Identity {
	hasher := sha256.New()
	_, _ = io.WriteString(hasher, "wahrwelt-password-hash-status-v2\x00")
	_, _ = io.WriteString(hasher, identity.String())
	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write(raw)
	digest := hasher.Sum(nil)
	token := identity
	token.Device = binary.BigEndian.Uint64(digest[:8])
	token.Inode = binary.BigEndian.Uint64(digest[8:16]) | 1<<63
	return token
}

func passwordHashStatus(path string, requiredUID uint32) (string, error) {
	parentPath := filepath.Dir(path)
	info, err := os.Lstat(parentPath)
	if errors.Is(err, os.ErrNotExist) {
		return "absent", nil
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("ownership collision: password hash parent is not an ordinary directory: %s", parentPath)
	}
	parent, err := fsowner.OpenDirectory(parentPath)
	if err != nil {
		return "", err
	}
	defer closeIgnoringError(parent)
	parentIdentity := parent.Identity()
	if parentIdentity.UID != requiredUID || parentIdentity.Mode&0o777 != 0o700 {
		return "", fmt.Errorf("ownership collision: password hash parent is not owned mode 0700: %s", parentPath)
	}
	entry, err := parent.Inspect(filepath.Base(path))
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, unix.ENOENT) {
		return "absent", nil
	}
	if err != nil {
		return "", err
	}
	raw, identity, err := readRegularNoFollowWithIdentity(path)
	if err != nil {
		return "", err
	}
	if identity != entry.Identity {
		return "", errors.New("ownership collision: password hash changed during status inspection")
	}
	if err := validatePasswordHashSnapshot(raw, identity, requiredUID); err != nil {
		return "", err
	}
	if err := verifyVisibleRegularIdentity(path, identity); err != nil {
		return "", err
	}
	return passwordHashStatusToken(identity, raw).String(), nil
}

func ensurePasswordHashParent(path string, requiredUID uint32) error {
	parentPath := filepath.Dir(path)
	if _, err := os.Lstat(parentPath); errors.Is(err, os.ErrNotExist) {
		grandparent, openErr := fsowner.OpenDirectory(filepath.Dir(parentPath))
		if openErr != nil {
			return openErr
		}
		grandparentIdentity := grandparent.Identity()
		if grandparentIdentity.UID != requiredUID || grandparentIdentity.Mode&0o022 != 0 {
			_ = grandparent.Close()
			return fmt.Errorf("ownership collision: password hash grandparent is not owned and protected: %s", filepath.Dir(parentPath))
		}
		if mkdirErr := os.Mkdir(parentPath, 0o700); mkdirErr != nil && !errors.Is(mkdirErr, os.ErrExist) {
			_ = grandparent.Close()
			return mkdirErr
		}
		_ = grandparent.Close()
	} else if err != nil {
		return err
	}
	status, err := passwordHashStatus(path, requiredUID)
	if err != nil {
		return err
	}
	if status != "absent" {
		return nil
	}
	return nil
}

var sealedDescriptorPattern = regexp.MustCompile(`^/proc/[1-9][0-9]*/fd/[0-9]+$`)

func readSealedPasswordHashSource(path string) ([]byte, error) {
	if !sealedDescriptorPattern.MatchString(path) {
		return nil, errors.New("password hash source must be an exact caller /proc/<pid>/fd/<fd> path")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer closeIgnoringError(file)
	var info unix.Stat_t
	fd, err := descriptorNumber(file)
	if err != nil {
		return nil, err
	}
	if err := unix.Fstat(fd, &info); err != nil {
		return nil, err
	}
	if info.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, errors.New("password hash source is not a regular sealed descriptor")
	}
	requiredSeals := unix.F_SEAL_WRITE | unix.F_SEAL_GROW | unix.F_SEAL_SHRINK | unix.F_SEAL_SEAL
	seals, err := unix.FcntlInt(file.Fd(), unix.F_GET_SEALS, 0)
	if err != nil || seals&requiredSeals != requiredSeals {
		return nil, errors.New("password hash source is not sealed against mutation")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxPasswordModuleBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxPasswordModuleBytes {
		return nil, errors.New("password hash source exceeds the allowed size")
	}
	hash := bytes.TrimSpace(data)
	if !sha512CryptPattern.Match(hash) {
		return nil, errors.New("unrecognized raw password hash source")
	}
	return append(append([]byte(nil), hash...), '\n'), nil
}

func expectedPasswordHashIdentity(text string) (fsowner.Identity, error) {
	if text == "absent" {
		return fsowner.Identity{Kind: fsowner.KindAbsent}, nil
	}
	identity, err := parseIdentity(text)
	if err != nil {
		return fsowner.Identity{}, err
	}
	uid, err := effectiveUID()
	if err != nil {
		return fsowner.Identity{}, err
	}
	if identity.Kind != fsowner.KindRegular || identity.UID != 0 && identity.UID != uid || identity.Links != 1 || identity.Mode&0o777 != 0o600 {
		return fsowner.Identity{}, errors.New("password hash expected identity is not a private regular file")
	}
	return identity, nil
}

func cleanupFailedPasswordTransaction(transaction string, cause error) error {
	rollbackErr := fsowner.Rollback(transaction)
	commitErr := error(nil)
	if rollbackErr == nil {
		commitErr = fsowner.Commit(transaction)
	}
	return errors.Join(cause, rollbackErr, commitErr)
}

//nolint:gocyclo // Publication is a fail-closed transaction across lock, identity, rollback, and verification steps.
func publishPasswordHash(path, source, expectedText string, requiredUID uint32, transactionRoot string) error {
	payload, err := readSealedPasswordHashSource(source)
	if err != nil {
		return err
	}
	if err := ensurePasswordHashParent(path, requiredUID); err != nil {
		return err
	}
	lockFD, err := unix.Open(filepath.Dir(path), unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer closeDescriptorIgnoringError(lockFD)
	if err := unix.Flock(lockFD, unix.LOCK_EX); err != nil {
		return fmt.Errorf("lock password hash publication: %w", err)
	}
	expectedToken, err := expectedPasswordHashIdentity(expectedText)
	if err != nil {
		return err
	}
	if expectedToken.Kind == fsowner.KindRegular && expectedToken.UID != requiredUID {
		return errors.New("password hash expected identity has the wrong owner")
	}
	pinnedActual, actualIdentity, err := openRegularNoFollow(path, unix.O_RDONLY)
	var actualToken fsowner.Identity
	var actualData []byte
	switch {
	case errors.Is(err, os.ErrNotExist) || errors.Is(err, unix.ENOENT):
		pinnedActual = nil
		actualIdentity = fsowner.Identity{Kind: fsowner.KindAbsent}
		actualToken = actualIdentity
	case err != nil:
		return fmt.Errorf("ownership collision: inspect password hash before publication: %w", err)
	default:
		defer closeIgnoringError(pinnedActual)
		actualData, err = readOpenRegular(pinnedActual)
		if err != nil {
			return err
		}
		if err := validatePasswordHashSnapshot(actualData, actualIdentity, requiredUID); err != nil {
			return err
		}
		if err := verifyVisibleRegularIdentity(path, actualIdentity); err != nil {
			return fmt.Errorf("ownership collision: password hash changed before publication: %w", err)
		}
		actualToken = passwordHashStatusToken(actualIdentity, actualData)
	}
	if actualToken != expectedToken {
		return fmt.Errorf("ownership collision: password hash changed before publication")
	}
	if _, statErr := os.Lstat(transactionRoot); statErr == nil {
		result, scavengeErr := fsowner.Scavenge(transactionRoot, []string{passwordTransactionPre})
		if scavengeErr != nil {
			return scavengeErr
		}
		if len(result.Issues) != 0 {
			return fmt.Errorf("owned password transaction cleanup incomplete: %s", strings.Join(result.Issues, "; "))
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	transaction, err := fsowner.BeginExpected(
		transactionRoot,
		passwordTransactionPre,
		[]string{path},
		map[string]fsowner.Identity{path: actualIdentity},
	)
	if err != nil {
		return err
	}
	if pinnedActual != nil {
		if verifyErr := verifyOpenRegularUnchanged(pinnedActual, actualIdentity, actualData); verifyErr != nil {
			return cleanupFailedPasswordTransaction(
				transaction,
				fmt.Errorf("ownership collision: password hash changed during transaction start: %w", verifyErr),
			)
		}
		if verifyErr := verifyVisibleRegularIdentity(path, actualIdentity); verifyErr != nil {
			return cleanupFailedPasswordTransaction(
				transaction,
				fmt.Errorf("ownership collision: password hash public name changed during transaction start: %w", verifyErr),
			)
		}
	}
	if err := fsowner.Write(transaction, path, payload, 0o600); err != nil {
		return cleanupFailedPasswordTransaction(transaction, err)
	}
	if err := fsowner.Commit(transaction); err != nil {
		return err
	}
	raw, identity, err := readRegularNoFollowWithIdentity(path)
	if err != nil {
		return err
	}
	if identity.UID != requiredUID || identity.Links != 1 || identity.Mode&0o777 != 0o600 || !bytes.Equal(raw, payload) {
		return errors.New("published password hash failed exact verification")
	}
	return nil
}

func readRegularNoFollowWithIdentity(path string) ([]byte, fsowner.Identity, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fsowner.Identity{}, err
	}
	file := newFileFromDescriptor(fd, path)
	if file == nil {
		_ = unix.Close(fd)
		return nil, fsowner.Identity{}, errors.New("invalid file descriptor")
	}
	defer closeIgnoringError(file)
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return nil, fsowner.Identity{}, err
	}
	identity := identityFromStat(stat)
	if identity.Kind != fsowner.KindRegular {
		return nil, identity, fmt.Errorf("ownership collision: %s is not a regular file", path)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxPasswordModuleBytes+1))
	if err != nil {
		return nil, identity, err
	}
	if len(data) > maxPasswordModuleBytes {
		return nil, identity, fmt.Errorf("file exceeds allowed size: %s", path)
	}
	return data, identity, nil
}

func identityFromStat(stat unix.Stat_t) fsowner.Identity {
	kind := fsowner.KindOther
	switch stat.Mode & unix.S_IFMT {
	case unix.S_IFREG:
		kind = fsowner.KindRegular
	case unix.S_IFDIR:
		kind = fsowner.KindDirectory
	case unix.S_IFLNK:
		kind = fsowner.KindSymlink
	}
	return fsowner.Identity{
		Device: stat.Dev,
		Inode:  stat.Ino,
		Mode:   stat.Mode,
		Links:  stat.Nlink,
		UID:    stat.Uid,
		Kind:   kind,
	}
}

func parseIdentity(text string) (fsowner.Identity, error) {
	parts := strings.Split(text, ":")
	if len(parts) != 6 {
		return fsowner.Identity{}, errors.New("invalid identity token")
	}
	values := make([]uint64, 5)
	for index := range values {
		value, err := strconv.ParseUint(parts[index], 10, 64)
		if err != nil {
			return fsowner.Identity{}, errors.New("invalid identity token")
		}
		values[index] = value
	}
	kind := fsowner.Kind(parts[5])
	switch kind {
	case fsowner.KindRegular, fsowner.KindDirectory, fsowner.KindSymlink, fsowner.KindOther:
	default:
		return fsowner.Identity{}, errors.New("invalid identity kind")
	}
	mode, err := checkedUint32(values[2])
	if err != nil {
		return fsowner.Identity{}, errors.New("identity token overflows")
	}
	uid, err := checkedUint32(values[4])
	if err != nil {
		return fsowner.Identity{}, errors.New("identity token overflows")
	}
	return fsowner.Identity{
		Device: values[0],
		Inode:  values[1],
		Mode:   mode,
		Links:  values[3],
		UID:    uid,
		Kind:   kind,
	}, nil
}
