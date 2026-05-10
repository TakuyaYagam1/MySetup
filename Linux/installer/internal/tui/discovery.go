package tui

import (
	"sort"

	"github.com/charmbracelet/huh"
)

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

func stringOptions(values []string) []huh.Option[string] {
	options := make([]huh.Option[string], 0, len(values))
	for _, value := range values {
		options = append(options, huh.NewOption(value, value))
	}
	return options
}

func sortedUnique(values []string, fallbacks []string) []string {
	for _, value := range fallbacks {
		values = appendUnique(values, value)
	}
	sort.Strings(values)
	return values
}
