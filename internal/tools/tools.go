package tools

import (
	"bytes"
	"os"
	"path/filepath"
	"syscall"
)

type ID string

const (
	Codex   ID = "codex"
	Claude  ID = "claude"
	Gemini  ID = "gemini"
	Copilot ID = "copilot"
)

type Tool struct {
	ID          ID
	DisplayName string
	Binary      string
	HomeEnv     string
	LoginArgs   []string
}

type Detection struct {
	Tool      Tool
	Installed bool
	Path      string
}

var registry = []Tool{
	{ID: Codex, DisplayName: "Codex", Binary: "codex", HomeEnv: "CODEX_HOME", LoginArgs: []string{"login"}},
	{ID: Claude, DisplayName: "Claude Code", Binary: "claude", HomeEnv: "CLAUDE_CONFIG_DIR", LoginArgs: []string{"login"}},
	{ID: Gemini, DisplayName: "Gemini CLI", Binary: "gemini", HomeEnv: "GEMINI_CLI_HOME", LoginArgs: []string{"login"}},
	{ID: Copilot, DisplayName: "GitHub Copilot CLI", Binary: "copilot", HomeEnv: "COPILOT_HOME", LoginArgs: []string{"login"}},
}

func cloneTool(tool Tool) Tool {
	cloned := tool
	if tool.LoginArgs != nil {
		cloned.LoginArgs = append([]string(nil), tool.LoginArgs...)
	}
	return cloned
}

func All() []Tool {
	out := make([]Tool, len(registry))
	for i, tool := range registry {
		out[i] = cloneTool(tool)
	}
	return out
}

func Get(id ID) (Tool, bool) {
	for _, tool := range registry {
		if tool.ID == id {
			return cloneTool(tool), true
		}
	}
	return Tool{}, false
}

func Detect() []Detection {
	tools := All()
	out := make([]Detection, 0, len(tools))
	for _, tool := range tools {
		path, ok := findRealExecutable(tool)
		out = append(out, Detection{
			Tool:      tool,
			Installed: ok,
			Path:      path,
		})
	}
	return out
}

func findRealExecutable(tool Tool) (string, bool) {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			dir = "."
		}

		candidate := filepath.Join(filepath.Clean(dir), tool.Binary)
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if info.IsDir() || syscall.Access(candidate, 0x1) != nil {
			continue
		}
		if isGeneratedAIPShim(candidate, string(tool.ID)) {
			continue
		}
		abs, err := filepath.Abs(candidate)
		if err != nil {
			return "", false
		}
		return abs, true
	}
	return "", false
}

func isGeneratedAIPShim(path, toolID string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	expected := "#!/bin/sh\nexec aip shim-exec " + toolID + " \"$@\"\n"
	return bytes.Equal(content, []byte(expected))
}
