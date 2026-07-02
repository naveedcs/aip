package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/naveedcs/aip/internal/profile"
	"github.com/naveedcs/aip/internal/prompt"
	"github.com/naveedcs/aip/internal/secrets"
	"github.com/spf13/cobra"
)

var secretNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func newSecretCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "secret", Short: "Manage secrets for AIP profiles"}
	cmd.AddCommand(newSecretSetCommand(opts))
	cmd.AddCommand(newSecretListCommand(opts))
	cmd.AddCommand(newSecretRmCommand(opts))
	return cmd
}

func newSecretSetCommand(opts *rootOptions) *cobra.Command {
	var fromStdin bool
	cmd := &cobra.Command{
		Use:   "set <profile> <NAME>",
		Short: "Set a secret value in the keychain",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			store := profile.NewStore(app.Paths)
			prof, err := store.Load(args[0])
			if err != nil {
				return friendlyProfileError(args[0], err)
			}

			name, err := validateSecretName(args[1])
			if err != nil {
				return err
			}
			value, err := readSecretValue(cmd, fromStdin)
			if err != nil {
				return err
			}
			if err := secrets.NewKeychain().Set(prof.Name, name, value); err != nil {
				return err
			}

			if !containsString(prof.Secrets.Keys, name) {
				prof.Secrets.Keys = append(prof.Secrets.Keys, name)
			}
			sort.Strings(prof.Secrets.Keys)
			prof.Secrets.Provider = "keychain"
			if err := store.Save(prof); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Saved secret %s\n", name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "Read the secret value from stdin instead of prompting")
	return cmd
}

func newSecretListCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list <profile>",
		Short: "List profile secret names",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			prof, err := profile.NewStore(app.Paths).Load(args[0])
			if err != nil {
				return friendlyProfileError(args[0], err)
			}

			names := append([]string(nil), prof.Secrets.Keys...)
			sort.Strings(names)
			for _, name := range names {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t********\n", name)
			}
			return nil
		},
	}
}

func newSecretRmCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "rm <profile> <NAME>",
		Short: "Remove a secret from the keychain",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			store := profile.NewStore(app.Paths)
			prof, err := store.Load(args[0])
			if err != nil {
				return friendlyProfileError(args[0], err)
			}

			name, err := validateSecretName(args[1])
			if err != nil {
				return err
			}
			if err := secrets.NewKeychain().Delete(prof.Name, name); err != nil && !errors.Is(err, secrets.ErrNotFound) {
				return err
			}

			prof.Secrets.Keys = removeString(prof.Secrets.Keys, name)
			sort.Strings(prof.Secrets.Keys)
			prof.Secrets.Provider = "keychain"
			if err := store.Save(prof); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Removed secret %s\n", name)
			return nil
		},
	}
}

func readSecretValue(cmd *cobra.Command, fromStdin bool) (string, error) {
	var value string
	var err error
	if fromStdin {
		value, err = readSecretValueFromStdin(cmd.InOrStdin())
	} else {
		value, err = prompt.New(cmd.InOrStdin(), cmd.ErrOrStderr()).Password("Secret value")
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("secret value must not be empty")
	}
	return value, nil
}

func readSecretValueFromStdin(in io.Reader) (string, error) {
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func validateSecretName(secretName string) (string, error) {
	name := strings.TrimSpace(secretName)
	if name == "" {
		return "", fmt.Errorf("secret name must not be empty")
	}
	if !secretNameRE.MatchString(name) {
		return "", fmt.Errorf("secret name %q must match %s", name, secretNameRE.String())
	}
	return name, nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	out := values[:0]
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return append([]string(nil), out...)
}
