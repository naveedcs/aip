package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	projectconfig "github.com/naveedcs/aip/internal/project"
)

func TestProjectInitAndShow(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	repo := t.TempDir()

	mustRun(t, home, "profile", "create",
		"--name", "acme",
		"--project-dir", repo,
		"--tools", "codex",
		"--yes",
	)
	mustRun(t, home, "mcp", "add", "acme", "zeta", "--command", "npx")
	mustRun(t, home, "mcp", "add", "acme", "alpha", "--command", "npx")

	initOut := mustRun(t, home, "project", "init", repo, "--profile", "acme", "--type", "ecommerce")
	if !strings.Contains(initOut, "Wrote "+projectconfig.ConfigPath(repo)) {
		t.Fatalf("init output = %q", initOut)
	}

	if _, err := os.Stat(projectconfig.ConfigPath(repo)); err != nil {
		t.Fatalf("project profile missing: %v", err)
	}

	gitignore, err := os.ReadFile(filepath.Join(repo, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(gitignore), ".aip/env.local") {
		t.Fatalf(".gitignore missing .aip/env.local: %q", string(gitignore))
	}

	showOut := mustRun(t, home, "project", "show", repo)
	if !strings.Contains(showOut, "Recommended profile: acme") {
		t.Fatalf("show output missing profile: %q", showOut)
	}
	if !strings.Contains(showOut, "Recommended MCP: alpha, zeta") {
		t.Fatalf("show output missing sorted MCP names: %q", showOut)
	}
}
