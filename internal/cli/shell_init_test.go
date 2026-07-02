package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellInitZshEmitsIntegrationAndShims(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")

	out := mustRun(t, home, "shell-init", "zsh")
	for _, want := range []string{"aip()", "AIP_PROFILE", "AIP_SHELL_INIT=1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("shell-init output missing %q in:\n%s", want, out)
		}
	}

	if _, err := os.Stat(filepath.Join(home, "shims", "claude")); err != nil {
		t.Fatalf("expected claude shim to exist: %v", err)
	}
}

func TestShellInitUnsupportedShellErrors(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")

	var out bytes.Buffer
	cmd := NewRootCommand(&out, &out)
	cmd.SetArgs([]string{"--home", home, "shell-init", "fish"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected unsupported shell error")
	}
	if !strings.Contains(err.Error(), `unsupported shell "fish"`) {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(home, "shims", "claude")); !os.IsNotExist(statErr) {
		t.Fatalf("expected unsupported shell to avoid generating shims, stat err = %v", statErr)
	}
}
