package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/naveedcs/aip/internal/honcho"
	"github.com/naveedcs/aip/internal/profile"
	"github.com/naveedcs/aip/internal/secrets"
	"github.com/spf13/cobra"
)

type honchoEnableOptions struct {
	workspaceID  string
	userName     string
	apiKeySecret string
	baseURL      string
}

func newHonchoCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "honcho",
		Short: "Manage per-profile Honcho memory",
	}
	cmd.AddCommand(newHonchoEnableCommand(opts))
	cmd.AddCommand(newHonchoDisableCommand(opts))
	cmd.AddCommand(newHonchoShowCommand(opts))
	return cmd
}

func newHonchoEnableCommand(opts *rootOptions) *cobra.Command {
	enableOpts := &honchoEnableOptions{}
	cmd := &cobra.Command{
		Use:   "enable <profile>",
		Short: "Enable Honcho memory for a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			store := profile.NewStore(app.Paths)
			prof, err := store.Load(args[0])
			if err != nil {
				return friendlyProfileError(args[0], err)
			}

			workspaceID := strings.TrimSpace(enableOpts.workspaceID)
			if workspaceID == "" {
				return fmt.Errorf("--workspace-id is required")
			}
			userName := strings.TrimSpace(enableOpts.userName)
			if userName == "" {
				return fmt.Errorf("--user-name is required")
			}
			apiKeySecret := strings.TrimSpace(enableOpts.apiKeySecret)
			if apiKeySecret == "" {
				return fmt.Errorf("--api-key-secret must not be empty")
			}
			if !secretNameRE.MatchString(apiKeySecret) {
				return fmt.Errorf("--api-key-secret %q must match %s", apiKeySecret, secretNameRE.String())
			}

			prof.Honcho = profile.HonchoConfig{
				Enabled:      true,
				WorkspaceID:  workspaceID,
				UserName:     userName,
				APIKeySecret: apiKeySecret,
				BaseURL:      strings.TrimSpace(enableOpts.baseURL),
			}
			server, ok := honcho.MCPServer(prof)
			if !ok {
				return fmt.Errorf("could not build Honcho MCP server")
			}
			if prof.MCP == nil {
				prof.MCP = map[string]profile.MCPServer{}
			}
			prof.MCP[honcho.MCPServerName] = server

			if err := store.Save(prof); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Enabled Honcho memory for %s\n", prof.Name)
			warnIfHonchoSecretMissing(cmd, prof.Name, apiKeySecret)
			return nil
		},
	}
	cmd.Flags().StringVar(&enableOpts.workspaceID, "workspace-id", "", "Honcho workspace ID")
	cmd.Flags().StringVar(&enableOpts.userName, "user-name", "", "Honcho user name")
	cmd.Flags().StringVar(&enableOpts.apiKeySecret, "api-key-secret", honcho.DefaultAPIKeySecret, "Keychain secret name holding the Honcho API key")
	cmd.Flags().StringVar(&enableOpts.baseURL, "base-url", "", "Honcho API base URL for SDK usage")
	return cmd
}

func newHonchoDisableCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <profile>",
		Short: "Disable Honcho memory for a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			store := profile.NewStore(app.Paths)
			prof, err := store.Load(args[0])
			if err != nil {
				return friendlyProfileError(args[0], err)
			}

			prof.Honcho = profile.HonchoConfig{}
			delete(prof.MCP, honcho.MCPServerName)
			if err := store.Save(prof); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Disabled Honcho memory for %s\n", prof.Name)
			return nil
		},
	}
}

func newHonchoShowCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "show <profile>",
		Short: "Show Honcho memory configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			prof, err := profile.NewStore(app.Paths).Load(args[0])
			if err != nil {
				return friendlyProfileError(args[0], err)
			}

			out := cmd.OutOrStdout()
			if !prof.Honcho.Enabled {
				fmt.Fprintln(out, "Honcho memory: disabled")
				return nil
			}
			fmt.Fprintln(out, "Honcho memory: enabled")
			fmt.Fprintf(out, "workspace_id: %s\n", strings.TrimSpace(prof.Honcho.WorkspaceID))
			fmt.Fprintf(out, "user_name: %s\n", strings.TrimSpace(prof.Honcho.UserName))
			fmt.Fprintf(out, "api_key_secret: %s\n", honcho.APIKeySecret(prof))
			fmt.Fprintf(out, "base_url: %s\n", honcho.BaseURL(prof))
			fmt.Fprintf(out, "mcp_server: %s (%s)\n", honcho.MCPServerName, honcho.MCPURL)
			return nil
		},
	}
}

func warnIfHonchoSecretMissing(cmd *cobra.Command, profileName, secretName string) {
	if _, err := secrets.NewKeychain().Get(profileName, secretName); errors.Is(err, secrets.ErrNotFound) {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: secret %s is not set; add it with: aip secret set %s %s\n", secretName, profileName, secretName)
	} else if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not check secret %s: %v\n", secretName, err)
	}
}
