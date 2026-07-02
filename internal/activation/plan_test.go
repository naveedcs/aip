package activation

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/naveedcs/aip/internal/honcho"
	"github.com/naveedcs/aip/internal/paths"
	"github.com/naveedcs/aip/internal/profile"
	"github.com/naveedcs/aip/internal/secrets"
	"github.com/naveedcs/aip/internal/tools"
)

func setup(t *testing.T) (paths.Paths, *secrets.Memory) {
	t.Helper()

	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := profile.Profile{
		Name:        "acme",
		ProjectDir:  t.TempDir(),
		SafetyLevel: profile.Standard,
		AuthMode:    profile.AuthAPIKey,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Codex: {Enabled: true}},
		Secrets:     profile.SecretConfig{Keys: []string{"GITHUB_TOKEN", "OPENAI_API_KEY"}},
	}
	if err := profile.NewStore(p).Save(prof); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	return p, secrets.NewMemory()
}

type countingProvider struct {
	inner  *secrets.Memory
	counts map[string]int
}

func newCountingProvider() *countingProvider {
	return &countingProvider{
		inner:  secrets.NewMemory(),
		counts: map[string]int{},
	}
}

func (p *countingProvider) Get(profileName, name string) (string, error) {
	p.counts[name]++
	return p.inner.Get(profileName, name)
}

func (p *countingProvider) Set(profileName, name, value string) error {
	return p.inner.Set(profileName, name, value)
}

func (p *countingProvider) Delete(profileName, name string) error {
	return p.inner.Delete(profileName, name)
}

func TestBuildSetsConfigDirAndInjectsSecrets(t *testing.T) {
	parentHome := t.TempDir()
	t.Setenv("HOME", parentHome)

	p, sp := setup(t)
	if err := sp.Set("acme", "GITHUB_TOKEN", "ghp_secret"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err := sp.Set("acme", "OPENAI_API_KEY", "sk-openai"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	plan, err := Build(p, sp, "acme", tools.Codex, []string{"--help"})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	wantDir := p.ToolConfigDir("acme", string(tools.Codex))
	if plan.Env["CODEX_HOME"] != wantDir {
		t.Fatalf("CODEX_HOME = %q, want %q", plan.Env["CODEX_HOME"], wantDir)
	}
	if plan.Env["AIP_PROFILE"] != "acme" {
		t.Fatalf("AIP_PROFILE = %q, want acme", plan.Env["AIP_PROFILE"])
	}
	if plan.Env["GITHUB_TOKEN"] != "ghp_secret" {
		t.Fatalf("GITHUB_TOKEN = %q, want injected secret", plan.Env["GITHUB_TOKEN"])
	}
	if plan.Env["OPENAI_API_KEY"] != "sk-openai" {
		t.Fatalf("OPENAI_API_KEY = %q, want injected secret", plan.Env["OPENAI_API_KEY"])
	}
	if plan.Env["HOME"] != parentHome {
		t.Fatalf("HOME = %q, want preserved parent HOME %q", plan.Env["HOME"], parentHome)
	}
	if plan.MaskedEnv["GITHUB_TOKEN"] != "********" {
		t.Fatalf("masked GITHUB_TOKEN = %q, want ********", plan.MaskedEnv["GITHUB_TOKEN"])
	}
	if plan.MaskedEnv["OPENAI_API_KEY"] != "********" {
		t.Fatalf("masked OPENAI_API_KEY = %q, want ********", plan.MaskedEnv["OPENAI_API_KEY"])
	}
	if plan.Command != "codex" {
		t.Fatalf("Command = %q, want codex", plan.Command)
	}
}

func TestBuildInjectsMCPSecretAndClaudeLaunchArgs(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := profile.Profile{
		Name:        "acme",
		ProjectDir:  t.TempDir(),
		SafetyLevel: profile.Standard,
		AuthMode:    profile.AuthSubscription,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Claude: {Enabled: true}},
		Secrets:     profile.SecretConfig{Keys: []string{"GITHUB_TOKEN"}},
		MCP: map[string]profile.MCPServer{
			"github": {
				Command: "npx",
				Env:     map[string]string{"GH_PAT": "${secret:GITHUB_TOKEN}"},
			},
		},
	}
	if err := profile.NewStore(p).Save(prof); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	sp := secrets.NewMemory()
	if err := sp.Set("acme", "GITHUB_TOKEN", "ghp_secret"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	plan, err := Build(p, sp, "acme", tools.Claude, []string{"chat"})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	if plan.Env["GH_PAT"] != "ghp_secret" {
		t.Fatalf("GH_PAT = %q, want MCP secret value", plan.Env["GH_PAT"])
	}
	if plan.MaskedEnv["GH_PAT"] != "********" {
		t.Fatalf("masked GH_PAT = %q, want ********", plan.MaskedEnv["GH_PAT"])
	}
	wantMCPPath := filepath.Join(p.ToolConfigDir("acme", string(tools.Claude)), "mcp.json")
	wantArgs := []string{"--mcp-config", wantMCPPath, "chat"}
	if len(plan.Args) != len(wantArgs) {
		t.Fatalf("Args = %#v, want %#v", plan.Args, wantArgs)
	}
	for i := range wantArgs {
		if plan.Args[i] != wantArgs[i] {
			t.Fatalf("Args = %#v, want %#v", plan.Args, wantArgs)
		}
	}
}

func TestBuildInjectsHonchoEnvAndMasksAPIKey(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := profile.Profile{
		Name:        "acme",
		ProjectDir:  t.TempDir(),
		SafetyLevel: profile.Standard,
		AuthMode:    profile.AuthSubscription,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Claude: {Enabled: true}},
		Honcho: profile.HonchoConfig{
			Enabled:      true,
			WorkspaceID:  "acme-ws",
			UserName:     "naveed",
			APIKeySecret: "HONCHO_API_KEY",
		},
	}
	server, ok := honcho.MCPServer(prof)
	if !ok {
		t.Fatal("MCPServer ok=false for enabled Honcho profile")
	}
	prof.MCP = map[string]profile.MCPServer{honcho.MCPServerName: server}
	if err := profile.NewStore(p).Save(prof); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	sp := secrets.NewMemory()
	if err := sp.Set("acme", "HONCHO_API_KEY", "hch-secret"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	plan, err := Build(p, sp, "acme", tools.Claude, nil)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	if plan.Env["HONCHO_WORKSPACE_ID"] != "acme-ws" {
		t.Fatalf("HONCHO_WORKSPACE_ID = %q, want acme-ws", plan.Env["HONCHO_WORKSPACE_ID"])
	}
	if plan.Env["HONCHO_BASE_URL"] != "https://api.honcho.dev" {
		t.Fatalf("HONCHO_BASE_URL = %q, want https://api.honcho.dev", plan.Env["HONCHO_BASE_URL"])
	}
	if plan.Env["HONCHO_API_KEY"] != "hch-secret" {
		t.Fatalf("HONCHO_API_KEY = %q, want Honcho API key", plan.Env["HONCHO_API_KEY"])
	}
	if plan.MaskedEnv["HONCHO_API_KEY"] != "********" {
		t.Fatalf("masked HONCHO_API_KEY = %q, want ********", plan.MaskedEnv["HONCHO_API_KEY"])
	}
	if plan.MaskedEnv["HONCHO_WORKSPACE_ID"] != "acme-ws" {
		t.Fatalf("masked HONCHO_WORKSPACE_ID = %q, want acme-ws", plan.MaskedEnv["HONCHO_WORKSPACE_ID"])
	}
	if plan.MaskedEnv["HONCHO_BASE_URL"] != "https://api.honcho.dev" {
		t.Fatalf("masked HONCHO_BASE_URL = %q, want https://api.honcho.dev", plan.MaskedEnv["HONCHO_BASE_URL"])
	}
}

func TestBuildReusesDeclaredSecretForMultipleMCPTargets(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := profile.Profile{
		Name:        "acme",
		ProjectDir:  t.TempDir(),
		SafetyLevel: profile.Standard,
		AuthMode:    profile.AuthSubscription,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Claude: {Enabled: true}},
		Secrets:     profile.SecretConfig{Keys: []string{"GITHUB_TOKEN"}},
		MCP: map[string]profile.MCPServer{
			"github": {
				Command: "npx",
				Env: map[string]string{
					"GH_PAT":        "${secret:GITHUB_TOKEN}",
					"SECOND_GH_PAT": "${secret:GITHUB_TOKEN}",
				},
			},
		},
	}
	if err := profile.NewStore(p).Save(prof); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	sp := newCountingProvider()
	if err := sp.Set("acme", "GITHUB_TOKEN", "ghp_secret"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	plan, err := Build(p, sp, "acme", tools.Claude, nil)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	if sp.counts["GITHUB_TOKEN"] != 1 {
		t.Fatalf("GITHUB_TOKEN Get count = %d, want 1", sp.counts["GITHUB_TOKEN"])
	}
	if plan.Env["GH_PAT"] != "ghp_secret" {
		t.Fatalf("GH_PAT = %q, want MCP secret value", plan.Env["GH_PAT"])
	}
	if plan.Env["SECOND_GH_PAT"] != "ghp_secret" {
		t.Fatalf("SECOND_GH_PAT = %q, want MCP secret value", plan.Env["SECOND_GH_PAT"])
	}
}

func TestBuildErrorsWhenMCPSecretTargetConflicts(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := profile.Profile{
		Name:        "acme",
		ProjectDir:  t.TempDir(),
		SafetyLevel: profile.Standard,
		AuthMode:    profile.AuthSubscription,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Claude: {Enabled: true}},
		MCP: map[string]profile.MCPServer{
			"github": {
				Command: "npx",
				Env:     map[string]string{"TOKEN": "${secret:GITHUB_TOKEN}"},
			},
			"gitlab": {
				Command: "npx",
				Env:     map[string]string{"TOKEN": "${secret:GITLAB_TOKEN}"},
			},
		},
	}
	if err := profile.NewStore(p).Save(prof); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	sp := secrets.NewMemory()
	if err := sp.Set("acme", "GITHUB_TOKEN", "github_secret"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err := sp.Set("acme", "GITLAB_TOKEN", "gitlab_secret"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	_, err := Build(p, sp, "acme", tools.Claude, nil)
	if err == nil {
		t.Fatal("Build accepted conflicting MCP target secrets, want error")
	}
	msg := err.Error()
	for _, want := range []string{"TOKEN", "GITHUB_TOKEN", "GITLAB_TOKEN"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error = %q, want %s context", msg, want)
		}
	}
}

func TestBuildErrorsWhenMCPSecretTargetConflictsWithDeclaredSecret(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := profile.Profile{
		Name:        "acme",
		ProjectDir:  t.TempDir(),
		SafetyLevel: profile.Standard,
		AuthMode:    profile.AuthAPIKey,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Codex: {Enabled: true}},
		Secrets:     profile.SecretConfig{Keys: []string{"OPENAI_API_KEY"}},
		MCP: map[string]profile.MCPServer{
			"github": {
				Command: "npx",
				Env:     map[string]string{"OPENAI_API_KEY": "${secret:GITHUB_TOKEN}"},
			},
		},
	}
	if err := profile.NewStore(p).Save(prof); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	sp := secrets.NewMemory()
	if err := sp.Set("acme", "OPENAI_API_KEY", "sk-openai"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	if err := sp.Set("acme", "GITHUB_TOKEN", "ghp_secret"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	_, err := Build(p, sp, "acme", tools.Codex, nil)
	if err == nil {
		t.Fatal("Build accepted MCP target conflicting with declared secret, want error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "OPENAI_API_KEY") || !strings.Contains(msg, "GITHUB_TOKEN") {
		t.Fatalf("error = %q, want OPENAI_API_KEY and GITHUB_TOKEN context", msg)
	}
}

func TestBuildAllowsMCPSecretTargetMatchingDeclaredSecret(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := profile.Profile{
		Name:        "acme",
		ProjectDir:  t.TempDir(),
		SafetyLevel: profile.Standard,
		AuthMode:    profile.AuthAPIKey,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Codex: {Enabled: true}},
		Secrets:     profile.SecretConfig{Keys: []string{"OPENAI_API_KEY"}},
		MCP: map[string]profile.MCPServer{
			"openai": {
				Command: "npx",
				Env:     map[string]string{"OPENAI_API_KEY": "${secret:OPENAI_API_KEY}"},
			},
		},
	}
	if err := profile.NewStore(p).Save(prof); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	sp := secrets.NewMemory()
	if err := sp.Set("acme", "OPENAI_API_KEY", "sk-openai"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	plan, err := Build(p, sp, "acme", tools.Codex, nil)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if plan.Env["OPENAI_API_KEY"] != "sk-openai" {
		t.Fatalf("OPENAI_API_KEY = %q, want declared secret value", plan.Env["OPENAI_API_KEY"])
	}
}

func TestBuildErrorsWhenMCPSecretMissing(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := profile.Profile{
		Name:        "acme",
		ProjectDir:  t.TempDir(),
		SafetyLevel: profile.Standard,
		AuthMode:    profile.AuthSubscription,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Claude: {Enabled: true}},
		MCP: map[string]profile.MCPServer{
			"github": {
				Command: "npx",
				Env:     map[string]string{"GH_PAT": "${secret:GITHUB_TOKEN}"},
			},
		},
	}
	if err := profile.NewStore(p).Save(prof); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	_, err := Build(p, secrets.NewMemory(), "acme", tools.Claude, nil)
	if err == nil {
		t.Fatal("Build accepted missing MCP secret, want error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "GITHUB_TOKEN") || !strings.Contains(msg, "GH_PAT") {
		t.Fatalf("error = %q, want GITHUB_TOKEN and GH_PAT context", msg)
	}
}

func TestBuildIgnoresUnsupportedMCPSecretRefs(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := profile.Profile{
		Name:        "acme",
		ProjectDir:  t.TempDir(),
		SafetyLevel: profile.Standard,
		AuthMode:    profile.AuthSubscription,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Claude: {Enabled: true}},
		MCP: map[string]profile.MCPServer{
			"remote": {
				Type:    "http",
				Command: "remote",
				Env:     map[string]string{"TOKEN": "${secret:MISSING}"},
			},
		},
	}
	if err := profile.NewStore(p).Save(prof); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	plan, err := Build(p, secrets.NewMemory(), "acme", tools.Claude, nil)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if _, ok := plan.Env["TOKEN"]; ok {
		t.Fatalf("TOKEN was injected from unsupported MCP server: %#v", plan.Env["TOKEN"])
	}
	if _, ok := plan.MaskedEnv["TOKEN"]; ok {
		t.Fatalf("TOKEN was masked from unsupported MCP server: %#v", plan.MaskedEnv["TOKEN"])
	}
}

func TestBuildInjectsGeminiMCPSecretAndMasksIt(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := profile.Profile{
		Name:        "acme",
		ProjectDir:  t.TempDir(),
		SafetyLevel: profile.Standard,
		AuthMode:    profile.AuthSubscription,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Gemini: {Enabled: true}},
		MCP: map[string]profile.MCPServer{
			"stdio": {
				Type:    "stdio",
				Command: "npx",
				Env:     map[string]string{"GH_TOKEN": "${secret:GEMINI_PAT}"},
			},
		},
	}
	if err := profile.NewStore(p).Save(prof); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	sp := secrets.NewMemory()
	if err := sp.Set("acme", "GEMINI_PAT", "g-secret"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	plan, err := Build(p, sp, "acme", tools.Gemini, nil)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if plan.Env["GH_TOKEN"] != "g-secret" {
		t.Fatalf("GH_TOKEN = %q, want MCP secret value", plan.Env["GH_TOKEN"])
	}
	if plan.MaskedEnv["GH_TOKEN"] != "********" {
		t.Fatalf("masked GH_TOKEN = %q, want ********", plan.MaskedEnv["GH_TOKEN"])
	}
}

func TestBuildRequiresToolAPIKeyForAPIKeyProfile(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := profile.Profile{
		Name:        "api",
		ProjectDir:  t.TempDir(),
		SafetyLevel: profile.Standard,
		AuthMode:    profile.AuthAPIKey,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Codex: {Enabled: true}},
	}
	if err := profile.NewStore(p).Save(prof); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	_, err := Build(p, secrets.NewMemory(), "api", tools.Codex, nil)
	if err == nil {
		t.Fatal("Build accepted api-key profile without OPENAI_API_KEY")
	}
}

func TestBuildRefusesAPIKeyInSubscriptionProfile(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	prof := profile.Profile{
		Name:        "sub",
		ProjectDir:  t.TempDir(),
		SafetyLevel: profile.Standard,
		AuthMode:    profile.AuthSubscription,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Claude: {Enabled: true}},
		Secrets:     profile.SecretConfig{Keys: []string{"ANTHROPIC_API_KEY"}},
	}
	if err := profile.NewStore(p).Save(prof); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	sp := secrets.NewMemory()
	if err := sp.Set("sub", "ANTHROPIC_API_KEY", "sk-ant-xxx"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}

	if _, err := Build(p, sp, "sub", tools.Claude, nil); err == nil {
		t.Fatal("Build accepted ANTHROPIC_API_KEY in a subscription profile, want error")
	}
}

func TestBuildRejectsDisabledTool(t *testing.T) {
	p, sp := setup(t)

	if _, err := Build(p, sp, "acme", tools.Claude, nil); err == nil {
		t.Fatal("Build accepted a disabled tool, want error")
	}
}
