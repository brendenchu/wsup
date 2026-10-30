package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/brendenchu/wsup/internal/configuration"
	"github.com/brendenchu/wsup/internal/execution"
	"github.com/brendenchu/wsup/internal/registry"
	"github.com/brendenchu/wsup/internal/reporting"
	"github.com/spf13/cobra"
)

func runChecks(command *cobra.Command, args []string) error {
	projects, err := command.Flags().GetStringArray("project")
	if err != nil {
		return err
	}
	all, err := command.Flags().GetBool("all")
	if err != nil {
		return err
	}
	if all == (len(projects) != 0) {
		return fmt.Errorf("check requires --project (repeatable) or --all, but not both")
	}
	names, err := command.Flags().GetStringArray("task")
	if err != nil {
		return err
	}
	timeoutValue, err := command.Flags().GetInt("timeout")
	if err != nil {
		return err
	}
	var override *int
	if command.Flags().Changed("timeout") {
		override = &timeoutValue
	}
	showOutput, err := command.Flags().GetBool("show-output")
	if err != nil {
		return err
	}
	name, err := command.Root().Flags().GetString("root")
	if err != nil {
		return err
	}
	root, err := registry.Resolve(name)
	if err != nil {
		return err
	}
	data, err := registry.Load(root)
	if err != nil {
		return err
	}
	config, err := configuration.Load(root, override, os.LookupEnv)
	if err != nil {
		return err
	}
	selected, err := data.Select(projects, names, all)
	if err != nil {
		return err
	}
	if err := execution.Supported(); err != nil {
		return err
	}
	// Preflight the entire selection and report location before executing tools.
	if err := reporting.PrepareDirectory(root, config.Settings.ReportDir); err != nil {
		return err
	}
	results := make([]reporting.Result, 0, len(selected))
	cancelled, failed := false, false
	var operational error
	output := command.OutOrStdout()
	for _, item := range selected {
		result := reporting.Result{}
		if cancelled || !item.Check.Enabled {
			result.Status, result.Reason = "skipped", item.Check.Reason
			if cancelled {
				result.Reason = "cancelled"
			}
		} else {
			if _, err := fmt.Fprintf(output, "Running %s:%s\n", item.Project.ID, item.Name); err != nil {
				return err
			}
			timeout := effectiveTimeout(config.Settings.TimeoutSeconds, item.Check.TimeoutSeconds, override)
			run, runErr := execution.Run(command.Context(), item.Check.Command,
				filepath.Join(root, filepath.FromSlash(item.Project.Path), filepath.FromSlash(item.Cwd)), time.Duration(timeout)*time.Second,
				config.Settings.OutputLimitBytes)
			if runErr != nil {
				if run.Status != "error" || run.Error == "" {
					return runErr
				}
				// Cleanup failure evidence is still recorded before the explicit
				// operational error is surfaced. Do not serialize diagnostic text.
				operational = errors.Join(operational, runErr)
			}
			if showOutput {
				for _, text := range []string{run.Stdout, run.Stderr} {
					if err := printToolOutput(output, text); err != nil {
						return err
					}
				}
			}
			result.Status = run.Status
			result.Evidence = &reporting.Evidence{
				ExitCode: run.ExitCode, DurationSeconds: run.DurationSeconds, Error: run.Error,
				OutputBytes: run.OutputBytes, OutputTruncated: run.OutputTruncated,
			}
			if result.Status == "failed" && item.Check.Mode == "report" &&
				run.ExitCode != nil && slices.Contains(item.Check.FindingExitCodes, *run.ExitCode) {
				result.Status = "findings"
			}
			cancelled = result.Status == "cancelled"
		}
		results = append(results, result)
		failed = failed || result.Status == "failed" || result.Status == "error" || result.Status == "timeout" ||
			(result.Status == "skipped" && item.Check.Mode == "required")
		if _, err := fmt.Fprintf(output, "%s:%s: %s\n", item.Project.ID, item.Name, result.Status); err != nil {
			return err
		}
	}
	report, err := reporting.Write(root, Version, config, selected, results)
	if err != nil {
		return errors.Join(operational, err)
	}
	relative, err := filepath.Rel(root, report)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "Report: %s\n", filepath.ToSlash(relative)); err != nil {
		return err
	}
	if operational != nil {
		return operational
	}
	if cancelled {
		return statusError{code: 130}
	}
	if failed {
		return statusError{code: 1}
	}
	return nil
}

func effectiveTimeout(workspace int, check, override *int) int {
	if override != nil {
		return *override
	}
	if check != nil {
		return *check
	}
	return workspace
}

func printToolOutput(destination io.Writer, text string) error {
	if text == "" {
		return nil
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	_, err := io.WriteString(destination, text)
	return err
}
