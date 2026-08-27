//go:build linux

package config

import (
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

const mutableSeedHelperSource = "../../../NixOS/home/lib/mutable-seed.py"

func mutableSeedCommand(arguments ...string) *exec.Cmd {
	return exec.Command("python3", append([]string{mutableSeedHelperSource}, arguments...)...)
}

func writeMutableSeedWinner(t *testing.T, target, kind string, content []byte) {
	t.Helper()
	switch kind {
	case "regular":
		if err := os.WriteFile(target, content, 0o600); err != nil {
			t.Fatal(err)
		}
	case "symlink":
		if err := os.Symlink("winner-target", target); err != nil {
			t.Fatal(err)
		}
	case "directory":
		if err := os.Mkdir(target, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(target, "winner"), content, 0o600); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unknown winner kind %q", kind)
	}
}

func assertMutableSeedWinner(t *testing.T, target, kind string, content []byte) {
	t.Helper()
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	switch kind {
	case "regular":
		got, readErr := os.ReadFile(target)
		if readErr != nil || string(got) != string(content) || !info.Mode().IsRegular() {
			t.Fatalf("regular winner changed: mode=%v bytes=%q err=%v", info.Mode(), got, readErr)
		}
	case "symlink":
		got, readErr := os.Readlink(target)
		if readErr != nil || got != "winner-target" {
			t.Fatalf("symlink winner changed: target=%q err=%v", got, readErr)
		}
	case "directory":
		got, readErr := os.ReadFile(filepath.Join(target, "winner"))
		if readErr != nil || string(got) != string(content) || !info.IsDir() {
			t.Fatalf("directory winner changed: mode=%v bytes=%q err=%v", info.Mode(), got, readErr)
		}
	}
}

func TestMutableSeedFilePreservesConcurrentWinners(t *testing.T) {
	for _, kind := range []string{"regular", "symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source")
			target := filepath.Join(root, "target")
			winner := []byte("concurrent winner\n")
			if err := os.WriteFile(source, []byte("canonical\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			output, err := runAtFDBarrier(
				t,
				mutableSeedCommand("seed-file", root, target, source, ""),
				"WAHRWELT_TEST_MUTABLE_SEED_READY_FD",
				"WAHRWELT_TEST_MUTABLE_SEED_CONTINUE_FD",
				func() { writeMutableSeedWinner(t, target, kind, winner) },
			)
			if err == nil || !strings.Contains(output, "concurrent winner appeared") {
				t.Fatalf("concurrent %s winner accepted: err=%v\n%s", kind, err, output)
			}
			assertMutableSeedWinner(t, target, kind, winner)
			candidates, globErr := filepath.Glob(filepath.Join(root, ".wahrwelt-seed-file-*"))
			if globErr != nil || len(candidates) != 1 {
				t.Fatalf("uncertain file candidate was not retained: %v err=%v", candidates, globErr)
			}
		})
	}
}

func TestMutableSeedRetainsChangedPrivateFileCandidate(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "target")
	if err := os.WriteFile(source, []byte("canonical\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := runAtFDBarrier(
		t,
		mutableSeedCommand("seed-file", root, target, source, ""),
		"WAHRWELT_TEST_MUTABLE_SEED_READY_FD",
		"WAHRWELT_TEST_MUTABLE_SEED_CONTINUE_FD",
		func() {
			candidates, globErr := filepath.Glob(filepath.Join(root, ".wahrwelt-seed-file-*"))
			if globErr != nil || len(candidates) != 1 {
				t.Fatalf("private candidate unavailable: %v err=%v", candidates, globErr)
			}
			if writeErr := os.WriteFile(candidates[0], []byte("candidate intruder\n"), 0o600); writeErr != nil {
				t.Fatal(writeErr)
			}
			writeMutableSeedWinner(t, target, "regular", []byte("public winner\n"))
		},
	)
	if err == nil || !strings.Contains(output, "recovery retained after candidate") {
		t.Fatalf("changed private candidate was deleted: err=%v\n%s", err, output)
	}
	assertMutableSeedWinner(t, target, "regular", []byte("public winner\n"))
	candidates, globErr := filepath.Glob(filepath.Join(root, ".wahrwelt-seed-file-*"))
	if globErr != nil || len(candidates) != 1 {
		t.Fatalf("changed private candidate not retained: %v err=%v", candidates, globErr)
	}
	if got, readErr := os.ReadFile(candidates[0]); readErr != nil || string(got) != "candidate intruder\n" {
		t.Fatalf("retained private candidate changed: %q err=%v", got, readErr)
	}
}

func TestMutableSeedTreePreservesWinnersAndRetainsPrivateTree(t *testing.T) {
	for _, kind := range []string{"regular", "symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source")
			target := filepath.Join(root, "target")
			if err := os.Mkdir(source, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, "seeded"), []byte("canonical\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			winner := []byte("tree winner\n")
			output, err := runAtFDBarrier(
				t,
				mutableSeedCommand("seed-tree", root, target, source),
				"WAHRWELT_TEST_MUTABLE_SEED_READY_FD",
				"WAHRWELT_TEST_MUTABLE_SEED_CONTINUE_FD",
				func() {
					candidates, globErr := filepath.Glob(filepath.Join(root, ".wahrwelt-seed-tree-*"))
					if globErr != nil || len(candidates) != 1 {
						t.Fatalf("private tree unavailable: %v err=%v", candidates, globErr)
					}
					if writeErr := os.WriteFile(filepath.Join(candidates[0], "untrusted"), []byte("preserve me\n"), 0o600); writeErr != nil {
						t.Fatal(writeErr)
					}
					writeMutableSeedWinner(t, target, kind, winner)
				},
			)
			if err == nil || !strings.Contains(output, "recovery retained") {
				t.Fatalf("concurrent tree winner accepted: err=%v\n%s", err, output)
			}
			assertMutableSeedWinner(t, target, kind, winner)
			candidates, globErr := filepath.Glob(filepath.Join(root, ".wahrwelt-seed-tree-*"))
			if globErr != nil || len(candidates) != 1 {
				t.Fatalf("changed private tree not retained: %v err=%v", candidates, globErr)
			}
			got, readErr := os.ReadFile(filepath.Join(candidates[0], "untrusted"))
			if readErr != nil || string(got) != "preserve me\n" {
				t.Fatalf("private tree content changed: %q err=%v", got, readErr)
			}
		})
	}
}

func writeFakeJQ(t *testing.T, root, output string) string {
	t.Helper()
	path := filepath.Join(root, "jq")
	program := "#!/bin/sh\nprintf '%s\\n' '" + output + "'\n"
	if err := os.WriteFile(path, []byte(program), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMutableJSONFreshSeedTransformsBeforePublication(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "config.json")
	source := filepath.Join(root, "canonical.json")
	if err := os.WriteFile(source, []byte("{\"base\":true}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	jq := writeFakeJQ(t, root, `{"base":true,"default":true}`)
	output, err := mutableSeedCommand(
		"seed-json-object", root, target, source, "", jq, ".default = true",
	).CombinedOutput()
	if err != nil {
		t.Fatalf("fresh JSON seed failed: %v\n%s", err, output)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "{\"base\":true,\"default\":true}\n" {
		t.Fatalf("fresh transformed JSON = %q", got)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("fresh JSON mode = %s, want 0644", info.Mode())
	}
}

func TestMutableJSONExistingTransformFailsWithoutMutation(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "config.json")
	source := filepath.Join(root, "canonical.json")
	existing := []byte("{\"owner\":\"user\"}\n")
	if err := os.WriteFile(target, existing, 0o440); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	jq := writeFakeJQ(t, root, `{"owner":"user","managed":true}`)
	output, runErr := mutableSeedCommand(
		"seed-json-object", root, target, source, "", jq, ".managed = true",
	).CombinedOutput()
	if runErr == nil || !strings.Contains(string(output), "was preserved") {
		t.Fatalf("existing JSON transform was accepted: err=%v\n%s", runErr, output)
	}
	after, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	beforeSys := before.Sys().(*syscall.Stat_t)
	afterSys := after.Sys().(*syscall.Stat_t)
	if beforeSys.Dev != afterSys.Dev || beforeSys.Ino != afterSys.Ino || before.Mode() != after.Mode() || string(got) != string(existing) {
		t.Fatalf("existing JSON changed: inode=%d:%d -> %d:%d mode=%v -> %v bytes=%q", beforeSys.Dev, beforeSys.Ino, afterSys.Dev, afterSys.Ino, before.Mode(), after.Mode(), got)
	}
}

func TestMutableJSONExistingValidationPreservesConcurrentWinners(t *testing.T) {
	for _, kind := range []string{"in-place", "regular", "symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, "config.json")
			displaced := filepath.Join(root, "config-before-race.json")
			source := filepath.Join(root, "canonical.json")
			original := []byte("{\"owner\":\"user\"}\n")
			winner := []byte("{\"winner\":true}\n")
			if err := os.WriteFile(target, original, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(source, []byte("{}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(target)
			if err != nil {
				t.Fatal(err)
			}
			output, waitErr := runAtFDBarrier(
				t,
				mutableSeedCommand("seed-json-object", root, target, source, "", "unused-jq", ""),
				"WAHRWELT_TEST_MUTABLE_SEED_READY_FD",
				"WAHRWELT_TEST_MUTABLE_SEED_CONTINUE_FD",
				func() {
					if kind == "in-place" {
						file, openErr := os.OpenFile(target, os.O_WRONLY|os.O_TRUNC, 0)
						if openErr != nil {
							t.Fatal(openErr)
						}
						if _, writeErr := file.Write(winner); writeErr != nil {
							_ = file.Close()
							t.Fatal(writeErr)
						}
						if closeErr := file.Close(); closeErr != nil {
							t.Fatal(closeErr)
						}
						return
					}
					if renameErr := os.Rename(target, displaced); renameErr != nil {
						t.Fatal(renameErr)
					}
					writeMutableSeedWinner(t, target, kind, winner)
				},
			)
			if waitErr == nil {
				t.Fatalf("existing JSON %s race accepted: err=%v\n%s", kind, waitErr, output)
			}
			if kind == "in-place" {
				after, statErr := os.Stat(target)
				if statErr != nil {
					t.Fatal(statErr)
				}
				beforeSys := before.Sys().(*syscall.Stat_t)
				afterSys := after.Sys().(*syscall.Stat_t)
				if beforeSys.Dev != afterSys.Dev || beforeSys.Ino != afterSys.Ino {
					t.Fatalf("in-place winner inode changed: %d:%d -> %d:%d", beforeSys.Dev, beforeSys.Ino, afterSys.Dev, afterSys.Ino)
				}
				got, readErr := os.ReadFile(target)
				if readErr != nil || string(got) != string(winner) {
					t.Fatalf("in-place winner changed: %q err=%v", got, readErr)
				}
				return
			}
			assertMutableSeedWinner(t, target, kind, winner)
			got, readErr := os.ReadFile(displaced)
			if readErr != nil || string(got) != string(original) {
				t.Fatalf("displaced original changed: %q err=%v", got, readErr)
			}
		})
	}
}

func TestMutableJSONFreshPublicationPreservesConcurrentWinners(t *testing.T) {
	for _, kind := range []string{"regular", "symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, "config.json")
			source := filepath.Join(root, "canonical.json")
			winner := []byte("fresh winner\n")
			if err := os.WriteFile(source, []byte("{}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			output, waitErr := runAtFDBarrier(
				t,
				mutableSeedCommand("seed-json-object", root, target, source, "", "unused-jq", ""),
				"WAHRWELT_TEST_MUTABLE_SEED_READY_FD",
				"WAHRWELT_TEST_MUTABLE_SEED_CONTINUE_FD",
				func() { writeMutableSeedWinner(t, target, kind, winner) },
			)
			if waitErr == nil || !strings.Contains(output, "concurrent winner appeared") {
				t.Fatalf("fresh JSON %s winner accepted: err=%v\n%s", kind, waitErr, output)
			}
			assertMutableSeedWinner(t, target, kind, winner)
			candidates, globErr := filepath.Glob(filepath.Join(root, ".wahrwelt-seed-file-*"))
			if globErr != nil || len(candidates) != 1 {
				t.Fatalf("uncertain JSON candidate was not retained: %v err=%v", candidates, globErr)
			}
		})
	}
}

func TestMutableSeedConsumersPreserveUnknownUserState(t *testing.T) {
	root := t.TempDir()
	walls := filepath.Join(root, "Pictures", "Wallpapers")
	nested := filepath.Join(walls, "nested")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	preview := filepath.Join(walls, "preview-user.png")
	readonly := filepath.Join(nested, "readonly-user.png")
	if err := os.WriteFile(preview, []byte("preview\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(readonly, []byte("nested\n"), 0o440); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(readonly)
	if err != nil {
		t.Fatal(err)
	}
	beforeBytes, err := os.ReadFile(readonly)
	if err != nil {
		t.Fatal(err)
	}
	beforeHash := sha256.Sum256(beforeBytes)
	source := filepath.Join(root, "wallpaper-source")
	if err := os.WriteFile(source, []byte("canonical wallpaper\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := mutableSeedCommand("ensure-dir", root, walls).CombinedOutput(); err != nil {
		t.Fatalf("ensure wallpaper directory: %v\n%s", err, output)
	}
	if output, err := mutableSeedCommand("seed-file", root, filepath.Join(walls, "1.jpg"), source, "").CombinedOutput(); err != nil {
		t.Fatalf("seed wallpaper: %v\n%s", err, output)
	}
	after, err := os.Stat(readonly)
	if err != nil {
		t.Fatal(err)
	}
	afterBytes, err := os.ReadFile(readonly)
	if err != nil {
		t.Fatal(err)
	}
	afterHash := sha256.Sum256(afterBytes)
	beforeSys := before.Sys().(*syscall.Stat_t)
	afterSys := after.Sys().(*syscall.Stat_t)
	if beforeSys.Ino != afterSys.Ino || before.Mode() != after.Mode() || beforeHash != afterHash {
		t.Fatalf("unknown nested wallpaper changed: inode=%d->%d mode=%v->%v hash=%x->%x", beforeSys.Ino, afterSys.Ino, before.Mode(), after.Mode(), beforeHash, afterHash)
	}
	if got, readErr := os.ReadFile(preview); readErr != nil || string(got) != "preview\n" {
		t.Fatalf("unknown preview changed: %q err=%v", got, readErr)
	}
}

func TestMutableSeedContractsUseOnlyAtomicPublications(t *testing.T) {
	helper := readContractFile(t, mutableSeedHelperSource)
	for _, want := range []string{
		"RENAME_NOREPLACE",
		"info.st_mtime_ns",
		"info.st_ctime_ns",
		"hashlib.sha256(value).digest()",
		"os.pread",
		"seed-file-replace-line",
		"seed-json-object",
		"existing JSON requires a managed transform and was preserved",
	} {
		if !strings.Contains(helper, want) {
			t.Fatalf("mutable seed helper missing %q\n%s", want, helper)
		}
	}
	for _, forbidden := range []string{
		"RENAME_EXCHANGE",
		"os.replace",
		"os.rename",
		"os.unlink",
		"shutil",
		"remove_tree_contents",
		"ensure-json-object",
		"apply-jq",
	} {
		if strings.Contains(helper, forbidden) {
			t.Fatalf("mutable seed helper retained path-based mutation %q\n%s", forbidden, helper)
		}
	}
	dotfiles := readContractFile(t, "../../../NixOS/home/lib/dotfiles.nix")
	for _, want := range []string{"wahrwelt-mutable-seed", "seed-file", "seed-tree", "seed-json-object"} {
		if !strings.Contains(dotfiles, want) {
			t.Fatalf("dotfiles helper is not a thin mutable seed wrapper: missing %q\n%s", want, dotfiles)
		}
	}
	for _, forbidden := range []string{"ensure-json-object", "apply-jq"} {
		if strings.Contains(dotfiles, forbidden) {
			t.Fatalf("dotfiles helper retained live JSON mutation %q\n%s", forbidden, dotfiles)
		}
	}

	wallpapers := readContractFile(t, "../../../NixOS/home/home.nix")
	for _, want := range []string{"wallpaperNames", "seed_if_missing", "ensure_real_directory"} {
		if !strings.Contains(wallpapers, want) {
			t.Fatalf("wallpaper seed missing %q\n%s", want, wallpapers)
		}
	}
	for _, forbidden := range []string{"preview-*", "chmod -R", "cp -n", "find \"$WALLS_DST\""} {
		if strings.Contains(wallpapers, forbidden) {
			t.Fatalf("wallpaper seed retained broad mutation %q\n%s", forbidden, wallpapers)
		}
	}

	noctalia := readContractFile(t, "../../../NixOS/home/noctalia/default.nix")
	for _, want := range []string{"ensure_real_directory", "seed_if_missing"} {
		if !strings.Contains(noctalia, want) {
			t.Fatalf("Noctalia colorscheme seed missing %q\n%s", want, noctalia)
		}
	}
	for _, forbidden := range []string{"mkdir -p \"${targetDirExpr}\"", "install -m 644 \"${storeFile}\""} {
		if strings.Contains(noctalia, forbidden) {
			t.Fatalf("Noctalia colorscheme seed retained racy mutation %q\n%s", forbidden, noctalia)
		}
	}

	apps := readContractFile(t, "../../../NixOS/home/end4/seed/apps.nix")
	for _, want := range []string{"seed_with_replaced_line_if_missing", `"Icon="`, `"Icon=$rfdump_icon"`} {
		if !strings.Contains(apps, want) {
			t.Fatalf("End4 rfdump seed missing %q\n%s", want, apps)
		}
	}
	for _, forbidden := range []string{"sed -i", "install -m 644", `[ ! -e "$rfdump_desktop" ]`} {
		if strings.Contains(apps, forbidden) {
			t.Fatalf("End4 rfdump seed retained racy mutation %q\n%s", forbidden, apps)
		}
	}
}

func TestMutableSeedReplacesDesktopLineBeforePublication(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.desktop")
	target := filepath.Join(root, "target.desktop")
	if err := os.WriteFile(source, []byte("[Desktop Entry]\nIcon=old\nName=RFdump\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := mutableSeedCommand(
		"seed-file-replace-line",
		root,
		target,
		source,
		"Icon=",
		"Icon=/exact/rfdump.png",
	).CombinedOutput()
	if err != nil {
		t.Fatalf("desktop seed failed: %v\n%s", err, output)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	want := "[Desktop Entry]\nIcon=/exact/rfdump.png\nName=RFdump\n"
	if string(got) != want {
		t.Fatalf("desktop seed = %q, want %q", got, want)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("desktop seed mode = %s, want 0644", info.Mode())
	}
}
