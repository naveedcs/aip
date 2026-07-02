package activation

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunExecutesCommandWithEnvAndDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell script as the fake binary")
	}

	work := t.TempDir()
	out := filepath.Join(work, "out.txt")
	bin := filepath.Join(work, "faketool")
	script := "#!/bin/sh\n{ pwd -P; echo \"PROFILE=$AIP_PROFILE\"; } > \"" + out + "\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake bin: %v", err)
	}

	plan := Plan{
		Command: bin,
		Dir:     work,
		Env:     map[string]string{"AIP_PROFILE": "acme", "PATH": os.Getenv("PATH")},
	}
	if err := Run(plan); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	got := string(data)
	wantDir, err := filepath.EvalSymlinks(work)
	if err != nil {
		t.Fatalf("resolve work dir: %v", err)
	}
	if !strings.Contains(got, wantDir+"\n") {
		t.Fatalf("working dir not applied: got %q, want %q", got, wantDir)
	}
	if !strings.Contains(got, "PROFILE=acme") {
		t.Fatalf("env not applied: %q", got)
	}
}

func TestRunPropagatesExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell script as the fake binary")
	}

	work := t.TempDir()
	bin := filepath.Join(work, "faketool")
	script := "#!/bin/sh\nexit 7\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake bin: %v", err)
	}

	err := Run(Plan{
		Command: bin,
		Dir:     work,
		Env:     map[string]string{"PATH": os.Getenv("PATH")},
	})
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Run error = %T %v, want ExitCodeError", err, err)
	}
	if exitErr.Code != 7 {
		t.Fatalf("exit code = %d, want 7", exitErr.Code)
	}
}

func TestRunRawPropagatesExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell script as the fake binary")
	}

	work := t.TempDir()
	bin := filepath.Join(work, "faketool")
	script := "#!/bin/sh\nexit 3\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake bin: %v", err)
	}

	err := RunRaw(bin, nil)
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("RunRaw error = %T %v, want ExitCodeError", err, err)
	}
	if exitErr.Code != 3 {
		t.Fatalf("exit code = %d, want 3", exitErr.Code)
	}
}

func TestRunRawMapsSignalExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell script as the fake binary")
	}

	work := t.TempDir()
	bin := filepath.Join(work, "faketool")
	script := "#!/bin/sh\nkill -TERM $$\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake bin: %v", err)
	}

	err := RunRaw(bin, nil)
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("RunRaw error = %T %v, want ExitCodeError", err, err)
	}
	if exitErr.Code != 143 {
		t.Fatalf("exit code = %d, want 143", exitErr.Code)
	}
}

func TestRunRawInheritsEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell script as the fake binary")
	}

	work := t.TempDir()
	out := filepath.Join(work, "out.txt")
	bin := filepath.Join(work, "faketool")
	script := "#!/bin/sh\nprintf '%s' \"$AIP_RAW_TEST\" > \"" + out + "\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake bin: %v", err)
	}
	t.Setenv("AIP_RAW_TEST", "from-parent")

	if err := RunRaw(bin, nil); err != nil {
		t.Fatalf("RunRaw returned error: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != "from-parent" {
		t.Fatalf("raw env = %q, want from-parent", data)
	}
}

func TestFormatPlanMasksSecrets(t *testing.T) {
	var b bytes.Buffer
	FormatPlan(&b, Plan{
		Command:   "codex",
		Args:      []string{"--help"},
		Dir:       "/work",
		Env:       map[string]string{"GITHUB_TOKEN": "ghp_secret"},
		MaskedEnv: map[string]string{"GITHUB_TOKEN": "********", "AIP_PROFILE": "acme"},
	})

	s := b.String()
	if !strings.Contains(s, "codex --help") {
		t.Fatalf("command missing from output:\n%s", s)
	}
	if !strings.Contains(s, "GITHUB_TOKEN=********") {
		t.Fatalf("masked secret missing from output:\n%s", s)
	}
	if !strings.Contains(s, "AIP_PROFILE=acme") {
		t.Fatalf("profile marker missing from output:\n%s", s)
	}
	if strings.Contains(s, "ghp_secret") {
		t.Fatalf("unmasked secret leaked in output:\n%s", s)
	}
	if strings.Index(s, "AIP_PROFILE=acme") > strings.Index(s, "GITHUB_TOKEN=********") {
		t.Fatalf("masked env is not sorted:\n%s", s)
	}
}
