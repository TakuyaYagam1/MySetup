package tui

import (
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
)

func localeOptions(current string) []huh.Option[string] {
	locales := sortedUnique(discoverLocales(localeFiles()), fallbackLocales())
	if current != "" && !containsString(locales, current) {
		locales = append([]string{current}, locales...)
	}
	return stringOptions(locales)
}

func localeFiles() []string {
	return []string{
		"/etc/locale.gen",
		"/run/current-system/sw/share/i18n/SUPPORTED",
		"/usr/share/i18n/SUPPORTED",
	}
}

func discoverLocales(files []string) []string {
	seen := map[string]struct{}{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(strings.TrimSpace(line))
			if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
				continue
			}
			locale := normalizeLocale(fields[0])
			if validLocaleName(locale) {
				seen[locale] = struct{}{}
			}
		}
	}
	locales := make([]string, 0, len(seen))
	for locale := range seen {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	return locales
}

func normalizeLocale(locale string) string {
	locale = strings.TrimSpace(locale)
	locale = strings.Replace(locale, "/UTF-8", ".UTF-8", 1)
	locale = strings.Replace(locale, ".utf8", ".UTF-8", 1)
	locale = strings.Replace(locale, ".UTF8", ".UTF-8", 1)
	return locale
}

func validLocaleName(locale string) bool {
	return strings.Contains(locale, ".") && strings.Contains(locale, "_")
}

func fallbackLocales() []string {
	return []string{
		"en_US.UTF-8",
		"ru_RU.UTF-8",
		"de_DE.UTF-8",
		"en_GB.UTF-8",
		"es_ES.UTF-8",
		"fr_FR.UTF-8",
		"it_IT.UTF-8",
		"ja_JP.UTF-8",
		"nl_NL.UTF-8",
		"pl_PL.UTF-8",
		"pt_BR.UTF-8",
		"tr_TR.UTF-8",
		"uk_UA.UTF-8",
	}
}
