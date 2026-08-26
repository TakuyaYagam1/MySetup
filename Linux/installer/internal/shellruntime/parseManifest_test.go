package shellruntime

import (
	"strconv"
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
		"profiles":[{"id":"caelestia","family":"caelestia","adapter":"caelestia.keybinds"}]
	}`))
	if err == nil {
		t.Fatal("expected error when defaultProfile is not listed in profiles")
	}
	if !strings.Contains(err.Error(), "not listed in profiles") {
		t.Fatalf("expected unlisted defaultProfile error, got %v", err)
	}
}

func TestParseManifestRejectsDuplicateProfileID(t *testing.T) {
	_, err := parseManifest([]byte(`{
		"defaultProfile":"caelestia",
		"profiles":[
			{"id":"caelestia","family":"caelestia","adapter":"caelestia.keybinds"},
			{"id":"caelestia","family":"caelestia","adapter":"caelestia.keybinds"}
		]
	}`))
	if err == nil || !strings.Contains(err.Error(), "duplicate profile id") {
		t.Fatalf("expected duplicate profile id error, got %v", err)
	}
}

func TestParseManifestRequiresProfileFamily(t *testing.T) {
	_, err := parseManifest([]byte(`{
		"defaultProfile":"caelestia",
		"profiles":[{"id":"caelestia","adapter":"caelestia.keybinds"}]
	}`))
	if err == nil || !strings.Contains(err.Error(), "family is empty") {
		t.Fatalf("expected missing family error, got %v", err)
	}
}

func TestParseManifestRejectsDuplicateQuickshellConfigWithinFamily(t *testing.T) {
	_, err := parseManifest([]byte(`{
		"defaultProfile":"end4",
		"profiles":[
			{"id":"end4","family":"end4","adapter":"end4-adapter","quickshellConfig":"ii","variantLabel":"Official"},
			{"id":"end4-pc","family":"end4","adapter":"end4-adapter","quickshellConfig":"ii","variantLabel":"pC"}
		]
	}`))
	if err == nil || !strings.Contains(err.Error(), "duplicate quickshellConfig") {
		t.Fatalf("expected duplicate quickshell config error, got %v", err)
	}
}

func TestParseManifestRequiresEnd4VariantMetadata(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		want    string
	}{
		{
			name:    "quickshell config",
			profile: `{"id":"end4","family":"end4","adapter":"end4-adapter","variantLabel":"Official"}`,
			want:    "quickshellConfig is empty",
		},
		{
			name:    "variant label",
			profile: `{"id":"end4","family":"end4","adapter":"end4-adapter","quickshellConfig":"ii"}`,
			want:    "variantLabel is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseManifest([]byte(`{"defaultProfile":"end4","profiles":[` + tt.profile + `]}`))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestParseManifestRequiresSafeAdapterModule(t *testing.T) {
	for _, adapter := range []string{"", "../end4", "end4/adapter", `end4\"adapter`, ".end4"} {
		t.Run(adapter, func(t *testing.T) {
			_, err := parseManifest([]byte(`{
				"defaultProfile":"caelestia",
				"profiles":[{"id":"caelestia","family":"caelestia","adapter":` + strconv.Quote(adapter) + `}]
			}`))
			if err == nil || !strings.Contains(err.Error(), "adapter is not a safe Lua module name") {
				t.Fatalf("expected unsafe adapter error for %q, got %v", adapter, err)
			}
		})
	}
}

func TestEmbeddedManifestDefinesEnd4Variants(t *testing.T) {
	parsed, err := parseManifest(manifestData)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]Profile{
		"end4": {
			ID:               "end4",
			Family:           "end4",
			Adapter:          "end4-adapter",
			QuickshellConfig: "ii",
			VariantLabel:     "Official",
		},
		"end4-pc": {
			ID:               "end4-pc",
			Family:           "end4",
			Adapter:          "end4-adapter",
			QuickshellConfig: "end4-pC",
			VariantLabel:     "pC",
		},
	}

	for _, profile := range parsed.Profiles {
		if profile.Family == "" {
			t.Fatalf("profile %q has no family", profile.ID)
		}
		if profile.ID != "end4" && profile.ID != "end4-pc" && profile.Family != profile.ID {
			t.Fatalf("profile %q family must match its id, got %q", profile.ID, profile.Family)
		}
		if expected, ok := want[profile.ID]; ok {
			if profile.Family != expected.Family || profile.Adapter != expected.Adapter || profile.QuickshellConfig != expected.QuickshellConfig || profile.VariantLabel != expected.VariantLabel {
				t.Fatalf("profile %q metadata mismatch: got %#v want %#v", profile.ID, profile, expected)
			}
			delete(want, profile.ID)
		}
	}
	if len(want) != 0 {
		t.Fatalf("embedded manifest is missing end4 variants: %#v", want)
	}
}

func TestEmbeddedManifestDefinesCanonicalAdapters(t *testing.T) {
	want := map[string]string{
		Noctalia:  "noctalia.keybinds",
		Caelestia: "caelestia.keybinds",
		End4:      "end4-adapter",
		End4PC:    "end4-adapter",
	}
	for _, profile := range ProfileSpecs {
		if got := profile.Adapter; got != want[profile.ID] {
			t.Fatalf("profile %q adapter = %q, want %q", profile.ID, got, want[profile.ID])
		}
		delete(want, profile.ID)
	}
	if len(want) != 0 {
		t.Fatalf("manifest is missing adapter profiles: %#v", want)
	}
}
