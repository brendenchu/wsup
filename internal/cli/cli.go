// Package cli translates command arguments and service results into output and exit codes.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/brendenchu/wsup/internal/catalogue"
	"github.com/brendenchu/wsup/internal/configuration"
	"github.com/brendenchu/wsup/internal/guidance"
	"github.com/brendenchu/wsup/internal/jsoncompat"
	"github.com/brendenchu/wsup/internal/registry"
	"github.com/brendenchu/wsup/internal/scaffolding"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Version is the product version shared by the CLI, templates, and reports.
const Version = "0.2.0"

// statusError carries findings already printed as structured output, not operational errors.
type statusError struct{ code int }

func (e statusError) Error() string { return fmt.Sprintf("exit status %d", e.code) }

// Run constructs a fresh command tree so invocations do not share flag state.
// It returns 0 on success, 1 for findings/failing checks, 2 for invalid input or
// operational failures, and 130 for a cancelled check run. It does not exit.
func Run(args []string, stdout, stderr io.Writer) int {
	return RunContext(context.Background(), args, stdout, stderr)
}

// RunContext accepts cancellation explicitly; the executable maps Ctrl-C to it.
func RunContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	command := newCommand()
	command.SetArgs(args)
	command.SetOut(stdout)
	command.SetErr(stderr)
	if err := command.ExecuteContext(ctx); err != nil {
		if status, ok := err.(statusError); ok {
			return status.code
		}
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	return 0
}

func newCommand() *cobra.Command {
	root := &cobra.Command{
		Use:              "wsup",
		Short:            "WSUP portable local workspace tools",
		Long:             "WSUP portable local workspace tools. Inspect projects and guidance, create workspace/project starters, run explicit Unix checks, manage collections, and preview or apply managed-content updates.",
		Version:          Version,
		SilenceErrors:    true,
		SilenceUsage:     true,
		TraverseChildren: true,
		Args:             cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			return fmt.Errorf("a command is required; use --help")
		},
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.SetGlobalNormalizationFunc(abbreviatedFlag)
	root.CompletionOptions.DisableDefaultCmd = true
	root.Flags().String("root", ".", "Explicit workspace root (before the command)")
	for _, name := range []string{"validate", "projects", "config"} {
		root.AddCommand(&cobra.Command{
			Use:   name,
			Short: "Inspect workspace " + name,
			Args:  cobra.NoArgs,
			RunE:  inspectWorkspace,
		})
	}
	inspect := &cobra.Command{Use: "guidance", Short: "Inspect portable instructions", Args: cobra.NoArgs, RunE: inspectWorkspace}
	inspect.Flags().String("project", "", "Inspect one registered project")
	root.AddCommand(inspect)
	editor := &cobra.Command{Use: "editor", Short: "Create an optional VS Code multi-root file", Args: cobra.NoArgs, RunE: inspectWorkspace}
	editor.Flags().String("output", "workspace.code-workspace", "New filename in the workspace root")
	root.AddCommand(editor)
	check := &cobra.Command{Use: "check", Short: "Run explicitly selected project checks", Args: cobra.NoArgs, RunE: runChecks}
	check.Flags().StringArray("project", nil, "Project ID (required; repeat to select more)")
	check.Flags().StringArray("check", nil, "Check name (repeat; defaults to all configured checks)")
	check.Flags().Int("timeout", 0, "Override timeout in seconds (1..86400)")
	check.Flags().Bool("show-output", false, "Print retained stdout/stderr to the terminal")
	root.AddCommand(check)
	init := &cobra.Command{
		Use: "init DESTINATION", Short: "Create a native workspace or project starter",
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			kind, err := command.Flags().GetString("template")
			if err != nil {
				return err
			}
			collections, err := command.Flags().GetStringArray("collection")
			if err != nil {
				return err
			}
			preset, err := command.Flags().GetString("preset")
			if err != nil {
				return err
			}
			agentNames, err := command.Flags().GetStringArray("agent")
			if err != nil {
				return err
			}
			agents, err := catalogue.ComposeAgents(agentNames)
			if err != nil {
				return err
			}
			if kind != "workspace" && kind != "project" {
				return fmt.Errorf("template must be workspace or project")
			}
			if kind == "workspace" {
				// Unknown presets and collections fail here, before any write.
				if collections, err = catalogue.ComposeCollections(preset, collections); err != nil {
					return err
				}
			} else if command.Flags().Changed("preset") {
				return fmt.Errorf("presets require a workspace template")
			}
			destination, err := scaffolding.InitialiseWith(args[0], Version, kind, collections, agents)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "Created %s\n", destination)
			return err
		},
	}
	init.Flags().String("template", "workspace", "Starter kind (workspace or project)")
	init.Flags().String("preset", catalogue.DefaultPreset, "Workspace preset ("+strings.Join(catalogue.PresetNames(), ", ")+"; "+catalogue.NoPreset+" for the base only)")
	init.Flags().StringArray("collection", nil, "Workspace collection added to the preset (repeatable)")
	init.Flags().StringArray("agent", nil, "Agent adapter ("+strings.Join(catalogue.AgentNames(), ", ")+"; default all; "+catalogue.NoPreset+" for none; repeatable)")
	root.AddCommand(init)
	update := &cobra.Command{
		Use: "update", Short: "Preview native managed-content updates; --apply writes",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			root, err := command.Root().Flags().GetString("root")
			if err != nil {
				return err
			}
			keep, err := command.Flags().GetStringArray("keep")
			if err != nil {
				return err
			}
			apply, err := command.Flags().GetBool("apply")
			if err != nil {
				return err
			}
			result, err := scaffolding.Update(root, Version, keep, apply)
			if err != nil {
				return err
			}
			if err := writeJSON(command.OutOrStdout(), result); err != nil {
				return err
			}
			if len(result.Conflicts) != 0 {
				return statusError{1}
			}
			return nil
		},
	}
	update.Flags().Bool("apply", false, "Apply a conflict-free native content update")
	update.Flags().StringArray("keep", nil, "Preserve an existing same-path customization (repeatable)")
	root.AddCommand(update)
	collections := &cobra.Command{
		Use: "collections", Short: "List or preview optional collection additions",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			root, err := command.Root().Flags().GetString("root")
			if err != nil {
				return err
			}
			names, err := command.Flags().GetStringArray("add")
			if err != nil {
				return err
			}
			apply, err := command.Flags().GetBool("apply")
			if err != nil {
				return err
			}
			result, err := scaffolding.AddCollections(root, names, apply)
			if err != nil {
				return err
			}
			if err := writeJSON(command.OutOrStdout(), result); err != nil {
				return err
			}
			if len(result.Conflicts) != 0 {
				return statusError{1}
			}
			return nil
		},
	}
	collections.Flags().StringArray("add", nil, "Collection name to add (repeatable)")
	collections.Flags().Bool("apply", false, "Apply conflict-free collection additions")
	root.AddCommand(collections)
	return root
}

// Resolve unambiguous long-option prefixes within this flag set only so root
// options cannot follow a command.
func abbreviatedFlag(flags *pflag.FlagSet, name string) pflag.NormalizedName {
	var match string
	count := 0
	exact := false
	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Name == name {
			exact = true
		}
		if strings.HasPrefix(flag.Name, name) {
			match = flag.Name
			count++
		}
	})
	if !exact && count == 1 {
		return pflag.NormalizedName(match)
	}
	return pflag.NormalizedName(name)
}

func inspectWorkspace(command *cobra.Command, args []string) error {
	name, err := command.Root().Flags().GetString("root")
	if err != nil {
		return err
	}
	root, err := registry.Resolve(name)
	if err != nil {
		return err
	}
	projectID := ""
	if command.Name() == "guidance" {
		projectID, err = command.Flags().GetString("project")
		if err != nil {
			return err
		}
		if projectID == "" {
			// Root guidance can diagnose an incomplete workspace without a valid registry.
			return inspectGuidance(command.OutOrStdout(), root)
		}
	}
	data, err := registry.Load(root)
	if err != nil {
		return err
	}
	config, err := configuration.Load(root, nil, os.LookupEnv)
	if err != nil {
		return err
	}
	switch command.Name() {
	case "validate":
		findings, err := scaffolding.CheckLinks(root)
		if err != nil {
			return err
		}
		if len(findings) != 0 {
			return fmt.Errorf("%s", strings.Join(findings, "; "))
		}
		_, err = fmt.Fprintf(command.OutOrStdout(), "Valid: %s (%d projects)\n", data.Name, len(data.Projects))
		return err
	case "projects":
		projects := make([]map[string]any, 0, len(data.Projects))
		for _, project := range data.Projects {
			projects = append(projects, project.Raw)
		}
		return writeJSON(command.OutOrStdout(), projects)
	case "guidance":
		for _, project := range data.Projects {
			if project.ID == projectID {
				target, err := registry.RelativeTarget(root, project.Path)
				if err != nil {
					return err
				}
				target, err = registry.Resolve(target)
				if err != nil {
					return err
				}
				return inspectGuidance(command.OutOrStdout(), target)
			}
		}
		return fmt.Errorf("Unknown project")
	case "editor":
		output, err := command.Flags().GetString("output")
		if err != nil {
			return err
		}
		return createEditor(command.OutOrStdout(), root, output, data)
	default:
		return writeJSON(command.OutOrStdout(), config)
	}
}

func inspectGuidance(output io.Writer, root string) error {
	result, err := guidance.Inspect(root)
	if err != nil {
		return err
	}
	// Managed adapter links are structural guidance, recorded in starter state.
	if links, err := scaffolding.CheckLinks(root); err == nil {
		result.Errors = append(result.Errors, links...)
		sort.Strings(result.Errors)
	}
	if err := writeJSON(output, result); err != nil {
		return err
	}
	if len(result.Errors) != 0 {
		return statusError{code: 1}
	}
	return nil
}

func createEditor(output io.Writer, root, name string, data registry.Registry) error {
	target, err := registry.RelativeTarget(root, name)
	if err != nil {
		return err
	}
	_, statErr := os.Lstat(target)
	if filepath.Dir(target) != root || statErr == nil {
		return fmt.Errorf("Editor output must be a new file in the workspace root")
	}
	if !os.IsNotExist(statErr) {
		return statErr
	}
	type folder struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	document := struct {
		Folders  []folder       `json:"folders"`
		Settings map[string]any `json:"settings"`
	}{Folders: []folder{{Name: "Workspace", Path: "."}}, Settings: map[string]any{}}
	for _, project := range data.Projects {
		if project.Path != "." {
			document.Folders = append(document.Folders, folder{Name: project.ID, Path: project.Path})
		}
	}
	// Exclusive creation also refuses files created after the preflight check.
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return err
	}
	writeErr := writeEditorJSON(file, document)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	_, err = fmt.Fprintf(output, "Created %s\n", target)
	return err
}

// Managed editor files use ASCII escapes; preserve those bytes, including
// UTF-16 surrogate pairs for non-BMP characters, rather than just parsed equality.
func writeEditorJSON(destination io.Writer, value any) error {
	encoded, err := jsoncompat.Marshal(value, "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	written, err := destination.Write(encoded)
	if err == nil && written != len(encoded) {
		return io.ErrShortWrite
	}
	return err
}

func writeJSON(destination io.Writer, value any) error {
	encoder := json.NewEncoder(destination)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}
