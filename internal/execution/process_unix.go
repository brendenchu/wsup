//go:build unix

package execution

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func prepareProcess(command *exec.Cmd) error {
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return nil
}

func stopProcess(command *exec.Cmd) error {
	err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func processExitCode(state *os.ProcessState) int {
	status := state.Sys().(syscall.WaitStatus)
	if status.Signaled() {
		return -int(status.Signal())
	}
	return state.ExitCode()
}
