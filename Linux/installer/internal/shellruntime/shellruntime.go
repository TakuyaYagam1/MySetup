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

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
)

const (
	Caelestia = "caelestia"
	Noctalia  = "noctalia"
	End4      = "end4"
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
	ID       string `json:"id"`
	Title    string `json:"title"`
	Accent   string `json:"accent"`
	Surface  string `json:"surface"`
	Logo     string `json:"logo"`
	Launcher string `json:"launcher"`
	Keybinds string `json:"keybinds"`
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
	for _, profile := range parsed.Profiles {
		if profile.ID == "" {
			return Manifest{}, errors.New("shell runtime manifest profile id is empty")
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

func RuntimeDir(home string) string {
	return filepath.Join(paths.XDGStateHome(home), "mysetup", "hypr-runtime")
}

func RuntimeFile(home, name string) string {
	return filepath.Join(RuntimeDir(home), name)
}

func ActiveShellStatePath(home string) string {
	return paths.ActiveShellStatePath(home)
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

func DetectShellFromEntrypoint(entrypointPath, keybindsPath string) string {
	data, err := os.ReadFile(entrypointPath)
	if err != nil {
		return ""
	}
	text := string(data)
	switch {
	case strings.Contains(text, "end4/hyprland.conf"):
		return End4
	case strings.Contains(text, "mysetup/hyprland.conf"):
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
	case strings.Contains(text, "noctalia/keybinds.conf") || strings.Contains(text, "noctalia-shell ipc call") || strings.Contains(text, "noctalia-launcher.sh"):
		return Noctalia
	case strings.Contains(text, "caelestia/keybinds.conf") || strings.Contains(text, "caelestia:launcher"):
		return Caelestia
	default:
		return ""
	}
}

func BootstrapActiveShell(home, hyprDir string) string {
	if profile := ReadActiveShell(ActiveShellStatePath(home)); profile != "" {
		return profile
	}
	if profile := DetectShellFromEntrypoint(RuntimeFile(home, "hyprland.conf"), RuntimeFile(home, "shell-keybinds.conf")); profile != "" {
		return profile
	}
	if profile := DetectShellFromEntrypoint(filepath.Join(hyprDir, "hyprland.conf"), filepath.Join(hyprDir, "shell-keybinds.conf")); profile != "" {
		return profile
	}
	return DefaultProfile
}

// End4SourceFromHomeManager locates the Home Manager-managed end4 hypr profile.
// It first asks the active HM generation gcroot directly; this is stable and
// independent of how user symlinks under XDG_CONFIG_HOME were resolved or torn
// down. Falls back to inspecting ~/.config/quickshell/ii via a single Readlink
// (not EvalSymlinks) so it works even when ii is itself a chain of symlinks
// landing in an end4-quickshell-patched store output.
func End4SourceFromHomeManager(configDir string) (string, error) {
	home := filepath.Dir(configDir)
	if source, err := end4SourceFromGCRoot(home); err != nil || source != "" {
		return source, err
	}
	return end4SourceFromQuickshellLink(configDir)
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
	ok, err := RuntimeConfigExists(filepath.Join(source, "hyprland.conf"))
	if err != nil || !ok {
		return "", err
	}
	return source, nil
}

func end4SourceFromQuickshellLink(configDir string) (string, error) {
	qsPath := filepath.Join(configDir, "quickshell", "ii")
	target, err := os.Readlink(qsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", nil
	}
	suffix := string(os.PathSeparator) + filepath.Join(".config", "quickshell", "ii")
	root, ok := strings.CutSuffix(target, suffix)
	if !ok || root == "" {
		return "", nil
	}
	source := filepath.Join(root, ".config", "hypr", "end4")
	ok, err = RuntimeConfigExists(filepath.Join(source, "hyprland.conf"))
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
