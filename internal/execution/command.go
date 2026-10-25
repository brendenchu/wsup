// Package execution supervises literal checks without a shell on supported platforms.
package execution

import (
	"fmt"
	"os/exec"
	"strings"
)

// ResolveCommand validates and copies literal arguments without substitution.
// Executable lookup is performed by the runner when launching the command.
func ResolveCommand(command []string) ([]string, error) {
	if len(command) == 0 || command[0] == "" {
		return nil, fmt.Errorf("check command requires a nonempty executable")
	}
	for _, argument := range command {
		if strings.ContainsRune(argument, 0) {
			return nil, fmt.Errorf("check arguments must not contain NUL")
		}
	}
	return append([]string(nil), command...), nil
}

// Supported rejects platforms without verified descendant supervision before
// a CLI creates report directories or runs any selected check.
func Supported() error { return prepareProcess(&exec.Cmd{}) }
