package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/naveedcs/aip/internal/secrets"
	"github.com/zalando/go-keyring"
)

func TestSecretSetReadsStdinAndListsMasked(t *testing.T) {
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

	listed := mustRun(t, home, "secret", "list", "acme")
	if !strings.Contains(listed, "GITHUB_TOKEN") || !strings.Contains(listed, "********") {
		t.Fatalf("list output unexpected:\n%s", listed)
	}
	if strings.Contains(listed, "ghp_secret") {
		t.Fatalf("list leaked the secret value:\n%s", listed)
	}
}

func TestSecretSetPromptsOnStderrAndStoresInteractiveValue(t *testing.T) {
	keyring.MockInit()
	home := t.TempDir()
	mustRun(t, home, "profile", "create", "--name", "acme",
		"--project-dir", t.TempDir(), "--tools", "codex", "--yes")

	var out, errOut bytes.Buffer
	cmd := NewRootCommand(&out, &errOut)
	cmd.SetIn(strings.NewReader(" interactive-secret \n"))
	cmd.SetArgs([]string{"--home", home, "secret", "set", "acme", "OPENAI_API_KEY"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("secret set failed: %v\n%s", err, errOut.String())
	}

	if !strings.Contains(out.String(), "Saved secret OPENAI_API_KEY") {
		t.Fatalf("stdout = %q", out.String())
	}
	if strings.Contains(out.String(), "Secret value") {
		t.Fatalf("secret prompt was written to stdout: %q", out.String())
	}
	if !strings.Contains(errOut.String(), "Secret value: ") {
		t.Fatalf("stderr = %q", errOut.String())
	}

	got, err := secrets.NewKeychain().Get("acme", "OPENAI_API_KEY")
	if err != nil {
		t.Fatalf("keychain get failed: %v", err)
	}
	if got != " interactive-secret " {
		t.Fatalf("stored secret = %q, want %q", got, " interactive-secret ")
	}
}

func TestSecretRmRemovesDeclaredKeyAndToleratesMissing(t *testing.T) {
	keyring.MockInit()
	home := t.TempDir()
	mustRun(t, home, "profile", "create", "--name", "acme",
		"--project-dir", t.TempDir(), "--tools", "codex", "--yes")

	var out, errOut bytes.Buffer
	setCmd := NewRootCommand(&out, &errOut)
	setCmd.SetIn(strings.NewReader("ghp_secret\n"))
	setCmd.SetArgs([]string{"--home", home, "secret", "set", "acme", "GITHUB_TOKEN", "--stdin"})
	if err := setCmd.Execute(); err != nil {
		t.Fatalf("secret set failed: %v\n%s", err, errOut.String())
	}

	mustRun(t, home, "secret", "rm", "acme", "GITHUB_TOKEN")
	listed := mustRun(t, home, "secret", "list", "acme")
	if strings.Contains(listed, "GITHUB_TOKEN") || strings.Contains(listed, "ghp_secret") {
		t.Fatalf("list output after rm unexpected:\n%s", listed)
	}

	mustRun(t, home, "secret", "rm", "acme", "GITHUB_TOKEN")
}

func TestSecretSetAndListMissingProfileErrors(t *testing.T) {
	keyring.MockInit()
	home := t.TempDir()

	tests := []struct {
		name string
		args []string
		in   string
		want string
	}{
		{
			name: "list missing profile",
			args: []string{"--home", home, "secret", "list", "missing"},
			want: `Profile "missing" does not exist.`,
		},
		{
			name: "set missing profile",
			args: []string{"--home", home, "secret", "set", "missing", "TOKEN", "--stdin"},
			in:   "x\n",
			want: `Profile "missing" does not exist.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			cmd := NewRootCommand(out, out)
			cmd.SetIn(strings.NewReader(tt.in))
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected error for missing profile")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want contains %q", err, tt.want)
			}
			if !strings.Contains(err.Error(), "aip profile create --name missing") || !strings.Contains(err.Error(), "aip profile list") {
				t.Fatalf("error missing profile guidance: %v", err)
			}
		})
	}
}

func TestSecretSetValidationErrors(t *testing.T) {
	keyring.MockInit()
	home := t.TempDir()
	createProfileForTest(t, home, t.TempDir())

	tests := []struct {
		name string
		args []string
		in   string
		want string
	}{
		{
			name: "empty secret name",
			args: []string{"--home", home, "secret", "set", "mgcs", "", "--stdin"},
			in:   "x\n",
			want: "secret name must not be empty",
		},
		{
			name: "blank stdin value",
			args: []string{"--home", home, "secret", "set", "mgcs", "EMPTY", "--stdin"},
			in:   "\n",
			want: "secret value must not be empty",
		},
		{
			name: "hash-prefixed name",
			args: []string{"--home", home, "secret", "set", "mgcs", "#TOKEN", "--stdin"},
			in:   "x\n",
			want: "must match",
		},
		{
			name: "equals in name",
			args: []string{"--home", home, "secret", "set", "mgcs", "A=B", "--stdin"},
			in:   "x\n",
			want: "must match",
		},
		{
			name: "leading digit name",
			args: []string{"--home", home, "secret", "set", "mgcs", "1TOKEN", "--stdin"},
			in:   "x\n",
			want: "must match",
		},
		{
			name: "newline in name",
			args: []string{"--home", home, "secret", "set", "mgcs", "BAD\nNAME", "--stdin"},
			in:   "x\n",
			want: "must match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			cmd := NewRootCommand(out, out)
			cmd.SetIn(strings.NewReader(tt.in))
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
