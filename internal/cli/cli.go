package cli

import (
	"fmt"
	"io"

	"github.com/brendenchu/wsup/internal/catalogue"
	"github.com/spf13/cobra"
)

const Version = "0.1.0"

// Run constructs a fresh command tree so invocations do not share flag state.
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
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	return 0
}

func newCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "wsup",
		Short:         "WSUP portable local workspace tools",
		Long:          "WSUP Go migration bootstrap. Workspace operations still require the Python CLI.",
		Version:       Version,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			return command.Help()
		},
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentFlags().String("root", ".", "Explicit workspace root (defaults to current directory)")
	for _, name := range []string{
		"validate", "projects", "config", "guidance", "editor",
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
