package shellruntime

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
)

const (
	Caelestia = "caelestia"
	Noctalia  = "noctalia"
	End4      = "end4"
	End4PC    = "end4-pc"

	End4Family = "end4"
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
	data, err := os.ReadFile(path)
	if err != nil {
		return End4
	}
	switch string(data) {
	case End4 + "\n":
		return End4
	case End4PC + "\n":
		return End4PC
	}
	return End4
}

func DetectShellFromEntrypoint(entrypointPath, keybindsPath string) string {
	return DetectShellFromEntrypointWithEnd4Variant(entrypointPath, keybindsPath, "")
}

func DetectShellFromEntrypointWithEnd4Variant(entrypointPath, keybindsPath, end4VariantPath string) string {
	data, err := os.ReadFile(entrypointPath)
	if err != nil {
		return ""
	}
	text := string(data)
	switch {
	case strings.Contains(text, "end4/hyprland.lua"):
		if end4VariantPath != "" {
			return ReadEnd4Variant(end4VariantPath)
		}
		return End4
	case strings.Contains(text, "wahrwelt/hyprland.lua"),
		strings.Contains(text, "mysetup/hyprland.lua"):
		return DetectShellFromKeybinds(keybindsPath)
	default:
		return ""
	}
}

func DetectShellFromKeybinds(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := string(data)
	switch {
	case strings.Contains(text, "noctalia.keybinds") || strings.Contains(text, "noctalia/keybinds.lua") || strings.Contains(text, "noctalia-shell ipc call") || strings.Contains(text, "noctalia-msg.sh") || strings.Contains(text, "noctalia-launcher.sh"):
		return Noctalia
	case strings.Contains(text, "caelestia.keybinds") || strings.Contains(text, "caelestia/keybinds.lua") || strings.Contains(text, "caelestia:launcher"):
		return Caelestia
	default:
		return ""
	}
}

func BootstrapActiveShell(home, hyprDir string) string {
	if profile := ReadActiveShell(ActiveShellStatePath(home)); profile != "" {
		return profile
	}
	if profile := ReadActiveShell(paths.LegacyActiveShellStatePath(home)); profile != "" {
		return profile
	}
	variantPath := End4VariantStatePath(home)
	if profile := DetectShellFromEntrypointWithEnd4Variant(RuntimeFile(home, "hyprland.lua"), RuntimeFile(home, "shell-keybinds.lua"), variantPath); profile != "" {
		return profile
	}
	if profile := DetectShellFromEntrypointWithEnd4Variant(filepath.Join(hyprDir, "hyprland.lua"), filepath.Join(hyprDir, "shell-keybinds.lua"), variantPath); profile != "" {
		return profile
	}
	return DefaultProfile
}

func End4SourceFromHomeManager(configDir string) (string, error) {
	home := filepath.Dir(configDir)
	return End4SourceForProfileFromHomeManager(configDir, ReadEnd4Variant(End4VariantStatePath(home)))
}

func End4SourceForProfileFromHomeManager(configDir, profileID string) (string, error) {
	profile, ok := ProfileByID(profileID)
	if !ok || profile.Family != End4Family {
		profile, _ = ProfileByID(End4)
	}
	if source, err := end4SourceFromQuickshellLink(configDir, profile.QuickshellConfig); err != nil || source != "" {
		return source, err
	}
	return end4SourceFromGCRoot(filepath.Dir(configDir))
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
	source := filepath.Join(target, "home-files", ".config", "hypr", "end4")
	ok, err := RuntimeConfigExists(filepath.Join(source, "hyprland.lua"))
	if err != nil || !ok {
		return "", err
	}
	return source, nil
}

func end4SourceFromQuickshellLink(configDir, configName string) (string, error) {
	qsPath := filepath.Join(configDir, "quickshell", configName)
	target, err := os.Readlink(qsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", nil
	}
	suffix := string(os.PathSeparator) + filepath.Join(".config", "quickshell", configName)
	root, ok := strings.CutSuffix(target, suffix)
	if !ok || root == "" {
		return "", nil
	}
	source := filepath.Join(root, ".config", "hypr", "end4")
	ok, err = RuntimeConfigExists(filepath.Join(source, "hyprland.lua"))
	if err != nil || !ok {
		return "", err
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
