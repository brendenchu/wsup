//go:build !unix

package execution

import (
	"fmt"
	"os"
	"os/exec"
)

func prepareProcess(command *exec.Cmd) error {
	return fmt.Errorf("native check execution is not supported on this platform: descendant cleanup is not implemented")
}

func stopProcess(command *exec.Cmd) error {
	return command.Process.Kill()
}

func terminateProcess(command *exec.Cmd) error {
	return command.Process.Kill()
}

func processExitCode(state *os.ProcessState) int { return state.ExitCode() }
