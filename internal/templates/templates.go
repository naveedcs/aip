package templates

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/naveedcs/aip/internal/paths"
	"github.com/naveedcs/aip/internal/profile"
	"github.com/naveedcs/aip/internal/tools"
	"gopkg.in/yaml.v3"
)

//go:embed builtin/*.yaml
var builtinFS embed.FS

var templateNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

type Template struct {
	Description string                       `yaml:"description"`
	SafetyLevel profile.SafetyLevel          `yaml:"safety_level"`
	AuthMode    profile.AuthMode             `yaml:"auth_mode"`
	Tools       []tools.ID                   `yaml:"tools"`
	SecretKeys  []string                     `yaml:"secret_keys"`
	MCP         map[string]profile.MCPServer `yaml:"mcp"`
}

type Info struct {
	Name        string
	Description string
	Source      string
}

func List(p paths.Paths) ([]Info, error) {
	infos := map[string]Info{}

	builtins, err := listBuiltins()
	if err != nil {
		return nil, err
	}
	for _, info := range builtins {
		infos[info.Name] = info
	}

	team, err := listTeam(p.TemplatesDir)
	if err != nil {
		return nil, err
	}
	for _, info := range team {
		infos[info.Name] = info
	}

	out := make([]Info, 0, len(infos))
	for _, info := range infos {
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func Get(p paths.Paths, name string) (Template, error) {
	if err := validateName(name); err != nil {
		return Template{}, err
	}
	if tmpl, ok, err := getTeam(p.TemplatesDir, name); err != nil {
		return Template{}, err
	} else if ok {
		return tmpl, nil
	}

	tmpl, ok, err := getBuiltin(name)
	if err != nil {
		return Template{}, err
	}
	if !ok {
		return Template{}, fmt.Errorf("unknown template %q", name)
	}
	return tmpl, nil
}

func validateName(name string) error {
	if !templateNameRE.MatchString(name) {
		return fmt.Errorf("invalid template name %q", name)
	}
	return nil
}

func (t Template) ToProfile(name, displayName, projectDir string) profile.Profile {
	if displayName == "" {
		displayName = name
	}
	toolConfigs := make(map[tools.ID]profile.ToolConfig, len(t.Tools))
	for _, id := range t.Tools {
		toolConfigs[id] = profile.ToolConfig{Enabled: true}
	}
	return profile.Profile{
		Name:        name,
		DisplayName: displayName,
		Description: t.Description,
		ProjectDir:  projectDir,
		SafetyLevel: t.SafetyLevel,
		AuthMode:    t.AuthMode,
		Tools:       toolConfigs,
		Secrets: profile.SecretConfig{
			Keys: append([]string(nil), t.SecretKeys...),
		},
		MCP: cloneMCP(t.MCP),
	}
}

func listBuiltins() ([]Info, error) {
	entries, err := fs.ReadDir(builtinFS, "builtin")
	if err != nil {
		return nil, err
	}
	out := make([]Info, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".yaml")
		tmpl, ok, err := getBuiltin(name)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, Info{Name: name, Description: tmpl.Description, Source: "built-in"})
		}
	}
	return out, nil
}

func listTeam(dir string) ([]Info, error) {
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]Info, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".yaml")
		tmpl, err := readTemplate(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, Info{Name: name, Description: tmpl.Description, Source: "team"})
	}
	return out, nil
}

func getBuiltin(name string) (Template, bool, error) {
	data, err := builtinFS.ReadFile(filepath.ToSlash(filepath.Join("builtin", name+".yaml")))
	if errors.Is(err, fs.ErrNotExist) {
		return Template{}, false, nil
	}
	if err != nil {
		return Template{}, false, err
	}
	tmpl, err := parseTemplate(data)
	return tmpl, true, err
}

func getTeam(dir, name string) (Template, bool, error) {
	if dir == "" {
		return Template{}, false, nil
	}
	tmpl, err := readTemplate(filepath.Join(dir, name+".yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return Template{}, false, nil
	}
	if err != nil {
		return Template{}, false, err
	}
	return tmpl, true, nil
}

func readTemplate(path string) (Template, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Template{}, err
	}
	return parseTemplate(data)
}

func parseTemplate(data []byte) (Template, error) {
	var tmpl Template
	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		return Template{}, err
	}
	return tmpl, nil
}

func cloneMCP(in map[string]profile.MCPServer) map[string]profile.MCPServer {
	if in == nil {
		return nil
	}
	out := make(map[string]profile.MCPServer, len(in))
	for name, server := range in {
		server.Args = append([]string(nil), server.Args...)
		if server.Env != nil {
			env := make(map[string]string, len(server.Env))
			for key, value := range server.Env {
				env[key] = value
			}
			server.Env = env
		}
		out[name] = server
	}
	return out
}
