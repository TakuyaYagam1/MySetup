package dots

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

const managedMarkerVersion = 2

type managedMarkerFile struct {
	Manager    string `json:"manager"`
	Kind       string `json:"kind"`
	Version    int    `json:"version"`
	SourceHash string `json:"sourceHash,omitempty"`
}

type ManagedRootStatus string

const (
	ManagedRootMissing     ManagedRootStatus = "missing"
	ManagedRootManaged     ManagedRootStatus = "managed"
	ManagedRootUnmanaged   ManagedRootStatus = "unmanaged"
	ManagedRootSymlink     ManagedRootStatus = "symlink"
	ManagedRootNonDir      ManagedRootStatus = "non-dir"
	ManagedRootInvalidMark ManagedRootStatus = "invalid-marker"
)

type ManagedRootInspection struct {
	Path   string
	Kind   string
	Status ManagedRootStatus
}

func InspectManagedRoot(target, kind string) (ManagedRootInspection, error) {
	inspection := ManagedRootInspection{Path: target, Kind: kind}
	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			inspection.Status = ManagedRootMissing
			return inspection, nil
		}
		return inspection, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		inspection.Status = ManagedRootSymlink
		return inspection, nil
	}
	if !info.IsDir() {
		inspection.Status = ManagedRootNonDir
		return inspection, nil
	}

	markerPath := filepath.Join(target, ".mysetup-managed.json")
	markerInfo, err := os.Lstat(markerPath)
	if err != nil {
		if os.IsNotExist(err) {
			inspection.Status = ManagedRootUnmanaged
			return inspection, nil
		}
		return inspection, fmt.Errorf("stat managed marker for %s: %w", target, err)
	}
	if markerInfo.Mode()&os.ModeSymlink != 0 || !markerInfo.Mode().IsRegular() {
		inspection.Status = ManagedRootInvalidMark
		return inspection, nil
	}
	if validManagedMarker(markerPath, kind) {
		inspection.Status = ManagedRootManaged
		return inspection, nil
	}
	inspection.Status = ManagedRootInvalidMark
	return inspection, nil
}

func writeMarker(runner run.CommandRunner, target, kind string) error {
	if runner.IsDryRun() {
		fmt.Printf("write marker %s\n", target)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := replaceExistingMarker(target); err != nil {
		return err
	}
	return os.WriteFile(target, []byte(managedMarker(kind)), 0o644)
}

func writeMarkerWithOwner(ctx context.Context, runner run.CommandRunner, target, kind, username string) error {
	return writeMarkerWithOwnerAndSourceHash(ctx, runner, target, kind, username, "")
}

func writeMarkerWithOwnerAndSourceHash(ctx context.Context, runner run.CommandRunner, target, kind, username, sourceHash string) error {
	if err := writeMarker(runner, target, kind); err != nil {
		if runner.IsDryRun() || !os.IsPermission(err) || username == "" {
			return err
		}
		return sudoInstallMarker(ctx, runner, target, kind, username, sourceHash)
	}
	if sourceHash != "" && !runner.IsDryRun() {
		return writeMarkerWithSourceHash(target, kind, sourceHash)
	}
	return nil
}

func sudoInstallMarker(ctx context.Context, runner run.CommandRunner, target, kind, username, sourceHash string) error {
	temp, err := os.CreateTemp("", "mysetup-managed-marker-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()
	if _, err := temp.WriteString(managedMarkerWithSourceHash(kind, sourceHash)); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := runner.Command(ctx, "sudo", "rm", "-f", "--", target); err != nil {
		return err
	}
	return runner.Command(ctx, "sudo", "install", "-D", "-m", "644", "-o", username, tempPath, target)
}

func writeMarkerWithSourceHash(target, kind, sourceHash string) error {
	if err := replaceExistingMarker(target); err != nil {
		return err
	}
	return os.WriteFile(target, []byte(managedMarkerWithSourceHash(kind, sourceHash)), 0o644)
}

func replaceExistingMarker(target string) error {
	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("refusing to replace managed marker directory: %s", target)
	}
	if err := os.Remove(target); err != nil {
		return fmt.Errorf("remove stale managed marker %s: %w", target, err)
	}
	return nil
}

func managedMarker(kind string) string {
	return managedMarkerWithSourceHash(kind, "")
}

func managedMarkerWithSourceHash(kind, sourceHash string) string {
	marker := managedMarkerFile{
		Manager:    "mysetup",
		Kind:       kind,
		Version:    managedMarkerVersion,
		SourceHash: sourceHash,
	}
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return fmt.Sprintf(
			"{\n  \"manager\": \"mysetup\",\n  \"kind\": %q,\n  \"version\": %d,\n  \"sourceHash\": %q\n}\n",
			kind, managedMarkerVersion, sourceHash,
		)
	}
	return string(append(data, '\n'))
}

func readManagedMarker(path string) (managedMarkerFile, bool) {
	var marker managedMarkerFile
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return marker, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return marker, false
	}
	if err := json.Unmarshal(data, &marker); err != nil {
		return marker, false
	}
	if marker.Manager != "mysetup" || marker.Version != managedMarkerVersion {
		return marker, false
	}
	return marker, true
}

func validManagedMarker(path, kind string) bool {
	marker, ok := readManagedMarker(path)
	return ok && marker.Kind == kind
}
