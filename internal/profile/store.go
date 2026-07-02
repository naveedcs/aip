package profile

import (
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/naveedcs/aip/internal/fsutil"
	"github.com/naveedcs/aip/internal/paths"
	"github.com/naveedcs/aip/internal/tools"
	"gopkg.in/yaml.v3"
)

type Store struct {
	paths paths.Paths
}

func NewStore(paths paths.Paths) Store {
	return Store{paths: paths}
}

func (s Store) Save(p Profile) error {
	if err := ValidateName(p.Name); err != nil {
		return err
	}
	if p.SafetyLevel == "" {
		p.SafetyLevel = ReadOnly
	}
	if p.AuthMode == "" {
		p.AuthMode = AuthSubscription
	}
	s.normalizeDerivedFields(&p)
	if p.Secrets.Provider == "" {
		p.Secrets.Provider = "keychain"
	}
	p.Instructions = s.paths.InstructionsFile(p.Name)

	if err := Validate(p); err != nil {
		return err
	}
	if err := os.MkdirAll(s.paths.ProfileDir(p.Name), 0o700); err != nil {
		return err
	}
	for _, tool := range tools.All() {
		if p.Tools[tool.ID].Enabled {
			if err := os.MkdirAll(s.paths.ToolConfigDir(p.Name, string(tool.ID)), 0o700); err != nil {
				return err
			}
		}
	}
	if err := writeFileIfMissing(s.paths.InstructionsFile(p.Name), []byte(instructionsContent(p))); err != nil {
		return err
	}

	data, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.paths.ProfileFile(p.Name), data, 0o600)
}

func (s Store) Load(name string) (Profile, error) {
	if err := ValidateName(name); err != nil {
		return Profile{}, err
	}
	data, err := os.ReadFile(s.paths.ProfileFile(name))
	if err != nil {
		return Profile{}, err
	}
	var p Profile
	if err := decodeProfileYAML(data, &p); err != nil {
		return Profile{}, err
	}
	if err := ValidateName(p.Name); err != nil {
		return Profile{}, err
	}
	if p.Name != name {
		return Profile{}, errors.New("profile file name does not match requested profile")
	}
	if len(p.MCP) == 0 {
		if err := s.loadLegacyMCPSidecar(&p); err != nil {
			return Profile{}, err
		}
	}
	if p.AuthMode == "" {
		p.AuthMode = AuthSubscription
	}
	s.normalizeDerivedFields(&p)
	if p.Secrets.Provider == "" {
		p.Secrets.Provider = "keychain"
	}
	if err := Validate(p); err != nil {
		return Profile{}, err
	}
	return p, nil
}

func (s Store) loadLegacyMCPSidecar(p *Profile) error {
	data, err := os.ReadFile(filepath.Join(s.paths.ProfileDir(p.Name), "mcp", "servers.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var sidecar struct {
		Servers map[string]MCPServer `yaml:"servers"`
	}
	if err := yaml.Unmarshal(data, &sidecar); err != nil {
		return err
	}
	p.MCP = sidecar.Servers
	return nil
}

func decodeProfileYAML(data []byte, p *Profile) error {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	dropLegacyMCPServerList(&doc)
	return doc.Decode(p)
}

func dropLegacyMCPServerList(doc *yaml.Node) {
	root := doc
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value != "mcp" {
			continue
		}
		mcp := root.Content[i+1]
		if mcp.Kind != yaml.MappingNode {
			return
		}
		filtered := mcp.Content[:0]
		for j := 0; j < len(mcp.Content)-1; j += 2 {
			key := mcp.Content[j]
			value := mcp.Content[j+1]
			if key.Value == "servers" && value.Kind == yaml.SequenceNode {
				continue
			}
			filtered = append(filtered, key, value)
		}
		mcp.Content = filtered
		return
	}
}

func (s Store) normalizeDerivedFields(p *Profile) {
	if p.Tools == nil {
		p.Tools = map[tools.ID]ToolConfig{}
	}
	for _, tool := range tools.All() {
		cfg := p.Tools[tool.ID]
		if cfg.Enabled {
			dir := s.paths.ToolConfigDir(p.Name, string(tool.ID))
			cfg.Home = dir
			cfg.Config = filepath.Join(dir, configFileName(tool.ID))
		} else {
			cfg.Home = ""
			cfg.Config = ""
		}
		p.Tools[tool.ID] = cfg
	}
}

func (s Store) List() ([]Profile, error) {
	entries, err := os.ReadDir(s.paths.ProfilesDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]Profile, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		p, err := s.Load(entry.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func writeFileIfMissing(path string, data []byte) error {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode().IsRegular() {
			return nil
		}
		return &os.PathError{Op: "write", Path: path, Err: os.ErrInvalid}
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func instructionsContent(p Profile) string {
	title := p.DisplayName
	if title == "" {
		title = p.Name
	}
	return "# " + title + "\n\nProfile-specific AI instructions live here.\n"
}

func configFileName(id tools.ID) string {
	switch id {
	case tools.Codex:
		return "config.toml"
	default:
		return "settings.json"
	}
}
