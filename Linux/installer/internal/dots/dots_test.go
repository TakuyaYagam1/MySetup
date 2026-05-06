package dots

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

func TestWriteShellProfileConfigUsesCurrentProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shell-profile.conf")
	state := config.Default()
	state.Shell.Profile = "noctalia"

	if err := writeShellProfileConfig(path, state); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"# Active shell profile: noctalia",
		"exec-once = " + filepath.Join(dir, "scripts", "start-shell.sh") + " noctalia",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("shell profile missing %q\n%s", want, got)
		}
	}
}

func TestWriteShellProfileConfigDefaultsToCaelestia(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shell-profile.conf")
	state := config.Default()
	state.Shell.Profile = ""

	if err := writeShellProfileConfig(path, state); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "start-shell.sh caelestia") {
		t.Fatalf("shell profile should default to caelestia\n%s", string(data))
	}
}

func TestWriteShellProfileConfigReplacesExistingSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shell-profile.conf")
	target := filepath.Join(dir, "store-shell-profile.conf")
	if err := os.WriteFile(target, []byte("old target\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}

	state := config.Default()
	state.Shell.Profile = "noctalia"
	if err := writeShellProfileConfig(path, state); err != nil {
		t.Fatal(err)
	}

	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("shell profile should be rewritten as a regular file, got symlink %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "start-shell.sh noctalia") {
		t.Fatalf("shell profile should contain requested profile\n%s", string(data))
	}
	targetData, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(targetData) != "old target\n" {
		t.Fatalf("symlink target should not be overwritten, got %q", string(targetData))
	}
}

func TestWriteShellKeybindsConfigUsesCurrentProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shell-keybinds.conf")
	noctaliaKeybinds := filepath.Join(dir, "noctalia", "keybinds.conf")
	if err := os.MkdirAll(filepath.Dir(noctaliaKeybinds), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(noctaliaKeybinds, []byte("# noctalia binds\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := config.Default()
	state.Shell.Profile = "noctalia"

	if err := writeShellKeybindsConfig(path, state); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"# Active shell keybind profile: noctalia",
		"source = " + noctaliaKeybinds,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("shell keybind config missing %q\n%s", want, got)
		}
	}
}

func TestWriteShellKeybindsConfigReplacesExistingSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shell-keybinds.conf")
	caelestiaKeybinds := filepath.Join(dir, "caelestia", "keybinds.conf")
	if err := os.MkdirAll(filepath.Dir(caelestiaKeybinds), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caelestiaKeybinds, []byte("# caelestia binds\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "store-shell-keybinds.conf")
	if err := os.WriteFile(target, []byte("old target\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}

	if err := writeShellKeybindsConfig(path, config.Default()); err != nil {
		t.Fatal(err)
	}

	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("shell keybinds should be rewritten as a regular file, got symlink %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "source = "+caelestiaKeybinds) {
		t.Fatalf("shell keybind config should source default profile\n%s", string(data))
	}
}

func TestWriteShellKeybindsConfigErrorsWhenProfileMissing(t *testing.T) {
	dir := t.TempDir()
	state := config.Default()
	state.Shell.Profile = "noctalia"

	err := writeShellKeybindsConfig(filepath.Join(dir, "shell-keybinds.conf"), state)
	if err == nil {
		t.Fatal("expected missing shell keybind profile error")
	}
	if !strings.Contains(err.Error(), "shell keybind profile missing") {
		t.Fatalf("expected missing profile error, got %v", err)
	}
}

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
	err := setupZen(context.Background(), runner, t.TempDir(), t.TempDir(), config.Dots{ZenTheme: true})
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
	err := setupZen(context.Background(), runner, t.TempDir(), home, config.Dots{ZenTheme: true})
	if err == nil {
		t.Fatal("expected missing Zen theme source error")
	}
	if !strings.Contains(err.Error(), "theme source missing") {
		t.Fatalf("expected theme source error, got %v", err)
	}
}

func TestVerifyFileSHA256RejectsMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload.zip")
	if err := os.WriteFile(path, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := verifyFileSHA256(path, strings.Repeat("0", 64))
	if err == nil {
		t.Fatal("expected sha256 mismatch")
	}
	if !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("expected sha256 mismatch, got %v", err)
	}
}

func TestSafeExtractZipExtractsRegularFile(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "safe.zip")
	writeZip(t, zipPath, []zipEntry{{name: "dir/file.txt", body: "hello"}})
	dest := t.TempDir()

	if err := safeExtractZip(zipPath, dest); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "dir", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected extracted content: %q", string(data))
	}
}

func TestSafeExtractZipRejectsTraversal(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "traversal.zip")
	writeZip(t, zipPath, []zipEntry{{name: "../escape.txt", body: "nope"}})

	err := safeExtractZip(zipPath, t.TempDir())
	if err == nil {
		t.Fatal("expected traversal rejection")
	}
	if !strings.Contains(err.Error(), "outside destination") {
		t.Fatalf("expected outside destination error, got %v", err)
	}
}

func TestSafeExtractZipRejectsSymlink(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "symlink.zip")
	writeZip(t, zipPath, []zipEntry{{name: "link", body: "target", mode: os.ModeSymlink | 0o777}})

	err := safeExtractZip(zipPath, t.TempDir())
	if err == nil {
		t.Fatal("expected symlink rejection")
	}
	if !strings.Contains(err.Error(), "refusing to extract symlink") {
		t.Fatalf("expected symlink error, got %v", err)
	}
}

func TestSafeExtractZipRejectsPreExistingSymlinkPath(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "payload.zip")
	writeZip(t, zipPath, []zipEntry{{name: "linked/file.txt", body: "nope"}})
	dest := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dest, "linked")); err != nil {
		t.Fatal(err)
	}

	err := safeExtractZip(zipPath, dest)
	if err == nil {
		t.Fatal("expected pre-existing symlink path rejection")
	}
	if !strings.Contains(err.Error(), "existing symlink") {
		t.Fatalf("expected existing symlink rejection, got %v", err)
	}
}

func TestBackupIfUnmanagedReturnsMarkerStatError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission mode test is Unix-specific")
	}
	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(target, 0o755)
	})

	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	err := backupIfUnmanaged(context.Background(), runner, target)
	if err == nil {
		t.Fatal("expected marker stat error")
	}
	if !strings.Contains(err.Error(), "stat managed marker") {
		t.Fatalf("expected marker stat error, got %v", err)
	}
}

func TestSyncHyprReturnsExecutableCommandError(t *testing.T) {
	dotsSrc := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dotsSrc, "hypr"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeScript(t, filepath.Join(bin, "rsync"), `last=
for arg do last=$arg; done
mkdir -p "$last/scripts" "$last/caelestia"
printf '%s\n' '# caelestia binds' > "$last/caelestia/keybinds.conf"`)
	writeScript(t, filepath.Join(bin, "chmod"), "exit 0")
	writeScript(t, filepath.Join(bin, "find"), "exit 27")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	var out bytes.Buffer
	runner := run.Runner{Stdout: &out, Stderr: &out}
	err := syncHypr(context.Background(), runner, dotsSrc, t.TempDir(), config.Default())
	if err == nil {
		t.Fatal("expected scripts executable command error")
	}
	if !strings.Contains(err.Error(), "find failed") {
		t.Fatalf("expected find failure, got %v", err)
	}
}

func TestSyncHyprIgnoresHyprctlReloadFailure(t *testing.T) {
	dotsSrc := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dotsSrc, "hypr"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeScript(t, filepath.Join(bin, "rsync"), `last=
for arg do last=$arg; done
mkdir -p "$last/scripts" "$last/caelestia"
printf '%s\n' '# caelestia binds' > "$last/caelestia/keybinds.conf"`)
	writeScript(t, filepath.Join(bin, "chmod"), "exit 0")
	writeScript(t, filepath.Join(bin, "find"), "exit 0")
	writeScript(t, filepath.Join(bin, "sh"), "exit 42")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))

	var out bytes.Buffer
	runner := run.Runner{Stdout: &out, Stderr: &out}
	if err := syncHypr(context.Background(), runner, dotsSrc, t.TempDir(), config.Default()); err != nil {
		t.Fatalf("hyprctl reload failure should stay best-effort, got %v", err)
	}
}

func TestSharedHyprKeybindsDoNotContainShellSpecificBindings(t *testing.T) {
	data, err := os.ReadFile("../../../dots/hypr/hyprland/keybinds.conf")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{
		"caelestia clipboard",
		"caelestia emoji",
		"caelestia record",
		"caelestia resizer pip",
		"noctalia-shell ipc call",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("shared Hypr keybinds must stay shell-neutral; found %q\n%s", forbidden, text)
		}
	}
}

func TestShellKeybindProfilesUseExpectedLaunchers(t *testing.T) {
	caelestia, err := os.ReadFile("../../../dots/hypr/caelestia/keybinds.conf")
	if err != nil {
		t.Fatal(err)
	}
	noctalia, err := os.ReadFile("../../../dots/hypr/noctalia/keybinds.conf")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"bindi = Super, Super_L, global, caelestia:launcher",
		"bindin = Super, catchall, global, caelestia:launcherInterrupt",
		"bindin = Super, mouse:272, global, caelestia:launcherInterrupt",
		"bindin = Super, mouse_down, global, caelestia:launcherInterrupt",
		"start-shell.sh caelestia",
		"caelestia clipboard",
	} {
		if !strings.Contains(string(caelestia), want) {
			t.Fatalf("caelestia profile missing %q\n%s", want, string(caelestia))
		}
	}
	for _, want := range []string{
		"noctalia-shell ipc call",
		"noctalia-launcher.sh press",
		"noctalia-launcher.sh interrupt",
		"noctalia-launcher.sh release",
		"start-shell.sh noctalia",
	} {
		if !strings.Contains(string(noctalia), want) {
			t.Fatalf("noctalia profile missing %q\n%s", want, string(noctalia))
		}
	}
}

func TestNoctaliaLauncherScriptIsGuarded(t *testing.T) {
	data, err := os.ReadFile("../../../dots/hypr/scripts/noctalia-launcher.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"active_file=",
		"interrupt_file=",
		"lock_dir=",
		"lock_owner_file=",
		"mysetup-noctalia-launcher",
		"noctalia-launcher\\.sh",
		"acquire_lock",
		"noctalia-shell ipc call launcher toggle",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("noctalia launcher wrapper missing %q\n%s", want, text)
		}
	}
}

func TestRecordToggleScriptUsesLockAndPidValidation(t *testing.T) {
	data, err := os.ReadFile("../../../dots/hypr/scripts/record-toggle.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`mkdir "$lock_dir"`,
		`lock_pid_file=`,
		`lock_owner_file=`,
		"mysetup-record-toggle",
		"record-toggle\\.sh",
		`ps -p "$pid" -o args=`,
		"gpu-screen-recorder",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("record toggle script missing %q\n%s", want, text)
		}
	}
}

func TestStartShellScriptCleansDuplicateProfiles(t *testing.T) {
	data, err := os.ReadFile("../../../dots/hypr/scripts/start-shell.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"lock_owner_file=",
		"mysetup-start-shell",
		"pid_matches()",
		"start-shell\\.sh",
		"running_count()",
		"dedupe_shell()",
		"sync_hypr_profile()",
		"shell-keybinds.conf",
		"hyprctl reload",
		`("$@" >>"$log_file" 2>&1 &)`,
		`(caelestia resizer -d >>"$log_file" 2>&1 &)`,
		`dedupe_shell "caelestia"`,
		`dedupe_shell "noctalia"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("start-shell script missing %q\n%s", want, text)
		}
	}
}

func TestWsactionGroupModeShiftsFishArgv(t *testing.T) {
	if _, err := exec.LookPath("fish"); err != nil {
		t.Skip("fish is not installed")
	}
	bin := t.TempDir()
	callFile := filepath.Join(t.TempDir(), "hyprctl-call")
	writeScript(t, filepath.Join(bin, "hyprctl"), `if [ "$1" = "activeworkspace" ]; then
  printf '{"id":%s}\n' "$ACTIVE_WS"
  exit 0
fi
printf '%s\n' "$*" > "$CALL_FILE"
`)
	writeScript(t, filepath.Join(bin, "jq"), `printf '%s\n' "$ACTIVE_WS"`)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("CALL_FILE", callFile)

	script, err := filepath.Abs("../../../dots/hypr/scripts/wsaction.fish")
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		"10": "dispatch workspace 10",
		"20": "dispatch workspace 10",
		"23": "dispatch workspace 3",
	}
	for activeWS, want := range cases {
		t.Run(activeWS, func(t *testing.T) {
			t.Setenv("ACTIVE_WS", activeWS)
			if err := os.Remove(callFile); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}

			cmd := exec.Command("fish", script, "-g", "workspace", "1")
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("wsaction.fish failed: %v\n%s", err, string(output))
			}
			data, err := os.ReadFile(callFile)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.TrimSpace(string(data)); got != want {
				t.Fatalf("unexpected hyprctl dispatch call: got %q want %q", got, want)
			}
		})
	}
}

func writeScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

type zipEntry struct {
	name string
	body string
	mode os.FileMode
}

func writeZip(t *testing.T, path string, entries []zipEntry) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name}
		if entry.mode != 0 {
			header.SetMode(entry.mode)
		}
		part, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(entry.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
