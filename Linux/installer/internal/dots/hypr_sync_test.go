package dots

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
	migrationv1tov2 "github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/migrations/v1_to_v2"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/shellruntime"
)

func TestSyncHyprReturnsExecutableCommandError(t *testing.T) {
	dotsSrc := t.TempDir()
	writeRequiredHyprSource(t, dotsSrc)
	home := t.TempDir()
	configDir := filepath.Join(home, ".config")
	bin := t.TempDir()
	writeScript(t, filepath.Join(bin, "rsync"), `last=
for arg do last=$arg; done
mkdir -p "$last/scripts" "$last/caelestia" "$last/hyprland"
printf '%s\n' '-- caelestia binds' > "$last/caelestia/keybinds.lua"
printf '%s\n' '-- caelestia launcher' > "$last/caelestia/launcher.lua"
printf '%s\n' 'hl.config({ input = { kb_layout = "us", kb_options = "grp:alt_shift_toggle" } })' > "$last/hyprland/input.lua"
printf '%s\n' '-- canonical keybinds' > "$last/hyprland/keybinds.lua"
printf '%s\n' '#!/usr/bin/env bash' > "$last/scripts/start-shell.sh"`)
	writeScript(t, filepath.Join(bin, "chmod"), "exit 0")
	writeScript(t, filepath.Join(bin, "find"), "exit 27")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	var out bytes.Buffer
	runner := run.Runner{Stdout: &out, Stderr: &out}
	err := syncHypr(context.Background(), runner, dotsSrc, configDir, config.Default())
	if err == nil {
		t.Fatal("expected scripts executable command error")
	}
	if !strings.Contains(err.Error(), "find failed") {
		t.Fatalf("expected find failure, got %v", err)
	}
	for _, path := range []string{shellruntime.ActiveShellStatePath(home), shellruntime.End4VariantStatePath(home)} {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("failed hypr apply published shell selection state at %s: %v", path, statErr)
		}
	}
}

func TestSyncHyprDoesNotReloadLiveHyprland(t *testing.T) {
	dotsSrc := t.TempDir()
	writeRequiredHyprSource(t, dotsSrc)

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := syncHypr(context.Background(), runner, dotsSrc, t.TempDir(), config.Default()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "hyprctl reload") {
		t.Fatalf("hypr sync must not reload the live session before nixos-rebuild switch:\n%s", out.String())
	}
}

func TestSyncHyprExcludesCanonicalAndLegacyUserDirectories(t *testing.T) {
	dotsSrc := t.TempDir()
	writeRequiredHyprSource(t, dotsSrc)

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := syncHypr(context.Background(), runner, dotsSrc, t.TempDir(), config.Default()); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, required := range []string{"--exclude /user/", "--exclude /wahrwelt/", "--exclude /mysetup/"} {
		if !strings.Contains(text, required) {
			t.Fatalf("hypr sync must preserve user and legacy user directories, missing %q:\n%s", required, text)
		}
	}
	for _, forbidden := range []string{"--exclude /wahrwelt/local.lua", "--exclude /wahrwelt/hyprland.lua"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("hypr sync retained narrow user-directory exclusion %q:\n%s", forbidden, text)
		}
	}
}

func TestSyncHyprDefersCurrentHomeManagerTreeToActivation(t *testing.T) {
	for preset, want := range map[string]bool{"minimal": false, "desktop": true, "developer": true, "personal": true} {
		if got := homeManagerOwnsHyprStaticTreeForPreset(preset); got != want {
			t.Fatalf("Home Manager ownership for preset %q = %t, want %t", preset, got, want)
		}
	}

	dotsSrc := t.TempDir()
	writeRequiredHyprSource(t, dotsSrc)
	home := t.TempDir()
	hyprDir, _ := prepareHomeManagerHyprProof(t, home, knownHomeManagerHyprEntrypoint(home))
	paths := append([]string{hyprDir, filepath.Join(hyprDir, "hyprland.lua")}, addHomeManagerHyprStaticLinks(t, hyprDir)...)
	userDefault := filepath.Join(hyprDir, "user", "default.lua")
	writeModeTestFile(t, userDefault, "-- private user default\n", 0o600)
	paths = append(paths, userDefault)
	for _, path := range append(
		[]string{shellruntime.ActiveShellStatePath(home), shellruntime.End4VariantStatePath(home)},
		runtimeStatePaths(home)...,
	) {
		writeModeTestFile(t, path, "pre-switch state\n", 0o600)
		paths = append(paths, path)
	}
	before := snapshotHyprPaths(t, paths)

	state := config.Default()
	state.Packages.Preset = "personal"
	var out bytes.Buffer
	if err := syncHypr(context.Background(), run.Runner{Stdout: &out, Stderr: &out}, dotsSrc, filepath.Join(home, ".config"), state); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("pre-switch Home Manager tree ran commands:\n%s", out.String())
	}
	if _, err := os.Lstat(filepath.Join(hyprDir, managedMarkerName)); !os.IsNotExist(err) {
		t.Fatalf("pre-switch Home Manager tree published a marker: %v", err)
	}
	if backups, _ := filepath.Glob(hyprDir + ".bak.*"); len(backups) != 0 {
		t.Fatalf("pre-switch Home Manager tree created backups: %v", backups)
	}
	assertHyprPathsUnchanged(t, before)
}

func TestSyncHyprFailsClosedWhenCurrentHomeManagerProofIsIncomplete(t *testing.T) {
	for _, test := range []struct {
		name    string
		breakIt func(t *testing.T, home, currentHome string)
		wantErr string
	}{
		{"missing current-home", func(t *testing.T, _, currentHome string) { t.Helper(); mustRemove(t, currentHome) }, "current Home Manager generation is unavailable"},
		{"mismatched current-home", replaceWithMismatchedCurrentHome, "does not belong to the current Home Manager generation"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dotsSrc := t.TempDir()
			writeRequiredHyprSource(t, dotsSrc)
			home := t.TempDir()
			hyprDir, currentHome := prepareHomeManagerHyprProof(t, home, knownHomeManagerHyprEntrypoint(home))
			before := snapshotHyprPaths(t, []string{hyprDir, filepath.Join(hyprDir, "hyprland.lua")})
			test.breakIt(t, home, currentHome)
			state := config.Default()
			state.Packages.Preset = "personal"
			var out bytes.Buffer
			err := syncHypr(context.Background(), run.Runner{Stdout: &out, Stderr: &out}, dotsSrc, filepath.Join(home, ".config"), state)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
			if out.Len() != 0 {
				t.Fatalf("failed ownership proof ran commands:\n%s", out.String())
			}
			if backups, _ := filepath.Glob(hyprDir + ".bak.*"); len(backups) != 0 {
				t.Fatalf("failed ownership proof created backups: %v", backups)
			}
			if _, markerErr := os.Lstat(filepath.Join(hyprDir, managedMarkerName)); !os.IsNotExist(markerErr) {
				t.Fatalf("failed ownership proof published marker: %v", markerErr)
			}
			assertHyprPathsUnchanged(t, before)
		})
	}
}

func TestSyncHyprMirrorsTreesNotOwnedByCurrentHomeManager(t *testing.T) {
	for _, test := range []struct {
		name, preset, entrypoint string
		homeManager              bool
	}{
		{"minimal preset", "minimal", "", true},
		{"unknown Home Manager payload", "personal", "-- unrelated Home Manager entrypoint\n", true},
		{"regular legacy payload", "personal", migrationv1tov2.LegacyUserEntrypoint(), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			dotsSrc := t.TempDir()
			writeRequiredHyprSource(t, dotsSrc)
			writeModeTestFile(t, filepath.Join(dotsSrc, "hypr", "mirror-canary.lua"), "-- current mirror\n", 0o644)
			home := t.TempDir()
			hyprDir := filepath.Join(home, ".config", "hypr")
			entrypoint := test.entrypoint
			if entrypoint == "" {
				entrypoint = knownHomeManagerHyprEntrypoint(home)
			}
			if test.homeManager {
				hyprDir, _ = prepareHomeManagerHyprProof(t, home, entrypoint)
			} else {
				writeModeTestFile(t, filepath.Join(hyprDir, "hyprland.lua"), entrypoint, 0o644)
			}
			writeModeTestFile(t, filepath.Join(hyprDir, "mirror-canary.lua"), "-- previous tree\n", 0o644)
			state := config.Default()
			state.Packages.Preset = test.preset
			var out bytes.Buffer
			if err := syncHypr(context.Background(), run.Runner{Stdout: &out, Stderr: &out}, dotsSrc, filepath.Join(home, ".config"), state); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), "$ rsync ") || readTestFile(t, filepath.Join(hyprDir, "mirror-canary.lua")) != "-- current mirror\n" {
				t.Fatalf("%s did not use normal mirror path:\n%s", test.name, out.String())
			}
			if backups, _ := filepath.Glob(hyprDir + ".bak.*"); len(backups) != 1 {
				t.Fatalf("%s backups = %v, want one", test.name, backups)
			}
		})
	}
}

func TestSyncHyprRejectsSourceWithoutVMKeybindsBeforeSync(t *testing.T) {
	dotsSrc := t.TempDir()
	writeRequiredHyprSource(t, dotsSrc)
	missing := filepath.Join(dotsSrc, "hypr", "vm-keybinds.lua")
	mustRemove(t, missing)
	var out bytes.Buffer
	err := syncHypr(context.Background(), run.Runner{Stdout: &out, Stderr: &out}, dotsSrc, filepath.Join(t.TempDir(), ".config"), config.Default())
	if err == nil || !strings.Contains(err.Error(), "required hypr source file missing: "+missing) || out.Len() != 0 {
		t.Fatalf("invalid source result: err=%v output=%q", err, out.String())
	}
}

func TestSyncHyprRejectsUnknownEnd4DirectoryForNonEnd4Profile(t *testing.T) {
	dotsSrc := t.TempDir()
	writeRequiredHyprSource(t, dotsSrc)
	configDir := filepath.Join(t.TempDir(), ".config")
	hyprDir := filepath.Join(configDir, "hypr")
	staleEnd4Dir := filepath.Join(hyprDir, "end4")
	if err := os.MkdirAll(staleEnd4Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staleEnd4Dir, "launcher.lua"), []byte("-- stale lite profile\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeMarker(run.Runner{}, filepath.Join(hyprDir, managedMarkerName), "hypr"); err != nil {
		t.Fatal(err)
	}
	writeHomeManagerEnd4Source(t, configDir, "ii")

	state := config.Default()
	state.User.Username = "tester"
	var out bytes.Buffer
	runner := run.Runner{Stdout: &out, Stderr: &out}
	err := syncHypr(context.Background(), runner, dotsSrc, configDir, state)
	if err == nil || !strings.Contains(err.Error(), "unowned End4 profile collision") {
		t.Fatalf("expected unknown End4 ownership collision, got %v", err)
	}
	if strings.Contains(out.String(), "rm -rf -- "+staleEnd4Dir) {
		t.Fatalf("unknown End4 directory must not be removed:\n%s", out.String())
	}
	if got := readTestFile(t, filepath.Join(staleEnd4Dir, "launcher.lua")); got != "-- stale lite profile\n" {
		t.Fatalf("unknown End4 contents changed: %q", got)
	}
}

func TestSyncHyprRejectsUnknownEnd4DirectoryWhenEnd4IsActive(t *testing.T) {
	dotsSrc := t.TempDir()
	writeRequiredHyprSource(t, dotsSrc)
	configDir := filepath.Join(t.TempDir(), ".config")
	hyprDir := filepath.Join(configDir, "hypr")
	staleEnd4Dir := filepath.Join(hyprDir, "end4")
	if err := os.MkdirAll(staleEnd4Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staleEnd4Dir, "launcher.lua"), []byte("-- stale lite profile\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeMarker(run.Runner{}, filepath.Join(hyprDir, managedMarkerName), "hypr"); err != nil {
		t.Fatal(err)
	}
	writeHomeManagerEnd4Source(t, configDir, "ii")
	bin := t.TempDir()
	writeScript(t, filepath.Join(bin, "rsync"), "exit 0")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	home := homeDirFromConfigDir(configDir)
	if err := os.MkdirAll(filepath.Dir(paths.ActiveShellStatePath(home)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.ActiveShellStatePath(home), []byte("end4\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	state := config.Default()
	state.User.Username = "tester"
	var out bytes.Buffer
	runner := run.Runner{Stdout: &out, Stderr: &out}
	err := syncHypr(context.Background(), runner, dotsSrc, configDir, state)
	if err == nil || !strings.Contains(err.Error(), "unowned End4 profile collision") {
		t.Fatalf("expected unknown active End4 ownership collision, got %v", err)
	}
	if strings.Contains(out.String(), "rm -rf -- "+staleEnd4Dir) {
		t.Fatalf("unknown active End4 directory must not be removed:\n%s", out.String())
	}
	if got := readTestFile(t, filepath.Join(staleEnd4Dir, "launcher.lua")); got != "-- stale lite profile\n" {
		t.Fatalf("unknown active End4 contents changed: %q", got)
	}
}

func TestSyncHyprAcceptsExactHomeManagerEnd4SourceWhenPCVariantIsActive(t *testing.T) {
	dotsSrc := t.TempDir()
	writeRequiredHyprSource(t, dotsSrc)
	configDir := filepath.Join(t.TempDir(), ".config")
	hyprDir := filepath.Join(configDir, "hypr")
	end4Dir := filepath.Join(hyprDir, "end4")
	end4Source := writeHomeManagerEnd4Source(t, configDir, "end4-pC")
	if err := os.MkdirAll(filepath.Dir(end4Dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(end4Source, end4Dir); err != nil {
		t.Fatal(err)
	}

	home := homeDirFromConfigDir(configDir)
	if err := os.MkdirAll(filepath.Dir(paths.ActiveShellStatePath(home)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.ActiveShellStatePath(home), []byte("end4-pc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	state := config.Default()
	state.User.Username = "tester"
	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := syncHypr(context.Background(), runner, dotsSrc, configDir, state); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "rm -rf -- "+end4Dir) {
		t.Fatalf("active end4-pc profile must not be pruned during hypr sync:\n%s", out.String())
	}
	info, err := os.Lstat(end4Dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("exact Home Manager End4 target was replaced: mode=%s", info.Mode())
	}
}

func TestRestoreActiveEnd4RejectsUnknownFileAndSymlinkWithoutMutation(t *testing.T) {
	for _, kind := range []string{"file", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			configDir := filepath.Join(t.TempDir(), ".config")
			hyprDir := filepath.Join(configDir, "hypr")
			target := filepath.Join(hyprDir, "end4")
			writeHomeManagerEnd4Source(t, configDir, "ii")
			if err := os.MkdirAll(hyprDir, 0o755); err != nil {
				t.Fatal(err)
			}

			var unknownSource string
			switch kind {
			case "file":
				if err := os.WriteFile(target, []byte("unknown file\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				unknownSource = t.TempDir()
				if err := os.Symlink(unknownSource, target); err != nil {
					t.Fatal(err)
				}
			}

			runner := run.Runner{DryRun: true}
			err := restoreActiveEnd4Profile(context.Background(), runner, configDir, hyprDir, shellruntime.End4)
			if err == nil || !strings.Contains(err.Error(), "unowned End4 profile collision") {
				t.Fatalf("expected unknown %s collision, got %v", kind, err)
			}
			if kind == "file" {
				if got := readTestFile(t, target); got != "unknown file\n" {
					t.Fatalf("unknown file changed: %q", got)
				}
			} else if got, err := os.Readlink(target); err != nil || got != unknownSource {
				t.Fatalf("unknown symlink changed: target=%q err=%v", got, err)
			}
		})
	}
}

func TestValidateEnd4TargetOwnershipRequiresTargetSymlink(t *testing.T) {
	target := filepath.Join(t.TempDir(), "end4")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "hyprland.lua"), []byte("-- ordinary collision\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := validateEnd4TargetOwnership(target, []string{target})
	if err == nil || !strings.Contains(err.Error(), "target is not a symlink") {
		t.Fatalf("ordinary directory self-identification was accepted: %v", err)
	}
	if got := readTestFile(t, filepath.Join(target, "hyprland.lua")); got != "-- ordinary collision\n" {
		t.Fatalf("ordinary collision changed: %q", got)
	}
}

func TestValidateEnd4TargetOwnershipRejectsBrokenAndWrongLinks(t *testing.T) {
	proven := filepath.Join(t.TempDir(), "proven")
	if err := os.MkdirAll(proven, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"broken", "wrong"} {
		t.Run(kind, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "end4")
			linkTarget := filepath.Join(t.TempDir(), "missing")
			if kind == "wrong" {
				if err := os.MkdirAll(linkTarget, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(linkTarget, target); err != nil {
				t.Fatal(err)
			}
			err := validateEnd4TargetOwnership(target, []string{proven})
			if err == nil {
				t.Fatalf("%s End4 link was accepted", kind)
			}
			got, readErr := os.Readlink(target)
			if readErr != nil || got != linkTarget {
				t.Fatalf("%s link changed: got %q err=%v", kind, got, readErr)
			}
		})
	}
}

func TestSyncHyprFailsWhenRequiredSourceMissing(t *testing.T) {
	dotsSrc := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dotsSrc, "hypr"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := syncHypr(context.Background(), run.Runner{DryRun: true}, dotsSrc, t.TempDir(), config.Default())
	if err == nil {
		t.Fatal("expected missing required source error")
	}
	if !strings.Contains(err.Error(), "required hypr source file missing") {
		t.Fatalf("expected required source error, got %v", err)
	}
}

func TestWriteHyprLocalConfigRepairsTreeOnPermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("permission mode test needs non-root Unix user")
	}
	hyprDir := t.TempDir()
	hyprSourceDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(hyprDir, "hyprland"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hyprSourceDir, "hyprland.lua"), []byte("-- old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hyprDir, "hyprland", "input.lua"), []byte("hl.config({ input = { kb_layout = \"old\", kb_options = \"old\" } })\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(hyprDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(hyprDir, 0o700)
	})

	bin := t.TempDir()
	writeScript(t, filepath.Join(bin, "sudo"), `case "$1" in
chown) exit 0 ;;
chmod) shift; /bin/chmod "$@"; exit $? ;;
*) exit 1 ;;
esac`)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	var out bytes.Buffer
	runner := run.Runner{Stdout: &out, Stderr: &out}
	if err := writeHyprLocalConfig(context.Background(), runner, "tester", hyprSourceDir, hyprDir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(hyprDir, "user", "default.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != wahrweltDefaultLua {
		t.Fatalf("expected canonical wahrwelt default template, got:\n%s", data)
	}
	for _, want := range []string{
		"sudo chown -R tester:",
		"chmod -R u+rwX",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("expected repair command %q, got:\n%s", want, out.String())
		}
	}
}

func writeRequiredHyprSource(t *testing.T, dotsSrc string) {
	t.Helper()

	files := map[string]string{
		"hypr/end4-adapter.lua":             "return {}\n",
		"hypr/hyprland.lua":                 "require(\"hyprland.keybinds\")\nwahrwelt.load_runtime(\"shell-profile.lua\")\nwahrwelt.load_runtime(\"shell-launcher.lua\")\nwahrwelt.load_runtime(\"shell-keybinds.lua\")\n",
		"hypr/hyprland/input.lua":           "hl.config({ input = { kb_layout = \"us\", kb_options = \"grp:alt_shift_toggle\" } })\n",
		"hypr/hyprland/keybinds.lua":        "-- canonical keybinds\n",
		"hypr/shell-common-keybinds.lua":    "wahrwelt.bind_exec(\"SUPER + SHIFT + W\", wahrwelt.hypr .. \"/scripts/shell-selector.sh toggle\")\n",
		"hypr/shell-common-rules.lua":       "hl.window_rule({ match = { class = \"spotify\" }, workspace = \"special:music\" })\n",
		"hypr/shell-workspace-keybinds.lua": "wahrwelt.bind_exec(\"SUPER + 1\", wahrwelt.hypr .. \"/scripts/wsaction.fish -g workspace 1\")\n",
		"hypr/vm-keybinds.lua":              "-- canonical VM keybinds\n",
		"hypr/lib/wahrwelt.lua":             "return {}\n",
		"hypr/variables.lua":                "return {}\n",
		"hypr/scheme/default.lua":           "return {}\n",
	}
	for _, profile := range shellruntime.ProfileSpecs {
		files[filepath.Join("hypr", profile.Launcher)] = "# launcher profile\n"
		files[filepath.Join("hypr", profile.Keybinds)] = "# keybind profile\n"
	}
	for _, script := range shellruntime.HyprScripts {
		files[filepath.Join("hypr", "scripts", script)] = "#!/usr/bin/env bash\n"
	}
	for rel, content := range files {
		path := filepath.Join(dotsSrc, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

type hyprPathSnapshot struct {
	path, target, content string
	info                  os.FileInfo
}

func knownHomeManagerHyprEntrypoint(home string) string {
	return stableRuntimeSourceConfig(shellruntime.RuntimeFile(home, "hyprland.lua"), "Wahrwelt stable Hyprland entrypoint.")
}

func prepareHomeManagerHyprProof(t *testing.T, home, content string) (hyprDir, currentHome string) {
	t.Helper()
	hyprDir = filepath.Join(home, ".config", "hypr")
	if err := os.MkdirAll(hyprDir, 0o755); err != nil {
		t.Fatal(err)
	}
	storeRoot, target := addHomeManagerFilesStoreLeaf(t, "hyprland.lua", content)
	if err := os.Symlink(target, filepath.Join(hyprDir, "hyprland.lua")); err != nil {
		t.Fatal(err)
	}
	generation := filepath.Join(t.TempDir(), "generation")
	if err := os.MkdirAll(generation, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(storeRoot, filepath.Join(generation, "home-files")); err != nil {
		t.Fatal(err)
	}
	currentHome = filepath.Join(home, ".local", "state", "home-manager", "gcroots", "current-home")
	if err := os.MkdirAll(filepath.Dir(currentHome), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(generation, currentHome); err != nil {
		t.Fatal(err)
	}
	return hyprDir, currentHome
}

func addHomeManagerHyprStaticLinks(t *testing.T, hyprDir string) []string {
	t.Helper()
	files := map[string]string{
		"vm-keybinds.lua":        "-- vm\n",
		"hyprland/input.lua":     "-- input\n",
		"scripts/start-shell.sh": "#!/usr/bin/env bash\n",
		"user/hyprland.lua":      "-- user adapter\n",
		"end4/hyprland.lua":      "-- end4\n",
	}
	tree := t.TempDir()
	for relative, content := range files {
		writeModeTestFile(t, filepath.Join(tree, ".config", "hypr", filepath.FromSlash(relative)), content, 0o444)
	}
	storeRoot := addHomeManagerFilesNixStorePath(t, tree)
	paths := make([]string, 0, 4)
	for _, relative := range []string{"vm-keybinds.lua", "hyprland/input.lua", "scripts/start-shell.sh", "user/hyprland.lua"} {
		path := filepath.Join(hyprDir, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(storeRoot, ".config", "hypr", filepath.FromSlash(relative)), path); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	end4 := filepath.Join(hyprDir, "end4")
	if err := os.Symlink(filepath.Join(storeRoot, ".config", "hypr", "end4"), end4); err != nil {
		t.Fatal(err)
	}
	return append(paths, end4)
}

func runtimeStatePaths(home string) []string {
	paths := make([]string, 0, len(shellruntime.RuntimeFiles))
	for _, name := range shellruntime.RuntimeFiles {
		paths = append(paths, shellruntime.RuntimeFile(home, name))
	}
	return paths
}

func replaceWithMismatchedCurrentHome(t *testing.T, home, currentHome string) {
	t.Helper()
	storeRoot, _ := addHomeManagerFilesStoreLeaf(t, "hyprland.lua", knownHomeManagerHyprEntrypoint(home)+"-- mismatch\n")
	mustRemove(t, currentHome)
	generation := filepath.Join(t.TempDir(), "generation")
	if err := os.MkdirAll(generation, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(storeRoot, filepath.Join(generation, "home-files")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(generation, currentHome); err != nil {
		t.Fatal(err)
	}
}

func snapshotHyprPaths(t *testing.T, paths []string) []hyprPathSnapshot {
	t.Helper()
	result := make([]hyprPathSnapshot, 0, len(paths))
	for _, path := range paths {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		snapshot := hyprPathSnapshot{path: path, info: info}
		if info.Mode()&os.ModeSymlink != 0 {
			snapshot.target, err = os.Readlink(path)
		} else if info.Mode().IsRegular() {
			var data []byte
			data, err = os.ReadFile(path)
			snapshot.content = string(data)
		}
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, snapshot)
	}
	return result
}

func assertHyprPathsUnchanged(t *testing.T, snapshots []hyprPathSnapshot) {
	t.Helper()
	for _, before := range snapshots {
		after, err := os.Lstat(before.path)
		if err != nil || !os.SameFile(before.info, after) || after.Mode() != before.info.Mode() {
			t.Fatalf("path changed: %s: %v", before.path, err)
		}
		if before.target != "" {
			if target, err := os.Readlink(before.path); err != nil || target != before.target {
				t.Fatalf("symlink changed: %s: target=%q err=%v", before.path, target, err)
			}
		} else if before.info.Mode().IsRegular() && readTestFile(t, before.path) != before.content {
			t.Fatalf("file content changed: %s", before.path)
		}
	}
}

func mustRemove(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

func writeHomeManagerEnd4Source(t *testing.T, configDir, quickshellConfig string) string {
	t.Helper()
	_ = quickshellConfig
	home := filepath.Dir(configDir)
	end4Store := filepath.Join(t.TempDir(), "end4-store")
	if err := os.MkdirAll(end4Store, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(end4Store, "hyprland.lua"), []byte("-- exact HM End4 source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	generation := filepath.Join(t.TempDir(), "home-manager-generation")
	end4Source := filepath.Join(generation, "home-files", ".config", "hypr", "end4")
	if err := os.MkdirAll(filepath.Dir(end4Source), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(end4Store, end4Source); err != nil {
		t.Fatal(err)
	}
	gcroot := filepath.Join(home, ".local", "state", "home-manager", "gcroots", "current-home")
	if err := os.MkdirAll(filepath.Dir(gcroot), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(generation, gcroot); err != nil {
		t.Fatal(err)
	}
	return end4Source
}
