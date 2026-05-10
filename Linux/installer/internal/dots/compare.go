package dots

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func sourceSubsetMatches(src, dst string, excludes map[string]bool) (bool, error) {
	if _, err := os.Stat(dst); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	matches := true
	err := filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil || !matches {
			return err
		}
		return sourceSubsetEntryMatches(src, dst, path, entry, excludes, &matches)
	})
	return matches, err
}

func sourceSubsetEntryMatches(src, dst, path string, entry os.DirEntry, excludes map[string]bool, matches *bool) error {
	rel, err := filepath.Rel(src, path)
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
	return compareSourceTargetEntry(path, filepath.Join(dst, filepath.FromSlash(rel)), entry, matches)
}

func compareSourceTargetEntry(source, target string, entry os.DirEntry, matches *bool) error {
	sourceInfo, err := entry.Info()
	if err != nil {
		return err
	}
	targetInfo, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			*matches = false
			return nil
		}
		return err
	}
	if entry.Type()&os.ModeSymlink != 0 {
		return compareSymlink(source, target, matches)
	}
	if entry.IsDir() {
		*matches = targetInfo.IsDir()
		return nil
	}
	if !sourceInfo.Mode().IsRegular() {
		return nil
	}
	if !targetInfo.Mode().IsRegular() || sourceInfo.Size() != targetInfo.Size() {
		*matches = false
		return nil
	}
	return compareRegularFile(source, target, matches)
}

func excludedPath(rel string, excludes map[string]bool) bool {
	if len(excludes) == 0 {
		return false
	}
	if excludes[rel] {
		return true
	}
	for exclude := range excludes {
		if strings.HasPrefix(rel, strings.TrimSuffix(exclude, "/")+"/") {
			return true
		}
	}
	return false
}

func compareSymlink(source, target string, matches *bool) error {
	sourceTarget, err := os.Readlink(source)
	if err != nil {
		return err
	}
	targetTarget, err := os.Readlink(target)
	if err != nil {
		if os.IsNotExist(err) {
			*matches = false
			return nil
		}
		return err
	}
	*matches = sourceTarget == targetTarget
	return nil
}

func compareRegularFile(source, target string, matches *bool) error {
	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() {
		_ = sourceFile.Close()
	}()
	targetFile, err := os.Open(target)
	if err != nil {
		if os.IsNotExist(err) {
			*matches = false
			return nil
		}
		return err
	}
	defer func() {
		_ = targetFile.Close()
	}()
	return streamsMatch(sourceFile, targetFile, matches)
}

func streamsMatch(left, right io.Reader, matches *bool) error {
	leftHash := sha256.New()
	rightHash := sha256.New()
	if _, err := io.Copy(leftHash, left); err != nil {
		return err
	}
	if _, err := io.Copy(rightHash, right); err != nil {
		return err
	}
	*matches = hex.EncodeToString(leftHash.Sum(nil)) == hex.EncodeToString(rightHash.Sum(nil))
	return nil
}
