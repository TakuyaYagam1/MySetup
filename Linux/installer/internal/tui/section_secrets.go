package tui

import (
	"os"
	"path/filepath"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
)

type secretPresence string

const (
	secretPresenceMissing secretPresence = "missing"
	secretPresenceExists  secretPresence = "exists"
	secretPresenceUnknown secretPresence = "unknown"
)

type secretAvailability struct {
	UserPassword secretPresence
}

func detectExistingSecrets(opts paths.Options) secretAvailability {
	return secretAvailability{
		UserPassword: detectSecretPaths(userPasswordHashPaths(opts)),
	}
}

func detectSecretPaths(paths []string) secretPresence {
	result := secretPresenceMissing
	for _, path := range paths {
		switch detectSecretPath(path) {
		case secretPresenceExists:
			return secretPresenceExists
		case secretPresenceUnknown:
			result = secretPresenceUnknown
		}
	}
	return result
}

func detectSecretPath(path string) secretPresence {
	if path == "" {
		return secretPresenceMissing
	}
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode().IsRegular() {
			return secretPresenceExists
		}
		return secretPresenceUnknown
	}
	if os.IsNotExist(err) {
		return secretPresenceMissing
	}
	return secretPresenceUnknown
}

func userPasswordHashPath(_ paths.Options) string {
	return paths.DefaultPasswordHashPath
}

func userPasswordHashPaths(opts paths.Options) []string {
	return []string{
		userPasswordHashPath(opts),
		filepath.Join(opts.NixOSDest, "hashed-password.nix"),
		filepath.Join(opts.NixOSDest, "hosts", "NixOS", "hashed-password.nix"),
	}
}

func secretStatus(value string, existing secretPresence, emptyStatus string) string {
	if value != "" {
		return "ready for apply (session only)"
	}
	switch existing {
	case secretPresenceExists:
		return "already exists (preserved if left blank)"
	case secretPresenceUnknown:
		return "unknown (could not check existing file)"
	}
	return emptyStatus
}

func secretSummaryStatus(value string, existing secretPresence) string {
	if value != "" {
		return "ready"
	}
	switch existing {
	case secretPresenceExists:
		return "existing"
	case secretPresenceUnknown:
		return "unknown"
	}
	return "not-entered"
}
