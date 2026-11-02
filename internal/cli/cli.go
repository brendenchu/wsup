// Package cli translates command arguments and service results into output and exit codes.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/wavykit/wsup/internal/catalogue"
	"github.com/wavykit/wsup/internal/configuration"
	"github.com/wavykit/wsup/internal/guidance"
	"github.com/wavykit/wsup/internal/registry"
	"github.com/wavykit/wsup/internal/scaffolding"
	"github.com/wavykit/wsup/schema"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Version is the product version shared by the CLI, templates, and reports.
const Version = "0.3.0"

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
// init asks its questions only when stdin and stdout are terminals.
func RunContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return runSession(ctx, args, stdout, stderr, terminalSession(stdout))
}

func runSession(ctx context.Context, args []string, stdout, stderr io.Writer, s session) int {
	command := newCommand(s)
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

func newCommand(s session) *cobra.Command {
	root := &cobra.Command{
		Use:              "wsup",
		Short:            "WSUP: AI-native, local-first workspaces",
		Long:             "WSUP: AI-native, local-first workspaces. Create workspace and project starters ready for coding agents, register projects in wsup.json, run their tasks and checks, add modules, and preview or apply managed-content updates. wsup never calls an AI service.",
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
	for name, short := range map[string]string{
		"validate": "Validate wsup.json and adapter links without running project code",
		"projects": "List registered projects",
		"config":   "Show effective settings and their sources",
	} {
		root.AddCommand(&cobra.Command{
			Use:   name,
			Short: short,
			Args:  cobra.NoArgs,
			RunE:  inspectWorkspace,
		})
	}
	inspect := &cobra.Command{Use: "guidance", Short: "Check AGENTS.md files, skills, markers, and local links", Args: cobra.NoArgs, RunE: inspectWorkspace}
	inspect.Flags().String("project", "", "Inspect one registered project")
	root.AddCommand(inspect)
	vscode := &cobra.Command{
		Use: "vscode", Short: "Preview regenerating <name>.code-workspace from wsup.json; --apply writes it",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			root, err := command.Root().Flags().GetString("root")
			if err != nil {
				return err
			}
			apply, err := command.Flags().GetBool("apply")
			if err != nil {
				return err
			}
			result, err := scaffolding.RefreshEditor(root, Version, apply)
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
	vscode.Flags().Bool("apply", false, "Write the regenerated file when it has no hand edits")
	root.AddCommand(vscode)
	check := &cobra.Command{Use: "check", Short: "Run the check tasks of selected projects", Args: cobra.NoArgs, RunE: runChecks}
	check.Flags().StringArray("project", nil, "Project ID (repeat to select more; or use --all)")
	check.Flags().Bool("all", false, "Select every project with check tasks")
	check.Flags().StringArray("task", nil, "Check task name (repeat; defaults to every check task)")
	check.Flags().Int("timeout", 0, "Override timeout in seconds (1..86400)")
	check.Flags().Bool("show-output", false, "Print retained stdout/stderr to the terminal")
	root.AddCommand(check)
	run := &cobra.Command{Use: "run PROJECT TASK", Short: "Run one project task in the terminal", Args: cobra.ExactArgs(2), RunE: runTask}
	run.Flags().Int("timeout", 0, "Stop the task after this many seconds (1..86400; default: the task's own, or none)")
	root.AddCommand(run)
	init := &cobra.Command{
		Use: "init DESTINATION", Short: "Create a workspace, or a project with --template project",
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
			editorNames, err := command.Flags().GetStringArray("editor")
			if err != nil {
				return err
			}
			if kind != "workspace" && kind != "project" {
				return fmt.Errorf("template must be workspace or project")
			}
			if kind == "project" && command.Flags().Changed("preset") {
				return fmt.Errorf("presets require a workspace template")
			}
			if kind == "project" && len(editorNames) != 0 && !slices.Equal(editorNames, []string{catalogue.NoPreset}) {
				return fmt.Errorf("editor adapters require a workspace template")
			}
			// Flag values fail before any question is asked.
			if _, err := catalogue.ComposeAdapters(catalogue.KindAgent, agentNames); err != nil {
				return err
			}
			if _, err := catalogue.ComposeAdapters(catalogue.KindEditor, editorNames); err != nil {
				return err
			}
			if kind == "workspace" {
				if _, err := catalogue.ComposeCollections(preset, collections); err != nil {
					return err
				}
			}
			yes, err := command.Flags().GetBool("yes")
			if err != nil {
				return err
			}
			if s.interactive && !yes {
				answers, err := askInit(s, command.OutOrStdout(), kind, collections, command.Flags().Changed)
				if err != nil {
					return err
				}
				if answers.preset != nil {
					preset = *answers.preset
				}
				if answers.agents != nil {
					agentNames = answers.agents
				}
				if answers.editors != nil {
					editorNames = answers.editors
				}
			}
			agents, err := catalogue.ComposeAdapters(catalogue.KindAgent, agentNames)
			if err != nil {
				return err
			}
			editors, err := catalogue.ComposeAdapters(catalogue.KindEditor, editorNames)
			if err != nil {
				return err
			}
			if kind == "workspace" {
				// Unknown presets and collections fail here, before any write.
				if collections, err = catalogue.ComposeCollections(preset, collections); err != nil {
					return err
				}
			} else {
				// Editor adapters are workspace-only.
				editors = nil
			}
			destination, err := scaffolding.InitialiseWith(args[0], Version, kind, catalogue.Selection{Collections: collections, Agents: agents, Editors: editors})
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
	init.Flags().StringArray("editor", nil, "Workspace editor adapter ("+strings.Join(catalogue.EditorNames(), ", ")+"; default all; "+catalogue.NoPreset+" for none; repeatable)")
	init.Flags().Bool("yes", false, "Use defaults for any question flags leave open, even in a terminal")
	init.Flags().StringArray("agent", nil, "Agent adapter ("+strings.Join(catalogue.AgentNames(), ", ")+"; default all; "+catalogue.NoPreset+" for none; repeatable)")
	root.AddCommand(init)
	update := &cobra.Command{
		Use: "update", Short: "Preview managed-content updates; --apply writes them",
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
	migrate := &cobra.Command{
		Use: "migrate", Short: "Preview converting a wsup 0.2.0 workspace or project; --apply converts it",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			root, err := command.Root().Flags().GetString("root")
			if err != nil {
				return err
			}
			apply, err := command.Flags().GetBool("apply")
			if err != nil {
				return err
			}
			options := scaffolding.MigrationOptions{}
			if options.Agents, err = command.Flags().GetStringArray("agent"); err != nil {
				return err
			}
			if options.Editors, err = command.Flags().GetStringArray("editor"); err != nil {
				return err
			}
			if options.Keep, err = command.Flags().GetStringArray("keep"); err != nil {
				return err
			}
			result, err := scaffolding.Migrate(root, Version, options, apply)
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
	migrate.Flags().Bool("apply", false, "Convert when conflict-free, backing up every replaced or removed file")
	migrate.Flags().StringArray("keep", nil, "Keep an existing owner file at a path the conversion installs (repeatable)")
	migrate.Flags().StringArray("agent", nil, "Agent adapter ("+strings.Join(catalogue.AgentNames(), ", ")+"; default all; "+catalogue.NoPreset+" for none; repeatable)")
	migrate.Flags().StringArray("editor", nil, "Workspace editor adapter ("+strings.Join(catalogue.EditorNames(), ", ")+"; default all; "+catalogue.NoPreset+" for none; repeatable)")
	root.AddCommand(migrate)
	add := &cobra.Command{
		Use: "add MODULE...", Short: "Preview adding collections or adapters; --apply installs them",
		Args: cobra.MinimumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			root, err := command.Root().Flags().GetString("root")
			if err != nil {
				return err
			}
			apply, err := command.Flags().GetBool("apply")
			if err != nil {
				return err
			}
			keep, err := command.Flags().GetStringArray("keep")
			if err != nil {
				return err
			}
			result, err := scaffolding.AddModulesKeeping(root, args, keep, apply)
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
	add.Flags().Bool("apply", false, "Apply a conflict-free addition")
	add.Flags().StringArray("keep", nil, "Keep an existing owner file at a path the addition installs (repeatable)")
	root.AddCommand(add)
	modules := &cobra.Command{
		Use: "modules [ID]", Short: "List built-in modules, or show one module's files and skills",
		Args: cobra.MaximumNArgs(1),
		RunE: listModules,
	}
	modules.Flags().String("kind", "", "Only list modules of this kind (base, collection, agent, editor, preset)")
	root.AddCommand(modules)
	root.AddCommand(&cobra.Command{
		Use: "schema", Short: "Print the JSON Schema for wsup.json", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			_, err := command.OutOrStdout().Write(schema.Workspace)
			return err
		},
	})
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
		projects := make([]registry.ProjectEntry, 0, len(data.Projects))
		for _, project := range data.Projects {
			projects = append(projects, project.Entry)
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

// moduleSummary is one row of wsup modules.
type moduleSummary struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// moduleDetail is wsup modules ID: the manifest plus the files it installs.
type moduleDetail struct {
	moduleSummary
	AppliesTo []string          `json:"applies_to"`
	Requires  []string          `json:"requires"`
	Modules   []string          `json:"modules,omitempty"`
	Skills    []string          `json:"skills"`
	Files     []string          `json:"files"`
	Ignore    []string          `json:"ignore"`
	Generate  []string          `json:"generate"`
	Links     map[string]string `json:"links,omitempty"`
}

func listModules(command *cobra.Command, args []string) error {
	kind, err := command.Flags().GetString("kind")
	if err != nil {
		return err
	}
	if kind != "" && !slices.Contains([]string{catalogue.KindBase, catalogue.KindCollection, catalogue.KindAgent, catalogue.KindEditor, catalogue.KindPreset}, kind) {
		return fmt.Errorf("unknown kind %q", kind)
	}
	summaries := []moduleSummary{}
	for _, module := range catalogue.Modules() {
		summary := moduleSummary{ID: module.ID, Kind: module.Kind, Version: module.Version, Description: module.Description}
		if len(args) == 1 && module.ID == args[0] {
			files := slices.Collect(maps.Keys(module.Files))
			files = slices.Sorted(slices.Values(append(files, slices.Collect(maps.Keys(module.Templates))...)))
			return writeJSON(command.OutOrStdout(), moduleDetail{
				moduleSummary: summary, AppliesTo: nonNil(module.AppliesTo), Requires: nonNil(module.Requires),
				Modules: module.Modules, Skills: nonNil(module.Skills), Files: files, Ignore: nonNil(module.Ignore),
				Generate: nonNil(module.Generate), Links: module.Links,
			})
		}
		if kind == "" || module.Kind == kind {
			summaries = append(summaries, summary)
		}
	}
	if len(args) == 1 {
		return fmt.Errorf("unknown module %q", args[0])
	}
	return writeJSON(command.OutOrStdout(), summaries)
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func writeJSON(destination io.Writer, value any) error {
	encoder := json.NewEncoder(destination)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}
