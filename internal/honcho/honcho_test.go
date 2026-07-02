package honcho

import (
	"reflect"
	"testing"

	"github.com/naveedcs/aip/internal/profile"
)

func enabledProfile() profile.Profile {
	return profile.Profile{
		Name: "acme",
		Honcho: profile.HonchoConfig{
			Enabled:     true,
			WorkspaceID: "acme-ws",
			UserName:    "naveed",
		},
	}
}

func TestMCPServerBuildsBridge(t *testing.T) {
	server, ok := MCPServer(enabledProfile())
	if !ok {
		t.Fatal("MCPServer ok=false for enabled profile")
	}
	if server.Type != "stdio" {
		t.Fatalf("type = %q, want stdio", server.Type)
	}
	if server.Command != "npx" {
		t.Fatalf("command = %q, want npx", server.Command)
	}
	wantArgs := []string{
		"-y", "mcp-remote", "https://mcp.honcho.dev",
		"--transport", "http-only",
		"--header", "Authorization:Bearer ${HONCHO_API_KEY}",
		"--header", "X-Honcho-Workspace-ID:acme-ws",
		"--header", "X-Honcho-User-Name:naveed",
	}
	if !reflect.DeepEqual(server.Args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", server.Args, wantArgs)
	}
	wantEnv := map[string]string{"HONCHO_API_KEY": "${secret:HONCHO_API_KEY}"}
	if !reflect.DeepEqual(server.Env, wantEnv) {
		t.Fatalf("env = %#v, want %#v", server.Env, wantEnv)
	}
}

func TestMCPServerUsesCustomSecretName(t *testing.T) {
	prof := enabledProfile()
	prof.Honcho.APIKeySecret = " ACME_HONCHO_KEY "
	server, ok := MCPServer(prof)
	if !ok {
		t.Fatal("MCPServer ok=false for enabled profile")
	}
	if server.Env["HONCHO_API_KEY"] != "${secret:ACME_HONCHO_KEY}" {
		t.Fatalf("env = %#v, want HONCHO_API_KEY as custom secret ref", server.Env)
	}
}

func TestMCPServerTrimsWorkspaceAndUser(t *testing.T) {
	prof := enabledProfile()
	prof.Honcho.WorkspaceID = " acme-ws "
	prof.Honcho.UserName = " naveed "
	server, ok := MCPServer(prof)
	if !ok {
		t.Fatal("MCPServer ok=false for enabled profile")
	}
	wantArgs := []string{
		"-y", "mcp-remote", "https://mcp.honcho.dev",
		"--transport", "http-only",
		"--header", "Authorization:Bearer ${HONCHO_API_KEY}",
		"--header", "X-Honcho-Workspace-ID:acme-ws",
		"--header", "X-Honcho-User-Name:naveed",
	}
	if !reflect.DeepEqual(server.Args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", server.Args, wantArgs)
	}
}

func TestMCPServerDisabled(t *testing.T) {
	if _, ok := MCPServer(profile.Profile{}); ok {
		t.Fatal("MCPServer ok=true for disabled profile")
	}
}

func TestEnvVars(t *testing.T) {
	got := EnvVars(enabledProfile())
	want := map[string]string{
		"HONCHO_WORKSPACE_ID": "acme-ws",
		"HONCHO_BASE_URL":     "https://api.honcho.dev",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EnvVars = %#v, want %#v", got, want)
	}

	prof := enabledProfile()
	prof.Honcho.WorkspaceID = " acme-ws "
	prof.Honcho.BaseURL = " https://honcho.example.test "
	got = EnvVars(prof)
	want["HONCHO_BASE_URL"] = "https://honcho.example.test"
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EnvVars with custom base URL = %#v, want %#v", got, want)
	}

	if got := EnvVars(profile.Profile{}); got != nil {
		t.Fatalf("EnvVars disabled = %#v, want nil", got)
	}
}

func TestDefaultsAndOverrides(t *testing.T) {
	if got := APIKeySecret(enabledProfile()); got != "HONCHO_API_KEY" {
		t.Fatalf("APIKeySecret default = %q, want HONCHO_API_KEY", got)
	}
	if got := BaseURL(enabledProfile()); got != "https://api.honcho.dev" {
		t.Fatalf("BaseURL default = %q, want https://api.honcho.dev", got)
	}

	prof := enabledProfile()
	prof.Honcho.APIKeySecret = " ACME_HONCHO_KEY "
	prof.Honcho.BaseURL = " https://honcho.example.test "
	if got := APIKeySecret(prof); got != "ACME_HONCHO_KEY" {
		t.Fatalf("APIKeySecret override = %q, want ACME_HONCHO_KEY", got)
	}
	if got := BaseURL(prof); got != "https://honcho.example.test" {
		t.Fatalf("BaseURL override = %q, want https://honcho.example.test", got)
	}
}
