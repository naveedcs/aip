//go:build !windows

package shim

import (
	"os"

	"golang.org/x/sys/unix"
)

func executableCandidateNames(binary string) []string {
	return []string{binary}
}

func isExecutableFile(path string, info os.FileInfo) bool {
	return !info.IsDir() && info.Mode().IsRegular() && unix.Access(path, unix.X_OK) == nil
}
