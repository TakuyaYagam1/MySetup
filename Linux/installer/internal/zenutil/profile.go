// Package zenutil exposes shared helpers for locating Zen Browser data on disk.
package zenutil

import (
	"os"
	"path/filepath"
	"strings"
)

// FindProfile walks Zen's known profile roots and returns a directory that looks
// like a usable profile. Profile names containing "default" win; otherwise the
// first directory becomes the fallback. Returns "" when no profile is found.
func FindProfile(home string) string {
	for _, base := range []string{
		filepath.Join(home, ".zen"),
		filepath.Join(home, ".config", "zen"),
	} {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		var fallback string
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			path := filepath.Join(base, entry.Name())
			if strings.Contains(strings.ToLower(entry.Name()), "default") {
				return path
			}
			if fallback == "" {
				fallback = path
			}
		}
		if fallback != "" {
			return fallback
		}
	}
	return ""
}
