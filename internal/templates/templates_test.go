package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/naveedcs/aip/internal/paths"
	"github.com/naveedcs/aip/internal/profile"
	"github.com/naveedcs/aip/internal/tools"
)

func TestGetBuiltinSoftwareReadonly(t *testing.T) {
	tmpl, err := Get(paths.Paths{}, "software-readonly")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if tmpl.SafetyLevel != profile.ReadOnly {
		t.Fatalf("SafetyLevel = %q, want %q", tmpl.SafetyLevel, profile.ReadOnly)
	}
	if _, ok := tmpl.MCP["github"]; !ok {
		t.Fatal("MCP missing github server")
	}
}

func TestGetUnknownTemplate(t *testing.T) {
	_, err := Get(paths.Paths{}, "missing")
	if err == nil {
		t.Fatal("Get returned nil error for unknown template")
	}
	if !strings.Contains(err.Error(), "unknown template") {
		t.Fatalf("Get error = %q, want unknown template", err)
	}
}

func TestGetRejectsInvalidTemplateNamesBeforeReadingFiles(t *testing.T) {
	root := t.TempDir()
	p := paths.ForRoot(root)
	if err := os.MkdirAll(filepath.Join(p.TemplatesDir, "nested"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	for path, description := range map[string]string{
		filepath.Join(root, "outside.yaml"):                  "Outside escape",
		filepath.Join(p.TemplatesDir, "nested", "name.yaml"): "Nested escape",
		filepath.Join(p.TemplatesDir, ".yaml"):               "Empty name escape",
		filepath.Join(root, "absolute.yaml"):                 "Absolute escape",
	} {
		writeTemplateFile(t, path, description)
	}

	cases := []string{
		"../outside",
		"nested/name",
		"",
		filepath.Join(root, "absolute"),
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			tmpl, err := Get(p, name)
			if err == nil {
				t.Fatalf("Get(%q) returned nil error and template %#v", name, tmpl)
			}
			if !strings.Contains(err.Error(), "invalid template name") {
				t.Fatalf("Get(%q) error = %q, want invalid template name", name, err)
			}
		})
	}
}

func TestListIncludesBuiltins(t *testing.T) {
	infos, err := List(paths.Paths{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	for _, info := range infos {
		if info.Name == "software-readonly" {
			if info.Source != "built-in" {
				t.Fatalf("Source = %q, want built-in", info.Source)
			}
			return
		}
	}
	t.Fatal("software-readonly not listed")
}

func TestListOutputIsSortedByName(t *testing.T) {
	infos, err := List(paths.Paths{})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	for i := 1; i < len(infos); i++ {
		if infos[i-1].Name > infos[i].Name {
			t.Fatalf("List returned unsorted names at %d: %q before %q", i, infos[i-1].Name, infos[i].Name)
		}
	}
}

func TestListTeamTemplateOverridesBuiltinInfo(t *testing.T) {
	p := paths.ForRoot(t.TempDir())
	if err := os.MkdirAll(p.TemplatesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	content := []byte(`description: Team readonly override
safety_level: admin
auth_mode: subscription
tools:
  - codex
`)
	if err := os.WriteFile(filepath.Join(p.TemplatesDir, "software-readonly.yaml"), content, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	infos, err := List(p)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	info, ok := findInfo(infos, "software-readonly")
	if !ok {
		t.Fatal("software-readonly not listed")
	}
	if info.Source != "team" {
		t.Fatalf("Source = %q, want team", info.Source)
	}
	if info.Description != "Team readonly override" {
		t.Fatalf("Description = %q, want team override description", info.Description)
	}
}

func TestListIncludesDistinctTeamTemplate(t *testing.T) {
	p := paths.ForRoot(t.TempDir())
	if err := os.MkdirAll(p.TemplatesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	content := []byte(`description: Custom team workflow
safety_level: standard
auth_mode: subscription
tools:
  - codex
`)
	if err := os.WriteFile(filepath.Join(p.TemplatesDir, "custom-team.yaml"), content, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	infos, err := List(p)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if _, ok := findInfo(infos, "software-readonly"); !ok {
		t.Fatal("software-readonly built-in not listed")
	}
	info, ok := findInfo(infos, "custom-team")
	if !ok {
		t.Fatal("custom-team team template not listed")
	}
	if info.Source != "team" {
		t.Fatalf("Source = %q, want team", info.Source)
	}
	if info.Description != "Custom team workflow" {
		t.Fatalf("Description = %q, want custom team description", info.Description)
	}
}

func TestListIgnoresMissingTemplatesDir(t *testing.T) {
	p := paths.ForRoot(t.TempDir())

	if _, err := List(p); err != nil {
		t.Fatalf("List returned error for missing templates dir: %v", err)
	}
}

func TestListIgnoresNonYAMLTeamTemplateFiles(t *testing.T) {
	p := paths.ForRoot(t.TempDir())
	if err := os.MkdirAll(p.TemplatesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(p.TemplatesDir, "notes.txt"), []byte("description: Ignore me\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	infos, err := List(p)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if _, ok := findInfo(infos, "notes"); ok {
		t.Fatal("List included non-.yaml team template file")
	}
}

func TestTeamTemplateOverridesBuiltin(t *testing.T) {
	p := paths.ForRoot(t.TempDir())
	if err := os.MkdirAll(p.TemplatesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	content := []byte(`description: Team override
safety_level: admin
auth_mode: subscription
tools:
  - codex
`)
	if err := os.WriteFile(filepath.Join(p.TemplatesDir, "software-readonly.yaml"), content, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	tmpl, err := Get(p, "software-readonly")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if tmpl.SafetyLevel != profile.Admin {
		t.Fatalf("SafetyLevel = %q, want %q", tmpl.SafetyLevel, profile.Admin)
	}
	if len(tmpl.Tools) != 1 || tmpl.Tools[0] != tools.Codex {
		t.Fatalf("Tools = %#v, want codex only", tmpl.Tools)
	}
}

func TestToProfileEnablesTools(t *testing.T) {
	tmpl := Template{
		Description: "Example",
		SafetyLevel: profile.Standard,
		AuthMode:    profile.AuthSubscription,
		Tools:       []tools.ID{tools.Codex},
		SecretKeys:  []string{"GITHUB_TOKEN"},
	}

	p := tmpl.ToProfile("acme", "", "/tmp/acme")

	if !p.Tools[tools.Codex].Enabled {
		t.Fatal("codex tool is not enabled")
	}
	if p.DisplayName != "acme" {
		t.Fatalf("DisplayName = %q, want acme", p.DisplayName)
	}
	if len(p.Secrets.Keys) != 1 || p.Secrets.Keys[0] != "GITHUB_TOKEN" {
		t.Fatalf("Secret keys = %#v, want GITHUB_TOKEN", p.Secrets.Keys)
	}
}

func TestToProfilePropagatesTemplateAndInputFields(t *testing.T) {
	tmpl := Template{
		Description: "Profile description",
		SafetyLevel: profile.Admin,
		AuthMode:    profile.AuthAPIKey,
	}

	p := tmpl.ToProfile("acme", "Acme Team", "/projects/acme")

	if p.Name != "acme" {
		t.Fatalf("Name = %q, want acme", p.Name)
	}
	if p.Description != "Profile description" {
		t.Fatalf("Description = %q, want template description", p.Description)
	}
	if p.ProjectDir != "/projects/acme" {
		t.Fatalf("ProjectDir = %q, want /projects/acme", p.ProjectDir)
	}
	if p.SafetyLevel != profile.Admin {
		t.Fatalf("SafetyLevel = %q, want %q", p.SafetyLevel, profile.Admin)
	}
	if p.AuthMode != profile.AuthAPIKey {
		t.Fatalf("AuthMode = %q, want %q", p.AuthMode, profile.AuthAPIKey)
	}
}

func TestToProfileDoesNotAliasMCPArgsOrEnv(t *testing.T) {
	tmpl := Template{
		MCP: map[string]profile.MCPServer{
			"github": {
				Command: "npx",
				Args:    []string{"one"},
				Env:     map[string]string{"TOKEN": "${secret:GITHUB_TOKEN}"},
			},
		},
	}

	p := tmpl.ToProfile("acme", "", "/tmp/acme")
	server := p.MCP["github"]
	server.Args[0] = "mutated"
	server.Env["TOKEN"] = "mutated"

	if tmpl.MCP["github"].Args[0] != "one" {
		t.Fatalf("template MCP Args aliased profile Args: got %q", tmpl.MCP["github"].Args[0])
	}
	if tmpl.MCP["github"].Env["TOKEN"] != "${secret:GITHUB_TOKEN}" {
		t.Fatalf("template MCP Env aliased profile Env: got %q", tmpl.MCP["github"].Env["TOKEN"])
	}
}

func TestToProfileDoesNotAliasMCPMap(t *testing.T) {
	tmpl := Template{
		MCP: map[string]profile.MCPServer{
			"github": {Command: "npx"},
		},
	}

	p := tmpl.ToProfile("acme", "", "/tmp/acme")
	p.MCP["new"] = profile.MCPServer{Command: "x"}

	if _, ok := tmpl.MCP["new"]; ok {
		t.Fatal("template MCP map aliased profile MCP map")
	}
}

func TestToProfileDoesNotAliasSecretKeys(t *testing.T) {
	tmpl := Template{
		SecretKeys: []string{"GITHUB_TOKEN"},
	}

	p := tmpl.ToProfile("acme", "", "/tmp/acme")
	p.Secrets.Keys[0] = "MUTATED"

	if tmpl.SecretKeys[0] != "GITHUB_TOKEN" {
		t.Fatalf("template SecretKeys aliased profile secret keys: got %q", tmpl.SecretKeys[0])
	}
}

func findInfo(infos []Info, name string) (Info, bool) {
	for _, info := range infos {
		if info.Name == name {
			return info, true
		}
	}
	return Info{}, false
}

func writeTemplateFile(t *testing.T, path, description string) {
	t.Helper()
	content := []byte("description: " + description + "\n" +
		"safety_level: standard\n" +
		"auth_mode: subscription\n" +
		"tools:\n" +
		"  - codex\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) returned error: %v", path, err)
	}
}
