package doctor

import (
	"errors"
	"os"
	"sort"
	"strings"

	"github.com/naveedcs/aip/internal/honcho"
	"github.com/naveedcs/aip/internal/mcp"
	"github.com/naveedcs/aip/internal/paths"
	"github.com/naveedcs/aip/internal/profile"
	"github.com/naveedcs/aip/internal/secrets"
	"github.com/naveedcs/aip/internal/tools"
)

type ToolStatus struct {
	Tool      tools.Tool
	Enabled   bool
	Installed bool
	Path      string
	Home      string
	ConfigDir string
}

type Report struct {
	Profile       profile.Profile
	AuthMode      profile.AuthMode
	ProjectExists bool
	Tools         []ToolStatus
	SecretNames   []string
	MCPServers    []string
	Honcho        profile.HonchoConfig
	Warnings      []string
}

type secretCheck struct {
	targets []string
}

func CheckProfile(p paths.Paths, name string) (Report, error) {
	prof, err := profile.NewStore(p).Load(name)
	if err != nil {
		return Report{}, err
	}
	projectDir, err := paths.Expand(prof.ProjectDir)
	if err != nil {
		return Report{}, err
	}

	names := append([]string(nil), prof.Secrets.Keys...)
	sort.Strings(names)
	mcpNames := make([]string, 0, len(prof.MCP))
	for name := range prof.MCP {
		mcpNames = append(mcpNames, name)
	}
	sort.Strings(mcpNames)

	detections := tools.Detect()
	statuses := make([]ToolStatus, 0, len(detections))
	warnings := []string{}
	if prof.Honcho.Enabled {
		if _, ok := prof.MCP[honcho.MCPServerName]; !ok {
			warnings = append(warnings, "honcho is enabled but its MCP server is missing; re-run: aip honcho enable")
		}
	}
	keychain := secrets.NewKeychain()
	secretChecks := map[string]*secretCheck{}
	for _, name := range names {
		ensureSecretCheck(secretChecks, name)
	}
	for _, ref := range mcp.SecretRefs(prof) {
		check := ensureSecretCheck(secretChecks, ref.SecretName)
		if !slicesContains(check.targets, ref.Target) {
			check.targets = append(check.targets, ref.Target)
		}
	}
	for _, name := range sortedSecretCheckNames(secretChecks) {
		check := secretChecks[name]
		sort.Strings(check.targets)
		if _, err := keychain.Get(prof.Name, name); errors.Is(err, secrets.ErrNotFound) {
			warnings = append(warnings, missingSecretWarning(name, *check))
		} else if err != nil {
			warnings = append(warnings, secretCheckFailedWarning(name, *check, err))
		}
	}
	for _, detected := range detections {
		cfg := prof.Tools[detected.Tool.ID]
		status := ToolStatus{
			Tool:      detected.Tool,
			Enabled:   cfg.Enabled,
			Installed: detected.Installed,
			Path:      detected.Path,
			Home:      cfg.Home,
			ConfigDir: p.ToolConfigDir(prof.Name, string(detected.Tool.ID)),
		}
		if cfg.Enabled && !detected.Installed {
			warnings = append(warnings, string(detected.Tool.ID)+" is enabled but not installed")
		}
		statuses = append(statuses, status)
	}
	if prof.SafetyLevel == profile.Admin {
		warnings = append(warnings, "admin profile requires launch confirmation")
	}
	if prof.AuthMode == profile.AuthSubscription {
		for _, name := range names {
			if name == "ANTHROPIC_API_KEY" {
				warnings = append(warnings, "subscription profile declares ANTHROPIC_API_KEY; it would override the OAuth login")
			}
		}
	}

	_, err = os.Stat(projectDir)
	if err != nil && !os.IsNotExist(err) {
		warnings = append(warnings, "project status check failed: "+err.Error())
	}
	return Report{
		Profile:       prof,
		AuthMode:      prof.AuthMode,
		ProjectExists: err == nil,
		Tools:         statuses,
		SecretNames:   names,
		MCPServers:    mcpNames,
		Honcho:        prof.Honcho,
		Warnings:      warnings,
	}, nil
}

func ensureSecretCheck(checks map[string]*secretCheck, name string) *secretCheck {
	check, ok := checks[name]
	if !ok {
		check = &secretCheck{}
		checks[name] = check
	}
	return check
}

func sortedSecretCheckNames(checks map[string]*secretCheck) []string {
	names := make([]string, 0, len(checks))
	for name := range checks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func missingSecretWarning(name string, check secretCheck) string {
	if len(check.targets) > 0 {
		return "mcp secret " + name + " (" + mcpTargetContext(check.targets) + ") is referenced but missing from keychain"
	}
	return "secret " + name + " is declared but missing from keychain"
}

func secretCheckFailedWarning(name string, check secretCheck, err error) string {
	if len(check.targets) > 0 {
		return "mcp secret " + name + " (" + mcpTargetContext(check.targets) + ") keychain check failed: " + err.Error()
	}
	return "secret " + name + " keychain check failed: " + err.Error()
}

func mcpTargetContext(targets []string) string {
	if len(targets) == 1 {
		return "mcp target " + targets[0]
	}
	return "mcp targets " + strings.Join(targets, ",")
}

func slicesContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
