package activation

import (
	"fmt"
	"os"
	"strings"

	"github.com/naveedcs/aip/internal/honcho"
	"github.com/naveedcs/aip/internal/mcp"
	"github.com/naveedcs/aip/internal/paths"
	"github.com/naveedcs/aip/internal/profile"
	"github.com/naveedcs/aip/internal/secrets"
	"github.com/naveedcs/aip/internal/tools"
)

type Plan struct {
	Profile   profile.Profile
	Tool      tools.Tool
	Command   string
	Args      []string
	Dir       string
	Env       map[string]string
	MaskedEnv map[string]string
}

func Build(p paths.Paths, sp secrets.Provider, profileName string, toolID tools.ID, args []string) (Plan, error) {
	prof, err := profile.NewStore(p).Load(profileName)
	if err != nil {
		return Plan{}, err
	}

	tool, ok := tools.Get(toolID)
	if !ok {
		return Plan{}, fmt.Errorf("unsupported tool %q", toolID)
	}
	if !prof.Tools[toolID].Enabled {
		return Plan{}, fmt.Errorf("tool %q is not enabled for profile %q", toolID, profileName)
	}

	env := safeParentEnv()
	env[tool.HomeEnv] = p.ToolConfigDir(prof.Name, string(tool.ID))
	env["AIP_PROFILE"] = prof.Name
	env["AIP_PROFILE_SAFETY"] = string(prof.SafetyLevel)

	if prof.AuthMode == profile.AuthAPIKey {
		name, ok := requiredAPIKeyName(tool.ID)
		if ok && !declaredSecret(name, prof.Secrets.Keys) {
			return Plan{}, fmt.Errorf("profile %q is api-key auth_mode; declare secret %s", prof.Name, name)
		}
	}

	mcpSecretRefs := mcp.LaunchSecretRefs(prof, toolID)
	if err := validateMCPSecretTargetRefs(mcpSecretRefs, prof.Secrets.Keys); err != nil {
		return Plan{}, err
	}
	secretValues := make(map[string]string, len(prof.Secrets.Keys))
	for _, name := range prof.Secrets.Keys {
		if prof.AuthMode == profile.AuthSubscription && name == "ANTHROPIC_API_KEY" {
			return Plan{}, fmt.Errorf("profile %q is subscription auth_mode; remove ANTHROPIC_API_KEY or set auth_mode: api-key", prof.Name)
		}
		value, err := sp.Get(prof.Name, name)
		if err != nil {
			return Plan{}, fmt.Errorf("secret %q for profile %q: %w", name, prof.Name, err)
		}
		secretValues[name] = value
		env[name] = value
	}
	for _, ref := range mcpSecretRefs {
		value, ok := secretValues[ref.SecretName]
		if !ok {
			var err error
			value, err = sp.Get(prof.Name, ref.SecretName)
			if err != nil {
				return Plan{}, fmt.Errorf("MCP secret %q for profile %q target %q: %w", ref.SecretName, prof.Name, ref.Target, err)
			}
			secretValues[ref.SecretName] = value
		}
		env[ref.Target] = value
	}
	for key, value := range honcho.EnvVars(prof) {
		env[key] = value
	}

	dir, err := paths.Expand(prof.ProjectDir)
	if err != nil {
		return Plan{}, err
	}

	masked := make(map[string]string, len(env))
	for key, value := range env {
		if declaredSecret(key, prof.Secrets.Keys) || mcpSecretTarget(key, mcpSecretRefs) {
			masked[key] = "********"
			continue
		}
		masked[key] = value
	}
	launchArgs := mcp.LaunchArgs(p, prof, toolID)
	planArgs := append([]string(nil), launchArgs...)
	planArgs = append(planArgs, args...)

	return Plan{
		Profile:   prof,
		Tool:      tool,
		Command:   tool.Binary,
		Args:      planArgs,
		Dir:       dir,
		Env:       env,
		MaskedEnv: masked,
	}, nil
}

func requiredAPIKeyName(toolID tools.ID) (string, bool) {
	switch toolID {
	case tools.Codex:
		return "OPENAI_API_KEY", true
	case tools.Claude:
		return "ANTHROPIC_API_KEY", true
	case tools.Gemini:
		return "GEMINI_API_KEY", true
	case tools.Copilot:
		return "GITHUB_TOKEN", true
	default:
		return "", false
	}
}

func declaredSecret(key string, secretKeys []string) bool {
	for _, secretKey := range secretKeys {
		if key == secretKey {
			return true
		}
	}
	return false
}

func validateMCPSecretTargetRefs(refs []mcp.SecretRef, declaredSecretNames []string) error {
	targetSecrets := map[string]string{}
	for _, name := range declaredSecretNames {
		targetSecrets[name] = name
	}
	for _, ref := range refs {
		existing, ok := targetSecrets[ref.Target]
		if ok && existing != ref.SecretName {
			return fmt.Errorf("MCP secret target %q references both secrets %q and %q", ref.Target, existing, ref.SecretName)
		}
		targetSecrets[ref.Target] = ref.SecretName
	}
	return nil
}

func mcpSecretTarget(key string, refs []mcp.SecretRef) bool {
	for _, ref := range refs {
		if key == ref.Target {
			return true
		}
	}
	return false
}

func safeParentEnv() map[string]string {
	out := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || !safeParentEnvKey(key) {
			continue
		}
		out[key] = value
	}
	return out
}

func safeParentEnvKey(key string) bool {
	switch key {
	case "HOME", "PATH", "LANG", "TERM", "TMPDIR", "USER", "SHELL", "SSH_AUTH_SOCK":
		return true
	default:
		return strings.HasPrefix(key, "LC_")
	}
}
