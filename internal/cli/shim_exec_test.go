package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/naveedcs/aip/internal/paths"
	"github.com/naveedcs/aip/internal/profile"
	"github.com/naveedcs/aip/internal/secrets"
	"github.com/zalando/go-keyring"
)

func TestShimExecPassthroughWhenProfileUnset(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell test binary")
	}

	home := filepath.Join(t.TempDir(), "aip")
	binDir := t.TempDir()
	shimDir := filepath.Join(home, "shims")
	if err := os.MkdirAll(shimDir, 0o700); err != nil {
		t.Fatalf("mkdir shims: %v", err)
	}
	capture := filepath.Join(t.TempDir(), "capture.txt")
	writeFakeTool(t, filepath.Join(binDir, "codex"), capture)

	t.Setenv("AIP_HOME", home)
	t.Setenv("AIP_PROFILE", "")
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+binDir)
	t.Setenv("AIP_PARENT_ONLY", "kept")
	workDir := t.TempDir()
	changeDir(t, workDir)
	wantWD := currentWorkingDir(t)

	runShimExec(t, "codex", "hello")

	got := readCapture(t, capture)
	assertPWD(t, got, wantWD)
	for _, want := range []string{
		"arg:hello",
		"AIP_PARENT_ONLY=kept",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("capture missing %q in:\n%s", want, got)
		}
	}
}

func TestShimExecPassesToolFlagsThrough(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell test binary")
	}

	home := filepath.Join(t.TempDir(), "aip")
	binDir := t.TempDir()
	shimDir := filepath.Join(home, "shims")
	if err := os.MkdirAll(shimDir, 0o700); err != nil {
		t.Fatalf("mkdir shims: %v", err)
	}
	capture := filepath.Join(t.TempDir(), "capture.txt")
	writeFakeTool(t, filepath.Join(binDir, "codex"), capture)

	t.Setenv("AIP_HOME", home)
	t.Setenv("AIP_PROFILE", "")
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+binDir)

	runShimExec(t, "codex", "--foo", "bar")

	got := readCapture(t, capture)
	for _, want := range []string{"arg:--foo", "arg:bar"} {
		if !strings.Contains(got, want) {
			t.Fatalf("capture missing %q in:\n%s", want, got)
		}
	}
}

func TestShimExecUnsupportedTool(t *testing.T) {
	var out, errOut bytes.Buffer
	cmd := NewRootCommand(&out, &errOut)
	cmd.SetArgs([]string{"shim-exec", "unknown"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected unsupported tool error")
	}
	if got, want := err.Error(), `unsupported tool "unknown"`; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestShimExecActiveProfileInjectsToolHomeAndSecret(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell test binary")
	}
	keyring.MockInit()

	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createProfileForTest(t, home, project)

	store := profile.NewStore(paths.ForRoot(home))
	prof, err := store.Load("mgcs")
	if err != nil {
		t.Fatalf("load profile: %v", err)
	}
	prof.Secrets.Keys = []string{"GITHUB_TOKEN"}
	if err := store.Save(prof); err != nil {
		t.Fatalf("save profile: %v", err)
	}
	if err := secrets.NewKeychain().Set("mgcs", "GITHUB_TOKEN", "ghp_secret"); err != nil {
		t.Fatalf("set secret: %v", err)
	}

	binDir := t.TempDir()
	capture := filepath.Join(t.TempDir(), "capture.txt")
	writeFakeTool(t, filepath.Join(binDir, "codex"), capture)

	t.Setenv("AIP_HOME", home)
	t.Setenv("AIP_PROFILE", "mgcs")
	t.Setenv("PATH", filepath.Join(home, "shims")+string(os.PathListSeparator)+binDir)
	t.Setenv("AIP_PARENT_ONLY", "should-not-leak")
	workDir := t.TempDir()
	changeDir(t, workDir)
	wantWD := currentWorkingDir(t)

	runShimExec(t, "codex", "run")

	got := readCapture(t, capture)
	assertPWD(t, got, wantWD)
	toolHome := filepath.Join(home, "profiles", "mgcs", "tools", "codex")
	for _, want := range []string{
		"arg:run",
		"CODEX_HOME=" + toolHome,
		"AIP_PROFILE=mgcs",
		"GITHUB_TOKEN=ghp_secret",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("capture missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "AIP_PARENT_ONLY=should-not-leak") {
		t.Fatalf("capture leaked unsafe parent env:\n%s", got)
	}
}

func TestShimExecAdminActiveProfileCancelsBeforeExecution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell test binary")
	}

	home := filepath.Join(t.TempDir(), "aip")
	project := t.TempDir()
	createAdminProfileForTest(t, home, project)

	binDir := t.TempDir()
	capture := filepath.Join(t.TempDir(), "capture.txt")
	writeFakeTool(t, filepath.Join(binDir, "codex"), capture)

	t.Setenv("AIP_HOME", home)
	t.Setenv("AIP_PROFILE", "mgcs")
	t.Setenv("PATH", filepath.Join(home, "shims")+string(os.PathListSeparator)+binDir)

	var out, errOut bytes.Buffer
	cmd := NewRootCommand(&out, &errOut)
	cmd.SetIn(strings.NewReader("wrong\n"))
	cmd.SetArgs([]string{"shim-exec", "codex", "run"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected admin cancellation error")
	}
	if !strings.Contains(err.Error(), "admin launch cancelled") {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(capture); !os.IsNotExist(statErr) {
		t.Fatalf("fake tool executed, capture stat error = %v", statErr)
	}
	if got := out.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
}

func runShimExec(t *testing.T, args ...string) {
	t.Helper()

	var out, errOut bytes.Buffer
	cmd := NewRootCommand(&out, &errOut)
	cmd.SetArgs(append([]string{"shim-exec"}, args...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("shim-exec %v failed: %v\nstderr: %s", args, err, errOut.String())
	}
}

func writeFakeTool(t *testing.T, path string, capture string) {
	t.Helper()

	script := `#!/bin/sh
{
  printf 'pwd=%s\n' "$(pwd)"
  printf 'CODEX_HOME=%s\n' "${CODEX_HOME:-}"
  printf 'AIP_PROFILE=%s\n' "${AIP_PROFILE:-}"
  printf 'GITHUB_TOKEN=%s\n' "${GITHUB_TOKEN:-}"
  printf 'AIP_PARENT_ONLY=%s\n' "${AIP_PARENT_ONLY:-}"
  for arg in "$@"; do
    printf 'arg:%s\n' "$arg"
  done
} > ` + shellQuote(capture) + `
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tool: %v", err)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func readCapture(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read capture: %v", err)
	}
	return string(data)
}

func currentWorkingDir(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return wd
}

func changeDir(t *testing.T, dir string) {
	t.Helper()

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore wd %s: %v", oldWD, err)
		}
	})
}

func assertPWD(t *testing.T, got string, want string) {
	t.Helper()

	if strings.Contains(got, "pwd="+want+"\n") {
		return
	}
	if evaluated, err := filepath.EvalSymlinks(want); err == nil && strings.Contains(got, "pwd="+evaluated+"\n") {
		return
	}
	t.Fatalf("capture missing current working directory %q in:\n%s", want, got)
}
