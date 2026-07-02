package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/naveedcs/aip/internal/activation"
	"github.com/naveedcs/aip/internal/mcp"
	"github.com/naveedcs/aip/internal/profile"
	"github.com/naveedcs/aip/internal/secrets"
	"github.com/naveedcs/aip/internal/tools"
	"github.com/spf13/cobra"
)

type runOptions struct {
	dryRun bool
}

func newRunCommand(opts *rootOptions) *cobra.Command {
	runOpts := &runOptions{}
	cmd := &cobra.Command{
		Use:     "run <profile> <tool> [-- <args...>]",
		Short:   "Run a command in an AIP profile",
		Example: "aip run smoke codex\naip run smoke codex -- --help",
		Args:    friendlyMinimumArgs(2, "Missing profile and tool.", "Missing tool.", "aip run <profile> <tool> [-- <args...>]", "aip run smoke codex", "aip run smoke codex -- --help"),
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := buildPlan(opts, args[0], tools.ID(args[1]), args[2:])
			if err != nil {
				return err
			}
			if runOpts.dryRun {
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
	cmd.Flags().BoolVar(&runOpts.dryRun, "dry-run", false, "Preview the command instead of running it")
	return cmd
}

func buildPlan(opts *rootOptions, profileName string, toolID tools.ID, args []string) (activation.Plan, error) {
	app, err := newApp(opts)
	if err != nil {
		return activation.Plan{}, err
	}
	plan, err := activation.Build(app.Paths, secrets.NewKeychain(), profileName, toolID, args)
	if err != nil {
		return activation.Plan{}, friendlyProfileError(profileName, err)
	}
	return plan, nil
}

func renderMCPForLaunch(cmd *cobra.Command, opts *rootOptions, prof profile.Profile) error {
	app, err := newApp(opts)
	if err != nil {
		return err
	}
	notes, err := mcp.RenderWithOptions(app.Paths, prof, mcp.RenderOptions{SkipCopilot: true})
	if err != nil {
		return err
	}
	for _, note := range notes {
		fmt.Fprintln(cmd.ErrOrStderr(), note)
	}
	return nil
}

func confirmAdminLaunch(cmd *cobra.Command, plan activation.Plan) error {
	if plan.Profile.SafetyLevel != profile.Admin {
		return nil
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Warning: You are launching an ADMIN profile.\nType %q to continue: ", plan.Profile.Name)
	reader := bufio.NewReader(cmd.InOrStdin())
	answer, _ := reader.ReadString('\n')
	if strings.TrimSpace(answer) != plan.Profile.Name {
		return fmt.Errorf("admin launch cancelled")
	}
	return nil
}
