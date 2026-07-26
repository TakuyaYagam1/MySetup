package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
)

func TestFormatBool(t *testing.T) {
	if got := formatBool(true); got != "enabled" {
		t.Fatalf("expected enabled, got %q", got)
	}
	if got := formatBool(false); got != "disabled" {
		t.Fatalf("expected disabled, got %q", got)
	}
}

func TestPreviewSettingsJoinsBlocksWithBlankSeparator(t *testing.T) {
	lines := previewSettings(
		[]string{"## A", "desc-a", "current: v"},
		[]string{"## B", "desc-b", "current: w"},
	)
	want := []string{
		"## A", "desc-a", "current: v",
		"",
		"## B", "desc-b", "current: w",
	}
	if len(lines) != len(want) {
		t.Fatalf("length mismatch: got %v want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("line %d: got %q want %q", i, lines[i], want[i])
		}
	}
}

func TestPreviewSettingsHandlesEmptyInput(t *testing.T) {
	if got := previewSettings(); len(got) != 0 {
		t.Fatalf("expected no lines for empty call, got %v", got)
	}
}

func TestPreviewSettingUsesCurrentLabel(t *testing.T) {
	got := previewSetting("Locale", "Pick a locale", "en_US.UTF-8")
	if got[0] != "## Locale" {
		t.Fatalf("expected heading prefix, got %q", got[0])
	}
	if got[1] != "Pick a locale" {
		t.Fatalf("expected description line, got %q", got[1])
	}
	if got[2] != "current: en_US.UTF-8" {
		t.Fatalf("expected current label line, got %q", got[2])
	}
}

func TestPreviewSettingWithLabelOverridesValueLabel(t *testing.T) {
	got := previewSettingWithLabel("Mode", "desc", "options", "fast | safe")
	if !strings.HasPrefix(got[2], "options: ") {
		t.Fatalf("expected custom label prefix, got %q", got[2])
	}
}

func TestIsPreviewValueLineAcceptsKnownLabels(t *testing.T) {
	for _, label := range []string{"current", "options", "status", "action", "target", "order", "mode"} {
		line := previewValuePrefix(label) + "value"
		if !isPreviewValueLine(line) {
			t.Fatalf("label %q must be recognised: %q", label, line)
		}
	}
}

func TestIsPreviewValueLineRejectsUnknownPrefix(t *testing.T) {
	if isPreviewValueLine("note: ignored") {
		t.Fatal("unexpected match for unknown prefix")
	}
	if isPreviewValueLine("") {
		t.Fatal("empty line should not match a value line")
	}
}

func TestDetectSecretPathReportsExistsForRegularFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(target, []byte("hash"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := detectSecretPath(target); got != secretPresenceExists {
		t.Fatalf("existing file should be reported as exists, got %q", got)
	}
}

func TestDetectSecretPathReportsMissingForAbsentFile(t *testing.T) {
	target := filepath.Join(t.TempDir(), "ghost.txt")
	if got := detectSecretPath(target); got != secretPresenceMissing {
		t.Fatalf("absent file should be missing, got %q", got)
	}
}

func TestDetectSecretPathReportsMissingForEmptyPath(t *testing.T) {
	if got := detectSecretPath(""); got != secretPresenceMissing {
		t.Fatalf("empty path should be missing, got %q", got)
	}
}

func TestPasswordPathHelpersJoinUnderNixosDest(t *testing.T) {
	opts := paths.Options{NixOSDest: "/srv/nixos"}
	if got := userPasswordHashPath(opts); got != "/srv/nixos/hashed-password.nix" {
		t.Fatalf("user password path mismatch: %q", got)
	}
}

func TestSecretStatusReadyAndExistingPaths(t *testing.T) {
	if got := secretStatus("typed", secretPresenceMissing, "fallback"); got != "ready for apply (session only)" {
		t.Fatalf("typed value should be ready: %q", got)
	}
	if got := secretStatus("", secretPresenceExists, "fallback"); !strings.Contains(got, "already exists") {
		t.Fatalf("existing secret should mention existence: %q", got)
	}
	if got := secretStatus("", secretPresenceUnknown, "fallback"); !strings.Contains(got, "unknown") {
		t.Fatalf("unknown secret should mention unknown: %q", got)
	}
	if got := secretStatus("", secretPresenceMissing, "fallback"); got != "fallback" {
		t.Fatalf("missing secret should fall back: %q", got)
	}
}

func TestSecretSummaryStatusEnumerates(t *testing.T) {
	cases := []struct {
		value    string
		existing secretPresence
		want     string
	}{
		{"x", secretPresenceMissing, "ready"},
		{"", secretPresenceExists, "existing"},
		{"", secretPresenceUnknown, "unknown"},
		{"", secretPresenceMissing, "not-entered"},
	}
	for _, tc := range cases {
		if got := secretSummaryStatus(tc.value, tc.existing); got != tc.want {
			t.Fatalf("secretSummaryStatus(%q, %q) = %q, want %q", tc.value, tc.existing, got, tc.want)
		}
	}
}

func TestClampSectionCursorBoundsToZero(t *testing.T) {
	if got := clampSectionCursor(-5); got != 0 {
		t.Fatalf("negative cursor must clamp to 0, got %d", got)
	}
	if got := clampSectionCursor(0); got != 0 {
		t.Fatalf("zero cursor must stay zero, got %d", got)
	}
	if got := clampSectionCursor(3); got < 0 {
		t.Fatalf("positive cursor must not go negative, got %d", got)
	}
}
