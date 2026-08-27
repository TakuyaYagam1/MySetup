package shellruntime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	migrationv1tov2 "github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/migrations/v1_to_v2"
	"golang.org/x/sys/unix"
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

func TestFreshRuntimeDoesNotOwnV1MigrationRecognizers(t *testing.T) {
	data, err := os.ReadFile("shellruntime.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{
		"internal/migrations/v1_to_v2",
		`dofile(hypr_root .. "/wahrwelt/hyprland.lua")`,
		"Wahrwelt Hypr user namespace transition entrypoint",
		"legacyDirectEnd4Entrypoint",
		"LegacyActiveShellStatePath",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("fresh shell runtime owns v1 migration recognizer %q", forbidden)
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
	if !strings.Contains(runtime, `wahrwelt_default_shell_profile="`+DefaultProfile+`"`) {
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
			"logo":     filepath.Join("../../../NixOS/home/shells/quickshell/wahrwelt-shell-selector", profile.Logo),
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

func TestManifestHyprScriptsCoverEveryProfileScriptReference(t *testing.T) {
	managed := make(map[string]bool, len(HyprScripts))
	for _, script := range HyprScripts {
		if managed[script] {
			t.Fatalf("duplicate managed Hypr script %q", script)
		}
		managed[script] = true
	}
	reference := regexp.MustCompile(`scripts/([A-Za-z0-9._-]+)`)
	for _, profile := range ProfileSpecs {
		for label, rel := range map[string]string{"launcher": profile.Launcher, "keybinds": profile.Keybinds} {
			data, err := os.ReadFile(filepath.Join("../../../dots/hypr", filepath.FromSlash(rel)))
			if err != nil {
				t.Fatal(err)
			}
			for _, match := range reference.FindAllStringSubmatch(string(data), -1) {
				if !managed[match[1]] {
					t.Errorf("profile %q %s references Hypr script %q missing from manifest", profile.ID, label, match[1])
				}
			}
		}
	}
}

func TestHomeManagerExposesEveryManifestHyprScript(t *testing.T) {
	module, err := os.ReadFile("../../../NixOS/home/shells/default.nix")
	if err != nil {
		t.Fatal(err)
	}
	libSource, err := os.ReadFile("../../../NixOS/home/lib/dotfiles.nix")
	if err != nil {
		t.Fatal(err)
	}
	contract := string(module) + string(libSource)
	for _, want := range []string{
		"inherit (shellRuntimeManifest) hyprScripts end4Scripts;",
		"inherit (dotfilesLib) hyprScripts;",
		"hyprScriptFiles = lib.genAttrs",
		`map (name: "hypr/scripts/${name}") hyprScripts`,
		"executable = true;",
		`source = dotsRoot + "/${target}";`,
		"xdg.configFile =\n    hyprScriptFiles",
	} {
		if !strings.Contains(contract, want) {
			t.Fatalf("Home Manager script exposure is missing %q", want)
		}
	}
	for _, script := range HyprScripts {
		path := filepath.Join("../../../dots/hypr/scripts", filepath.FromSlash(script))
		if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("Home Manager manifest script %q is not a regular source: info=%v err=%v", script, info, err)
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

func TestReadEnd4VariantRequiresExactPersistedFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "end4-variant")
	tests := []struct {
		name string
		data string
		want string
	}{
		{name: "official", data: "end4\n", want: End4},
		{name: "pc", data: "end4-pc\n", want: End4PC},
		{name: "missing newline", data: "end4-pc", want: End4},
		{name: "leading whitespace", data: " end4-pc\n", want: End4},
		{name: "extra line", data: "end4-pc\nend4\n", want: End4},
		{name: "untrusted value", data: "../../end4-pc\n", want: End4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tt.data), 0o644); err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := ReadEnd4Variant(path); got != tt.want {
				t.Fatalf("ReadEnd4Variant() = %q, want %q", got, tt.want)
			}
			after, err := os.Lstat(path)
			if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
				t.Fatalf("variant state identity or mode changed: before=%v after=%v err=%v", before, after, err)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != tt.data {
				t.Fatalf("variant state content changed: data=%q err=%v", data, err)
			}
		})
	}
}

func TestReadEnd4VariantRejectsSymlinkWithoutMutation(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	path := filepath.Join(dir, "end4-variant")
	if err := os.WriteFile(target, []byte(End4PC+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	targetBefore, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}

	if got := ReadEnd4Variant(path); got != End4 {
		t.Fatalf("symlink variant = %q, want Official", got)
	}
	linkTarget, err := os.Readlink(path)
	if err != nil || linkTarget != target {
		t.Fatalf("variant symlink changed: target=%q err=%v", linkTarget, err)
	}
	targetAfter, err := os.Lstat(target)
	if err != nil || !os.SameFile(targetBefore, targetAfter) {
		t.Fatalf("variant symlink target identity changed: before=%v after=%v err=%v", targetBefore, targetAfter, err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != End4PC+"\n" {
		t.Fatalf("variant symlink target content changed: data=%q err=%v", data, err)
	}
}

func TestReadEnd4VariantRejectsBrokenSymlinkWithoutMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "end4-variant")
	target := "missing-target"
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}

	if got := ReadEnd4Variant(path); got != End4 {
		t.Fatalf("broken symlink variant = %q, want Official", got)
	}
	if got, err := os.Readlink(path); err != nil || got != target {
		t.Fatalf("broken variant symlink changed: target=%q err=%v", got, err)
	}
}

func TestReadEnd4VariantRejectsFIFOWithoutBlockingOrMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "end4-variant")
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}

	result := make(chan string, 1)
	go func() {
		result <- ReadEnd4Variant(path)
	}()
	select {
	case got := <-result:
		if got != End4 {
			t.Fatalf("FIFO variant = %q, want Official", got)
		}
	case <-time.After(time.Second):
		fd, openErr := unix.Open(path, unix.O_WRONLY|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
		if openErr == nil {
			_ = unix.Close(fd)
		}
		t.Fatal("ReadEnd4Variant blocked while opening FIFO")
	}

	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) || after.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("variant FIFO changed: before=%v after=%v err=%v", before, after, err)
	}
}

func TestReadEnd4VariantMissingFileDefaultsWithoutMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-end4-variant")
	if got := ReadEnd4Variant(path); got != End4 {
		t.Fatalf("missing variant = %q, want Official", got)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("missing variant path was mutated: %v", err)
	}
}

func TestDetectShellFromEntrypointUsesEntrypointAndKeybinds(t *testing.T) {
	dir := t.TempDir()
	configHome := filepath.Join(dir, ".config")
	entrypoint := filepath.Join(dir, "hyprland.lua")
	keybinds := filepath.Join(dir, "shell-keybinds.lua")

	if err := os.WriteFile(entrypoint, legacyEnd4Fixture(t, End4), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectShellFromEntrypointWithEnd4VariantForConfigHome(entrypoint, keybinds, "", configHome); got != "" {
		t.Fatalf("fresh runtime detected v1 direct End4 profile: %q", got)
	}

	if err := os.WriteFile(entrypoint, []byte(CanonicalEntrypoint()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keybinds, []byte(AdapterMarker(Noctalia)+"\n"+`require("noctalia.keybinds")`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectShellFromEntrypointWithEnd4VariantForConfigHome(entrypoint, keybinds, "", configHome); got != Noctalia {
		t.Fatalf("expected noctalia profile, got %q", got)
	}
}

func TestDetectShellFromKeybindsRequiresExactFirstLineMarker(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shell-keybinds.lua")
	for _, profile := range ProfileSpecs {
		t.Run(profile.ID, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(AdapterMarker(profile.ID)+"\nrequire(\"adapter\")\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := DetectShellFromKeybinds(path); got != profile.ID {
				t.Fatalf("DetectShellFromKeybinds() = %q, want %q", got, profile.ID)
			}
		})
	}

	for _, content := range []string{
		`require("noctalia.keybinds")` + "\n",
		`local marker = "-- Wahrwelt shell adapter: noctalia"` + "\n",
		`-- unrelated comment mentioning caelestia.keybinds` + "\n",
		"\n" + AdapterMarker(End4) + "\n",
		AdapterMarker(Noctalia) + " extra\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := DetectShellFromKeybinds(path); got != "" {
			t.Fatalf("false-positive marker detection for %q: %q", content, got)
		}
	}
}

func TestFreshRuntimeRejectsV1DirectEnd4Entrypoints(t *testing.T) {
	dir := t.TempDir()
	entrypoint := filepath.Join(dir, "hyprland.lua")
	keybinds := filepath.Join(dir, "shell-keybinds.lua")
	variant := filepath.Join(dir, "end4-variant")
	if err := os.WriteFile(variant, []byte(End4PC+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{End4, End4PC} {
		if err := os.WriteFile(entrypoint, legacyEnd4Fixture(t, profile), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := DetectShellFromEntrypointWithEnd4Variant(entrypoint, keybinds, variant); got != "" {
			t.Fatalf("fresh runtime detected v1 %s entrypoint as %q", profile, got)
		}
	}
}

func TestCanonicalEntrypointRequiresCompleteExactPayload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hyprland.lua")
	for name, payload := range map[string]string{
		"go and bash":          CanonicalEntrypoint(),
		"home manager initial": HomeManagerInitialCanonicalEntrypoint(),
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
				t.Fatal(err)
			}
			if !IsCanonicalEntrypoint(path) {
				t.Fatalf("complete canonical payload %q was not recognized", name)
			}
		})
	}

	payload := CanonicalEntrypoint()
	for name, content := range map[string]string{
		"prefix":                  "-- prefix\n" + payload,
		"suffix":                  payload + "-- suffix\n",
		"truncated":               payload[:len(payload)-20],
		"missing final newline":   strings.TrimSuffix(payload, "\n"),
		"absolute suffix root":    `dofile("/tmp/arbitrary/wahrwelt/hyprland.lua")` + "\n",
		"old mysetup suffix root": `dofile("/tmp/arbitrary/mysetup/hyprland.lua")` + "\n",
		"matching line only":      `dofile(hypr_root .. "/wahrwelt/hyprland.lua")` + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			if IsCanonicalEntrypoint(path) {
				t.Fatalf("non-exact canonical content was accepted: %q", content)
			}
		})
	}
}

func TestFreshRuntimeRejectsAllV1MigrationEntrypoints(t *testing.T) {
	entrypoint := filepath.Join(t.TempDir(), "hyprland.lua")
	keybinds := filepath.Join(filepath.Dir(entrypoint), "shell-keybinds.lua")
	if err := os.WriteFile(keybinds, []byte(AdapterMarker(Noctalia)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"transition": migrationv1tov2.UserNamespaceTransitionEntrypoint(),
		"old user":   migrationv1tov2.LegacyUserEntrypoint(),
		"old HM user": migrationv1tov2.LegacyHomeManagerUserEntrypoint(
			DefaultProfile,
		),
		"seeded wahrwelt": migrationv1tov2.HistoricalHomeManagerSeededUserEntrypoint(
			DefaultProfile,
			migrationv1tov2.LegacyWahrweltNamespace,
		),
		"seeded user": migrationv1tov2.HistoricalHomeManagerSeededUserEntrypoint(
			DefaultProfile,
			migrationv1tov2.CanonicalUserNamespace,
		),
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(entrypoint, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := DetectShellFromEntrypoint(entrypoint, keybinds); got != "" {
				t.Fatalf("fresh runtime detected v1 %s entrypoint as %q", name, got)
			}
		})
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

	if err := os.WriteFile(RuntimeFile(home, "hyprland.lua"), []byte(CanonicalEntrypoint()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(RuntimeFile(home, "shell-keybinds.lua"), []byte(AdapterMarker(Caelestia)+"\n"+`require("caelestia.keybinds")`+"\n"), 0o644); err != nil {
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

func TestFreshBootstrapIgnoresV1DirectEnd4Variant(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")

	home := t.TempDir()
	hyprDir := filepath.Join(home, ".config", "hypr")
	if err := os.MkdirAll(RuntimeDir(home), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(RuntimeFile(home, "hyprland.lua"), legacyEnd4Fixture(t, End4), 0o644); err != nil {
		t.Fatal(err)
	}
	variantPath := filepath.Join(home, ".local", "state", "wahrwelt", "end4-variant")
	if err := os.WriteFile(variantPath, []byte("end4-pc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := BootstrapActiveShell(home, hyprDir); got != DefaultProfile {
		t.Fatalf("fresh bootstrap detected v1 direct End4 entrypoint as %q", got)
	}

	if err := os.WriteFile(variantPath, []byte("../../untrusted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := BootstrapActiveShell(home, hyprDir); got != DefaultProfile {
		t.Fatalf("fresh bootstrap used invalid v1 variant as %q", got)
	}
}

func TestEnd4SourceFromHomeManagerResolvesGenerationSource(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".config")
	end4Source := writeCurrentHomeManagerGeneration(t, home)

	got, err := End4SourceFromHomeManager(configDir)
	if err != nil {
		t.Fatal(err)
	}
	if got != end4Source {
		t.Fatalf("unexpected end4 source: got %q want %q", got, end4Source)
	}
}

func TestEnd4SourceFromHomeManagerRejectsSuffixShapedQuickshellSource(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".config")
	untrustedRoot := t.TempDir()
	end4Source := filepath.Join(untrustedRoot, ".config", "hypr", "end4")
	quickshellSource := filepath.Join(untrustedRoot, ".config", "quickshell", "ii")
	for _, dir := range []string{end4Source, quickshellSource, filepath.Join(configDir, "quickshell")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(end4Source, "hyprland.lua"), []byte("-- untrusted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(quickshellSource, filepath.Join(configDir, "quickshell", "ii")); err != nil {
		t.Fatal(err)
	}
	got, err := End4SourceFromHomeManager(configDir)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("suffix-shaped QuickShell source was trusted: %q", got)
	}
	if err := os.MkdirAll(filepath.Join(configDir, "hypr"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(end4Source, filepath.Join(configDir, "hypr", "end4")); err != nil {
		t.Fatal(err)
	}
	sources, err := ProvenEnd4SourcesFromHomeManager(configDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 0 {
		t.Fatalf("self-controlled End4 target was trusted as Home Manager-owned: %#v", sources)
	}
}

func TestImmutableHomeManagerEnd4SourceShape(t *testing.T) {
	valid := "/nix/store/298dl7szmlqjj2jlv2dm5wsq8zj4kcc8-home-manager-files/.config/hypr/end4"
	if !isImmutableHomeManagerEnd4Source(valid) {
		t.Fatalf("valid immutable Home Manager source was rejected: %s", valid)
	}
	for _, path := range []string{
		"/tmp/298dl7szmlqjj2jlv2dm5wsq8zj4kcc8-home-manager-files/.config/hypr/end4",
		"/nix/store/not-a-hash-home-manager-files/.config/hypr/end4",
		"/nix/store/298dl7szmlqjj2jlv2dm5wsq8zj4kcc8-home-manager-generation/home-files/.config/hypr/end4",
		"/nix/store/298dl7szmlqjj2jlv2dm5wsq8zj4kcc8-home-manager-files/.config/quickshell/ii",
		valid + "/extra",
	} {
		if isImmutableHomeManagerEnd4Source(path) {
			t.Fatalf("unproven Home Manager source shape accepted: %s", path)
		}
	}
}

func legacyEnd4Fixture(t *testing.T, profile string) []byte {
	t.Helper()
	name := profile + ".lua"
	data, err := os.ReadFile(filepath.Join("../../../NixOS/home/migrations/v1_to_v2/hypr-runtime", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeCurrentHomeManagerGeneration(t *testing.T, home string) string {
	t.Helper()
	end4Store := filepath.Join(t.TempDir(), "end4-store")
	if err := os.MkdirAll(end4Store, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(end4Store, "hyprland.lua"), []byte("-- exact HM End4 source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	generation := filepath.Join(t.TempDir(), "home-manager-generation")
	source := filepath.Join(generation, "home-files", ".config", "hypr", "end4")
	if err := os.MkdirAll(filepath.Dir(source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(end4Store, source); err != nil {
		t.Fatal(err)
	}
	gcroot := filepath.Join(home, ".local", "state", "home-manager", "gcroots", "current-home")
	if err := os.MkdirAll(filepath.Dir(gcroot), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(generation, gcroot); err != nil {
		t.Fatal(err)
	}
	return source
}
