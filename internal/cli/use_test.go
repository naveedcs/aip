package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestUsePrintsShellExportAndStatus(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	createProfileForTest(t, home, t.TempDir())
	t.Setenv("AIP_SHELL_INIT", "1")

	var out, errOut bytes.Buffer
	cmd := NewRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"--home", home, "use", "mgcs"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("use returned error: %v\nstderr: %s", err, errOut.String())
	}

	if got, want := out.String(), "export AIP_PROFILE='mgcs'\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if got, want := errOut.String(), "[aip] using profile 'mgcs'\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestUsePrintsSetupHintWhenShellInitMissing(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")
	createProfileForTest(t, home, t.TempDir())
	t.Setenv("AIP_SHELL_INIT", "")

	var out, errOut bytes.Buffer
	cmd := NewRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"--home", home, "use", "mgcs"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("use returned error: %v\nstderr: %s", err, errOut.String())
	}

	if got, want := out.String(), "export AIP_PROFILE='mgcs'\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if got := errOut.String(); !strings.Contains(got, "[aip] using profile 'mgcs'") || !strings.Contains(got, "aip shell-init") {
		t.Fatalf("stderr missing status or setup hint: %q", got)
	}
}

func TestUseMissingProfileReturnsFriendlyErrorWithoutStdout(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip")

	var out, errOut bytes.Buffer
	cmd := NewRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"--home", home, "use", "missing"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected missing profile error")
	}
	if out.String() != "" {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
	for _, want := range []string{`Profile "missing" does not exist.`, "aip profile create --name missing", "aip profile list"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q in %q", want, err.Error())
		}
	}
}

func TestUseValidationErrorsDoNotEmitStdout(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing profile", args: []string{"use"}, want: "Missing profile name."},
		{name: "too many args", args: []string{"use", "one", "two"}, want: "Too many arguments."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := filepath.Join(t.TempDir(), "aip")

			var out, errOut bytes.Buffer
			cmd := NewRootCommand(&out, &errOut)
			cmd.SetArgs(append([]string{"--home", home}, tt.args...))

			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected validation error")
			}
			if out.String() != "" {
				t.Fatalf("stdout = %q, want empty", out.String())
			}
			for _, want := range []string{tt.want, "Usage:"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error missing %q in %q", want, err.Error())
				}
			}
		})
	}
}

func TestDeactivatePrintsUnsetAndStatus(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := NewRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"deactivate"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("deactivate returned error: %v\nstderr: %s", err, errOut.String())
	}

	if got, want := out.String(), "unset AIP_PROFILE\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if got, want := errOut.String(), "[aip] deactivated\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestDeactivateValidationErrorsDoNotEmitStdout(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := NewRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"deactivate", "extra"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if out.String() != "" {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
	for _, want := range []string{"Too many arguments.", "Usage:"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q in %q", want, err.Error())
		}
	}
}
