package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		RecommendedProfile: "acme",
		ProjectType:        "ecommerce",
		RecommendedMCP:     []string{"filesystem", "github"},
	}

	if err := Save(dir, cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if _, err := os.Stat(ConfigPath(dir)); err != nil {
		t.Fatalf("profile config missing: %v", err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got.RecommendedProfile != cfg.RecommendedProfile {
		t.Fatalf("RecommendedProfile = %q, want %q", got.RecommendedProfile, cfg.RecommendedProfile)
	}
	if got.ProjectType != cfg.ProjectType {
		t.Fatalf("ProjectType = %q, want %q", got.ProjectType, cfg.ProjectType)
	}
	if strings.Join(got.RecommendedMCP, ",") != strings.Join(cfg.RecommendedMCP, ",") {
		t.Fatalf("RecommendedMCP = %#v, want %#v", got.RecommendedMCP, cfg.RecommendedMCP)
	}
}

func TestEnsureGitignoreAppendsOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(path, []byte("dist/\n"), 0o644); err != nil {
		t.Fatalf("seed .gitignore: %v", err)
	}

	changed, err := EnsureGitignore(dir)
	if err != nil {
		t.Fatalf("EnsureGitignore returned error: %v", err)
	}
	if !changed {
		t.Fatal("EnsureGitignore changed = false, want true")
	}

	changed, err = EnsureGitignore(dir)
	if err != nil {
		t.Fatalf("second EnsureGitignore returned error: %v", err)
	}
	if changed {
		t.Fatal("second EnsureGitignore changed = true, want false")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	text := string(data)
	if strings.Count(text, GitignoreBlock) != 1 {
		t.Fatalf(".gitignore should contain one AIP block, got %q", text)
	}
	if !strings.Contains(text, ".aip/env.local") {
		t.Fatalf(".gitignore missing env.local pattern: %q", text)
	}
}

func TestEnsureGitignorePreservesExistingMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(path, []byte("dist/\n"), 0o600); err != nil {
		t.Fatalf("seed .gitignore: %v", err)
	}

	changed, err := EnsureGitignore(dir)
	if err != nil {
		t.Fatalf("EnsureGitignore returned error: %v", err)
	}
	if !changed {
		t.Fatal("EnsureGitignore changed = false, want true")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat .gitignore: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf(".gitignore mode = %o, want 600", got)
	}
}

func TestEnsureGitignoreUsesEnvLocalSentinel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	content := "# custom local ignores\r\n.aip/env.local\r\ndist/\r\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("seed .gitignore: %v", err)
	}

	changed, err := EnsureGitignore(dir)
	if err != nil {
		t.Fatalf("EnsureGitignore returned error: %v", err)
	}
	if changed {
		t.Fatal("EnsureGitignore changed = true, want false")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	text := string(data)
	if text != content {
		t.Fatalf(".gitignore changed unexpectedly: got %q, want %q", text, content)
	}
	if strings.Contains(text, GitignoreBlock) {
		t.Fatalf(".gitignore should not get duplicate AIP block: %q", text)
	}
}
