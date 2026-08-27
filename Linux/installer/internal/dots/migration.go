package dots

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	migrationv1tov2 "github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/migrations/v1_to_v2"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
	"golang.org/x/sys/unix"
)

func migrateLegacyUserPaths(ctx context.Context, runner run.CommandRunner, home string) error {
	return migrateLegacyUserPathsWithHooks(ctx, runner, home, legacyUserMigrationHooks{})
}

func migrateLegacyUserPathsWithHyprMigrationHook(ctx context.Context, runner run.CommandRunner, home string, hook hyprUserMigrationCommitHook) error {
	return migrateLegacyUserPathsWithHooks(ctx, runner, home, legacyUserMigrationHooks{hypr: hook})
}

type legacyUserMigrationHooks struct {
	hypr      hyprUserMigrationCommitHook
	namespace func(index int, parent *migrationPinnedDirectory) error
	link      legacyLinkQuarantineHook
	cache     legacyCacheCommitHook
}

type legacyMigrationHomes struct {
	home       string
	configHome string
	stateHome  string
	cacheHome  string
}

func migrateLegacyUserPathsWithHooks(ctx context.Context, runner run.CommandRunner, home string, hooks legacyUserMigrationHooks) error {
	homes, err := resolveLegacyMigrationHomes(home)
	if err != nil {
		return err
	}
	legacyLinks := migrationv1tov2.LegacyManagedLinks(homes.configHome)
	preflight, err := preflightLegacyUserPaths(homes.home, homes.configHome, homes.stateHome, homes.cacheHome, legacyLinks)
	if err != nil {
		return fmt.Errorf("wahrwelt migration conflict: %w", err)
	}
	journal := legacyMigrationJournal{}
	defer journal.close()
	var linkRecovery *legacyLinkRecovery
	rollback := func(cause error) error {
		rollbackErr := journal.rollback(cause)
		if linkRecovery != nil {
			return fmt.Errorf("%w; link recovery retained at %s", rollbackErr, linkRecovery.path())
		}
		return rollbackErr
	}

	if err := migrateLegacyNamespaceMoves(ctx, runner, preflight.namespaceMoves, hooks.namespace, &journal); err != nil {
		return rollback(err)
	}

	cacheMove := migrationv1tov2.LegacyCacheMove(homes.cacheHome)
	cacheOld := cacheMove.Source
	cacheNew := cacheMove.Target
	deferCacheMerge, err := prepareLegacyCacheCommit(cacheOld, cacheNew, preflight.cacheSource, preflight.cacheTarget)
	if err != nil {
		return rollback(err)
	}

	if err := verifyLegacyPathSnapshots(preflight.hyprPaths); err != nil {
		return rollback(err)
	}
	hyprMove, err := commitLegacyHyprUserMigrationWithRecord(ctx, runner, preflight.hyprMigration, hooks.hypr)
	journal.add(hyprMove)
	if err != nil {
		return rollback(err)
	}

	linkRecovery, err = quarantineLegacyLinksWithHook(ctx, runner, homes.configHome, preflight.links, &journal, hooks.link)
	if linkRecovery != nil {
		defer linkRecovery.close()
	}
	if err != nil {
		return rollback(err)
	}

	if deferCacheMerge {
		cacheHook := buildLegacyCacheCommitHook(runner, hooks.cache, &journal, preflight)
		if err := mergeLegacyCacheWithSnapshotsHook(
			ctx,
			runner,
			cacheOld,
			cacheNew,
			preflight.cacheSource,
			preflight.cacheTarget,
			cacheHook,
		); err != nil {
			return rollback(err)
		}
	}
	if !runner.IsDryRun() {
		if err := verifyCompletedLegacyMigration(&journal, preflight); err != nil {
			return rollback(err)
		}
	}
	if linkRecovery != nil {
		fmt.Printf("Wahrwelt migration link recovery retained at %s\n", linkRecovery.path())
	}
	return nil
}

func resolveLegacyMigrationHomes(home string) (legacyMigrationHomes, error) {
	managedHomeProvided := home != ""
	if !managedHomeProvided {
		resolvedHome, err := os.UserHomeDir()
		if err != nil {
			return legacyMigrationHomes{}, fmt.Errorf("resolve current home for legacy migration: %w", err)
		}
		home = resolvedHome
	}
	homes := legacyMigrationHomes{
		home:       home,
		configHome: filepath.Join(home, ".config"),
		stateHome:  paths.XDGStateHome(home),
		cacheHome:  filepath.Join(home, ".cache"),
	}
	if managedHomeProvided {
		return homes, nil
	}
	if value := os.Getenv("XDG_CONFIG_HOME"); value != "" {
		homes.configHome = value
	}
	if value := os.Getenv("XDG_STATE_HOME"); value != "" {
		homes.stateHome = value
	}
	if value := os.Getenv("XDG_CACHE_HOME"); value != "" {
		homes.cacheHome = value
	}
	return homes, nil
}

func migrateLegacyNamespaceMoves(
	ctx context.Context,
	runner run.CommandRunner,
	plans []legacyNamespaceMove,
	hook func(index int, parent *migrationPinnedDirectory) error,
	journal *legacyMigrationJournal,
) error {
	for index, movePlan := range plans {
		var moveHook legacyPathMoveCommitHook
		if hook != nil {
			moveIndex := index
			moveHook = func(parent *migrationPinnedDirectory) error {
				return hook(moveIndex, parent)
			}
		}
		move, err := moveLegacyPathWithSnapshotHook(ctx, runner, movePlan.source, movePlan.target, true, movePlan.sourceSnapshot, moveHook)
		journal.add(move)
		if err != nil {
			return err
		}
	}
	return nil
}

func buildLegacyCacheCommitHook(
	runner run.CommandRunner,
	hook legacyCacheCommitHook,
	journal *legacyMigrationJournal,
	preflight legacyUserPathsPreflight,
) legacyCacheCommitHook {
	if runner.IsDryRun() {
		return hook
	}
	return func(recovery *migrationPinnedDirectory, pinnedNew string) error {
		if hook != nil {
			if err := hook(recovery, pinnedNew); err != nil {
				return err
			}
		}
		if err := journal.verifyCommittedTargets(); err != nil {
			return err
		}
		return verifyMigratedLegacyPublicNames(preflight, false)
	}
}

func verifyCompletedLegacyMigration(journal *legacyMigrationJournal, preflight legacyUserPathsPreflight) error {
	if err := journal.verifyCommittedTargets(); err != nil {
		return err
	}
	if err := verifyMigratedCacheTarget(preflight); err != nil {
		return err
	}
	return verifyMigratedLegacyPublicNames(preflight, true)
}

func commitLegacyHyprUserMigrationWithRecord(ctx context.Context, runner run.CommandRunner, migration hyprUserMigration, hook hyprUserMigrationCommitHook) (*legacyPathMove, error) {
	if migration.source == "" {
		return nil, nil
	}
	if runner.IsDryRun() {
		return nil, runner.Command(ctx, "mv", "-T", "--no-copy", "--update=none-fail", "--", migration.source, migration.target)
	}
	parent, err := commitHyprUserMigrationRetainingParent(migration, hook)
	if err == nil {
		return &legacyPathMove{
			source:     migration.source,
			target:     migration.target,
			info:       migration.sourceDirectoryInfo,
			parent:     parent,
			sourceName: filepath.Base(migration.source),
			targetName: filepath.Base(migration.target),
		}, nil
	}
	return nil, err
}

type legacyPathSnapshot struct {
	path       string
	exists     bool
	info       os.FileInfo
	parentPath string
	parentInfo os.FileInfo
}

type migrationPinnedDirectory struct {
	path       string
	file       *os.File
	info       os.FileInfo
	descriptor int
	anchor     *migrationPinnedDirectory
	relative   string
}

func pinOrdinaryDirectory(path string) (*migrationPinnedDirectory, error) {
	fd, err := unix.Open(path, unix.O_PATH|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file, fileErr := newFileFromUnixFD(fd, path)
	if fileErr != nil {
		_ = unix.Close(fd)
		return nil, fileErr
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	visible, err := os.Lstat(path)
	if err != nil || visible.Mode()&os.ModeSymlink != 0 || !visible.IsDir() || !os.SameFile(info, visible) {
		_ = file.Close()
		return nil, fmt.Errorf("directory changed while pinning %s", path)
	}
	return &migrationPinnedDirectory{path: path, file: file, info: info, descriptor: fd}, nil
}

func pinDirectoryBeneath(anchor *migrationPinnedDirectory, relative string) (*migrationPinnedDirectory, error) {
	clean := filepath.Clean(relative)
	if clean == "." {
		return nil, fmt.Errorf("refusing to duplicate pinned directory root %s", anchor.path)
	}
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("directory escapes pinned root %s: %s", anchor.path, relative)
	}
	fd, err := unix.Openat2(anchor.descriptor, clean, &unix.OpenHow{
		Flags:   unix.O_PATH | unix.O_DIRECTORY | unix.O_CLOEXEC,
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS,
	})
	if err != nil {
		return nil, err
	}
	file, fileErr := newFileFromUnixFD(fd, filepath.Join(anchor.path, clean))
	if fileErr != nil {
		_ = unix.Close(fd)
		return nil, fileErr
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return &migrationPinnedDirectory{
		path:       filepath.Join(anchor.path, clean),
		file:       file,
		info:       info,
		descriptor: fd,
		anchor:     anchor,
		relative:   clean,
	}, nil
}

func createPinnedRecoveryDirectory(parent *migrationPinnedDirectory, pattern string) (*migrationPinnedDirectory, error) {
	return createPinnedRecoveryDirectoryWithHook(parent, pattern, nil)
}

func createPinnedRecoveryDirectoryWithHook(
	parent *migrationPinnedDirectory,
	pattern string,
	afterCreated func(createdPath string) error,
) (*migrationPinnedDirectory, error) {
	created, err := os.MkdirTemp(parent.procPath(), pattern)
	if err != nil {
		return nil, err
	}
	name := filepath.Base(created)
	createdInfo, err := os.Lstat(created)
	if err != nil {
		return nil, fmt.Errorf("capture created recovery directory identity at %s: %w", filepath.Join(parent.path, name), err)
	}
	if createdInfo.Mode()&os.ModeSymlink != 0 || !createdInfo.IsDir() {
		return nil, fmt.Errorf("created recovery path is not an ordinary directory: %s", filepath.Join(parent.path, name))
	}
	if afterCreated != nil {
		if err := afterCreated(filepath.Join(parent.path, name)); err != nil {
			return nil, fmt.Errorf("after creating recovery directory at %s: %w", filepath.Join(parent.path, name), err)
		}
	}
	return pinCreatedRecoveryDirectory(parent, name, createdInfo)
}

func pinCreatedRecoveryDirectory(
	parent *migrationPinnedDirectory,
	name string,
	createdInfo os.FileInfo,
) (*migrationPinnedDirectory, error) {
	fd, err := unix.Openat2(parent.descriptor, name, &unix.OpenHow{
		Flags:   unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC,
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS,
	})
	if err != nil {
		return nil, fmt.Errorf("pin recovery directory %s: %w", filepath.Join(parent.path, name), err)
	}
	file, fileErr := newFileFromUnixFD(fd, filepath.Join(parent.path, name))
	if fileErr != nil {
		_ = unix.Close(fd)
		return nil, fileErr
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !os.SameFile(createdInfo, info) {
		_ = file.Close()
		return nil, fmt.Errorf("recovery directory changed before pinning %s", filepath.Join(parent.path, name))
	}
	entry, err := os.Lstat(parent.child(name))
	if err != nil || entry.Mode()&os.ModeSymlink != 0 || !entry.IsDir() || !os.SameFile(createdInfo, entry) {
		_ = file.Close()
		return nil, fmt.Errorf("recovery directory changed while pinning %s", filepath.Join(parent.path, name))
	}
	recovery := &migrationPinnedDirectory{
		path:       filepath.Join(parent.path, name),
		file:       file,
		info:       info,
		descriptor: fd,
		anchor:     parent,
		relative:   name,
	}
	if err := recovery.file.Chmod(0o700); err != nil {
		_ = recovery.file.Close()
		return nil, fmt.Errorf("set recovery directory permissions at %s: %w", recovery.path, err)
	}
	if err := parent.verifyVisible(); err != nil {
		_ = recovery.file.Close()
		return nil, err
	}
	return recovery, nil
}

func (d *migrationPinnedDirectory) procPath() string {
	return fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), d.file.Fd())
}

func (d *migrationPinnedDirectory) child(relative string) string {
	clean := filepath.Clean(relative)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		panic("invalid pinned-directory relative path: " + relative)
	}
	return filepath.Join(d.procPath(), clean)
}

func (d *migrationPinnedDirectory) verifyVisible() error {
	if d.anchor != nil {
		fd, err := unix.Openat2(d.anchor.descriptor, d.relative, &unix.OpenHow{
			Flags:   unix.O_PATH | unix.O_DIRECTORY | unix.O_CLOEXEC,
			Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS,
		})
		if err != nil {
			return fmt.Errorf("pinned directory path changed at %s: %w", d.path, err)
		}
		current, fileErr := newFileFromUnixFD(fd, d.path)
		if fileErr != nil {
			_ = unix.Close(fd)
			return fileErr
		}
		info, err := current.Stat()
		closeErr := current.Close()
		if err != nil || !os.SameFile(d.info, info) {
			return fmt.Errorf("pinned directory path identity changed at %s", d.path)
		}
		if closeErr != nil {
			return fmt.Errorf("close pinned directory identity descriptor at %s: %w", d.path, closeErr)
		}
		return nil
	}
	visible, err := os.Lstat(d.path)
	if err != nil {
		return fmt.Errorf("pinned directory path changed at %s: %w", d.path, err)
	}
	if visible.Mode()&os.ModeSymlink != 0 || !visible.IsDir() || !os.SameFile(d.info, visible) {
		return fmt.Errorf("pinned directory path identity changed at %s", d.path)
	}
	return nil
}

func (d *migrationPinnedDirectory) close() {
	if d != nil && d.file != nil {
		_ = d.file.Close()
	}
}

func snapshotAtPinnedPath(snapshot legacyPathSnapshot, pinnedPath string) legacyPathSnapshot {
	snapshot.path = pinnedPath
	snapshot.parentPath = ""
	snapshot.parentInfo = nil
	return snapshot
}

type legacyNamespaceMove struct {
	source         string
	target         string
	sourceSnapshot legacyPathSnapshot
}

type legacyUserPathsPreflight struct {
	hyprMigration  hyprUserMigration
	hyprPaths      []legacyPathSnapshot
	namespaceMoves []legacyNamespaceMove
	cacheSource    legacyPathSnapshot
	cacheTarget    legacyPathSnapshot
	links          []legacyLinkSnapshot
}

func preflightLegacyUserPaths(home, configHome, stateHome, cacheHome string, legacyLinks []string) (legacyUserPathsPreflight, error) {
	var result legacyUserPathsPreflight
	hyprMigration, err := preflightHyprUserMigrationWithTargets(
		filepath.Join(configHome, "hypr"),
		nil,
		activeHomeManagerHyprEntrypointTargets(home),
	)
	if err != nil {
		return result, err
	}
	result.hyprMigration = hyprMigration
	for _, path := range migrationv1tov2.LegacyHyprUserDirectories(filepath.Join(configHome, "hypr")) {
		snapshot, err := snapshotLegacyPath(path)
		if err != nil {
			return result, err
		}
		result.hyprPaths = append(result.hyprPaths, snapshot)
	}
	for _, pair := range migrationv1tov2.LegacyNamespaceMoves(configHome, stateHome) {
		snapshot, err := preflightMoveLegacyPath(pair.Source, pair.Target)
		if err != nil {
			return result, err
		}
		result.namespaceMoves = append(result.namespaceMoves, legacyNamespaceMove{
			source:         pair.Source,
			target:         pair.Target,
			sourceSnapshot: snapshot,
		})
	}
	cacheMove := migrationv1tov2.LegacyCacheMove(cacheHome)
	result.cacheSource, result.cacheTarget, err = preflightMergeLegacyCache(
		cacheMove.Source,
		cacheMove.Target,
	)
	if err != nil {
		return result, err
	}
	result.links, err = snapshotLegacyLinks(configHome, legacyLinks)
	if err != nil {
		return result, err
	}
	return result, nil
}

func snapshotLegacyPath(path string) (legacyPathSnapshot, error) {
	snapshot := legacyPathSnapshot{path: path, parentPath: filepath.Dir(path)}
	parent, pinErr := pinOrdinaryDirectory(snapshot.parentPath)
	if pinErr != nil {
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			return snapshot, nil
		}
		return snapshot, fmt.Errorf("pin legacy namespace parent %s: %w", snapshot.parentPath, pinErr)
	}
	defer parent.close()
	info, err := os.Lstat(parent.child(filepath.Base(path)))
	if err != nil {
		if os.IsNotExist(err) {
			if err := parent.verifyVisible(); err != nil {
				return snapshot, err
			}
			snapshot.parentInfo = parent.info
			return snapshot, nil
		}
		return snapshot, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return snapshot, fmt.Errorf("source namespace must be an ordinary directory: %s", path)
	}
	snapshot.exists = true
	snapshot.info = info
	snapshot.parentInfo = parent.info
	if err := parent.verifyVisible(); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func preflightMoveLegacyPath(oldPath, newPath string) (legacyPathSnapshot, error) {
	snapshot, err := snapshotLegacyPath(oldPath)
	if err != nil {
		return snapshot, err
	}
	_, targetErr := os.Lstat(newPath)
	if targetErr == nil {
		if snapshot.exists {
			return snapshot, fmt.Errorf("both %s and %s exist", oldPath, newPath)
		}
		return snapshot, nil
	}
	if !os.IsNotExist(targetErr) {
		return snapshot, targetErr
	}
	return snapshot, nil
}

func preflightMergeLegacyCache(oldPath, newPath string) (legacyPathSnapshot, legacyPathSnapshot, error) {
	oldSnapshot, err := snapshotLegacyPath(oldPath)
	if err != nil {
		return oldSnapshot, legacyPathSnapshot{}, fmt.Errorf("cache paths must be directories: %s, %s: %w", oldPath, newPath, err)
	}
	if !oldSnapshot.exists {
		return oldSnapshot, legacyPathSnapshot{path: newPath}, nil
	}
	newSnapshot, err := snapshotLegacyPath(newPath)
	if err != nil {
		return oldSnapshot, newSnapshot, fmt.Errorf("cache paths must be directories: %s, %s: %w", oldPath, newPath, err)
	}
	return oldSnapshot, newSnapshot, nil
}

type legacyPathMove struct {
	source     string
	target     string
	info       os.FileInfo
	recovery   *os.File
	parent     *migrationPinnedDirectory
	sourceName string
	targetName string
}

func (m *legacyPathMove) close() {
	if m != nil && m.recovery != nil {
		_ = m.recovery.Close()
		m.recovery = nil
	}
	if m != nil && m.parent != nil {
		m.parent.close()
		m.parent = nil
	}
}

func (m *legacyPathMove) recoveryPath() string {
	if m == nil {
		return "<unavailable migration recovery>"
	}
	if m.recovery != nil {
		if path, err := os.Readlink(fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), m.recovery.Fd())); err == nil && filepath.IsAbs(path) {
			return path
		}
	}
	return m.target
}

type legacyMigrationJournal struct {
	moves []*legacyPathMove
}

func (j *legacyMigrationJournal) add(move *legacyPathMove) {
	if move != nil {
		j.moves = append(j.moves, move)
	}
}

func (j *legacyMigrationJournal) verifyCommittedTargets() error {
	for _, move := range j.moves {
		if err := verifyCommittedLegacyPathMove(move); err != nil {
			return err
		}
	}
	return nil
}

func verifyCommittedLegacyPathMove(move *legacyPathMove) error {
	if move == nil {
		return nil
	}
	if move.parent != nil {
		if err := move.parent.verifyVisible(); err != nil {
			return err
		}
		if _, err := os.Lstat(move.parent.child(move.sourceName)); err == nil {
			return fmt.Errorf("legacy migration source reappeared before final validation: %s", move.source)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect legacy migration source before final validation %s: %w", move.source, err)
		}
		info, err := os.Lstat(move.parent.child(move.targetName))
		if err != nil || !os.SameFile(move.info, info) {
			return fmt.Errorf("canonical migration target changed before final validation: %s", move.target)
		}
		return move.parent.verifyVisible()
	}
	return verifyLegacyPathMovePostcondition(move.source, move.target, move.info)
}

func (j *legacyMigrationJournal) rollback(cause error) error {
	if len(j.moves) == 0 {
		return cause
	}
	var rollbackErrors []error
	var recoveryPaths []string
	for index := len(j.moves) - 1; index >= 0; index-- {
		move := j.moves[index]
		if err := rollbackLegacyPathMove(move); err != nil {
			rollbackErrors = append(rollbackErrors, err)
			recoveryPaths = append(recoveryPaths, move.recoveryPath())
		}
	}
	if len(rollbackErrors) != 0 {
		return fmt.Errorf(
			"%w; rollback incomplete: %w; recovery retained at %s",
			cause,
			errors.Join(rollbackErrors...),
			strings.Join(recoveryPaths, ", "),
		)
	}
	return fmt.Errorf("%w; rolled back committed Wahrwelt namespace changes", cause)
}

func (j *legacyMigrationJournal) close() {
	for _, move := range j.moves {
		move.close()
	}
}

func verifyLegacyPathSnapshot(snapshot legacyPathSnapshot) error {
	path := snapshot.path
	var parent *migrationPinnedDirectory
	if snapshot.parentInfo != nil {
		var err error
		parent, err = pinOrdinaryDirectory(snapshot.parentPath)
		if err != nil || !os.SameFile(snapshot.parentInfo, parent.info) {
			if parent != nil {
				parent.close()
			}
			return fmt.Errorf("legacy namespace parent changed after preflight: %s", snapshot.parentPath)
		}
		defer parent.close()
		path = parent.child(filepath.Base(snapshot.path))
	}
	current, err := os.Lstat(path)
	if !snapshot.exists {
		if err == nil {
			return fmt.Errorf("legacy source appeared after preflight: %s", snapshot.path)
		}
		if os.IsNotExist(err) {
			if parent != nil {
				return parent.verifyVisible()
			}
			return nil
		}
		return fmt.Errorf("inspect legacy source %s after preflight: %w", snapshot.path, err)
	}
	if err != nil {
		return fmt.Errorf("legacy source changed after preflight: %s: %w", snapshot.path, err)
	}
	if !os.SameFile(snapshot.info, current) {
		return fmt.Errorf("legacy source changed after preflight: %s", snapshot.path)
	}
	if parent != nil {
		return parent.verifyVisible()
	}
	return nil
}

func verifyLegacyPathSnapshots(snapshots []legacyPathSnapshot) error {
	for _, snapshot := range snapshots {
		if err := verifyLegacyPathSnapshot(snapshot); err != nil {
			return err
		}
	}
	return nil
}

func verifyLegacyPathAbsentAfterMigration(snapshot legacyPathSnapshot) error {
	path := snapshot.path
	var parent *migrationPinnedDirectory
	if snapshot.parentInfo != nil {
		var err error
		parent, err = pinOrdinaryDirectory(snapshot.parentPath)
		if err != nil || !os.SameFile(snapshot.parentInfo, parent.info) {
			if parent != nil {
				parent.close()
			}
			return fmt.Errorf("legacy namespace parent changed before final validation: %s", snapshot.parentPath)
		}
		defer parent.close()
		path = parent.child(filepath.Base(snapshot.path))
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("legacy source remains after migration: %s", snapshot.path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect legacy source after migration %s: %w", snapshot.path, err)
	}
	if parent != nil {
		return parent.verifyVisible()
	}
	return nil
}

func verifyLegacyLinkAbsentAfterMigration(snapshot legacyLinkSnapshot) error {
	if snapshot.configRootInfo == nil {
		if _, err := os.Lstat(snapshot.path); err == nil {
			return fmt.Errorf("legacy link remains after migration: %s", snapshot.path)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect legacy link after migration %s: %w", snapshot.path, err)
		}
		return nil
	}
	configRootPath := filepath.Dir(snapshot.path)
	for range strings.Split(filepath.Clean(snapshot.sourceParentRelative), string(filepath.Separator)) {
		configRootPath = filepath.Dir(configRootPath)
	}
	configRoot, err := pinOrdinaryDirectory(configRootPath)
	if err != nil || !os.SameFile(snapshot.configRootInfo, configRoot.info) {
		if configRoot != nil {
			configRoot.close()
		}
		return fmt.Errorf("legacy link config root changed before final validation: %s", snapshot.path)
	}
	defer configRoot.close()
	parent, err := pinDirectoryBeneath(configRoot, snapshot.sourceParentRelative)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return configRoot.verifyVisible()
		}
		return fmt.Errorf("pin legacy link parent for final validation %s: %w", snapshot.path, err)
	}
	defer parent.close()
	if snapshot.sourceParentInfo != nil && !os.SameFile(snapshot.sourceParentInfo, parent.info) {
		return fmt.Errorf("legacy link parent changed before final validation: %s", snapshot.path)
	}
	if _, err := os.Lstat(parent.child(filepath.Base(snapshot.path))); err == nil {
		return fmt.Errorf("legacy link remains after migration: %s", snapshot.path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect legacy link after migration %s: %w", snapshot.path, err)
	}
	if err := parent.verifyVisible(); err != nil {
		return err
	}
	return configRoot.verifyVisible()
}

func verifyMigratedLegacyPublicNames(preflight legacyUserPathsPreflight, includeCache bool) error {
	paths := make([]legacyPathSnapshot, 0, len(preflight.namespaceMoves)+len(preflight.hyprPaths)+1)
	for _, move := range preflight.namespaceMoves {
		paths = append(paths, move.sourceSnapshot)
	}
	paths = append(paths, preflight.hyprPaths...)
	if includeCache {
		paths = append(paths, preflight.cacheSource)
	}
	for _, snapshot := range paths {
		if err := verifyLegacyPathAbsentAfterMigration(snapshot); err != nil {
			return err
		}
	}
	for _, snapshot := range preflight.links {
		if err := verifyLegacyLinkAbsentAfterMigration(snapshot); err != nil {
			return err
		}
	}
	return nil
}

func verifyMigratedCacheTarget(preflight legacyUserPathsPreflight) error {
	source := preflight.cacheSource
	if !source.exists {
		return nil
	}
	expected := source.info
	if preflight.cacheTarget.exists {
		expected = preflight.cacheTarget.info
	}
	if source.parentInfo == nil {
		return fmt.Errorf("cache migration parent identity is unavailable for final validation: %s", source.parentPath)
	}
	parent, err := pinOrdinaryDirectory(source.parentPath)
	if err != nil || !os.SameFile(source.parentInfo, parent.info) {
		if parent != nil {
			parent.close()
		}
		return fmt.Errorf("cache migration parent changed before final validation: %s", source.parentPath)
	}
	defer parent.close()
	if _, err := os.Lstat(parent.child(filepath.Base(source.path))); err == nil {
		return fmt.Errorf("legacy cache source remains after migration: %s", source.path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect legacy cache source after migration %s: %w", source.path, err)
	}
	target := preflight.cacheTarget.path
	info, err := os.Lstat(parent.child(filepath.Base(target)))
	if err != nil || !os.SameFile(expected, info) {
		return fmt.Errorf("canonical cache target changed before final validation: %s", target)
	}
	return parent.verifyVisible()
}

func rollbackLegacyPathMove(move *legacyPathMove) error {
	if move == nil {
		return nil
	}
	if move.parent != nil {
		return rollbackLegacyPathMovePinned(move)
	}
	targetInfo, err := os.Lstat(move.target)
	if err != nil {
		return fmt.Errorf("inspect rollback recovery %s: %w", move.recoveryPath(), err)
	}
	if !os.SameFile(move.info, targetInfo) {
		return fmt.Errorf("rollback recovery identity changed at %s; exact recovery retained at %s", move.target, move.recoveryPath())
	}
	if _, err := os.Lstat(move.source); err == nil {
		return fmt.Errorf("rollback source is occupied at %s; exact recovery retained at %s", move.source, move.recoveryPath())
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect rollback source %s: %w", move.source, err)
	}
	if err := unix.Renameat2(unix.AT_FDCWD, move.target, unix.AT_FDCWD, move.source, unix.RENAME_NOREPLACE); err != nil {
		return fmt.Errorf("restore %s from %s without replacement: %w; exact recovery retained at %s", move.source, move.target, err, move.recoveryPath())
	}
	if err := verifyLegacyPathMovePostcondition(move.target, move.source, move.info); err != nil {
		movedInfo, inspectErr := os.Lstat(move.source)
		if inspectErr != nil {
			return fmt.Errorf("verify rollback %s from %s: %w; inspect raced recovery: %w", move.source, move.target, err, inspectErr)
		}
		if restoreErr := unix.Renameat2(unix.AT_FDCWD, move.source, unix.AT_FDCWD, move.target, unix.RENAME_NOREPLACE); restoreErr != nil {
			return fmt.Errorf("verify rollback %s from %s: %w; recovery retained at %s: %w", move.source, move.target, err, move.source, restoreErr)
		}
		if restoreErr := verifyLegacyPathMovePostcondition(move.source, move.target, movedInfo); restoreErr != nil {
			return fmt.Errorf("verify rollback %s from %s: %w; raced recovery restore is incomplete: %w", move.source, move.target, err, restoreErr)
		}
		return fmt.Errorf("verify rollback %s from %s: %w; raced node restored to recovery path %s", move.source, move.target, err, move.target)
	}
	return nil
}

func rollbackLegacyPathMovePinned(move *legacyPathMove) error {
	target := move.parent.child(move.targetName)
	source := move.parent.child(move.sourceName)
	targetInfo, err := os.Lstat(target)
	if err != nil {
		return fmt.Errorf("inspect pinned rollback recovery %s: %w; exact recovery retained at %s", move.target, err, move.recoveryPath())
	}
	if !os.SameFile(move.info, targetInfo) {
		return fmt.Errorf("pinned rollback recovery identity changed at %s; exact recovery retained at %s", move.target, move.recoveryPath())
	}
	if _, err := os.Lstat(source); err == nil {
		return fmt.Errorf("pinned rollback source is occupied at %s; exact recovery retained at %s", move.source, move.recoveryPath())
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect pinned rollback source %s: %w", move.source, err)
	}
	if err := unix.Renameat2(
		move.parent.descriptor, move.targetName,
		move.parent.descriptor, move.sourceName,
		unix.RENAME_NOREPLACE,
	); err != nil {
		return fmt.Errorf("restore %s from %s through pinned parent without replacement: %w; exact recovery retained at %s", move.source, move.target, err, move.recoveryPath())
	}
	if err := verifyLegacyPathMovePostcondition(target, source, move.info); err != nil {
		return fmt.Errorf("verify pinned rollback %s from %s: %w", move.source, move.target, err)
	}
	return nil
}

func moveLegacyPathWithSnapshot(
	ctx context.Context,
	runner run.CommandRunner,
	oldPath, newPath string,
	requireDirectory bool,
	expected legacyPathSnapshot,
) (*legacyPathMove, error) {
	return moveLegacyPathWithSnapshotHook(ctx, runner, oldPath, newPath, requireDirectory, expected, nil)
}

type legacyPathMoveCommitHook func(parent *migrationPinnedDirectory) error

type legacyPathMoveAfterRenameHook func(move *legacyPathMove) error

func retainLegacyPathRecoveryAt(parent *migrationPinnedDirectory, name, display string, expected os.FileInfo) (*os.File, error) {
	fd, err := unix.Openat2(parent.descriptor, name, &unix.OpenHow{
		Flags:   unix.O_PATH | unix.O_CLOEXEC | unix.O_NOFOLLOW,
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_MAGICLINKS,
	})
	if err != nil {
		return nil, fmt.Errorf("retain migration source %s: %w", display, err)
	}
	file, fileErr := newFileFromUnixFD(fd, display)
	if fileErr != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("wrap retained migration source %s: %w", display, fileErr)
	}
	info, err := file.Stat()
	if err != nil || !os.SameFile(expected, info) {
		_ = file.Close()
		return nil, fmt.Errorf("migration source changed while retaining recovery: %s", display)
	}
	return file, nil
}

func retainLegacyPathRecovery(path string, expected os.FileInfo) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_PATH|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("retain migration source %s: %w", path, err)
	}
	file, fileErr := newFileFromUnixFD(fd, path)
	if fileErr != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("wrap retained migration source %s: %w", path, fileErr)
	}
	info, err := file.Stat()
	if err != nil || !os.SameFile(expected, info) {
		_ = file.Close()
		return nil, fmt.Errorf("migration source changed while retaining recovery: %s", path)
	}
	return file, nil
}

func moveLegacyPathWithSnapshotHook(
	ctx context.Context,
	runner run.CommandRunner,
	oldPath, newPath string,
	requireDirectory bool,
	expected legacyPathSnapshot,
	hook legacyPathMoveCommitHook,
) (*legacyPathMove, error) {
	return moveLegacyPathWithSnapshotPreparedHook(ctx, runner, oldPath, newPath, requireDirectory, expected, false, hook)
}

func moveLegacyPathWithSnapshotPrepared(
	ctx context.Context,
	runner run.CommandRunner,
	oldPath, newPath string,
	requireDirectory bool,
	expected legacyPathSnapshot,
	parentPrepared bool,
) (*legacyPathMove, error) {
	return moveLegacyPathWithSnapshotPreparedHook(ctx, runner, oldPath, newPath, requireDirectory, expected, parentPrepared, nil)
}

func moveLegacyPathWithSnapshotPreparedHook(
	ctx context.Context,
	runner run.CommandRunner,
	oldPath, newPath string,
	requireDirectory bool,
	expected legacyPathSnapshot,
	parentPrepared bool,
	hook legacyPathMoveCommitHook,
) (*legacyPathMove, error) {
	if expected.path != oldPath {
		return nil, fmt.Errorf("wahrwelt migration preflight path mismatch: expected %s, got %s", expected.path, oldPath)
	}
	if !expected.exists {
		if err := verifyLegacyPathSnapshot(expected); err != nil {
			return nil, fmt.Errorf("wahrwelt migration conflict: %w", err)
		}
		return nil, nil
	}
	if !parentPrepared && !runner.IsDryRun() {
		return moveLegacyPathWithPinnedParent(oldPath, newPath, requireDirectory, expected, hook)
	}
	identity, err := validatePreparedLegacyMove(oldPath, newPath, requireDirectory, expected)
	if err != nil {
		return nil, err
	}
	return executeUnpinnedLegacyMove(ctx, runner, oldPath, newPath, identity)
}

func validatePreparedLegacyMove(
	oldPath, newPath string,
	requireDirectory bool,
	expected legacyPathSnapshot,
) (os.FileInfo, error) {
	sourceInfo, err := os.Lstat(oldPath)
	if err != nil {
		return nil, fmt.Errorf("wahrwelt migration conflict: legacy source changed after preflight: %s: %w", oldPath, err)
	}
	if requireDirectory && (sourceInfo.Mode()&os.ModeSymlink != 0 || !sourceInfo.IsDir()) {
		return nil, fmt.Errorf("wahrwelt migration conflict: source namespace must be an ordinary directory: %s", oldPath)
	}
	if !os.SameFile(expected.info, sourceInfo) {
		return nil, fmt.Errorf("wahrwelt migration conflict: source changed after preflight: %s", oldPath)
	}
	if _, err := os.Lstat(newPath); err == nil {
		return nil, fmt.Errorf("wahrwelt migration conflict: both %s and %s exist", oldPath, newPath)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return expected.info, nil
}

func executeUnpinnedLegacyMove(
	ctx context.Context,
	runner run.CommandRunner,
	oldPath, newPath string,
	identity os.FileInfo,
) (*legacyPathMove, error) {
	if runner.IsDryRun() {
		return nil, runner.Command(ctx, "mv", "-T", "--no-copy", "--update=none-fail", "--", oldPath, newPath)
	}
	recovery, err := retainLegacyPathRecovery(oldPath, identity)
	if err != nil {
		return nil, err
	}
	if err := unix.Renameat2(unix.AT_FDCWD, oldPath, unix.AT_FDCWD, newPath, unix.RENAME_NOREPLACE); err != nil {
		_ = recovery.Close()
		if errors.Is(err, unix.EEXIST) || errors.Is(err, unix.ENOTEMPTY) {
			return nil, fmt.Errorf("wahrwelt migration conflict: canonical path appeared during migration: %s", newPath)
		}
		return nil, fmt.Errorf("atomically migrate %s to %s without replacement: %w", oldPath, newPath, err)
	}
	targetInfo, inspectErr := os.Lstat(newPath)
	move := &legacyPathMove{source: oldPath, target: newPath, info: identity, recovery: recovery}
	if inspectErr != nil {
		return move, fmt.Errorf("inspect migrated target %s: %w", newPath, inspectErr)
	}
	if !os.SameFile(identity, targetInfo) {
		return move, fmt.Errorf("migrated target changed immediately after rename: %s; recovery retained at %s", newPath, move.recoveryPath())
	}
	return move, verifyLegacyPathMovePostcondition(oldPath, newPath, identity)
}

func moveLegacyPathWithPinnedParent(
	oldPath, newPath string,
	requireDirectory bool,
	expected legacyPathSnapshot,
	hook legacyPathMoveCommitHook,
) (*legacyPathMove, error) {
	return moveLegacyPathWithPinnedParentHooks(oldPath, newPath, requireDirectory, expected, hook, nil)
}

func moveLegacyPathWithPinnedParentHooks(
	oldPath, newPath string,
	requireDirectory bool,
	expected legacyPathSnapshot,
	hook legacyPathMoveCommitHook,
	afterRename legacyPathMoveAfterRenameHook,
) (*legacyPathMove, error) {
	parent, sourceName, targetName, err := pinLegacyMoveParent(oldPath, newPath, expected)
	if err != nil {
		return nil, err
	}
	keepParent := false
	defer func() {
		if !keepParent {
			parent.close()
		}
	}()
	pinnedSource := parent.child(sourceName)
	pinnedTarget := parent.child(targetName)
	recovery, err := preparePinnedLegacyMoveSource(parent, sourceName, pinnedSource, pinnedTarget, oldPath, newPath, requireDirectory, expected)
	if err != nil {
		return nil, err
	}
	keepRecovery := false
	defer func() {
		if !keepRecovery {
			_ = recovery.Close()
		}
	}()
	if err := renamePinnedLegacyMove(parent, sourceName, targetName, oldPath, newPath, hook); err != nil {
		return nil, err
	}
	move := &legacyPathMove{
		source:     oldPath,
		target:     newPath,
		info:       expected.info,
		recovery:   recovery,
		parent:     parent,
		sourceName: sourceName,
		targetName: targetName,
	}
	keepRecovery = true
	keepParent = true
	return verifyPinnedLegacyMove(move, pinnedSource, pinnedTarget, afterRename)
}

func pinLegacyMoveParent(
	oldPath, newPath string,
	expected legacyPathSnapshot,
) (*migrationPinnedDirectory, string, string, error) {
	parentPath := filepath.Dir(oldPath)
	if filepath.Dir(newPath) != parentPath {
		return nil, "", "", fmt.Errorf("wahrwelt migration paths do not share one parent: %s, %s", oldPath, newPath)
	}
	if expected.parentPath != parentPath || expected.parentInfo == nil {
		return nil, "", "", fmt.Errorf("wahrwelt migration preflight parent mismatch for %s", oldPath)
	}
	parent, err := pinOrdinaryDirectory(parentPath)
	if err != nil {
		return nil, "", "", fmt.Errorf("pin Wahrwelt namespace parent %s: %w", parentPath, err)
	}
	if !os.SameFile(expected.parentInfo, parent.info) {
		parent.close()
		return nil, "", "", fmt.Errorf("wahrwelt migration conflict: namespace parent changed after preflight: %s", parentPath)
	}
	return parent, filepath.Base(oldPath), filepath.Base(newPath), nil
}

func preparePinnedLegacyMoveSource(
	parent *migrationPinnedDirectory,
	sourceName, pinnedSource, pinnedTarget, oldPath, newPath string,
	requireDirectory bool,
	expected legacyPathSnapshot,
) (*os.File, error) {
	sourceInfo, err := os.Lstat(pinnedSource)
	if err != nil {
		return nil, fmt.Errorf("wahrwelt migration conflict: legacy source changed after preflight: %s: %w", oldPath, err)
	}
	if requireDirectory && (sourceInfo.Mode()&os.ModeSymlink != 0 || !sourceInfo.IsDir()) {
		return nil, fmt.Errorf("wahrwelt migration conflict: source namespace must be an ordinary directory: %s", oldPath)
	}
	if !os.SameFile(expected.info, sourceInfo) {
		return nil, fmt.Errorf("wahrwelt migration conflict: source changed after preflight: %s", oldPath)
	}
	recovery, err := retainLegacyPathRecoveryAt(parent, sourceName, oldPath, expected.info)
	if err != nil {
		return nil, err
	}
	if _, err := os.Lstat(pinnedTarget); err == nil {
		_ = recovery.Close()
		return nil, fmt.Errorf("wahrwelt migration conflict: both %s and %s exist", oldPath, newPath)
	} else if !os.IsNotExist(err) {
		_ = recovery.Close()
		return nil, err
	}
	return recovery, nil
}

func renamePinnedLegacyMove(
	parent *migrationPinnedDirectory,
	sourceName, targetName, oldPath, newPath string,
	hook legacyPathMoveCommitHook,
) error {
	if err := parent.verifyVisible(); err != nil {
		return err
	}
	if hook != nil {
		if err := hook(parent); err != nil {
			return err
		}
	}
	if err := unix.Renameat2(
		parent.descriptor, sourceName,
		parent.descriptor, targetName,
		unix.RENAME_NOREPLACE,
	); err != nil {
		if errors.Is(err, unix.EEXIST) || errors.Is(err, unix.ENOTEMPTY) {
			return fmt.Errorf("wahrwelt migration conflict: canonical path appeared during migration: %s", newPath)
		}
		return fmt.Errorf("atomically migrate %s to %s through pinned parent without replacement: %w", oldPath, newPath, err)
	}
	return nil
}

func verifyPinnedLegacyMove(
	move *legacyPathMove,
	pinnedSource, pinnedTarget string,
	afterRename legacyPathMoveAfterRenameHook,
) (*legacyPathMove, error) {
	if afterRename != nil {
		if err := afterRename(move); err != nil {
			return move, err
		}
	}
	movedInfo, inspectErr := os.Lstat(pinnedTarget)
	if inspectErr != nil {
		return move, fmt.Errorf("inspect migrated target %s: %w", move.target, inspectErr)
	}
	if !os.SameFile(move.info, movedInfo) {
		return move, fmt.Errorf("wahrwelt migration target changed immediately after rename: %s; recovery retained at %s", move.target, move.recoveryPath())
	}
	if err := verifyLegacyPathMovePostcondition(pinnedSource, pinnedTarget, move.info); err != nil {
		return rollbackVerifiedPinnedLegacyMove(move, err, "exact moved owner rollback incomplete", "concurrent source replacement restored through pinned parent")
	}
	if err := move.parent.verifyVisible(); err != nil {
		return rollbackVerifiedPinnedLegacyMove(move, err, "pinned rollback incomplete", "committed namespace move rolled back through pinned parent")
	}
	return move, nil
}

func rollbackVerifiedPinnedLegacyMove(move *legacyPathMove, cause error, failureDetail, successDetail string) (*legacyPathMove, error) {
	if rollbackErr := rollbackLegacyPathMovePinned(move); rollbackErr != nil {
		return move, fmt.Errorf("%w; %s: %w; recovery retained at %s", cause, failureDetail, rollbackErr, move.recoveryPath())
	}
	move.close()
	return nil, fmt.Errorf("%w; %s", cause, successDetail)
}

func verifyLegacyPathMovePostcondition(oldPath, newPath string, sourceInfo os.FileInfo) error {
	if _, err := os.Lstat(oldPath); err == nil {
		return fmt.Errorf("wahrwelt migration postcondition failed: legacy source remains: %s", oldPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect legacy source after migration %s: %w", oldPath, err)
	}
	targetInfo, err := os.Lstat(newPath)
	if err != nil {
		return fmt.Errorf("wahrwelt migration postcondition failed: canonical target is missing: %s: %w", newPath, err)
	}
	if !os.SameFile(sourceInfo, targetInfo) {
		return fmt.Errorf("wahrwelt migration postcondition failed: canonical target identity changed: %s", newPath)
	}
	return nil
}

func prepareLegacyCacheCommit(
	oldPath, newPath string,
	oldSnapshot, newSnapshot legacyPathSnapshot,
) (bool, error) {
	if oldSnapshot.path != oldPath || newSnapshot.path != newPath {
		return false, fmt.Errorf("wahrwelt cache migration preflight path mismatch")
	}
	if !oldSnapshot.exists {
		if err := verifyLegacyPathSnapshot(oldSnapshot); err != nil {
			return false, err
		}
		return false, nil
	}
	if err := verifyLegacyPathSnapshot(oldSnapshot); err != nil {
		return false, err
	}
	if !newSnapshot.exists {
		return true, nil
	}
	if err := verifyLegacyPathSnapshot(newSnapshot); err != nil {
		return false, err
	}
	return true, nil
}

type legacyLinkSnapshot struct {
	path                 string
	exists               bool
	info                 os.FileInfo
	linkTarget           string
	configRootInfo       os.FileInfo
	sourceParentRelative string
	sourceParentInfo     os.FileInfo
}

func snapshotLegacyLinks(configHome string, paths []string) ([]legacyLinkSnapshot, error) {
	snapshots := make([]legacyLinkSnapshot, 0, len(paths))
	configRoot, rootErr := pinOrdinaryDirectory(configHome)
	if rootErr != nil {
		return snapshotLegacyLinksWithoutConfigRoot(paths, rootErr)
	}
	defer configRoot.close()
	for _, path := range paths {
		snapshot, err := snapshotLegacyLinkAtConfigRoot(configRoot, configHome, path)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	if err := configRoot.verifyVisible(); err != nil {
		return nil, err
	}
	return snapshots, nil
}

func snapshotLegacyLinksWithoutConfigRoot(paths []string, rootErr error) ([]legacyLinkSnapshot, error) {
	snapshots := make([]legacyLinkSnapshot, 0, len(paths))
	for _, path := range paths {
		if _, err := os.Lstat(path); err == nil || !os.IsNotExist(err) {
			return nil, fmt.Errorf("pin config root for legacy link preflight: %w", rootErr)
		}
		snapshots = append(snapshots, legacyLinkSnapshot{path: path})
	}
	return snapshots, nil
}

func snapshotLegacyLinkAtConfigRoot(
	configRoot *migrationPinnedDirectory,
	configHome, path string,
) (legacyLinkSnapshot, error) {
	relative, err := filepath.Rel(configHome, path)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return legacyLinkSnapshot{}, fmt.Errorf("legacy link escaped pinned config root: %s", path)
	}
	snapshot := legacyLinkSnapshot{
		path:                 path,
		configRootInfo:       configRoot.info,
		sourceParentRelative: filepath.Dir(relative),
	}
	parent, err := pinDirectoryBeneath(configRoot, snapshot.sourceParentRelative)
	if err != nil {
		if _, statErr := os.Lstat(path); os.IsNotExist(statErr) {
			return snapshot, nil
		}
		return legacyLinkSnapshot{}, fmt.Errorf("pin legacy link parent for %s: %w", path, err)
	}
	defer parent.close()
	snapshot.sourceParentInfo = parent.info
	info, err := os.Lstat(parent.child(filepath.Base(relative)))
	if err != nil {
		if os.IsNotExist(err) {
			return snapshot, nil
		}
		return legacyLinkSnapshot{}, err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return legacyLinkSnapshot{}, fmt.Errorf("refusing to remove non-symlink %s", path)
	}
	target, err := os.Readlink(parent.child(filepath.Base(relative)))
	if err != nil {
		return legacyLinkSnapshot{}, err
	}
	if !isManagedLegacyLinkTarget(path, target) {
		return legacyLinkSnapshot{}, fmt.Errorf("unmanaged legacy symlink collision at %s: target %s", path, target)
	}
	snapshot.exists = true
	snapshot.info = info
	snapshot.linkTarget = target
	return snapshot, parent.verifyVisible()
}

func isManagedLegacyLinkTarget(path, target string) bool {
	var expectedRelative string
	switch {
	case strings.HasSuffix(filepath.ToSlash(path), "/hypr/lib/mysetup.lua"):
		expectedRelative = ".config/hypr/lib/mysetup.lua"
	case strings.HasSuffix(filepath.ToSlash(path), "/quickshell/mysetup-shell-selector"):
		expectedRelative = ".config/quickshell/mysetup-shell-selector"
	default:
		return false
	}

	clean := filepath.Clean(target)
	if !filepath.IsAbs(clean) {
		return false
	}
	relative, err := filepath.Rel(filepath.Clean("/nix/store"), clean)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	parts := strings.Split(filepath.ToSlash(relative), "/")
	if len(parts) < 2 || strings.Join(parts[1:], "/") != expectedRelative {
		return false
	}
	const suffix = "-home-manager-files"
	storeName := parts[0]
	if !strings.HasSuffix(storeName, suffix) {
		return false
	}
	hash := strings.TrimSuffix(storeName, suffix)
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

type legacyLinkRecovery struct {
	configRoot    *migrationPinnedDirectory
	recovery      *migrationPinnedDirectory
	sourceParents []*migrationPinnedDirectory
}

func (r *legacyLinkRecovery) path() string {
	if r == nil || r.recovery == nil {
		return ""
	}
	return r.recovery.path
}

func (r *legacyLinkRecovery) verifyVisible() error {
	if err := r.configRoot.verifyVisible(); err != nil {
		return err
	}
	for _, parent := range r.sourceParents {
		if err := parent.verifyVisible(); err != nil {
			return err
		}
	}
	return r.recovery.verifyVisible()
}

func (r *legacyLinkRecovery) close() {
	if r == nil {
		return
	}
	r.recovery.close()
	for index := len(r.sourceParents) - 1; index >= 0; index-- {
		r.sourceParents[index].close()
	}
	r.configRoot.close()
}

func quarantineLegacyLinks(
	ctx context.Context,
	runner run.CommandRunner,
	configHome string,
	snapshots []legacyLinkSnapshot,
	journal *legacyMigrationJournal,
) (*legacyLinkRecovery, error) {
	return quarantineLegacyLinksWithHook(ctx, runner, configHome, snapshots, journal, nil)
}

type legacyLinkQuarantineHook func(index int, recovery *legacyLinkRecovery, sourceParent *migrationPinnedDirectory) error

func quarantineLegacyLinksWithHook(
	ctx context.Context,
	runner run.CommandRunner,
	configHome string,
	snapshots []legacyLinkSnapshot,
	journal *legacyMigrationJournal,
	hook legacyLinkQuarantineHook,
) (*legacyLinkRecovery, error) {
	if !legacyLinksBoundToConfigRoot(snapshots) {
		return nil, verifyUnboundLegacyLinkSnapshots(snapshots)
	}
	configRoot, err := pinOrdinaryDirectory(configHome)
	if err != nil {
		return nil, err
	}
	keepConfigRoot := false
	defer func() {
		if !keepConfigRoot {
			configRoot.close()
		}
	}()
	present, err := collectPresentLegacyLinks(configRoot, configHome, snapshots)
	if err != nil {
		return nil, err
	}
	if runner.IsDryRun() {
		return nil, removeLegacyLinksDryRun(ctx, runner, present)
	}
	if len(present) == 0 {
		return nil, nil
	}
	recoveryDir, err := createPinnedRecoveryDirectory(configRoot, ".wahrwelt-migration-recovery-links-")
	if err != nil {
		return nil, err
	}
	recovery := &legacyLinkRecovery{configRoot: configRoot, recovery: recoveryDir}
	keepConfigRoot = true
	if err := quarantinePresentLegacyLinks(ctx, runner, present, journal, recovery, hook); err != nil {
		return recovery, err
	}
	if err := recovery.verifyVisible(); err != nil {
		return recovery, err
	}
	return recovery, nil
}

func legacyLinksBoundToConfigRoot(snapshots []legacyLinkSnapshot) bool {
	for _, snapshot := range snapshots {
		if snapshot.configRootInfo != nil {
			return true
		}
	}
	return false
}

func verifyUnboundLegacyLinkSnapshots(snapshots []legacyLinkSnapshot) error {
	for _, snapshot := range snapshots {
		if err := verifyLegacyLinkSnapshot(snapshot); err != nil {
			return err
		}
	}
	return nil
}

func collectPresentLegacyLinks(
	configRoot *migrationPinnedDirectory,
	configHome string,
	snapshots []legacyLinkSnapshot,
) ([]legacyLinkSnapshot, error) {
	var present []legacyLinkSnapshot
	for _, snapshot := range snapshots {
		if snapshot.configRootInfo == nil || !os.SameFile(snapshot.configRootInfo, configRoot.info) {
			return nil, fmt.Errorf("legacy link config root changed after preflight: %s", configHome)
		}
		if err := verifyLegacyLinkSnapshotAtRoot(snapshot, configRoot); err != nil {
			return nil, err
		}
		if snapshot.exists {
			present = append(present, snapshot)
		}
	}
	return present, nil
}

func removeLegacyLinksDryRun(ctx context.Context, runner run.CommandRunner, snapshots []legacyLinkSnapshot) error {
	for _, snapshot := range snapshots {
		if err := runner.Command(ctx, "rm", "-f", "--", snapshot.path); err != nil {
			return err
		}
	}
	return nil
}

func quarantinePresentLegacyLinks(
	ctx context.Context,
	runner run.CommandRunner,
	present []legacyLinkSnapshot,
	journal *legacyMigrationJournal,
	recovery *legacyLinkRecovery,
	hook legacyLinkQuarantineHook,
) error {
	for index, snapshot := range present {
		sourceParent, err := pinDirectoryBeneath(recovery.configRoot, snapshot.sourceParentRelative)
		if err != nil {
			return fmt.Errorf("pin legacy link parent for %s: %w", snapshot.path, err)
		}
		if snapshot.sourceParentInfo == nil || !os.SameFile(snapshot.sourceParentInfo, sourceParent.info) {
			sourceParent.close()
			return fmt.Errorf("legacy link parent changed after preflight: %s", snapshot.path)
		}
		recovery.sourceParents = append(recovery.sourceParents, sourceParent)
		if hook != nil {
			if err := hook(index, recovery, sourceParent); err != nil {
				return err
			}
		}
		pinnedSource := sourceParent.child(filepath.Base(snapshot.path))
		target := recovery.recovery.child(fmt.Sprintf("link-%d", index))
		move, moveErr := moveLegacyPathWithSnapshotPrepared(ctx, runner, pinnedSource, target, false, legacyPathSnapshot{
			path:   pinnedSource,
			exists: true,
			info:   snapshot.info,
		}, true)
		journal.add(move)
		if moveErr != nil {
			return moveErr
		}
		if movedTarget, err := os.Readlink(target); err != nil || movedTarget != snapshot.linkTarget {
			if err != nil {
				return fmt.Errorf("legacy link changed after preflight at %s: %w", snapshot.path, err)
			}
			return fmt.Errorf("legacy link target changed after preflight: %s", snapshot.path)
		}
	}
	return nil
}

func verifyLegacyLinkSnapshotAtRoot(snapshot legacyLinkSnapshot, configRoot *migrationPinnedDirectory) error {
	parent, err := pinDirectoryBeneath(configRoot, snapshot.sourceParentRelative)
	if err != nil {
		if !snapshot.exists && snapshot.sourceParentInfo == nil && errors.Is(err, unix.ENOENT) {
			return configRoot.verifyVisible()
		}
		return fmt.Errorf("legacy link parent changed after preflight: %s: %w", snapshot.path, err)
	}
	defer parent.close()
	if snapshot.sourceParentInfo != nil && !os.SameFile(snapshot.sourceParentInfo, parent.info) {
		return fmt.Errorf("legacy link parent changed after preflight: %s", snapshot.path)
	}
	pinnedPath := parent.child(filepath.Base(snapshot.path))
	info, err := os.Lstat(pinnedPath)
	if !snapshot.exists {
		return verifyAbsentLegacyLinkAtRoot(snapshot, configRoot, parent, err)
	}
	return verifyPresentLegacyLinkAtRoot(snapshot, configRoot, parent, pinnedPath, info, err)
}

func verifyAbsentLegacyLinkAtRoot(
	snapshot legacyLinkSnapshot,
	configRoot, parent *migrationPinnedDirectory,
	statErr error,
) error {
	if statErr == nil {
		return fmt.Errorf("legacy link appeared after preflight: %s", snapshot.path)
	}
	if !os.IsNotExist(statErr) {
		return statErr
	}
	if err := parent.verifyVisible(); err != nil {
		return err
	}
	return configRoot.verifyVisible()
}

func verifyPresentLegacyLinkAtRoot(
	snapshot legacyLinkSnapshot,
	configRoot, parent *migrationPinnedDirectory,
	pinnedPath string,
	info os.FileInfo,
	statErr error,
) error {
	if statErr != nil || info.Mode()&os.ModeSymlink == 0 || !os.SameFile(snapshot.info, info) {
		return fmt.Errorf("legacy link changed after preflight: %s", snapshot.path)
	}
	target, err := os.Readlink(pinnedPath)
	if err != nil || target != snapshot.linkTarget {
		return fmt.Errorf("legacy link target changed after preflight: %s", snapshot.path)
	}
	if err := parent.verifyVisible(); err != nil {
		return err
	}
	return configRoot.verifyVisible()
}

func verifyLegacyLinkSnapshot(snapshot legacyLinkSnapshot) error {
	info, err := os.Lstat(snapshot.path)
	if !snapshot.exists {
		if err == nil {
			return fmt.Errorf("legacy link appeared after preflight: %s", snapshot.path)
		}
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err != nil {
		return fmt.Errorf("legacy link changed after preflight: %s: %w", snapshot.path, err)
	}
	if info.Mode()&os.ModeSymlink == 0 || !os.SameFile(snapshot.info, info) {
		return fmt.Errorf("legacy link changed after preflight: %s", snapshot.path)
	}
	target, err := os.Readlink(snapshot.path)
	if err != nil {
		return err
	}
	if target != snapshot.linkTarget {
		return fmt.Errorf("legacy link target changed after preflight: %s", snapshot.path)
	}
	return nil
}

type legacyCacheCommitHook func(recovery *migrationPinnedDirectory, pinnedNew string) error

func mergeLegacyCacheWithSnapshotsHook(
	ctx context.Context,
	runner run.CommandRunner,
	oldPath, newPath string,
	oldSnapshot, newSnapshot legacyPathSnapshot,
	hook legacyCacheCommitHook,
) error {
	handled, err := preflightLegacyCacheMerge(ctx, runner, oldPath, newPath, oldSnapshot, newSnapshot)
	if err != nil || handled {
		return err
	}
	if runner.IsDryRun() {
		return previewLegacyCacheQuarantine(ctx, runner, oldPath, newPath)
	}
	cacheRoot, pinnedOld, pinnedNew, pinnedOldSnapshot, pinnedNewSnapshot, err := pinLegacyCacheMerge(
		oldPath,
		newPath,
		oldSnapshot,
		newSnapshot,
	)
	if err != nil {
		return err
	}
	defer cacheRoot.close()
	if !newSnapshot.exists {
		return commitSingleLegacyCacheMove(ctx, runner, cacheRoot, pinnedOld, pinnedNew, pinnedOldSnapshot)
	}
	return quarantineLegacyCacheSource(
		ctx,
		runner,
		cacheRoot,
		pinnedOld,
		pinnedNew,
		oldPath,
		pinnedOldSnapshot,
		pinnedNewSnapshot,
		hook,
	)
}

func preflightLegacyCacheMerge(
	ctx context.Context,
	runner run.CommandRunner,
	oldPath, newPath string,
	oldSnapshot, newSnapshot legacyPathSnapshot,
) (bool, error) {
	if oldSnapshot.path != oldPath || newSnapshot.path != newPath {
		return false, fmt.Errorf("wahrwelt cache migration preflight path mismatch")
	}
	if !oldSnapshot.exists {
		return true, verifyLegacyPathSnapshot(oldSnapshot)
	}
	if err := verifyLegacyPathSnapshot(oldSnapshot); err != nil {
		return false, err
	}
	if !newSnapshot.exists && runner.IsDryRun() {
		_, err := moveLegacyPathWithSnapshot(ctx, runner, oldPath, newPath, true, oldSnapshot)
		return true, err
	}
	if newSnapshot.exists {
		if err := verifyLegacyPathSnapshot(newSnapshot); err != nil {
			return false, err
		}
	}
	return false, nil
}

func previewLegacyCacheQuarantine(ctx context.Context, runner run.CommandRunner, oldPath, newPath string) error {
	recoveryPreview := filepath.Join(filepath.Dir(newPath), ".wahrwelt-migration-recovery-cache-<random>")
	if err := runner.Command(ctx, "mkdir", "-m", "0700", "--", recoveryPreview); err != nil {
		return err
	}
	return runner.Command(
		ctx,
		"mv", "-T", "--no-copy", "--update=none-fail", "--",
		oldPath, filepath.Join(recoveryPreview, "legacy-original"),
	)
}

func pinLegacyCacheMerge(
	oldPath, newPath string,
	oldSnapshot, newSnapshot legacyPathSnapshot,
) (*migrationPinnedDirectory, string, string, legacyPathSnapshot, legacyPathSnapshot, error) {
	if filepath.Dir(oldPath) != filepath.Dir(newPath) {
		return nil, "", "", legacyPathSnapshot{}, legacyPathSnapshot{}, fmt.Errorf("cache migration paths do not share one pinned parent: %s, %s", oldPath, newPath)
	}
	cacheRoot, err := pinOrdinaryDirectory(filepath.Dir(newPath))
	if err != nil {
		return nil, "", "", legacyPathSnapshot{}, legacyPathSnapshot{}, err
	}
	fail := func(err error) (*migrationPinnedDirectory, string, string, legacyPathSnapshot, legacyPathSnapshot, error) {
		cacheRoot.close()
		return nil, "", "", legacyPathSnapshot{}, legacyPathSnapshot{}, err
	}
	pinnedOld := cacheRoot.child(filepath.Base(oldPath))
	pinnedNew := cacheRoot.child(filepath.Base(newPath))
	pinnedOldSnapshot := snapshotAtPinnedPath(oldSnapshot, pinnedOld)
	pinnedNewSnapshot := snapshotAtPinnedPath(newSnapshot, pinnedNew)
	if err := verifyLegacyPathSnapshot(pinnedOldSnapshot); err != nil {
		return fail(err)
	}
	if err := verifyLegacyPathSnapshot(pinnedNewSnapshot); err != nil {
		return fail(err)
	}
	return cacheRoot, pinnedOld, pinnedNew, pinnedOldSnapshot, pinnedNewSnapshot, nil
}

func commitSingleLegacyCacheMove(
	ctx context.Context,
	runner run.CommandRunner,
	cacheRoot *migrationPinnedDirectory,
	pinnedOld, pinnedNew string,
	pinnedOldSnapshot legacyPathSnapshot,
) error {
	move, err := moveLegacyPathWithSnapshotPrepared(
		ctx, runner, pinnedOld, pinnedNew, true, pinnedOldSnapshot, true,
	)
	if err != nil {
		return rollbackSingleLegacyCacheMove(move, err, pinnedNew)
	}
	if err := cacheRoot.verifyVisible(); err != nil {
		return rollbackSingleLegacyCacheMove(move, err, pinnedNew)
	}
	return nil
}

func rollbackSingleLegacyCacheMove(move *legacyPathMove, cause error, pinnedNew string) error {
	if move == nil {
		return cause
	}
	if rollbackErr := rollbackLegacyPathMove(move); rollbackErr != nil {
		return fmt.Errorf("%w; cache rollback incomplete: %w; recovery retained at %s", cause, rollbackErr, pinnedNew)
	}
	return fmt.Errorf("%w; rolled back single cache move", cause)
}

func quarantineLegacyCacheSource(
	ctx context.Context,
	runner run.CommandRunner,
	cacheRoot *migrationPinnedDirectory,
	pinnedOld, pinnedNew, oldPath string,
	pinnedOldSnapshot, pinnedNewSnapshot legacyPathSnapshot,
	hook legacyCacheCommitHook,
) error {
	recovery, err := createPinnedRecoveryDirectory(cacheRoot, ".wahrwelt-migration-recovery-cache-")
	if err != nil {
		return err
	}
	defer recovery.close()
	retainRecovery := func(cause error) error {
		return fmt.Errorf("%w; cache recovery retained at %s", cause, recovery.path)
	}
	if hook != nil {
		if err := hook(recovery, pinnedNew); err != nil {
			return retainRecovery(err)
		}
	}
	legacyRecovery := recovery.child("legacy-original")
	legacyMove, err := moveLegacyPathWithSnapshotPrepared(
		ctx, runner, pinnedOld, legacyRecovery, true, pinnedOldSnapshot, true,
	)
	if err != nil {
		if legacyMove == nil {
			return retainRecovery(err)
		}
		if rollbackErr := rollbackLegacyPathMove(legacyMove); rollbackErr != nil {
			return fmt.Errorf("%w; cache rollback incomplete: %w; recovery retained at %s", err, rollbackErr, recovery.path)
		}
		return fmt.Errorf("%w; rolled back legacy cache quarantine; recovery retained at %s", err, recovery.path)
	}
	if err := verifyLegacyCacheQuarantine(cacheRoot, recovery, pinnedOld, pinnedNew, oldPath, pinnedNewSnapshot); err != nil {
		return rollbackLegacyCacheQuarantine(legacyMove, recovery.path, err)
	}
	fmt.Printf("Wahrwelt migration preserved canonical cache unchanged; legacy cache recovery retained at %s\n", recovery.path)
	return nil
}

func verifyLegacyCacheQuarantine(
	cacheRoot, recovery *migrationPinnedDirectory,
	pinnedOld, pinnedNew, oldPath string,
	pinnedNewSnapshot legacyPathSnapshot,
) error {
	if _, err := os.Lstat(pinnedOld); err == nil {
		return fmt.Errorf("legacy cache source appeared during commit: %s", oldPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect legacy cache source after commit: %w", err)
	}
	if err := requireSamePathIdentity(pinnedNew, pinnedNewSnapshot.info, "preserved canonical cache"); err != nil {
		return err
	}
	if err := cacheRoot.verifyVisible(); err != nil {
		return err
	}
	return recovery.verifyVisible()
}

func rollbackLegacyCacheQuarantine(move *legacyPathMove, recoveryPath string, cause error) error {
	if rollbackErr := rollbackLegacyPathMove(move); rollbackErr != nil {
		return fmt.Errorf("%w; rollback incomplete: %w; recovery retained at %s", cause, rollbackErr, recoveryPath)
	}
	return fmt.Errorf("%w; legacy cache quarantine rolled back", cause)
}

func requireSamePathIdentity(path string, expected os.FileInfo, label string) error {
	current, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect %s at %s: %w", label, path, err)
	}
	if !os.SameFile(expected, current) {
		return fmt.Errorf("%s identity changed at %s", label, path)
	}
	return nil
}
