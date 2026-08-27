package dots

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
)

func TestCopyWallpapersSeedsOnlyKnownRegularBasenames(t *testing.T) {
	nixosSource := t.TempDir()
	source := filepath.Join(nixosSource, "Wallpapers")
	home := t.TempDir()
	target := filepath.Join(home, "Pictures", "Wallpapers")
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(target, "user-nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"1.jpg":                "upstream existing\n",
		"2.jpg":                "upstream new\n",
		"preview-upstream.png": "upstream preview\n",
	} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "nested", "ignored.jpg"), []byte("ignored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	userExisting := filepath.Join(target, "1.jpg")
	userPreview := filepath.Join(target, "preview-user.png")
	userNested := filepath.Join(target, "user-nested", "readonly-user.png")
	for path, content := range map[string]string{
		userExisting: "user existing\n",
		userPreview:  "user preview\n",
		userNested:   "user nested\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(userNested, 0o440); err != nil {
		t.Fatal(err)
	}

	if err := copyWallpapers(context.Background(), run.New(false), nixosSource, home); err != nil {
		t.Fatal(err)
	}
	assertWallpaperFile(t, userExisting, "user existing\n", 0o640)
	assertWallpaperFile(t, userPreview, "user preview\n", 0o640)
	assertWallpaperFile(t, userNested, "user nested\n", 0o440)
	assertWallpaperFile(t, filepath.Join(target, "2.jpg"), "upstream new\n", 0o644)
	for _, absent := range []string{
		filepath.Join(target, "preview-upstream.png"),
		filepath.Join(target, "nested", "ignored.jpg"),
	} {
		if _, err := os.Lstat(absent); !os.IsNotExist(err) {
			t.Fatalf("filtered source was copied to %s: %v", absent, err)
		}
	}
}

func TestCopyWallpapersDryRunHasNoBroadTargetMutation(t *testing.T) {
	nixosSource := t.TempDir()
	source := filepath.Join(nixosSource, "Wallpapers")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "1.jpg"), []byte("wallpaper\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "preview-upstream.png"), []byte("preview\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := copyWallpapers(context.Background(), run.Runner{DryRun: true, Stdout: &out}, nixosSource, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	commands := out.String()
	if !strings.Contains(commands, filepath.Join(source, "1.jpg")) {
		t.Fatalf("known wallpaper was not seeded:\n%s", commands)
	}
	for _, forbidden := range []string{"preview-upstream", "find ", "chmod -R", source + "/ "} {
		if strings.Contains(commands, forbidden) {
			t.Fatalf("wallpaper dry-run retained broad mutation %q:\n%s", forbidden, commands)
		}
	}
}

func assertWallpaperFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Fatalf("%s content = %q, want %q", path, data, content)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != mode {
		t.Fatalf("%s mode = %04o, want %04o", path, info.Mode().Perm(), mode)
	}
}

func TestSetupZenMissingProfileWarnsAndSkips(t *testing.T) {
	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	err := setupZen(context.Background(), runner, t.TempDir(), t.TempDir(), "tester", config.Dots{ZenTheme: true})
	if err != nil {
		t.Fatalf("missing Zen profile should be a warning, got %v", err)
	}
}

func TestSetupZenErrorsWhenThemeSourceMissing(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".zen", "profile.Default"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	err := setupZen(context.Background(), runner, t.TempDir(), home, "tester", config.Dots{ZenTheme: true})
	if err == nil {
		t.Fatal("expected missing Zen theme source error")
	}
	if !strings.Contains(err.Error(), "theme source missing") {
		t.Fatalf("expected theme source error, got %v", err)
	}
}

func TestSetupSineProfileIncludesPinnedLocalesArchive(t *testing.T) {
	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	chrome := filepath.Join(t.TempDir(), "chrome")

	if err := setupSineProfile(context.Background(), runner, chrome, "tester"); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	for _, want := range []string{
		"https://github.com/sineorg/bootloader/releases/download/v0.1.4/profile.zip",
		"https://github.com/CosmoCreeper/Sine/releases/download/v2.3/engine.zip",
		"https://github.com/CosmoCreeper/Sine/releases/download/v2.3/locales.zip",
		"locales.zip",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected Sine dry-run output to contain %q, got:\n%s", want, got)
		}
	}
}

func TestSetupSineProfileSkipsWhenAlreadyInstalled(t *testing.T) {
	chrome := t.TempDir()
	for _, rel := range []string{
		"JS/sine.sys.mjs",
		"JS/engine.json",
		"utils/chrome.manifest",
		"sine-mods/mods.json",
		"locales/en-US/sine-preferences.ftl",
	} {
		path := filepath.Join(chrome, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("installed\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := setupSineProfile(context.Background(), runner, chrome, "tester"); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if strings.Contains(got, "curl -fsSL") {
		t.Fatalf("already-installed Sine should not download archives, got:\n%s", got)
	}
}

func TestSetupZenThemeSkipsWhenSourceAlreadyInstalled(t *testing.T) {
	dotsSrc := t.TempDir()
	src := filepath.Join(dotsSrc, "zen", "chrome")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "userChrome.css"), []byte("/* theme */\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	chrome := t.TempDir()
	if err := os.WriteFile(filepath.Join(chrome, "userChrome.css"), []byte("/* theme */\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceHash, err := sourceTreeHash(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chrome, ".wahrwelt-managed.json"), []byte(managedMarkerWithSourceHash("zen-chrome", sourceHash)), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := setupZenTheme(context.Background(), runner, dotsSrc, chrome, "tester"); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, forbidden := range []string{"rsync", "$ mv "} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("already-installed Zen chrome should not run %q, got:\n%s", forbidden, got)
		}
	}
}

func TestSetupZenThemeDoesNotBackupManagedChrome(t *testing.T) {
	dotsSrc := t.TempDir()
	src := filepath.Join(dotsSrc, "zen", "chrome")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "userChrome.css"), []byte("/* theme */\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	chrome := t.TempDir()
	if err := os.WriteFile(filepath.Join(chrome, "userChrome.css"), []byte("/* theme */\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceHash, err := sourceTreeHash(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chrome, ".wahrwelt-managed.json"), []byte(managedMarkerWithSourceHash("zen-chrome", sourceHash)), 0o644); err != nil {
		t.Fatal(err)
	}

	bin := t.TempDir()
	writeScript(t, filepath.Join(bin, "mv"), "exit 42")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	if err := setupZenTheme(context.Background(), run.Runner{}, dotsSrc, chrome, "tester"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(chrome, "userChrome.css")); err != nil {
		t.Fatalf("managed Zen chrome should remain in place: %v", err)
	}
}

func TestSetupZenThemePreservesSineChromeFiles(t *testing.T) {
	dotsSrc := t.TempDir()
	src := filepath.Join(dotsSrc, "zen", "chrome")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "userChrome.css"), []byte("/* updated theme */\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	chrome := t.TempDir()
	if err := os.WriteFile(filepath.Join(chrome, ".wahrwelt-managed.json"), []byte(managedMarkerWithSourceHash("zen-chrome", "old-source-hash")), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := setupZenTheme(context.Background(), runner, dotsSrc, chrome, "tester"); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	for _, want := range []string{
		"--exclude /JS/",
		"--exclude /utils/",
		"--exclude /sine-mods/",
		"--exclude /locales/",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected Zen theme rsync to preserve Sine path %q, got:\n%s", want, got)
		}
	}
}

func TestEnsureZenCustomCSSPrefCreatesUserJS(t *testing.T) {
	profile := t.TempDir()

	if err := ensureZenCustomCSSPref(context.Background(), run.Runner{}, profile, "tester"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(profile, "user.js"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != zenCustomCSSPrefLine+"\n" {
		t.Fatalf("unexpected user.js content:\n%s", got)
	}
}

func TestEnsureZenCustomCSSPrefReplacesDisabledValue(t *testing.T) {
	profile := t.TempDir()
	userJS := filepath.Join(profile, "user.js")
	if err := os.WriteFile(userJS, []byte(`user_pref("toolkit.legacyUserProfileCustomizations.stylesheets", false);
user_pref("other.pref", true);
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ensureZenCustomCSSPref(context.Background(), run.Runner{}, profile, "tester"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(userJS)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.Count(got, zenCustomCSSPref) != 1 {
		t.Fatalf("expected one custom CSS pref, got:\n%s", got)
	}
	if !strings.Contains(got, zenCustomCSSPrefLine) {
		t.Fatalf("expected enabled custom CSS pref, got:\n%s", got)
	}
	if !strings.Contains(got, `user_pref("other.pref", true);`) {
		t.Fatalf("expected unrelated prefs to be preserved, got:\n%s", got)
	}
}
