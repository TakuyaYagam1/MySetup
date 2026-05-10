// Package dots manages user-facing dotfiles. This file is the orchestrator
// between hashing (hash.go), tree comparison (compare.go), and marker/backup
// metadata (marker.go).
package dots

import "path/filepath"

// managedSourceAlreadyInstalled returns the source-tree hash plus whether the
// destination already mirrors that source under a managed marker. Callers use
// the hash to refresh the marker even when the on-disk tree is identical.
func managedSourceAlreadyInstalled(src, dst, kind string, excludes map[string]bool) (string, bool, error) {
	sourceHash, err := sourceTreeHash(src, excludes)
	if err != nil {
		return "", false, err
	}
	marker, ok := readManagedMarker(filepath.Join(dst, ".mysetup-managed.json"))
	if !ok || marker.Kind != kind {
		return sourceHash, false, nil
	}
	if marker.SourceHash != "" && marker.SourceHash != sourceHash {
		return sourceHash, false, nil
	}
	matches, err := sourceSubsetMatches(src, dst, excludes)
	if err != nil {
		return sourceHash, false, err
	}
	return sourceHash, matches, nil
}
