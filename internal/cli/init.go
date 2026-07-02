package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/naveedcs/aip/internal/prompt"
	"github.com/naveedcs/aip/internal/shim"
	"github.com/naveedcs/aip/internal/tools"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type initOptions struct {
	yes bool
}

func newInitCommand(opts *rootOptions) *cobra.Command {
	initOpts := &initOptions{}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize AIP storage",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			if !initOpts.yes {
				ok, err := prompt.New(cmd.InOrStdin(), cmd.ErrOrStderr()).Confirm(
					fmt.Sprintf("Initialize AIP in %s?", app.Paths.RootDir),
					true,
				)
				if err != nil {
					return err
				}
				if !ok {
					return errors.New("init cancelled")
				}
			}

			for _, dir := range []string{
				app.Paths.RootDir,
				app.Paths.ProfilesDir,
				app.Paths.ShimsDir,
				filepath.Dir(app.Paths.LibrarySkillsDir),
				app.Paths.LibrarySkillsDir,
				app.Paths.SecretsDir,
				app.Paths.TemplatesDir,
				app.Paths.LogsDir,
			} {
				if err := os.MkdirAll(dir, 0o700); err != nil {
					return err
				}
				if err := os.Chmod(dir, 0o700); err != nil {
					return err
				}
			}
			if err := shim.Generate(app.Paths); err != nil {
				return err
			}

			// config.yaml is reserved for future global settings; not yet read.
			cfg := map[string]string{
				"root_dir":       app.Paths.RootDir,
				"default_safety": "read-only",
			}
			data, err := yaml.Marshal(cfg)
			if err != nil {
				return err
			}
			if err := os.WriteFile(app.Paths.ConfigFile, data, 0o600); err != nil {
				return err
			}
			if err := os.Chmod(app.Paths.ConfigFile, 0o600); err != nil {
				return err
			}

			if !initOpts.yes {
				fmt.Fprintln(cmd.OutOrStdout(), "Detected AI CLIs:")
				for _, detection := range tools.Detect() {
					status := " "
					path := "not found"
					if detection.Installed {
						status = "x"
						path = detection.Path
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s\t%s\n", status, detection.Tool.ID, path)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "AIP initialized at %s\n", app.Paths.RootDir)
			return nil
		},
	}
	cmd.Flags().BoolVar(&initOpts.yes, "yes", false, "Use defaults without interactive prompts")
	return cmd
}
