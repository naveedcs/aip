package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestRunDryRunPrintsPlan(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()

	binDir := t.TempDir()
	fakeCodex := filepath.Join(binDir, "codex")
	if err := os.WriteFile(fakeCodex, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile fake codex returned error: %v", err)
	}
	t.Setenv("PATH", binDir)

	mustRun(t, home, "profile", "create",
		"--name", "acme",
		"--project-dir", project,
		"--tools", "codex",
		"--yes",
	)

	out := mustRun(t, home, "run", "acme", "codex", "--dry-run")
	if !strings.Contains(out, "Command: codex") {
		t.Fatalf("dry-run output missing command:\n%s", out)
	}
	if !strings.Contains(out, "CODEX_HOME=") {
		t.Fatalf("dry-run output missing CODEX_HOME:\n%s", out)
	}
}

func TestRunDryRunShowsClaudeMCPConfigArg(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()

	mustRun(t, home, "profile", "create",
		"--name", "acme",
		"--project-dir", project,
		"--tools", "claude",
		"--yes",
	)
	mustRun(t, home, "mcp", "add", "acme", "github", "--command", "npx")

	out := mustRun(t, home, "run", "acme", "claude", "--dry-run")
	wantPath := filepath.Join(home, "profiles", "acme", "tools", "claude", "mcp.json")
	if !strings.Contains(out, "--mcp-config") || !strings.Contains(out, wantPath) {
		t.Fatalf("dry-run output missing Claude MCP config arg %q:\n%s", wantPath, out)
	}
}

func TestRunDryRunDoesNotRenderMCPConfig(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()

	mustRun(t, home, "profile", "create",
		"--name", "acme",
		"--project-dir", project,
		"--tools", "claude",
		"--yes",
	)
	mustRun(t, home, "mcp", "add", "acme", "github", "--command", "npx")

	mcpPath := filepath.Join(home, "profiles", "acme", "tools", "claude", "mcp.json")
	out := mustRun(t, home, "run", "acme", "claude", "--dry-run")
	if !strings.Contains(out, mcpPath) {
		t.Fatalf("dry-run output missing MCP path %q:\n%s", mcpPath, out)
	}
	if _, err := os.Stat(mcpPath); !os.IsNotExist(err) {
		t.Fatalf("dry-run rendered MCP config, stat error = %v", err)
	}
}

func TestLoginDryRunDoesNotRenderMCPConfig(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()

	mustRun(t, home, "profile", "create",
		"--name", "acme",
		"--project-dir", project,
		"--tools", "claude",
		"--yes",
	)
	mustRun(t, home, "mcp", "add", "acme", "github", "--command", "npx")

	mcpPath := filepath.Join(home, "profiles", "acme", "tools", "claude", "mcp.json")
	out := mustRun(t, home, "login", "acme", "claude", "--dry-run")
	if !strings.Contains(out, "--mcp-config") || !strings.Contains(out, mcpPath) || !strings.Contains(out, "login") {
		t.Fatalf("login dry-run output missing MCP config login plan for %q:\n%s", mcpPath, out)
	}
	if _, err := os.Stat(mcpPath); !os.IsNotExist(err) {
		t.Fatalf("login dry-run rendered MCP config, stat error = %v", err)
	}
}

func TestRunRendersMCPConfigBeforeLaunch(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()

	binDir := t.TempDir()
	fakeClaude := filepath.Join(binDir, "claude")
	mcpPath := filepath.Join(home, "profiles", "acme", "tools", "claude", "mcp.json")
	writeFakeClaudeMCPValidator(t, fakeClaude, mcpPath, `"NODE_ENV": "test"`)
	t.Setenv("PATH", binDir)

	mustRun(t, home, "profile", "create",
		"--name", "acme",
		"--project-dir", project,
		"--tools", "claude",
		"--yes",
	)
	mustRun(t, home, "mcp", "add", "acme", "github", "--command", "npx", "--env", "NODE_ENV=test")

	mustRun(t, home, "run", "acme", "claude")
}

func TestLoginRendersMCPConfigBeforeLaunch(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()

	binDir := t.TempDir()
	fakeClaude := filepath.Join(binDir, "claude")
	mcpPath := filepath.Join(home, "profiles", "acme", "tools", "claude", "mcp.json")
	writeFakeClaudeMCPValidator(t, fakeClaude, mcpPath)
	t.Setenv("PATH", binDir)

	mustRun(t, home, "profile", "create",
		"--name", "acme",
		"--project-dir", project,
		"--tools", "claude",
		"--yes",
	)
	mustRun(t, home, "mcp", "add", "acme", "github", "--command", "npx")

	mustRun(t, home, "login", "acme", "claude")
}

func TestShimExecActiveProfileRendersMCPConfigBeforeLaunch(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()

	shimDir := filepath.Join(home, "shims")
	if err := os.MkdirAll(shimDir, 0o700); err != nil {
		t.Fatalf("MkdirAll shim dir returned error: %v", err)
	}
	binDir := t.TempDir()
	fakeClaude := filepath.Join(binDir, "claude")
	mcpPath := filepath.Join(home, "profiles", "acme", "tools", "claude", "mcp.json")
	writeFakeClaudeMCPValidator(t, fakeClaude, mcpPath)
	t.Setenv("AIP_HOME", home)
	t.Setenv("AIP_PROFILE", "acme")
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+binDir)

	mustRun(t, home, "profile", "create",
		"--name", "acme",
		"--project-dir", project,
		"--tools", "claude",
		"--yes",
	)
	mustRun(t, home, "mcp", "add", "acme", "github", "--command", "npx")

	var out, errOut bytes.Buffer
	cmd := NewRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"shim-exec", "claude"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("shim-exec returned error: %v\nstderr: %s", err, errOut.String())
	}
}

func TestDryRunShowsIsolatedLaunchEnvironment(t *testing.T) {
	t.Setenv("PATH", "/usr/local/aip-test/bin")
	parentHome := filepath.Join(t.TempDir(), "ambient-home")
	t.Setenv("HOME", parentHome)
	t.Setenv("OPENAI_API_KEY", "parent-openai")

	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createProfileForTest(t, home, project)

	out := &bytes.Buffer{}
	cmd := NewRootCommand(out, out)
	cmd.SetArgs([]string{"--home", home, "dry-run", "mgcs", "codex"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run returned error: %v", err)
	}
	got := out.String()
	toolHome := filepath.Join(home, "profiles", "mgcs", "tools", "codex")
	for _, want := range []string{
		"PATH=/usr/local/aip-test/bin",
		"HOME=" + parentHome,
		"CODEX_HOME=" + toolHome,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("dry-run output missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "OPENAI_API_KEY") || strings.Contains(got, "parent-openai") {
		t.Fatalf("dry-run leaked parent credential: %q", got)
	}
}

func TestDryRunMasksSecretsFromKeychain(t *testing.T) {
	keyring.MockInit()
	home := t.TempDir()
	mustRun(t, home, "profile", "create", "--name", "acme",
		"--project-dir", t.TempDir(), "--tools", "codex", "--yes")

	var out, errOut bytes.Buffer
	cmd := NewRootCommand(&out, &errOut)
	cmd.SetIn(strings.NewReader("ghp_secret\n"))
	cmd.SetArgs([]string{"--home", home, "secret", "set", "acme", "GITHUB_TOKEN", "--stdin"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("secret set failed: %v\n%s", err, errOut.String())
	}

	got := mustRun(t, home, "dry-run", "acme", "codex")
	if !strings.Contains(got, "GITHUB_TOKEN=********") {
		t.Fatalf("dry-run output missing masked secret:\n%s", got)
	}
	if strings.Contains(got, "ghp_secret") {
		t.Fatalf("dry-run leaked secret value:\n%s", got)
	}
}

func TestLoginDryRunUsesToolLoginArgs(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createProfileForTest(t, home, project)

	out := &bytes.Buffer{}
	cmd := NewRootCommand(out, out)
	cmd.SetArgs([]string{"--home", home, "login", "mgcs", "codex", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("login dry-run returned error: %v", err)
	}
	if !strings.Contains(out.String(), "Command: codex login") {
		t.Fatalf("login output = %q", out.String())
	}
}

func TestLoginMissingArgsShowsFriendlyError(t *testing.T) {
	var out bytes.Buffer
	cmd := NewRootCommand(&out, &out)
	cmd.SetArgs([]string{"login"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected missing login args error")
	}
	got := err.Error()
	for _, want := range []string{"Missing profile and tool.", "Usage:", "aip login <profile> <tool>", "Examples:", "aip login smoke codex", "aip login smoke codex --dry-run"} {
		if !strings.Contains(got, want) {
			t.Fatalf("error missing %q in %q", want, got)
		}
	}
}

func TestLoginMissingProfileShowsCreateGuidance(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")

	var out bytes.Buffer
	cmd := NewRootCommand(&out, &out)
	cmd.SetArgs([]string{"--home", home, "login", "smoke", "codex"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected missing profile error")
	}
	got := err.Error()
	for _, want := range []string{
		`Profile "smoke" does not exist.`,
		"Create it:",
		"aip profile create --name smoke --project-dir . --safety read-only --tools codex --yes",
		"List profiles:",
		"aip profile list",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("error missing %q in %q", want, got)
		}
	}
}

func TestLoginAdminCancelsBeforeExecution(t *testing.T) {
	binDir := t.TempDir()
	fakeCodex := filepath.Join(binDir, "codex")
	if err := os.WriteFile(fakeCodex, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile fake codex returned error: %v", err)
	}
	t.Setenv("PATH", binDir)

	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createAdminProfileForTest(t, home, project)

	out := &bytes.Buffer{}
	cmd := NewRootCommand(out, out)
	cmd.SetIn(strings.NewReader("wrong\n"))
	cmd.SetArgs([]string{"--home", home, "login", "mgcs", "codex"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected admin cancellation error")
	}
	if !strings.Contains(err.Error(), "admin launch cancelled") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunAdminDryRunBypassesPrompt(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createAdminProfileForTest(t, home, project)

	out := &bytes.Buffer{}
	cmd := NewRootCommand(out, out)
	cmd.SetIn(strings.NewReader("wrong\n"))
	cmd.SetArgs([]string{"--home", home, "run", "mgcs", "codex", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("admin dry-run returned error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Command: codex") {
		t.Fatalf("admin dry-run output = %q", got)
	}
	if strings.Contains(got, "Warning: You are launching an ADMIN profile.") {
		t.Fatalf("admin dry-run should not prompt: %q", got)
	}
}

func TestRunAdminCancelsBeforeExecution(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createAdminProfileForTest(t, home, project)

	out := &bytes.Buffer{}
	cmd := NewRootCommand(out, out)
	cmd.SetIn(strings.NewReader("wrong\n"))
	cmd.SetArgs([]string{"--home", home, "run", "mgcs", "codex"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected admin cancellation error")
	}
	if !strings.Contains(err.Error(), "admin launch cancelled") {
		t.Fatalf("error = %v", err)
	}
}

func mustRun(t *testing.T, home string, args ...string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := NewRootCommand(&out, &errOut)
	cmd.SetArgs(append([]string{"--home", home}, args...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command %v failed: %v\nstderr: %s", args, err, errOut.String())
	}
	return out.String()
}

func writeFakeClaudeMCPValidator(t *testing.T, path string, wantMCPPath string, extraPatterns ...string) {
	t.Helper()

	patterns := append([]string{`"github"`, `"command": "npx"`}, extraPatterns...)
	var script strings.Builder
	script.WriteString(`#!/bin/sh
mcp_config=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--mcp-config" ]; then
    shift
    if [ "$#" -eq 0 ]; then
      echo "missing --mcp-config value" >&2
      exit 64
    fi
    mcp_config="$1"
  fi
  shift
done

if [ -z "$mcp_config" ]; then
  echo "missing --mcp-config argument" >&2
  exit 65
fi
if [ "$mcp_config" != ` + shellQuote(wantMCPPath) + ` ]; then
  printf 'unexpected --mcp-config path: %s\n' "$mcp_config" >&2
  exit 66
fi
if [ ! -f "$mcp_config" ]; then
  printf 'missing MCP config during launch: %s\n' "$mcp_config" >&2
  exit 67
fi
`)
	for i, pattern := range patterns {
		foundVar := fmt.Sprintf("found_%d", i)
		script.WriteString(foundVar + `=0
while IFS= read -r line; do
  case "$line" in
    *` + shellQuote(pattern) + `*) ` + foundVar + `=1 ;;
  esac
done < "$mcp_config"
if [ "$` + foundVar + `" -ne 1 ]; then
  printf 'MCP config missing pattern: %s\n' ` + shellQuote(pattern) + ` >&2
  exit 68
fi
`)
	}
	script.WriteString("exit 0\n")

	if err := os.WriteFile(path, []byte(script.String()), 0o755); err != nil {
		t.Fatalf("WriteFile fake claude returned error: %v", err)
	}
}

func createAdminProfileForTest(t *testing.T, home string, project string) {
	t.Helper()
	cmd := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	cmd.SetArgs([]string{
		"--home", home,
		"profile", "create",
		"--name", "mgcs",
		"--display-name", "MGCS",
		"--project-dir", project,
		"--safety", "admin",
		"--tools", "codex,claude",
		"--yes",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("admin profile create returned error: %v", err)
	}
}
