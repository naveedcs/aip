package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/naveedcs/aip/internal/paths"
	"github.com/naveedcs/aip/internal/profile"
	"github.com/naveedcs/aip/internal/secrets"
	"github.com/naveedcs/aip/internal/templates"
	"github.com/naveedcs/aip/internal/tools"
	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"
)

func createProfileForTest(t *testing.T, home string, project string) {
	t.Helper()
	cmd := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	cmd.SetArgs([]string{
		"--home", home,
		"profile", "create",
		"--name", "mgcs",
		"--display-name", "MGCS",
		"--project-dir", project,
		"--safety", "read-only",
		"--tools", "codex,claude",
		"--yes",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("profile create returned error: %v", err)
	}
}

func templateMenuChoiceForTest(t *testing.T, home string, name string) int {
	t.Helper()
	infos, err := templates.List(paths.ForRoot(home))
	if err != nil {
		t.Fatalf("list templates: %v", err)
	}
	for i, info := range infos {
		if info.Name == name {
			return i + 2
		}
	}
	t.Fatalf("template %q not found in %v", name, infos)
	return 0
}

func TestProfileCreateListShow(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()

	createOut := &bytes.Buffer{}
	create := NewRootCommand(createOut, createOut)
	create.SetArgs([]string{
		"--home", home,
		"profile", "create",
		"--name", "mgcs",
		"--display-name", "MGCS",
		"--project-dir", project,
		"--safety", "read-only",
		"--tools", "codex,claude",
		"--yes",
	})
	if err := create.Execute(); err != nil {
		t.Fatalf("profile create returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "profiles", "mgcs", "profile.yaml")); err != nil {
		t.Fatalf("profile.yaml missing: %v", err)
	}

	listOut := &bytes.Buffer{}
	list := NewRootCommand(listOut, listOut)
	list.SetArgs([]string{"--home", home, "profile", "list"})
	if err := list.Execute(); err != nil {
		t.Fatalf("profile list returned error: %v", err)
	}
	if !strings.Contains(listOut.String(), "mgcs") {
		t.Fatalf("list output = %q", listOut.String())
	}

	showOut := &bytes.Buffer{}
	show := NewRootCommand(showOut, showOut)
	show.SetArgs([]string{"--home", home, "profile", "show", "mgcs"})
	if err := show.Execute(); err != nil {
		t.Fatalf("profile show returned error: %v", err)
	}
	if !strings.Contains(showOut.String(), "Safety: read-only") {
		t.Fatalf("show output = %q", showOut.String())
	}
}

func TestProfileCreateWithAuthModeAndRemove(t *testing.T) {
	keyring.MockInit()
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()

	createOut := mustRun(t, home, "profile", "create",
		"--name", "acme",
		"--project-dir", project,
		"--tools", "codex",
		"--auth-mode", "api-key",
		"--yes",
	)
	if !strings.Contains(createOut, "Created profile acme") {
		t.Fatalf("create output = %q", createOut)
	}

	showOut := mustRun(t, home, "profile", "show", "acme")
	if !strings.Contains(showOut, "Auth: api-key") {
		t.Fatalf("show output = %q", showOut)
	}

	secretStore := secrets.NewKeychain()
	if err := secretStore.Set("acme", "OPENAI_API_KEY", "sk-test"); err != nil {
		t.Fatalf("keychain set failed: %v", err)
	}
	store := profile.NewStore(paths.ForRoot(home))
	prof, err := store.Load("acme")
	if err != nil {
		t.Fatalf("profile load failed: %v", err)
	}
	prof.Secrets.Provider = "keychain"
	prof.Secrets.Keys = []string{"MISSING_TOKEN", "OPENAI_API_KEY"}
	if err := store.Save(prof); err != nil {
		t.Fatalf("profile save failed: %v", err)
	}

	rmOut := mustRun(t, home, "profile", "rm", "acme")
	if !strings.Contains(rmOut, "Removed profile acme") {
		t.Fatalf("rm output = %q", rmOut)
	}
	if _, err := os.Stat(filepath.Join(home, "profiles", "acme")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("profile dir still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := secretStore.Get("acme", "OPENAI_API_KEY"); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("keychain secret still exists or get failed unexpectedly: %v", err)
	}

	out := &bytes.Buffer{}
	show := NewRootCommand(out, out)
	show.SetArgs([]string{"--home", home, "profile", "show", "acme"})
	err = show.Execute()
	if err == nil {
		t.Fatal("expected show to fail after profile rm")
	}
	if !strings.Contains(err.Error(), `Profile "acme" does not exist.`) {
		t.Fatalf("show error = %v", err)
	}
}

func TestProfileTemplatesLists(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")

	out := mustRun(t, home, "profile", "templates")
	if !strings.Contains(out, "software-readonly") {
		t.Fatalf("templates output = %q", out)
	}
}

func TestProfileCreateFromTemplate(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()

	mustRun(t, home, "profile", "create",
		"--name", "acme",
		"--project-dir", project,
		"--template", "software-readonly",
		"--yes",
	)

	showOut := mustRun(t, home, "profile", "show", "acme")
	if !strings.Contains(showOut, "Safety: read-only") {
		t.Fatalf("show output = %q", showOut)
	}

	data, err := os.ReadFile(filepath.Join(home, "profiles", "acme", "profile.yaml"))
	if err != nil {
		t.Fatalf("read profile.yaml: %v", err)
	}
	if !strings.Contains(string(data), "github") {
		t.Fatalf("profile.yaml = %q", string(data))
	}
}

func TestProfileCreateWizard(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := NewRootCommand(stdout, stderr)
	cmd.SetIn(strings.NewReader(strings.Join([]string{
		"acme",
		"1",
		"",
		project,
		"2",
		"",
		"",
	}, "\n")))
	cmd.SetArgs([]string{"--home", home, "profile", "create"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("profile create wizard returned error: %v", err)
	}

	if got := stdout.String(); !strings.Contains(got, "Created profile acme") {
		t.Fatalf("stdout = %q", got)
	}
	if strings.Contains(stdout.String(), "Profile name") {
		t.Fatalf("wizard prompt was written to stdout: %q", stdout.String())
	}
	if got := stderr.String(); !strings.Contains(got, "Profile name") || !strings.Contains(got, "Which AI tools?") {
		t.Fatalf("stderr = %q", got)
	}

	data, err := os.ReadFile(filepath.Join(home, "profiles", "acme", "profile.yaml"))
	if err != nil {
		t.Fatalf("read profile.yaml: %v", err)
	}
	if !strings.Contains(string(data), "safety_level: standard") {
		t.Fatalf("profile.yaml = %q", string(data))
	}
	if !strings.Contains(string(data), "codex:") {
		t.Fatalf("profile.yaml = %q", string(data))
	}
	if !strings.Contains(string(data), "claude:") {
		t.Fatalf("profile.yaml = %q", string(data))
	}
	var created profile.Profile
	if err := yaml.Unmarshal(data, &created); err != nil {
		t.Fatalf("unmarshal profile.yaml: %v", err)
	}
	if !created.Tools[tools.Codex].Enabled || !created.Tools[tools.Claude].Enabled {
		t.Fatalf("tools = %#v", created.Tools)
	}
}

func TestProfileCreateWizardUsesFlagInputsDirectly(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := NewRootCommand(stdout, stderr)
	cmd.SetIn(strings.NewReader("acme\n"))
	cmd.SetArgs([]string{
		"--home", home,
		"profile", "create",
		"--display-name", "Flag Display",
		"--project-dir", project,
		"--template", "software-readonly",
		"--safety", "admin",
		"--tools", "codex",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("profile create wizard returned error: %v", err)
	}
	if got := stderr.String(); strings.Contains(got, "Start from a template?") ||
		strings.Contains(got, "Display name") ||
		strings.Contains(got, "Project directory") ||
		strings.Contains(got, "Safety level") ||
		strings.Contains(got, "Which AI tools?") {
		t.Fatalf("wizard reprompted for flag-provided values:\n%s", got)
	}

	data, err := os.ReadFile(filepath.Join(home, "profiles", "acme", "profile.yaml"))
	if err != nil {
		t.Fatalf("read profile.yaml: %v", err)
	}
	profileYAML := string(data)
	if !strings.Contains(profileYAML, "display_name: Flag Display") {
		t.Fatalf("profile.yaml = %q", profileYAML)
	}
	if !strings.Contains(profileYAML, "project_dir: "+project) {
		t.Fatalf("profile.yaml = %q", profileYAML)
	}
	if !strings.Contains(profileYAML, "github:") {
		t.Fatalf("profile.yaml = %q", profileYAML)
	}
	if !strings.Contains(profileYAML, "safety_level: admin") {
		t.Fatalf("profile.yaml = %q", profileYAML)
	}
	if !strings.Contains(profileYAML, "codex:") {
		t.Fatalf("profile.yaml = %q", profileYAML)
	}
	var created profile.Profile
	if err := yaml.Unmarshal(data, &created); err != nil {
		t.Fatalf("unmarshal profile.yaml: %v", err)
	}
	if !created.Tools[tools.Codex].Enabled || created.Tools[tools.Claude].Enabled {
		t.Fatalf("tools = %#v", created.Tools)
	}
}

func TestProfileCreateWizardUsesTemplateDefaults(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	templateChoice := strconv.Itoa(templateMenuChoiceForTest(t, home, "software-readonly"))

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := NewRootCommand(stdout, stderr)
	cmd.SetIn(strings.NewReader(strings.Join([]string{
		"acme",
		templateChoice,
		"",
		project,
		"",
		"",
		"",
	}, "\n")))
	cmd.SetArgs([]string{"--home", home, "profile", "create"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("profile create wizard returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(home, "profiles", "acme", "profile.yaml"))
	if err != nil {
		t.Fatalf("read profile.yaml: %v", err)
	}
	profileYAML := string(data)
	if !strings.Contains(profileYAML, "safety_level: read-only") {
		t.Fatalf("profile.yaml = %q", profileYAML)
	}
	if !strings.Contains(profileYAML, "github:") {
		t.Fatalf("profile.yaml = %q", profileYAML)
	}
	if !strings.Contains(profileYAML, "codex:") || !strings.Contains(profileYAML, "claude:") {
		t.Fatalf("profile.yaml = %q", profileYAML)
	}
	var created profile.Profile
	if err := yaml.Unmarshal(data, &created); err != nil {
		t.Fatalf("unmarshal profile.yaml: %v", err)
	}
	if !created.Tools[tools.Codex].Enabled || !created.Tools[tools.Claude].Enabled {
		t.Fatalf("tools = %#v", created.Tools)
	}
}

func TestProfileCreateTemplateSafetyOverride(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()

	mustRun(t, home, "profile", "create",
		"--name", "acme",
		"--project-dir", project,
		"--template", "software-readonly",
		"--safety", "standard",
		"--yes",
	)

	showOut := mustRun(t, home, "profile", "show", "acme")
	if !strings.Contains(showOut, "Safety: standard") {
		t.Fatalf("show output = %q", showOut)
	}
}

func TestProfileClone(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createProfileForTest(t, home, project)

	out := mustRun(t, home, "profile", "clone", "mgcs", "mgcs-admin")
	if !strings.Contains(out, "Cloned mgcs to mgcs-admin") {
		t.Fatalf("clone output = %q", out)
	}
	if !strings.Contains(out, "re-add secrets") {
		t.Fatalf("clone output missing secrets reminder: %q", out)
	}
	if _, err := os.Stat(filepath.Join(home, "profiles", "mgcs-admin", "profile.yaml")); err != nil {
		t.Fatalf("cloned profile.yaml missing: %v", err)
	}

	showOut := mustRun(t, home, "profile", "show", "mgcs-admin")
	if !strings.Contains(showOut, "Name: mgcs-admin") {
		t.Fatalf("show output = %q", showOut)
	}
}

func TestProfileCloneRefusesExisting(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createProfileForTest(t, home, project)
	mustRun(t, home, "profile", "create",
		"--name", "mgcs-admin",
		"--project-dir", project,
		"--safety", "standard",
		"--tools", "codex",
		"--yes",
	)

	out := &bytes.Buffer{}
	cmd := NewRootCommand(out, out)
	cmd.SetArgs([]string{"--home", home, "profile", "clone", "mgcs", "mgcs-admin"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected clone to refuse existing destination")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("clone error = %v", err)
	}
}

func TestProfileCloneRenamesDefaultDisplayName(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	mustRun(t, home, "profile", "create",
		"--name", "acme",
		"--project-dir", project,
		"--tools", "codex",
		"--yes",
	)

	mustRun(t, home, "profile", "clone", "acme", "acme-admin")

	showOut := mustRun(t, home, "profile", "show", "acme-admin")
	if !strings.Contains(showOut, "Display: acme-admin") {
		t.Fatalf("show output = %q", showOut)
	}
}

func TestProfileExportImportRoundTrip(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createProfileForTest(t, home, project)
	mustRun(t, home, "mcp", "add", "mgcs", "github", "--command", "npx", "--env", "GH_PAT=${secret:GITHUB_TOKEN}")

	exportPath := filepath.Join(t.TempDir(), "profile.yaml")
	mustRun(t, home, "profile", "export", "mgcs", "--out", exportPath)

	exported, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	if strings.Contains(string(exported), home) {
		t.Fatalf("export leaked AIP home path: %q", string(exported))
	}

	importProject := t.TempDir()
	out := mustRun(t, home, "profile", "import", exportPath, "--name", "mgcs2", "--project-dir", importProject)
	if !strings.Contains(out, "re-add secrets") {
		t.Fatalf("import output missing secrets reminder: %q", out)
	}

	imported, err := os.ReadFile(filepath.Join(home, "profiles", "mgcs2", "profile.yaml"))
	if err != nil {
		t.Fatalf("read imported profile.yaml: %v", err)
	}
	if !strings.Contains(string(imported), "github") {
		t.Fatalf("imported profile.yaml = %q", string(imported))
	}
}

func TestProfileExportRejectsLiteralMCPEnvValue(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createProfileForTest(t, home, project)

	store := profile.NewStore(paths.ForRoot(home))
	prof, err := store.Load("mgcs")
	if err != nil {
		t.Fatalf("profile load failed: %v", err)
	}
	prof.MCP = map[string]profile.MCPServer{
		"github": {
			Command: "npx",
			Env: map[string]string{
				"GH_PAT": "plain-secret",
			},
		},
	}
	if err := store.Save(prof); err != nil {
		t.Fatalf("profile save failed: %v", err)
	}

	exportPath := filepath.Join(t.TempDir(), "profile.yaml")
	out := &bytes.Buffer{}
	cmd := NewRootCommand(out, out)
	cmd.SetArgs([]string{"--home", home, "profile", "export", "mgcs", "--out", exportPath})
	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected export to reject literal MCP env value")
	}
	if !strings.Contains(err.Error(), "github") || !strings.Contains(err.Error(), "GH_PAT") {
		t.Fatalf("export error = %v", err)
	}
	if data, readErr := os.ReadFile(exportPath); readErr == nil && len(data) != 0 {
		t.Fatalf("export file should be absent or empty, got %q", string(data))
	} else if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		t.Fatalf("read export file: %v", readErr)
	}
}

func TestProfileImportRefusesExisting(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createProfileForTest(t, home, project)

	exportPath := filepath.Join(t.TempDir(), "profile.yaml")
	mustRun(t, home, "profile", "export", "mgcs", "--out", exportPath)

	out := &bytes.Buffer{}
	cmd := NewRootCommand(out, out)
	cmd.SetArgs([]string{"--home", home, "profile", "import", exportPath})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected import to refuse existing profile")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("import error = %v", err)
	}
}

func TestProfileImportRejectsLiteralMCPEnvValue(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	importPath := filepath.Join(t.TempDir(), "profile.yaml")
	data := []byte(`name: acme
project_dir: /tmp/acme
safety_level: read-only
tools:
  codex:
    enabled: true
mcp:
  github:
    command: npx
    env:
      GH_PAT: plain-secret
`)
	if err := os.WriteFile(importPath, data, 0o600); err != nil {
		t.Fatalf("write import file: %v", err)
	}

	out := &bytes.Buffer{}
	cmd := NewRootCommand(out, out)
	cmd.SetArgs([]string{"--home", home, "profile", "import", importPath, "--project-dir", t.TempDir()})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected import to reject literal MCP env value")
	}
	if !strings.Contains(err.Error(), "github") || !strings.Contains(err.Error(), "GH_PAT") {
		t.Fatalf("import error = %v", err)
	}
}

func TestProfileImportStripsDerivedFields(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	importPath := filepath.Join(t.TempDir(), "profile.yaml")
	fakePath := "/external/aip/profiles/acme/tools/codex"
	data := []byte(`name: acme
project_dir: /tmp/acme
safety_level: read-only
instructions: /external/aip/profiles/acme/instructions.md
tools:
  codex:
    enabled: true
    home: /external/aip/profiles/acme/tools/codex
    config: /external/aip/profiles/acme/tools/codex/config.toml
`)
	if err := os.WriteFile(importPath, data, 0o600); err != nil {
		t.Fatalf("write import file: %v", err)
	}

	mustRun(t, home, "profile", "import", importPath, "--project-dir", project)

	importedData, err := os.ReadFile(filepath.Join(home, "profiles", "acme", "profile.yaml"))
	if err != nil {
		t.Fatalf("read imported profile.yaml: %v", err)
	}
	if strings.Contains(string(importedData), fakePath) {
		t.Fatalf("imported profile.yaml preserved fake path: %q", string(importedData))
	}

	var imported profile.Profile
	if err := yaml.Unmarshal(importedData, &imported); err != nil {
		t.Fatalf("unmarshal imported profile: %v", err)
	}
	wantHome := filepath.Join(home, "profiles", "acme", "tools", "codex")
	if imported.Tools[tools.Codex].Home != wantHome {
		t.Fatalf("codex home = %q, want %q", imported.Tools[tools.Codex].Home, wantHome)
	}
	if strings.Contains(imported.Instructions, "/external/aip") {
		t.Fatalf("instructions path preserved fake path: %q", imported.Instructions)
	}
}

func TestProfileCreateValidationErrors(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing name",
			args: []string{"--home", home, "profile", "create", "--project-dir", project, "--tools", "codex", "--yes"},
			want: "missing required --name",
		},
		{
			name: "missing project dir",
			args: []string{"--home", home, "profile", "create", "--name", "mgcs", "--tools", "codex", "--yes"},
			want: "missing required --project-dir",
		},
		{
			name: "stray arg",
			args: []string{"--home", home, "profile", "create", "--name", "mgcs", "--project-dir", project, "--tools", "codex", "--yes", "extra"},
			want: "unknown command \"extra\"",
		},
		{
			name: "empty tools",
			args: []string{"--home", home, "profile", "create", "--name", "mgcs", "--project-dir", project, "--tools", "", "--yes"},
			want: "tools list must not be empty",
		},
		{
			name: "comma only tools",
			args: []string{"--home", home, "profile", "create", "--name", "mgcs", "--project-dir", project, "--tools", ",,,", "--yes"},
			want: "tools list contains an empty entry",
		},
		{
			name: "empty token in tools",
			args: []string{"--home", home, "profile", "create", "--name", "mgcs", "--project-dir", project, "--tools", "codex,,claude", "--yes"},
			want: "tools list contains an empty entry",
		},
		{
			name: "unsupported tool",
			args: []string{"--home", home, "profile", "create", "--name", "mgcs", "--project-dir", project, "--tools", "codex,unknown", "--yes"},
			want: "unsupported tool \"unknown\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			cmd := NewRootCommand(out, out)
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want contains %q", err, tt.want)
			}
		})
	}
}

func TestProfileListValidationErrors(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")

	out := &bytes.Buffer{}
	cmd := NewRootCommand(out, out)
	cmd.SetArgs([]string{"--home", home, "profile", "list", "extra"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error for stray args")
	}
	if !strings.Contains(err.Error(), "unknown command \"extra\"") {
		t.Fatalf("error = %v", err)
	}
}
