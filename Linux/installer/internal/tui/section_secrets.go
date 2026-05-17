package tui

import (
	"os"
	"path/filepath"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
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
		UserPassword: detectSecretPath(userPasswordHashPath(opts)),
	}
}

func detectSecretPath(path string) secretPresence {
	if path == "" {
		return secretPresenceMissing
	}
	if _, err := os.Stat(path); err == nil {
		return secretPresenceExists
	} else if os.IsNotExist(err) {
		return secretPresenceMissing
	}
	return secretPresenceUnknown
}

func userPasswordHashPath(opts paths.Options) string {
	return filepath.Join(opts.NixOSDest, "hosts", "NixOS", "hashed-password.nix")
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
