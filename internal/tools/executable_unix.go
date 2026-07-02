//go:build !windows

package tools

import (
	"os"

	"golang.org/x/sys/unix"
)

func isExecutableFile(path string, info os.FileInfo) bool {
	return !info.IsDir() && info.Mode().IsRegular() && unix.Access(path, unix.X_OK) == nil
}
