package dots

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	migrationv1tov2 "github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/migrations/v1_to_v2"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
	"golang.org/x/sys/unix"
)

const wahrweltDefaultLua = `local wahrwelt = require("lib.wahrwelt")

wahrwelt.optional_require("wahrwelt.execs")
wahrwelt.optional_require("wahrwelt.general")
wahrwelt.optional_require("wahrwelt.rules")
wahrwelt.optional_require("wahrwelt.keybinds")
`

type managedHyprEntrypointTargets map[string]struct{}

type wahrweltDefaultSeedStage string

const (
	wahrweltDefaultSeedWrite   wahrweltDefaultSeedStage = "write"
	wahrweltDefaultSeedChmod   wahrweltDefaultSeedStage = "chmod"
	wahrweltDefaultSeedClose   wahrweltDefaultSeedStage = "close"
	wahrweltDefaultSeedPublish wahrweltDefaultSeedStage = "publish"
)

type wahrweltDefaultSeedHook func(stage wahrweltDefaultSeedStage, tempPath, finalPath string) error

type hyprUserMigrationCommitStage string

const hyprUserMigrationBeforeRename hyprUserMigrationCommitStage = "before-rename"

type hyprUserMigrationCommitHook func(stage hyprUserMigrationCommitStage, migration hyprUserMigration) error

type hyprUserWriteStage string

const (
	hyprUserWriteAfterPin            hyprUserWriteStage = "after-pin"
	hyprUserWriteBetweenPublications hyprUserWriteStage = "between-publications"
)

type hyprUserWriteHook func(stage hyprUserWriteStage, userDirectory *pinnedDirectory) error

func writeHyprLocalConfig(ctx context.Context, runner run.CommandRunner, username, hyprSourceDir, hyprDir string) error {
	home := homeDirFromConfigDir(filepath.Dir(hyprDir))
	if err := writeHyprLocalConfigFilesForHome(hyprSourceDir, hyprDir, home); err != nil {
		if (!os.IsPermission(err) && !errors.Is(err, unix.EACCES) && !errors.Is(err, unix.EPERM)) || username == "" {
			return err
		}
		if repairErr := repairUserWritableTree(ctx, runner, hyprDir, username); repairErr != nil {
			return repairErr
		}
		return writeHyprLocalConfigFilesForHome(hyprSourceDir, hyprDir, home)
	}
	return nil
}

func writeHyprLocalConfigFiles(hyprSourceDir, hyprDir string) error {
	return writeHyprLocalConfigFilesForHome(hyprSourceDir, hyprDir, "")
}

func writeHyprLocalConfigFilesForHome(hyprSourceDir, hyprDir, home string) error {
	return writeHyprLocalConfigFilesForHomeWithHook(hyprSourceDir, hyprDir, home, nil)
}

func writeHyprLocalConfigFilesForHomeWithHook(hyprSourceDir, hyprDir, home string, hook hyprUserWriteHook) error {
	sourceHyprland := filepath.Join(hyprSourceDir, "hyprland.lua")
	userDir := filepath.Join(hyprDir, "user")
	userHyprland := filepath.Join(userDir, "hyprland.lua")
	defaultConfig := filepath.Join(userDir, "default.lua")
	currentEntrypoint, err := readManagedHyprEntrypointSource(sourceHyprland)
	if err != nil {
		return err
	}
	activeHomeManagerTargets := activeHomeManagerHyprEntrypointTargets(home)

	if err := migrateLegacyHyprUserTreeWithSourceTargetsAndHook(
		hyprDir,
		currentEntrypoint,
		activeHomeManagerTargets,
		nil,
	); err != nil {
		return err
	}
	hyprRoot, err := openPinnedDirectory(hyprDir)
	if err != nil {
		return err
	}
	defer hyprRoot.close()
	userDirectory, err := pinOrCreateHyprUserDirectory(hyprRoot, userDir)
	if err != nil {
		return err
	}
	defer userDirectory.close()
	if _, err := wahrweltDefaultExistsAt(userDirectory, filepath.Base(defaultConfig), defaultConfig); err != nil {
		return err
	}
	if hook != nil {
		if err := hook(hyprUserWriteAfterPin, userDirectory); err != nil {
			return err
		}
	}
	if err := hyprRoot.checkCanonical(); err != nil {
		return err
	}
	if err := userDirectory.checkCanonical(); err != nil {
		return err
	}
	if err := publishManagedHyprUserEntrypointAtDirectory(
		userDirectory,
		currentEntrypoint,
		filepath.Base(userHyprland),
		userHyprland,
		activeHomeManagerTargets,
		nil,
	); err != nil {
		return err
	}
	if hook != nil {
		if err := hook(hyprUserWriteBetweenPublications, userDirectory); err != nil {
			return err
		}
	}
	if err := hyprRoot.checkCanonical(); err != nil {
		return err
	}
	if err := userDirectory.checkCanonical(); err != nil {
		return err
	}
	return seedWahrweltDefaultAtDirectory(userDirectory, defaultConfig, nil)
}

func migrateLegacyHyprUserTreeWithHook(hyprDir string, hook hyprUserMigrationCommitHook) error {
	return migrateLegacyHyprUserTreeWithSourceAndHook(hyprDir, nil, hook)
}

func migrateLegacyHyprUserTreeWithSourceAndHook(
	hyprDir string,
	currentEntrypoint []byte,
	hook hyprUserMigrationCommitHook,
) error {
	return migrateLegacyHyprUserTreeWithSourceTargetsAndHook(hyprDir, currentEntrypoint, nil, hook)
}

func migrateLegacyHyprUserTreeWithSourceTargetsAndHook(
	hyprDir string,
	currentEntrypoint []byte,
	activeHomeManagerTargets managedHyprEntrypointTargets,
	hook hyprUserMigrationCommitHook,
) error {
	migration, err := preflightHyprUserMigrationWithTargets(hyprDir, currentEntrypoint, activeHomeManagerTargets)
	if err != nil {
		return err
	}
	if migration.source == "" {
		return nil
	}
	return commitHyprUserMigration(migration, hook)
}

type hyprUserMigration struct {
	source                   string
	target                   string
	parentDirectoryInfo      os.FileInfo
	sourceDirectoryInfo      os.FileInfo
	currentEntrypoint        []byte
	activeHomeManagerTargets managedHyprEntrypointTargets
	sourceEntrypointState    managedHyprEntrypointState
}

func commitHyprUserMigration(migration hyprUserMigration, hook hyprUserMigrationCommitHook) error {
	parent, err := commitHyprUserMigrationRetainingParent(migration, hook)
	if parent != nil {
		parent.close()
	}
	return err
}

func commitHyprUserMigrationRetainingParent(
	migration hyprUserMigration,
	hook hyprUserMigrationCommitHook,
) (*migrationPinnedDirectory, error) {
	return commitHyprUserMigrationRetainingParentWithAfterHook(migration, hook, nil)
}

func commitHyprUserMigrationRetainingParentWithAfterHook(
	migration hyprUserMigration,
	hook hyprUserMigrationCommitHook,
	afterRename func(migration hyprUserMigration) error,
) (*migrationPinnedDirectory, error) {
	if migration.source == "" {
		return nil, nil
	}
	parentPath, sourceName, targetName, err := hyprUserMigrationNames(migration)
	if err != nil {
		return nil, err
	}
	parent, err := openPinnedDirectory(parentPath)
	if err != nil {
		return nil, err
	}
	retainParent := false
	defer func() {
		if !retainParent {
			parent.close()
		}
	}()
	sourceDirectory, err := retainHyprUserMigrationSource(parent, migration, sourceName)
	if err != nil {
		return nil, err
	}
	defer sourceDirectory.close()
	if err := renameHyprUserMigration(parent, sourceName, targetName, migration, hook); err != nil {
		return nil, err
	}
	if afterRename != nil {
		if err := afterRename(migration); err != nil {
			return nil, rollbackPinnedHyprUserMigration(
				parent,
				sourceName,
				targetName,
				migration.sourceDirectoryInfo,
				sourceDirectory,
				err,
			)
		}
	}
	rollbackCause, terminalErr := verifyCommittedHyprUserMigration(parent, sourceDirectory, sourceName, targetName, migration)
	if terminalErr != nil {
		return nil, terminalErr
	}
	if rollbackCause != nil {
		return nil, rollbackPinnedHyprUserMigration(
			parent,
			sourceName,
			targetName,
			migration.sourceDirectoryInfo,
			sourceDirectory,
			rollbackCause,
		)
	}
	if err := parent.checkCanonical(); err != nil {
		return nil, rollbackPinnedHyprUserMigration(parent, sourceName, targetName, migration.sourceDirectoryInfo, sourceDirectory, err)
	}
	retainParent = true
	return &migrationPinnedDirectory{
		path:       parent.path,
		file:       parent.file,
		info:       parent.info,
		descriptor: parent.fd(),
	}, nil
}

func hyprUserMigrationNames(migration hyprUserMigration) (string, string, string, error) {
	parentPath := filepath.Dir(migration.source)
	sourceName := filepath.Base(migration.source)
	targetName := filepath.Base(migration.target)
	if parentPath != filepath.Dir(migration.target) || sourceName == "." || targetName == "." {
		return "", "", "", fmt.Errorf("hypr user migration paths do not share one parent: %s, %s", migration.source, migration.target)
	}
	return parentPath, sourceName, targetName, nil
}

func retainHyprUserMigrationSource(parent *pinnedDirectory, migration hyprUserMigration, sourceName string) (*pinnedDirectory, error) {
	if migration.parentDirectoryInfo == nil || !os.SameFile(migration.parentDirectoryInfo, parent.info) {
		return nil, fmt.Errorf("wahrwelt config parent directory changed after preflight: %s", parent.path)
	}
	if err := parent.checkCanonical(); err != nil {
		return nil, err
	}
	anchoredSource := parent.anchoredPath(sourceName)
	currentDirectoryInfo, err := os.Lstat(anchoredSource)
	if err != nil || !sameOrdinaryDirectory(migration.sourceDirectoryInfo, currentDirectoryInfo) {
		return nil, fmt.Errorf("legacy Hypr user directory changed after preflight: %s", migration.source)
	}
	sourceDirectory, err := openPinnedRuntimeChild(parent, sourceName, migration.source)
	if err != nil {
		return nil, fmt.Errorf("legacy Hypr user directory changed while retaining recovery: %s", migration.source)
	}
	if !os.SameFile(migration.sourceDirectoryInfo, sourceDirectory.info) {
		sourceDirectory.close()
		return nil, fmt.Errorf("legacy Hypr user directory changed while retaining recovery: %s", migration.source)
	}
	entrypoint := filepath.Join(anchoredSource, "hyprland.lua")
	currentState, err := snapshotManagedHyprEntrypoint(entrypoint, migration.activeHomeManagerTargets)
	if err != nil || !sameManagedHyprEntrypointState(migration.sourceEntrypointState, currentState) {
		sourceDirectory.close()
		return nil, fmt.Errorf("managed Hypr user adapter changed after preflight: %s", filepath.Join(migration.source, "hyprland.lua"))
	}
	return sourceDirectory, nil
}

func sameOrdinaryDirectory(expected, current os.FileInfo) bool {
	return expected != nil && current != nil && current.Mode()&os.ModeSymlink == 0 && current.IsDir() && os.SameFile(expected, current)
}

func renameHyprUserMigration(
	parent *pinnedDirectory,
	sourceName, targetName string,
	migration hyprUserMigration,
	hook hyprUserMigrationCommitHook,
) error {
	if err := parent.checkCanonical(); err != nil {
		return err
	}
	if hook != nil {
		if err := hook(hyprUserMigrationBeforeRename, migration); err != nil {
			return err
		}
	}
	if err := unix.Renameat2(parent.fd(), sourceName, parent.fd(), targetName, unix.RENAME_NOREPLACE); err != nil {
		if parentErr := parent.checkCanonical(); parentErr != nil {
			return parentErr
		}
		if errors.Is(err, unix.EEXIST) || errors.Is(err, unix.ENOTEMPTY) {
			return fmt.Errorf("canonical Hypr user config appeared during migration; refusing to overwrite: %s", migration.target)
		}
		return fmt.Errorf("atomically rename legacy Hypr user config %s to %s without replacement: %w", migration.source, migration.target, err)
	}
	return nil
}

func verifyCommittedHyprUserMigration(
	parent, sourceDirectory *pinnedDirectory,
	sourceName, targetName string,
	migration hyprUserMigration,
) (rollbackCause, terminalErr error) {
	anchoredTarget := parent.anchoredPath(targetName)
	movedInfo, err := os.Lstat(anchoredTarget)
	if err != nil {
		return nil, fmt.Errorf("inspect moved Hypr user config %s: %w; recovery retained at %s", migration.target, err, currentPinnedDirectoryPath(sourceDirectory))
	}
	if !os.SameFile(migration.sourceDirectoryInfo, movedInfo) {
		return fmt.Errorf("hypr user migration target changed immediately after commit: %s", migration.target), nil
	}
	movedEntrypoint, err := snapshotManagedHyprEntrypoint(
		filepath.Join(anchoredTarget, "hyprland.lua"),
		migration.activeHomeManagerTargets,
	)
	if err != nil || !sameManagedHyprEntrypointState(migration.sourceEntrypointState, movedEntrypoint) {
		return fmt.Errorf("managed Hypr user adapter changed during commit: %s", filepath.Join(migration.source, "hyprland.lua")), nil
	}
	anchoredMigration := migration
	anchoredMigration.source = parent.anchoredPath(sourceName)
	anchoredMigration.target = anchoredTarget
	if err := verifyHyprUserMigrationPostcondition(anchoredMigration); err != nil {
		return err, nil
	}
	return nil, nil
}

func rollbackPinnedHyprUserMigration(
	parent *pinnedDirectory,
	sourceName, targetName string,
	movedInfo os.FileInfo,
	recovery *pinnedDirectory,
	cause error,
) error {
	recoveryPath := currentPinnedDirectoryPath(recovery)
	anchoredSource := parent.anchoredPath(sourceName)
	anchoredTarget := parent.anchoredPath(targetName)
	targetInfo, err := os.Lstat(anchoredTarget)
	if err != nil || !os.SameFile(movedInfo, targetInfo) {
		return fmt.Errorf("%w; Hypr user migration rollback incomplete; recovery retained at %s", cause, recoveryPath)
	}
	if _, err := os.Lstat(anchoredSource); err == nil || !os.IsNotExist(err) {
		return fmt.Errorf("%w; Hypr user migration rollback source is occupied; recovery retained at %s", cause, recoveryPath)
	}
	if err := unix.Renameat2(parent.fd(), targetName, parent.fd(), sourceName, unix.RENAME_NOREPLACE); err != nil {
		return fmt.Errorf("%w; restore legacy Hypr user config without replacement: %w; recovery retained at %s", cause, err, recoveryPath)
	}
	restoredInfo, err := os.Lstat(anchoredSource)
	if err != nil || !os.SameFile(movedInfo, restoredInfo) {
		return fmt.Errorf("%w; Hypr user migration rollback postcondition failed; recovery retained at %s", cause, currentPinnedDirectoryPath(recovery))
	}
	return fmt.Errorf("%w; rolled back Hypr user migration", cause)
}

func verifyHyprUserMigrationPostcondition(migration hyprUserMigration) error {
	if _, err := os.Lstat(migration.source); err == nil {
		return fmt.Errorf("hypr user migration postcondition failed: legacy source remains: %s", migration.source)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect legacy Hypr user source after migration: %w", err)
	}
	info, err := os.Lstat(migration.target)
	if err != nil {
		return fmt.Errorf("hypr user migration postcondition failed: canonical target is missing: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || !os.SameFile(migration.sourceDirectoryInfo, info) {
		return fmt.Errorf("hypr user migration postcondition failed: canonical target is not an ordinary directory: %s", migration.target)
	}
	return nil
}

func preflightHyprUserMigrationWithTargets(
	hyprDir string,
	currentEntrypoint []byte,
	activeHomeManagerTargets managedHyprEntrypointTargets,
) (hyprUserMigration, error) {
	user := filepath.Join(hyprDir, "user")
	legacyPaths := migrationv1tov2.LegacyHyprUserDirectories(hyprDir)
	hyprInfo, hyprExists, err := inspectHyprConfigParent(hyprDir)
	if err != nil {
		return hyprUserMigration{}, err
	}
	if !hyprExists {
		return hyprUserMigration{target: user}, nil
	}
	userExists, err := ordinaryHyprUserDirectory(user)
	if err != nil {
		return hyprUserMigration{}, err
	}
	legacy, err := findLegacyHyprUserDirectory(legacyPaths)
	if err != nil {
		return hyprUserMigration{}, err
	}
	if legacy == "" {
		if err := inspectExistingHyprUserConfig(user, userExists, currentEntrypoint, activeHomeManagerTargets); err != nil {
			return hyprUserMigration{}, err
		}
		return hyprUserMigration{target: user}, nil
	}
	if userExists {
		return hyprUserMigration{}, fmt.Errorf("legacy and canonical Hypr user config directories coexist: %s, %s", legacy, user)
	}
	if _, err := wahrweltDefaultExists(filepath.Join(legacy, "default.lua")); err != nil {
		return hyprUserMigration{}, err
	}
	entrypointState, err := inspectManagedHyprUserEntrypoint(
		filepath.Join(legacy, "hyprland.lua"),
		currentEntrypoint,
		activeHomeManagerTargets,
	)
	if err != nil {
		return hyprUserMigration{}, err
	}
	legacyInfo, err := os.Lstat(legacy)
	if err != nil {
		return hyprUserMigration{}, err
	}
	return hyprUserMigration{
		source:                   legacy,
		target:                   user,
		parentDirectoryInfo:      hyprInfo,
		sourceDirectoryInfo:      legacyInfo,
		currentEntrypoint:        bytes.Clone(currentEntrypoint),
		activeHomeManagerTargets: cloneManagedHyprEntrypointTargets(activeHomeManagerTargets),
		sourceEntrypointState:    entrypointState,
	}, nil
}

func inspectHyprConfigParent(path string) (os.FileInfo, bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, false, fmt.Errorf("refusing unsupported Hypr config parent: %s", path)
	}
	return info, true, nil
}

func findLegacyHyprUserDirectory(paths []string) (string, error) {
	var legacy string
	for _, path := range paths {
		exists, err := ordinaryHyprUserDirectory(path)
		if err != nil {
			return "", err
		}
		if !exists {
			continue
		}
		if legacy != "" {
			return "", fmt.Errorf("both legacy Hypr user config directories exist: %s, %s", legacy, path)
		}
		legacy = path
	}
	return legacy, nil
}

func inspectExistingHyprUserConfig(
	user string,
	exists bool,
	currentEntrypoint []byte,
	activeHomeManagerTargets managedHyprEntrypointTargets,
) error {
	if !exists {
		return nil
	}
	if _, err := wahrweltDefaultExists(filepath.Join(user, "default.lua")); err != nil {
		return err
	}
	_, err := inspectManagedHyprUserEntrypoint(
		filepath.Join(user, "hyprland.lua"),
		currentEntrypoint,
		activeHomeManagerTargets,
	)
	return err
}

type managedHyprEntrypointState struct {
	exists      bool
	info        os.FileInfo
	linkTarget  string
	contentHash [sha256.Size]byte
}

func activeHomeManagerHyprEntrypointTargets(home string) managedHyprEntrypointTargets {
	targets := make(managedHyprEntrypointTargets)
	if home == "" {
		return targets
	}
	currentHome := filepath.Join(home, ".local", "state", "home-manager", "gcroots", "current-home")
	currentHomeInfo, err := os.Lstat(currentHome)
	if err != nil || currentHomeInfo.Mode()&os.ModeSymlink == 0 {
		return targets
	}
	homeFiles, err := filepath.EvalSymlinks(filepath.Join(currentHome, "home-files"))
	if err != nil || !filepath.IsAbs(homeFiles) {
		return targets
	}
	info, err := os.Lstat(homeFiles)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return targets
	}
	for _, namespace := range []string{
		migrationv1tov2.CanonicalUserNamespace,
		migrationv1tov2.LegacyWahrweltNamespace,
		migrationv1tov2.LegacyMySetupNamespace,
	} {
		targets[filepath.Join(homeFiles, ".config", "hypr", namespace, "hyprland.lua")] = struct{}{}
	}
	return targets
}

func cloneManagedHyprEntrypointTargets(targets managedHyprEntrypointTargets) managedHyprEntrypointTargets {
	cloned := make(managedHyprEntrypointTargets, len(targets))
	for target := range targets {
		cloned[target] = struct{}{}
	}
	return cloned
}

func inspectManagedHyprUserEntrypoint(
	path string,
	currentEntrypoint []byte,
	activeHomeManagerTargets managedHyprEntrypointTargets,
) (managedHyprEntrypointState, error) {
	state, err := snapshotManagedHyprEntrypoint(path, activeHomeManagerTargets)
	if err != nil {
		if _, statErr := os.Lstat(path); statErr == nil {
			return managedHyprEntrypointState{}, unownedManagedHyprEntrypointError(path)
		}
		return managedHyprEntrypointState{}, fmt.Errorf("inspect managed Hypr user adapter %s: %w", path, err)
	}
	if !state.exists {
		return state, nil
	}
	if state.info.Mode()&os.ModeSymlink != 0 {
		if !isManagedHomeManagerHyprEntrypointTarget(state.linkTarget, activeHomeManagerTargets) {
			return managedHyprEntrypointState{}, unownedManagedHyprEntrypointError(path)
		}
		if !recognizedManagedHyprEntrypointHash(state.contentHash, currentEntrypoint) {
			return managedHyprEntrypointState{}, unownedManagedHyprEntrypointError(path)
		}
		return state, nil
	}
	if !state.info.Mode().IsRegular() || !recognizedManagedHyprEntrypointHash(state.contentHash, currentEntrypoint) {
		return managedHyprEntrypointState{}, unownedManagedHyprEntrypointError(path)
	}
	return state, nil
}

func snapshotManagedHyprEntrypoint(
	path string,
	activeHomeManagerTargets managedHyprEntrypointTargets,
) (managedHyprEntrypointState, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return managedHyprEntrypointState{}, nil
		}
		return managedHyprEntrypointState{}, err
	}
	state := managedHyprEntrypointState{exists: true, info: info}
	if info.Mode()&os.ModeSymlink != 0 {
		state.linkTarget, err = os.Readlink(path)
		if err != nil {
			return managedHyprEntrypointState{}, err
		}
		if !isManagedHomeManagerHyprEntrypointTarget(state.linkTarget, activeHomeManagerTargets) {
			return state, nil
		}
		data, _, readErr := readRegularFileNoFollowResolved(state.linkTarget)
		if readErr != nil {
			return managedHyprEntrypointState{}, readErr
		}
		state.contentHash = sha256.Sum256(data)
		return state, nil
	}
	if info.Mode().IsRegular() {
		data, openedInfo, readErr := readRegularFileNoFollow(path)
		if readErr != nil {
			return managedHyprEntrypointState{}, readErr
		}
		if !os.SameFile(info, openedInfo) {
			return managedHyprEntrypointState{}, fmt.Errorf("managed Hypr user adapter changed while reading: %s", path)
		}
		state.contentHash = sha256.Sum256(data)
	}
	return state, nil
}

func isManagedHomeManagerHyprEntrypointTarget(
	target string,
	activeHomeManagerTargets managedHyprEntrypointTargets,
) bool {
	if _, ok := activeHomeManagerTargets[target]; ok {
		return true
	}
	return isNixStoreHomeManagerHyprEntrypointTarget(target)
}

func isNixStoreHomeManagerHyprEntrypointTarget(target string) bool {
	const storePrefix = "/nix/store/"
	const objectSuffix = "-home-manager-files"
	if !strings.HasPrefix(target, storePrefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(target, storePrefix), "/")
	if len(parts) != 5 || parts[1] != ".config" || parts[2] != "hypr" || parts[4] != "hyprland.lua" {
		return false
	}
	if parts[3] != migrationv1tov2.CanonicalUserNamespace &&
		parts[3] != migrationv1tov2.LegacyWahrweltNamespace &&
		parts[3] != migrationv1tov2.LegacyMySetupNamespace {
		return false
	}
	object := parts[0]
	if !strings.HasSuffix(object, objectSuffix) {
		return false
	}
	hash := strings.TrimSuffix(object, objectSuffix)
	if len(hash) != 32 {
		return false
	}
	for _, char := range hash {
		if !strings.ContainsRune("0123456789abcdfghijklmnpqrsvwxyz", char) {
			return false
		}
	}
	return true
}

func readRegularFileNoFollowResolved(path string) ([]byte, os.FileInfo, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, nil, err
	}
	return readRegularFileNoFollow(resolved)
}

func readRegularFileNoFollow(path string) ([]byte, os.FileInfo, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, err
	}
	file, err := newFileFromUnixFD(fd, path)
	if err != nil {
		_ = unix.Close(fd)
		return nil, nil, fmt.Errorf("wrap managed Hypr user adapter descriptor: %w", err)
	}
	defer func() { _ = file.Close() }()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !openedInfo.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("managed Hypr user adapter is not a regular file: %s", path)
	}
	pathInfo, err := os.Lstat(path)
	if err != nil || pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() || !os.SameFile(openedInfo, pathInfo) {
		return nil, nil, fmt.Errorf("managed Hypr user adapter changed while opening: %s", path)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, err
	}
	afterInfo, err := file.Stat()
	if err != nil || !os.SameFile(openedInfo, afterInfo) || afterInfo.Size() != int64(len(data)) {
		return nil, nil, fmt.Errorf("managed Hypr user adapter changed while reading: %s", path)
	}
	return data, openedInfo, nil
}

func recognizedManagedHyprEntrypointHash(hash [sha256.Size]byte, currentEntrypoint []byte) bool {
	if len(currentEntrypoint) != 0 && hash == sha256.Sum256(currentEntrypoint) {
		return true
	}
	return migrationv1tov2.IsHistoricalManagedHyprEntrypointDigest(hash)
}

func sameManagedHyprEntrypointState(left, right managedHyprEntrypointState) bool {
	if left.exists != right.exists {
		return false
	}
	if !left.exists {
		return true
	}
	return left.info.Mode() == right.info.Mode() &&
		left.info.Size() == right.info.Size() &&
		os.SameFile(left.info, right.info) &&
		left.linkTarget == right.linkTarget &&
		left.contentHash == right.contentHash
}

func unownedManagedHyprEntrypointError(path string) error {
	return fmt.Errorf("unowned managed Hypr user adapter collision: %s", path)
}

func ordinaryHyprUserDirectory(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false, fmt.Errorf("refusing unsupported Hypr user config path: %s", path)
	}
	return true, nil
}

func pinOrCreateHyprUserDirectory(parent *pinnedDirectory, path string) (*pinnedDirectory, error) {
	if parent == nil {
		return nil, fmt.Errorf("missing pinned Hypr config root for %s", path)
	}
	if filepath.Dir(path) != parent.path || filepath.Base(path) != "user" {
		return nil, fmt.Errorf("hypr user config path is not beneath the pinned root: %s", path)
	}
	child, err := openPinnedRuntimeChild(parent, "user", path)
	if err == nil {
		return validatePinnedHyprUserDirectory(parent, child)
	}
	if !errors.Is(err, unix.ENOENT) {
		return nil, fmt.Errorf("refusing unsupported Hypr user config path %s: %w", path, err)
	}
	return createPinnedHyprUserDirectory(parent, path)
}

func validatePinnedHyprUserDirectory(parent, child *pinnedDirectory) (*pinnedDirectory, error) {
	if err := parent.checkCanonical(); err != nil {
		child.close()
		return nil, err
	}
	if err := child.checkCanonical(); err != nil {
		child.close()
		return nil, err
	}
	return child, nil
}

func createPinnedHyprUserDirectory(parent *pinnedDirectory, path string) (*pinnedDirectory, error) {
	if err := parent.checkCanonical(); err != nil {
		return nil, err
	}
	if err := unix.Mkdirat(parent.fd(), "user", 0o755); err != nil {
		if errors.Is(err, unix.EEXIST) {
			return nil, fmt.Errorf("hypr user config path appeared during creation: %s", path)
		}
		return nil, fmt.Errorf("create Hypr user config directory %s: %w", path, err)
	}
	createdInfo, err := runtimePathInfoAt(parent, "user", path)
	if err != nil {
		return nil, fmt.Errorf("capture newly created Hypr user config identity %s: %w", path, err)
	}
	child, err := openPinnedRuntimeChild(parent, "user", path)
	if err != nil {
		return nil, fmt.Errorf("pin newly created Hypr user config directory %s: %w", path, err)
	}
	if !os.SameFile(createdInfo, child.info) {
		child.close()
		return nil, fmt.Errorf("newly created Hypr user config directory changed before pin: %s", path)
	}
	if err := parent.checkCanonical(); err != nil {
		child.close()
		return nil, err
	}
	if err := child.checkCanonical(); err != nil {
		child.close()
		return nil, fmt.Errorf("newly created Hypr user config directory changed before pin: %w", err)
	}
	return child, nil
}

type pinnedDirectory struct {
	path       string
	file       *os.File
	info       os.FileInfo
	descriptor int
}

func openPinnedDirectory(path string) (*pinnedDirectory, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return nil, fmt.Errorf("refusing non-directory Wahrwelt config collision: %s", path)
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	file, err := newFileFromUnixFD(fd, path)
	if err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("wrap Wahrwelt config directory descriptor: %w", err)
	}
	after, err := file.Stat()
	if err != nil || !after.IsDir() || !os.SameFile(before, after) {
		_ = file.Close()
		return nil, fmt.Errorf("wahrwelt config parent directory changed while opening: %s", path)
	}
	return &pinnedDirectory{path: path, file: file, info: after, descriptor: fd}, nil
}

func (directory *pinnedDirectory) close() {
	_ = directory.file.Close()
}

func (directory *pinnedDirectory) fd() int {
	return directory.descriptor
}

func (directory *pinnedDirectory) anchoredPath(name string) string {
	return fmt.Sprintf("/proc/self/fd/%d/%s", directory.file.Fd(), name)
}

func currentPinnedDirectoryPath(directory *pinnedDirectory) string {
	if directory == nil || directory.file == nil {
		return "<unavailable pinned directory>"
	}
	path, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", directory.file.Fd()))
	if err == nil && filepath.IsAbs(path) {
		return path
	}
	return fmt.Sprintf("%s (inode retained by descriptor during operation)", directory.path)
}

func createPinnedConfigChildDirectory(parent *pinnedDirectory, prefix, label string, mode os.FileMode) (string, *pinnedDirectory, error) {
	if parent == nil {
		return "", nil, fmt.Errorf("missing pinned parent for %s", label)
	}
	for attempt := 0; attempt < 32; attempt++ {
		name, err := randomRuntimeEntryName(prefix)
		if err != nil {
			return "", nil, err
		}
		path := filepath.Join(parent.path, name)
		if err := unix.Mkdirat(parent.fd(), name, uint32(mode.Perm())); err != nil {
			if errors.Is(err, unix.EEXIST) {
				continue
			}
			return "", nil, fmt.Errorf("create %s %s: %w", label, path, err)
		}
		createdInfo, err := runtimePathInfoAt(parent, name, path)
		if err != nil {
			return "", nil, fmt.Errorf("capture created %s identity; recovery retained at %s: %w", label, path, err)
		}
		directory, err := openPinnedRuntimeChild(parent, name, path)
		if err != nil {
			return "", nil, fmt.Errorf("pin created %s; recovery retained at %s: %w", label, path, err)
		}
		if !os.SameFile(createdInfo, directory.info) {
			directory.close()
			return "", nil, fmt.Errorf("created %s changed before pin; recovery retained at %s", label, path)
		}
		if err := unix.Fchmod(directory.fd(), uint32(mode.Perm())); err != nil {
			directory.close()
			return "", nil, fmt.Errorf("set %s permissions; recovery retained at %s: %w", label, path, err)
		}
		current, err := runtimePathInfoAt(parent, name, path)
		if err != nil || !os.SameFile(directory.info, current) {
			recoveryPath := currentPinnedDirectoryPath(directory)
			directory.close()
			return "", nil, fmt.Errorf("created %s changed during pin; transaction recovery retained at %s; unknown entry preserved at %s", label, recoveryPath, path)
		}
		return name, directory, nil
	}
	return "", nil, fmt.Errorf("no collision-free name for %s", label)
}

func (directory *pinnedDirectory) checkCanonical() error {
	current, err := os.Lstat(directory.path)
	if err != nil || current.Mode()&os.ModeSymlink != 0 || !current.IsDir() || !os.SameFile(directory.info, current) {
		return fmt.Errorf("wahrwelt config parent directory changed during publication: %s", directory.path)
	}
	return nil
}

func createAnonymousConfigFile(directory *pinnedDirectory, label string) (*os.File, error) {
	fd, err := unix.Openat(
		directory.fd(),
		".",
		unix.O_RDWR|unix.O_TMPFILE|unix.O_CLOEXEC,
		0o600,
	)
	if err != nil {
		return nil, fmt.Errorf("create anonymous %s publication file: %w", label, err)
	}
	file, err := newFileFromUnixFD(fd, label)
	if err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("wrap anonymous %s publication file: %w", label, err)
	}
	return file, nil
}

func anonymousFilePath(file *os.File) string {
	return fmt.Sprintf("/proc/self/fd/%d", file.Fd())
}

func linkAnonymousConfigFile(file *os.File, directory *pinnedDirectory, name string) error {
	return unix.Linkat(
		unix.AT_FDCWD,
		anonymousFilePath(file),
		directory.fd(),
		name,
		unix.AT_SYMLINK_FOLLOW,
	)
}

func seedWahrweltDefault(path string) error {
	return seedWahrweltDefaultWithHook(path, nil)
}

func seedWahrweltDefaultWithHook(path string, hook wahrweltDefaultSeedHook) error {
	directory, err := openPinnedDirectory(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer directory.close()
	return seedWahrweltDefaultAtDirectory(directory, path, hook)
}

func seedWahrweltDefaultAtDirectory(directory *pinnedDirectory, path string, hook wahrweltDefaultSeedHook) error {
	if directory == nil || filepath.Dir(path) != directory.path {
		return fmt.Errorf("wahrwelt default path is not beneath the pinned user directory: %s", path)
	}
	name := filepath.Base(path)
	exists, err := wahrweltDefaultExistsAt(directory, name, path)
	if err != nil || exists {
		return err
	}

	file, err := createAnonymousConfigFile(directory, "Wahrwelt default")
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	tempPath := anonymousFilePath(file)
	if err := prepareWahrweltDefaultCandidate(file, hook, tempPath, path); err != nil {
		return err
	}
	return publishWahrweltDefaultCandidate(directory, file, name, path)
}

func prepareWahrweltDefaultCandidate(file *os.File, hook wahrweltDefaultSeedHook, tempPath, path string) error {
	if err := callWahrweltDefaultSeedHook(hook, wahrweltDefaultSeedWrite, tempPath, path); err != nil {
		return err
	}
	if _, err := file.WriteString(wahrweltDefaultLua); err != nil {
		return err
	}
	if err := callWahrweltDefaultSeedHook(hook, wahrweltDefaultSeedChmod, tempPath, path); err != nil {
		return err
	}
	if err := file.Chmod(0o644); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := callWahrweltDefaultSeedHook(hook, wahrweltDefaultSeedClose, tempPath, path); err != nil {
		return err
	}
	if err := callWahrweltDefaultSeedHook(hook, wahrweltDefaultSeedPublish, tempPath, path); err != nil {
		return err
	}
	return nil
}

func publishWahrweltDefaultCandidate(directory *pinnedDirectory, file *os.File, name, path string) error {
	if err := directory.checkCanonical(); err != nil {
		return err
	}
	if err := linkAnonymousConfigFile(file, directory, name); err != nil {
		if !errors.Is(err, unix.EEXIST) {
			return fmt.Errorf("publish Wahrwelt default without replacement: %w", err)
		}
		exists, inspectErr := wahrweltDefaultExistsAt(directory, name, path)
		if inspectErr != nil {
			return inspectErr
		}
		if !exists {
			return fmt.Errorf("wahrwelt user config changed while seeding: %s", path)
		}
	}
	return directory.checkCanonical()
}

func callWahrweltDefaultSeedHook(hook wahrweltDefaultSeedHook, stage wahrweltDefaultSeedStage, tempPath, finalPath string) error {
	if hook == nil {
		return nil
	}
	return hook(stage, tempPath, finalPath)
}

func wahrweltDefaultExists(path string) (bool, error) {
	return wahrweltDefaultExistsPath(path, path)
}

func wahrweltDefaultExistsAt(directory *pinnedDirectory, name, displayPath string) (bool, error) {
	return wahrweltDefaultExistsPath(directory.anchoredPath(name), displayPath)
}

func wahrweltDefaultExistsPath(path, displayPath string) (bool, error) {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || info.Mode().IsRegular() {
			return true, nil
		}
		return false, fmt.Errorf("refusing non-regular Wahrwelt user config collision: %s", displayPath)
	}
	if !os.IsNotExist(err) {
		return false, err
	}
	return false, nil
}

func writeRuntimeConfigFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := prepareWritableConfigFile(path); err != nil {
		return err
	}
	return atomicWriteFile(path, []byte(content), 0o644)
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := file.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmp)
		}
	}()

	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func readManagedHyprEntrypointSource(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("managed Hypr user adapter source is not a regular file: %s", path)
	}
	data, openedInfo, err := readRegularFileNoFollow(path)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, openedInfo) {
		return nil, fmt.Errorf("managed Hypr user adapter source changed while reading: %s", path)
	}
	return data, nil
}

func publishManagedHyprUserEntrypointWithHook(source, target string, hook func() error) error {
	data, err := readManagedHyprEntrypointSource(source)
	if err != nil {
		return err
	}
	return publishManagedHyprUserEntrypointWithData(data, target, hook)
}

func publishManagedHyprUserEntrypointWithData(currentEntrypoint []byte, target string, hook func() error) error {
	return publishManagedHyprUserEntrypointWithDataAndTargets(currentEntrypoint, target, nil, hook)
}

func publishManagedHyprUserEntrypointWithDataAndTargets(
	currentEntrypoint []byte,
	target string,
	activeHomeManagerTargets managedHyprEntrypointTargets,
	hook func() error,
) error {
	directory, err := openPinnedDirectory(filepath.Dir(target))
	if err != nil {
		return err
	}
	defer directory.close()
	return publishManagedHyprUserEntrypointAtDirectory(directory, currentEntrypoint, filepath.Base(target), target, activeHomeManagerTargets, hook)
}

func publishManagedHyprUserEntrypointAtDirectory(
	directory *pinnedDirectory,
	currentEntrypoint []byte,
	name,
	target string,
	activeHomeManagerTargets managedHyprEntrypointTargets,
	hook func() error,
) error {
	if directory == nil || filepath.Dir(target) != directory.path || filepath.Base(target) != name {
		return fmt.Errorf("managed Hypr user adapter is not beneath the pinned user directory: %s", target)
	}
	anchoredTarget := directory.anchoredPath(name)
	initialState, err := inspectManagedHyprUserEntrypoint(
		anchoredTarget,
		currentEntrypoint,
		activeHomeManagerTargets,
	)
	if err != nil {
		return fmt.Errorf("inspect managed Hypr user adapter %s: %w", target, err)
	}

	if err := prepareManagedHyprEntrypointPublication(directory, hook); err != nil {
		return err
	}
	if !initialState.exists {
		return publishFreshManagedHyprEntrypoint(directory, name, target, currentEntrypoint)
	}
	unchanged, err := managedHyprEntrypointAlreadyCurrent(
		directory,
		anchoredTarget,
		target,
		initialState,
		currentEntrypoint,
		activeHomeManagerTargets,
	)
	if err != nil || unchanged {
		return err
	}
	return updateManagedHyprEntrypointInPlace(directory, name, target, initialState, currentEntrypoint)
}

func prepareManagedHyprEntrypointPublication(directory *pinnedDirectory, hook func() error) error {
	if hook != nil {
		if err := hook(); err != nil {
			return err
		}
	}
	return directory.checkCanonical()
}

func managedHyprEntrypointAlreadyCurrent(
	directory *pinnedDirectory,
	anchoredTarget, target string,
	initialState managedHyprEntrypointState,
	currentEntrypoint []byte,
	activeHomeManagerTargets managedHyprEntrypointTargets,
) (bool, error) {
	if initialState.info.Mode()&os.ModeSymlink != 0 {
		return verifyUnchangedManagedHyprEntrypoint(directory, anchoredTarget, target, initialState, activeHomeManagerTargets)
	}
	currentHash := sha256.Sum256(currentEntrypoint)
	if initialState.contentHash != currentHash || initialState.info.Mode().Perm() != 0o644 {
		return false, nil
	}
	return verifyUnchangedManagedHyprEntrypoint(directory, anchoredTarget, target, initialState, activeHomeManagerTargets)
}

func verifyUnchangedManagedHyprEntrypoint(
	directory *pinnedDirectory,
	anchoredTarget, target string,
	initialState managedHyprEntrypointState,
	activeHomeManagerTargets managedHyprEntrypointTargets,
) (bool, error) {
	currentState, err := snapshotManagedHyprEntrypoint(anchoredTarget, activeHomeManagerTargets)
	if err != nil || !sameManagedHyprEntrypointState(initialState, currentState) {
		return false, fmt.Errorf("managed Hypr user adapter changed before publication: %s", target)
	}
	if err := directory.checkCanonical(); err != nil {
		return false, err
	}
	return true, nil
}

func publishFreshManagedHyprEntrypoint(
	directory *pinnedDirectory,
	name,
	target string,
	currentEntrypoint []byte,
) error {
	file, err := createAnonymousConfigFile(directory, "managed Hypr user adapter")
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	if _, err := file.Write(currentEntrypoint); err != nil {
		return err
	}
	if err := file.Chmod(0o644); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := directory.checkCanonical(); err != nil {
		return err
	}
	if err := linkAnonymousConfigFile(file, directory, name); err != nil {
		if errors.Is(err, unix.EEXIST) {
			return fmt.Errorf("managed Hypr user adapter appeared before publication: %s", target)
		}
		return fmt.Errorf("publish managed Hypr user adapter without replacement: %w", err)
	}
	openedInfo, err := file.Stat()
	if err != nil {
		return err
	}
	publishedInfo, err := os.Lstat(directory.anchoredPath(name))
	if err != nil || !publishedInfo.Mode().IsRegular() || !os.SameFile(openedInfo, publishedInfo) {
		return fmt.Errorf("managed Hypr user adapter changed during publication: %s", target)
	}
	return directory.checkCanonical()
}

func updateManagedHyprEntrypointInPlace(
	directory *pinnedDirectory,
	name,
	target string,
	initialState managedHyprEntrypointState,
	currentEntrypoint []byte,
) error {
	return updateManagedHyprEntrypointWithExchangeHook(directory, name, target, initialState, currentEntrypoint, nil)
}

const managedHyprEntrypointRecoveryName = "previous-entrypoint"

func updateManagedHyprEntrypointWithExchangeHook(
	directory *pinnedDirectory,
	name,
	target string,
	initialState managedHyprEntrypointState,
	currentEntrypoint []byte,
	beforeExchange func(recoveryPath string) error,
) error {
	candidate, candidateInfo, err := prepareManagedHyprEntrypointCandidate(directory, currentEntrypoint)
	if err != nil {
		return err
	}
	defer func() { _ = candidate.Close() }()
	recovery, recoveryPath, err := prepareManagedHyprEntrypointRecovery(directory, candidate)
	if err != nil {
		return err
	}
	defer recovery.close()
	if err := prepareManagedHyprEntrypointExchange(directory, recovery, recoveryPath, beforeExchange); err != nil {
		return err
	}
	rollback := func(cause error) error {
		return rollbackManagedHyprEntrypointExchange(directory, recovery, recoveryPath, name, target, cause)
	}

	if err := unix.Renameat2(
		directory.fd(),
		name,
		recovery.fd(),
		managedHyprEntrypointRecoveryName,
		unix.RENAME_EXCHANGE,
	); err != nil {
		return fmt.Errorf("exchange managed Hypr adapter with anonymous candidate: %w", err)
	}
	displaced, err := snapshotManagedHyprEntrypoint(recovery.anchoredPath(managedHyprEntrypointRecoveryName), nil)
	if err != nil || !sameManagedHyprEntrypointState(displaced, initialState) {
		return rollback(fmt.Errorf("managed Hypr user adapter changed before publication: %s", target))
	}
	updated, err := snapshotManagedHyprEntrypoint(directory.anchoredPath(name), nil)
	currentHash := sha256.Sum256(currentEntrypoint)
	if err != nil || !updated.exists || !updated.info.Mode().IsRegular() ||
		!os.SameFile(updated.info, candidateInfo) || updated.contentHash != currentHash ||
		updated.info.Mode().Perm() != 0o644 {
		return rollback(fmt.Errorf("managed Hypr user adapter update postcondition failed: %s", target))
	}
	if err := directory.checkCanonical(); err != nil {
		return rollback(err)
	}
	if err := recovery.checkCanonical(); err != nil {
		return rollback(err)
	}
	fmt.Printf("Wahrwelt managed Hypr adapter recovery retained at %s\n", filepath.Join(recoveryPath, managedHyprEntrypointRecoveryName))
	return nil
}

func prepareManagedHyprEntrypointCandidate(directory *pinnedDirectory, content []byte) (*os.File, os.FileInfo, error) {
	candidate, err := createAnonymousConfigFile(directory, "managed Hypr user adapter update")
	if err != nil {
		return nil, nil, err
	}
	fail := func(err error) (*os.File, os.FileInfo, error) {
		_ = candidate.Close()
		return nil, nil, err
	}
	if _, err := candidate.Write(content); err != nil {
		return fail(err)
	}
	if err := candidate.Chmod(0o644); err != nil {
		return fail(err)
	}
	if err := candidate.Sync(); err != nil {
		return fail(err)
	}
	info, err := candidate.Stat()
	if err != nil {
		return fail(err)
	}
	return candidate, info, nil
}

func prepareManagedHyprEntrypointRecovery(directory *pinnedDirectory, candidate *os.File) (*pinnedDirectory, string, error) {
	recoveryName, recovery, err := createPinnedConfigChildDirectory(
		directory,
		".wahrwelt-migration-recovery-hypr-adapter-",
		"managed Hypr adapter recovery",
		0o700,
	)
	if err != nil {
		return nil, "", err
	}
	recoveryPath := filepath.Join(directory.path, recoveryName)
	if err := linkAnonymousConfigFile(candidate, recovery, managedHyprEntrypointRecoveryName); err != nil {
		recovery.close()
		return nil, "", fmt.Errorf("link managed Hypr adapter candidate in recovery %s: %w", recoveryPath, err)
	}
	return recovery, recoveryPath, nil
}

func prepareManagedHyprEntrypointExchange(
	directory, recovery *pinnedDirectory,
	recoveryPath string,
	beforeExchange func(recoveryPath string) error,
) error {
	if err := directory.checkCanonical(); err != nil {
		return err
	}
	if err := recovery.checkCanonical(); err != nil {
		return err
	}
	if beforeExchange == nil {
		return nil
	}
	return beforeExchange(filepath.Join(recoveryPath, managedHyprEntrypointRecoveryName))
}

func rollbackManagedHyprEntrypointExchange(
	directory, recovery *pinnedDirectory,
	recoveryPath, name, target string,
	cause error,
) error {
	currentTarget, targetErr := snapshotManagedHyprEntrypoint(directory.anchoredPath(name), nil)
	displaced, recoveryErr := snapshotManagedHyprEntrypoint(recovery.anchoredPath(managedHyprEntrypointRecoveryName), nil)
	if targetErr != nil || recoveryErr != nil || !regularManagedHyprState(currentTarget) || !regularManagedHyprState(displaced) {
		return fmt.Errorf(
			"%w; exact rollback refused; recoveries retained at %s and %s",
			cause,
			target,
			filepath.Join(recoveryPath, managedHyprEntrypointRecoveryName),
		)
	}
	if err := unix.Renameat2(
		directory.fd(),
		name,
		recovery.fd(),
		managedHyprEntrypointRecoveryName,
		unix.RENAME_EXCHANGE,
	); err != nil {
		return fmt.Errorf(
			"%w; exact rollback exchange failed; recoveries retained at %s and %s: %w",
			cause,
			target,
			filepath.Join(recoveryPath, managedHyprEntrypointRecoveryName),
			err,
		)
	}
	restored, restoredErr := snapshotManagedHyprEntrypoint(directory.anchoredPath(name), nil)
	candidateRecovery, candidateErr := snapshotManagedHyprEntrypoint(recovery.anchoredPath(managedHyprEntrypointRecoveryName), nil)
	if restoredErr != nil || candidateErr != nil ||
		!sameManagedHyprEntrypointState(restored, displaced) ||
		!sameManagedHyprEntrypointState(candidateRecovery, currentTarget) {
		return fmt.Errorf(
			"%w; rollback exchange had an uncertain postcondition; inspect %s and %s",
			cause,
			target,
			filepath.Join(recoveryPath, managedHyprEntrypointRecoveryName),
		)
	}
	return fmt.Errorf("%w; exact managed adapter entry restored", cause)
}

func regularManagedHyprState(state managedHyprEntrypointState) bool {
	return state.exists && state.info != nil && state.info.Mode().IsRegular()
}

func readOpenRegularFile(file *os.File) ([]byte, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(file)
}

func samePathFile(path string, expected os.FileInfo) bool {
	current, err := os.Lstat(path)
	return err == nil && current.Mode()&os.ModeSymlink == 0 && current.Mode().IsRegular() && os.SameFile(expected, current)
}

func prepareWritableConfigFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return os.Remove(path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing to overwrite non-regular config file: %s", path)
	}
	return nil
}
