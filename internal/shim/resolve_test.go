package shim

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func TestResolveRealSkipsShimDir(t *testing.T) {
	shimDir := t.TempDir()
	realDir := t.TempDir()
	mkExec(t, filepath.Join(shimDir, "codex"))
	realBin := filepath.Join(realDir, "codex")
	mkExec(t, realBin)
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+realDir)
	got, err := ResolveReal("codex", shimDir)
	if err != nil {
		t.Fatalf("ResolveReal returned error: %v", err)
	}
	if got != realBin {
		t.Fatalf("ResolveReal = %q, want %q", got, realBin)
	}
}

func TestResolveRealNotFoundWhenOnlyInShimDir(t *testing.T) {
	shimDir := t.TempDir()
	mkExec(t, filepath.Join(shimDir, "codex"))
	t.Setenv("PATH", shimDir)
	if _, err := ResolveReal("codex", shimDir); err == nil {
		t.Fatal("ResolveReal found a binary inside the shim dir, want error")
	}
}

func TestResolveRealSkipsSymlinkAliasForShimDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	parent := t.TempDir()
	shimDir := filepath.Join(parent, "shims")
	if err := os.MkdirAll(shimDir, 0o700); err != nil {
		t.Fatalf("mkdir shim dir: %v", err)
	}
	alias := filepath.Join(parent, "shim-alias")
	if err := os.Symlink(shimDir, alias); err != nil {
		t.Fatalf("symlink shim dir: %v", err)
	}
	mkExec(t, filepath.Join(alias, "codex"))
	realDir := t.TempDir()
	realBin := filepath.Join(realDir, "codex")
	mkExec(t, realBin)

	t.Setenv("PATH", alias+string(os.PathListSeparator)+realDir)
	got, err := ResolveReal("codex", shimDir)
	if err != nil {
		t.Fatalf("ResolveReal returned error: %v", err)
	}
	if got != realBin {
		t.Fatalf("ResolveReal = %q, want %q", got, realBin)
	}
}

func TestResolveRealChoosesFirstExecutableOutsideShimDir(t *testing.T) {
	shimDir := t.TempDir()
	firstDir := t.TempDir()
	secondDir := t.TempDir()
	firstBin := filepath.Join(firstDir, "codex")
	mkExec(t, firstBin)
	mkExec(t, filepath.Join(secondDir, "codex"))

	t.Setenv("PATH", firstDir+string(os.PathListSeparator)+secondDir)
	got, err := ResolveReal("codex", shimDir)
	if err != nil {
		t.Fatalf("ResolveReal returned error: %v", err)
	}
	if got != firstBin {
		t.Fatalf("ResolveReal = %q, want first executable %q", got, firstBin)
	}
}

func TestResolveRealIgnoresNonExecutablesAndDirectories(t *testing.T) {
	shimDir := t.TempDir()
	nonExecDir := t.TempDir()
	dirNamedBinary := t.TempDir()
	realDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(nonExecDir, "codex"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("write non-executable: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dirNamedBinary, "codex"), 0o700); err != nil {
		t.Fatalf("mkdir binary dir: %v", err)
	}
	realBin := filepath.Join(realDir, "codex")
	mkExec(t, realBin)

	t.Setenv("PATH", nonExecDir+string(os.PathListSeparator)+dirNamedBinary+string(os.PathListSeparator)+realDir)
	got, err := ResolveReal("codex", shimDir)
	if err != nil {
		t.Fatalf("ResolveReal returned error: %v", err)
	}
	if got != realBin {
		t.Fatalf("ResolveReal = %q, want %q", got, realBin)
	}
}

func TestResolveRealTreatsEmptyPathEntryAsCurrentDirectory(t *testing.T) {
	shimDir := t.TempDir()
	workDir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	})
	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Chdir returned error: %v", err)
	}
	realBin := filepath.Join(workDir, "codex")
	mkExec(t, realBin)
	laterDir := t.TempDir()
	mkExec(t, filepath.Join(laterDir, "codex"))

	t.Setenv("PATH", string(os.PathListSeparator)+laterDir)
	got, err := ResolveReal("codex", shimDir)
	if err != nil {
		t.Fatalf("ResolveReal returned error: %v", err)
	}
	want, err := filepath.Abs("codex")
	if err != nil {
		t.Fatalf("Abs returned error: %v", err)
	}
	if got != want {
		t.Fatalf("ResolveReal = %q, want absolute current-directory candidate %q", got, want)
	}
}

func TestResolveRealReturnsAbsolutePathForRelativePathEntry(t *testing.T) {
	shimDir := t.TempDir()
	parent := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	})
	if err := os.Chdir(parent); err != nil {
		t.Fatalf("Chdir returned error: %v", err)
	}
	if err := os.Mkdir("bin", 0o700); err != nil {
		t.Fatalf("mkdir relative bin: %v", err)
	}
	realBin := filepath.Join(parent, "bin", "codex")
	mkExec(t, realBin)

	t.Setenv("PATH", "bin")
	got, err := ResolveReal("codex", shimDir)
	if err != nil {
		t.Fatalf("ResolveReal returned error: %v", err)
	}
	want, err := filepath.Abs(filepath.Join("bin", "codex"))
	if err != nil {
		t.Fatalf("Abs returned error: %v", err)
	}
	if got != want {
		t.Fatalf("ResolveReal = %q, want absolute relative-entry candidate %q", got, want)
	}
}

func TestResolveRealSkipsGeneratedAIPShimsOutsideCurrentShimDir(t *testing.T) {
	currentShims := t.TempDir()
	oldShims := t.TempDir()
	realDir := t.TempDir()
	mkAIPShim(t, filepath.Join(currentShims, "codex"), "codex")
	mkAIPShim(t, filepath.Join(oldShims, "codex"), "codex")
	realBin := filepath.Join(realDir, "codex")
	mkExec(t, realBin)

	t.Setenv("PATH", currentShims+string(os.PathListSeparator)+oldShims+string(os.PathListSeparator)+realDir)
	got, err := ResolveReal("codex", currentShims)
	if err != nil {
		t.Fatalf("ResolveReal returned error: %v", err)
	}
	if got != realBin {
		t.Fatalf("ResolveReal = %q, want real binary %q", got, realBin)
	}
}

func TestResolveRealDoesNotSkipUserWrapperWithSimilarContent(t *testing.T) {
	shimDir := t.TempDir()
	wrapperDir := t.TempDir()
	wrapper := filepath.Join(wrapperDir, "codex")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec aip shim-exec claude \"$@\"\n"), 0o755); err != nil {
		t.Fatalf("write wrapper: %v", err)
	}

	t.Setenv("PATH", wrapperDir)
	got, err := ResolveReal("codex", shimDir)
	if err != nil {
		t.Fatalf("ResolveReal returned error: %v", err)
	}
	if got != wrapper {
		t.Fatalf("ResolveReal = %q, want wrapper %q", got, wrapper)
	}
}

func TestResolveRealIgnoresFileCurrentUserCannotExecute(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission test")
	}
	shimDir := t.TempDir()
	blockedDir := t.TempDir()
	realDir := t.TempDir()
	blocked := filepath.Join(blockedDir, "codex")
	if err := os.WriteFile(blocked, []byte("#!/bin/sh\n"), 0o001); err != nil {
		t.Fatalf("write blocked executable: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(blocked, 0o700)
	})
	realBin := filepath.Join(realDir, "codex")
	mkExec(t, realBin)

	t.Setenv("PATH", blockedDir+string(os.PathListSeparator)+realDir)
	got, err := ResolveReal("codex", shimDir)
	if err != nil {
		t.Fatalf("ResolveReal returned error: %v", err)
	}
	if os.Geteuid() == 0 {
		t.Skip("root can execute the mode " + strconv.FormatInt(0o001, 8) + " probe")
	}
	if got != realBin {
		t.Fatalf("ResolveReal = %q, want %q", got, realBin)
	}
}

func mkExec(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
}

func mkAIPShim(t *testing.T, path, toolID string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(ShimContent(toolID)), 0o755); err != nil {
		t.Fatalf("write generated shim: %v", err)
	}
}
