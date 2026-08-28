package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnd4WrapperUsesXDGConfigAndPreservesCallerRuntimeEnvironmentWithoutCompgen(t *testing.T) {
	testRoot := t.TempDir()
	home := filepath.Join(testRoot, "home")
	xdgConfig := filepath.Join(testRoot, "custom-config")
	fakePackage := filepath.Join(testRoot, "quickshell-package")

	writeEnd4WrapperFixture(t, filepath.Join(home, ".config", "hypr", "scripts", "end4-runtime-env.sh"), hostileEnd4RuntimeEnv("home"))
	writeEnd4WrapperFixture(t, filepath.Join(xdgConfig, "hypr", "scripts", "end4-runtime-env.sh"), hostileEnd4RuntimeEnv("xdg"))
	writeEnd4WrapperFixture(t, filepath.Join(fakePackage, "bin", "qs"), `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' \
  "source=$WAHRWELT_RUNTIME_ENV_SOURCE" \
  "profile=$WAHRWELT_END4_PROFILE" \
  "wahrwelt_qs=$WAHRWELT_QS_CONFIG" \
  "qs=$qsConfig" \
  "dotfiles=$ILLOGICAL_IMPULSE_DOTFILES_SOURCE" \
  "venv=$ILLOGICAL_IMPULSE_VIRTUAL_ENV" \
  "custom=$ILLOGICAL_IMPULSE_CUSTOM_SENTINEL"
`)
	if err := os.Chmod(filepath.Join(fakePackage, "bin", "qs"), 0o755); err != nil {
		t.Fatal(err)
	}

	wrapper := renderEnd4WrapperForTest(t, fakePackage)
	wrapperPath := filepath.Join(testRoot, "qs-end4")
	writeEnd4WrapperFixture(t, wrapperPath, "#!/usr/bin/env bash\nset -euo pipefail\n"+wrapper)
	if err := os.Chmod(wrapperPath, 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", "-c", `enable -n compgen; . "$1"`, "bash", wrapperPath)
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + xdgConfig,
		"WAHRWELT_END4_PROFILE=end4-pc",
		"WAHRWELT_QS_CONFIG=/exact/quickshell/end4-pC",
		"qsConfig=/exact/caller/qs-config",
		"ILLOGICAL_IMPULSE_DOTFILES_SOURCE=/exact/config",
		"ILLOGICAL_IMPULSE_VIRTUAL_ENV=/exact/state/quickshell/.venv",
		"ILLOGICAL_IMPULSE_CUSTOM_SENTINEL=exact-custom",
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("qs-end4 wrapper failed: %v\n%s", err, output)
	}
	const want = `source=xdg
profile=end4-pc
wahrwelt_qs=/exact/quickshell/end4-pC
qs=/exact/caller/qs-config
dotfiles=/exact/config
venv=/exact/state/quickshell/.venv
custom=exact-custom
`
	if string(output) != want {
		t.Fatalf("qs-end4 used the wrong runtime env or overwrote exact caller values:\nwant:\n%s\ngot:\n%s", want, output)
	}
}

func hostileEnd4RuntimeEnv(source string) string {
	return `export WAHRWELT_RUNTIME_ENV_SOURCE=` + source + `
export WAHRWELT_END4_PROFILE=home-default
export WAHRWELT_QS_CONFIG=/home/default/quickshell
export qsConfig=/home/default/qs-config
export ILLOGICAL_IMPULSE_DOTFILES_SOURCE=/home/default/.config
export ILLOGICAL_IMPULSE_VIRTUAL_ENV=/home/default/.local/state/quickshell/.venv
export ILLOGICAL_IMPULSE_CUSTOM_SENTINEL=home-default
`
}

func renderEnd4WrapperForTest(t *testing.T, fakePackage string) string {
	t.Helper()
	module := readContractFile(t, "../../../NixOS/home/end4/quickshell.nix")
	const startToken = `(pkgs.writeShellScriptBin "qs-end4" ''
`
	start := strings.Index(module, startToken)
	if start < 0 {
		t.Fatal("qs-end4 wrapper body is missing")
	}
	start += len(startToken)
	end := strings.Index(module[start:], "\n    '')")
	if end < 0 {
		t.Fatal("qs-end4 wrapper terminator is missing")
	}
	wrapper := module[start : start+end]
	replacements := map[string]string{
		"${end4Lib.runtimeEnv.quickshellExports}": ": # test fallback",
		"${qsPackage}": fakePackage,
	}
	for old, replacement := range replacements {
		if strings.Count(wrapper, old) != 1 {
			t.Fatalf("qs-end4 test renderer expected one %q interpolation", old)
		}
		wrapper = strings.Replace(wrapper, old, replacement, 1)
	}
	return strings.ReplaceAll(wrapper, "''${", "${")
}

func writeEnd4WrapperFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
