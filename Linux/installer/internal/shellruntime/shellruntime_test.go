package shellruntime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestRuntimeContractMatchesNixSource(t *testing.T) {
	data, err := os.ReadFile("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var want Manifest
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}

	for name, pair := range map[string]struct {
		got  []string
		want []string
	}{
		"profiles":     {Profiles, profileIDs(want.Profiles)},
		"runtimeFiles": {RuntimeFiles, want.RuntimeFiles},
		"hyprScripts":  {HyprScripts, want.HyprScripts},
		"end4Scripts":  {End4Scripts, want.End4Scripts},
	} {
		if !reflect.DeepEqual(pair.got, pair.want) {
			t.Fatalf("%s drifted from manifest\nGo:  %#v\nManifest: %#v", name, pair.got, pair.want)
		}
	}
}

func TestProfilesMatchNixAndShellRuntime(t *testing.T) {
	profilesData, err := os.ReadFile("../../../NixOS/home/shells/profiles.nix")
	if err != nil {
		t.Fatal(err)
	}
	runtimeData, err := os.ReadFile("../../../dots/hypr/scripts/shell-runtime.sh")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(profilesData), "ordered = shellRuntimeManifest.profiles;") {
		t.Fatalf("profiles.nix must source ordered profile metadata from shell runtime manifest\n%s", profilesData)
	}
	if !strings.Contains(string(profilesData), "inherit (shellRuntimeManifest) defaultProfile;") {
		t.Fatalf("profiles.nix must source defaultProfile from shell runtime manifest\n%s", profilesData)
	}
	runtime := string(runtimeData)
	if !strings.Contains(runtime, `mysetup_default_shell_profile="`+DefaultProfile+`"`) {
		t.Fatalf("shell-runtime default profile drifted from manifest default %q\n%s", DefaultProfile, runtime)
	}
	for _, profile := range Profiles {
		pattern := regexp.MustCompile(`(\|\s*` + regexp.QuoteMeta(profile) + `\b)|(\b` + regexp.QuoteMeta(profile) + `\s*\|)`)
		if !pattern.MatchString(runtime) {
			t.Fatalf("shell-runtime valid profile case is missing %q\n%s", profile, runtime)
		}
	}
}

func TestManifestProfilesReferenceTrackedAssetsAndDotfiles(t *testing.T) {
	for _, profile := range ProfileSpecs {
		for label, rel := range map[string]string{
			"logo":     filepath.Join("../../../NixOS/home/shells/quickshell/mysetup-shell-selector", profile.Logo),
			"launcher": filepath.Join("../../../dots/hypr", profile.Launcher),
			"keybinds": filepath.Join("../../../dots/hypr", profile.Keybinds),
		} {
			info, err := os.Stat(rel)
			if err != nil {
				t.Fatalf("%s profile %q references missing %s %s: %v", profile.ID, profile.Title, label, rel, err)
			}
			if !info.Mode().IsRegular() {
				t.Fatalf("%s profile %q references non-file %s %s", profile.ID, profile.Title, label, rel)
			}
		}
	}
}

func TestReadActiveShellAcceptsKnownProfilesOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "active-shell")
	if got := ReadActiveShell(path); got != "" {
		t.Fatalf("missing state should return empty profile, got %q", got)
	}

	if err := os.WriteFile(path, []byte("noctalia\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ReadActiveShell(path); got != Noctalia {
		t.Fatalf("expected noctalia profile, got %q", got)
	}

	if err := os.WriteFile(path, []byte("unknown\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ReadActiveShell(path); got != "" {
		t.Fatalf("unknown profile should return empty profile, got %q", got)
	}
}

func TestDetectShellFromEntrypointUsesEntrypointAndKeybinds(t *testing.T) {
	dir := t.TempDir()
	entrypoint := filepath.Join(dir, "hyprland.conf")
	keybinds := filepath.Join(dir, "shell-keybinds.conf")

	if err := os.WriteFile(entrypoint, []byte("source = /home/user/.config/hypr/end4/hyprland.conf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectShellFromEntrypoint(entrypoint, keybinds); got != End4 {
		t.Fatalf("expected end4 profile, got %q", got)
	}

	if err := os.WriteFile(entrypoint, []byte("source = /home/user/.config/hypr/mysetup/hyprland.conf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keybinds, []byte("source = /home/user/.config/hypr/noctalia/keybinds.conf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectShellFromEntrypoint(entrypoint, keybinds); got != Noctalia {
		t.Fatalf("expected noctalia profile, got %q", got)
	}
}

func TestBootstrapActiveShellPriorityAndFallback(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")

	home := t.TempDir()
	hyprDir := filepath.Join(home, ".config", "hypr")
	runtimeDir := RuntimeDir(home)
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(hyprDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if got := BootstrapActiveShell(home, hyprDir); got != DefaultProfile {
		t.Fatalf("empty runtime should fall back to default profile, got %q", got)
	}

	if err := os.WriteFile(RuntimeFile(home, "hyprland.conf"), []byte("source = /home/user/.config/hypr/mysetup/hyprland.conf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(RuntimeFile(home, "shell-keybinds.conf"), []byte("source = /home/user/.config/hypr/caelestia/keybinds.conf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := BootstrapActiveShell(home, hyprDir); got != Caelestia {
		t.Fatalf("runtime entrypoint should detect caelestia profile, got %q", got)
	}

	if err := os.MkdirAll(filepath.Dir(ActiveShellStatePath(home)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ActiveShellStatePath(home), []byte("end4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := BootstrapActiveShell(home, hyprDir); got != End4 {
		t.Fatalf("active shell state should take priority, got %q", got)
	}
}

func TestEnd4SourceFromHomeManagerResolvesGenerationSource(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), ".config")
	hmRoot := t.TempDir()
	qsSource := filepath.Join(hmRoot, ".config", "quickshell", "ii")
	end4Source := filepath.Join(hmRoot, ".config", "hypr", "end4")

	for _, dir := range []string{
		filepath.Join(configDir, "quickshell"),
		qsSource,
		end4Source,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(end4Source, "hyprland.conf"), []byte("source = ./custom/general.conf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(qsSource, filepath.Join(configDir, "quickshell", "ii")); err != nil {
		t.Fatal(err)
	}

	got, err := End4SourceFromHomeManager(configDir)
	if err != nil {
		t.Fatal(err)
	}
	if got != end4Source {
		t.Fatalf("unexpected end4 source: got %q want %q", got, end4Source)
	}
}
