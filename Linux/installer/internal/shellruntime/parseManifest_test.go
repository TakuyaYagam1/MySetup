package shellruntime

import (
	"strings"
	"testing"
)

func TestParseManifestSucceedsOnEmbedded(t *testing.T) {
	parsed, err := parseManifest(manifestData)
	if err != nil {
		t.Fatalf("embedded manifest must parse cleanly: %v", err)
	}
	if parsed.DefaultProfile == "" {
		t.Fatal("embedded manifest defaultProfile is empty")
	}
	if len(parsed.Profiles) == 0 {
		t.Fatal("embedded manifest has no profiles")
	}
}

func TestManifestErrorIsNilForEmbeddedManifest(t *testing.T) {
	if err := ManifestError(); err != nil {
		t.Fatalf("init-time manifest validation must succeed for the bundled manifest: %v", err)
	}
}

func TestParseManifestRejectsBrokenJSON(t *testing.T) {
	_, err := parseManifest([]byte("{not json"))
	if err == nil {
		t.Fatal("expected parse error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "parse shell runtime manifest") {
		t.Fatalf("expected wrapped parse error, got %v", err)
	}
}

func TestParseManifestRejectsEmptyDefaultProfile(t *testing.T) {
	_, err := parseManifest([]byte(`{"defaultProfile":"","profiles":[]}`))
	if err == nil {
		t.Fatal("expected error for empty defaultProfile")
	}
	if !strings.Contains(err.Error(), "defaultProfile is empty") {
		t.Fatalf("expected empty defaultProfile error, got %v", err)
	}
}

func TestParseManifestRejectsEmptyProfileID(t *testing.T) {
	_, err := parseManifest([]byte(`{
		"defaultProfile":"caelestia",
		"profiles":[{"id":""}]
	}`))
	if err == nil {
		t.Fatal("expected error for empty profile id")
	}
	if !strings.Contains(err.Error(), "profile id is empty") {
		t.Fatalf("expected empty profile id error, got %v", err)
	}
}

func TestParseManifestRejectsUnknownDefaultProfile(t *testing.T) {
	_, err := parseManifest([]byte(`{
		"defaultProfile":"ghost",
		"profiles":[{"id":"caelestia"}]
	}`))
	if err == nil {
		t.Fatal("expected error when defaultProfile is not listed in profiles")
	}
	if !strings.Contains(err.Error(), "not listed in profiles") {
		t.Fatalf("expected unlisted defaultProfile error, got %v", err)
	}
}
