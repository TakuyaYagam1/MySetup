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
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
)

func TestWriteShellLauncherConfigWritesRuntimeLauncher(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shell-profile.conf")

	if err := writeShellLauncherConfig(path); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"# Runtime shell launcher",
		"exec-once = " + filepath.Join(dir, "scripts", "start-shell.sh"),
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("shell launcher missing %q\n%s", want, got)
		}
	}
}

func TestWriteShellLauncherConfigReplacesExistingSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shell-profile.conf")
	target := filepath.Join(dir, "store-shell-profile.conf")
	if err := os.WriteFile(target, []byte("old target\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}

	if err := writeShellLauncherConfig(path); err != nil {
		t.Fatal(err)
	}

	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("shell launcher should be rewritten as a regular file, got symlink %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "start-shell.sh") || strings.Contains(string(data), "start-shell.sh noctalia") {
		t.Fatalf("shell launcher should contain runtime launcher without hardcoded profile\n%s", string(data))
	}
	targetData, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(targetData) != "old target\n" {
		t.Fatalf("symlink target should not be overwritten, got %q", string(targetData))
	}
}

func TestWriteShellKeybindsConfigUsesCurrentShell(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shell-keybinds.conf")
	noctaliaKeybinds := filepath.Join(dir, "noctalia", "keybinds.conf")
	if err := os.MkdirAll(filepath.Dir(noctaliaKeybinds), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(noctaliaKeybinds, []byte("# noctalia binds\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeShellKeybindsConfig(path, "noctalia"); err != nil {
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
			t.Fatalf("shell keybind layer missing %q\n%s", want, got)
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

	if err := writeShellKeybindsConfig(path, "caelestia"); err != nil {
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

	err := writeShellKeybindsConfig(filepath.Join(dir, "shell-keybinds.conf"), "noctalia")
	if err == nil {
		t.Fatal("expected missing shell keybind layer error")
	}
	if !strings.Contains(err.Error(), "shell keybind profile missing") {
		t.Fatalf("expected missing shell keybind layer error, got %v", err)
	}
}

func TestWriteHyprRuntimeShellStateSeedsLegacyRuntimeFiles(t *testing.T) {
	home := t.TempDir()
	hyprDir := filepath.Join(home, ".config", "hypr")
	if err := os.MkdirAll(filepath.Join(hyprDir, "caelestia"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(hyprDir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(hyprDir, "mysetup"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hyprDir, "caelestia", "keybinds.conf"), []byte("# binds\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hyprDir, "mysetup", "hyprland.conf"), []byte("monitor = eDP-1, 2560x1600@120, 0x0, 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	state := config.Default()
	state.User.HomeDirectory = home

	if err := writeHyprRuntimeShellState(home, state, hyprDir); err != nil {
		t.Fatal(err)
	}

	activeShell, err := os.ReadFile(paths.ActiveShellStatePath(home))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(activeShell)) != "caelestia" {
		t.Fatalf("expected active shell caelestia, got %q", string(activeShell))
	}

	entrypoint, err := os.ReadFile(filepath.Join(hyprDir, "hyprland.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(entrypoint), "mysetup/hyprland.conf") {
		t.Fatalf("expected legacy entrypoint to source mysetup config\n%s", string(entrypoint))
	}

	hyprlock, err := os.ReadFile(filepath.Join(hyprDir, "hyprlock.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(hyprlock), "shell-managed (caelestia)") {
		t.Fatalf("expected legacy hyprlock placeholder to mention caelestia\n%s", string(hyprlock))
	}

	hypridle, err := os.ReadFile(filepath.Join(hyprDir, "hypridle.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(hypridle), "shell-managed (caelestia)") {
		t.Fatalf("expected legacy hypridle placeholder to mention caelestia\n%s", string(hypridle))
	}
}

func TestWriteHyprRuntimeShellStateSeedsEnd4StateBeforeProfileExists(t *testing.T) {
	home := t.TempDir()
	hyprDir := filepath.Join(home, ".config", "hypr")
	if err := os.MkdirAll(filepath.Join(hyprDir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}

	state := config.Default()
	state.User.HomeDirectory = home
	state.Shell.Profile = "end4"

	if err := writeHyprRuntimeShellState(home, state, hyprDir); err != nil {
		t.Fatal(err)
	}

	activeShell, err := os.ReadFile(paths.ActiveShellStatePath(home))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(activeShell)) != "end4" {
		t.Fatalf("expected active shell end4, got %q", string(activeShell))
	}

	if _, err := os.Stat(filepath.Join(hyprDir, "hyprland.conf")); !os.IsNotExist(err) {
		t.Fatalf("expected end4 entrypoint to stay untouched until Home Manager installs the profile, got err=%v", err)
	}
}

func TestWriteHyprRuntimeShellStateSeedsEnd4RuntimeFilesWhenProfileExists(t *testing.T) {
	home := t.TempDir()
	hyprDir := filepath.Join(home, ".config", "hypr")
	if err := os.MkdirAll(filepath.Join(hyprDir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(hyprDir, "end4"), 0o755); err != nil {
		t.Fatal(err)
	}
	for rel := range map[string]string{
		"end4/hyprland.conf": "source = ~/.config/hypr/end4/hyprland/env.conf\n",
		"end4/hyprlock.conf": "background {}\n",
		"end4/hypridle.conf": "general {}\n",
	} {
		path := filepath.Join(hyprDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("placeholder\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	state := config.Default()
	state.User.HomeDirectory = home
	state.Shell.Profile = "end4"

	if err := writeHyprRuntimeShellState(home, state, hyprDir); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		path string
		want string
	}{
		{path: filepath.Join(hyprDir, "hyprland.conf"), want: "source = " + filepath.Join(hyprDir, "end4", "hyprland.conf")},
		{path: filepath.Join(hyprDir, "hyprlock.conf"), want: "source = " + filepath.Join(hyprDir, "end4", "hyprlock.conf")},
		{path: filepath.Join(hyprDir, "hypridle.conf"), want: "source = " + filepath.Join(hyprDir, "end4", "hypridle.conf")},
	} {
		data, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), tc.want) {
			t.Fatalf("expected %s to contain %q\n%s", tc.path, tc.want, string(data))
		}
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
		"app2unit -- $terminal",
		"$hypr/scripts/screenshot.sh full",
		"$hypr/scripts/record-toggle.sh",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("shared Hypr keybinds must stay shell-neutral; found %q\n%s", forbidden, text)
		}
	}
	if !strings.Contains(text, "source = $hypr/shell-keybinds.conf") {
		t.Fatalf("shared Hypr keybinds must source the runtime profile layer\n%s", text)
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
		"shell-selector.sh toggle",
		"app2unit -- $terminal",
		"$hypr/scripts/screenshot.sh full",
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
		"shell-selector.sh toggle",
		"app2unit -- $terminal",
		"$hypr/scripts/screenshot.sh full",
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

func TestShellSelectorScriptTracksFocusedMonitorAndActiveShell(t *testing.T) {
	data, err := os.ReadFile("../../../dots/hypr/scripts/shell-selector.sh")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`state_dir="$runtime_dir/mysetup-shell-selector"`,
		`lock_dir="$state_dir/lock"`,
		"selector_name=\"mysetup-shell-selector\"",
		"MYSETUP_SHELL_SELECTOR_MONITOR",
		"MYSETUP_ACTIVE_SHELL",
		"acquire_lock()",
		"wait_for_selector_spawn()",
		"detect_shell_from_processes()",
		"detect_shell_from_entrypoint()",
		"quickshell/ii/shell\\.qml",
		"mysetup/hyprland.conf",
		"hyprctl monitors -j",
		"active-shell",
		"start-shell.sh",
		"qs -c \"$selector_name\"",
		"switch_shell()",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("shell selector script missing %q\n%s", want, text)
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
		"sync_runtime_shell_files()",
		"persistent_state_file=",
		"selector_pattern=",
		"end4_pattern=",
		"stop_shell_selector()",
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
	if strings.Contains(text, "qs kill --any-display") {
		t.Fatalf("start-shell script must not kill every quickshell instance\n%s", text)
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

func captureStdoutDots(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = write
	defer func() {
		os.Stdout = original
	}()
	fn()
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := out.ReadFrom(read); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func configSourcesForDotsTest(t *testing.T) paths.Sources {
	t.Helper()
	base := t.TempDir()
	return paths.Sources{
		RepoRoot:  base,
		NixOS:     filepath.Join(base, "nixos"),
		Dots:      filepath.Join(base, "dots"),
		Installer: filepath.Join(base, "installer"),
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
