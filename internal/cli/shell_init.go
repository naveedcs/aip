package cli

import (
	"fmt"

	"github.com/naveedcs/aip/internal/shim"
	"github.com/spf13/cobra"
)

func newShellInitCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell-init <zsh|bash>",
		Short: "Print shell integration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			snippet, err := shim.ShellInit(args[0], app.Paths)
			if err != nil {
				return err
			}
			if err := shim.Generate(app.Paths); err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), snippet)
			return err
		},
	}
	return cmd
}
