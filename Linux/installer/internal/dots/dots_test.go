package dots

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
	migrationv1tov2 "github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/migrations/v1_to_v2"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/shellruntime"
)

func legacyUserRuntimeEntrypoint() string {
	return migrationv1tov2.LegacyUserEntrypoint()
}

func legacyDirectEnd4RuntimeEntrypoint(profile string) string {
	return migrationv1tov2.DirectEnd4Entrypoint(profile)
}

func legacyDirectEnd4LauncherPlaceholder(profile string) string {
	return migrationv1tov2.DirectEnd4LauncherPlaceholder(profile)
}

func legacyDirectEnd4KeybindsPlaceholder(profile string) string {
	return migrationv1tov2.DirectEnd4KeybindsPlaceholder(profile)
}

func TestBootstrapActiveShellForUpgradeOwnsV1Fallbacks(t *testing.T) {
	home := t.TempDir()
	hyprDir := filepath.Join(home, ".config", "hypr")
	runtimeDir := shellruntime.RuntimeDir(home)
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(hyprDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entrypoint := shellruntime.RuntimeFile(home, "hyprland.lua")
	keybinds := shellruntime.RuntimeFile(home, "shell-keybinds.lua")
	if err := os.WriteFile(entrypoint, []byte(migrationv1tov2.DirectEnd4Entrypoint(shellruntime.End4)), 0o644); err != nil {
		t.Fatal(err)
	}
	variant := shellruntime.End4VariantStatePath(home)
	if err := os.MkdirAll(filepath.Dir(variant), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(variant, []byte(shellruntime.End4PC+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := bootstrapActiveShellForUpgrade(home, hyprDir); got != shellruntime.End4PC {
		t.Fatalf("upgrade bootstrap direct End4 variant = %q, want %q", got, shellruntime.End4PC)
	}
	if got := shellruntime.BootstrapActiveShell(home, hyprDir); got != shellruntime.DefaultProfile {
		t.Fatalf("fresh bootstrap consumed v1 direct End4 entrypoint: %q", got)
	}

	if err := os.WriteFile(entrypoint, []byte(migrationv1tov2.LegacyUserEntrypoint()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keybinds, []byte(shellruntime.AdapterMarker(shellruntime.Noctalia)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := bootstrapActiveShellForUpgrade(home, hyprDir); got != shellruntime.Noctalia {
		t.Fatalf("upgrade bootstrap legacy user profile = %q, want %q", got, shellruntime.Noctalia)
	}

	legacyState := migrationv1tov2.LegacyActiveShellStatePath(filepath.Join(home, ".local", "state"))
	if err := os.MkdirAll(filepath.Dir(legacyState), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyState, []byte(shellruntime.End4+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := bootstrapActiveShellForUpgrade(home, hyprDir); got != shellruntime.End4 {
		t.Fatalf("upgrade bootstrap legacy state = %q, want %q", got, shellruntime.End4)
	}
}

func TestSetupV2rayNSkipsWhenTargetRootMissing(t *testing.T) {
	home := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := setupV2rayN(context.Background(), run.Runner{Stdout: &stdout, Stderr: &stderr}, home)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout.String(), "command -v sing-box") {
		t.Fatalf("sing-box lookup should not run without v2rayN root, got %q", stdout.String())
	}
}

func TestDirectEnd4MigrationAssetsCoverManifestRuntimeClosure(t *testing.T) {
	if directEnd4MigrationAssetSpecsErr != nil {
		t.Fatal(directEnd4MigrationAssetSpecsErr)
	}
	assets := make(map[string]directEnd4MigrationAsset, len(directEnd4MigrationAssetSpecs))
	destinations := make(map[string]string, len(directEnd4MigrationAssetSpecs))
	for _, asset := range directEnd4MigrationAssetSpecs {
		if _, exists := assets[asset.sourceRel]; exists {
			t.Fatalf("duplicate direct End4 migration asset %q", asset.sourceRel)
		}
		if previous, exists := destinations[asset.destRel]; exists {
			t.Fatalf("direct End4 migration destination %q is shared by %q and %q", asset.destRel, previous, asset.sourceRel)
		}
		assets[asset.sourceRel] = asset
		destinations[asset.destRel] = asset.sourceRel
	}

	for _, profile := range shellruntime.ProfileSpecs {
		if profile.Family == shellruntime.End4Family {
			continue
		}
		for label, rel := range map[string]string{"launcher": profile.Launcher, "keybinds": profile.Keybinds} {
			asset, ok := assets[rel]
			if !ok {
				t.Errorf("standalone profile %q %s %q is missing from direct End4 migration assets", profile.ID, label, rel)
				continue
			}
			if asset.destRel != rel || asset.mode.Perm() != 0o644 {
				t.Errorf("standalone profile %q %s asset = %#v, want same destination and mode 0644", profile.ID, label, asset)
			}
		}
	}

	for _, script := range shellruntime.HyprScripts {
		rel := "scripts/" + script
		asset, ok := assets[rel]
		if !ok {
			t.Errorf("managed Hypr script %q is missing from direct End4 migration assets", rel)
			continue
		}
		if asset.destRel != rel {
			t.Errorf("managed Hypr script %q destination = %q, want %q", rel, asset.destRel, rel)
		}
		if asset.mode.Perm() != 0o755 {
			t.Errorf("managed Hypr script %q mode = %#o, want 0755", rel, asset.mode.Perm())
		}
	}
}

func TestBuildDirectEnd4MigrationAssetSpecsRejectsManifestCollisions(t *testing.T) {
	tests := []struct {
		name     string
		profiles []shellruntime.Profile
		scripts  []string
		want     string
	}{
		{
			name: "duplicate profile module",
			profiles: []shellruntime.Profile{
				{ID: "one", Family: "one", Launcher: "shared/launcher.lua", Keybinds: "one/keybinds.lua"},
				{ID: "two", Family: "two", Launcher: "shared/launcher.lua", Keybinds: "two/keybinds.lua"},
			},
			want: "duplicate direct End4 migration asset source",
		},
		{name: "duplicate script", scripts: []string{"same.sh", "same.sh"}, want: "duplicate direct End4 migration asset source"},
		{
			name: "profile collides with canonical base",
			profiles: []shellruntime.Profile{
				{ID: "collision", Family: "collision", Launcher: "variables.lua", Keybinds: "collision/keybinds.lua"},
			},
			want: "duplicate direct End4 migration asset source",
		},
		{name: "unsafe script", scripts: []string{"../escape.sh"}, want: "unsafe managed Hypr script path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := buildDirectEnd4MigrationAssetSpecs(tt.profiles, tt.scripts); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("buildDirectEnd4MigrationAssetSpecs() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestSetupV2rayNSeedsSingBoxWhenTargetRootExists(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".local", "share", "v2rayN")
	bin := t.TempDir()
	singbox := filepath.Join(bin, "sing-box")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(singbox, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := setupV2rayN(context.Background(), run.Runner{Stdout: &stdout, Stderr: &stderr}, home)
	if err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(root, "bin", "sing_box", "sing-box")
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("seeded sing-box mode = %v, want 0755", info.Mode().Perm())
	}
}

func TestRefreshThumbnailDaemonsRestartsHelpers(t *testing.T) {
	var stdout, stderr bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &stdout, Stderr: &stderr}

	refreshThumbnailDaemons(context.Background(), runner, "alice", "/home/alice")

	text := stdout.String()
	for _, want := range []string{
		"pkill -u alice -x gvfsd",
		"pkill -u alice -x gvfsd-fuse",
		"pkill -u alice -x Thunar",
		"pkill -u alice -x thunar",
		"pkill -u alice -f gvfs-udisks2-volume-monitor",
		"pkill -u alice -f tumbler-1/tumblerd",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("refreshThumbnailDaemons missing %q in command log:\n%s", want, text)
		}
	}
	if strings.Contains(text, ".cache/thumbnails") {
		t.Fatalf("thumbnail cache must not be wiped; got:\n%s", text)
	}
}

func TestRefreshThumbnailDaemonsSkipsWithoutUsername(t *testing.T) {
	var stdout bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &stdout}

	refreshThumbnailDaemons(context.Background(), runner, "", "/home/alice")

	if stdout.Len() != 0 {
		t.Fatalf("no commands expected without username, got %q", stdout.String())
	}
}

func TestApplyFinalizesCanonicalHyprRuntimeWhenHyprSyncIsDisabled(t *testing.T) {
	home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
	state := config.Default()
	state.User.HomeDirectory = home
	state.User.Username = ""
	state.Dots = config.Dots{}

	if err := Apply(context.Background(), Options{State: state, Runner: run.New(false)}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(hyprDir, "wahrwelt")); !os.IsNotExist(err) {
		t.Fatalf("legacy Hypr user directory remains after apply: %v", err)
	}
	if info, err := os.Lstat(filepath.Join(hyprDir, "user")); err != nil || !info.IsDir() {
		t.Fatalf("canonical Hypr user directory missing after apply: info=%v err=%v", info, err)
	}
	if got := readTestFile(t, runtimePath); got != shellruntime.CanonicalEntrypoint() {
		t.Fatalf("runtime was not finalized independently of Dots.Hypr:\n%s", got)
	}
}

func TestApplyKeepsTransitionRuntimeLoadableWhenMigrationFailsBeforeRename(t *testing.T) {
	home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
	state := config.Default()
	state.User.HomeDirectory = home
	state.User.Username = ""
	state.Dots = config.Dots{}
	injected := errors.New("injected migration failure")

	err := applyWithHooks(context.Background(), Options{State: state, Runner: run.New(false)}, applyHooks{
		migration: legacyUserMigrationHooks{
			hypr: func(stage hyprUserMigrationCommitStage, _ hyprUserMigration) error {
				if stage != hyprUserMigrationBeforeRename {
					t.Fatalf("unexpected migration stage %q", stage)
				}
				if got := readTestFile(t, runtimePath); got != migrationv1tov2.UserNamespaceTransitionEntrypoint() {
					t.Fatalf("transition was not published before migration commit:\n%s", got)
				}
				return injected
			},
		},
	})
	if !errors.Is(err, injected) {
		t.Fatalf("apply error = %v, want injected migration failure", err)
	}
	if info, statErr := os.Lstat(filepath.Join(hyprDir, "wahrwelt")); statErr != nil || !info.IsDir() {
		t.Fatalf("legacy adapter disappeared after failed migration: info=%v err=%v", info, statErr)
	}
	if _, statErr := os.Lstat(filepath.Join(hyprDir, "user")); !os.IsNotExist(statErr) {
		t.Fatalf("canonical adapter appeared after failed migration: %v", statErr)
	}
	if got := readTestFile(t, runtimePath); got != migrationv1tov2.UserNamespaceTransitionEntrypoint() {
		t.Fatalf("failed migration did not retain its loadable transition runtime:\n%s", got)
	}
}

func TestApplyRollsFinalizationBackToLoadableTransitionOnPublicationFailure(t *testing.T) {
	home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
	state := config.Default()
	state.User.HomeDirectory = home
	state.User.Username = ""
	state.Dots = config.Dots{}
	injected := errors.New("injected final publication failure")

	err := applyWithHooks(context.Background(), Options{State: state, Runner: run.New(false)}, applyHooks{
		finalRuntime: func(operation, path string) error {
			if operation == runtimeMutationWrite && path == runtimePath {
				return injected
			}
			return nil
		},
	})
	if !errors.Is(err, injected) {
		t.Fatalf("apply error = %v, want injected final publication failure", err)
	}
	if _, statErr := os.Lstat(filepath.Join(hyprDir, "wahrwelt")); !os.IsNotExist(statErr) {
		t.Fatalf("legacy namespace returned after successful migration: %v", statErr)
	}
	if info, statErr := os.Lstat(filepath.Join(hyprDir, "user")); statErr != nil || !info.IsDir() {
		t.Fatalf("canonical adapter missing after final publication failure: info=%v err=%v", info, statErr)
	}
	if got := readTestFile(t, runtimePath); got != migrationv1tov2.UserNamespaceTransitionEntrypoint() {
		t.Fatalf("failed finalization did not roll back to the loadable transition:\n%s", got)
	}
}

func TestApplyDoesNotPublishTransitionBeforeCoexistencePreflight(t *testing.T) {
	home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
	if err := os.MkdirAll(filepath.Join(hyprDir, "user"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := config.Default()
	state.User.HomeDirectory = home
	state.User.Username = ""
	state.Dots = config.Dots{}

	err := Apply(context.Background(), Options{State: state, Runner: run.New(false)})
	if err == nil || !strings.Contains(err.Error(), "legacy and canonical Hypr user config directories coexist") {
		t.Fatalf("apply error = %v, want coexistence preflight failure", err)
	}
	if got := readTestFile(t, runtimePath); got != legacyUserRuntimeEntrypoint() {
		t.Fatalf("runtime changed before coexistence preflight completed:\n%s", got)
	}
	for _, namespace := range []string{"user", "wahrwelt"} {
		if info, statErr := os.Lstat(filepath.Join(hyprDir, namespace)); statErr != nil || !info.IsDir() {
			t.Fatalf("%s namespace changed after preflight failure: info=%v err=%v", namespace, info, statErr)
		}
	}
}

func TestApplyRejectsUnknownStateRuntimeBeforeHyprNamespaceMove(t *testing.T) {
	home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
	unknown := "-- private runtime\ndofile(hypr_root .. \"/wahrwelt/hyprland.lua\")\n"
	if err := os.WriteFile(runtimePath, []byte(unknown), 0o600); err != nil {
		t.Fatal(err)
	}
	state := config.Default()
	state.User.HomeDirectory = home
	state.User.Username = ""
	state.Dots = config.Dots{}

	err := Apply(context.Background(), Options{State: state, Runner: run.New(false)})
	if err == nil || !strings.Contains(err.Error(), "unowned Hypr state runtime collision") {
		t.Fatalf("apply error = %v, want unknown state runtime collision", err)
	}
	if got := readTestFile(t, runtimePath); got != unknown {
		t.Fatalf("unknown state runtime changed:\n%s", got)
	}
	if info, statErr := os.Lstat(filepath.Join(hyprDir, "wahrwelt")); statErr != nil || !info.IsDir() {
		t.Fatalf("legacy namespace moved despite state runtime collision: info=%v err=%v", info, statErr)
	}
	if _, statErr := os.Lstat(filepath.Join(hyprDir, "user")); !os.IsNotExist(statErr) {
		t.Fatalf("canonical namespace appeared despite state runtime collision: %v", statErr)
	}
}

func TestApplyRejectsUnknownTopLevelRuntimeBeforeHyprNamespaceMove(t *testing.T) {
	home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
	topLevel := filepath.Join(hyprDir, "hyprland.lua")
	unknown := "-- private top-level runtime\n"
	if err := os.WriteFile(topLevel, []byte(unknown), 0o600); err != nil {
		t.Fatal(err)
	}
	state := config.Default()
	state.User.HomeDirectory = home
	state.User.Username = ""
	state.Dots = config.Dots{}

	err := Apply(context.Background(), Options{State: state, Runner: run.New(false)})
	if err == nil || !strings.Contains(err.Error(), "unowned top-level Hypr runtime collision") {
		t.Fatalf("apply error = %v, want unknown top-level runtime collision", err)
	}
	if got := readTestFile(t, topLevel); got != unknown {
		t.Fatalf("unknown top-level runtime changed: %q", got)
	}
	if got := readTestFile(t, runtimePath); got != legacyUserRuntimeEntrypoint() {
		t.Fatalf("state runtime changed before top-level collision was reported:\n%s", got)
	}
	if info, statErr := os.Lstat(filepath.Join(hyprDir, "wahrwelt")); statErr != nil || !info.IsDir() {
		t.Fatalf("legacy namespace moved despite top-level collision: info=%v err=%v", info, statErr)
	}
}

func TestApplyMigratesHistoricalHomeManagerUserRuntimes(t *testing.T) {
	for _, namespace := range []string{"wahrwelt", "user"} {
		t.Run(namespace, func(t *testing.T) {
			home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
			if namespace == "user" {
				if err := os.Rename(filepath.Join(hyprDir, "wahrwelt"), filepath.Join(hyprDir, "user")); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(runtimePath, []byte(historicalHomeManagerUserRuntime(namespace)), 0o644); err != nil {
				t.Fatal(err)
			}
			state := config.Default()
			state.User.HomeDirectory = home
			state.User.Username = ""
			state.Dots = config.Dots{}

			if err := Apply(context.Background(), Options{State: state, Runner: run.New(false)}); err != nil {
				t.Fatal(err)
			}
			if got := readTestFile(t, runtimePath); got != shellruntime.CanonicalEntrypoint() {
				t.Fatalf("historical %s runtime was not finalized:\n%s", namespace, got)
			}
		})
	}
}

func TestApplyMigratesExactDirectEnd4RuntimeAsCoherentBundleWhenHyprSyncIsDisabled(t *testing.T) {
	cases := []struct {
		name        string
		mainProfile string
		remembered  string
		wantProfile string
		locations   []string
	}{
		{name: "official-exact", mainProfile: shellruntime.End4, remembered: shellruntime.End4 + "\n", wantProfile: shellruntime.End4, locations: []string{"state", "top-level"}},
		{name: "pc-exact", mainProfile: shellruntime.End4PC, remembered: shellruntime.End4PC + "\n", wantProfile: shellruntime.End4PC, locations: []string{"state", "top-level"}},
		{name: "pc-main-missing-state", mainProfile: shellruntime.End4PC, wantProfile: shellruntime.End4, locations: []string{"state"}},
		{name: "pc-main-invalid-state", mainProfile: shellruntime.End4PC, remembered: "end4-pc \n", wantProfile: shellruntime.End4, locations: []string{"state"}},
	}
	for _, tc := range cases {
		for _, location := range tc.locations {
			t.Run(tc.name+"/"+location, func(t *testing.T) {
				home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
				if location == "top-level" {
					if err := os.Remove(runtimePath); err != nil {
						t.Fatal(err)
					}
					runtimePath = filepath.Join(hyprDir, "hyprland.lua")
				}
				if err := os.WriteFile(runtimePath, []byte(legacyDirectEnd4RuntimeEntrypoint(tc.mainProfile)), 0o644); err != nil {
					t.Fatal(err)
				}
				profile, ok := shellruntime.ProfileByID(tc.wantProfile)
				if !ok {
					t.Fatalf("missing profile %s", tc.wantProfile)
				}
				writeDirectEnd4MigrationAssets(t, home, hyprDir, profile)
				writeTestFile(t, shellruntime.RuntimeFile(home, "shell-launcher.lua"), legacyDirectEnd4LauncherPlaceholder(tc.mainProfile))
				writeTestFile(t, shellruntime.RuntimeFile(home, "shell-keybinds.lua"), legacyDirectEnd4KeybindsPlaceholder(tc.mainProfile))

				variantPath := shellruntime.End4VariantStatePath(home)
				if tc.remembered != "" {
					writeTestFile(t, variantPath, tc.remembered)
				}
				opposite := shellruntime.End4
				if tc.wantProfile == shellruntime.End4 {
					opposite = shellruntime.End4PC
				}
				writeTestFile(t, shellruntime.ActiveShellStatePath(home), opposite+"\n")
				state := config.Default()
				state.User.HomeDirectory = home
				state.User.Username = ""
				state.Dots = config.Dots{}

				if err := Apply(context.Background(), Options{Sources: paths.Sources{Dots: "../../../dots"}, State: state, Runner: run.New(false)}); err != nil {
					t.Fatal(err)
				}
				stateRuntime := shellruntime.RuntimeFile(home, "hyprland.lua")
				if got := readTestFile(t, stateRuntime); got != shellruntime.CanonicalEntrypoint() {
					t.Fatalf("state runtime was not canonicalized:\n%s", got)
				}
				if got := readTestFile(t, shellruntime.RuntimeFile(home, "shell-keybinds.lua")); got != shellKeybindsConfig(hyprDir, profile) {
					t.Fatalf("End4 adapter was not published coherently:\n%s", got)
				}
				if got := readTestFile(t, shellruntime.RuntimeFile(home, "shell-launcher.lua")); got != shellLauncherBindingsConfig(profile) {
					t.Fatalf("End4 launcher was not published coherently:\n%s", got)
				}
				if tc.remembered == "" {
					if _, err := os.Lstat(variantPath); !os.IsNotExist(err) {
						t.Fatalf("missing remembered End4 variant was created: %v", err)
					}
				} else if got := readTestFile(t, variantPath); got != tc.remembered {
					t.Fatalf("remembered End4 variant changed: %q", got)
				}
				if got := readTestFile(t, shellruntime.ActiveShellStatePath(home)); got != opposite+"\n" {
					t.Fatalf("active shell priority unexpectedly selected the migration profile: %q", got)
				}
			})
		}
	}
}

func TestApplyRepublishesDirectEnd4ExecutableModeBeforeCanonicalMain(t *testing.T) {
	home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
	writeTestFile(t, runtimePath, legacyDirectEnd4RuntimeEntrypoint(shellruntime.End4))
	profile, _ := shellruntime.ProfileByID(shellruntime.End4)
	writeDirectEnd4MigrationAssets(t, home, hyprDir, profile)
	writeTestFile(t, shellruntime.RuntimeFile(home, "shell-launcher.lua"), legacyDirectEnd4LauncherPlaceholder(shellruntime.End4))
	writeTestFile(t, shellruntime.RuntimeFile(home, "shell-keybinds.lua"), legacyDirectEnd4KeybindsPlaceholder(shellruntime.End4))
	startShellPath := filepath.Join(hyprDir, "scripts", "start-shell.sh")
	writeTestFile(t, startShellPath, readTestFile(t, "../../../dots/hypr/scripts/start-shell.sh"))
	if info, err := os.Stat(startShellPath); err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("fixture start-shell mode = %v, info=%v, want 0644", err, info)
	}

	state := config.Default()
	state.User.HomeDirectory = home
	state.User.Username = ""
	state.Dots = config.Dots{}
	modeAtMain := os.FileMode(0)
	err := applyWithHooks(context.Background(), Options{Sources: paths.Sources{Dots: "../../../dots"}, State: state, Runner: run.New(false)}, applyHooks{
		finalRuntime: func(operation, path string) error {
			if operation != runtimeMutationWrite || path != runtimePath {
				return nil
			}
			info, err := os.Stat(startShellPath)
			if err != nil {
				return err
			}
			modeAtMain = info.Mode().Perm()
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if modeAtMain != 0o755 {
		t.Fatalf("start-shell mode at canonical main publication = %#o, want 0755", modeAtMain)
	}
	if got := readTestFile(t, runtimePath); got != shellruntime.CanonicalEntrypoint() {
		t.Fatalf("runtime was not canonicalized after mode repair:\n%s", got)
	}
}

func TestApplyPreparesHistoricalDirectEnd4AdapterFromManagedSource(t *testing.T) {
	home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
	legacyAdapter := filepath.Join(hyprDir, "wahrwelt", "hyprland.lua")
	writeTestFile(t, legacyAdapter, historicalCanonicalHyprUserAdapter)
	writeTestFile(t, runtimePath, legacyDirectEnd4RuntimeEntrypoint(shellruntime.End4))
	profile, _ := shellruntime.ProfileByID(shellruntime.End4)
	writeDirectEnd4MigrationAssets(t, home, hyprDir, profile)
	writeTestFile(t, shellruntime.RuntimeFile(home, "shell-launcher.lua"), legacyDirectEnd4LauncherPlaceholder(shellruntime.End4))
	writeTestFile(t, shellruntime.RuntimeFile(home, "shell-keybinds.lua"), legacyDirectEnd4KeybindsPlaceholder(shellruntime.End4))
	if _, err := os.Lstat(filepath.Join(hyprDir, "end4-adapter.lua")); !os.IsNotExist(err) {
		t.Fatalf("fixture unexpectedly has a new End4 adapter: %v", err)
	}
	state := config.Default()
	state.User.HomeDirectory = home
	state.User.Username = ""
	state.Dots = config.Dots{}

	if err := Apply(context.Background(), Options{Sources: paths.Sources{Dots: "../../../dots"}, State: state, Runner: run.New(false)}); err != nil {
		t.Fatal(err)
	}
	for _, asset := range directEnd4MigrationAssetSpecs {
		destination := filepath.Join(hyprDir, filepath.FromSlash(asset.destRel))
		want := readTestFile(t, filepath.Join("../../../dots/hypr", filepath.FromSlash(asset.sourceRel)))
		if got := readTestFile(t, destination); got != want {
			t.Fatalf("managed migration asset %s was not updated from source", asset.sourceRel)
		}
		info, err := os.Stat(destination)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != asset.mode.Perm() {
			t.Fatalf("managed migration asset %s mode = %#o, want %#o", asset.sourceRel, info.Mode().Perm(), asset.mode.Perm())
		}
	}
}

func TestApplyCanonicalRuntimeInvokesPreparedEnd4Adapter(t *testing.T) {
	lua, err := exec.LookPath("lua")
	if err != nil {
		t.Skip("lua interpreter is unavailable")
	}
	home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
	writeTestFile(t, filepath.Join(hyprDir, "wahrwelt", "hyprland.lua"), historicalCanonicalHyprUserAdapter)
	writeTestFile(t, runtimePath, legacyDirectEnd4RuntimeEntrypoint(shellruntime.End4PC))
	profile, _ := shellruntime.ProfileByID(shellruntime.End4PC)
	writeDirectEnd4MigrationAssets(t, home, hyprDir, profile)
	writeTestFile(t, shellruntime.RuntimeFile(home, "shell-launcher.lua"), legacyDirectEnd4LauncherPlaceholder(shellruntime.End4PC))
	writeTestFile(t, shellruntime.RuntimeFile(home, "shell-keybinds.lua"), legacyDirectEnd4KeybindsPlaceholder(shellruntime.End4PC))
	writeTestFile(t, shellruntime.End4VariantStatePath(home), shellruntime.End4PC+"\n")
	dotsSource := writeDirectEnd4MigrationSource(t, "")
	marker := filepath.Join(home, "adapter-load.marker")
	userAdapter := `local home = assert(os.getenv("HOME"))
local state_home = os.getenv("XDG_STATE_HOME") or (home .. "/.local/state")
dofile(state_home .. "/wahrwelt/hypr-runtime/shell-keybinds.lua")
`
	adapter := fmt.Sprintf(`local adapter = {}
function adapter.load(args)
    assert(args.profile == "end4-pc")
    local marker = assert(io.open(%q, "w"))
    marker:write(args.profile, "\n", args.quickshell_config, "\n")
    marker:close()
end
return adapter
`, marker)
	writeTestFile(t, filepath.Join(dotsSource, "hypr", "hyprland.lua"), userAdapter)
	writeTestFile(t, filepath.Join(dotsSource, "hypr", "end4-adapter.lua"), adapter)
	state := config.Default()
	state.User.HomeDirectory = home
	state.User.Username = ""
	state.Dots = config.Dots{}

	if err := Apply(context.Background(), Options{Sources: paths.Sources{Dots: dotsSource}, State: state, Runner: run.New(false)}); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(lua, runtimePath)
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
		"XDG_STATE_HOME="+filepath.Join(home, ".local", "state"),
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("canonical runtime failed: %v\n%s", err, output)
	}
	want := shellruntime.End4PC + "\n" + filepath.Join(home, ".config", "quickshell", profile.QuickshellConfig) + "\n"
	if got := readTestFile(t, marker); got != want {
		t.Fatalf("prepared End4 adapter was not invoked: got %q want %q", got, want)
	}
}

func TestApplyDefersDirectEnd4BundleUntilValidatedHomeManagerArtifactExists(t *testing.T) {
	home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
	direct := legacyDirectEnd4RuntimeEntrypoint(shellruntime.End4)
	writeTestFile(t, runtimePath, direct)
	profile, _ := shellruntime.ProfileByID(shellruntime.End4)
	writeDirectEnd4MigrationAssets(t, home, hyprDir, profile)
	if err := os.Remove(filepath.Join(hyprDir, "end4", ".wahrwelt-runtime-contract")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(home, ".config", "quickshell", profile.QuickshellConfig, "shell.qml")); err != nil {
		t.Fatal(err)
	}
	launcherPath := shellruntime.RuntimeFile(home, "shell-launcher.lua")
	keybindsPath := shellruntime.RuntimeFile(home, "shell-keybinds.lua")
	launcher := legacyDirectEnd4LauncherPlaceholder(shellruntime.End4)
	keybinds := legacyDirectEnd4KeybindsPlaceholder(shellruntime.End4)
	writeTestFile(t, launcherPath, launcher)
	writeTestFile(t, keybindsPath, keybinds)
	state := config.Default()
	state.User.HomeDirectory = home
	state.User.Username = ""
	state.Dots = config.Dots{Hypr: true}

	if err := Apply(context.Background(), Options{Sources: paths.Sources{Dots: "../../../dots"}, State: state, Runner: run.New(false)}); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, runtimePath); got != direct {
		t.Fatalf("direct main changed before validated HM artifact was available:\n%s", got)
	}
	if got := readTestFile(t, launcherPath); got != launcher {
		t.Fatalf("legacy launcher changed during deferred migration: %q", got)
	}
	if got := readTestFile(t, keybindsPath); got != keybinds {
		t.Fatalf("legacy keybinds changed during deferred migration: %q", got)
	}
	if _, err := os.Lstat(filepath.Join(hyprDir, "end4-adapter.lua")); !os.IsNotExist(err) {
		t.Fatalf("Go Hypr sync was not skipped during deferred migration: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(hyprDir, "wahrwelt")); !os.IsNotExist(err) {
		t.Fatalf("legacy namespace remains after safe deferred rename: %v", err)
	}
	if info, err := os.Lstat(filepath.Join(hyprDir, "user")); err != nil || !info.IsDir() {
		t.Fatalf("canonical namespace missing after safe deferred rename: info=%v err=%v", info, err)
	}
}

func TestApplyDefersDirectEnd4BundleForHistoricalActiveHomeManagerAsset(t *testing.T) {
	home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
	direct := legacyDirectEnd4RuntimeEntrypoint(shellruntime.End4PC)
	writeTestFile(t, runtimePath, direct)
	profile, _ := shellruntime.ProfileByID(shellruntime.End4PC)
	writeDirectEnd4MigrationAssets(t, home, hyprDir, profile)
	writeTestFile(t, shellruntime.RuntimeFile(home, "shell-launcher.lua"), legacyDirectEnd4LauncherPlaceholder(shellruntime.End4PC))
	writeTestFile(t, shellruntime.RuntimeFile(home, "shell-keybinds.lua"), legacyDirectEnd4KeybindsPlaceholder(shellruntime.End4PC))
	writeTestFile(t, shellruntime.End4VariantStatePath(home), shellruntime.End4PC+"\n")
	legacyAdapter := filepath.Join(hyprDir, "wahrwelt", "hyprland.lua")
	if err := os.Remove(legacyAdapter); err != nil {
		t.Fatal(err)
	}
	gcroot := filepath.Join(home, ".local", "state", "home-manager", "gcroots", "current-home")
	generation, err := os.Readlink(gcroot)
	if err != nil {
		t.Fatal(err)
	}
	activeTarget := filepath.Join(generation, "home-files", ".config", "hypr", "wahrwelt", "hyprland.lua")
	storeTarget := filepath.Join(t.TempDir(), "historical-hyprland.lua")
	writeTestFile(t, storeTarget, historicalCanonicalHyprUserAdapter)
	if err := os.MkdirAll(filepath.Dir(activeTarget), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(storeTarget, activeTarget); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(activeTarget, legacyAdapter); err != nil {
		t.Fatal(err)
	}
	state := config.Default()
	state.User.HomeDirectory = home
	state.User.Username = ""
	state.Dots = config.Dots{}

	if err := Apply(context.Background(), Options{Sources: paths.Sources{Dots: "../../../dots"}, State: state, Runner: run.New(false)}); err != nil {
		t.Fatal(err)
	}
	if got := readTestFile(t, runtimePath); got != direct {
		t.Fatalf("direct main changed while historical HM asset remained active:\n%s", got)
	}
	canonicalAdapter := filepath.Join(hyprDir, "user", "hyprland.lua")
	if got, err := os.Readlink(canonicalAdapter); err != nil || got != activeTarget {
		t.Fatalf("historical active-HM adapter was replaced: target=%q err=%v", got, err)
	}
	if got := readTestFile(t, shellruntime.End4VariantStatePath(home)); got != shellruntime.End4PC+"\n" {
		t.Fatalf("remembered End4 variant changed during deferred HM migration: %q", got)
	}
}

func TestApplyRejectsUnprovenDirectEnd4TreeBeforeNamespaceMove(t *testing.T) {
	home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
	direct := legacyDirectEnd4RuntimeEntrypoint(shellruntime.End4)
	writeTestFile(t, runtimePath, direct)
	profile, _ := shellruntime.ProfileByID(shellruntime.End4)
	launcher := readTestFile(t, filepath.Join("../../../dots/hypr", filepath.FromSlash(profile.Launcher)))
	writeTestFile(t, filepath.Join(hyprDir, filepath.FromSlash(profile.Launcher)), launcher)
	for _, name := range []string{"hyprland.lua", "hyprlock.conf", "hypridle.conf", ".wahrwelt-runtime-contract"} {
		content := "-- unproven tree\n"
		if name == ".wahrwelt-runtime-contract" {
			content = "end4-adapter-v1\n"
		}
		writeTestFile(t, filepath.Join(hyprDir, "end4", name), content)
	}
	writeTestFile(t, filepath.Join(home, ".config", "quickshell", profile.QuickshellConfig, "shell.qml"), "-- shell\n")
	state := config.Default()
	state.User.HomeDirectory = home
	state.User.Username = ""
	state.Dots = config.Dots{}

	err := Apply(context.Background(), Options{Sources: paths.Sources{Dots: "../../../dots"}, State: state, Runner: run.New(false)})
	if err == nil || !strings.Contains(err.Error(), "unproven direct End4 profile collision") {
		t.Fatalf("apply error = %v, want unproven End4 collision", err)
	}
	if got := readTestFile(t, runtimePath); got != direct {
		t.Fatalf("direct main changed after unproven End4 collision:\n%s", got)
	}
	if info, statErr := os.Lstat(filepath.Join(hyprDir, "wahrwelt")); statErr != nil || !info.IsDir() {
		t.Fatalf("legacy namespace moved before End4 provenance preflight: info=%v err=%v", info, statErr)
	}
}

func TestApplyRejectsInvalidDirectEnd4ArtifactMarkerBeforeNamespaceMove(t *testing.T) {
	home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
	direct := legacyDirectEnd4RuntimeEntrypoint(shellruntime.End4)
	writeTestFile(t, runtimePath, direct)
	profile, _ := shellruntime.ProfileByID(shellruntime.End4)
	writeDirectEnd4MigrationAssets(t, home, hyprDir, profile)
	marker := filepath.Join(hyprDir, "end4", ".wahrwelt-runtime-contract")
	if err := os.Chmod(marker, 0o644); err != nil {
		t.Fatal(err)
	}
	state := config.Default()
	state.User.HomeDirectory = home
	state.User.Username = ""
	state.Dots = config.Dots{}

	err := Apply(context.Background(), Options{Sources: paths.Sources{Dots: "../../../dots"}, State: state, Runner: run.New(false)})
	if err == nil || !strings.Contains(err.Error(), "invalid direct End4 runtime contract") {
		t.Fatalf("apply error = %v, want exact marker rejection", err)
	}
	if got := readTestFile(t, runtimePath); got != direct {
		t.Fatalf("direct main changed after invalid artifact marker:\n%s", got)
	}
	if info, statErr := os.Lstat(filepath.Join(hyprDir, "wahrwelt")); statErr != nil || !info.IsDir() {
		t.Fatalf("legacy namespace moved before marker preflight: info=%v err=%v", info, statErr)
	}
}

func TestApplyRejectsUnknownDirectEnd4AncillaryBeforeNamespaceMove(t *testing.T) {
	for _, name := range []string{"shell-launcher.lua", "shell-keybinds.lua"} {
		t.Run(name, func(t *testing.T) {
			home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
			writeTestFile(t, runtimePath, legacyDirectEnd4RuntimeEntrypoint(shellruntime.End4))
			profile, _ := shellruntime.ProfileByID(shellruntime.End4)
			writeDirectEnd4MigrationAssets(t, home, hyprDir, profile)
			unknownPath := shellruntime.RuntimeFile(home, name)
			unknown := "-- private End4 ancillary\n"
			writeTestFile(t, unknownPath, unknown)
			state := config.Default()
			state.User.HomeDirectory = home
			state.User.Username = ""
			state.Dots = config.Dots{}

			err := Apply(context.Background(), Options{Sources: paths.Sources{Dots: "../../../dots"}, State: state, Runner: run.New(false)})
			if err == nil || !strings.Contains(err.Error(), "unowned direct End4 ancillary runtime collision") {
				t.Fatalf("apply error = %v, want ancillary ownership collision", err)
			}
			if got := readTestFile(t, unknownPath); got != unknown {
				t.Fatalf("unknown ancillary runtime changed: %q", got)
			}
			if got := readTestFile(t, runtimePath); got != legacyDirectEnd4RuntimeEntrypoint(shellruntime.End4) {
				t.Fatalf("direct End4 main runtime changed after ancillary collision:\n%s", got)
			}
			if info, statErr := os.Lstat(filepath.Join(hyprDir, "wahrwelt")); statErr != nil || !info.IsDir() {
				t.Fatalf("legacy namespace moved before ancillary preflight: info=%v err=%v", info, statErr)
			}
		})
	}
}

func TestApplyRejectsUnknownDirectEnd4MigrationAssetBeforeNamespaceMove(t *testing.T) {
	home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
	writeTestFile(t, runtimePath, legacyDirectEnd4RuntimeEntrypoint(shellruntime.End4))
	profile, _ := shellruntime.ProfileByID(shellruntime.End4)
	writeDirectEnd4MigrationAssets(t, home, hyprDir, profile)
	unknownPath := filepath.Join(hyprDir, "lib", "wahrwelt.lua")
	unknown := "return { private = true }\n"
	writeTestFile(t, unknownPath, unknown)
	before, _ := os.Lstat(unknownPath)
	state := config.Default()
	state.User.HomeDirectory = home
	state.User.Username = ""
	state.Dots = config.Dots{}

	err := Apply(context.Background(), Options{Sources: paths.Sources{Dots: "../../../dots"}, State: state, Runner: run.New(false)})
	if err == nil || !strings.Contains(err.Error(), "unowned direct End4 migration asset collision") {
		t.Fatalf("apply error = %v, want migration asset collision", err)
	}
	after, statErr := os.Lstat(unknownPath)
	if statErr != nil || !os.SameFile(before, after) || readTestFile(t, unknownPath) != unknown {
		t.Fatalf("unknown migration asset changed: before=%v after=%v err=%v", before, after, statErr)
	}
	if got := readTestFile(t, runtimePath); got != legacyDirectEnd4RuntimeEntrypoint(shellruntime.End4) {
		t.Fatalf("direct main changed after migration asset collision:\n%s", got)
	}
	if info, statErr := os.Lstat(filepath.Join(hyprDir, "wahrwelt")); statErr != nil || !info.IsDir() {
		t.Fatalf("legacy namespace moved before migration asset preflight: info=%v err=%v", info, statErr)
	}
}

func TestApplyRejectsMissingDirectEnd4AssetBeforeNamespaceMove(t *testing.T) {
	home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
	main := legacyDirectEnd4RuntimeEntrypoint(shellruntime.End4)
	writeTestFile(t, runtimePath, main)
	profile, _ := shellruntime.ProfileByID(shellruntime.End4)
	writeDirectEnd4MigrationAssets(t, home, hyprDir, profile)
	launcherPath := shellruntime.RuntimeFile(home, "shell-launcher.lua")
	keybindsPath := shellruntime.RuntimeFile(home, "shell-keybinds.lua")
	writeTestFile(t, launcherPath, legacyDirectEnd4LauncherPlaceholder(shellruntime.End4))
	writeTestFile(t, keybindsPath, legacyDirectEnd4KeybindsPlaceholder(shellruntime.End4))
	legacyDir := filepath.Join(hyprDir, "wahrwelt")
	beforeMain, _ := os.Lstat(runtimePath)
	beforeLauncher, _ := os.Lstat(launcherPath)
	beforeKeybinds, _ := os.Lstat(keybindsPath)
	beforeLegacy, _ := os.Lstat(legacyDir)
	state := config.Default()
	state.User.HomeDirectory = home
	state.User.Username = ""
	state.Dots = config.Dots{}

	dotsSource := writeDirectEnd4MigrationSource(t, "end4-adapter.lua")
	err := Apply(context.Background(), Options{Sources: paths.Sources{Dots: dotsSource}, State: state, Runner: run.New(false)})
	if err == nil || !strings.Contains(err.Error(), "direct End4 migration source missing or unreadable") {
		t.Fatalf("apply error = %v, want missing asset failure", err)
	}
	for path, before := range map[string]os.FileInfo{
		runtimePath:  beforeMain,
		launcherPath: beforeLauncher,
		keybindsPath: beforeKeybinds,
		legacyDir:    beforeLegacy,
	} {
		after, statErr := os.Lstat(path)
		if statErr != nil || !os.SameFile(before, after) {
			t.Fatalf("%s identity changed before asset preflight: before=%v after=%v err=%v", path, before, after, statErr)
		}
	}
	if got := readTestFile(t, runtimePath); got != main {
		t.Fatalf("direct main changed after missing asset failure:\n%s", got)
	}
}

func TestApplyRejectsUnknownLegacyRuntimeRemovalBeforeNamespaceMove(t *testing.T) {
	home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
	writeTestFile(t, runtimePath, legacyDirectEnd4RuntimeEntrypoint(shellruntime.End4))
	profile, _ := shellruntime.ProfileByID(shellruntime.End4)
	writeDirectEnd4MigrationAssets(t, home, hyprDir, profile)
	legacyPath := filepath.Join(hyprDir, "hyprland.conf")
	unknown := "# private legacy runtime\n"
	writeTestFile(t, legacyPath, unknown)
	before, _ := os.Lstat(legacyPath)
	state := config.Default()
	state.User.HomeDirectory = home
	state.User.Username = ""
	state.Dots = config.Dots{}

	err := Apply(context.Background(), Options{Sources: paths.Sources{Dots: "../../../dots"}, State: state, Runner: run.New(false)})
	if err == nil || !strings.Contains(err.Error(), "unowned legacy Hyprland runtime collision") {
		t.Fatalf("apply error = %v, want legacy removal collision", err)
	}
	after, statErr := os.Lstat(legacyPath)
	if statErr != nil || !os.SameFile(before, after) || readTestFile(t, legacyPath) != unknown {
		t.Fatalf("unknown legacy runtime changed: before=%v after=%v err=%v", before, after, statErr)
	}
	if info, statErr := os.Lstat(filepath.Join(hyprDir, "wahrwelt")); statErr != nil || !info.IsDir() {
		t.Fatalf("legacy namespace moved before removal preflight: info=%v err=%v", info, statErr)
	}
}

func TestApplyPublishesDirectEnd4StateMainBeforeTopLevelCommit(t *testing.T) {
	home, hyprDir, stateRuntime := prepareLegacyHyprUserRuntime(t)
	if err := os.Remove(stateRuntime); err != nil {
		t.Fatal(err)
	}
	topRuntime := filepath.Join(hyprDir, "hyprland.lua")
	direct := legacyDirectEnd4RuntimeEntrypoint(shellruntime.End4PC)
	writeTestFile(t, topRuntime, direct)
	profile, _ := shellruntime.ProfileByID(shellruntime.End4PC)
	writeDirectEnd4MigrationAssets(t, home, hyprDir, profile)
	writeTestFile(t, shellruntime.RuntimeFile(home, "shell-launcher.lua"), legacyDirectEnd4LauncherPlaceholder(shellruntime.End4PC))
	writeTestFile(t, shellruntime.RuntimeFile(home, "shell-keybinds.lua"), legacyDirectEnd4KeybindsPlaceholder(shellruntime.End4PC))
	writeTestFile(t, shellruntime.End4VariantStatePath(home), shellruntime.End4PC+"\n")
	state := config.Default()
	state.User.HomeDirectory = home
	state.User.Username = ""
	state.Dots = config.Dots{}
	injected := errors.New("injected top-level commit barrier")

	err := applyWithHooks(context.Background(), Options{Sources: paths.Sources{Dots: "../../../dots"}, State: state, Runner: run.New(false)}, applyHooks{
		finalRuntime: func(operation, path string) error {
			if operation != runtimeMutationWrite || path != stateRuntime {
				return nil
			}
			if got := readTestFile(t, stateRuntime); got != shellruntime.CanonicalEntrypoint() {
				t.Fatalf("state main was not ready before top-level commit:\n%s", got)
			}
			if got := readTestFile(t, topRuntime); got != direct {
				t.Fatalf("direct top-level main changed before commit barrier:\n%s", got)
			}
			return injected
		},
	})
	if !errors.Is(err, injected) {
		t.Fatalf("apply error = %v, want commit barrier failure", err)
	}
	if _, statErr := os.Lstat(stateRuntime); !os.IsNotExist(statErr) {
		t.Fatalf("state main was not rolled back after commit failure: %v", statErr)
	}
	if got := readTestFile(t, topRuntime); got != direct {
		t.Fatalf("loadable direct top-level main was not preserved after rollback:\n%s", got)
	}
}

func TestApplyPreservesSupportedHomeManagerStableTopLevelEntrypoint(t *testing.T) {
	home, hyprDir, _ := prepareLegacyHyprUserRuntime(t)
	generation := filepath.Join(t.TempDir(), "home-manager-generation")
	topTarget := filepath.Join(generation, "home-files", ".config", "hypr", "hyprland.lua")
	if err := os.MkdirAll(filepath.Dir(topTarget), 0o755); err != nil {
		t.Fatal(err)
	}
	stable := stableRuntimeSourceConfig(shellruntime.RuntimeFile(home, "hyprland.lua"), "Wahrwelt stable Hyprland entrypoint.")
	storePayload := filepath.Join(t.TempDir(), "hm-hyprland.lua")
	if err := os.WriteFile(storePayload, []byte(stable), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(storePayload, topTarget); err != nil {
		t.Fatal(err)
	}
	gcroot := filepath.Join(home, ".local", "state", "home-manager", "gcroots", "current-home")
	if err := os.MkdirAll(filepath.Dir(gcroot), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(generation, gcroot); err != nil {
		t.Fatal(err)
	}
	topLevel := filepath.Join(hyprDir, "hyprland.lua")
	if err := os.Symlink(topTarget, topLevel); err != nil {
		t.Fatal(err)
	}
	state := config.Default()
	state.User.HomeDirectory = home
	state.User.Username = ""
	state.Dots = config.Dots{}

	if err := Apply(context.Background(), Options{State: state, Runner: run.New(false)}); err != nil {
		t.Fatal(err)
	}
	if target, err := os.Readlink(topLevel); err != nil || target != topTarget {
		t.Fatalf("supported Home Manager top-level entrypoint changed: target=%q err=%v", target, err)
	}
}

func TestApplyKeepsTransitionRuntimeLoadableAfterLateMigrationRollback(t *testing.T) {
	home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
	configHome := filepath.Join(home, ".config")
	links := map[string]string{
		filepath.Join(configHome, "hypr", "lib", "mysetup.lua"):           historicalHyprLegacyLinkTarget,
		filepath.Join(configHome, "quickshell", "mysetup-shell-selector"): historicalSelectorLinkTarget,
	}
	for path, target := range links {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
	}
	state := config.Default()
	state.User.HomeDirectory = home
	state.User.Username = ""
	state.Dots = config.Dots{}
	injected := errors.New("injected late migration failure")

	err := applyWithHooks(context.Background(), Options{State: state, Runner: run.New(false)}, applyHooks{
		migration: legacyUserMigrationHooks{
			link: func(index int, _ *legacyLinkRecovery, _ *migrationPinnedDirectory) error {
				if index == 1 {
					return injected
				}
				return nil
			},
		},
	})
	if !errors.Is(err, injected) {
		t.Fatalf("apply error = %v, want injected late migration failure", err)
	}
	if info, statErr := os.Lstat(filepath.Join(hyprDir, "wahrwelt")); statErr != nil || !info.IsDir() {
		t.Fatalf("legacy adapter was not restored after late rollback: info=%v err=%v", info, statErr)
	}
	if _, statErr := os.Lstat(filepath.Join(hyprDir, "user")); !os.IsNotExist(statErr) {
		t.Fatalf("canonical adapter remains after late rollback: %v", statErr)
	}
	if got := readTestFile(t, runtimePath); got != migrationv1tov2.UserNamespaceTransitionEntrypoint() {
		t.Fatalf("late rollback did not retain its loadable transition runtime:\n%s", got)
	}
}

func TestApplyKeepsCanonicalRuntimeAfterPostMigrationFailure(t *testing.T) {
	home, hyprDir, runtimePath := prepareLegacyHyprUserRuntime(t)
	nixosSource := t.TempDir()
	if err := os.MkdirAll(filepath.Join(nixosSource, "Wallpapers"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := config.Default()
	state.User.HomeDirectory = home
	state.User.Username = ""
	state.Dots = config.Dots{Wallpapers: true}
	injected := errors.New("injected wallpaper failure")
	runner := failWallpapersRunner{Runner: run.New(false), err: injected}

	err := Apply(context.Background(), Options{
		Sources: paths.Sources{NixOS: nixosSource},
		State:   state,
		Runner:  runner,
	})
	if !errors.Is(err, injected) {
		t.Fatalf("apply error = %v, want injected post-migration failure", err)
	}
	if _, statErr := os.Lstat(filepath.Join(hyprDir, "wahrwelt")); !os.IsNotExist(statErr) {
		t.Fatalf("legacy namespace returned after post-migration failure: %v", statErr)
	}
	if info, statErr := os.Lstat(filepath.Join(hyprDir, "user")); statErr != nil || !info.IsDir() {
		t.Fatalf("canonical adapter missing after post-migration failure: info=%v err=%v", info, statErr)
	}
	if got := readTestFile(t, runtimePath); got != shellruntime.CanonicalEntrypoint() {
		t.Fatalf("post-migration failure lost the canonical runtime:\n%s", got)
	}
}

type failWallpapersRunner struct {
	run.Runner
	err error
}

func (r failWallpapersRunner) Command(ctx context.Context, name string, args ...string) error {
	if name == "mkdir" && len(args) == 2 && args[0] == "-p" && strings.HasSuffix(args[1], filepath.Join("Pictures", "Wallpapers")) {
		return r.err
	}
	return r.Runner.Command(ctx, name, args...)
}

func prepareLegacyHyprUserRuntime(t *testing.T) (home, hyprDir, runtimePath string) {
	t.Helper()
	home = t.TempDir()
	hyprDir = filepath.Join(home, ".config", "hypr")
	legacyDir := filepath.Join(hyprDir, "wahrwelt")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	managedAdapter := readTestFile(t, "../../../dots/hypr/hyprland.lua")
	if err := os.WriteFile(filepath.Join(legacyDir, "hyprland.lua"), []byte(managedAdapter), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "default.lua"), []byte("-- user config\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runtimePath = shellruntime.RuntimeFile(home, "hyprland.lua")
	if err := os.MkdirAll(filepath.Dir(runtimePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtimePath, []byte(legacyUserRuntimeEntrypoint()), 0o644); err != nil {
		t.Fatal(err)
	}
	return home, hyprDir, runtimePath
}

func historicalHomeManagerUserRuntime(namespace string) string {
	return fmt.Sprintf(`-- Active Hyprland profile: wahrwelt (%s)
local home = os.getenv("HOME")
if home == nil then
    error("HOME is not set; cannot locate Wahrwelt Hyprland config")
end

local config_home = os.getenv("XDG_CONFIG_HOME") or (home .. "/.config")
local state_home = os.getenv("XDG_STATE_HOME") or (home .. "/.local/state")
local hypr_root = config_home .. "/hypr"
local runtime_root = state_home .. "/wahrwelt/hypr-runtime"
package.path = hypr_root .. "/?.lua;" .. hypr_root .. "/?/init.lua;" .. package.path
dofile(hypr_root .. "/%s/hyprland.lua")
dofile(runtime_root .. "/shell-profile.lua")
`, shellruntime.DefaultProfile, namespace)
}

func writeDirectEnd4MigrationAssets(t *testing.T, home, hyprDir string, profile shellruntime.Profile) {
	t.Helper()
	launcher, err := os.ReadFile(filepath.Join("../../../dots/hypr", filepath.FromSlash(profile.Launcher)))
	if err != nil {
		t.Fatal(err)
	}
	end4Store := filepath.Join(t.TempDir(), "end4-store")
	writeTestFile(t, filepath.Join(end4Store, filepath.Base(profile.Launcher)), string(launcher))
	for _, name := range []string{"hyprland.lua", "hyprlock.conf", "hypridle.conf"} {
		writeTestFile(t, filepath.Join(end4Store, name), "-- managed migration asset\n")
	}
	marker := filepath.Join(end4Store, ".wahrwelt-runtime-contract")
	writeTestFile(t, marker, "end4-adapter-v1\n")
	if err := os.Chmod(marker, 0o444); err != nil {
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
	if err := os.MkdirAll(hyprDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(end4Source, filepath.Join(hyprDir, "end4")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(home, ".config", "quickshell", profile.QuickshellConfig, "shell.qml"), "-- managed migration asset\n")
}

func writeDirectEnd4MigrationSource(t *testing.T, skip string) string {
	t.Helper()
	if directEnd4MigrationAssetSpecsErr != nil {
		t.Fatal(directEnd4MigrationAssetSpecsErr)
	}
	dotsSource := t.TempDir()
	for _, asset := range directEnd4MigrationAssetSpecs {
		if asset.sourceRel == skip {
			continue
		}
		data, err := os.ReadFile(filepath.Join("../../../dots/hypr", filepath.FromSlash(asset.sourceRel)))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dotsSource, "hypr", filepath.FromSlash(asset.sourceRel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, asset.mode); err != nil {
			t.Fatal(err)
		}
	}
	if skip != "end4/launcher.lua" {
		data, err := os.ReadFile("../../../dots/hypr/end4/launcher.lua")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dotsSource, "hypr", "end4", "launcher.lua")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dotsSource
}
