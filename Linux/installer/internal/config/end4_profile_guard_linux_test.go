//go:build linux

package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestEnd4ProfileGuardAcceptsOnlyContractedNestedHomeManagerStoreSymlink(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skipf("nix is unavailable: %v", err)
	}

	validContract := "end4-adapter-v1\n"
	wrongContract := "unknown-adapter\n"
	for _, fixture := range []struct {
		name     string
		contract *string
		wantOK   bool
	}{
		{name: "exact-contract", contract: &validContract, wantOK: true},
		{name: "missing-contract"},
		{name: "wrong-contract", contract: &wrongContract},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			artifact := filepath.Join(t.TempDir(), "end4-hypr-validated")
			if err := os.MkdirAll(artifact, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(artifact, "hyprland.lua"), []byte("-- managed End4\n"), 0o444); err != nil {
				t.Fatal(err)
			}
			if fixture.contract != nil {
				if err := os.WriteFile(filepath.Join(artifact, ".wahrwelt-runtime-contract"), []byte(*fixture.contract), 0o444); err != nil {
					t.Fatal(err)
				}
			}
			artifactStore := addNixStorePath(t, "end4-hypr-validated", artifact)

			filesTree := filepath.Join(t.TempDir(), "home-manager-files")
			managedLeaf := filepath.Join(filesTree, ".config", "hypr", "end4")
			if err := os.MkdirAll(filepath.Dir(managedLeaf), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(artifactStore, managedLeaf); err != nil {
				t.Fatal(err)
			}
			filesStore := addNixStorePath(t, "home-manager-files", filesTree)
			managedTarget := filepath.Join(filesStore, ".config", "hypr", "end4")

			home := t.TempDir()
			hyprDir := filepath.Join(home, ".config", "hypr")
			if err := os.MkdirAll(hyprDir, 0o755); err != nil {
				t.Fatal(err)
			}
			end4Link := filepath.Join(hyprDir, "end4")
			if err := os.Symlink(managedTarget, end4Link); err != nil {
				t.Fatal(err)
			}

			cmd := exec.Command("bash", "-c", `
set -euo pipefail
profile=end4
hypr_dir() { printf '%s' "$WAHRWELT_TEST_HYPR_DIR"; }
wahrwelt_shell_family() { printf '%s' end4; }
log() { printf '%s\n' "$*" >&2; }
. "$WAHRWELT_TEST_PROFILE_SYNC"
validate_end4_profile_tree
`)
			cmd.Env = append(os.Environ(),
				"HOME="+home,
				"WAHRWELT_TEST_HYPR_DIR="+hyprDir,
				"WAHRWELT_TEST_PROFILE_SYNC=../../../dots/hypr/scripts/shell-profile-sync.sh",
			)
			output, err := cmd.CombinedOutput()
			if fixture.wantOK && err != nil {
				t.Fatalf("contracted Home Manager End4 symlink rejected: %v\n%s", err, output)
			}
			if !fixture.wantOK && err == nil {
				t.Fatal("uncontracted Home Manager End4 symlink accepted")
			}
			if got, err := os.Readlink(end4Link); err != nil || got != managedTarget {
				t.Fatalf("managed End4 link changed: target=%q err=%v", got, err)
			}
		})
	}
}
