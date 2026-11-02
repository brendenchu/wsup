package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wavykit/wsup/internal/configuration"
	"github.com/wavykit/wsup/internal/projectview"
	"github.com/wavykit/wsup/internal/registry"
)

type projectEnvelope struct {
	Version int                 `json:"version"`
	Command string              `json:"command"`
	OK      bool                `json:"ok"`
	Result  *projectview.Result `json:"result"`
	Errors  []string            `json:"errors"`
}

func projectsCommand() *cobra.Command {
	command := &cobra.Command{
		Use: "projects [ID]", Short: "List registered projects, or inspect one project's configured metadata",
		Args: func(command *cobra.Command, args []string) error {
			if err := cobra.MaximumNArgs(1)(command, args); err != nil {
				return err
			}
			if len(args) == 0 && command.Flags().Changed("json") {
				return fmt.Errorf("--json requires a project ID")
			}
			return nil
		},
		RunE: inspectProject,
	}
	command.Flags().Bool("json", false, "With ID, print a version-1 project result or error envelope")
	return command
}

func inspectProject(command *cobra.Command, args []string) error {
	if len(args) == 0 {
		return inspectWorkspace(command, args)
	}
	root, err := command.Root().Flags().GetString("root")
	if err != nil {
		return err
	}
	asJSON, err := command.Flags().GetBool("json")
	if err != nil {
		return err
	}
	result, inspectErr := projectview.Inspect(root, args[0])
	if inspectErr == nil {
		inspectErr = projectConfiguration(root)
	}
	if !asJSON {
		if inspectErr != nil {
			return inspectErr
		}
		return writeProject(command.OutOrStdout(), result)
	}
	envelope := projectEnvelope{Version: 1, Command: "projects", OK: inspectErr == nil, Errors: []string{}}
	if inspectErr != nil {
		envelope.Errors = append(envelope.Errors, inspectErr.Error())
	} else {
		envelope.Result = &result
	}
	if err := writeJSON(command.OutOrStdout(), envelope); err != nil {
		return err
	}
	if inspectErr != nil {
		return statusError{code: 2}
	}
	return nil
}

func projectConfiguration(root string) error {
	local, err := registry.RelativeTarget(root, "wsup.local.json")
	if err != nil {
		return err
	}
	if _, err := os.Lstat(local); err != nil && !os.IsNotExist(err) {
		return err
	} else if err == nil {
		info, err := os.Stat(local)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("wsup.local.json must be a regular file")
		}
	}
	_, err = configuration.Load(root, nil, os.LookupEnv)
	return err
}

func writeProject(output io.Writer, result projectview.Result) error {
	var text strings.Builder
	project := result.Project
	fmt.Fprintf(&text, "Project: %s\nWorkspace: %s\nLocation: %s\nOwner: %s\nPurpose: %s\nKind: %s\nRepository: %s\n",
		project.ID, result.Workspace.Name, project.Path, project.Owner, project.Purpose, project.Kind, project.Repository)
	list := func(label string, values []string) {
		fmt.Fprintf(&text, "%s:", label)
		if len(values) == 0 {
			text.WriteString(" (none)")
		}
		text.WriteByte('\n')
		for _, value := range values {
			fmt.Fprintf(&text, "  %s\n", value)
		}
	}
	list("Direct dependencies", result.Dependencies)
	list("Direct dependents", result.Dependents)
	text.WriteString("Configured tasks (not executed; prerequisites not checked):")
	if len(result.Tasks) == 0 {
		text.WriteString(" (none)")
	}
	text.WriteByte('\n')
	for _, task := range result.Tasks {
		argv, _ := json.Marshal(task.Command)
		fmt.Fprintf(&text, "  %s: %s\n    cwd: %s\n    run: wsup run %s %s\n", task.Name, argv, task.Cwd, project.ID, task.Name)
		if policy := task.CheckPolicy; policy != nil {
			timeout := "workspace policy"
			if policy.TimeoutSeconds != nil {
				timeout = fmt.Sprintf("%ds", *policy.TimeoutSeconds)
			}
			fmt.Fprintf(&text, "    check: mode=%s enabled=%t timeout=%s finding_exit_codes=%v reason=%q\n    check command: wsup check --project %s --task %s\n",
				policy.Mode, policy.Enabled, timeout, policy.FindingExitCodes, policy.Reason, project.ID, task.Name)
		}
	}
	list("Instructions (ancestor first)", result.Instructions)
	skills := []string{}
	for _, skill := range result.Skills {
		skills = append(skills, fmt.Sprintf("%s (owner: %s, path: %s)", skill.Name, skill.Owner, skill.Path))
	}
	list("Skills", skills)
	text.WriteString("Paths are workspace-relative; task cwd is project-relative (empty means project root).\nUse the selected workspace root for run/check commands (--root PATH before the command).\nPresence does not prove an AI client loads these instructions or skills. Nothing was executed.\n")
	_, err := io.WriteString(output, text.String())
	return err
}
