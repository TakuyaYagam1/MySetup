package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
)

func TestSectionPreviewFollowsSelectedSection(t *testing.T) {
	state := config.Default()
	state.Locale.TimeZone = "Asia/Tokyo"
	model := sectionModel{cursor: sectionIndex(t, "Region"), state: state}

	preview := strings.Join(sectionPreview(model), "\n")
	if !strings.Contains(preview, "## Timezone\nNix time.timeZone value.\ncurrent: Asia/Tokyo") {
		t.Fatalf("expected region preview, got:\n%s", preview)
	}
	if strings.Contains(preview, "Hostname") {
		t.Fatalf("region preview should not show general host summary, got:\n%s", preview)
	}
}

func TestSectionPreviewUsesSettingBlocksWithBlankLines(t *testing.T) {
	state := config.Default()
	state.User.Username = "takuya"
	state.User.FullName = "Takuya"
	state.User.HomeDirectory = "/home/takuya"
	model := sectionModel{cursor: sectionIndex(t, "User"), state: state}
	preview := sectionPreview(model)
	joined := strings.Join(preview, "\n")

	for _, want := range []string{
		"## Username\nLinux account name used by users/user.nix and Home Manager.\ncurrent: takuya",
		"## Full name\nDisplay name stored in the generated user module.\ncurrent: Takuya",
		"## Home directory\nUser home used by Home Manager and dots install paths.\ncurrent: /home/takuya",
		"## Git identity\nGit user.name and user.email written by Home Manager.",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected user preview block %q, got:\n%s", want, joined)
		}
	}
	if !hasBlankLineAfterBlock(preview, "current: takuya") {
		t.Fatalf("expected blank line between setting blocks, got %#v", preview)
	}
}

func TestRenderPreviewLineStylesHeadingsDescriptionsAndValues(t *testing.T) {
	source, err := os.ReadFile("section_render.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"previewHeadingStyle.Render(line)",
		"previewDescriptionStyle.Render(line)",
		"previewValueStyle.Render(line)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected preview renderer source to contain %q", want)
		}
	}

	cases := []struct {
		name string
		line string
	}{
		{name: "heading", line: "## Timezone"},
		{name: "description", line: "Nix time.timeZone value."},
		{name: "value", line: "current: Europe/Moscow"},
		{name: "options", line: "options: caelestia, noctalia, end4"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rendered := renderPreviewLine(80, tc.line)
			if !strings.Contains(rendered, tc.line) {
				t.Fatalf("expected rendered preview line to contain %q, got %q", tc.line, rendered)
			}
		})
	}
}

func TestApplyPreviewDescribesTransactionalOrder(t *testing.T) {
	model := sectionModel{cursor: sectionIndex(t, "Apply"), state: config.Default()}
	preview := strings.Join(sectionPreview(model), "\n")
	for _, want := range []string{
		"## Stage config",
		"temporary flake containing host hardware",
		"## Dry-build",
		"before touching /etc/nixos",
		"## Mirror /etc/nixos",
		"## Apply dots",
		"## Switch",
		"TUI always asks before nixos-rebuild switch.",
	} {
		if !strings.Contains(preview, want) {
			t.Fatalf("apply preview missing %q, got:\n%s", want, preview)
		}
	}
	if strings.Contains(preview, "before dry-build") {
		t.Fatalf("apply preview should not claim user dots apply before dry-build, got:\n%s", preview)
	}
	if strings.Contains(preview, "--yes") {
		t.Fatalf("TUI preview should not mention CLI --yes behavior, got:\n%s", preview)
	}
}

func TestSectionPreviewPackagesExplainsPresetAndRiskyToggles(t *testing.T) {
	state := config.Default()
	model := sectionModel{cursor: sectionIndex(t, "Packages"), state: state}

	preview := strings.Join(sectionPreview(model), "\n")
	for _, want := range []string{
		"Personal keeps the full Takuya package set",
		"CTF tools",
		"Lanzaboote should stay disabled",
	} {
		if !strings.Contains(preview, want) {
			t.Fatalf("expected package preview to contain %q, got:\n%s", want, preview)
		}
	}
}

func TestPackagesFormDocumentsSecureBoot(t *testing.T) {
	source, err := os.ReadFile("tui_packages.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		`Title("Secure Boot (Lanzaboote)")`,
		`huh.NewSelect[bool]()`,
		"Enable only on machines already prepared for Secure Boot",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected package form source to contain %q", want)
		}
	}
}
