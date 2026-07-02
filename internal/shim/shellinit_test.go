package shim

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/naveedcs/aip/internal/paths"
)

func TestShellInitZshContainsCoreParts(t *testing.T) {
	p := paths.ForRoot(t.TempDir())

	got, err := ShellInit("zsh", p)
	if err != nil {
		t.Fatalf("ShellInit returned error: %v", err)
	}

	for _, want := range []string{
		p.ShimsDir,
		"aip()",
		"AIP_SHELL_INIT=1",
		"AIP_PROFILE",
		"PROMPT",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("ShellInit zsh output missing %q:\n%s", want, got)
		}
	}
}

func TestShellInitBashUsesPS1(t *testing.T) {
	p := paths.ForRoot(t.TempDir())

	got, err := ShellInit("bash", p)
	if err != nil {
		t.Fatalf("ShellInit returned error: %v", err)
	}

	if !strings.Contains(got, "PS1") {
		t.Fatalf("ShellInit bash output missing PS1:\n%s", got)
	}
}

func TestShellInitRejectsUnsupportedShell(t *testing.T) {
	p := paths.ForRoot(t.TempDir())

	if _, err := ShellInit("fish", p); err == nil {
		t.Fatal("ShellInit fish returned nil error")
	}
}

func TestShellInitFunctionPropagatesUseFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell")
	}
	for _, shell := range []string{"bash", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			if _, err := exec.LookPath(shell); err != nil {
				t.Skipf("%s not available: %v", shell, err)
			}
			dir := t.TempDir()
			aipPath := filepath.Join(dir, "aip")
			if err := os.WriteFile(aipPath, []byte("#!/bin/sh\nexit 7\n"), 0o755); err != nil {
				t.Fatalf("write fake aip: %v", err)
			}
			snippet, err := ShellInit(shell, paths.ForRoot(filepath.Join(dir, "root with ' quote")))
			if err != nil {
				t.Fatalf("ShellInit returned error: %v", err)
			}
			snippetPath := filepath.Join(dir, "aip-init.sh")
			if err := os.WriteFile(snippetPath, []byte(snippet), 0o600); err != nil {
				t.Fatalf("write snippet: %v", err)
			}

			cmd := exec.Command(shell, "-c", `. "`+snippetPath+`"; aip use ghost; code=$?; [ "$code" -eq 7 ]`)
			cmd.Env = append(os.Environ(), "PATH="+dir)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s did not propagate use failure: %v\n%s", shell, err, out)
			}
		})
	}
}

func TestShellInitIsIdempotentInBash(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skipf("bash not available: %v", err)
	}
	root := filepath.Join(t.TempDir(), "root with spaces")
	snippet, err := ShellInit("bash", paths.ForRoot(root))
	if err != nil {
		t.Fatalf("ShellInit returned error: %v", err)
	}
	dir := t.TempDir()
	snippetPath := filepath.Join(dir, "aip-init.sh")
	if err := os.WriteFile(snippetPath, []byte(snippet), 0o600); err != nil {
		t.Fatalf("write snippet: %v", err)
	}

	cmd := exec.Command("bash", "-c", `. "`+snippetPath+`"; . "`+snippetPath+`"; case "$PATH" in *"`+root+`/shims:`+root+`/shims"*) exit 1;; esac; case "$PS1" in *'$(_aip_prompt_segment)*$(_aip_prompt_segment)'*) exit 1;; esac`)
	cmd.Env = append(os.Environ(), "PATH=/usr/bin:/bin", "PS1=$ ")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shell-init was not idempotent in bash: %v\n%s", err, out)
	}
}
