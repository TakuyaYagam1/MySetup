package doctor

import (
	"os"
	"strings"
)

func check(out *reportWriter, label, path string) {
	if _, err := os.Stat(path); err == nil {
		out.printf("OK   %s: %s\n", label, path)
		return
	}
	out.printf("WARN %s missing: %s\n", label, path)
}

func checkRegularFile(out *reportWriter, label, path string) {
	info, err := os.Stat(path)
	if err != nil {
		out.printf("WARN %s missing: %s\n", label, path)
		return
	}
	if !info.Mode().IsRegular() {
		out.printf("WARN %s is not a regular file: %s\n", label, path)
		return
	}
	file, err := os.Open(path)
	if err != nil {
		out.printf("WARN %s unreadable: %s\n", label, path)
		return
	}
	if err := file.Close(); err != nil {
		out.printf("WARN %s unreadable: %s\n", label, path)
		return
	}
	out.printf("OK   %s: %s\n", label, path)
}

func checkAny(out *reportWriter, label string, paths ...string) {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			out.printf("OK   %s: %s\n", label, path)
			return
		}
	}
	if len(paths) == 0 {
		out.printf("WARN %s missing\n", label)
		return
	}
	out.printf("WARN %s missing: %s\n", label, paths[0])
}

func checkDirectory(out *reportWriter, label, path string) {
	info, err := os.Stat(path)
	if err != nil {
		out.printf("WARN %s missing: %s\n", label, path)
		return
	}
	if !info.IsDir() {
		out.printf("WARN %s is not a directory: %s\n", label, path)
		return
	}
	out.printf("OK   %s: %s\n", label, path)
}

func checkExecutable(out *reportWriter, label, path string) {
	info, err := os.Stat(path)
	if err != nil {
		out.printf("WARN %s missing: %s\n", label, path)
		return
	}
	if info.Mode().Perm()&0o111 == 0 {
		out.printf("WARN %s not executable: %s\n", label, path)
		return
	}
	out.printf("OK   %s executable: %s\n", label, path)
}

func checkWritableFile(out *reportWriter, label, path string) {
	info, err := os.Stat(path)
	if err != nil {
		out.printf("WARN %s missing: %s\n", label, path)
		return
	}
	if info.IsDir() {
		out.printf("WARN %s is a directory, expected file: %s\n", label, path)
		return
	}
	if info.Mode().Perm()&0o200 == 0 {
		out.printf("WARN %s not writable: %s\n", label, path)
		return
	}
	out.printf("OK   %s writable: %s\n", label, path)
}

func checkOptionalFile(out *reportWriter, label, path string) {
	if _, err := os.Stat(path); err == nil {
		out.printf("OK   %s: %s\n", label, path)
		return
	}
	out.printf("WARN %s missing (created on first shell run): %s\n", label, path)
}

func checkRuntimeConfig(out *reportWriter, label, path, wantFragment string) {
	data, err := os.ReadFile(path)
	if err != nil {
		out.printf("WARN %s missing: %s\n", label, path)
		return
	}
	if strings.Contains(string(data), wantFragment) {
		out.printf("OK   %s: %s\n", label, path)
		return
	}
	out.printf("WARN %s does not point at %q: %s\n", label, wantFragment, path)
}

func readSymlink(path string) (string, bool) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return "", false
	}
	target, err := os.Readlink(path)
	if err != nil {
		return "", false
	}
	return target, true
}
