package shim

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const executableAccess = 0x1

func ResolveReal(binary, shimDir string) (string, error) {
	shimKey := canonicalDir(shimDir)
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			dir = "."
		}

		cleanDir := filepath.Clean(dir)
		if sameDir(cleanDir, shimKey) {
			continue
		}

		candidate := filepath.Join(cleanDir, binary)
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if info.IsDir() || syscall.Access(candidate, executableAccess) != nil {
			continue
		}
		if isGeneratedAIPShim(candidate, binary) {
			continue
		}

		abs, err := filepath.Abs(candidate)
		if err != nil {
			return "", err
		}
		return abs, nil
	}

	return "", fmt.Errorf("%q not found outside the aip shim directory", binary)
}

func isGeneratedAIPShim(path, toolID string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return bytes.Equal(content, []byte(ShimContent(toolID)))
}

func sameDir(dir string, shimKey dirKey) bool {
	key := canonicalDir(dir)
	if key.path != "" && key.path == shimKey.path {
		return true
	}
	return key.abs != "" && key.abs == shimKey.abs
}

type dirKey struct {
	path string
	abs  string
}

func canonicalDir(dir string) dirKey {
	abs, absErr := filepath.Abs(filepath.Clean(dir))
	evaluated, evalErr := filepath.EvalSymlinks(abs)
	if evalErr == nil {
		return dirKey{path: filepath.Clean(evaluated), abs: abs}
	}
	if absErr == nil {
		return dirKey{path: abs, abs: abs}
	}
	return dirKey{path: filepath.Clean(dir)}
}
