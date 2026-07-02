package cli

import (
	"fmt"

	"github.com/naveedcs/aip/internal/activation"
	"github.com/naveedcs/aip/internal/tools"
	"github.com/spf13/cobra"
)

type loginOptions struct {
	dryRun bool
}

func newLoginCommand(opts *rootOptions) *cobra.Command {
	loginOpts := &loginOptions{}
	cmd := &cobra.Command{
		Use:     "login <profile> <tool>",
		Short:   "Manage login sessions",
		Example: "aip login smoke codex\naip login smoke codex --dry-run",
		Args:    friendlyExactArgs(2, "Missing profile and tool.", "Missing tool.", "aip login <profile> <tool>", "aip login smoke codex", "aip login smoke codex --dry-run"),
		RunE: func(cmd *cobra.Command, args []string) error {
			toolID := tools.ID(args[1])
			tool, ok := tools.Get(toolID)
			if !ok {
				return fmt.Errorf("unsupported tool %q", toolID)
			}
			plan, err := buildPlan(opts, args[0], tool.ID, tool.LoginArgs)
			if err != nil {
				return err
			}
			if loginOpts.dryRun {
				activation.FormatPlan(cmd.OutOrStdout(), plan)
				return nil
			}
			if err := renderMCPForLaunch(cmd, opts, plan.Profile); err != nil {
				return err
			}
			if err := confirmAdminLaunch(cmd, plan); err != nil {
				return err
			}
			return activation.Run(plan)
		},
	}
	cmd.Flags().BoolVar(&loginOpts.dryRun, "dry-run", false, "Preview the login command instead of running it")
	return cmd
}
