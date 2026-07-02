package profile

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/naveedcs/aip/internal/tools"
)

type SafetyLevel string

const (
	ReadOnly SafetyLevel = "read-only"
	Standard SafetyLevel = "standard"
	Admin    SafetyLevel = "admin"
)

type AuthMode string

const (
	AuthSubscription AuthMode = "subscription"
	AuthAPIKey       AuthMode = "api-key"
)

type Profile struct {
	Name         string                  `yaml:"name"`
	DisplayName  string                  `yaml:"display_name,omitempty"`
	Description  string                  `yaml:"description,omitempty"`
	ProjectDir   string                  `yaml:"project_dir"`
	SafetyLevel  SafetyLevel             `yaml:"safety_level"`
	AuthMode     AuthMode                `yaml:"auth_mode,omitempty"`
	Tools        map[tools.ID]ToolConfig `yaml:"tools"`
	Secrets      SecretConfig            `yaml:"secrets,omitempty"`
	MCP          map[string]MCPServer    `yaml:"mcp,omitempty"`
	Instructions string                  `yaml:"instructions,omitempty"`
	Honcho       HonchoConfig            `yaml:"honcho,omitempty"`
}

type ToolConfig struct {
	Enabled bool   `yaml:"enabled"`
	Home    string `yaml:"home,omitempty"`
	Config  string `yaml:"config,omitempty"`
}

type SecretConfig struct {
	Provider string   `yaml:"provider,omitempty"`
	Keys     []string `yaml:"keys,omitempty"`
}

type HonchoConfig struct {
	Enabled      bool   `yaml:"enabled,omitempty"`
	WorkspaceID  string `yaml:"workspace_id,omitempty"`
	UserName     string `yaml:"user_name,omitempty"`
	APIKeySecret string `yaml:"api_key_secret,omitempty"`
	BaseURL      string `yaml:"base_url,omitempty"`
}

type MCPServer struct {
	Type     string            `yaml:"type,omitempty"`
	Command  string            `yaml:"command"`
	Args     []string          `yaml:"args,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
	ReadOnly bool              `yaml:"read_only,omitempty"`
}

const secretNamePattern = `[A-Za-z_][A-Za-z0-9_]*`

var profileNameRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)
var secretNameRE = regexp.MustCompile(`^` + secretNamePattern + `$`)
var secretRefRE = regexp.MustCompile(`^\$\{secret:(` + secretNamePattern + `)\}$`)

func ParseSecretRef(value string) (name string, ok bool) {
	matches := secretRefRE.FindStringSubmatch(value)
	if matches == nil {
		return "", false
	}
	return matches[1], true
}

func ValidateName(name string) error {
	if !profileNameRE.MatchString(name) {
		return fmt.Errorf("profile name %q must use letters, numbers, dash, or underscore and must not contain path separators", name)
	}
	return nil
}

func Validate(p Profile) error {
	if err := ValidateName(p.Name); err != nil {
		return err
	}
	if p.ProjectDir == "" {
		return fmt.Errorf("project_dir is required")
	}
	switch p.SafetyLevel {
	case ReadOnly, Standard, Admin:
	default:
		return fmt.Errorf("safety_level must be read-only, standard, or admin")
	}
	switch p.AuthMode {
	case "", AuthSubscription, AuthAPIKey:
	default:
		return fmt.Errorf("auth_mode must be subscription or api-key")
	}
	if err := validateHoncho(p.Honcho); err != nil {
		return err
	}
	return nil
}

func validateHoncho(h HonchoConfig) error {
	fields := []struct {
		name  string
		value string
	}{
		{name: "workspace_id", value: h.WorkspaceID},
		{name: "user_name", value: h.UserName},
		{name: "api_key_secret", value: h.APIKeySecret},
		{name: "base_url", value: h.BaseURL},
	}
	for _, field := range fields {
		if containsControlCharacter(field.value) {
			return fmt.Errorf("honcho.%s must not contain control characters", field.name)
		}
	}
	if secretName := strings.TrimSpace(h.APIKeySecret); secretName != "" && !secretNameRE.MatchString(secretName) {
		return fmt.Errorf("honcho.api_key_secret must use letters, numbers, or underscore and start with a letter or underscore")
	}
	if !h.Enabled {
		return nil
	}
	if strings.TrimSpace(h.WorkspaceID) == "" {
		return fmt.Errorf("honcho.workspace_id is required when honcho is enabled")
	}
	if strings.TrimSpace(h.UserName) == "" {
		return fmt.Errorf("honcho.user_name is required when honcho is enabled")
	}
	return nil
}

func containsControlCharacter(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
