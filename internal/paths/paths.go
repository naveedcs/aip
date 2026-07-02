package paths

import (
	"os"
	"path/filepath"
	"strings"
)

type Paths struct {
	RootDir          string
	ConfigFile       string
	ProfilesDir      string
	ShimsDir         string
	LibrarySkillsDir string
	SecretsDir       string
	TemplatesDir     string
	LogsDir          string
}

func ResolveRoot(flagHome string) (string, error) {
	raw := strings.TrimSpace(flagHome)
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("AIP_HOME"))
	}
	if raw == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		raw = filepath.Join(home, ".aip")
	}
	return Expand(raw)
}

func Expand(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return filepath.Abs(path)
}

func ForRoot(root string) Paths {
	return Paths{
		RootDir:          root,
		ConfigFile:       filepath.Join(root, "config.yaml"),
		ProfilesDir:      filepath.Join(root, "profiles"),
		ShimsDir:         filepath.Join(root, "shims"),
		LibrarySkillsDir: filepath.Join(root, "library", "skills"),
		SecretsDir:       filepath.Join(root, "secrets"),
		TemplatesDir:     filepath.Join(root, "templates"),
		LogsDir:          filepath.Join(root, "logs"),
	}
}

func (p Paths) ProfileDir(name string) string {
	return filepath.Join(p.ProfilesDir, name)
}

func (p Paths) ProfileFile(name string) string {
	return filepath.Join(p.ProfileDir(name), "profile.yaml")
}

func (p Paths) InstructionsFile(name string) string {
	return filepath.Join(p.ProfileDir(name), "instructions.md")
}

// ToolConfigDir is the single directory the tool is launched against
// and where per-tool materialized config is written.
func (p Paths) ToolConfigDir(profileName string, toolID string) string {
	return filepath.Join(p.ProfileDir(profileName), "tools", toolID)
}
