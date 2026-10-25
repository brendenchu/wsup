// Package execution supervises literal checks without a shell on supported platforms.
package execution

import (
	"fmt"
	"os/exec"
	"strings"
)

// ResolveCommand substitutes exact {python} arguments with python3 from PATH.
// Other arguments, including empty strings and shell-looking text, remain literal.
// A missing interpreter is returned to the runner as a launch failure.
func ResolveCommand(command []string) ([]string, error) {
	return resolveCommand(command, exec.LookPath)
}

// Supported rejects platforms without verified descendant supervision before
// a CLI creates report directories or runs any selected check.
func Supported() error { return prepareProcess(&exec.Cmd{}) }

func resolveCommand(command []string, lookup func(string) (string, error)) ([]string, error) {
	if len(command) == 0 || command[0] == "" {
		return nil, fmt.Errorf("check command requires a nonempty executable")
	}
	for _, argument := range command {
		if strings.ContainsRune(argument, 0) {
			return nil, fmt.Errorf("check arguments must not contain NUL")
		}
	}
	resolved := append([]string(nil), command...)
	python := ""
	for index, argument := range resolved {
		if argument != "{python}" {
			continue
		}
		if python == "" {
			var err error
			python, err = lookup("python3")
			if err != nil {
				return nil, fmt.Errorf("resolve {python} through python3 on PATH: %w", err)
			}
		}
		resolved[index] = python
	}
	return resolved, nil
}
