package dots

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

func TestCopyWallpapersReturnsPreviewCleanupError(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "Wallpapers"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeScript(t, filepath.Join(bin, "mkdir"), "exit 0")
	writeScript(t, filepath.Join(bin, "find"), "exit 17")
	writeScript(t, filepath.Join(bin, "rsync"), "exit 0")
	t.Setenv("PATH", bin)

	var out bytes.Buffer
	runner := run.Runner{Stdout: &out, Stderr: &out}
	err := copyWallpapers(context.Background(), runner, src, t.TempDir())
	if err == nil {
		t.Fatal("expected preview cleanup error")
	}
	if !strings.Contains(err.Error(), "find failed") {
		t.Fatalf("expected find failure, got %v", err)
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
	if err := os.WriteFile(filepath.Join(chrome, ".mysetup-managed.json"), []byte(managedMarkerWithSourceHash("zen-chrome", sourceHash)), 0o644); err != nil {
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
	if err := os.WriteFile(filepath.Join(chrome, ".mysetup-managed.json"), []byte(managedMarkerWithSourceHash("zen-chrome", sourceHash)), 0o644); err != nil {
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
