package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootVersion(t *testing.T) {
	var out bytes.Buffer
	cmd := NewRootCommand(&out, &out)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "aip version 0.1.0") {
		t.Fatalf("version output = %q", got)
	}
}

func TestRootHelpMentionsCoreCommands(t *testing.T) {
	var out bytes.Buffer
	cmd := NewRootCommand(&out, &out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	got := out.String()
	for _, want := range []string{"init", "profile", "run", "login", "doctor", "dry-run", "tools", "secret", "mcp", "use", "deactivate"} {
		if !strings.Contains(got, want) {
			t.Fatalf("help output missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "setup") {
		t.Fatalf("help output should not mention setup after wizard removal: %q", got)
	}
	for _, want := range []string{"Examples:", "aip init", "aip doctor smoke"} {
		if !strings.Contains(got, want) {
			t.Fatalf("help output missing example %q in %q", want, got)
		}
	}
}

func TestBareRootShowsHelp(t *testing.T) {
	var out bytes.Buffer
	cmd := NewRootCommand(&out, &out)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	got := out.String()
	for _, want := range []string{"Usage:", "Available Commands:", "aip [command]"} {
		if !strings.Contains(got, want) {
			t.Fatalf("bare root help missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "Start here") {
		t.Fatalf("bare root should not render wizard home screen: %q", got)
	}
}

func TestRootUnknownFlagForCommandExplainsCommandNotFlag(t *testing.T) {
	var out bytes.Buffer
	cmd := NewRootCommand(&out, &out)
	cmd.SetArgs([]string{"--doctor"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected unknown flag error")
	}
	got := err.Error()
	for _, want := range []string{"Unknown flag: --doctor", "`doctor` is a command, not a flag", "aip doctor <profile>", "aip doctor --help"} {
		if !strings.Contains(got, want) {
			t.Fatalf("error missing %q in %q", want, got)
		}
	}
}

func TestRootTypoFlagSuggestsClosestCommand(t *testing.T) {
	var out bytes.Buffer
	cmd := NewRootCommand(&out, &out)
	cmd.SetArgs([]string{"--dcotor"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected unknown flag error")
	}
	got := err.Error()
	for _, want := range []string{"Unknown flag: --dcotor", "Did you mean command `doctor`?", "aip doctor <profile>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("error missing %q in %q", want, got)
		}
	}
}

func TestRootShorthandRejectsUnknownTool(t *testing.T) {
	var out bytes.Buffer
	cmd := NewRootCommand(&out, &out)
	cmd.SetArgs([]string{"mgcs", "codexx"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected shorthand typo to return an error")
	}
	if !strings.Contains(err.Error(), "unsupported tool") {
		t.Fatalf("error = %v", err)
	}
}

func TestRootShorthandAdminCancelsBeforeExecution(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createAdminProfileForTest(t, home, project)

	var out bytes.Buffer
	cmd := NewRootCommand(&out, &out)
	cmd.SetIn(strings.NewReader("wrong\n"))
	cmd.SetArgs([]string{"--home", home, "mgcs", "codex"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected admin cancellation error")
	}
	if !strings.Contains(err.Error(), "admin launch cancelled") {
		t.Fatalf("error = %v", err)
	}
}
