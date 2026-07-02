package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetCodex(t *testing.T) {
	tool, ok := Get("codex")
	if !ok {
		t.Fatal("codex tool not found")
	}
	if tool.HomeEnv != "CODEX_HOME" {
		t.Fatalf("codex HomeEnv = %q", tool.HomeEnv)
	}
	if tool.Binary != "codex" {
		t.Fatalf("codex Binary = %q", tool.Binary)
	}
}

func TestAllIncludesMVPTools(t *testing.T) {
	got := All()
	want := []ID{"codex", "claude", "gemini", "copilot"}
	if len(got) != len(want) {
		t.Fatalf("All length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i] {
			t.Fatalf("All[%d] = %q, want %q", i, got[i].ID, want[i])
		}
	}
}

func TestUnknownTool(t *testing.T) {
	if _, ok := Get("goose"); ok {
		t.Fatal("goose should not be enabled in the MVP registry")
	}
}

func TestGeminiUsesIsolatedCLIHomeEnv(t *testing.T) {
	tool, ok := Get(Gemini)
	if !ok {
		t.Fatal("gemini missing from registry")
	}
	if tool.HomeEnv != "GEMINI_CLI_HOME" {
		t.Fatalf("gemini HomeEnv = %q, want GEMINI_CLI_HOME", tool.HomeEnv)
	}
}

func TestReturnedToolsDoNotShareLoginArgs(t *testing.T) {
	got := All()
	got[0].LoginArgs[0] = "mutated"

	fresh := All()
	if fresh[0].LoginArgs[0] != "login" {
		t.Fatalf("fresh All LoginArgs[0] = %q, want %q", fresh[0].LoginArgs[0], "login")
	}

	tool, ok := Get(Codex)
	if !ok {
		t.Fatal("codex tool not found")
	}
	if tool.LoginArgs[0] != "login" {
		t.Fatalf("Get(Codex) LoginArgs[0] = %q, want %q", tool.LoginArgs[0], "login")
	}
}

func TestDetectDoesNotReportGeneratedAIPShimAsInstalledTool(t *testing.T) {
	shimDir := t.TempDir()
	for _, tool := range All() {
		mkToolShim(t, filepath.Join(shimDir, tool.Binary), string(tool.ID))
	}

	t.Setenv("PATH", shimDir)
	for _, detection := range Detect() {
		if detection.Installed {
			t.Fatalf("Detect reported %s installed via generated shim %q", detection.Tool.ID, detection.Path)
		}
		if detection.Path != "" {
			t.Fatalf("Detect path for %s = %q, want empty", detection.Tool.ID, detection.Path)
		}
	}
}

func TestDetectSkipsGeneratedAIPShimAndReportsLaterRealTool(t *testing.T) {
	shimDir := t.TempDir()
	realDir := t.TempDir()
	mkToolShim(t, filepath.Join(shimDir, "codex"), string(Codex))
	realCodex := filepath.Join(realDir, "codex")
	mkToolExec(t, realCodex)

	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+realDir)
	var codex Detection
	for _, detection := range Detect() {
		if detection.Tool.ID == Codex {
			codex = detection
			break
		}
	}
	if !codex.Installed {
		t.Fatal("Detect did not report codex installed")
	}
	if codex.Path != realCodex {
		t.Fatalf("Detect codex path = %q, want real binary %q", codex.Path, realCodex)
	}
}

func TestDetectSkipsNonExecutableFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows executable detection is extension based")
	}
	binDir := t.TempDir()
	codexPath := filepath.Join(binDir, "codex")
	if err := os.WriteFile(codexPath, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("write non-executable codex: %v", err)
	}

	t.Setenv("PATH", binDir)
	var codex Detection
	for _, detection := range Detect() {
		if detection.Tool.ID == Codex {
			codex = detection
			break
		}
	}
	if codex.Installed {
		t.Fatalf("Detect reported non-executable file as installed: %q", codex.Path)
	}
}

func TestWindowsPathExtsNormalizesAndDeduplicates(t *testing.T) {
	t.Setenv("PATHEXT", ".EXE;.CMD;BAT;.exe;;")
	got := windowsPathExts()
	want := []string{".EXE", ".CMD", ".BAT"}
	if len(got) != len(want) {
		t.Fatalf("windowsPathExts length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("windowsPathExts[%d] = %q, want %q; got %#v", i, got[i], want[i], got)
		}
	}
}

func mkToolShim(t *testing.T, path, toolID string) {
	t.Helper()
	content := "#!/bin/sh\nexec aip shim-exec " + toolID + " \"$@\"\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write generated shim: %v", err)
	}
}

func mkToolExec(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
}
