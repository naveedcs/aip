package cli

import (
	"github.com/naveedcs/aip/internal/activation"
	"github.com/naveedcs/aip/internal/tools"
	"github.com/spf13/cobra"
)

func newDryRunCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "dry-run <profile> <tool>",
		Short:   "Preview AIP actions",
		Example: "aip dry-run smoke codex",
		Args:    friendlyExactArgs(2, "Missing profile and tool.", "Missing tool.", "aip dry-run <profile> <tool>", "aip dry-run smoke codex"),
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := buildPlan(opts, args[0], tools.ID(args[1]), nil)
			if err != nil {
				return err
			}
			activation.FormatPlan(cmd.OutOrStdout(), plan)
			return nil
		},
	}
	return cmd
}
