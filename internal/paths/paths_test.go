package paths

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func setTestHome(t *testing.T, home string) {
	t.Helper()

	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		drive := filepath.VolumeName(home)
		t.Setenv("USERPROFILE", home)
		t.Setenv("HOMEDRIVE", drive)
		t.Setenv("HOMEPATH", strings.TrimPrefix(home, drive))
	}
}

func TestResolveUsesFlagBeforeEnvAndDefault(t *testing.T) {
	t.Setenv("AIP_HOME", filepath.Join(t.TempDir(), "env-home"))

	got, err := ResolveRoot(filepath.Join(t.TempDir(), "flag-home"))
	if err != nil {
		t.Fatalf("ResolveRoot returned error: %v", err)
	}
	if filepath.Base(got) != "flag-home" {
		t.Fatalf("ResolveRoot should prefer flag home, got %q", got)
	}
}

func TestResolveUsesEnvWhenFlagEmpty(t *testing.T) {
	home := t.TempDir()
	envHome := filepath.Join(t.TempDir(), "env-home")
	setTestHome(t, home)
	t.Setenv("AIP_HOME", envHome)

	got, err := ResolveRoot("")
	if err != nil {
		t.Fatalf("ResolveRoot returned error: %v", err)
	}

	if filepath.Base(got) != "env-home" {
		t.Fatalf("ResolveRoot should prefer AIP_HOME, got %q", got)
	}
	if got != envHome {
		t.Fatalf("ResolveRoot = %q, want %q", got, envHome)
	}
}

func TestResolveDefaultsToHomeWhenFlagAndEnvEmpty(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	t.Setenv("AIP_HOME", "")

	got, err := ResolveRoot("")
	if err != nil {
		t.Fatalf("ResolveRoot returned error: %v", err)
	}

	want := filepath.Join(home, ".aip")
	if got != want {
		t.Fatalf("ResolveRoot = %q, want %q", got, want)
	}
}

func TestResolveExpandsTilde(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	t.Setenv("AIP_HOME", "")

	got, err := ResolveRoot("~/.custom-aip")
	if err != nil {
		t.Fatalf("ResolveRoot returned error: %v", err)
	}

	want := filepath.Join(home, ".custom-aip")
	if got != want {
		t.Fatalf("ResolveRoot = %q, want %q", got, want)
	}
}

func TestForRootBuildsKnownDirectories(t *testing.T) {
	root := filepath.Join(t.TempDir(), "aip")
	got := ForRoot(root)

	if got.ProfileDir("mgcs") != filepath.Join(root, "profiles", "mgcs") {
		t.Fatalf("bad profile dir: %q", got.ProfileDir("mgcs"))
	}
	if got.ToolConfigDir("mgcs", "codex") != filepath.Join(root, "profiles", "mgcs", "tools", "codex") {
		t.Fatalf("bad tool config dir: %q", got.ToolConfigDir("mgcs", "codex"))
	}
}

func TestToolConfigDirIsUnderProfile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "aip")
	p := ForRoot(root)

	got := p.ToolConfigDir("acme", "claude")
	want := filepath.Join(root, "profiles", "acme", "tools", "claude")
	if got != want {
		t.Fatalf("ToolConfigDir = %q, want %q", got, want)
	}
}

func TestShimsAndLibraryDirs(t *testing.T) {
	root := filepath.Join(t.TempDir(), "aip")
	p := ForRoot(root)

	if p.ShimsDir != filepath.Join(root, "shims") {
		t.Fatalf("ShimsDir = %q", p.ShimsDir)
	}
	if p.LibrarySkillsDir != filepath.Join(root, "library", "skills") {
		t.Fatalf("LibrarySkillsDir = %q", p.LibrarySkillsDir)
	}
}
