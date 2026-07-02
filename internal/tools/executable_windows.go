//go:build windows

package tools

import (
	"os"
	"path/filepath"
	"strings"
)

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
