package app

import (
	"testing"

	"github.com/naveedcs/aip/internal/paths"
)

func TestNewStoresResolvedPaths(t *testing.T) {
	root := t.TempDir()

	got := New(root)
	want := paths.ForRoot(root)

	if got.Paths.RootDir != want.RootDir {
		t.Fatalf("RootDir = %q, want %q", got.Paths.RootDir, want.RootDir)
	}
	if got.Paths.ProfileDir("mgcs") != want.ProfileDir("mgcs") {
		t.Fatalf("ProfileDir = %q, want %q", got.Paths.ProfileDir("mgcs"), want.ProfileDir("mgcs"))
	}
}
