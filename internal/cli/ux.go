package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func applyFriendlyFlagErrors(cmd *cobra.Command) {
	cmd.SetFlagErrorFunc(friendlyFlagError)
	for _, child := range cmd.Commands() {
		applyFriendlyFlagErrors(child)
	}
}

func friendlyFlagError(cmd *cobra.Command, err error) error {
	const prefix = "unknown flag: "
	msg := err.Error()
	if !strings.HasPrefix(msg, prefix) {
		return err
	}

	flag := strings.TrimSpace(strings.TrimPrefix(msg, prefix))
	name := strings.TrimLeft(flag, "-")
	root := cmd.Root()
	if command := directCommand(root, name); command != nil {
		return fmt.Errorf("Unknown flag: %s\n\n`%s` is a command, not a flag.\n\nTry:\n  %s\n\nSee:\n  %s --help", flag, name, exampleUsage(command), command.CommandPath())
	}

	if command := closestDirectCommand(root, name); command != nil {
		return fmt.Errorf("Unknown flag: %s\n\nDid you mean command `%s`?\n\nTry:\n  %s\n\nSee:\n  %s --help", flag, command.Name(), exampleUsage(command), command.CommandPath())
	}

	return fmt.Errorf("Unknown flag: %s\n\nSee:\n  %s --help", flag, cmd.CommandPath())
}

func friendlyExactArgs(count int, missingAll string, missingPartial string, usage string, examples ...string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == count {
			return nil
		}
		if len(args) < count {
			message := missingAll
			if len(args) > 0 && missingPartial != "" {
				message = missingPartial
			}
			return friendlyUsageError(message, usage, examples...)
		}
		return friendlyUsageError("Too many arguments.", usage, examples...)
	}
}

func friendlyNoArgs(usage string, examples ...string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}
		return friendlyUsageError("Too many arguments.", usage, examples...)
	}
}

func friendlyMinimumArgs(count int, missingAll string, missingPartial string, usage string, examples ...string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) >= count {
			return nil
		}
		message := missingAll
		if len(args) > 0 && missingPartial != "" {
			message = missingPartial
		}
		return friendlyUsageError(message, usage, examples...)
	}
}

func friendlyUsageError(message string, usage string, examples ...string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\nUsage:\n  %s", message, usage)
	if len(examples) == 1 {
		fmt.Fprintf(&b, "\n\nExample:\n  %s", examples[0])
	}
	if len(examples) > 1 {
		b.WriteString("\n\nExamples:")
		for _, example := range examples {
			fmt.Fprintf(&b, "\n  %s", example)
		}
	}
	return fmt.Errorf("%s", b.String())
}

func friendlyProfileError(profileName string, err error) error {
	if err == nil {
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return fmt.Errorf("Profile %q does not exist.\n\nCreate it:\n  aip profile create --name %s --project-dir . --safety read-only --tools codex --yes\n\nList profiles:\n  aip profile list\n\nIf you are using a custom AIP home, pass the same --home value on every command.", profileName, profileName)
}

func directCommand(root *cobra.Command, name string) *cobra.Command {
	for _, command := range root.Commands() {
		if command.Name() == name {
			return command
		}
	}
	return nil
}

func closestDirectCommand(root *cobra.Command, name string) *cobra.Command {
	var best *cobra.Command
	bestDistance := 3
	for _, command := range root.Commands() {
		distance := levenshtein(name, command.Name())
		if distance < bestDistance {
			best = command
			bestDistance = distance
		}
	}
	return best
}

func exampleUsage(command *cobra.Command) string {
	switch command.Name() {
	case "doctor":
		return "aip doctor <profile>"
	case "login":
		return "aip login <profile> <tool>"
	case "run":
		return "aip run <profile> <tool>"
	case "dry-run":
		return "aip dry-run <profile> <tool>"
	default:
		return command.UseLine()
	}
}

func levenshtein(a string, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return len(b)
	}
	if b == "" {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i, ar := range a {
		cur[0] = i + 1
		for j, br := range b {
			cost := 1
			if ar == br {
				cost = 0
			}
			cur[j+1] = minInt(cur[j]+1, prev[j+1]+1, prev[j]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func minInt(values ...int) int {
	out := values[0]
	for _, value := range values[1:] {
		if value < out {
			out = value
		}
	}
	return out
}
