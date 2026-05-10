package dots

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func sourceTreeHash(root string, excludes map[string]bool) (string, error) {
	sum := sha256.New()
	if _, err := fmt.Fprintf(sum, "tree:%s\n", filepath.Base(root)); err != nil {
		return "", err
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if excludedPath(rel, excludes) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(sum, "symlink\t%s\t%s\n", rel, target)
			return err
		}
		if entry.IsDir() {
			_, err = fmt.Fprintf(sum, "dir\t%s\n", rel)
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if _, err := fmt.Fprintf(sum, "file\t%s\t%d\n", rel, info.Size()); err != nil {
			return err
		}
		return hashRegularFile(sum, path)
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

func hashRegularFile(sum io.Writer, path string) error {
	// #nosec G304,G122 -- trusted local installer tree.
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	_, err = io.Copy(sum, file)
	return err
}
