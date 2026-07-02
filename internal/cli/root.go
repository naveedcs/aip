package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/naveedcs/aip/internal/activation"
	"github.com/naveedcs/aip/internal/app"
	"github.com/naveedcs/aip/internal/paths"
	"github.com/naveedcs/aip/internal/tools"
	"github.com/spf13/cobra"
)

var Version = "0.1.0"

type rootOptions struct {
	home string
}

func newApp(opts *rootOptions) (app.App, error) {
	root, err := paths.ResolveRoot(opts.home)
	if err != nil {
		return app.App{}, err
	}
	return app.New(root), nil
}

func NewRootCommand(out io.Writer, errOut io.Writer) *cobra.Command {
	opts := &rootOptions{}

	cmd := &cobra.Command{
		Use:           "aip",
		Short:         "AI Profile Manager for CLI agents",
		Long:          "AIP isolates AI CLI accounts, credentials, MCP servers, instructions, and project folders by profile.",
		Example:       "  aip init\n  aip profile create --name smoke --project-dir . --safety read-only --tools codex --yes\n  aip doctor smoke\n  aip dry-run smoke codex\n  aip login smoke codex --dry-run",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 2 {
				toolID := tools.ID(args[1])
				if _, ok := tools.Get(toolID); ok {
					run := newRunCommand(opts)
					run.SetOut(cmd.OutOrStdout())
					run.SetErr(cmd.ErrOrStderr())
					run.SetIn(cmd.InOrStdin())
					run.SetArgs(args)
					return run.Execute()
				}
				return fmt.Errorf("unsupported tool %q", toolID)
			}
			return cmd.Help()
		},
	}

	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.PersistentFlags().StringVar(&opts.home, "home", "", "AIP home directory for commands that access profile storage")

	cmd.AddCommand(newInitCommand(opts))
	cmd.AddCommand(newProfileCommand(opts))
	cmd.AddCommand(newProjectCommand(opts))
	cmd.AddCommand(newSecretCommand(opts))
	cmd.AddCommand(newToolsCommand(opts))
	cmd.AddCommand(newRunCommand(opts))
	cmd.AddCommand(newLoginCommand(opts))
	cmd.AddCommand(newDoctorCommand(opts))
	cmd.AddCommand(newDryRunCommand(opts))
	cmd.AddCommand(newMCPCommand(opts))
	cmd.AddCommand(newHonchoCommand(opts))
	cmd.AddCommand(newShellInitCommand(opts))
	cmd.AddCommand(newUseCommand(opts))
	cmd.AddCommand(newDeactivateCommand())
	cmd.AddCommand(newShimExecCommand(opts))
	applyFriendlyFlagErrors(cmd)

	return cmd
}

func Execute() {
	cmd := NewRootCommand(os.Stdout, os.Stderr)
	if err := cmd.Execute(); err != nil {
		var exitErr *activation.ExitCodeError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
