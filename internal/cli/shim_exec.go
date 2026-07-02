package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/naveedcs/aip/internal/activation"
	"github.com/naveedcs/aip/internal/secrets"
	"github.com/naveedcs/aip/internal/shim"
	"github.com/naveedcs/aip/internal/tools"
	"github.com/spf13/cobra"
)

func newShimExecCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:                "shim-exec <tool> [args...]",
		Short:              "Execute a tool through an AIP shim",
		Hidden:             true,
		DisableFlagParsing: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("missing tool")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			toolID := tools.ID(args[0])
			tool, ok := tools.Get(toolID)
			if !ok {
				return fmt.Errorf("unsupported tool %q", toolID)
			}

			app, err := newApp(opts)
			if err != nil {
				return err
			}

			realPath, err := shim.ResolveReal(tool.Binary, app.Paths.ShimsDir)
			if err != nil {
				return err
			}

			toolArgs := args[1:]
			profileName := strings.TrimSpace(os.Getenv("AIP_PROFILE"))
			if profileName == "" {
				return activation.RunRaw(realPath, toolArgs)
			}

			plan, err := activation.Build(app.Paths, secrets.NewKeychain(), profileName, toolID, toolArgs)
			if err != nil {
				return err
			}
			plan.Command = realPath
			plan.Dir = ""
			if err := renderMCPForLaunch(cmd, opts, plan.Profile); err != nil {
				return err
			}
			if err := confirmAdminLaunch(cmd, plan); err != nil {
				return err
			}
			return activation.Run(plan)
		},
	}
	return cmd
}
