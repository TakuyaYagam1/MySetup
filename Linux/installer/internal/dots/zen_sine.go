package dots

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/run"
	"github.com/TakuyaYagam1/wahrwelt/Linux/installer/internal/zenutil"
)

const (
	zenCustomCSSPref     = "toolkit.legacyUserProfileCustomizations.stylesheets"
	zenCustomCSSPrefLine = `user_pref("toolkit.legacyUserProfileCustomizations.stylesheets", true);`
)

var sineChromePreservePaths = []string{
	"/JS/",
	"/locales/",
	"/sine-mods/",
	"/utils/",
}

func setupZen(ctx context.Context, runner run.CommandRunner, dotsSrc, home, username string, cfg config.Dots) error {
	profile := zenutil.FindProfile(home)
	if profile == "" {
		fmt.Println("Zen Browser profile not found; launch Zen once and rerun mysetup dots/apply")
		return nil
	}
	chrome := filepath.Join(profile, "chrome")
	if cfg.ZenTheme {
		if err := setupZenTheme(ctx, runner, dotsSrc, chrome, username); err != nil {
			return err
		}
		if err := ensureZenCustomCSSPref(ctx, runner, profile, username); err != nil {
			return err
		}
	}
	if cfg.Sine {
		if err := setupSineProfile(ctx, runner, chrome, username); err != nil {
			return err
		}
	}
	return nil
}

func setupZenTheme(ctx context.Context, runner run.CommandRunner, dotsSrc, chrome, username string) error {
	src := filepath.Join(dotsSrc, "zen", "chrome")
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("zen browser theme source missing: %s", src)
		}
		return fmt.Errorf("zen browser theme source unreadable: %w", err)
	}
	if err := backupIfUnmanaged(ctx, runner, chrome, "zen-chrome"); err != nil {
		return err
	}
	sourceHash, alreadyInstalled, err := managedSourceAlreadyInstalled(src, chrome, "zen-chrome", nil)
	if err != nil {
		return err
	}
	if alreadyInstalled {
		if err := writeMarkerWithOwnerAndSourceHash(ctx, runner, filepath.Join(chrome, ".mysetup-managed.json"), "zen-chrome", username, sourceHash); err != nil {
			return err
		}
		fmt.Printf("Zen chrome already exists in %s; skipping sync\n", chrome)
		return nil
	}
	if err := runner.Command(ctx, "mkdir", "-p", chrome); err != nil {
		return err
	}
	if err := ensureUserWritableTree(ctx, runner, chrome, username); err != nil {
		return err
	}
	args := []string{"-a", "--delete"}
	for _, rel := range sineChromePreservePaths {
		args = append(args, "--exclude", rel)
	}
	args = append(args, src+"/", chrome+"/")
	if err := runner.Command(ctx, "rsync", args...); err != nil {
		return err
	}
	if err := ensureUserWritableTree(ctx, runner, chrome, username); err != nil {
		return err
	}
	return writeMarkerWithOwnerAndSourceHash(ctx, runner, filepath.Join(chrome, ".mysetup-managed.json"), "zen-chrome", username, sourceHash)
}

func ensureZenCustomCSSPref(ctx context.Context, runner run.CommandRunner, profile, username string) error {
	userJS := filepath.Join(profile, "user.js")
	data, err := os.ReadFile(userJS)
	mode := os.FileMode(0o644)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read Zen user.js: %w", err)
		}
	} else if info, statErr := os.Stat(userJS); statErr == nil {
		mode = info.Mode().Perm()
	} else {
		return fmt.Errorf("stat Zen user.js: %w", statErr)
	}

	next := upsertZenCustomCSSPref(string(data))
	if next == string(data) {
		return nil
	}
	if runner.IsDryRun() {
		fmt.Printf("write Zen custom CSS pref %s\n", userJS)
		return nil
	}
	if err := os.WriteFile(userJS, []byte(next), mode); err != nil {
		if os.IsPermission(err) && username != "" {
			return sudoInstallZenUserJS(ctx, runner, userJS, username, next, mode)
		}
		return fmt.Errorf("write Zen user.js: %w", err)
	}
	return nil
}

func upsertZenCustomCSSPref(text string) string {
	if text == "" {
		return zenCustomCSSPrefLine + "\n"
	}

	prefPrefix := `user_pref("` + zenCustomCSSPref + `"`
	lines := strings.Split(text, "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), prefPrefix) {
			lines[i] = zenCustomCSSPrefLine
			found = true
		}
	}
	if found {
		next := strings.Join(lines, "\n")
		if !strings.HasSuffix(next, "\n") {
			next += "\n"
		}
		return next
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return text + zenCustomCSSPrefLine + "\n"
}

func sudoInstallZenUserJS(ctx context.Context, runner run.CommandRunner, target, username, content string, mode os.FileMode) error {
	temp, err := os.CreateTemp("", "mysetup-zen-user-js-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()
	if _, err := temp.WriteString(content); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return runner.Command(ctx, "sudo", "install", "-D", "-m", fmt.Sprintf("%04o", mode.Perm()), "-o", username, tempPath, target)
}

func setupSineProfile(ctx context.Context, runner run.CommandRunner, chrome, username string) error {
	fmt.Println("Sine profile install is intentionally pinned and best-effort")
	if sineProfileAlreadyInstalled(chrome) {
		fmt.Printf("Sine profile files already exist in %s; skipping download\n", chrome)
		return nil
	}
	tmp, err := os.MkdirTemp("", "mysetup-sine-*")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(tmp)
	}()

	archives, ok := downloadSineArchives(ctx, runner, tmp)
	if !ok {
		return nil
	}
	if err := runner.Command(ctx, "mkdir", "-p", chrome); err != nil {
		return err
	}
	if err := ensureUserWritableTree(ctx, runner, chrome, username); err != nil {
		return err
	}
	if runner.IsDryRun() {
		printSineArchivePlan(archives, chrome)
		return nil
	}
	if err := verifySineArchives(archives); err != nil {
		return err
	}
	if err := extractSineArchives(archives, chrome); err != nil {
		return err
	}
	if err := ensureUserWritableTree(ctx, runner, chrome, username); err != nil {
		return err
	}
	fmt.Println("Sine profile part installed; clear Zen startup cache from about:support, then restart Zen Browser")
	return nil
}
