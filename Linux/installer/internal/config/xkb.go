package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type XKBLayout struct {
	Code        string
	Description string
}

var xkbLayoutCodeRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func AvailableXKBLayouts() []XKBLayout {
	return mergeXKBLayouts(discoverXKBLayouts(xkbLayoutRuleFiles()), fallbackXKBLayouts())
}

func xkbLayoutRuleFiles() []string {
	ruleDirs := []string{}
	for _, name := range []string{"WAHRWELT_XKB_RULES_DIR", "MYSETUP_XKB_RULES_DIR"} {
		for _, dir := range filepath.SplitList(os.Getenv(name)) {
			if dir != "" {
				ruleDirs = appendUnique(ruleDirs, dir)
			}
		}
	}
	for _, dataDir := range filepath.SplitList(os.Getenv("XDG_DATA_DIRS")) {
		if dataDir != "" {
			ruleDirs = appendUnique(ruleDirs, filepath.Join(dataDir, "X11/xkb/rules"))
		}
	}
	for _, dir := range []string{
		"/run/current-system/sw/share/X11/xkb/rules",
		"/usr/share/X11/xkb/rules",
	} {
		ruleDirs = appendUnique(ruleDirs, dir)
	}

	files := make([]string, 0, len(ruleDirs)*2)
	for _, dir := range ruleDirs {
		files = append(files, filepath.Join(dir, "base.lst"), filepath.Join(dir, "evdev.lst"))
	}
	return files
}

func discoverXKBLayouts(files []string) []XKBLayout {
	seen := map[string]XKBLayout{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		inLayoutSection := false
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "!") {
				inLayoutSection = trimmed == "! layout"
				continue
			}
			if !inLayoutSection {
				continue
			}

			fields := strings.Fields(trimmed)
			if len(fields) < 2 || !ValidXKBLayoutCode(fields[0]) {
				continue
			}
			code := fields[0]
			description := strings.TrimSpace(strings.TrimPrefix(trimmed, code))
			if description == "" {
				continue
			}
			seen[code] = XKBLayout{Code: code, Description: description}
		}
	}

	layouts := make([]XKBLayout, 0, len(seen))
	for _, layout := range seen {
		layouts = append(layouts, layout)
	}
	sortXKBLayouts(layouts)
	return layouts
}

func mergeXKBLayouts(discovered, fallbacks []XKBLayout) []XKBLayout {
	seen := map[string]XKBLayout{}
	for _, layout := range append(discovered, fallbacks...) {
		if !ValidXKBLayoutCode(layout.Code) || layout.Description == "" {
			continue
		}
		if _, ok := seen[layout.Code]; ok {
			continue
		}
		seen[layout.Code] = layout
	}

	layouts := make([]XKBLayout, 0, len(seen))
	for _, layout := range seen {
		layouts = append(layouts, layout)
	}
	sortXKBLayouts(layouts)
	return layouts
}

func sortXKBLayouts(layouts []XKBLayout) {
	sort.Slice(layouts, func(i, j int) bool {
		if layouts[i].Description == layouts[j].Description {
			return layouts[i].Code < layouts[j].Code
		}
		return layouts[i].Description < layouts[j].Description
	})
}

func XKBLayoutLabel(layout XKBLayout) string {
	return fmt.Sprintf("%-7s %s", layout.Code, layout.Description)
}

func SplitKeyboardLayouts(value string) []string {
	parts := strings.Split(value, ",")
	layouts := make([]string, 0, len(parts))
	for _, part := range parts {
		layout := strings.TrimSpace(part)
		if layout == "" || containsString(layouts, layout) {
			continue
		}
		layouts = append(layouts, layout)
	}
	return layouts
}

func JoinKeyboardLayouts(layouts []string) string {
	clean := make([]string, 0, len(layouts))
	for _, layout := range layouts {
		layout = strings.TrimSpace(layout)
		if layout == "" || containsString(clean, layout) {
			continue
		}
		clean = append(clean, layout)
	}
	return strings.Join(clean, ",")
}

func ValidateXKBLayoutSelection(layouts []string) error {
	return ValidateXKBLayoutSelectionAgainst(layouts, AvailableXKBLayouts())
}

func ValidateXKBLayout(layout string) error {
	return ValidateXKBLayoutSelection([]string{layout})
}

func ValidateOptionalXKBLayoutSelection(layouts []string) error {
	if len(layouts) == 0 {
		return nil
	}
	return ValidateXKBLayoutSelection(layouts)
}

func ValidateXKBLayoutSelectionAgainst(layouts []string, available []XKBLayout) error {
	if len(layouts) == 0 {
		return fmt.Errorf("select at least one Hypr/XKB layout")
	}
	known := make(map[string]struct{}, len(available))
	for _, layout := range available {
		known[layout.Code] = struct{}{}
	}
	for _, layout := range layouts {
		if !ValidXKBLayoutCode(layout) {
			return fmt.Errorf("invalid Hypr/XKB layout %q", layout)
		}
		if _, ok := known[layout]; !ok {
			return fmt.Errorf("unknown Hypr/XKB layout %q; choose one from xkeyboard-config", layout)
		}
	}
	return nil
}

func ValidXKBLayoutCode(code string) bool {
	return xkbLayoutCodeRe.MatchString(code)
}

func appendUnique(values []string, value string) []string {
	if containsString(values, value) {
		return values
	}
	return append(values, value)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func fallbackXKBLayouts() []XKBLayout {
	return []XKBLayout{
		{Code: "al", Description: "Albanian"},
		{Code: "et", Description: "Amharic"},
		{Code: "am", Description: "Armenian"},
		{Code: "ara", Description: "Arabic"},
		{Code: "eg", Description: "Arabic (Egypt)"},
		{Code: "iq", Description: "Arabic (Iraq)"},
		{Code: "ma", Description: "Arabic (Morocco)"},
		{Code: "sy", Description: "Arabic (Syria)"},
		{Code: "az", Description: "Azerbaijani"},
		{Code: "ml", Description: "Bambara"},
		{Code: "bd", Description: "Bangla"},
		{Code: "by", Description: "Belarusian"},
		{Code: "be", Description: "Belgian"},
		{Code: "dz", Description: "Berber (Algeria, Latin)"},
		{Code: "ba", Description: "Bosnian"},
		{Code: "brai", Description: "Braille"},
		{Code: "bg", Description: "Bulgarian"},
		{Code: "mm", Description: "Burmese"},
		{Code: "cn", Description: "Chinese"},
		{Code: "hr", Description: "Croatian"},
		{Code: "cz", Description: "Czech"},
		{Code: "dk", Description: "Danish"},
		{Code: "af", Description: "Dari"},
		{Code: "mv", Description: "Dhivehi"},
		{Code: "nl", Description: "Dutch"},
		{Code: "bt", Description: "Dzongkha"},
		{Code: "au", Description: "English (Australia)"},
		{Code: "cm", Description: "English (Cameroon)"},
		{Code: "gh", Description: "English (Ghana)"},
		{Code: "nz", Description: "English (New Zealand)"},
		{Code: "ng", Description: "English (Nigeria)"},
		{Code: "za", Description: "English (South Africa)"},
		{Code: "gb", Description: "English (UK)"},
		{Code: "us", Description: "English (US)"},
		{Code: "epo", Description: "Esperanto"},
		{Code: "ee", Description: "Estonian"},
		{Code: "fo", Description: "Faroese"},
		{Code: "ph", Description: "Filipino"},
		{Code: "fi", Description: "Finnish"},
		{Code: "fr", Description: "French"},
		{Code: "ca", Description: "French (Canada)"},
		{Code: "cd", Description: "French (Democratic Republic of the Congo)"},
		{Code: "tg", Description: "French (Togo)"},
		{Code: "ge", Description: "Georgian"},
		{Code: "de", Description: "German"},
		{Code: "at", Description: "German (Austria)"},
		{Code: "ch", Description: "German (Switzerland)"},
		{Code: "gr", Description: "Greek"},
		{Code: "il", Description: "Hebrew"},
		{Code: "hu", Description: "Hungarian"},
		{Code: "is", Description: "Icelandic"},
		{Code: "in", Description: "Indian"},
		{Code: "id", Description: "Indonesian (Latin)"},
		{Code: "ie", Description: "Irish"},
		{Code: "it", Description: "Italian"},
		{Code: "jp", Description: "Japanese"},
		{Code: "kz", Description: "Kazakh"},
		{Code: "kh", Description: "Khmer (Cambodia)"},
		{Code: "kr", Description: "Korean"},
		{Code: "kg", Description: "Kyrgyz"},
		{Code: "la", Description: "Lao"},
		{Code: "lv", Description: "Latvian"},
		{Code: "lt", Description: "Lithuanian"},
		{Code: "mk", Description: "Macedonian"},
		{Code: "my", Description: "Malay (Jawi, Arabic Keyboard)"},
		{Code: "mt", Description: "Maltese"},
		{Code: "md", Description: "Moldavian"},
		{Code: "mn", Description: "Mongolian"},
		{Code: "me", Description: "Montenegrin"},
		{Code: "np", Description: "Nepali"},
		{Code: "gn", Description: "N'Ko (AZERTY)"},
		{Code: "no", Description: "Norwegian"},
		{Code: "ir", Description: "Persian"},
		{Code: "pl", Description: "Polish"},
		{Code: "pt", Description: "Portuguese"},
		{Code: "br", Description: "Portuguese (Brazil)"},
		{Code: "ro", Description: "Romanian"},
		{Code: "ru", Description: "Russian"},
		{Code: "rs", Description: "Serbian"},
		{Code: "lk", Description: "Sinhala (phonetic)"},
		{Code: "sk", Description: "Slovak"},
		{Code: "si", Description: "Slovenian"},
		{Code: "es", Description: "Spanish"},
		{Code: "latam", Description: "Spanish (Latin American)"},
		{Code: "ke", Description: "Swahili (Kenya)"},
		{Code: "tz", Description: "Swahili (Tanzania)"},
		{Code: "se", Description: "Swedish"},
		{Code: "tw", Description: "Taiwanese"},
		{Code: "tj", Description: "Tajik"},
		{Code: "th", Description: "Thai"},
		{Code: "bw", Description: "Tswana"},
		{Code: "tm", Description: "Turkmen"},
		{Code: "tr", Description: "Turkish"},
		{Code: "ua", Description: "Ukrainian"},
		{Code: "pk", Description: "Urdu (Pakistan)"},
		{Code: "uz", Description: "Uzbek"},
		{Code: "vn", Description: "Vietnamese"},
		{Code: "sn", Description: "Wolof"},
		{Code: "custom", Description: "A user-defined custom Layout"},
	}
}
