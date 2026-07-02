package shim

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/naveedcs/aip/internal/paths"
	"github.com/naveedcs/aip/internal/tools"
)

func TestGenerateWritesExecutableShims(t *testing.T) {
	p := paths.ForRoot(t.TempDir())

	if err := Generate(p); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	for _, tool := range tools.All() {
		shimPath := filepath.Join(p.ShimsDir, tool.Binary)
		info, err := os.Stat(shimPath)
		if err != nil {
			t.Fatalf("stat %s shim: %v", tool.Binary, err)
		}
		if info.Mode().Perm()&0o111 == 0 {
			t.Fatalf("%s shim mode = %v, want executable bit set", tool.Binary, info.Mode().Perm())
		}

		data, err := os.ReadFile(shimPath)
		if err != nil {
			t.Fatalf("read %s shim: %v", tool.Binary, err)
		}
		want := "exec aip shim-exec " + string(tool.ID)
		if !strings.Contains(string(data), want) {
			t.Fatalf("%s shim content = %q, want %q", tool.Binary, string(data), want)
		}
	}
}

func TestGenerateIsIdempotent(t *testing.T) {
	p := paths.ForRoot(t.TempDir())

	if err := Generate(p); err != nil {
		t.Fatalf("first Generate returned error: %v", err)
	}
	shimPath := filepath.Join(p.ShimsDir, "claude")
	before, err := os.Stat(shimPath)
	if err != nil {
		t.Fatalf("stat claude shim after first Generate: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	if err := Generate(p); err != nil {
		t.Fatalf("second Generate returned error: %v", err)
	}
	after, err := os.Stat(shimPath)
	if err != nil {
		t.Fatalf("stat claude shim after second Generate: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("claude shim modtime changed on idempotent Generate: before %v after %v", before.ModTime(), after.ModTime())
	}
}

func TestGenerateReplacesSymlinkedShimPath(t *testing.T) {
	p := paths.ForRoot(t.TempDir())
	if err := os.MkdirAll(p.ShimsDir, 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(target, []byte(ShimContent("claude")), 0o600); err != nil {
		t.Fatalf("WriteFile target returned error: %v", err)
	}
	shimPath := filepath.Join(p.ShimsDir, "claude")
	if err := os.Symlink(target, shimPath); err != nil {
		t.Fatalf("Symlink returned error: %v", err)
	}

	if err := Generate(p); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	info, err := os.Lstat(shimPath)
	if err != nil {
		t.Fatalf("Lstat shim returned error: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("Generate left shim path as a symlink")
	}
	targetInfo, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat target returned error: %v", err)
	}
	if targetInfo.Mode().Perm() != 0o600 {
		t.Fatalf("Generate chmodded symlink target to %o", targetInfo.Mode().Perm())
	}
}

func TestShimContent(t *testing.T) {
	got := ShimContent("claude")
	want := "#!/bin/sh\nexec aip shim-exec claude \"$@\"\n"
	if got != want {
		t.Fatalf("ShimContent = %q, want %q", got, want)
	}
}
