package cli

import (
	"fmt"

	"github.com/naveedcs/aip/internal/tools"
	"github.com/spf13/cobra"
)

func newToolsCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "tools", Short: "Inspect supported AI tools"}
	cmd.AddCommand(&cobra.Command{
		Use:   "detect",
		Short: "Detect installed AI CLIs",
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, detected := range tools.Detect() {
				if detected.Installed {
					fmt.Fprintf(cmd.OutOrStdout(), "yes\t%s\t%s\n", detected.Tool.ID, detected.Path)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "no\t%s\tnot found\n", detected.Tool.ID)
				}
			}
			return nil
		},
	})
	return cmd
}
