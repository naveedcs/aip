package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/naveedcs/aip/internal/paths"
	"github.com/naveedcs/aip/internal/profile"
	"github.com/zalando/go-keyring"
)

func TestMCPAddListSyncRendersProfileServers(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createProfileForTest(t, home, project)

	addOut := &bytes.Buffer{}
	add := NewRootCommand(addOut, addOut)
	add.SetArgs([]string{
		"--home", home,
		"mcp", "add", "mgcs", "github",
		"--command", "npx",
		"--arg", "-y",
		"--arg", "@modelcontextprotocol/server-github",
		"--env", "GH_PAT=${secret:GITHUB_TOKEN}",
		"--env", "NODE_ENV=production",
		"--readonly",
	})
	if err := add.Execute(); err != nil {
		t.Fatalf("mcp add returned error: %v", err)
	}
	if !strings.Contains(addOut.String(), "Saved MCP server github") {
		t.Fatalf("add output = %q", addOut.String())
	}

	prof := loadProfileForMCPTest(t, home, "mgcs")
	got := prof.MCP["github"]
	if got.Command != "npx" || len(got.Args) != 2 || got.Args[0] != "-y" || got.Args[1] != "@modelcontextprotocol/server-github" {
		t.Fatalf("saved server = %#v", got)
	}
	if got.Env["GH_PAT"] != "${secret:GITHUB_TOKEN}" || got.Env["NODE_ENV"] != "production" || !got.ReadOnly {
		t.Fatalf("saved server env/readonly = %#v", got)
	}

	listOut := &bytes.Buffer{}
	list := NewRootCommand(listOut, listOut)
	list.SetArgs([]string{"--home", home, "mcp", "list", "mgcs"})
	if err := list.Execute(); err != nil {
		t.Fatalf("mcp list returned error: %v", err)
	}
	if !strings.Contains(listOut.String(), "github\tnpx\treadonly=true\tenv=GH_PAT,NODE_ENV") {
		t.Fatalf("list output = %q", listOut.String())
	}
	if strings.Contains(listOut.String(), "GITHUB_TOKEN") || strings.Contains(listOut.String(), "production") {
		t.Fatalf("list output leaked env value: %q", listOut.String())
	}

	syncOut := &bytes.Buffer{}
	syncErr := &bytes.Buffer{}
	syncCmd := NewRootCommand(syncOut, syncErr)
	syncCmd.SetArgs([]string{"--home", home, "mcp", "sync", "mgcs"})
	if err := syncCmd.Execute(); err != nil {
		t.Fatalf("mcp sync returned error: %v", err)
	}
	if strings.TrimSpace(syncOut.String()) != "Synced MCP servers for mgcs" {
		t.Fatalf("sync stdout = %q", syncOut.String())
	}
	if syncErr.Len() != 0 {
		t.Fatalf("sync stderr = %q", syncErr.String())
	}

	codexConfig, err := os.ReadFile(filepath.Join(home, "profiles", "mgcs", "tools", "codex", "config.toml"))
	if err != nil {
		t.Fatalf("ReadFile codex config returned error: %v", err)
	}
	claudeMCP, err := os.ReadFile(filepath.Join(home, "profiles", "mgcs", "tools", "claude", "mcp.json"))
	if err != nil {
		t.Fatalf("ReadFile claude mcp returned error: %v", err)
	}
	if !strings.Contains(string(codexConfig), `env_vars = ["GH_PAT"]`) {
		t.Fatalf("codex config = %s", codexConfig)
	}
	if !strings.Contains(string(codexConfig), `NODE_ENV = "production"`) {
		t.Fatalf("codex config = %s", codexConfig)
	}
	if !strings.Contains(string(claudeMCP), `"GH_PAT": "${GH_PAT}"`) {
		t.Fatalf("claude mcp = %s", claudeMCP)
	}
	if !strings.Contains(string(claudeMCP), `"NODE_ENV": "production"`) {
		t.Fatalf("claude mcp = %s", claudeMCP)
	}
}

func TestMCPRemoveDeletesServerFromProfileAndList(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createProfileForTest(t, home, project)
	mustRunMCPTest(t, home, "mcp", "add", "mgcs", "github", "--command", "npx")
	mustRunMCPTest(t, home, "mcp", "add", "mgcs", "files", "--command", "node")

	rmOut := mustRunMCPTest(t, home, "mcp", "rm", "mgcs", "github")
	if !strings.Contains(rmOut, "Removed MCP server github") {
		t.Fatalf("rm output = %q", rmOut)
	}

	prof := loadProfileForMCPTest(t, home, "mgcs")
	if _, ok := prof.MCP["github"]; ok {
		t.Fatalf("github server still present: %#v", prof.MCP)
	}
	if _, ok := prof.MCP["files"]; !ok {
		t.Fatalf("files server missing after remove: %#v", prof.MCP)
	}

	listOut := mustRunMCPTest(t, home, "mcp", "list", "mgcs")
	if strings.Contains(listOut, "github") || !strings.Contains(listOut, "files") {
		t.Fatalf("list output = %q", listOut)
	}

	removeOut := mustRunMCPTest(t, home, "mcp", "remove", "mgcs", "files")
	if !strings.Contains(removeOut, "Removed MCP server files") {
		t.Fatalf("remove output = %q", removeOut)
	}
	if prof := loadProfileForMCPTest(t, home, "mgcs"); len(prof.MCP) != 0 {
		t.Fatalf("MCP servers after remove alias = %#v", prof.MCP)
	}
}

func TestMCPTestConnectsToStdioServer(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createProfileForTest(t, home, project)

	script := filepath.Join(t.TempDir(), "fake-mcp")
	if err := os.WriteFile(script, []byte(`#!/bin/sh
read line
printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","serverInfo":{"name":"fake-mcp","version":"9.9"}}}'
`), 0o755); err != nil {
		t.Fatalf("WriteFile fake MCP server returned error: %v", err)
	}
	mustRunMCPTest(t, home, "mcp", "add", "mgcs", "fake", "--command", script)

	out := mustRunMCPTest(t, home, "mcp", "test", "mgcs", "fake")
	if !strings.Contains(out, "OK fake") || !strings.Contains(out, "fake-mcp") {
		t.Fatalf("mcp test output = %q", out)
	}
}

func TestMCPTestReportsMissingSecret(t *testing.T) {
	keyring.MockInit()
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createProfileForTest(t, home, project)
	mustRunMCPTest(t, home, "mcp", "add", "mgcs", "github", "--command", "echo", "--env", "GH=${secret:GITHUB_TOKEN}")

	out := &bytes.Buffer{}
	cmd := NewRootCommand(out, out)
	cmd.SetArgs([]string{"--home", home, "mcp", "test", "mgcs", "github", "--offline"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected missing secret error")
	}
	if !strings.Contains(out.String(), "GITHUB_TOKEN") {
		t.Fatalf("mcp test output = %q", out.String())
	}
}

func TestMCPTestRejectsUnknownServer(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createProfileForTest(t, home, project)

	out := &bytes.Buffer{}
	cmd := NewRootCommand(out, out)
	cmd.SetArgs([]string{"--home", home, "mcp", "test", "mgcs", "nope"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected unknown server error")
	}
}

func TestMCPTestRejectsRelativeExecutablePath(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createProfileForTest(t, home, project)

	workDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(workDir, "tools"), 0o755); err != nil {
		t.Fatalf("Mkdir tools returned error: %v", err)
	}
	serverPath := filepath.Join(workDir, "tools", "server")
	if err := os.WriteFile(serverPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile server returned error: %v", err)
	}
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd returned error: %v", err)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Chdir returned error: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore Chdir returned error: %v", err)
		}
	}()

	mustRunMCPTest(t, home, "mcp", "add", "mgcs", "relative", "--command", "tools/server")

	out := &bytes.Buffer{}
	cmd := NewRootCommand(out, out)
	cmd.SetArgs([]string{"--home", home, "mcp", "test", "mgcs", "relative", "--offline"})
	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected relative executable path error")
	}
	if !strings.Contains(out.String(), "relative path") || !strings.Contains(out.String(), "tools/server") {
		t.Fatalf("mcp test output = %q", out.String())
	}
}

func TestMCPTestNormalizesStdioType(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createProfileForTest(t, home, project)

	prof := loadProfileForMCPTest(t, home, "mgcs")
	prof.MCP = map[string]profile.MCPServer{}
	prof.MCP["upper"] = profile.MCPServer{Type: "STDIO", Command: "echo"}
	prof.MCP["padded"] = profile.MCPServer{Type: " stdio ", Command: "echo"}
	if err := profile.NewStore(paths.ForRoot(home)).Save(prof); err != nil {
		t.Fatalf("Save profile returned error: %v", err)
	}

	out := mustRunMCPTest(t, home, "mcp", "test", "mgcs", "--offline")
	if !strings.Contains(out, "OK upper") || !strings.Contains(out, "OK padded") {
		t.Fatalf("mcp test output = %q", out)
	}
}

func TestMCPAddRejectsInvalidEnvKey(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	createProfileForTest(t, home, t.TempDir())

	out := &bytes.Buffer{}
	cmd := NewRootCommand(out, out)
	cmd.SetArgs([]string{
		"--home", home,
		"mcp", "add", "mgcs", "github",
		"--command", "npx",
		"--env", "BAD-KEY=value",
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected invalid env key error")
	}
	if !strings.Contains(err.Error(), "env key") {
		t.Fatalf("error = %v", err)
	}
}

func TestMCPAddRejectsMalformedSecretRefAndDoesNotSaveServer(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	createProfileForTest(t, home, t.TempDir())

	out := &bytes.Buffer{}
	cmd := NewRootCommand(out, out)
	cmd.SetArgs([]string{
		"--home", home,
		"mcp", "add", "mgcs", "github",
		"--command", "npx",
		"--env", "GH_PAT=${secret:bad-name}",
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected malformed secret ref error")
	}
	if !strings.Contains(err.Error(), "malformed secret reference") {
		t.Fatalf("error = %v", err)
	}

	prof := loadProfileForMCPTest(t, home, "mgcs")
	if _, ok := prof.MCP["github"]; ok {
		t.Fatalf("server was saved after invalid env: %#v", prof.MCP["github"])
	}
}

func TestMCPAddAcceptsSecretRefTextInsideLiteral(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	createProfileForTest(t, home, t.TempDir())

	out := &bytes.Buffer{}
	cmd := NewRootCommand(out, out)
	cmd.SetArgs([]string{
		"--home", home,
		"mcp", "add", "mgcs", "notes",
		"--command", "npx",
		"--env", "NOTE=prefix-${secret:GITHUB_TOKEN}",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mcp add returned error: %v", err)
	}

	prof := loadProfileForMCPTest(t, home, "mgcs")
	if got := prof.MCP["notes"].Env["NOTE"]; got != "prefix-${secret:GITHUB_TOKEN}" {
		t.Fatalf("NOTE env = %q", got)
	}
}

func TestMCPAddRejectsMissingCommand(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	createProfileForTest(t, home, t.TempDir())

	out := &bytes.Buffer{}
	cmd := NewRootCommand(out, out)
	cmd.SetArgs([]string{"--home", home, "mcp", "add", "mgcs", "github"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected missing command error")
	}
	if !strings.Contains(err.Error(), "command must not be empty") {
		t.Fatalf("error = %v", err)
	}
}

func TestMCPAddRejectsMalformedServerNames(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	createProfileForTest(t, home, t.TempDir())

	tests := []struct {
		name       string
		serverName string
	}{
		{name: "blank", serverName: ""},
		{name: "path separator", serverName: "bad/name"},
		{name: "traversal", serverName: "../github"},
		{name: "control", serverName: "bad\nname"},
		{name: "leading punctuation", serverName: ".github"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			cmd := NewRootCommand(out, out)
			cmd.SetArgs([]string{
				"--home", home,
				"mcp", "add", "mgcs", tt.serverName,
				"--command", "npx",
			})
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected malformed MCP server name error")
			}
			if !strings.Contains(err.Error(), "MCP server name") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestMCPSyncPrintsUnsupportedServerNotesToStderr(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createOut := &bytes.Buffer{}
	create := NewRootCommand(createOut, createOut)
	create.SetArgs([]string{
		"--home", home,
		"profile", "create",
		"--name", "mgcs",
		"--project-dir", project,
		"--safety", "standard",
		"--tools", "codex",
		"--yes",
	})
	if err := create.Execute(); err != nil {
		t.Fatalf("profile create returned error: %v", err)
	}
	mustRunMCPTest(t, home, "mcp", "add", "mgcs", "remote", "--type", "http", "--command", "npx")

	syncOut := &bytes.Buffer{}
	syncErr := &bytes.Buffer{}
	syncCmd := NewRootCommand(syncOut, syncErr)
	syncCmd.SetArgs([]string{"--home", home, "mcp", "sync", "mgcs"})
	if err := syncCmd.Execute(); err != nil {
		t.Fatalf("mcp sync returned error: %v", err)
	}
	if strings.TrimSpace(syncOut.String()) != "Synced MCP servers for mgcs" {
		t.Fatalf("sync stdout = %q", syncOut.String())
	}
	if !strings.Contains(syncErr.String(), `MCP server "remote" uses unsupported type "http"; skipping`) {
		t.Fatalf("sync stderr = %q", syncErr.String())
	}
}

func TestMCPSyncRendersGeminiAndCopilot(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createOut := &bytes.Buffer{}
	create := NewRootCommand(createOut, createOut)
	create.SetArgs([]string{
		"--home", home,
		"profile", "create",
		"--name", "mgcs",
		"--project-dir", project,
		"--safety", "standard",
		"--tools", "gemini,copilot",
		"--yes",
	})
	if err := create.Execute(); err != nil {
		t.Fatalf("profile create returned error: %v", err)
	}
	mustRunMCPTest(t, home, "mcp", "add", "mgcs", "docs", "--command", "npx", "--env", "REGION=us-east-1")

	syncOut := &bytes.Buffer{}
	syncErr := &bytes.Buffer{}
	syncCmd := NewRootCommand(syncOut, syncErr)
	syncCmd.SetArgs([]string{"--home", home, "mcp", "sync", "mgcs"})
	if err := syncCmd.Execute(); err != nil {
		t.Fatalf("mcp sync returned error: %v", err)
	}

	geminiSettings, err := os.ReadFile(filepath.Join(home, "profiles", "mgcs", "tools", "gemini", ".gemini", "settings.json"))
	if err != nil {
		t.Fatalf("ReadFile gemini settings returned error: %v", err)
	}
	if !strings.Contains(string(geminiSettings), `"mcpServers"`) || !strings.Contains(string(geminiSettings), `"REGION": "us-east-1"`) {
		t.Fatalf("gemini settings = %s", geminiSettings)
	}

	copilotConfig, err := os.ReadFile(filepath.Join(home, "profiles", "mgcs", "tools", "copilot", "mcp-config.json"))
	if err != nil {
		t.Fatalf("ReadFile copilot config returned error: %v", err)
	}
	if !strings.Contains(string(copilotConfig), `"type": "local"`) {
		t.Fatalf("copilot config = %s", copilotConfig)
	}
}

func TestMCPSyncCopilotSecretsGatedByFlag(t *testing.T) {
	keyring.MockInit()
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createOut := &bytes.Buffer{}
	create := NewRootCommand(createOut, createOut)
	create.SetArgs([]string{
		"--home", home,
		"profile", "create",
		"--name", "mgcs",
		"--project-dir", project,
		"--safety", "standard",
		"--tools", "copilot",
		"--yes",
	})
	if err := create.Execute(); err != nil {
		t.Fatalf("profile create returned error: %v", err)
	}
	setSecretForMCPTest(t, home, "mgcs", "GITHUB_TOKEN", "ghp_value")
	mustRunMCPTest(t, home, "mcp", "add", "mgcs", "github", "--command", "npx", "--env", "GH_PAT=${secret:GITHUB_TOKEN}")

	syncOut := &bytes.Buffer{}
	syncErr := &bytes.Buffer{}
	syncCmd := NewRootCommand(syncOut, syncErr)
	syncCmd.SetArgs([]string{"--home", home, "mcp", "sync", "mgcs"})
	if err := syncCmd.Execute(); err != nil {
		t.Fatalf("mcp sync returned error: %v", err)
	}
	if !strings.Contains(syncErr.String(), `GitHub Copilot MCP server "github" uses secret env; skipping plaintext config`) {
		t.Fatalf("sync stderr = %q", syncErr.String())
	}
	copilotConfig, err := os.ReadFile(filepath.Join(home, "profiles", "mgcs", "tools", "copilot", "mcp-config.json"))
	if err != nil {
		t.Fatalf("ReadFile copilot config returned error: %v", err)
	}
	if strings.Contains(string(copilotConfig), "ghp_value") {
		t.Fatalf("copilot config leaked secret without flag: %s", copilotConfig)
	}

	syncPlainOut := &bytes.Buffer{}
	syncPlainErr := &bytes.Buffer{}
	syncPlainCmd := NewRootCommand(syncPlainOut, syncPlainErr)
	syncPlainCmd.SetArgs([]string{"--home", home, "mcp", "sync", "mgcs", "--allow-plaintext-secrets"})
	if err := syncPlainCmd.Execute(); err != nil {
		t.Fatalf("mcp sync --allow-plaintext-secrets returned error: %v", err)
	}
	copilotConfig, err = os.ReadFile(filepath.Join(home, "profiles", "mgcs", "tools", "copilot", "mcp-config.json"))
	if err != nil {
		t.Fatalf("ReadFile copilot config returned error: %v", err)
	}
	if !strings.Contains(string(copilotConfig), `"GH_PAT": "ghp_value"`) {
		t.Fatalf("copilot config = %s", copilotConfig)
	}
}

func loadProfileForMCPTest(t *testing.T, home, name string) profile.Profile {
	t.Helper()
	prof, err := profile.NewStore(paths.ForRoot(home)).Load(name)
	if err != nil {
		t.Fatalf("Load profile returned error: %v", err)
	}
	return prof
}

func mustRunMCPTest(t *testing.T, home string, args ...string) string {
	t.Helper()
	out := &bytes.Buffer{}
	cmd := NewRootCommand(out, out)
	cmd.SetArgs(append([]string{"--home", home}, args...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v returned error: %v", args, err)
	}
	return out.String()
}

func setSecretForMCPTest(t *testing.T, home, profileName, secretName, value string) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := NewRootCommand(&out, &errOut)
	cmd.SetIn(strings.NewReader(value + "\n"))
	cmd.SetArgs([]string{"--home", home, "secret", "set", profileName, secretName, "--stdin"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("secret set returned error: %v\n%s", err, errOut.String())
	}
}

func TestMCPAddStoresReadOnlyForReadOnlyProfile(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	createProfileForTest(t, home, t.TempDir())

	mustRunMCPTest(t, home, "mcp", "add", "mgcs", "localfs", "--command", "node")

	prof := loadProfileForMCPTest(t, home, "mgcs")
	if !prof.MCP["localfs"].ReadOnly {
		t.Fatalf("read-only profile should save server as read-only: %#v", prof.MCP["localfs"])
	}
}

func TestMCPCommandsValidateProfileExists(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "add",
			args: []string{"--home", home, "mcp", "add", "missing", "github", "--command", "npx"},
		},
		{
			name: "list",
			args: []string{"--home", home, "mcp", "list", "missing"},
		},
		{
			name: "sync",
			args: []string{"--home", home, "mcp", "sync", "missing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			cmd := NewRootCommand(out, out)
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected missing profile error")
			}
			if !strings.Contains(err.Error(), `Profile "missing" does not exist.`) {
				t.Fatalf("error = %v", err)
			}
			if !strings.Contains(err.Error(), "aip profile create --name missing") || !strings.Contains(err.Error(), "aip profile list") {
				t.Fatalf("error missing profile guidance: %v", err)
			}
		})
	}
}

func TestMCPAddStoresType(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	createProfileForTest(t, home, t.TempDir())

	mustRunMCPTest(t, home, "mcp", "add", "mgcs", "remote", "--type", "stdio", "--command", "npx")

	prof := loadProfileForMCPTest(t, home, "mgcs")
	if prof.MCP["remote"].Type != "stdio" {
		t.Fatalf("server type = %q", prof.MCP["remote"].Type)
	}
}
