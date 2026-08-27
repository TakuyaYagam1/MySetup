//go:build linux

// Package fsowner provides small, fd-pinned filesystem ownership operations.
// It never follows the managed leaf and requires callers to retain an exact
// identity token between inspection and mutation.
package fsowner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/unix"
)

// Kind is the no-follow type of a directory entry.
type Kind string

const (
	KindAbsent    Kind = "absent"
	KindRegular   Kind = "regular"
	KindDirectory Kind = "directory"
	KindSymlink   Kind = "symlink"
	KindOther     Kind = "other"
)

// Identity is an exact no-follow inode identity token.
type Identity struct {
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
	Mode   uint32 `json:"mode"`
	Links  uint64 `json:"links"`
	UID    uint32 `json:"uid"`
	Kind   Kind   `json:"kind"`
}

func (i Identity) String() string {
	return fmt.Sprintf("%d:%d:%d:%d:%d:%s", i.Device, i.Inode, i.Mode, i.Links, i.UID, i.Kind)
}

// Entry is one inspected child of a pinned directory.
type Entry struct {
	Name     string
	Identity Identity
}

// RemoveOptions narrows which exact inode may be removed. AdditionalUIDs is
// only for an independently authenticated recursive tree whose copied payload
// intentionally preserves more than one owner; it does not relax RemoveRegular.
type RemoveOptions struct {
	UID            uint32
	AdditionalUIDs []uint32
	Recursive      bool
	RequireEmpty   bool
	SameDevice     bool
}

// AllowsUID reports whether uid belongs to the authenticated recursive tree.
func (o RemoveOptions) AllowsUID(uid uint32) bool {
	if uid == o.UID {
		return true
	}
	for _, allowed := range o.AdditionalUIDs {
		if uid == allowed {
			return true
		}
	}
	return false
}

// Directory pins a canonical directory for a sequence of inspections and
// mutations. Directory is not safe for concurrent use.
type Directory struct {
	fd       int
	path     string
	identity Identity
	closed   bool
}

// OpenDirectory opens path without following its final component.
func OpenDirectory(path string) (*Directory, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("absolute directory path: %w", err)
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return nil, fmt.Errorf("inspect directory %s: %w", absolute, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("ownership collision: %s is not an ordinary directory", absolute)
	}
	fd, err := unix.Open(absolute, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("open directory %s: %w", absolute, err)
	}
	identity, err := identityFromFD(fd)
	if err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("identify directory %s: %w", absolute, err)
	}
	if identity.Kind != KindDirectory {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("ownership collision: %s changed while opening", absolute)
	}
	visible, err := identityAt(unix.AT_FDCWD, absolute)
	if err != nil || !sameObject(visible, identity) {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("ownership collision: %s changed while pinning", absolute)
	}
	return &Directory{fd: fd, path: absolute, identity: identity}, nil
}

// Close releases the pinned directory descriptor.
func (d *Directory) Close() error {
	if d == nil || d.closed {
		return nil
	}
	d.closed = true
	return unix.Close(d.fd)
}

// Identity returns the pinned directory identity.
func (d *Directory) Identity() Identity {
	if d == nil {
		return Identity{}
	}
	return d.identity
}

// Sync durably records prior mutations made through the pinned directory.
func (d *Directory) Sync() error {
	if err := d.verifyCanonical(); err != nil {
		return err
	}
	if err := unix.Fsync(d.fd); err != nil {
		return fmt.Errorf("sync pinned directory %s: %w", d.path, err)
	}
	return nil
}

// List returns sorted no-follow children accepted by match.
func (d *Directory) List(match func(string) bool) ([]Entry, error) {
	if err := d.verifyCanonical(); err != nil {
		return nil, err
	}
	names, err := directoryNames(d.fd)
	if err != nil {
		return nil, fmt.Errorf("list pinned directory %s: %w", d.path, err)
	}
	entries := make([]Entry, 0, len(names))
	for _, name := range names {
		if match != nil && !match(name) {
			continue
		}
		entry, inspectErr := d.Inspect(name)
		if inspectErr != nil {
			return nil, inspectErr
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

// Inspect returns the current exact no-follow identity of name.
func (d *Directory) Inspect(name string) (Entry, error) {
	if err := d.verifyCanonical(); err != nil {
		return Entry{}, err
	}
	if err := validateLeafName(name); err != nil {
		return Entry{}, err
	}
	identity, err := identityAt(d.fd, name)
	if err != nil {
		return Entry{}, fmt.Errorf("inspect %s/%s: %w", d.path, name, err)
	}
	return Entry{Name: name, Identity: identity}, nil
}

// RemoveRegular unlinks exactly the inspected private regular file.
func (d *Directory) RemoveRegular(name string, expected Identity, options RemoveOptions) error {
	if err := d.verifyCanonical(); err != nil {
		return err
	}
	if err := validateLeafName(name); err != nil {
		return err
	}
	fd, current, err := openPathEntry(d.fd, name)
	if err != nil {
		return fmt.Errorf("open regular removal candidate %s/%s: %w", d.path, name, err)
	}
	defer func() { _ = unix.Close(fd) }()
	if current != expected {
		return fmt.Errorf("ownership collision: %s/%s identity is %s, expected %s", d.path, name, current, expected)
	}
	if current.Kind != KindRegular || current.Links != 1 || current.UID != options.UID {
		return fmt.Errorf("ownership collision: %s/%s is not the expected private regular file", d.path, name)
	}
	if err := d.verifyVisible(name, expected); err != nil {
		return err
	}
	if err := unix.Unlinkat(d.fd, name, 0); err != nil {
		return fmt.Errorf("unlink verified regular file %s/%s: %w", d.path, name, err)
	}
	var after unix.Stat_t
	if err := unix.Fstat(fd, &after); err != nil || after.Nlink != 0 {
		return fmt.Errorf("verified regular unlink could not be proven for %s/%s", d.path, name)
	}
	return nil
}

// RemoveDirectory removes exactly the inspected directory. Recursive removal
// never follows symlinks and verifies ownership and device for every child.
//
//nolint:gocyclo // The descriptor-pinned removal checks stay linear so each ownership refusal remains auditable.
func (d *Directory) RemoveDirectory(name string, expected Identity, options RemoveOptions) error {
	if err := d.verifyCanonical(); err != nil {
		return err
	}
	if err := validateLeafName(name); err != nil {
		return err
	}
	fd, err := unix.Openat(d.fd, name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("ownership collision: open directory removal candidate %s/%s: %w", d.path, name, err)
	}
	plan, err := buildRemovalPlan(fd, name, expected.Device, options)
	if err != nil {
		_ = unix.Close(fd)
		return err
	}
	if plan.identity != expected {
		closeRemovalPlan(plan)
		return fmt.Errorf("ownership collision: %s/%s identity is %s, expected %s", d.path, name, plan.identity, expected)
	}
	if options.RequireEmpty && len(plan.children) != 0 {
		closeRemovalPlan(plan)
		return fmt.Errorf("ownership collision: directory %s/%s is not empty", d.path, name)
	}
	if !options.Recursive && !options.RequireEmpty && len(plan.children) != 0 {
		closeRemovalPlan(plan)
		return fmt.Errorf("ownership collision: recursive removal was not authorized for %s/%s", d.path, name)
	}
	if err := d.verifyVisible(name, expected); err != nil {
		closeRemovalPlan(plan)
		return err
	}
	if err := deleteRemovalChildren(plan, options); err != nil {
		closeRemovalPlan(plan)
		return err
	}
	if err := unix.Unlinkat(d.fd, name, unix.AT_REMOVEDIR); err != nil {
		closeRemovalPlan(plan)
		return fmt.Errorf("remove verified directory %s/%s: %w", d.path, name, err)
	}
	var after unix.Stat_t
	if err := unix.Fstat(plan.fd, &after); err != nil || after.Nlink != 0 {
		closeRemovalPlan(plan)
		return fmt.Errorf("verified directory unlink could not be proven for %s/%s", d.path, name)
	}
	closeRemovalPlan(plan)
	return nil
}

func (d *Directory) verifyCanonical() error {
	if d == nil || d.closed || d.fd < 0 {
		return errors.New("pinned directory is closed")
	}
	current, err := identityFromFD(d.fd)
	if err != nil || !sameObject(current, d.identity) {
		return fmt.Errorf("ownership collision: pinned directory %s changed", d.path)
	}
	visible, err := identityAt(unix.AT_FDCWD, d.path)
	if err != nil || !sameObject(visible, d.identity) {
		return fmt.Errorf("ownership collision: canonical directory %s changed", d.path)
	}
	return nil
}

func (d *Directory) verifyVisible(name string, expected Identity) error {
	current, err := identityAt(d.fd, name)
	if err != nil || current != expected {
		return fmt.Errorf("ownership collision: %s/%s changed before removal", d.path, name)
	}
	return nil
}

type removalPlan struct {
	name     string
	fd       int
	identity Identity
	children []*removalPlan
}

func buildRemovalPlan(fd int, name string, rootDevice uint64, options RemoveOptions) (*removalPlan, error) {
	identity, err := identityFromFD(fd)
	if err != nil {
		return nil, fmt.Errorf("identify removal candidate %s: %w", name, err)
	}
	if identity.Kind != KindDirectory || !options.AllowsUID(identity.UID) {
		return nil, fmt.Errorf("ownership collision: %s is not an owned ordinary directory", name)
	}
	if options.SameDevice && identity.Device != rootDevice {
		return nil, fmt.Errorf("ownership collision: %s crosses a filesystem boundary", name)
	}
	plan := &removalPlan{name: name, fd: fd, identity: identity}
	names, err := directoryNames(fd)
	if err != nil {
		return nil, fmt.Errorf("list removal candidate %s: %w", name, err)
	}
	for _, childName := range names {
		childFD, childIdentity, openErr := openPathEntry(fd, childName)
		if openErr != nil {
			closeRemovalPlan(plan)
			return nil, fmt.Errorf("inspect removal child %s/%s: %w", name, childName, openErr)
		}
		if !options.AllowsUID(childIdentity.UID) || (options.SameDevice && childIdentity.Device != rootDevice) {
			_ = unix.Close(childFD)
			closeRemovalPlan(plan)
			return nil, fmt.Errorf("ownership collision: removal child %s/%s is not owned", name, childName)
		}
		if childIdentity.Kind == KindDirectory {
			_ = unix.Close(childFD)
			directoryFD, directoryErr := unix.Openat(fd, childName, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
			if directoryErr != nil {
				closeRemovalPlan(plan)
				return nil, fmt.Errorf("open removal child directory %s/%s: %w", name, childName, directoryErr)
			}
			child, childErr := buildRemovalPlan(directoryFD, childName, rootDevice, options)
			if childErr != nil {
				_ = unix.Close(directoryFD)
				closeRemovalPlan(plan)
				return nil, childErr
			}
			plan.children = append(plan.children, child)
			continue
		}
		plan.children = append(plan.children, &removalPlan{
			name:     childName,
			fd:       childFD,
			identity: childIdentity,
		})
	}
	return plan, nil
}

func deleteRemovalChildren(plan *removalPlan, options RemoveOptions) error {
	for _, child := range plan.children {
		current, err := identityAt(plan.fd, child.name)
		if err != nil || current != child.identity {
			return fmt.Errorf("ownership collision: removal child %s changed", child.name)
		}
		if child.identity.Kind == KindDirectory {
			if !options.Recursive {
				return fmt.Errorf("ownership collision: nested directory %s requires recursive removal", child.name)
			}
			if err := deleteRemovalChildren(child, options); err != nil {
				return err
			}
			if err := unix.Unlinkat(plan.fd, child.name, unix.AT_REMOVEDIR); err != nil {
				return fmt.Errorf("remove verified child directory %s: %w", child.name, err)
			}
		} else {
			if err := unix.Unlinkat(plan.fd, child.name, 0); err != nil {
				return fmt.Errorf("unlink verified child %s: %w", child.name, err)
			}
		}
		var after unix.Stat_t
		if err := unix.Fstat(child.fd, &after); err != nil || after.Nlink != 0 {
			return fmt.Errorf("verified child unlink could not be proven for %s", child.name)
		}
	}
	return nil
}

func closeRemovalPlan(plan *removalPlan) {
	if plan == nil {
		return
	}
	for _, child := range plan.children {
		closeRemovalPlan(child)
	}
	if plan.fd >= 0 {
		_ = unix.Close(plan.fd)
		plan.fd = -1
	}
}

func directoryNames(fd int) ([]string, error) {
	duplicate, err := unix.Openat(fd, ".", unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	file, err := fileFromDescriptor(duplicate, "pinned-directory")
	if err != nil {
		_ = unix.Close(duplicate)
		return nil, err
	}
	defer func() { _ = file.Close() }()
	names, err := file.Readdirnames(-1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

func validateLeafName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, '/') || strings.ContainsRune(name, '\x00') {
		return fmt.Errorf("invalid directory leaf %q", name)
	}
	return nil
}

func identityAt(parentFD int, name string) (Identity, error) {
	var stat unix.Stat_t
	if err := unix.Fstatat(parentFD, name, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return Identity{}, err
	}
	return identityFromStat(&stat), nil
}

func identityFromFD(fd int) (Identity, error) {
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return Identity{}, err
	}
	return identityFromStat(&stat), nil
}

func identityFromStat(stat *unix.Stat_t) Identity {
	kind := KindOther
	switch stat.Mode & unix.S_IFMT {
	case unix.S_IFREG:
		kind = KindRegular
	case unix.S_IFDIR:
		kind = KindDirectory
	case unix.S_IFLNK:
		kind = KindSymlink
	}
	return Identity{
		Device: stat.Dev,
		Inode:  stat.Ino,
		Mode:   stat.Mode,
		Links:  stat.Nlink,
		UID:    stat.Uid,
		Kind:   kind,
	}
}

func fileFromDescriptor(fd int, name string) (*os.File, error) {
	if fd < 0 {
		return nil, fmt.Errorf("invalid descriptor for %s", name)
	}
	file := os.NewFile(uintptr(fd), name)
	if file == nil {
		return nil, fmt.Errorf("wrap descriptor for %s", name)
	}
	return file, nil
}

func sameObject(left, right Identity) bool {
	return left.Device == right.Device && left.Inode == right.Inode && left.Kind == right.Kind && left.UID == right.UID
}

func openPathEntry(parentFD int, name string) (int, Identity, error) {
	if err := validateLeafName(name); err != nil {
		return -1, Identity{}, err
	}
	fd, err := unix.Openat(parentFD, name, unix.O_PATH|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return -1, Identity{}, err
	}
	identity, err := identityFromFD(fd)
	if err != nil {
		_ = unix.Close(fd)
		return -1, Identity{}, err
	}
	return fd, identity, nil
}
