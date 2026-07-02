package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInitCreatesAIPHome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip-home")
	runInit(t, home)

	assertInitLayout(t, home)
	assertConfigContent(t, home)
	assertModes(t, home, map[string]os.FileMode{
		"":                                 0o700,
		"profiles":                         0o700,
		"shims":                            0o700,
		"library":                          0o700,
		filepath.Join("library", "skills"): 0o700,
		"secrets":                          0o700,
		"templates":                        0o700,
		"logs":                             0o700,
	})
	assertFileMode(t, filepath.Join(home, "config.yaml"), 0o600)
	assertFileMode(t, filepath.Join(home, "shims", "codex"), 0o755)
}

func TestInitTightensExistingPermissions(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip-home")
	for _, dir := range []string{
		home,
		filepath.Join(home, "profiles"),
		filepath.Join(home, "shims"),
		filepath.Join(home, "library"),
		filepath.Join(home, "library", "skills"),
		filepath.Join(home, "secrets"),
		filepath.Join(home, "templates"),
		filepath.Join(home, "logs"),
	} {
		if err := os.MkdirAll(dir, 0o777); err != nil {
			t.Fatalf("precreate %s: %v", dir, err)
		}
		if err := os.Chmod(dir, 0o777); err != nil {
			t.Fatalf("chmod %s: %v", dir, err)
		}
	}
	config := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(config, []byte("root_dir: /tmp/old\ndefault_safety: permissive\n"), 0o666); err != nil {
		t.Fatalf("prewrite config: %v", err)
	}
	if err := os.Chmod(config, 0o666); err != nil {
		t.Fatalf("chmod config: %v", err)
	}

	runInit(t, home)

	assertConfigContent(t, home)
	assertModes(t, home, map[string]os.FileMode{
		"":                                 0o700,
		"profiles":                         0o700,
		"shims":                            0o700,
		"library":                          0o700,
		filepath.Join("library", "skills"): 0o700,
		"secrets":                          0o700,
		"templates":                        0o700,
		"logs":                             0o700,
	})
	assertFileMode(t, filepath.Join(home, "config.yaml"), 0o600)
}

func TestInitWizardConfirmsAndCreates(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip-home")

	var out, errOut bytes.Buffer
	cmd := NewRootCommand(&out, &errOut)
	cmd.SetIn(strings.NewReader("y\n"))
	cmd.SetArgs([]string{"--home", home, "init"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(home, "config.yaml")); err != nil {
		t.Fatalf("config.yaml missing: %v", err)
	}
	stdout := out.String()
	headingIndex := strings.Index(stdout, "Detected AI CLIs:\n")
	if headingIndex == -1 {
		t.Fatalf("stdout missing detected CLI summary:\n%s", stdout)
	}
	if !strings.Contains(stdout, "  [") {
		t.Fatalf("stdout missing detected CLI rows:\n%s", stdout)
	}
	finalIndex := strings.Index(stdout, "AIP initialized at "+home)
	if finalIndex == -1 {
		t.Fatalf("stdout missing final init message:\n%s", stdout)
	}
	if finalIndex < headingIndex {
		t.Fatalf("stdout final init message appears before detection summary:\n%s", stdout)
	}
	if got := errOut.String(); !strings.Contains(got, "Initialize AIP in "+home+"? [y]: ") {
		t.Fatalf("stderr missing confirmation prompt:\n%s", got)
	}
}

func TestInitWizardCancelsBeforeWriting(t *testing.T) {
	home := filepath.Join(t.TempDir(), "aip-home")

	var out, errOut bytes.Buffer
	cmd := NewRootCommand(&out, &errOut)
	cmd.SetIn(strings.NewReader("n\n"))
	cmd.SetArgs([]string{"--home", home, "init"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute returned nil error, want init cancelled")
	}
	if got := err.Error(); got != "init cancelled" {
		t.Fatalf("Execute error = %q, want %q", got, "init cancelled")
	}
	if _, statErr := os.Stat(home); !os.IsNotExist(statErr) {
		t.Fatalf("init root stat error = %v, want not exist", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(home, "config.yaml")); !os.IsNotExist(statErr) {
		t.Fatalf("config.yaml stat error = %v, want not exist", statErr)
	}
}

func runInit(t *testing.T, home string) {
	t.Helper()

	var out bytes.Buffer
	cmd := NewRootCommand(&out, &out)
	cmd.SetArgs([]string{"--home", home, "init", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
}

func assertInitLayout(t *testing.T, home string) {
	t.Helper()

	for _, rel := range []string{"config.yaml", "profiles", "shims", "library", filepath.Join("library", "skills"), "secrets", "templates", "logs"} {
		if _, err := os.Stat(filepath.Join(home, rel)); err != nil {
			t.Fatalf("%s missing: %v", rel, err)
		}
	}
}

func assertConfigContent(t *testing.T, home string) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(home, "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var cfg map[string]string
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if got := cfg["root_dir"]; got != home {
		t.Fatalf("root_dir = %q, want %q", got, home)
	}
	if got := cfg["default_safety"]; got != "read-only" {
		t.Fatalf("default_safety = %q, want %q", got, "read-only")
	}
}

func assertModes(t *testing.T, home string, checks map[string]os.FileMode) {
	t.Helper()

	for rel, want := range checks {
		assertFileMode(t, filepath.Join(home, rel), want)
	}
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %04o, want %04o", path, got, want)
	}
}
