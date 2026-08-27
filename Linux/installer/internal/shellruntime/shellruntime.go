package shellruntime

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
	"golang.org/x/sys/unix"
)

const (
	Caelestia = "caelestia"
	Noctalia  = "noctalia"
	End4      = "end4"
	End4PC    = "end4-pc"

	End4Family = "end4"

	AdapterMarkerPrefix = "-- Wahrwelt shell adapter: "

	canonicalEntrypoint = `-- Wahrwelt canonical Hyprland runtime entrypoint
local home = os.getenv("HOME")
if home == nil then
    error("HOME is not set; cannot locate Wahrwelt Hyprland config")
end

local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
local hypr_root = config_home .. "/hypr"
package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
dofile(hypr_root .. "/user/hyprland.lua")
`
)

var (
	safeLuaModuleName        = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*(\.[A-Za-z_][A-Za-z0-9_-]*)*$`)
	homeManagerFilesStoreDir = regexp.MustCompile(`^[0-9a-df-np-sv-z]{32}-home-manager-files$`)
)

//go:embed manifest.json
var manifestData []byte

var (
	DefaultProfile string
	ProfileSpecs   []Profile
	Profiles       []string
	RuntimeFiles   []string
	HyprScripts    []string
	End4Scripts    []string

	manifestErr error
)

func init() {
	parsed, err := parseManifest(manifestData)
	if err != nil {
		manifestErr = err
		return
	}
	DefaultProfile = parsed.DefaultProfile
	ProfileSpecs = parsed.Profiles
	Profiles = profileIDs(parsed.Profiles)
	RuntimeFiles = parsed.RuntimeFiles
	HyprScripts = parsed.HyprScripts
	End4Scripts = parsed.End4Scripts
}

func ManifestError() error { return manifestErr }

type Manifest struct {
	DefaultProfile string    `json:"defaultProfile"`
	Profiles       []Profile `json:"profiles"`
	RuntimeFiles   []string  `json:"runtimeFiles"`
	HyprScripts    []string  `json:"hyprScripts"`
	End4Scripts    []string  `json:"end4Scripts"`
}

type Profile struct {
	ID               string `json:"id"`
	Family           string `json:"family"`
	Adapter          string `json:"adapter"`
	QuickshellConfig string `json:"quickshellConfig,omitempty"`
	VariantLabel     string `json:"variantLabel,omitempty"`
	Title            string `json:"title"`
	Accent           string `json:"accent"`
	Surface          string `json:"surface"`
	Logo             string `json:"logo"`
	Launcher         string `json:"launcher"`
	Keybinds         string `json:"keybinds"`
}

func parseManifest(data []byte) (Manifest, error) {
	var parsed Manifest
	if err := json.Unmarshal(data, &parsed); err != nil {
		return Manifest{}, fmt.Errorf("parse shell runtime manifest: %w", err)
	}
	if parsed.DefaultProfile == "" {
		return Manifest{}, errors.New("shell runtime manifest defaultProfile is empty")
	}
	seen := map[string]bool{}
	quickshellConfigs := map[string]map[string]bool{}
	for _, profile := range parsed.Profiles {
		if profile.ID == "" {
			return Manifest{}, errors.New("shell runtime manifest profile id is empty")
		}
		if seen[profile.ID] {
			return Manifest{}, fmt.Errorf("shell runtime manifest duplicate profile id: %s", profile.ID)
		}
		if profile.Family == "" {
			return Manifest{}, fmt.Errorf("shell runtime manifest profile %q family is empty", profile.ID)
		}
		if !safeLuaModuleName.MatchString(profile.Adapter) {
			return Manifest{}, fmt.Errorf("shell runtime manifest profile %q adapter is not a safe Lua module name", profile.ID)
		}
		if profile.Family == End4Family {
			if profile.QuickshellConfig == "" {
				return Manifest{}, fmt.Errorf("shell runtime manifest profile %q quickshellConfig is empty", profile.ID)
			}
			if profile.VariantLabel == "" {
				return Manifest{}, fmt.Errorf("shell runtime manifest profile %q variantLabel is empty", profile.ID)
			}
		}
		if profile.QuickshellConfig != "" {
			familyConfigs := quickshellConfigs[profile.Family]
			if familyConfigs == nil {
				familyConfigs = map[string]bool{}
				quickshellConfigs[profile.Family] = familyConfigs
			}
			if familyConfigs[profile.QuickshellConfig] {
				return Manifest{}, fmt.Errorf(
					"shell runtime manifest duplicate quickshellConfig %q in family %q",
					profile.QuickshellConfig,
					profile.Family,
				)
			}
			familyConfigs[profile.QuickshellConfig] = true
		}
		seen[profile.ID] = true
	}
	if !seen[parsed.DefaultProfile] {
		return Manifest{}, errors.New("shell runtime manifest defaultProfile is not listed in profiles")
	}
	return parsed, nil
}

func profileIDs(profiles []Profile) []string {
	ids := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		ids = append(ids, profile.ID)
	}
	return ids
}

func IsProfile(value string) bool {
	return slices.Contains(Profiles, value)
}

func ProfileByID(id string) (Profile, bool) {
	for _, profile := range ProfileSpecs {
		if profile.ID == id {
			return profile, true
		}
	}
	return Profile{}, false
}

func IsFamily(profileID, family string) bool {
	profile, ok := ProfileByID(profileID)
	return ok && profile.Family == family
}

func IsEnd4Profile(profileID string) bool {
	return IsFamily(profileID, End4Family)
}

func AdapterMarker(profileID string) string {
	return AdapterMarkerPrefix + profileID
}

func CanonicalEntrypoint() string {
	return canonicalEntrypoint
}

func HomeManagerInitialCanonicalEntrypoint() string {
	return fmt.Sprintf(`-- Active Hyprland profile: wahrwelt (%s)
local home = os.getenv("HOME")
if home == nil then
    error("HOME is not set; cannot locate Wahrwelt Hyprland config")
end

local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
local hypr_root = config_home .. "/hypr"
package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
dofile(hypr_root .. "/user/hyprland.lua")
`, DefaultProfile)
}

func RuntimeDir(home string) string {
	return filepath.Join(paths.XDGStateHome(home), "wahrwelt", "hypr-runtime")
}

func RuntimeFile(home, name string) string {
	return filepath.Join(RuntimeDir(home), name)
}

func ActiveShellStatePath(home string) string {
	return paths.ActiveShellStatePath(home)
}

func End4VariantStatePath(home string) string {
	return filepath.Join(paths.XDGStateHome(home), "wahrwelt", "end4-variant")
}

func ReadActiveShell(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if profile := strings.TrimSpace(string(data)); IsProfile(profile) {
		return profile
	}
	return ""
}

func ReadEnd4Variant(path string) string {
	if exactRegularFileNoFollow(path, End4PC+"\n") {
		return End4PC
	}
	return End4
}

func exactRegularFileNoFollow(path, expected string) bool {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return false
	}
	if fd < 0 {
		return false
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = unix.Close(fd)
		return false
	}
	defer func() { _ = file.Close() }()

	openedInfo, err := file.Stat()
	if err != nil || !regularFileSizeMatches(openedInfo, len(expected)) {
		return false
	}
	if !pathNamesSameRegularFile(path, openedInfo) {
		return false
	}
	data, err := io.ReadAll(io.LimitReader(file, int64(len(expected)+1)))
	if err != nil || string(data) != expected {
		return false
	}
	openedAfter, err := file.Stat()
	if err != nil || !regularFileSizeMatches(openedAfter, len(data)) || !os.SameFile(openedInfo, openedAfter) {
		return false
	}
	return pathNamesSameRegularFile(path, openedAfter)
}

func regularFileSizeMatches(info os.FileInfo, size int) bool {
	return info.Mode().IsRegular() && info.Size() == int64(size)
}

func pathNamesSameRegularFile(path string, opened os.FileInfo) bool {
	pathInfo, err := os.Lstat(path)
	return err == nil && pathInfo.Mode().IsRegular() && os.SameFile(opened, pathInfo)
}

func DetectShellFromEntrypoint(entrypointPath, keybindsPath string) string {
	return DetectShellFromEntrypointWithEnd4VariantForConfigHome(entrypointPath, keybindsPath, "", "")
}

func DetectShellFromEntrypointWithEnd4Variant(entrypointPath, keybindsPath, end4VariantPath string) string {
	return DetectShellFromEntrypointWithEnd4VariantForConfigHome(entrypointPath, keybindsPath, end4VariantPath, "")
}

func DetectShellFromEntrypointWithEnd4VariantForConfigHome(entrypointPath, keybindsPath, _ string, _ string) string {
	data, err := os.ReadFile(entrypointPath)
	if err != nil {
		return ""
	}
	text := string(data)
	if isKnownCanonicalEntrypoint(text) {
		return DetectShellFromKeybinds(keybindsPath)
	}
	return ""
}

func DetectShellFromKeybinds(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	firstLine, _, _ := strings.Cut(string(data), "\n")
	for _, profile := range ProfileSpecs {
		if firstLine == AdapterMarker(profile.ID) {
			return profile.ID
		}
	}
	return ""
}

func IsCanonicalEntrypoint(path string) bool {
	data, err := os.ReadFile(path)
	return err == nil && isKnownCanonicalEntrypoint(string(data))
}

func isKnownCanonicalEntrypoint(text string) bool {
	return text == canonicalEntrypoint || text == HomeManagerInitialCanonicalEntrypoint()
}

func BootstrapActiveShell(home, hyprDir string) string {
	if profile := ReadActiveShell(ActiveShellStatePath(home)); profile != "" {
		return profile
	}
	variantPath := End4VariantStatePath(home)
	configHome := filepath.Dir(hyprDir)
	if profile := DetectShellFromEntrypointWithEnd4VariantForConfigHome(RuntimeFile(home, "hyprland.lua"), RuntimeFile(home, "shell-keybinds.lua"), variantPath, configHome); profile != "" {
		return profile
	}
	if profile := DetectShellFromEntrypointWithEnd4VariantForConfigHome(filepath.Join(hyprDir, "hyprland.lua"), filepath.Join(hyprDir, "shell-keybinds.lua"), variantPath, configHome); profile != "" {
		return profile
	}
	return DefaultProfile
}

func End4SourceFromHomeManager(configDir string) (string, error) {
	home := filepath.Dir(configDir)
	return End4SourceForProfileFromHomeManager(configDir, ReadEnd4Variant(End4VariantStatePath(home)))
}

func End4SourceForProfileFromHomeManager(configDir, _ string) (string, error) {
	return end4SourceFromGCRoot(filepath.Dir(configDir))
}

func ProvenEnd4SourcesFromHomeManager(configDir string) ([]string, error) {
	var sources []string
	current, err := end4SourceFromGCRoot(filepath.Dir(configDir))
	if err != nil {
		return nil, err
	}
	if current != "" {
		sources = append(sources, current)
	}
	immutable, err := immutableEnd4SourceFromTarget(filepath.Join(configDir, "hypr", "end4"))
	if err != nil {
		return nil, err
	}
	if immutable != "" && !slices.Contains(sources, immutable) {
		sources = append(sources, immutable)
	}
	return sources, nil
}

func ValidateEnd4TargetOwnership(target string, sources []string) error {
	targetLink, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("end4 profile target is an unreadable or broken collision: %s", target)
		}
		return err
	}
	if targetLink.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("refusing to mutate unowned End4 profile collision: target is not a symlink: %s", target)
	}
	targetInfo, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("end4 profile target is an unreadable or broken collision: %s", target)
		}
		return err
	}
	if !targetInfo.IsDir() {
		return fmt.Errorf("refusing to mutate unowned End4 profile collision: symlink does not resolve to a directory: %s", target)
	}
	for _, source := range sources {
		sourceInfo, err := os.Stat(source)
		if err != nil {
			return fmt.Errorf("exact Home Manager End4 source is unreadable: %s: %w", source, err)
		}
		if sourceInfo.IsDir() && os.SameFile(targetInfo, sourceInfo) {
			return nil
		}
	}
	return fmt.Errorf("refusing to mutate unowned End4 profile collision: %s does not resolve to a proven Home Manager source", target)
}

func end4SourceFromGCRoot(home string) (string, error) {
	gcroot := filepath.Join(home, ".local", "state", "home-manager", "gcroots", "current-home")
	target, err := os.Readlink(gcroot)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(gcroot), target)
	}
	source := filepath.Join(target, "home-files", ".config", "hypr", "end4")
	info, err := os.Lstat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", nil
	}
	return validateEnd4Source(source)
}

func immutableEnd4SourceFromTarget(targetPath string) (string, error) {
	info, err := os.Lstat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", nil
	}
	target, err := os.Readlink(targetPath)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(targetPath), target)
	}
	target = filepath.Clean(target)
	if !isImmutableHomeManagerEnd4Source(target) {
		return "", nil
	}
	return validateEnd4Source(target)
}

func isImmutableHomeManagerEnd4Source(path string) bool {
	rel, err := filepath.Rel(filepath.Clean("/nix/store"), filepath.Clean(path))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	return len(parts) == 4 &&
		homeManagerFilesStoreDir.MatchString(parts[0]) &&
		parts[1] == ".config" &&
		parts[2] == "hypr" &&
		parts[3] == "end4"
}

func validateEnd4Source(source string) (string, error) {
	entrypoint := filepath.Join(source, "hyprland.lua")
	info, err := os.Stat(entrypoint)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", nil
	}
	return source, nil
}

func RuntimeConfigExists(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
