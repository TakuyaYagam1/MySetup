package zenutil

import (
	"os"
	"path/filepath"
	"strings"
)

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
