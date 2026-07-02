package doctor

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/naveedcs/aip/internal/paths"
	"github.com/naveedcs/aip/internal/profile"
	"github.com/naveedcs/aip/internal/secrets"
	"github.com/naveedcs/aip/internal/tools"
	"github.com/zalando/go-keyring"
)

func TestCheckProfileReportsSecretsWithoutValues(t *testing.T) {
	testCheckProfileReportsSecretsWithoutValues(t)
}

func TestCheckProfileReportsMCPServersAndWarnsOnMissingMCPSecret(t *testing.T) {
	keyring.MockInit()
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	if err := profile.NewStore(p).Save(profile.Profile{
		Name:        "mgcs",
		DisplayName: "MGCS",
		ProjectDir:  t.TempDir(),
		SafetyLevel: profile.ReadOnly,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Codex: {Enabled: true}},
		MCP: map[string]profile.MCPServer{
			"zeta": {
				Type:    "http",
				Command: "remote",
				Env: map[string]string{
					"T":       "${secret:MISSING}",
					"LITERAL": "plain",
				},
			},
			"alpha": {
				Command: "npx",
				Env: map[string]string{
					"PRESENT_TARGET": "${secret:PRESENT}",
				},
			},
		},
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if err := secrets.NewKeychain().Set("mgcs", "PRESENT", "present-secret-value"); err != nil {
		t.Fatalf("Set secret returned error: %v", err)
	}

	report, err := CheckProfile(p, "mgcs")
	if err != nil {
		t.Fatalf("CheckProfile returned error: %v", err)
	}
	if want := []string{"alpha", "zeta"}; !slices.Equal(report.MCPServers, want) {
		t.Fatalf("MCPServers = %#v, want %#v", report.MCPServers, want)
	}

	found := false
	for _, warning := range report.Warnings {
		if strings.Contains(warning, "MISSING") && strings.Contains(warning, "T") {
			found = true
		}
		if strings.Contains(warning, "present-secret-value") {
			t.Fatalf("warning leaked secret value: %q", warning)
		}
	}
	if !found {
		t.Fatalf("expected missing MCP secret warning, got %v", report.Warnings)
	}
}

func TestCheckProfileDedupeDeclaredAndMCPSecretWarnings(t *testing.T) {
	keyring.MockInit()
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	if err := profile.NewStore(p).Save(profile.Profile{
		Name:        "mgcs",
		DisplayName: "MGCS",
		ProjectDir:  t.TempDir(),
		SafetyLevel: profile.ReadOnly,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Codex: {Enabled: true}},
		Secrets:     profile.SecretConfig{Keys: []string{"MISSING"}},
		MCP: map[string]profile.MCPServer{
			"zeta": {
				Type:    "http",
				Command: "remote",
				Env: map[string]string{
					"B_TARGET": "${secret:MISSING}",
				},
			},
			"alpha": {
				Command: "npx",
				Env: map[string]string{
					"A_TARGET": "${secret:MISSING}",
				},
			},
		},
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	report, err := CheckProfile(p, "mgcs")
	if err != nil {
		t.Fatalf("CheckProfile returned error: %v", err)
	}

	var matching []string
	for _, warning := range report.Warnings {
		if strings.Contains(warning, "MISSING") {
			matching = append(matching, warning)
		}
	}
	if len(matching) != 1 {
		t.Fatalf("warnings containing MISSING = %#v, want exactly one in %#v", matching, report.Warnings)
	}
	warning := matching[0]
	if !strings.Contains(warning, "A_TARGET") || !strings.Contains(warning, "B_TARGET") {
		t.Fatalf("warning missing MCP target context: %q", warning)
	}
	if strings.Index(warning, "A_TARGET") > strings.Index(warning, "B_TARGET") {
		t.Fatalf("warning target order is not deterministic: %q", warning)
	}
}

func TestCheckProfileWarnsHonchoEnabledWithoutServer(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	if err := profile.NewStore(p).Save(profile.Profile{
		Name:        "mgcs",
		DisplayName: "MGCS",
		ProjectDir:  t.TempDir(),
		SafetyLevel: profile.ReadOnly,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Codex: {Enabled: true}},
		Honcho: profile.HonchoConfig{
			Enabled:     true,
			WorkspaceID: "acme-ws",
			UserName:    "naveed",
		},
		MCP: map[string]profile.MCPServer{},
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	report, err := CheckProfile(p, "mgcs")
	if err != nil {
		t.Fatalf("CheckProfile returned error: %v", err)
	}
	if !report.Honcho.Enabled {
		t.Fatalf("Honcho.Enabled = false, report = %#v", report)
	}
	found := false
	for _, warning := range report.Warnings {
		if strings.Contains(warning, "honcho") && strings.Contains(warning, "MCP server is missing") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected missing honcho MCP server warning, got %v", report.Warnings)
	}
}

func TestDoctorCheckProfileReportsSecretsWithoutValues(t *testing.T) {
	testCheckProfileReportsSecretsWithoutValues(t)
}

func TestCheckProfileExpandsProjectDirBeforeStat(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	projectDir := filepath.Join(home, "project")
	if err := os.MkdirAll(projectDir, 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	store := profile.NewStore(p)
	if err := store.Save(profile.Profile{
		Name:        "mgcs",
		DisplayName: "MGCS",
		ProjectDir:  "~/project",
		SafetyLevel: profile.ReadOnly,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Codex: {Enabled: true}},
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	report, err := CheckProfile(p, "mgcs")
	if err != nil {
		t.Fatalf("CheckProfile returned error: %v", err)
	}
	if !report.ProjectExists {
		t.Fatalf("ProjectExists = false, report = %#v", report)
	}
}

func TestDoctorCheckProfileExpandsProjectDirBeforeStat(t *testing.T) {
	TestCheckProfileExpandsProjectDirBeforeStat(t)
}

func TestCheckProfileWarnsOnProjectStatError(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	store := profile.NewStore(p)
	if err := store.Save(profile.Profile{
		Name:        "mgcs",
		DisplayName: "MGCS",
		ProjectDir:  filepath.Join(t.TempDir(), "project") + string(rune(0)),
		SafetyLevel: profile.ReadOnly,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Codex: {Enabled: true}},
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	report, err := CheckProfile(p, "mgcs")
	if err != nil {
		t.Fatalf("CheckProfile returned error: %v", err)
	}
	if report.ProjectExists {
		t.Fatalf("ProjectExists = true, report = %#v", report)
	}
	found := false
	for _, warning := range report.Warnings {
		if strings.HasPrefix(warning, "project status check failed: ") && strings.Contains(warning, "invalid argument") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("warnings = %#v", report.Warnings)
	}
}

func testCheckProfileReportsSecretsWithoutValues(t *testing.T) {
	keyring.MockInit()
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	store := profile.NewStore(p)
	if err := store.Save(profile.Profile{
		Name:        "mgcs",
		DisplayName: "MGCS",
		ProjectDir:  t.TempDir(),
		SafetyLevel: profile.ReadOnly,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Codex: {Enabled: true}},
		Secrets:     profile.SecretConfig{Keys: []string{"GITHUB_TOKEN"}},
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if err := secrets.NewKeychain().Set("mgcs", "GITHUB_TOKEN", "secret"); err != nil {
		t.Fatalf("Set secret returned error: %v", err)
	}

	report, err := CheckProfile(p, "mgcs")
	if err != nil {
		t.Fatalf("CheckProfile returned error: %v", err)
	}
	if report.Profile.Name != "mgcs" || len(report.SecretNames) != 1 || report.SecretNames[0] != "GITHUB_TOKEN" {
		t.Fatalf("report = %#v", report)
	}
	if strings.Contains(strings.Join(report.SecretNames, "\n"), "secret") {
		t.Fatalf("report leaked secret value: %#v", report.SecretNames)
	}
}

func TestCheckProfileIgnoresMalformedEnvLocal(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	store := profile.NewStore(p)
	if err := store.Save(profile.Profile{
		Name:        "mgcs",
		DisplayName: "MGCS",
		ProjectDir:  t.TempDir(),
		SafetyLevel: profile.ReadOnly,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Codex: {Enabled: true}},
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(p.ProfileDir("mgcs"), "env.local"), []byte("BROKEN"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if _, err := CheckProfile(p, "mgcs"); err != nil {
		t.Fatalf("CheckProfile returned error: %v", err)
	}
}

func TestCheckProfileFlagsSubscriptionApiKeyConflict(t *testing.T) {
	keyring.MockInit()
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	if err := profile.NewStore(p).Save(profile.Profile{
		Name:        "sub",
		ProjectDir:  t.TempDir(),
		SafetyLevel: profile.Standard,
		AuthMode:    profile.AuthSubscription,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Claude: {Enabled: true}},
		Secrets:     profile.SecretConfig{Keys: []string{"ANTHROPIC_API_KEY"}},
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if err := secrets.NewKeychain().Set("sub", "ANTHROPIC_API_KEY", "sk-ant-secret"); err != nil {
		t.Fatalf("Set secret returned error: %v", err)
	}

	report, err := CheckProfile(p, "sub")
	if err != nil {
		t.Fatalf("CheckProfile returned error: %v", err)
	}
	if report.AuthMode != profile.AuthSubscription {
		t.Fatalf("AuthMode = %q, want %q", report.AuthMode, profile.AuthSubscription)
	}
	found := false
	for _, w := range report.Warnings {
		if strings.Contains(w, "ANTHROPIC_API_KEY") && strings.Contains(w, "OAuth") {
			found = true
		}
		if strings.Contains(w, "sk-ant-secret") {
			t.Fatalf("warning leaked secret value: %q", w)
		}
	}
	if !found {
		t.Fatalf("expected a subscription/ANTHROPIC_API_KEY warning, got %v", report.Warnings)
	}
}

func TestCheckProfileWarnsForDeclaredSecretMissingFromKeychain(t *testing.T) {
	keyring.MockInit()
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	if err := profile.NewStore(p).Save(profile.Profile{
		Name:        "mgcs",
		ProjectDir:  t.TempDir(),
		SafetyLevel: profile.ReadOnly,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Codex: {Enabled: true}},
		Secrets:     profile.SecretConfig{Keys: []string{"MISSING_TOKEN"}},
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	report, err := CheckProfile(p, "mgcs")
	if err != nil {
		t.Fatalf("CheckProfile returned error: %v", err)
	}
	found := false
	for _, w := range report.Warnings {
		if strings.Contains(w, "secret MISSING_TOKEN is declared but missing from keychain") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected missing keychain warning, got %v", report.Warnings)
	}
}

func TestCheckProfileReportsToolConfigDir(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	if err := profile.NewStore(p).Save(profile.Profile{
		Name:        "mgcs",
		ProjectDir:  t.TempDir(),
		SafetyLevel: profile.ReadOnly,
		Tools:       map[tools.ID]profile.ToolConfig{tools.Codex: {Enabled: true}},
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	report, err := CheckProfile(p, "mgcs")
	if err != nil {
		t.Fatalf("CheckProfile returned error: %v", err)
	}
	wantEnabled := p.ToolConfigDir("mgcs", string(tools.Codex))
	wantDisabled := p.ToolConfigDir("mgcs", string(tools.Gemini))
	foundEnabled := false
	foundDisabled := false
	for _, status := range report.Tools {
		if status.Tool.ID == tools.Codex {
			foundEnabled = true
			if status.ConfigDir != wantEnabled {
				t.Fatalf("codex ConfigDir = %q, want %q", status.ConfigDir, wantEnabled)
			}
		}
		if status.Tool.ID == tools.Gemini {
			foundDisabled = true
			if status.ConfigDir != wantDisabled {
				t.Fatalf("gemini ConfigDir = %q, want %q", status.ConfigDir, wantDisabled)
			}
		}
	}
	if !foundEnabled || !foundDisabled {
		t.Fatalf("tool statuses missing codex=%v gemini=%v in %#v", foundEnabled, foundDisabled, report.Tools)
	}
}
