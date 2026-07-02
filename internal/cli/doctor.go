package cli

import (
	"fmt"

	"github.com/naveedcs/aip/internal/doctor"
	"github.com/spf13/cobra"
)

func newDoctorCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "doctor <profile>",
		Short:   "Check AIP configuration",
		Example: "aip doctor smoke",
		Args:    friendlyExactArgs(1, "Missing profile name.", "", "aip doctor <profile>", "aip doctor smoke"),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			report, err := doctor.CheckProfile(app.Paths, args[0])
			if err != nil {
				return friendlyProfileError(args[0], err)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "AIP Doctor")
			fmt.Fprintf(out, "Profile: %s\n", report.Profile.Name)
			fmt.Fprintf(out, "Safety: %s\n", report.Profile.SafetyLevel)
			fmt.Fprintf(out, "Auth: %s\n", report.Profile.AuthMode)
			projectState := "missing"
			if report.ProjectExists {
				projectState = "exists"
			}
			fmt.Fprintf(out, "Project: %s (%s)\n", report.Profile.ProjectDir, projectState)
			fmt.Fprintln(out, "Tools:")
			for _, status := range report.Tools {
				formatInstalled := func() string {
					if status.Installed {
						return "true path=" + status.Path
					}
					return "false"
				}
				if status.Enabled {
					fmt.Fprintf(out, "  %s enabled installed=%s config_dir=%s\n", status.Tool.ID, formatInstalled(), status.ConfigDir)
					continue
				}
				fmt.Fprintf(out, "  %s disabled installed=%s config_dir=%s\n", status.Tool.ID, formatInstalled(), status.ConfigDir)
			}
			fmt.Fprintln(out, "Secrets:")
			for _, name := range report.SecretNames {
				fmt.Fprintf(out, "  %s\n", name)
			}
			fmt.Fprintln(out, "MCP servers:")
			for _, name := range report.MCPServers {
				fmt.Fprintf(out, "  %s\n", name)
			}
			if report.Honcho.Enabled {
				fmt.Fprintln(out, "Honcho memory:")
				fmt.Fprintf(out, "  workspace: %s\n", report.Honcho.WorkspaceID)
			}
			fmt.Fprintln(out, "Warnings:")
			for _, warning := range report.Warnings {
				fmt.Fprintf(out, "  %s\n", warning)
			}
			return nil
		},
	}
	return cmd
}
