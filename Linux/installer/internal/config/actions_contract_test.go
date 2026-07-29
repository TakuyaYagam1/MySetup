package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestGitHubActionsUseCurrentVersionTags(t *testing.T) {
	workflowPaths, err := filepath.Glob("../../../../.github/workflows/*.yml")
	if err != nil {
		t.Fatal(err)
	}
	if len(workflowPaths) == 0 {
		t.Fatal("no GitHub Actions workflows found")
	}

	usesSHA := regexp.MustCompile(`(?m)^\s*uses:\s+\S+@[0-9a-f]{40}(?:\s|$)`)
	expectedTags := map[string]string{
		"actions/checkout@":                  "v7.0.1",
		"actions/setup-go@":                  "v7.0.0",
		"cachix/install-nix-action@":         "v31.11.0",
		"nixbuild/nix-quick-install-action@": "v35",
		"nix-community/cache-nix-action@":    "v7",
		"peter-evans/create-pull-request@":   "v8.1.1",
	}

	for _, path := range workflowPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)

		if match := usesSHA.FindString(source); match != "" {
			t.Fatalf("%s contains a SHA-pinned action instead of a version tag: %s", path, strings.TrimSpace(match))
		}

		for action, version := range expectedTags {
			if strings.Contains(source, action) && !strings.Contains(source, action+version) {
				t.Fatalf("%s must use %s%s", path, action, version)
			}
		}
	}
}
