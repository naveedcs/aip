package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/naveedcs/aip/internal/profile"
	projectconfig "github.com/naveedcs/aip/internal/project"
	"github.com/spf13/cobra"
)

type projectInitOptions struct {
	profileName string
	projectType string
}

func newProjectCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "project", Short: "Manage project-local AIP config"}
	cmd.AddCommand(newProjectInitCommand(opts))
	cmd.AddCommand(newProjectShowCommand())
	return cmd
}

func newProjectInitCommand(opts *rootOptions) *cobra.Command {
	initOpts := &projectInitOptions{}
	cmd := &cobra.Command{
		Use:   "init [dir]",
		Short: "Write project-local AIP config",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}

			cfg := projectconfig.Config{ProjectType: initOpts.projectType}
			if initOpts.profileName != "" {
				app, err := newApp(opts)
				if err != nil {
					return err
				}
				prof, err := profile.NewStore(app.Paths).Load(initOpts.profileName)
				if err != nil {
					return friendlyProfileError(initOpts.profileName, err)
				}
				cfg.RecommendedProfile = prof.Name
				cfg.RecommendedMCP = sortedProjectMCPNames(prof.MCP)
			}

			if err := projectconfig.Save(dir, cfg); err != nil {
				return err
			}
			changed, err := projectconfig.EnsureGitignore(dir)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", projectconfig.ConfigPath(dir))
			if changed {
				fmt.Fprintln(cmd.OutOrStdout(), "Updated .gitignore with AIP secret patterns")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&initOpts.profileName, "profile", "", "Recommended AIP profile")
	cmd.Flags().StringVar(&initOpts.projectType, "type", "", "Project type")
	return cmd
}

func newProjectShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show [dir]",
		Short: "Show project-local AIP config",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			cfg, err := projectconfig.Load(dir)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Recommended profile: %s\n", cfg.RecommendedProfile)
			fmt.Fprintf(cmd.OutOrStdout(), "Project type: %s\n", cfg.ProjectType)
			fmt.Fprintf(cmd.OutOrStdout(), "Recommended MCP: %s\n", strings.Join(cfg.RecommendedMCP, ", "))
			return nil
		},
	}
}

func sortedProjectMCPNames(servers map[string]profile.MCPServer) []string {
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
