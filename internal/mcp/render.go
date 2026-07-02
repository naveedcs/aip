package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/naveedcs/aip/internal/fsutil"
	"github.com/naveedcs/aip/internal/paths"
	"github.com/naveedcs/aip/internal/profile"
	"github.com/naveedcs/aip/internal/tools"
)

type SecretRef struct {
	Target     string
	SecretName string
}

type RenderOptions struct {
	AllowPlaintextSecrets bool
	ResolveSecret         func(secretName string) (string, error)
	SkipCopilot           bool
}

type claudeConfig struct {
	MCPServers map[string]claudeServer `json:"mcpServers"`
}

type claudeServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type copilotConfig struct {
	MCPServers map[string]copilotServer `json:"mcpServers"`
}

type copilotServer struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Tools   []string          `json:"tools"`
}

func SecretRefs(prof profile.Profile) []SecretRef {
	return secretRefsFromServers(prof.MCP)
}

func LaunchSecretRefs(prof profile.Profile, toolID tools.ID) []SecretRef {
	switch toolID {
	case tools.Codex, tools.Claude, tools.Gemini:
	default:
		return nil
	}
	servers, _ := renderableServers(prof.MCP)
	return secretRefsFromServers(servers)
}

func secretRefsFromServers(servers map[string]profile.MCPServer) []SecretRef {
	var refs []SecretRef
	for _, serverName := range sortedServerNames(servers) {
		server := servers[serverName]
		for _, target := range sortedStringKeys(server.Env) {
			secretName, ok := profile.ParseSecretRef(server.Env[target])
			if !ok {
				continue
			}
			refs = append(refs, SecretRef{
				Target:     target,
				SecretName: secretName,
			})
		}
	}
	return refs
}

func ValidateEnvSecretRefs(env map[string]string) error {
	return validateEnvSecretRefs("", env)
}

func Render(p paths.Paths, prof profile.Profile) (notes []string, err error) {
	return RenderWithOptions(p, prof, RenderOptions{})
}

func RenderWithOptions(p paths.Paths, prof profile.Profile, opts RenderOptions) (notes []string, err error) {
	if err := validateServerSecretRefs(prof.MCP); err != nil {
		return nil, err
	}

	servers, skipNotes := renderableServers(prof.MCP)
	notes = append(notes, skipNotes...)

	if prof.Tools[tools.Codex].Enabled {
		path := filepath.Join(p.ToolConfigDir(prof.Name, string(tools.Codex)), "config.toml")
		if err := fsutil.WriteFileAtomic(path, []byte(renderCodexTOML(servers)), 0o600); err != nil {
			return notes, err
		}
	}
	if prof.Tools[tools.Claude].Enabled {
		path := filepath.Join(p.ToolConfigDir(prof.Name, string(tools.Claude)), "mcp.json")
		data, err := renderClaudeJSON(servers)
		if err != nil {
			return notes, err
		}
		if err := fsutil.WriteFileAtomic(path, data, 0o600); err != nil {
			return notes, err
		}
	}
	if prof.Tools[tools.Gemini].Enabled {
		path := filepath.Join(p.ToolConfigDir(prof.Name, string(tools.Gemini)), ".gemini", "settings.json")
		data, err := renderGeminiSettings(path, servers)
		if err != nil {
			return notes, err
		}
		if err := fsutil.WriteFileAtomic(path, data, 0o600); err != nil {
			return notes, err
		}
	}
	if prof.Tools[tools.Copilot].Enabled && !opts.SkipCopilot {
		path := filepath.Join(p.ToolConfigDir(prof.Name, string(tools.Copilot)), "mcp-config.json")
		data, copilotNotes, err := renderCopilotJSON(servers, opts)
		notes = append(notes, copilotNotes...)
		if err != nil {
			return notes, err
		}
		if err := fsutil.WriteFileAtomic(path, data, 0o600); err != nil {
			return notes, err
		}
	}

	return notes, nil
}

func LaunchArgs(p paths.Paths, prof profile.Profile, toolID tools.ID) []string {
	if toolID != tools.Claude {
		return nil
	}
	servers, _ := renderableServers(prof.MCP)
	if len(servers) == 0 {
		return nil
	}
	return []string{"--mcp-config", filepath.Join(p.ToolConfigDir(prof.Name, string(tools.Claude)), "mcp.json")}
}

func validateServerSecretRefs(servers map[string]profile.MCPServer) error {
	for _, name := range sortedServerNames(servers) {
		if err := validateEnvSecretRefs(name, servers[name].Env); err != nil {
			return err
		}
	}
	return nil
}

func validateEnvSecretRefs(serverName string, env map[string]string) error {
	for _, target := range sortedStringKeys(env) {
		value := env[target]
		if _, ok := profile.ParseSecretRef(value); ok {
			continue
		}
		if !strings.HasPrefix(value, "${secret:") || !strings.HasSuffix(value, "}") {
			continue
		}
		if serverName == "" {
			return fmt.Errorf("env %q has malformed secret reference %q", target, value)
		}
		return fmt.Errorf("MCP server %q env %q has malformed secret reference %q", serverName, target, value)
	}
	return nil
}

func renderableServers(input map[string]profile.MCPServer) (map[string]profile.MCPServer, []string) {
	out := make(map[string]profile.MCPServer, len(input))
	var notes []string
	for _, name := range sortedServerNames(input) {
		server := input[name]
		serverType := strings.TrimSpace(strings.ToLower(server.Type))
		if serverType != "" && serverType != "stdio" {
			notes = append(notes, fmt.Sprintf("MCP server %q uses unsupported type %q; skipping", name, server.Type))
			continue
		}
		out[name] = server
	}
	return out, notes
}

func renderCodexTOML(servers map[string]profile.MCPServer) string {
	var b strings.Builder
	b.WriteString("cli_auth_credentials_store = \"file\"\n")

	for _, name := range sortedServerNames(servers) {
		server := servers[name]
		table := "mcp_servers." + tomlKey(name)
		fmt.Fprintf(&b, "\n[%s]\n", table)
		fmt.Fprintf(&b, "command = %s\n", tomlBasicString(server.Command))
		writeTOMLStringArray(&b, "args", server.Args)

		literalEnv := map[string]string{}
		var secretTargets []string
		for _, target := range sortedStringKeys(server.Env) {
			value := server.Env[target]
			if _, ok := profile.ParseSecretRef(value); ok {
				secretTargets = append(secretTargets, target)
				continue
			}
			literalEnv[target] = value
		}
		writeTOMLStringArray(&b, "env_vars", secretTargets)
		if len(literalEnv) > 0 {
			fmt.Fprintf(&b, "\n[%s.env]\n", table)
			for _, key := range sortedStringKeys(literalEnv) {
				fmt.Fprintf(&b, "%s = %s\n", tomlKey(key), tomlBasicString(literalEnv[key]))
			}
		}
	}

	return b.String()
}

func renderClaudeJSON(servers map[string]profile.MCPServer) ([]byte, error) {
	cfg := claudeConfig{MCPServers: map[string]claudeServer{}}
	for _, name := range sortedServerNames(servers) {
		server := servers[name]
		out := claudeServer{
			Command: server.Command,
			Args:    cloneStrings(server.Args),
			Env:     renderClaudeEnv(server.Env),
		}
		cfg.MCPServers[name] = out
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func renderCopilotJSON(servers map[string]profile.MCPServer, opts RenderOptions) ([]byte, []string, error) {
	cfg := copilotConfig{MCPServers: map[string]copilotServer{}}
	var notes []string
	for _, name := range sortedServerNames(servers) {
		server := servers[name]
		env, hasSecret, err := renderCopilotEnv(name, server.Env, opts)
		if err != nil {
			return nil, notes, err
		}
		if hasSecret && (!opts.AllowPlaintextSecrets || opts.ResolveSecret == nil) {
			notes = append(notes, fmt.Sprintf("GitHub Copilot MCP server %q uses secret env; skipping plaintext config", name))
			continue
		}
		cfg.MCPServers[name] = copilotServer{
			Type:    "local",
			Command: server.Command,
			Args:    cloneStrings(server.Args),
			Env:     env,
			Tools:   []string{"*"},
		}
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, notes, err
	}
	return append(data, '\n'), notes, nil
}

func renderCopilotEnv(serverName string, env map[string]string, opts RenderOptions) (map[string]string, bool, error) {
	if len(env) == 0 {
		return nil, false, nil
	}
	out := make(map[string]string, len(env))
	hasSecret := false
	for _, target := range sortedStringKeys(env) {
		value := env[target]
		secretName, ok := profile.ParseSecretRef(value)
		if !ok {
			out[target] = value
			continue
		}
		hasSecret = true
		if !opts.AllowPlaintextSecrets || opts.ResolveSecret == nil {
			continue
		}
		resolved, err := opts.ResolveSecret(secretName)
		if err != nil {
			return nil, hasSecret, fmt.Errorf("resolve GitHub Copilot MCP server %q env %q secret %q: %w", serverName, target, secretName, err)
		}
		out[target] = resolved
	}
	if len(out) == 0 {
		return nil, hasSecret, nil
	}
	return out, hasSecret, nil
}

func renderGeminiSettings(path string, servers map[string]profile.MCPServer) ([]byte, error) {
	cfg := map[string]json.RawMessage{}
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if err == nil {
		if err := json.Unmarshal(existing, &cfg); err != nil {
			return nil, err
		}
	}

	rendered := map[string]claudeServer{}
	for _, name := range sortedServerNames(servers) {
		server := servers[name]
		rendered[name] = claudeServer{
			Command: server.Command,
			Args:    cloneStrings(server.Args),
			Env:     renderClaudeEnv(server.Env),
		}
	}
	mcpServers, err := json.Marshal(rendered)
	if err != nil {
		return nil, err
	}
	cfg["mcpServers"] = mcpServers

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func renderClaudeEnv(env map[string]string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	out := make(map[string]string, len(env))
	for _, target := range sortedStringKeys(env) {
		value := env[target]
		if _, ok := profile.ParseSecretRef(value); ok {
			out[target] = "${" + target + "}"
			continue
		}
		out[target] = value
	}
	return out
}

func writeTOMLStringArray(b *strings.Builder, key string, values []string) {
	if len(values) == 0 {
		return
	}
	b.WriteString(key)
	b.WriteString(" = [")
	for i, value := range values {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(tomlBasicString(value))
	}
	b.WriteString("]\n")
}

func sortedServerNames(servers map[string]profile.MCPServer) []string {
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedStringKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func tomlKey(key string) string {
	if key == "" {
		return tomlBasicString(key)
	}
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return tomlBasicString(key)
	}
	return key
}

func tomlBasicString(value string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u%04X`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return append([]string(nil), values...)
}
