package dots

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/config"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/paths"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/run"
	"github.com/TakuyaYagam1/MySetup/Linux/installer/internal/shellruntime"
)

func TestSyncHyprReturnsExecutableCommandError(t *testing.T) {
	dotsSrc := t.TempDir()
	writeRequiredHyprSource(t, dotsSrc)
	bin := t.TempDir()
	writeScript(t, filepath.Join(bin, "rsync"), `last=
for arg do last=$arg; done
mkdir -p "$last/scripts" "$last/caelestia" "$last/hyprland"
printf '%s\n' '-- caelestia binds' > "$last/caelestia/keybinds.lua"
printf '%s\n' '-- caelestia launcher' > "$last/caelestia/launcher.lua"
printf '%s\n' 'hl.config({ input = { kb_layout = "us", kb_options = "grp:alt_shift_toggle" } })' > "$last/hyprland/input.lua"
printf '%s\n' 'mysetup.load_runtime("shell-keybinds.lua")' > "$last/hyprland/keybinds.lua"
printf '%s\n' '#!/usr/bin/env bash' > "$last/scripts/start-shell.sh"`)
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

func TestSyncHyprExcludesHomeManagerEnd4Profile(t *testing.T) {
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

	state := config.Default()
	state.User.Username = "tester"
	var out bytes.Buffer
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := syncHypr(context.Background(), runner, dotsSrc, configDir, state); err != nil {
		t.Fatal(err)
	}

	got := out.String()
	for _, want := range []string{
		"rm -rf -- " + staleEnd4Dir,
		"--exclude /hyprland.lua",
		"--exclude /hyprlock.conf",
		"--exclude /hypridle.conf",
		"--exclude /mysetup/hyprland.lua",
		"--exclude /mysetup/local.lua",
		"--exclude /runtime/",
		"--exclude /end4/",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected hypr sync output to contain %q, got:\n%s", want, got)
		}
	}
}

func TestSyncHyprPreservesEnd4ProfileWhenActive(t *testing.T) {
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
	runner := run.Runner{DryRun: true, Stdout: &out, Stderr: &out}
	if err := syncHypr(context.Background(), runner, dotsSrc, configDir, state); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(out.String(), "rm -rf -- "+staleEnd4Dir) {
		t.Fatalf("active end4 profile must not be pruned during hypr sync:\n%s", out.String())
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

	state := config.Default()
	state.User.Username = "tester"
	var out bytes.Buffer
	runner := run.Runner{Stdout: &out, Stderr: &out}
	if err := writeHyprLocalConfig(context.Background(), runner, state, hyprSourceDir, hyprDir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(hyprDir, "mysetup", "local.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "hl.monitor") || !strings.Contains(string(data), "kb_layout") {
		t.Fatalf("expected generated mysetup local Lua config, got:\n%s", data)
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
		"hypr/hyprland.lua":                 "require(\"hyprland.keybinds\")\n",
		"hypr/hyprland/input.lua":           "hl.config({ input = { kb_layout = \"us\", kb_options = \"grp:alt_shift_toggle\" } })\n",
		"hypr/hyprland/keybinds.lua":        "mysetup.load_runtime(\"shell-keybinds.lua\")\n",
		"hypr/shell-common-keybinds.lua":    "mysetup.bind_exec(\"SUPER + SHIFT + W\", mysetup.hypr .. \"/scripts/shell-selector.sh toggle\")\n",
		"hypr/shell-workspace-keybinds.lua": "mysetup.bind_exec(\"SUPER + 1\", mysetup.hypr .. \"/scripts/wsaction.fish -g workspace 1\")\n",
		"hypr/lib/mysetup.lua":              "return {}\n",
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
