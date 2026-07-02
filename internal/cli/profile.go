package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/naveedcs/aip/internal/fsutil"
	"github.com/naveedcs/aip/internal/paths"
	"github.com/naveedcs/aip/internal/profile"
	"github.com/naveedcs/aip/internal/prompt"
	"github.com/naveedcs/aip/internal/secrets"
	"github.com/naveedcs/aip/internal/templates"
	"github.com/naveedcs/aip/internal/tools"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type profileCreateOptions struct {
	name        string
	displayName string
	projectDir  string
	safety      string
	authMode    string
	toolCSV     string
	template    string
	yes         bool
}

type profileExportOptions struct {
	outPath string
}

type profileImportOptions struct {
	name       string
	projectDir string
}

func newProfileCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "profile", Short: "Manage AIP profiles"}
	cmd.AddCommand(newProfileCreateCommand(opts))
	cmd.AddCommand(newProfileListCommand(opts))
	cmd.AddCommand(newProfileRemoveCommand(opts))
	cmd.AddCommand(newProfileShowCommand(opts))
	cmd.AddCommand(newProfileTemplatesCommand(opts))
	cmd.AddCommand(newProfileCloneCommand(opts))
	cmd.AddCommand(newProfileExportCommand(opts))
	cmd.AddCommand(newProfileImportCommand(opts))
	return cmd
}

func newProfileCreateCommand(opts *rootOptions) *cobra.Command {
	createOpts := &profileCreateOptions{safety: string(profile.ReadOnly), authMode: string(profile.AuthSubscription), toolCSV: "codex,claude"}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a profile",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			if createOpts.name == "" {
				if createOpts.yes {
					return fmt.Errorf("missing required --name (omit --yes to use the interactive wizard)")
				}
				if err := runProfileCreateWizard(cmd, app.Paths, createOpts); err != nil {
					return err
				}
			}
			if strings.TrimSpace(createOpts.projectDir) == "" {
				return fmt.Errorf("missing required --project-dir")
			}
			p, err := buildProfileForCreate(cmd, app.Paths, createOpts)
			if err != nil {
				return err
			}

			if err := profile.NewStore(app.Paths).Save(p); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created profile %s\n", p.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&createOpts.name, "name", "", "Profile name")
	cmd.Flags().StringVar(&createOpts.displayName, "display-name", "", "Display name")
	cmd.Flags().StringVar(&createOpts.projectDir, "project-dir", "", "Project directory")
	cmd.Flags().StringVar(&createOpts.safety, "safety", string(profile.ReadOnly), "Safety level: read-only, standard, admin")
	cmd.Flags().StringVar(&createOpts.authMode, "auth-mode", string(profile.AuthSubscription), "Auth mode: subscription, api-key")
	cmd.Flags().StringVar(&createOpts.toolCSV, "tools", "codex,claude", "Comma-separated tools")
	cmd.Flags().StringVar(&createOpts.template, "template", "", "Profile template name")
	cmd.Flags().BoolVar(&createOpts.yes, "yes", false, "Use flags without prompts")
	return cmd
}

func newProfileListCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List profiles",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			profiles, err := profile.NewStore(app.Paths).List()
			if err != nil {
				return err
			}
			for _, p := range profiles {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", p.Name, p.SafetyLevel, p.ProjectDir)
			}
			return nil
		},
	}
}

func newProfileTemplatesCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "templates",
		Short: "List profile templates",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			infos, err := templates.List(app.Paths)
			if err != nil {
				return err
			}
			for _, info := range infos {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", info.Name, info.Source, info.Description)
			}
			return nil
		},
	}
}

func newProfileCloneCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "clone <source> <dest>",
		Short: "Clone a profile",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			sourceName := args[0]
			destName := args[1]
			if err := profile.ValidateName(destName); err != nil {
				return err
			}
			if err := ensureProfileDoesNotExist(app.Paths.ProfileDir(destName), destName); err != nil {
				return err
			}

			store := profile.NewStore(app.Paths)
			source, err := store.Load(sourceName)
			if err != nil {
				return friendlyProfileError(sourceName, err)
			}

			cloned := source
			cloned.Name = destName
			if cloned.DisplayName == "" || cloned.DisplayName == source.Name {
				cloned.DisplayName = destName
			}
			cloned.Instructions = ""
			if err := store.Save(cloned); err != nil {
				return err
			}
			if err := copyInstructionsIfPresent(app.Paths.InstructionsFile(sourceName), app.Paths.InstructionsFile(destName)); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Cloned %s to %s\nReminder: re-add secrets for %s.\n", sourceName, destName, destName)
			return nil
		},
	}
}

func newProfileExportCommand(opts *rootOptions) *cobra.Command {
	exportOpts := &profileExportOptions{}
	cmd := &cobra.Command{
		Use:   "export <name>",
		Short: "Export a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			p, err := profile.NewStore(app.Paths).Load(args[0])
			if err != nil {
				return friendlyProfileError(args[0], err)
			}
			p = portableProfile(p)
			if err := validatePortableMCPEnv(p); err != nil {
				return err
			}
			data, err := yaml.Marshal(p)
			if err != nil {
				return err
			}
			if exportOpts.outPath != "" {
				return fsutil.WriteFileAtomic(exportOpts.outPath, data, 0o644)
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	}
	cmd.Flags().StringVarP(&exportOpts.outPath, "out", "o", "", "Output file")
	return cmd
}

func newProfileImportCommand(opts *rootOptions) *cobra.Command {
	importOpts := &profileImportOptions{}
	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			var p profile.Profile
			if err := yaml.Unmarshal(data, &p); err != nil {
				return err
			}
			if importOpts.name != "" {
				p.Name = importOpts.name
			}
			if importOpts.projectDir != "" {
				p.ProjectDir = importOpts.projectDir
			}
			if err := profile.ValidateName(p.Name); err != nil {
				return err
			}
			if strings.TrimSpace(p.ProjectDir) == "" {
				return fmt.Errorf("project_dir is required")
			}
			if err := ensureProfileDoesNotExist(app.Paths.ProfileDir(p.Name), p.Name); err != nil {
				return err
			}
			p = portableProfile(p)
			if err := validatePortableMCPEnv(p); err != nil {
				return err
			}
			if err := profile.NewStore(app.Paths).Save(p); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Imported profile %s\nReminder: re-add secrets for %s.\n", p.Name, p.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&importOpts.name, "name", "", "Imported profile name")
	cmd.Flags().StringVar(&importOpts.projectDir, "project-dir", "", "Project directory")
	return cmd
}

func newProfileRemoveCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "rm <name>",
		Aliases: []string{"remove"},
		Short:   "Remove a profile",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			prof, err := profile.NewStore(app.Paths).Load(args[0])
			if err != nil {
				return friendlyProfileError(args[0], err)
			}

			keychain := secrets.NewKeychain()
			for _, name := range prof.Secrets.Keys {
				if err := keychain.Delete(prof.Name, name); err != nil && !errors.Is(err, secrets.ErrNotFound) {
					return err
				}
			}
			if err := os.RemoveAll(app.Paths.ProfileDir(prof.Name)); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Removed profile %s\n", prof.Name)
			return nil
		},
	}
}

func newProfileShowCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show profile details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newApp(opts)
			if err != nil {
				return err
			}
			p, err := profile.NewStore(app.Paths).Load(args[0])
			if err != nil {
				return friendlyProfileError(args[0], err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Name: %s\nDisplay: %s\nProject: %s\nSafety: %s\nAuth: %s\n", p.Name, p.DisplayName, p.ProjectDir, p.SafetyLevel, p.AuthMode)
			return nil
		},
	}
}

func buildProfileForCreate(cmd *cobra.Command, appPaths paths.Paths, createOpts *profileCreateOptions) (profile.Profile, error) {
	if createOpts.template != "" {
		tmpl, err := templates.Get(appPaths, createOpts.template)
		if err != nil {
			return profile.Profile{}, err
		}
		p := tmpl.ToProfile(createOpts.name, createOpts.displayName, createOpts.projectDir)
		if cmd.Flags().Changed("safety") {
			p.SafetyLevel = profile.SafetyLevel(createOpts.safety)
		}
		if cmd.Flags().Changed("auth-mode") {
			p.AuthMode = profile.AuthMode(createOpts.authMode)
		}
		if cmd.Flags().Changed("tools") {
			enabled, err := parseToolCSV(createOpts.toolCSV)
			if err != nil {
				return profile.Profile{}, err
			}
			p.Tools = enabled
		}
		return p, nil
	}

	enabled, err := parseToolCSV(createOpts.toolCSV)
	if err != nil {
		return profile.Profile{}, err
	}
	p := profile.Profile{
		Name:        createOpts.name,
		DisplayName: createOpts.displayName,
		ProjectDir:  createOpts.projectDir,
		SafetyLevel: profile.SafetyLevel(createOpts.safety),
		AuthMode:    profile.AuthMode(createOpts.authMode),
		Tools:       enabled,
	}
	if p.DisplayName == "" {
		p.DisplayName = p.Name
	}
	return p, nil
}

func runProfileCreateWizard(cmd *cobra.Command, appPaths paths.Paths, createOpts *profileCreateOptions) error {
	p := prompt.New(cmd.InOrStdin(), cmd.ErrOrStderr())

	name, err := p.Text("Profile name", "")
	if err != nil {
		return err
	}
	if err := profile.ValidateName(name); err != nil {
		return err
	}
	createOpts.name = name

	infos, err := templates.List(appPaths)
	if err != nil {
		return err
	}
	var selectedTemplate templates.Template
	templateChosen := strings.TrimSpace(createOpts.template) != ""
	if cmd.Flags().Changed("template") {
		if templateChosen {
			selectedTemplate, err = templates.Get(appPaths, createOpts.template)
			if err != nil {
				return err
			}
		}
	} else {
		templateOptions := make([]prompt.Option, 0, len(infos)+1)
		templateOptions = append(templateOptions, prompt.Option{Label: "none"})
		for _, info := range infos {
			templateOptions = append(templateOptions, prompt.Option{Label: info.Name, Help: info.Description})
		}
		templateChoice, err := p.Select("Start from a template?", templateOptions, 0)
		if err != nil {
			return err
		}
		templateChosen = templateChoice > 0
		if !templateChosen {
			createOpts.template = ""
		} else {
			templateName := infos[templateChoice-1].Name
			selectedTemplate, err = templates.Get(appPaths, templateName)
			if err != nil {
				return err
			}
			createOpts.template = templateName
		}
	}
	if !cmd.Flags().Changed("display-name") {
		displayName, err := p.Text("Display name", createOpts.name)
		if err != nil {
			return err
		}
		createOpts.displayName = displayName
	}

	if !cmd.Flags().Changed("project-dir") {
		projectDir, err := p.Text("Project directory", ".")
		if err != nil {
			return err
		}
		createOpts.projectDir = projectDir
	}

	if !cmd.Flags().Changed("safety") {
		safetyOptions := []prompt.Option{
			{Label: string(profile.ReadOnly)},
			{Label: string(profile.Standard)},
			{Label: string(profile.Admin)},
		}
		defaultSafety := profile.ReadOnly
		if templateChosen {
			defaultSafety = selectedTemplate.SafetyLevel
		}
		safetyChoice, err := p.Select("Safety level", safetyOptions, safetyIndex(defaultSafety))
		if err != nil {
			return err
		}
		if err := cmd.Flags().Set("safety", safetyOptions[safetyChoice].Label); err != nil {
			return err
		}
	}

	if !cmd.Flags().Changed("tools") {
		allTools := tools.All()
		toolOptions := make([]prompt.Option, 0, len(allTools))
		for _, tool := range allTools {
			toolOptions = append(toolOptions, prompt.Option{Label: string(tool.ID), Help: tool.DisplayName})
		}
		selectedTools, err := p.MultiSelect("Which AI tools?", toolOptions, toolDefaultsFor(allTools, templateChosen, selectedTemplate.Tools))
		if err != nil {
			return err
		}
		if len(selectedTools) == 0 {
			return fmt.Errorf("select at least one AI tool")
		}
		toolIDs := make([]string, 0, len(selectedTools))
		for _, index := range selectedTools {
			toolIDs = append(toolIDs, string(allTools[index].ID))
		}
		return cmd.Flags().Set("tools", strings.Join(toolIDs, ","))
	}
	return nil
}

func safetyIndex(level profile.SafetyLevel) int {
	switch level {
	case profile.Standard:
		return 1
	case profile.Admin:
		return 2
	default:
		return 0
	}
}

func toolDefaultsFor(allTools []tools.Tool, templateChosen bool, templateTools []tools.ID) []bool {
	defaults := make([]bool, len(allTools))
	enabled := map[tools.ID]bool{}
	if templateChosen {
		for _, id := range templateTools {
			enabled[id] = true
		}
	} else {
		enabled[tools.Codex] = true
		enabled[tools.Claude] = true
	}
	for i, tool := range allTools {
		defaults[i] = enabled[tool.ID]
	}
	return defaults
}

func ensureProfileDoesNotExist(profileDir string, name string) error {
	if _, err := os.Stat(profileDir); err == nil {
		return fmt.Errorf("profile %q already exists", name)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func copyInstructionsIfPresent(sourcePath string, destPath string) error {
	data, err := os.ReadFile(sourcePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(destPath, data, 0o600)
}

func portableProfile(p profile.Profile) profile.Profile {
	p.Instructions = ""
	for id, cfg := range p.Tools {
		cfg.Home = ""
		cfg.Config = ""
		p.Tools[id] = cfg
	}
	return p
}

func validatePortableMCPEnv(p profile.Profile) error {
	for serverName, server := range p.MCP {
		for key, value := range server.Env {
			if value == "" {
				continue
			}
			if _, ok := profile.ParseSecretRef(value); ok {
				continue
			}
			return fmt.Errorf("MCP server %q env %q must use ${secret:NAME} for non-empty values", serverName, key)
		}
	}
	return nil
}

func parseToolCSV(csv string) (map[tools.ID]profile.ToolConfig, error) {
	if strings.TrimSpace(csv) == "" {
		return nil, fmt.Errorf("tools list must not be empty")
	}
	out := map[tools.ID]profile.ToolConfig{}
	for _, raw := range strings.Split(csv, ",") {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, fmt.Errorf("tools list contains an empty entry")
		}
		toolID := tools.ID(id)
		if _, ok := tools.Get(toolID); !ok {
			return nil, fmt.Errorf("unsupported tool %q", toolID)
		}
		out[toolID] = profile.ToolConfig{Enabled: true}
	}
	return out, nil
}
