//go:build windows

package shim

import (
	"os"
	"path/filepath"
	"strings"
)

func executableCandidateNames(binary string) []string {
	if filepath.Ext(binary) != "" {
		return []string{binary}
	}
	names := []string{binary}
	for _, ext := range windowsPathExts() {
		names = append(names, binary+ext)
	}
	return names
}

func isExecutableFile(path string, info os.FileInfo) bool {
	if info.IsDir() || !info.Mode().IsRegular() {
		return false
	}
	ext := filepath.Ext(path)
	for _, allowed := range windowsPathExts() {
		if strings.EqualFold(ext, allowed) {
			return true
		}
	}
	return false
}

func windowsPathExts() []string {
	raw := os.Getenv("PATHEXT")
	if strings.TrimSpace(raw) == "" {
		raw = ".COM;.EXE;.BAT;.CMD"
	}
	seen := map[string]bool{}
	exts := []string{}
	for _, ext := range strings.Split(raw, ";") {
		ext = strings.TrimSpace(ext)
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		key := strings.ToLower(ext)
		if seen[key] {
			continue
		}
		seen[key] = true
		exts = append(exts, ext)
	}
	return exts
}
