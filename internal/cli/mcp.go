package cli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/naveedcs/aip/internal/mcp"
	"github.com/naveedcs/aip/internal/paths"
	"github.com/naveedcs/aip/internal/profile"
	"github.com/naveedcs/aip/internal/secrets"
	"github.com/spf13/cobra"
)

var mcpServerNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

type mcpAddOptions struct {
	serverType string
	command    string
	args       []string
	env        []string
	readOnly   bool
}

func newMCPCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "mcp", Short: "Manage MCP servers"}
	cmd.AddCommand(newMCPAddCommand(opts))
	cmd.AddCommand(newMCPListCommand(opts))
	cmd.AddCommand(newMCPRemoveCommand(opts))
	cmd.AddCommand(newMCPSyncCommand(opts))
	cmd.AddCommand(newMCPTestCommand(opts))
	return cmd
}

func newMCPAddCommand(opts *rootOptions) *cobra.Command {
	addOpts := &mcpAddOptions{}
	cmd := &cobra.Command{
		Use:   "add <profile> <name>",
		Short: "Add or update a profile MCP server",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			prof, err := loadMCPProfile(app.Paths, args[0])
			if err != nil {
				return err
			}
			serverName, err := validateMCPServerName(args[1])
			if err != nil {
				return err
			}
			env, err := parseMCPEnv(addOpts.env)
			if err != nil {
				return err
			}
			if strings.TrimSpace(addOpts.command) == "" {
				return fmt.Errorf("command must not be empty")
			}

			if prof.MCP == nil {
				prof.MCP = map[string]profile.MCPServer{}
			}
			readOnly := addOpts.readOnly || prof.SafetyLevel == profile.ReadOnly
			prof.MCP[serverName] = profile.MCPServer{
				Type:     strings.TrimSpace(addOpts.serverType),
				Command:  addOpts.command,
				Args:     append([]string(nil), addOpts.args...),
				Env:      env,
				ReadOnly: readOnly,
			}
			if err := profile.NewStore(app.Paths).Save(prof); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved MCP server %s\n", serverName)
			return nil
		},
	}
	cmd.Flags().StringVar(&addOpts.serverType, "type", "", "MCP server type; stdio is the default")
	cmd.Flags().StringVar(&addOpts.command, "command", "", "MCP server command")
	cmd.Flags().StringArrayVar(&addOpts.args, "arg", nil, "MCP server command argument; repeat for multiple args")
	cmd.Flags().StringArrayVar(&addOpts.env, "env", nil, "MCP server environment KEY=VALUE; repeat for multiple vars")
	cmd.Flags().BoolVar(&addOpts.readOnly, "readonly", false, "Mark the MCP server read-only")
	return cmd
}

func newMCPListCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list <profile>",
		Short: "List profile MCP servers",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			prof, err := loadMCPProfile(app.Paths, args[0])
			if err != nil {
				return err
			}
			for _, name := range sortedMCPServerNames(prof.MCP) {
				server := prof.MCP[name]
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\treadonly=%t\t%s\n", name, server.Command, server.ReadOnly, formatMCPEnvKeys(server.Env))
			}
			return nil
		},
	}
}

func newMCPRemoveCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "rm <profile> <name>",
		Aliases: []string{"remove"},
		Short:   "Remove a profile MCP server",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			prof, err := loadMCPProfile(app.Paths, args[0])
			if err != nil {
				return err
			}
			serverName, err := validateMCPServerName(args[1])
			if err != nil {
				return err
			}
			delete(prof.MCP, serverName)
			if err := profile.NewStore(app.Paths).Save(prof); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed MCP server %s\n", serverName)
			return nil
		},
	}
}

func newMCPSyncCommand(opts *rootOptions) *cobra.Command {
	var allowPlaintext bool
	cmd := &cobra.Command{
		Use:   "sync <profile>",
		Short: "Sync profile MCP servers to enabled tools",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			prof, err := loadMCPProfile(app.Paths, args[0])
			if err != nil {
				return err
			}
			renderOpts := mcp.RenderOptions{}
			if allowPlaintext {
				kc := secrets.NewKeychain()
				renderOpts = mcp.RenderOptions{
					AllowPlaintextSecrets: true,
					ResolveSecret: func(name string) (string, error) {
						return kc.Get(prof.Name, name)
					},
				}
			}
			notes, err := mcp.RenderWithOptions(app.Paths, prof, renderOpts)
			if err != nil {
				return err
			}
			for _, note := range notes {
				fmt.Fprintln(cmd.ErrOrStderr(), note)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Synced MCP servers for %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&allowPlaintext, "allow-plaintext-secrets", false, "Write resolved secrets into plaintext MCP configs for tools that require them")
	return cmd
}

func newMCPTestCommand(opts *rootOptions) *cobra.Command {
	var offline bool
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "test <profile> [server]",
		Short: "Test profile MCP server connectivity",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			prof, err := loadMCPProfile(app.Paths, args[0])
			if err != nil {
				return err
			}

			names := sortedMCPServerNames(prof.MCP)
			if len(args) == 2 {
				name, err := validateMCPServerName(args[1])
				if err != nil {
					return err
				}
				if _, ok := prof.MCP[name]; !ok {
					return fmt.Errorf("MCP server %q not found in profile %q", name, prof.Name)
				}
				names = []string{name}
			}

			kc := secrets.NewKeychain()
			failures := 0
			for _, name := range names {
				server := prof.MCP[name]
				env, err := preflightMCPTestServer(prof.Name, name, server, kc)
				if err != nil {
					failures++
					fmt.Fprintf(cmd.OutOrStdout(), "FAIL %s: %v\n", name, err)
					continue
				}
				if offline {
					fmt.Fprintf(cmd.OutOrStdout(), "OK %s: preflight passed (command and secrets resolved)\n", name)
					continue
				}

				ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
				result, err := mcp.Probe(ctx, server, env, mcp.ClientInfo{Name: "aip", Version: "dev"})
				cancel()
				if err != nil {
					failures++
					fmt.Fprintf(cmd.OutOrStdout(), "FAIL %s: %v\n", name, err)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "OK %s: initialized %s %s (protocol %s)\n", name, result.ServerName, result.ServerVersion, result.ProtocolVersion)
			}
			if failures > 0 {
				return fmt.Errorf("%d MCP server(s) failed", failures)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&offline, "offline", false, "Only validate command and secret resolution without connecting")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Second, "MCP server test timeout")
	return cmd
}

func loadMCPProfile(appPaths paths.Paths, profileName string) (profile.Profile, error) {
	prof, err := profile.NewStore(appPaths).Load(profileName)
	if err != nil {
		return profile.Profile{}, friendlyProfileError(profileName, err)
	}
	return prof, nil
}

func preflightMCPTestServer(profileName, serverName string, server profile.MCPServer, kc secrets.Provider) (map[string]string, error) {
	serverType := strings.TrimSpace(strings.ToLower(server.Type))
	if serverType != "" && serverType != "stdio" {
		return nil, fmt.Errorf("unsupported server type %q", server.Type)
	}
	if strings.TrimSpace(server.Command) == "" {
		return nil, fmt.Errorf("command must not be empty")
	}
	if !filepath.IsAbs(server.Command) && strings.ContainsAny(server.Command, `/\`) {
		return nil, fmt.Errorf("command %q is a relative path; use a command on PATH or an absolute executable path", server.Command)
	}
	if _, err := exec.LookPath(server.Command); err != nil {
		return nil, fmt.Errorf("command %q is not executable or not on PATH: %w", server.Command, err)
	}
	env := make(map[string]string, len(server.Env))
	for _, target := range sortedMCPEnvKeys(server.Env) {
		value := server.Env[target]
		secretName, ok := profile.ParseSecretRef(value)
		if !ok {
			env[target] = value
			continue
		}
		resolved, err := kc.Get(profileName, secretName)
		if err != nil {
			if errors.Is(err, secrets.ErrNotFound) {
				return nil, fmt.Errorf("server %s env %s references missing secret %s", serverName, target, secretName)
			}
			return nil, fmt.Errorf("server %s env %s secret %s: %w", serverName, target, secretName, err)
		}
		env[target] = resolved
	}
	return env, nil
}

func validateMCPServerName(name string) (string, error) {
	if !mcpServerNameRE.MatchString(name) {
		return "", fmt.Errorf("MCP server name %q must match %s", name, mcpServerNameRE.String())
	}
	return name, nil
}

func parseMCPEnv(entries []string) (map[string]string, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	env := make(map[string]string, len(entries))
	for _, entry := range entries {
		key, value, ok := strings.Cut(entry, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("env entries must use KEY=VALUE")
		}
		if !secretNameRE.MatchString(key) {
			return nil, fmt.Errorf("env key %q must use KEY=VALUE with a valid key", key)
		}
		env[key] = value
	}
	if err := mcp.ValidateEnvSecretRefs(env); err != nil {
		return nil, err
	}
	return env, nil
}

func sortedMCPServerNames(servers map[string]profile.MCPServer) []string {
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedMCPEnvKeys(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func formatMCPEnvKeys(env map[string]string) string {
	if len(env) == 0 {
		return "env=-"
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return "env=" + strings.Join(keys, ",")
}
