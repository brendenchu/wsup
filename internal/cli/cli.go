// Package cli translates command arguments and service results into output and exit codes.
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode/utf16"

	"github.com/brendenchu/wsup/internal/catalogue"
	"github.com/brendenchu/wsup/internal/configuration"
	"github.com/brendenchu/wsup/internal/guidance"
	"github.com/brendenchu/wsup/internal/registry"
	"github.com/spf13/cobra"
)

// Version mirrors the Python product version until the native-runtime cutover.
const Version = "0.1.0"

// statusError carries findings already printed as structured output, not operational errors.
type statusError struct{ code int }

func (e statusError) Error() string { return fmt.Sprintf("exit status %d", e.code) }

// Run constructs a fresh command tree so invocations do not share flag state.
// It returns 0 on success, 1 for guidance findings, and 2 for invalid input or
// operational failures. It does not exit the process.
func Run(args []string, stdout, stderr io.Writer) int {
	if _, err := catalogue.Open(); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	command := newCommand()
	command.SetArgs(args)
	command.SetOut(stdout)
	command.SetErr(stderr)
	if err := command.Execute(); err != nil {
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
		Long:             "WSUP Go migration bootstrap. Inspection and editor commands are available; check, init, collections, and update still require Python.",
		Version:          Version,
		SilenceErrors:    true,
		SilenceUsage:     true,
		TraverseChildren: true,
		Args:             cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			return command.Help()
		},
	}
	root.SetVersionTemplate("{{.Version}}\n")
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
	for _, name := range []string{
		"check", "init", "collections", "update",
	} {
		root.AddCommand(&cobra.Command{
			Use:                name,
			Short:              "Not yet ported; use the Python CLI",
			DisableFlagParsing: true,
			RunE: func(command *cobra.Command, args []string) error {
				return fmt.Errorf("%s is not yet implemented in Go; use src/wsup.py "+
					"(source checkout) or .wsup/wsup.py (generated workspace) with Python 3.9+", command.Name())
			},
		})
	}
	return root
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

// Python's managed editor files use ensure_ascii; preserve those bytes, including
// UTF-16 surrogate pairs for non-BMP characters, rather than just parsed equality.
func writeEditorJSON(destination io.Writer, value any) error {
	var encoded bytes.Buffer
	if err := writeJSON(&encoded, value); err != nil {
		return err
	}
	var ascii bytes.Buffer
	for _, char := range encoded.String() {
		if char < 127 {
			ascii.WriteRune(char)
		} else if char <= 0xffff {
			fmt.Fprintf(&ascii, `\u%04x`, char)
		} else {
			high, low := utf16.EncodeRune(char)
			fmt.Fprintf(&ascii, `\u%04x\u%04x`, high, low)
		}
	}
	_, err := ascii.WriteTo(destination)
	return err
}

func writeJSON(destination io.Writer, value any) error {
	encoder := json.NewEncoder(destination)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}
