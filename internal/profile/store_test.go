package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/naveedcs/aip/internal/paths"
	"github.com/naveedcs/aip/internal/tools"
)

func TestSaveLoadProfile(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	store := NewStore(p)
	input := Profile{
		Name:        "mgcs-readonly",
		DisplayName: "MGCS Readonly",
		ProjectDir:  "~/projects/mgcs",
		SafetyLevel: ReadOnly,
		Tools: map[tools.ID]ToolConfig{
			tools.Codex:  {Enabled: true},
			tools.Claude: {Enabled: true},
		},
		Secrets: SecretConfig{Keys: []string{"GITHUB_TOKEN"}},
		MCP: map[string]MCPServer{
			"github": {
				Command: "npx",
				Args:    []string{"-y", "github-mcp-server"},
				Env:     map[string]string{"GITHUB_TOKEN": "${secret:GITHUB_TOKEN}"},
			},
			"filesystem": {
				Command:  "mcp-server-filesystem",
				Args:     []string{"~/projects/mgcs"},
				ReadOnly: true,
			},
		},
	}

	if err := store.Save(input); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got, err := store.Load("mgcs-readonly")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got.Name != input.Name || got.SafetyLevel != ReadOnly {
		t.Fatalf("loaded profile = %#v", got)
	}
	if len(got.MCP) != 2 {
		t.Fatalf("loaded MCP server count = %d, want 2: %#v", len(got.MCP), got.MCP)
	}
	github := got.MCP["github"]
	if github.Command != "npx" || len(github.Args) != 2 || github.Args[0] != "-y" || github.Args[1] != "github-mcp-server" || github.Env["GITHUB_TOKEN"] != "${secret:GITHUB_TOKEN}" {
		t.Fatalf("loaded github MCP server = %#v", github)
	}
	filesystem := got.MCP["filesystem"]
	if filesystem.Command != "mcp-server-filesystem" || len(filesystem.Args) != 1 || filesystem.Args[0] != "~/projects/mgcs" || !filesystem.ReadOnly {
		t.Fatalf("loaded filesystem MCP server = %#v", filesystem)
	}
	wantCodexDir := p.ToolConfigDir("mgcs-readonly", string(tools.Codex))
	if got.Tools[tools.Codex].Home != wantCodexDir {
		t.Fatalf("codex home = %q, want %q", got.Tools[tools.Codex].Home, wantCodexDir)
	}
	if got.Tools[tools.Codex].Config != filepath.Join(wantCodexDir, "config.toml") {
		t.Fatalf("codex config = %q, want inside %q", got.Tools[tools.Codex].Config, wantCodexDir)
	}
	if _, err := os.Stat(p.InstructionsFile("mgcs-readonly")); err != nil {
		t.Fatalf("instructions file missing: %v", err)
	}
}

func TestLoadNormalizesLegacyToolPaths(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	profileDir := p.ProfileDir("mgcs")
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	legacyHome := filepath.Join(p.RootDir, "homes", "mgcs", "codex")
	legacyConfig := filepath.Join(p.RootDir, "legacy", "codex.toml")
	data := []byte("name: mgcs\nproject_dir: .\nsafety_level: standard\ntools:\n  codex:\n    enabled: true\n    home: " + legacyHome + "\n    config: " + legacyConfig + "\n  claude:\n    enabled: false\n    home: /stale/home\n    config: /stale/settings.json\n")
	if err := os.WriteFile(p.ProfileFile("mgcs"), data, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	got, err := NewStore(p).Load("mgcs")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	wantCodexDir := p.ToolConfigDir("mgcs", string(tools.Codex))
	if got.Tools[tools.Codex].Home != wantCodexDir {
		t.Fatalf("codex home = %q, want %q", got.Tools[tools.Codex].Home, wantCodexDir)
	}
	if got.Tools[tools.Codex].Config != filepath.Join(wantCodexDir, "config.toml") {
		t.Fatalf("codex config = %q, want config.toml under %q", got.Tools[tools.Codex].Config, wantCodexDir)
	}
	if got.Tools[tools.Claude].Home != "" || got.Tools[tools.Claude].Config != "" {
		t.Fatalf("disabled claude tool config not cleared: %#v", got.Tools[tools.Claude])
	}
}

func TestLoadIgnoresLegacyMCPServerList(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	profileDir := p.ProfileDir("mgcs")
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	data := []byte("name: mgcs\nproject_dir: .\nsafety_level: standard\ntools: {}\nmcp:\n  servers:\n    - github\n    - filesystem\n")
	if err := os.WriteFile(p.ProfileFile("mgcs"), data, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	got, err := NewStore(p).Load("mgcs")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(got.MCP) != 0 {
		t.Fatalf("MCP = %#v, want no servers from legacy list", got.MCP)
	}
}

func TestLoadKeepsMCPServerNamedServers(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	profileDir := p.ProfileDir("mgcs")
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	data := []byte("name: mgcs\nproject_dir: .\nsafety_level: standard\ntools: {}\nmcp:\n  servers:\n    command: npx\n    args:\n      - -y\n      - srv\n")
	if err := os.WriteFile(p.ProfileFile("mgcs"), data, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	got, err := NewStore(p).Load("mgcs")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	server := got.MCP["servers"]
	if server.Command != "npx" || len(server.Args) != 2 || server.Args[1] != "srv" {
		t.Fatalf("MCP[servers] = %#v", server)
	}
}

func TestLoadMigratesLegacyMCPSidecarServers(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	profileDir := p.ProfileDir("mgcs")
	if err := os.MkdirAll(filepath.Join(profileDir, "mcp"), 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	data := []byte("name: mgcs\nproject_dir: .\nsafety_level: standard\ntools: {}\n")
	if err := os.WriteFile(p.ProfileFile("mgcs"), data, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	sidecar := []byte("servers:\n  github:\n    type: stdio\n    command: npx\n    args:\n      - -y\n      - srv\n    env:\n      GH_PAT: ${GITHUB_TOKEN}\n    read_only: true\n")
	if err := os.WriteFile(filepath.Join(profileDir, "mcp", "servers.yaml"), sidecar, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	got, err := NewStore(p).Load("mgcs")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	server := got.MCP["github"]
	if server.Type != "stdio" || server.Command != "npx" || len(server.Args) != 2 || server.Args[0] != "-y" || server.Args[1] != "srv" {
		t.Fatalf("MCP[github] command fields = %#v", server)
	}
	if server.Env["GH_PAT"] != "${GITHUB_TOKEN}" || !server.ReadOnly {
		t.Fatalf("MCP[github] env/read_only = %#v", server)
	}
}

func TestLoadInlineMCPWinsOverLegacySidecar(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	profileDir := p.ProfileDir("mgcs")
	if err := os.MkdirAll(filepath.Join(profileDir, "mcp"), 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	data := []byte("name: mgcs\nproject_dir: .\nsafety_level: standard\ntools: {}\nmcp:\n  github:\n    command: inline\n")
	if err := os.WriteFile(p.ProfileFile("mgcs"), data, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	sidecar := []byte("servers:\n  github:\n    command: legacy\n")
	if err := os.WriteFile(filepath.Join(profileDir, "mcp", "servers.yaml"), sidecar, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	got, err := NewStore(p).Load("mgcs")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if got.MCP["github"].Command != "inline" {
		t.Fatalf("MCP[github].Command = %q, want inline", got.MCP["github"].Command)
	}
}

func TestSaveDefaultsEmptyAuthModeToSubscription(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	store := NewStore(p)

	if err := store.Save(Profile{Name: "mgcs", ProjectDir: ".", SafetyLevel: Standard}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got, err := store.Load("mgcs")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got.AuthMode != AuthSubscription {
		t.Fatalf("AuthMode = %q, want %q", got.AuthMode, AuthSubscription)
	}
}

func TestSaveDefaultsSecretProviderToKeychainAndDoesNotSeedEnvFiles(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	store := NewStore(p)

	if err := store.Save(Profile{Name: "mgcs", ProjectDir: ".", SafetyLevel: Standard}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got, err := store.Load("mgcs")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got.Secrets.Provider != "keychain" {
		t.Fatalf("Secrets.Provider = %q, want keychain", got.Secrets.Provider)
	}
	for _, path := range []string{
		filepath.Join(p.ProfileDir("mgcs"), "env.local"),
		filepath.Join(p.ProfileDir("mgcs"), "env.example"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s exists or stat failed unexpectedly: %v", path, err)
		}
	}
}

func TestSaveClearsDisabledToolConfigAndUsesProfileNameForInstructions(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	store := NewStore(p)
	input := Profile{
		Name:        "mgcs",
		ProjectDir:  ".",
		SafetyLevel: Standard,
		Tools: map[tools.ID]ToolConfig{
			tools.Codex: {
				Enabled: false,
				Home:    "/stale/home",
				Config:  "/stale/config.toml",
			},
		},
	}

	if err := store.Save(input); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got, err := store.Load("mgcs")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got.Tools[tools.Codex].Home != "" || got.Tools[tools.Codex].Config != "" {
		t.Fatalf("disabled codex tool config not cleared: %#v", got.Tools[tools.Codex])
	}

	data, err := os.ReadFile(p.InstructionsFile("mgcs"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(data) != "# mgcs\n\nProfile-specific AI instructions live here.\n" {
		t.Fatalf("instructions content = %q", string(data))
	}
}

func TestValidateRejectsBadName(t *testing.T) {
	err := Validate(Profile{Name: "../bad", ProjectDir: ".", SafetyLevel: ReadOnly})
	if err == nil {
		t.Fatal("Validate should reject path traversal names")
	}
}

func TestLoadRejectsBadName(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	store := NewStore(p)

	if _, err := store.Load("../bad"); err == nil {
		t.Fatal("Load should reject path traversal names")
	}
}

func TestLoadRejectsProfileNameMismatch(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	if err := os.MkdirAll(p.ProfileDir("acme"), 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(p.ProfileFile("acme"), []byte("name: other\nproject_dir: .\nsafety_level: standard\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if _, err := NewStore(p).Load("acme"); err == nil {
		t.Fatal("Load accepted profile.yaml with mismatched name")
	}
}

func TestListProfiles(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	store := NewStore(p)
	for _, name := range []string{"mgcs", "phd"} {
		if err := store.Save(Profile{Name: name, ProjectDir: ".", SafetyLevel: Standard}); err != nil {
			t.Fatalf("Save(%q) returned error: %v", name, err)
		}
	}
	got, err := store.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(got) != 2 || got[0].Name != "mgcs" || got[1].Name != "phd" {
		t.Fatalf("List = %#v", got)
	}
}

func TestListErrorsOnMalformedProfile(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	store := NewStore(p)

	profileDir := p.ProfileDir("broken")
	if err := os.MkdirAll(profileDir, 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(p.ProfileFile("broken"), []byte("name: broken\nproject_dir: .\nsafety_level: invalid\n"), 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if _, err := store.List(); err == nil {
		t.Fatal("List should return an error for malformed profile.yaml")
	}
}

func TestSaveErrorsWhenInstructionsPathIsDirectory(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	store := NewStore(p)

	if err := os.MkdirAll(p.InstructionsFile("mgcs"), 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	err := store.Save(Profile{Name: "mgcs", ProjectDir: ".", SafetyLevel: Standard})
	if err == nil {
		t.Fatal("Save should return an error when instructions.md is a directory")
	}
}

func TestSaveUsesProfileNameForEmptyDisplayNameInstructions(t *testing.T) {
	p := paths.ForRoot(filepath.Join(t.TempDir(), "aip"))
	store := NewStore(p)

	if err := store.Save(Profile{Name: "mgcs", ProjectDir: ".", SafetyLevel: Standard}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	data, err := os.ReadFile(p.InstructionsFile("mgcs"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if len(data) == 0 || string(data[:len("# mgcs")]) != "# mgcs" {
		t.Fatalf("instructions content = %q", string(data))
	}
}
