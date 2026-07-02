package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/naveedcs/aip/internal/fsutil"
	"gopkg.in/yaml.v3"
)

type Config struct {
	RecommendedProfile string   `yaml:"recommended_profile,omitempty"`
	ProjectType        string   `yaml:"project_type,omitempty"`
	RecommendedMCP     []string `yaml:"recommended_mcp,omitempty"`
}

const GitignoreBlock = `# AIP local secrets
.aip/secrets*
.aip/env.local
.env
.env.*`

func ConfigPath(dir string) string {
	return filepath.Join(dir, ".aip", "profile.yaml")
}

func Load(dir string) (Config, error) {
	data, err := os.ReadFile(ConfigPath(dir))
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(dir string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(ConfigPath(dir), data, 0o644)
}

func EnsureGitignore(dir string) (bool, error) {
	path := filepath.Join(dir, ".gitignore")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return true, fsutil.WriteFileAtomic(path, []byte(GitignoreBlock+"\n"), 0o644)
	}
	if err != nil {
		return false, err
	}
	if hasAIPGitignoreSentinel(data) {
		return false, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	content := string(data)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if content != "" {
		content += "\n"
	}
	content += GitignoreBlock + "\n"
	return true, fsutil.WriteFileAtomic(path, []byte(content), info.Mode().Perm())
}

func hasAIPGitignoreSentinel(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(strings.TrimSuffix(line, "\r")) == ".aip/env.local" {
			return true
		}
	}
	return false
}
