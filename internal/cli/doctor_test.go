package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestDoctorCommandPrintsReport(t *testing.T) {
	keyring.MockInit()
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createProfileForDoctorTest(t, home, project)
	addDoctorSecretForTest(t, home, "GITHUB_TOKEN", "ghp_secret")
	addDoctorMCPServerForTest(t, home, "github", "GITHUB_TOKEN", "ghp_secret")
	enableDoctorHonchoForTest(t, home)
	binDir := t.TempDir()
	codexPath := filepath.Join(binDir, "codex")
	if err := os.WriteFile(codexPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	t.Setenv("PATH", binDir)

	out := &bytes.Buffer{}
	cmd := NewRootCommand(out, out)
	cmd.SetArgs([]string{"--home", home, "doctor", "mgcs"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor returned error: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"AIP Doctor",
		"Profile: mgcs",
		"Safety: read-only",
		"Auth: subscription",
		"Project: " + project + " (exists)",
		"Tools:",
		"codex enabled installed=true path=" + codexPath,
		"config_dir=" + filepath.Join(home, "profiles", "mgcs", "tools", "codex"),
		"claude enabled installed=false",
		"Secrets:",
		"  GITHUB_TOKEN",
		"MCP servers:",
		"  github",
		"Honcho memory:",
		"  workspace: mgcs-ws",
		"Warnings:",
		"claude is enabled but not installed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("doctor output missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "ghp_secret") {
		t.Fatalf("doctor output leaked secret value: %q", got)
	}
}

func TestDoctorMissingProfileShowsFriendlyError(t *testing.T) {
	var out bytes.Buffer
	cmd := NewRootCommand(&out, &out)
	cmd.SetArgs([]string{"doctor"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected missing profile error")
	}
	got := err.Error()
	for _, want := range []string{"Missing profile name.", "Usage:", "aip doctor <profile>", "Example:", "aip doctor smoke"} {
		if !strings.Contains(got, want) {
			t.Fatalf("error missing %q in %q", want, got)
		}
	}
}

func addDoctorSecretForTest(t *testing.T, home string, name string, value string) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := NewRootCommand(&out, &errOut)
	cmd.SetIn(strings.NewReader(value + "\n"))
	cmd.SetArgs([]string{"--home", home, "secret", "set", "mgcs", name, "--stdin"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("secret set returned error: %v\n%s", err, errOut.String())
	}
}

func addDoctorMCPServerForTest(t *testing.T, home string, name string, secretName string, secretValue string) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := NewRootCommand(&out, &errOut)
	cmd.SetArgs([]string{
		"--home", home,
		"mcp", "add", "mgcs", name,
		"--command", "npx",
		"--env", "TOKEN=${secret:" + secretName + "}",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("mcp add returned error: %v\n%s", err, errOut.String())
	}
	if strings.Contains(out.String()+errOut.String(), secretValue) {
		t.Fatalf("mcp add output leaked secret value: stdout=%q stderr=%q", out.String(), errOut.String())
	}
}

func enableDoctorHonchoForTest(t *testing.T, home string) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := NewRootCommand(&out, &errOut)
	cmd.SetArgs([]string{
		"--home", home,
		"honcho", "enable", "mgcs",
		"--workspace-id", "mgcs-ws",
		"--user-name", "naveed",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("honcho enable returned error: %v\n%s", err, errOut.String())
	}
}

func createProfileForDoctorTest(t *testing.T, home string, project string) {
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
