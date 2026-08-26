package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

type Options struct {
	RepoRoot  string
	NixOSDest string
	StatePath string
	DraftPath string
}

type Sources struct {
	RepoRoot  string
	NixOS     string
	Dots      string
	Installer string
}

const (
	DefaultStatePath        = "/etc/nixos/installer-state.json"
	LegacyWahrweltStatePath = "/etc/nixos/wahrwelt/state.json"
	LegacyMySetupStatePath  = "/etc/nixos/mysetup/state.json"
	LegacyStatePath         = LegacyMySetupStatePath
)

func DefaultOptions() Options {
	home, _ := os.UserHomeDir()
	stateHome := xdgStateHome(home, true)
	return Options{
		NixOSDest: "/etc/nixos",
		StatePath: DefaultStatePath,
		DraftPath: filepath.Join(stateHome, "wahrwelt", "draft.json"),
	}
}

func XDGStateHome(home string) string {
	return xdgStateHome(home, false)
}

func xdgStateHome(home string, preferEnv bool) string {
	if stateHome := os.Getenv("XDG_STATE_HOME"); stateHome != "" && (preferEnv || home == "") {
		return stateHome
	}
	if home != "" {
		return filepath.Join(home, ".local", "state")
	}
	return ""
}

func ActiveShellStatePath(home string) string {
	return filepath.Join(XDGStateHome(home), "wahrwelt", "active-shell")
}

func LegacyDraftPath(home string) string {
	return filepath.Join(XDGStateHome(home), "mysetup", "draft.json")
}

func LegacyActiveShellStatePath(home string) string {
	return filepath.Join(XDGStateHome(home), "mysetup", "active-shell")
}

func ExistingFile(primary, legacy string) string {
	if exists(primary) || legacy == "" || !exists(legacy) {
		return primary
	}
	return legacy
}

func (o Options) ExistingStatePath() (string, error) {
	if o.StatePath != DefaultStatePath {
		return existingStatePath(o.StatePath)
	}
	return existingStatePath(o.StatePath, LegacyWahrweltStatePath, LegacyMySetupStatePath)
}

func existingStatePath(canonical string, fallbacks ...string) (string, error) {
	for _, candidate := range append([]string{canonical}, fallbacks...) {
		exists, err := regularStateFile(candidate)
		if err != nil {
			return "", err
		}
		if exists {
			return candidate, nil
		}
	}
	return canonical, nil
}

func regularStateFile(path string) (bool, error) {
	if err := ordinaryStateParent(path); err != nil {
		return false, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("unsupported state path: %s", path)
	}
	return true, nil
}

func ordinaryStateParent(path string) error {
	for parent := filepath.Clean(filepath.Dir(path)); ; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return fmt.Errorf("unsupported state parent path: %s", parent)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		next := filepath.Dir(parent)
		if next == parent {
			return nil
		}
	}
}

func (o Options) ExistingDraftPath() string {
	home, _ := os.UserHomeDir()
	defaultDraft := filepath.Join(XDGStateHome(home), "wahrwelt", "draft.json")
	if o.DraftPath != defaultDraft {
		return o.DraftPath
	}
	return ExistingFile(o.DraftPath, LegacyDraftPath(home))
}

func ResolveSources(repoRoot string) (Sources, error) {
	candidates := make([]string, 0, 4)
	if repoRoot != "" {
		src, ok := resolveCandidate(repoRoot)
		if ok {
			return src, nil
		}
		return Sources{}, fmt.Errorf("explicit repository root is not a Wahrwelt source tree: %s", repoRoot)
	}
	for _, name := range []string{"WAHRWELT_REPO_ROOT", "MYSETUP_REPO_ROOT"} {
		if env := os.Getenv(name); env != "" {
			candidates = append(candidates, env)
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		for dir := cwd; ; dir = filepath.Dir(dir) {
			candidates = append(candidates, dir)
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if src, ok := resolveCandidate(candidate); ok {
			return src, nil
		}
	}
	return Sources{}, fmt.Errorf("could not locate repository root; run from Wahrwelt root or pass --repo")
}

func resolveCandidate(candidate string) (Sources, bool) {
	repoSrc := Sources{
		RepoRoot:  candidate,
		NixOS:     filepath.Join(candidate, "Linux", "NixOS"),
		Dots:      filepath.Join(candidate, "Linux", "dots"),
		Installer: filepath.Join(candidate, "Linux", "installer"),
	}
	if exists(filepath.Join(repoSrc.NixOS, "flake.nix")) && exists(repoSrc.Dots) && exists(repoSrc.Installer) {
		return repoSrc, true
	}
	if !exists(filepath.Join(candidate, "flake.nix")) {
		return Sources{}, false
	}
	dots := filepath.Join(candidate, "dots")
	if !exists(dots) {
		dots = filepath.Join(filepath.Dir(candidate), "dots")
	}
	installer := filepath.Join(candidate, "installer")
	if !exists(installer) {
		installer = filepath.Join(filepath.Dir(candidate), "installer")
	}
	if !exists(dots) || !exists(installer) {
		return Sources{}, false
	}
	return Sources{RepoRoot: candidate, NixOS: candidate, Dots: dots, Installer: installer}, true
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
