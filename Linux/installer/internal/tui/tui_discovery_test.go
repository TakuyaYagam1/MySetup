package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverTimeZonesFromZoneInfoDir(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{
		"Europe/Amsterdam",
		"Europe/Berlin",
		"posix/Europe/Moscow",
		"zone.tab",
	} {
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("tz"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	timeZones := discoverTimeZones([]string{root})
	for _, want := range []string{"Europe/Amsterdam", "Europe/Berlin"} {
		if !containsString(timeZones, want) {
			t.Fatalf("expected discovered timezone %q in %v", want, timeZones)
		}
	}
	if containsString(timeZones, "posix/Europe/Moscow") {
		t.Fatalf("posix timezone mirror should be ignored: %v", timeZones)
	}
	if containsString(timeZones, "zone.tab") {
		t.Fatalf("zone.tab metadata should be ignored: %v", timeZones)
	}
}

func TestTimeZoneOptionsPreserveCustomCurrentValue(t *testing.T) {
	options := timeZoneOptions("Custom/MoonBase")
	if len(options) == 0 || options[0].Value != "Custom/MoonBase" {
		t.Fatalf("expected custom current timezone to be first option, got %#v", options)
	}
}

func TestLocaleOptionsPreserveCustomCurrentValue(t *testing.T) {
	options := localeOptions("eo_EO.UTF-8")
	if len(options) == 0 || options[0].Value != "eo_EO.UTF-8" {
		t.Fatalf("expected custom current locale to be first option, got %#v", options)
	}
	if !containsOption(options, "en_US.UTF-8") {
		t.Fatalf("expected fallback locale list to include en_US.UTF-8, got %#v", options)
	}
}

func TestDiscoverLocalesFromSupportedFile(t *testing.T) {
	dir := t.TempDir()
	supported := filepath.Join(dir, "SUPPORTED")
	if err := os.WriteFile(supported, []byte(`
# comment
en_US.UTF-8 UTF-8
ru_RU.UTF-8 UTF-8
de_DE/UTF-8 UTF-8
`), 0o644); err != nil {
		t.Fatal(err)
	}

	locales := discoverLocales([]string{supported})
	for _, want := range []string{"en_US.UTF-8", "ru_RU.UTF-8", "de_DE.UTF-8"} {
		if !containsString(locales, want) {
			t.Fatalf("expected locale %q in %v", want, locales)
		}
	}
}

func TestConsoleKeymapOptionsPreserveCustomCurrentValue(t *testing.T) {
	options := consoleKeymapOptions("custom-map")
	if len(options) == 0 || options[0].Value != "custom-map" {
		t.Fatalf("expected custom keymap to be first option, got %#v", options)
	}
	if !containsOption(options, "us") {
		t.Fatalf("expected fallback keymap list to include us, got %#v", options)
	}
}

func TestDiscoverConsoleKeymapsFromDir(t *testing.T) {
	dir := t.TempDir()
	for _, path := range []string{
		"i386/qwerty/us.map.gz",
		"i386/qwerty/de.map",
		"README",
	} {
		fullPath := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("keymap"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	keymaps := discoverConsoleKeymaps([]string{dir})
	for _, want := range []string{"us", "de"} {
		if !containsString(keymaps, want) {
			t.Fatalf("expected keymap %q in %v", want, keymaps)
		}
	}
	if containsString(keymaps, "README") {
		t.Fatalf("README should not be a keymap: %v", keymaps)
	}
}

func TestWeatherLocationFromTimeZone(t *testing.T) {
	cases := map[string]string{
		"Europe/Amsterdam":               "Amsterdam",
		"America/Argentina/Buenos_Aires": "Buenos Aires",
		"UTC":                            "UTC",
	}
	for timeZone, want := range cases {
		if got := weatherLocationFromTimeZone(timeZone); got != want {
			t.Fatalf("expected %q from %q, got %q", want, timeZone, got)
		}
	}
}

func TestRegionTimezoneUsesBottomFilterSelect(t *testing.T) {
	source, err := os.ReadFile("tui_region.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		`Options(timeZoneOptions(s.state.Locale.TimeZone)...)`,
		`Options(localeOptions(s.state.Locale.DefaultLocale)...)`,
		`Options(localeOptions(s.state.Locale.ExtraLocale)...)`,
		`Options(consoleKeymapOptions(s.state.Locale.ConsoleKeyMap)...)`,
		`weatherLocationFromTimeZone(timeZone)`,
		`newLiveInput().`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected region form source to contain %q", want)
		}
	}
}
