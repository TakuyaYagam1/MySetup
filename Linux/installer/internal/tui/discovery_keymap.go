package tui

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
)

func consoleKeymapOptions(current string) []huh.Option[string] {
	keymaps := sortedUnique(discoverConsoleKeymaps(consoleKeymapDirs()), fallbackConsoleKeymaps())
	if current != "" && !containsString(keymaps, current) {
		keymaps = append([]string{current}, keymaps...)
	}
	return stringOptions(keymaps)
}

func consoleKeymapDirs() []string {
	return []string{
		"/run/current-system/sw/share/keymaps",
		"/run/current-system/sw/share/kbd/keymaps",
		"/usr/share/keymaps",
		"/usr/share/kbd/keymaps",
	}
}

func discoverConsoleKeymaps(dirs []string) []string {
	seen := map[string]struct{}{}
	for _, dir := range dirs {
		if err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			keymap, ok := keymapNameFromPath(path)
			if ok {
				seen[keymap] = struct{}{}
			}
			return nil
		}); err != nil {
			continue
		}
	}
	keymaps := make([]string, 0, len(seen))
	for keymap := range seen {
		keymaps = append(keymaps, keymap)
	}
	sort.Strings(keymaps)
	return keymaps
}

func keymapNameFromPath(path string) (string, bool) {
	base := filepath.Base(path)
	switch {
	case strings.HasSuffix(base, ".map.gz"):
		base = strings.TrimSuffix(base, ".map.gz")
	case strings.HasSuffix(base, ".map"):
		base = strings.TrimSuffix(base, ".map")
	default:
		return "", false
	}
	if base == "" {
		return "", false
	}
	return base, true
}

func fallbackConsoleKeymaps() []string {
	return []string{
		"us",
		"ru",
		"de",
		"fr",
		"es",
		"it",
		"jp",
		"pl",
		"pt",
		"uk",
		"br-abnt2",
		"ruwin_alt_sh-UTF-8",
	}
}
