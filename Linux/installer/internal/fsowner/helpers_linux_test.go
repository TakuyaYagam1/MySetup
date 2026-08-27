//go:build linux

package fsowner

import (
	"os"
	"path/filepath"
	"testing"
)

func privateTempDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func mustWritePrivate(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertNoRuntimeResidue(t *testing.T, root string) {
	t.Helper()
	assertNoGlob(t, root, ".runtime-rollback-*")
	assertNoGlob(t, root, ".state-switch-rollback-*")
	assertNoGlob(t, root, ".wahrwelt-runtime-stage-*")
}

func assertNoGlob(t *testing.T, root, pattern string) {
	t.Helper()
	var matches []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != root {
			matched, matchErr := filepath.Match(pattern, entry.Name())
			if matchErr != nil {
				return matchErr
			}
			if matched {
				matches = append(matches, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("residue %s = %v", pattern, matches)
	}
}
