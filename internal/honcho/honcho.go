// Package honcho links AIP profiles to a Honcho memory workspace. It
// synthesizes the mcp-remote stdio bridge to Honcho's hosted MCP server and
// the non-secret SDK environment variables.
package honcho

import (
	"strings"

	"github.com/naveedcs/aip/internal/profile"
)

const (
	// MCPServerName is the reserved key for the Honcho entry in prof.MCP.
	MCPServerName = "honcho"
	// MCPURL is Honcho's hosted MCP server.
	MCPURL = "https://mcp.honcho.dev"
	// DefaultBaseURL is the Honcho API base URL for SDK usage.
	DefaultBaseURL = "https://api.honcho.dev"
	// DefaultAPIKeySecret is the keychain secret name used when none is set.
	DefaultAPIKeySecret = "HONCHO_API_KEY"

	apiKeyEnv    = "HONCHO_API_KEY"
	workspaceEnv = "HONCHO_WORKSPACE_ID"
	baseURLEnv   = "HONCHO_BASE_URL"
)

// APIKeySecret returns the keychain secret name holding the Honcho API key.
func APIKeySecret(prof profile.Profile) string {
	if name := strings.TrimSpace(prof.Honcho.APIKeySecret); name != "" {
		return name
	}
	return DefaultAPIKeySecret
}

// BaseURL returns the Honcho API base URL for SDK usage.
func BaseURL(prof profile.Profile) string {
	if url := strings.TrimSpace(prof.Honcho.BaseURL); url != "" {
		return url
	}
	return DefaultBaseURL
}

// MCPServer builds the stdio MCP server entry that bridges the profile to
// Honcho's hosted MCP server via mcp-remote.
func MCPServer(prof profile.Profile) (profile.MCPServer, bool) {
	if !prof.Honcho.Enabled {
		return profile.MCPServer{}, false
	}
	workspaceID := strings.TrimSpace(prof.Honcho.WorkspaceID)
	userName := strings.TrimSpace(prof.Honcho.UserName)
	return profile.MCPServer{
		Type:    "stdio",
		Command: "npx",
		Args: []string{
			"-y", "mcp-remote", MCPURL,
			"--transport", "http-only",
			"--header", "Authorization:Bearer ${" + apiKeyEnv + "}",
			"--header", "X-Honcho-Workspace-ID:" + workspaceID,
			"--header", "X-Honcho-User-Name:" + userName,
		},
		Env: map[string]string{
			apiKeyEnv: "${secret:" + APIKeySecret(prof) + "}",
		},
	}, true
}

// EnvVars returns the non-secret Honcho env vars for SDK-based code run inside
// the profile. The API key is injected and masked through the MCP secret path.
func EnvVars(prof profile.Profile) map[string]string {
	if !prof.Honcho.Enabled {
		return nil
	}
	return map[string]string{
		workspaceEnv: strings.TrimSpace(prof.Honcho.WorkspaceID),
		baseURLEnv:   BaseURL(prof),
	}
}
