package cli

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/naveedcs/aip/internal/honcho"
	"github.com/naveedcs/aip/internal/profile"
	"github.com/zalando/go-keyring"
)

func TestHonchoEnableStoresConfigAndMCPServer(t *testing.T) {
	keyring.MockInit()
	home := filepath.Join(t.TempDir(), "aip")
	createProfileForTest(t, home, t.TempDir())

	out := mustRunMCPTest(t, home, "honcho", "enable", "mgcs",
		"--workspace-id", " mgcs-ws ",
		"--user-name", " naveed ",
		"--base-url", " https://honcho.example.test ")
	if !strings.Contains(out, "Enabled Honcho memory for mgcs") {
		t.Fatalf("enable output = %q", out)
	}

	prof := loadProfileForMCPTest(t, home, "mgcs")
	wantHoncho := profile.HonchoConfig{
		Enabled:      true,
		WorkspaceID:  "mgcs-ws",
		UserName:     "naveed",
		APIKeySecret: honcho.DefaultAPIKeySecret,
		BaseURL:      "https://honcho.example.test",
	}
	if !reflect.DeepEqual(prof.Honcho, wantHoncho) {
		t.Fatalf("honcho config = %#v, want %#v", prof.Honcho, wantHoncho)
	}

	server, ok := prof.MCP[honcho.MCPServerName]
	if !ok {
		t.Fatalf("honcho MCP server not stored: %#v", prof.MCP)
	}
	if server.Type != "stdio" {
		t.Fatalf("server type = %q, want stdio", server.Type)
	}
	if server.Command != "npx" {
		t.Fatalf("server command = %q, want npx", server.Command)
	}
	wantArgs := []string{
		"-y", "mcp-remote", honcho.MCPURL,
		"--transport", "http-only",
		"--header", "Authorization:Bearer ${HONCHO_API_KEY}",
		"--header", "X-Honcho-Workspace-ID:mgcs-ws",
		"--header", "X-Honcho-User-Name:naveed",
	}
	if !reflect.DeepEqual(server.Args, wantArgs) {
		t.Fatalf("server args = %#v, want %#v", server.Args, wantArgs)
	}
	wantEnv := map[string]string{"HONCHO_API_KEY": "${secret:HONCHO_API_KEY}"}
	if !reflect.DeepEqual(server.Env, wantEnv) {
		t.Fatalf("server env = %#v, want %#v", server.Env, wantEnv)
	}
}

func TestHonchoDisableRemovesConfigAndServer(t *testing.T) {
	keyring.MockInit()
	home := filepath.Join(t.TempDir(), "aip")
	createProfileForTest(t, home, t.TempDir())
	mustRunMCPTest(t, home, "mcp", "add", "mgcs", "docs", "--command", "node")
	mustRunMCPTest(t, home, "honcho", "enable", "mgcs", "--workspace-id", "ws", "--user-name", "n")

	out := mustRunMCPTest(t, home, "honcho", "disable", "mgcs")
	if !strings.Contains(out, "Disabled Honcho memory for mgcs") {
		t.Fatalf("disable output = %q", out)
	}

	prof := loadProfileForMCPTest(t, home, "mgcs")
	if prof.Honcho != (profile.HonchoConfig{}) {
		t.Fatalf("honcho config after disable = %#v, want zero config", prof.Honcho)
	}
	if _, ok := prof.MCP[honcho.MCPServerName]; ok {
		t.Fatalf("honcho MCP server still stored after disable: %#v", prof.MCP)
	}
	if _, ok := prof.MCP["docs"]; !ok {
		t.Fatalf("disable removed unrelated MCP server: %#v", prof.MCP)
	}
}

func TestHonchoShow(t *testing.T) {
	keyring.MockInit()
	home := filepath.Join(t.TempDir(), "aip")
	createProfileForTest(t, home, t.TempDir())

	disabled := mustRunMCPTest(t, home, "honcho", "show", "mgcs")
	if strings.TrimSpace(disabled) != "Honcho memory: disabled" {
		t.Fatalf("disabled show output = %q", disabled)
	}

	mustRunMCPTest(t, home, "honcho", "enable", "mgcs",
		"--workspace-id", "ws-42",
		"--user-name", "n",
		"--api-key-secret", "ACME_HONCHO_KEY",
		"--base-url", "https://honcho.example.test")

	out := mustRunMCPTest(t, home, "honcho", "show", "mgcs")
	for _, want := range []string{
		"Honcho memory: enabled",
		"workspace_id: ws-42",
		"user_name: n",
		"api_key_secret: ACME_HONCHO_KEY",
		"base_url: https://honcho.example.test",
		"mcp_server: honcho",
		honcho.MCPURL,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("show output missing %q in %q", want, out)
		}
	}
}

func TestHonchoEnableRequiresWorkspaceAndUser(t *testing.T) {
	keyring.MockInit()
	home := filepath.Join(t.TempDir(), "aip")
	createProfileForTest(t, home, t.TempDir())

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing workspace",
			args: []string{"--home", home, "honcho", "enable", "mgcs", "--user-name", "n"},
			want: "--workspace-id",
		},
		{
			name: "blank workspace",
			args: []string{"--home", home, "honcho", "enable", "mgcs", "--workspace-id", " \t ", "--user-name", "n"},
			want: "--workspace-id",
		},
		{
			name: "missing user",
			args: []string{"--home", home, "honcho", "enable", "mgcs", "--workspace-id", "ws"},
			want: "--user-name",
		},
		{
			name: "blank user",
			args: []string{"--home", home, "honcho", "enable", "mgcs", "--workspace-id", "ws", "--user-name", " \t "},
			want: "--user-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected honcho enable validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want mention %q", err, tt.want)
			}
		})
	}
}

func TestHonchoEnableWarnsWhenSecretMissing(t *testing.T) {
	keyring.MockInit()
	home := filepath.Join(t.TempDir(), "aip")
	createProfileForTest(t, home, t.TempDir())

	var out, errOut bytes.Buffer
	cmd := NewRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"--home", home, "honcho", "enable", "mgcs", "--workspace-id", "ws", "--user-name", "n"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("enable returned error: %v\nstderr: %s", err, errOut.String())
	}
	if !strings.Contains(out.String(), "Enabled Honcho memory for mgcs") {
		t.Fatalf("stdout = %q", out.String())
	}
	for _, want := range []string{honcho.DefaultAPIKeySecret, "aip secret set mgcs HONCHO_API_KEY"} {
		if !strings.Contains(errOut.String(), want) {
			t.Fatalf("stderr missing %q in %q", want, errOut.String())
		}
	}
}

func TestHonchoEnableRejectsInvalidAPIKeySecret(t *testing.T) {
	keyring.MockInit()
	home := filepath.Join(t.TempDir(), "aip")
	createProfileForTest(t, home, t.TempDir())

	cmd := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	cmd.SetArgs([]string{
		"--home", home,
		"honcho", "enable", "mgcs",
		"--workspace-id", "ws",
		"--user-name", "n",
		"--api-key-secret", "bad-name",
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected invalid --api-key-secret error")
	}
	if !strings.Contains(err.Error(), "--api-key-secret") {
		t.Fatalf("error = %v", err)
	}
}
