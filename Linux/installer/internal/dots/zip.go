package dots

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	sineBootloaderURL = "https://github.com/sineorg/bootloader/releases/download/v0.1.4/profile.zip"
	sineEngineURL     = "https://github.com/CosmoCreeper/Sine/releases/download/v2.3/engine.zip"
	sineLocalesURL    = "https://github.com/CosmoCreeper/Sine/releases/download/v2.3/locales.zip"
	sineProfileSHA256 = "285b3d589cc979f11f01c9c77410b717694ccc4f32cc1cb08bd6d8909fb98e00"
	sineEngineSHA256  = "5892add04ab4cf808018d8982495d53029de0b5cd62d80ea6905a741cf897bfd"
	sineLocalesSHA256 = "f7d269d86738cef4635d61e035795655326aabc6cacc71e97f8acae17e7cc57f"
	maxZipEntryBytes  = 128 * 1024 * 1024
)

func verifyFileSHA256(path, want string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return err
	}
	got := hex.EncodeToString(sum.Sum(nil))
	if got != want {
		return fmt.Errorf("sha256 mismatch for %s: got %s want %s", path, got, want)
	}
	return nil
}

func safeExtractZip(zipPath, dest string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = reader.Close()
	}()
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	for _, file := range reader.File {
		if err := safeExtractZipFile(file, destAbs); err != nil {
			return err
		}
	}
	return nil
}

func safeExtractZipFile(file *zip.File, destAbs string) error {
	if file.FileInfo().Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to extract symlink from zip: %s", file.Name)
	}
	if file.UncompressedSize64 > maxZipEntryBytes {
		return fmt.Errorf("refusing oversized zip entry: %s", file.Name)
	}
	target, err := safeZipTarget(destAbs, file.Name)
	if err != nil {
		return err
	}
	if err := ensureNoSymlinkInPath(destAbs, target); err != nil {
		return err
	}
	if file.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	source, err := file.Open()
	if err != nil {
		return err
	}
	defer func() {
		_ = source.Close()
	}()
	mode := file.FileInfo().Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	targetFile, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer func() {
		_ = targetFile.Close()
	}()
	written, err := io.Copy(targetFile, io.LimitReader(source, maxZipEntryBytes+1))
	if err != nil {
		return err
	}
	if written > maxZipEntryBytes {
		return fmt.Errorf("refusing oversized zip entry: %s", file.Name)
	}
	return nil
}

func ensureNoSymlinkInPath(destAbs, targetAbs string) error {
	rel, err := filepath.Rel(destAbs, targetAbs)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	current := destAbs
	parts := strings.Split(rel, string(os.PathSeparator))
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to extract through existing symlink: %s", current)
		}
		if i < len(parts)-1 && !info.IsDir() {
			return fmt.Errorf("refusing to extract through non-directory path: %s", current)
		}
	}
	return nil
}

func safeZipTarget(destAbs, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) {
		return "", fmt.Errorf("refusing unsafe zip path: %s", name)
	}
	target := filepath.Join(destAbs, filepath.FromSlash(name))
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(destAbs, targetAbs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("refusing zip path outside destination: %s", name)
	}
	return targetAbs, nil
}
