package dots

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

func sineProfileAlreadyInstalled(chrome string) bool {
	for _, rel := range []string{
		"JS/sine.sys.mjs",
		"JS/engine.json",
		"utils/chrome.manifest",
		"sine-mods/mods.json",
		"locales/en-US/sine-preferences.ftl",
	} {
		if _, err := os.Stat(filepath.Join(chrome, filepath.FromSlash(rel))); err != nil {
			return false
		}
	}
	return true
}

type sineArchives struct {
	profile    string
	engine     string
	locales    string
	hasLocales bool
}

func downloadSineArchives(ctx context.Context, runner run.CommandRunner, tmp string) (sineArchives, bool) {
	archives := sineArchives{
		profile: filepath.Join(tmp, "profile.zip"),
		engine:  filepath.Join(tmp, "engine.zip"),
		locales: filepath.Join(tmp, "locales.zip"),
	}
	if downloadErr := runner.Command(ctx, "curl", "-fsSL", sineBootloaderURL, "-o", archives.profile); downloadErr != nil {
		fmt.Println("Sine profile download failed; skipping")
		return archives, false
	}
	if downloadErr := runner.Command(ctx, "curl", "-fsSL", sineEngineURL, "-o", archives.engine); downloadErr != nil {
		fmt.Println("Sine engine download failed; skipping")
		return archives, false
	}
	archives.hasLocales = true
	if downloadErr := runner.Command(ctx, "curl", "-fsSL", sineLocalesURL, "-o", archives.locales); downloadErr != nil {
		archives.hasLocales = false
		fmt.Println("Sine locales download failed; continuing without locales")
	}
	return archives, true
}

func printSineArchivePlan(archives sineArchives, chrome string) {
	fmt.Printf("verify sha256 %s %s\n", archives.profile, sineProfileSHA256)
	fmt.Printf("verify sha256 %s %s\n", archives.engine, sineEngineSHA256)
	if archives.hasLocales {
		fmt.Printf("verify sha256 %s %s\n", archives.locales, sineLocalesSHA256)
	}
	fmt.Printf("safe extract %s -> %s\n", archives.profile, chrome)
	fmt.Printf("safe extract %s -> %s\n", archives.engine, chrome)
	if archives.hasLocales {
		fmt.Printf("safe extract %s -> %s\n", archives.locales, chrome)
	}
}

func verifySineArchives(archives sineArchives) error {
	if err := verifyFileSHA256(archives.profile, sineProfileSHA256); err != nil {
		return err
	}
	if err := verifyFileSHA256(archives.engine, sineEngineSHA256); err != nil {
		return err
	}
	if archives.hasLocales {
		return verifyFileSHA256(archives.locales, sineLocalesSHA256)
	}
	return nil
}

func extractSineArchives(archives sineArchives, chrome string) error {
	if err := safeExtractZip(archives.profile, chrome); err != nil {
		return err
	}
	if err := safeExtractZip(archives.engine, chrome); err != nil {
		return err
	}
	if archives.hasLocales {
		return safeExtractZip(archives.locales, chrome)
	}
	return nil
}
