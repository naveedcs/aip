package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/naveedcs/aip/internal/paths"
	"github.com/naveedcs/aip/internal/profile"
	"github.com/naveedcs/aip/internal/tools"
)

func TestSecretRefsExtractsTargetsDeterministically(t *testing.T) {
	prof := profile.Profile{
		MCP: map[string]profile.MCPServer{
			"zeta": {
				Env: map[string]string{
					"Z_SECRET": "${secret:ZED}",
					"A_SECRET": "${secret:ALPHA}",
					"LITERAL":  "plain",
				},
			},
			"alpha": {
				Env: map[string]string{
					"DB_TOKEN": "${secret:DATABASE_TOKEN}",
				},
			},
		},
	}

	got := SecretRefs(prof)
	want := []SecretRef{
		{Target: "DB_TOKEN", SecretName: "DATABASE_TOKEN"},
		{Target: "A_SECRET", SecretName: "ALPHA"},
		{Target: "Z_SECRET", SecretName: "ZED"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SecretRefs = %#v, want %#v", got, want)
	}
}

func TestLaunchSecretRefsUseRenderableServersForSupportedTools(t *testing.T) {
	prof := profile.Profile{
		MCP: map[string]profile.MCPServer{
			"remote": {
				Type:    "http",
				Command: "remote",
				Env:     map[string]string{"REMOTE_TOKEN": "${secret:REMOTE_TOKEN}"},
			},
			"stdio": {
				Type:    "stdio",
				Command: "npx",
				Env: map[string]string{
					"STDIO_TOKEN": "${secret:STDIO_TOKEN}",
					"LITERAL":     "plain",
				},
			},
		},
	}
	want := []SecretRef{{Target: "STDIO_TOKEN", SecretName: "STDIO_TOKEN"}}

	for _, toolID := range []tools.ID{tools.Codex, tools.Claude, tools.Gemini} {
		if got := LaunchSecretRefs(prof, toolID); !reflect.DeepEqual(got, want) {
			t.Fatalf("LaunchSecretRefs(%s) = %#v, want %#v", toolID, got, want)
		}
	}
	for _, toolID := range []tools.ID{tools.Copilot} {
		if got := LaunchSecretRefs(prof, toolID); got != nil {
			t.Fatalf("LaunchSecretRefs(%s) = %#v, want nil", toolID, got)
		}
	}
}

func TestRenderCodexUsesSecretEnvVarsAndPrivateFile(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))

	notes, err := Render(p, sampleRenderProfile())
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("Render notes = %#v, want none", notes)
	}

	path := filepath.Join(p.ToolConfigDir("mgcs", string(tools.Codex)), "config.toml")
	data := readFile(t, path)
	text := string(data)
	for _, want := range []string{
		`cli_auth_credentials_store = "file"`,
		`[mcp_servers.github]`,
		`command = "npx"`,
		`args = ["-y", "@modelcontextprotocol/server-github"]`,
		`env_vars = ["GITHUB_TOKEN"]`,
		`[mcp_servers.github.env]`,
		`GITHUB_OWNER = "octo"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("codex config missing %q:\n%s", want, text)
		}
	}
	for _, forbidden := range []string{"secret:", "GITHUB_PAT", "${GITHUB_TOKEN}", "read_only"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("codex config contains forbidden %q:\n%s", forbidden, text)
		}
	}
	assertFileMode(t, path, 0o600)
}

func TestRenderClaudeUsesTargetPlaceholdersAndLiteralEnv(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))

	if _, err := Render(p, sampleRenderProfile()); err != nil {
		t.Fatalf("Render returned error: %v", err)
	}

	path := filepath.Join(p.ToolConfigDir("mgcs", string(tools.Claude)), "mcp.json")
	data := readFile(t, path)
	var cfg struct {
		MCPServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args,omitempty"`
			Env     map[string]string `json:"env,omitempty"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal claude config returned error: %v\n%s", err, string(data))
	}

	github := cfg.MCPServers["github"]
	if github.Command != "npx" {
		t.Fatalf("github command = %q, want npx", github.Command)
	}
	if !reflect.DeepEqual(github.Args, []string{"-y", "@modelcontextprotocol/server-github"}) {
		t.Fatalf("github args = %#v", github.Args)
	}
	wantEnv := map[string]string{
		"GITHUB_TOKEN": "${GITHUB_TOKEN}",
		"GITHUB_OWNER": "octo",
	}
	if !reflect.DeepEqual(github.Env, wantEnv) {
		t.Fatalf("github env = %#v, want %#v", github.Env, wantEnv)
	}

	text := string(data)
	for _, forbidden := range []string{"secret:", "GITHUB_PAT", "readOnly"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("claude config contains forbidden %q:\n%s", forbidden, text)
		}
	}
	assertFileMode(t, path, 0o600)
}

func TestLaunchArgs(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := sampleRenderProfile()

	got := LaunchArgs(p, prof, tools.Claude)
	want := []string{"--mcp-config", filepath.Join(p.ToolConfigDir("mgcs", string(tools.Claude)), "mcp.json")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LaunchArgs Claude = %#v, want %#v", got, want)
	}
	if got := LaunchArgs(p, prof, tools.Codex); got != nil {
		t.Fatalf("LaunchArgs Codex = %#v, want nil", got)
	}
	prof.MCP = nil
	if got := LaunchArgs(p, prof, tools.Claude); got != nil {
		t.Fatalf("LaunchArgs Claude without servers = %#v, want nil", got)
	}
}

func TestLaunchArgsReturnsNilWhenClaudeHasOnlyUnsupportedServers(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := sampleRenderProfile()
	prof.MCP = map[string]profile.MCPServer{
		"remote-http": {Type: "http", Command: "remote"},
		"remote-sse":  {Type: "sse", Command: "remote"},
	}

	if got := LaunchArgs(p, prof, tools.Claude); got != nil {
		t.Fatalf("LaunchArgs Claude with only unsupported servers = %#v, want nil", got)
	}
}

func TestRenderCopilotRendersLiteralEnvAndSkipsSecretServers(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := sampleRenderProfile()
	prof.Tools = map[tools.ID]profile.ToolConfig{tools.Copilot: {Enabled: true}}
	prof.MCP["docs"] = profile.MCPServer{
		Type:    "stdio",
		Command: "docs-mcp",
		Args:    []string{"--serve"},
		Env:     map[string]string{"REGION": "us-east-1"},
	}

	notes, err := Render(p, prof)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if !notesContain(notes, "Copilot") || !notesContain(notes, "github") {
		t.Fatalf("notes = %#v, want Copilot github skip note", notes)
	}

	path := filepath.Join(p.ToolConfigDir("mgcs", string(tools.Copilot)), "mcp-config.json")
	data := readFile(t, path)
	var cfg struct {
		MCPServers map[string]struct {
			Type    string            `json:"type"`
			Command string            `json:"command"`
			Args    []string          `json:"args,omitempty"`
			Env     map[string]string `json:"env,omitempty"`
			Tools   []string          `json:"tools,omitempty"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal copilot config returned error: %v\n%s", err, string(data))
	}
	if _, ok := cfg.MCPServers["github"]; ok {
		t.Fatalf("github server rendered despite secret env: %#v", cfg.MCPServers)
	}
	docs, ok := cfg.MCPServers["docs"]
	if !ok {
		t.Fatalf("docs server missing: %#v", cfg.MCPServers)
	}
	if docs.Type != "local" {
		t.Fatalf("docs type = %q, want local", docs.Type)
	}
	if docs.Command != "docs-mcp" {
		t.Fatalf("docs command = %q, want docs-mcp", docs.Command)
	}
	if !reflect.DeepEqual(docs.Args, []string{"--serve"}) {
		t.Fatalf("docs args = %#v", docs.Args)
	}
	if !reflect.DeepEqual(docs.Env, map[string]string{"REGION": "us-east-1"}) {
		t.Fatalf("docs env = %#v", docs.Env)
	}
	if !reflect.DeepEqual(docs.Tools, []string{"*"}) {
		t.Fatalf("docs tools = %#v, want [*]", docs.Tools)
	}
	text := string(data)
	for _, forbidden := range []string{"${", "secret:", "GITHUB_PAT"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("copilot config contains forbidden %q:\n%s", forbidden, text)
		}
	}
	assertFileMode(t, path, 0o600)
}

func TestRenderCopilotInjectsLiteralSecretsWhenAllowed(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := sampleRenderProfile()
	prof.Tools = map[tools.ID]profile.ToolConfig{tools.Copilot: {Enabled: true}}

	notes, err := RenderWithOptions(p, prof, RenderOptions{
		AllowPlaintextSecrets: true,
		ResolveSecret: func(secretName string) (string, error) {
			if secretName != "GITHUB_PAT" {
				t.Fatalf("ResolveSecret(%q), want GITHUB_PAT", secretName)
			}
			return "ghp_literal", nil
		},
	})
	if err != nil {
		t.Fatalf("RenderWithOptions returned error: %v", err)
	}
	if notesContain(notes, "github") {
		t.Fatalf("notes = %#v, want no github skip note", notes)
	}

	path := filepath.Join(p.ToolConfigDir("mgcs", string(tools.Copilot)), "mcp-config.json")
	data := readFile(t, path)
	var cfg struct {
		MCPServers map[string]struct {
			Env map[string]string `json:"env,omitempty"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal copilot config returned error: %v\n%s", err, string(data))
	}
	github, ok := cfg.MCPServers["github"]
	if !ok {
		t.Fatalf("github server missing: %#v", cfg.MCPServers)
	}
	if got := github.Env["GITHUB_TOKEN"]; got != "ghp_literal" {
		t.Fatalf("GITHUB_TOKEN = %q, want ghp_literal", got)
	}
}

func TestRenderCopilotSkipsUnsupportedTransportsWithNotes(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := sampleRenderProfile()
	prof.Tools = map[tools.ID]profile.ToolConfig{tools.Copilot: {Enabled: true}}
	prof.MCP = map[string]profile.MCPServer{
		"docs": {
			Type:    "stdio",
			Command: "docs-mcp",
			Env:     map[string]string{"REGION": "us-east-1"},
		},
		"remote-http": {Type: "http", Command: "remote-http"},
		"remote-sse":  {Type: "sse", Command: "remote-sse"},
	}

	notes, err := Render(p, prof)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if !notesContain(notes, "remote-http") || !notesContain(notes, "http") {
		t.Fatalf("notes = %#v, want http server note", notes)
	}
	if !notesContain(notes, "remote-sse") || !notesContain(notes, "sse") {
		t.Fatalf("notes = %#v, want sse server note", notes)
	}

	path := filepath.Join(p.ToolConfigDir("mgcs", string(tools.Copilot)), "mcp-config.json")
	data := readFile(t, path)
	text := string(data)
	if !strings.Contains(text, `"docs"`) {
		t.Fatalf("copilot config missing renderable docs server:\n%s", text)
	}
	for _, forbidden := range []string{"remote-http", "remote-sse"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("unsupported server %q rendered:\n%s", forbidden, text)
		}
	}
}

func TestRenderWithOptionsSkipCopilotLeavesConfigUntouched(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := sampleRenderProfile()
	prof.Tools = map[tools.ID]profile.ToolConfig{tools.Copilot: {Enabled: true}}

	path := filepath.Join(p.ToolConfigDir("mgcs", string(tools.Copilot)), "mcp-config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll copilot returned error: %v", err)
	}
	existing := []byte(`{"existing":true}`)
	if err := os.WriteFile(path, existing, 0o600); err != nil {
		t.Fatalf("WriteFile copilot returned error: %v", err)
	}

	if _, err := RenderWithOptions(p, prof, RenderOptions{SkipCopilot: true}); err != nil {
		t.Fatalf("RenderWithOptions returned error: %v", err)
	}
	if got := string(readFile(t, path)); got != string(existing) {
		t.Fatalf("copilot config was overwritten: %q", got)
	}
}

func TestRenderWritesGeminiSettingsWithReferences(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := sampleRenderProfile()
	prof.Tools[tools.Gemini] = profile.ToolConfig{Enabled: true}
	prof.Tools[tools.Copilot] = profile.ToolConfig{Enabled: true}

	notes, err := Render(p, prof)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if notesContain(notes, "Gemini") {
		t.Fatalf("notes = %#v, want no Gemini skip note", notes)
	}
	if !notesContain(notes, "Copilot") {
		t.Fatalf("notes = %#v, want Copilot skip note", notes)
	}

	path := filepath.Join(p.ToolConfigDir("mgcs", string(tools.Gemini)), ".gemini", "settings.json")
	data := readFile(t, path)
	var cfg struct {
		MCPServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args,omitempty"`
			Env     map[string]string `json:"env,omitempty"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal gemini settings returned error: %v\n%s", err, string(data))
	}

	github := cfg.MCPServers["github"]
	if github.Command != "npx" {
		t.Fatalf("github command = %q, want npx", github.Command)
	}
	if !reflect.DeepEqual(github.Args, []string{"-y", "@modelcontextprotocol/server-github"}) {
		t.Fatalf("github args = %#v", github.Args)
	}
	wantEnv := map[string]string{
		"GITHUB_TOKEN": "${GITHUB_TOKEN}",
		"GITHUB_OWNER": "octo",
	}
	if !reflect.DeepEqual(github.Env, wantEnv) {
		t.Fatalf("github env = %#v, want %#v", github.Env, wantEnv)
	}

	text := string(data)
	for _, forbidden := range []string{"secret:", "GITHUB_PAT", "readOnly"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("gemini settings contains forbidden %q:\n%s", forbidden, text)
		}
	}
	assertFileMode(t, path, 0o600)
}

func TestRenderMergesIntoExistingGeminiSettings(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := sampleRenderProfile()
	prof.Tools = map[tools.ID]profile.ToolConfig{tools.Gemini: {Enabled: true}}

	path := filepath.Join(p.ToolConfigDir("mgcs", string(tools.Gemini)), ".gemini", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll gemini returned error: %v", err)
	}
	existing := []byte(`{
  "theme": "dark",
  "mcpServers": {
    "stale": {
      "command": "old"
    }
  }
}`)
	if err := os.WriteFile(path, existing, 0o600); err != nil {
		t.Fatalf("WriteFile gemini returned error: %v", err)
	}

	notes, err := Render(p, prof)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("Render notes = %#v, want none", notes)
	}

	data := readFile(t, path)
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal gemini settings returned error: %v\n%s", err, string(data))
	}
	var theme string
	if err := json.Unmarshal(cfg["theme"], &theme); err != nil {
		t.Fatalf("theme missing or invalid: %v\n%s", err, string(data))
	}
	if theme != "dark" {
		t.Fatalf("theme = %q, want dark", theme)
	}
	var servers map[string]struct {
		Command string            `json:"command"`
		Args    []string          `json:"args,omitempty"`
		Env     map[string]string `json:"env,omitempty"`
	}
	if err := json.Unmarshal(cfg["mcpServers"], &servers); err != nil {
		t.Fatalf("mcpServers missing or invalid: %v\n%s", err, string(data))
	}
	if _, ok := servers["stale"]; ok {
		t.Fatalf("stale mcpServers entry preserved: %#v", servers)
	}
	github := servers["github"]
	if github.Command != "npx" {
		t.Fatalf("github command = %q, want npx", github.Command)
	}
	if !reflect.DeepEqual(github.Args, []string{"-y", "@modelcontextprotocol/server-github"}) {
		t.Fatalf("github args = %#v", github.Args)
	}
	wantEnv := map[string]string{
		"GITHUB_TOKEN": "${GITHUB_TOKEN}",
		"GITHUB_OWNER": "octo",
	}
	if !reflect.DeepEqual(github.Env, wantEnv) {
		t.Fatalf("github env = %#v, want %#v", github.Env, wantEnv)
	}
}

func TestRenderSkipsHTTPAndSSEServersWithNotes(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := sampleRenderProfile()
	prof.MCP["remote-http"] = profile.MCPServer{Type: "http", Command: "remote"}
	prof.MCP["remote-sse"] = profile.MCPServer{Type: "sse", Command: "remote"}

	notes, err := Render(p, prof)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if !notesContain(notes, "remote-http") || !notesContain(notes, "http") {
		t.Fatalf("notes = %#v, want http server note", notes)
	}
	if !notesContain(notes, "remote-sse") || !notesContain(notes, "sse") {
		t.Fatalf("notes = %#v, want sse server note", notes)
	}

	codex := string(readFile(t, filepath.Join(p.ToolConfigDir("mgcs", string(tools.Codex)), "config.toml")))
	claude := string(readFile(t, filepath.Join(p.ToolConfigDir("mgcs", string(tools.Claude)), "mcp.json")))
	for _, forbidden := range []string{"remote-http", "remote-sse"} {
		if strings.Contains(codex, forbidden) || strings.Contains(claude, forbidden) {
			t.Fatalf("unsupported server %q rendered:\ncodex:\n%s\nclaude:\n%s", forbidden, codex, claude)
		}
	}
}

func TestRenderRejectsMalformedSecretRefBeforeWritingConfigs(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := sampleRenderProfile()
	prof.MCP["github"].Env["GITHUB_TOKEN"] = "${secret:bad-name}"

	codexPath := filepath.Join(p.ToolConfigDir("mgcs", string(tools.Codex)), "config.toml")
	claudePath := filepath.Join(p.ToolConfigDir("mgcs", string(tools.Claude)), "mcp.json")
	if err := os.MkdirAll(filepath.Dir(codexPath), 0o700); err != nil {
		t.Fatalf("MkdirAll codex returned error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(claudePath), 0o700); err != nil {
		t.Fatalf("MkdirAll claude returned error: %v", err)
	}
	if err := os.WriteFile(codexPath, []byte("existing codex"), 0o600); err != nil {
		t.Fatalf("WriteFile codex returned error: %v", err)
	}
	if err := os.WriteFile(claudePath, []byte("existing claude"), 0o600); err != nil {
		t.Fatalf("WriteFile claude returned error: %v", err)
	}

	_, err := Render(p, prof)
	if err == nil {
		t.Fatal("expected malformed secret ref error")
	}
	if !strings.Contains(err.Error(), "malformed secret reference") {
		t.Fatalf("error = %v", err)
	}
	if got := string(readFile(t, codexPath)); got != "existing codex" {
		t.Fatalf("codex config was overwritten: %q", got)
	}
	if got := string(readFile(t, claudePath)); got != "existing claude" {
		t.Fatalf("claude config was overwritten: %q", got)
	}
}

func TestRenderTreatsSecretRefTextInsideLiteralsAsLiteralEnv(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := sampleRenderProfile()
	prof.MCP["github"].Env["NOTE"] = "literal ${secret:bad-name}"

	if _, err := Render(p, prof); err != nil {
		t.Fatalf("Render returned error: %v", err)
	}

	codex := string(readFile(t, filepath.Join(p.ToolConfigDir("mgcs", string(tools.Codex)), "config.toml")))
	claude := string(readFile(t, filepath.Join(p.ToolConfigDir("mgcs", string(tools.Claude)), "mcp.json")))
	if !strings.Contains(codex, `NOTE = "literal ${secret:bad-name}"`) {
		t.Fatalf("codex config = %s", codex)
	}
	if !strings.Contains(claude, `"NOTE": "literal ${secret:bad-name}"`) {
		t.Fatalf("claude mcp = %s", claude)
	}
}

func TestRenderCodexEscapesControlCharacters(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := sampleRenderProfile()
	prof.MCP = map[string]profile.MCPServer{
		"control": {
			Command: "nul\x00cmd",
			Env:     map[string]string{"TOKEN": "value\x00with-nul"},
		},
	}

	if _, err := Render(p, prof); err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	text := string(readFile(t, filepath.Join(p.ToolConfigDir("mgcs", string(tools.Codex)), "config.toml")))
	if strings.ContainsRune(text, '\x00') {
		t.Fatalf("rendered TOML contains raw NUL: %q", text)
	}
	if !strings.Contains(text, `command = "nul\u0000cmd"`) {
		t.Fatalf("command was not TOML-escaped correctly:\n%s", text)
	}
	if !strings.Contains(text, `TOKEN = "value\u0000with-nul"`) {
		t.Fatalf("env value was not TOML-escaped correctly:\n%s", text)
	}
}

func sampleRenderProfile() profile.Profile {
	return profile.Profile{
		Name:        "mgcs",
		ProjectDir:  ".",
		SafetyLevel: profile.Standard,
		Tools: map[tools.ID]profile.ToolConfig{
			tools.Codex:  {Enabled: true},
			tools.Claude: {Enabled: true},
		},
		MCP: map[string]profile.MCPServer{
			"github": {
				Type:    "stdio",
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-github"},
				Env: map[string]string{
					"GITHUB_TOKEN": "${secret:GITHUB_PAT}",
					"GITHUB_OWNER": "octo",
				},
				ReadOnly: true,
			},
		},
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) returned error: %v", path, err)
	}
	return data
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%s) returned error: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %v, want %v", path, got, want)
	}
}

func notesContain(notes []string, want string) bool {
	for _, note := range notes {
		if strings.Contains(note, want) {
			return true
		}
	}
	return false
}
