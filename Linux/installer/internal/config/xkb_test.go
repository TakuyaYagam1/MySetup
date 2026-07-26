package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestXKBRuleFilesPreferWahrweltEnvironmentAndKeepLegacyFallback(t *testing.T) {
	wahrwelt := filepath.Join(t.TempDir(), "wahrwelt-rules")
	legacy := filepath.Join(t.TempDir(), "mysetup-rules")
	t.Setenv("WAHRWELT_XKB_RULES_DIR", wahrwelt)
	t.Setenv("MYSETUP_XKB_RULES_DIR", legacy)
	t.Setenv("XDG_DATA_DIRS", "")

	files := xkbLayoutRuleFiles()
	wantPrefix := []string{
		filepath.Join(wahrwelt, "base.lst"),
		filepath.Join(wahrwelt, "evdev.lst"),
		filepath.Join(legacy, "base.lst"),
		filepath.Join(legacy, "evdev.lst"),
	}
	if len(files) < len(wantPrefix) {
		t.Fatalf("XKB rule file list too short: %#v", files)
	}
	for index, want := range wantPrefix {
		if files[index] != want {
			t.Fatalf("XKB rule priority at index %d = %q, want %q; all files: %#v", index, files[index], want, files)
		}
	}
}

func TestDiscoverXKBLayoutsFromRulesFile(t *testing.T) {
	dir := t.TempDir()
	rules := filepath.Join(dir, "base.lst")
	if err := os.WriteFile(rules, []byte(`
! model
  pc105           Generic 105-key PC

! layout
  gb              English (UK)
  us              English (US)
  ru              Russian

! variant
  dvorak          us: English (Dvorak)
`), 0o644); err != nil {
		t.Fatal(err)
	}

	layouts := discoverXKBLayouts([]string{rules})
	for _, want := range []XKBLayout{
		{Code: "gb", Description: "English (UK)"},
		{Code: "us", Description: "English (US)"},
		{Code: "ru", Description: "Russian"},
	} {
		if !containsXKBLayout(layouts, want) {
			t.Fatalf("expected XKB layout %#v in %#v", want, layouts)
		}
	}
	if containsXKBLayout(layouts, XKBLayout{Code: "dvorak", Description: "us: English (Dvorak)"}) {
		t.Fatalf("variant section must not be treated as layouts: %#v", layouts)
	}
}

func TestValidateXKBLayoutSelectionRejectsUnknownLayouts(t *testing.T) {
	available := []XKBLayout{
		{Code: "gb", Description: "English (UK)"},
		{Code: "ru", Description: "Russian"},
	}
	if err := ValidateXKBLayoutSelectionAgainst([]string{"gb", "ru"}, available); err != nil {
		t.Fatalf("expected known XKB layouts to validate: %v", err)
	}
	if err := ValidateXKBLayoutSelectionAgainst([]string{"uk"}, available); err == nil {
		t.Fatal("expected console-only uk keymap to be rejected as an XKB layout")
	}
}

func TestValidateOptionalXKBLayoutSelectionAllowsEmptySelection(t *testing.T) {
	if err := ValidateOptionalXKBLayoutSelection(nil); err != nil {
		t.Fatalf("expected empty optional XKB layout selection to validate: %v", err)
	}
	if err := ValidateOptionalXKBLayoutSelection([]string{"gb"}); err != nil {
		t.Fatalf("expected known optional XKB layout to validate: %v", err)
	}
}

func TestValidateRejectsConsoleOnlyUKAsHyprLayout(t *testing.T) {
	state := Default()
	state.Locale.KeyboardLayouts = "uk"
	if err := Validate(state); err == nil {
		t.Fatal("expected config validation to reject uk as a Hypr/XKB layout")
	}

	state.Locale.KeyboardLayouts = "gb"
	if err := Validate(state); err != nil {
		t.Fatalf("expected gb to validate as English UK XKB layout: %v", err)
	}
}

func TestFallbackXKBLayoutsCoverBaseLayoutList(t *testing.T) {
	layouts := fallbackXKBLayouts()
	if got, want := len(layouts), 99; got != want {
		t.Fatalf("expected fallback to cover all %d xkeyboard-config top-level layouts, got %d", want, got)
	}
	for _, want := range []XKBLayout{
		{Code: "eg", Description: "Arabic (Egypt)"},
		{Code: "iq", Description: "Arabic (Iraq)"},
		{Code: "gb", Description: "English (UK)"},
		{Code: "custom", Description: "A user-defined custom Layout"},
	} {
		if !containsXKBLayout(layouts, want) {
			t.Fatalf("expected fallback XKB layout %#v in %#v", want, layouts)
		}
	}
}

func TestKeyboardLayoutsSplitJoinNormalizesValues(t *testing.T) {
	got := JoinKeyboardLayouts(SplitKeyboardLayouts(" gb, ru,gb ,, us "))
	if got != "gb,ru,us" {
		t.Fatalf("expected normalized keyboard layouts, got %q", got)
	}
}

func containsXKBLayout(layouts []XKBLayout, want XKBLayout) bool {
	for _, layout := range layouts {
		if layout == want {
			return true
		}
	}
	return false
}
