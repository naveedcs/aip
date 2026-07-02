package cli

import (
	"fmt"
	"os"

	"github.com/naveedcs/aip/internal/profile"
	"github.com/spf13/cobra"
)

func newUseCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "use <profile>",
		Short:   "Switch the active shell profile",
		Example: "aip use acme",
		Args:    friendlyExactArgs(1, "Missing profile name.", "", "aip use <profile>", "aip use acme"),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			prof, err := profile.NewStore(app.Paths).Load(args[0])
			if err != nil {
				return friendlyProfileError(args[0], err)
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "export AIP_PROFILE='%s'\n", prof.Name); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "[aip] using profile '%s'\n", prof.Name); err != nil {
				return err
			}
			if os.Getenv("AIP_SHELL_INIT") == "" {
				_, err = fmt.Fprintln(cmd.ErrOrStderr(), "[aip] run `eval \"$(aip shell-init zsh)\"` to enable automatic shell switching")
			}
			return err
		},
	}
}

func newDeactivateCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "deactivate",
		Short:   "Unset the active shell profile",
		Example: "aip deactivate",
		Args:    friendlyNoArgs("aip deactivate", "aip deactivate"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "unset AIP_PROFILE"); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.ErrOrStderr(), "[aip] deactivated")
			return err
		},
	}
}
