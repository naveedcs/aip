package shim

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/naveedcs/aip/internal/fsutil"
	"github.com/naveedcs/aip/internal/paths"
	"github.com/naveedcs/aip/internal/tools"
)

const shimPerm os.FileMode = 0o755

func ShimContent(toolID string) string {
	return "#!/bin/sh\nexec aip shim-exec " + toolID + " \"$@\"\n"
}

func Generate(p paths.Paths) error {
	for _, tool := range tools.All() {
		path := filepath.Join(p.ShimsDir, tool.Binary)
		content := []byte(ShimContent(string(tool.ID)))
		if same, err := sameFileContent(path, content); err != nil {
			return err
		} else if same {
			info, err := os.Stat(path)
			if err != nil {
				return err
			}
			if info.Mode().Perm() != shimPerm {
				if err := os.Chmod(path, shimPerm); err != nil {
					return err
				}
			}
			continue
		}

		if err := fsutil.WriteFileAtomic(path, content, shimPerm); err != nil {
			return err
		}
	}
	return nil
}

func sameFileContent(path string, content []byte) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, nil
	}
	existing, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return bytes.Equal(existing, content), nil
}
