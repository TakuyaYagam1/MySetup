package dots

import "path/filepath"

func managedSourceAlreadyInstalled(src, dst, kind string, excludes map[string]bool) (string, bool, error) {
	sourceHash, err := sourceTreeHash(src, excludes)
	if err != nil {
		return "", false, err
	}
	marker, ok := readManagedMarker(filepath.Join(dst, managedMarkerName))
	if !ok {
		marker, ok = readManagedMarker(filepath.Join(dst, legacyManagedMarkerName))
	}
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
