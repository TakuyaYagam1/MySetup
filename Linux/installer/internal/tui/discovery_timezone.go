package tui

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
)

func weatherLocationFromTimeZone(timeZone string) string {
	if timeZone == "" {
		return ""
	}
	parts := strings.Split(timeZone, "/")
	city := parts[len(parts)-1]
	city = strings.ReplaceAll(city, "_", " ")
	return city
}

func timeZoneOptions(current string) []huh.Option[string] {
	timeZones := discoverTimeZones(zoneInfoDirs())
	if current != "" && !containsString(timeZones, current) {
		timeZones = append([]string{current}, timeZones...)
	}
	return stringOptions(timeZones)
}

func zoneInfoDirs() []string {
	return []string{"/etc/zoneinfo", "/usr/share/zoneinfo"}
}

func discoverTimeZones(dirs []string) []string {
	seen := map[string]struct{}{}
	for _, dir := range dirs {
		root, err := filepath.EvalSymlinks(dir)
		if err != nil {
			root = dir
		}
		if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			timeZone := filepath.ToSlash(rel)
			if validTimeZoneName(timeZone) {
				seen[timeZone] = struct{}{}
			}
			return nil
		}); err != nil {
			continue
		}
	}
	for _, timeZone := range fallbackTimeZones() {
		seen[timeZone] = struct{}{}
	}
	timeZones := make([]string, 0, len(seen))
	for timeZone := range seen {
		timeZones = append(timeZones, timeZone)
	}
	sort.Strings(timeZones)
	return timeZones
}

func validTimeZoneName(name string) bool {
	if name == "" || strings.HasPrefix(name, "posix/") || strings.HasPrefix(name, "right/") {
		return false
	}
	base := filepath.Base(name)
	if strings.HasPrefix(base, ".") ||
		strings.HasSuffix(base, ".tab") ||
		strings.HasSuffix(base, ".zi") ||
		base == "leapseconds" ||
		base == "localtime" ||
		base == "posixrules" {
		return false
	}
	return strings.Contains(name, "/") || name == "UTC"
}

func fallbackTimeZones() []string {
	return []string{
		"UTC",
		"America/Chicago",
		"America/Los_Angeles",
		"America/New_York",
		"Asia/Tokyo",
		"Europe/Amsterdam",
		"Europe/Berlin",
		"Europe/London",
		"Europe/Moscow",
		"Europe/Paris",
	}
}
