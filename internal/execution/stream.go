package execution

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// stopGrace is how long a streamed task may take to exit after it is asked to
// stop before its process group is killed.
const stopGrace = 5 * time.Second

// Stream runs literal arguments in cwd connected to the given standard
// streams, for tasks a person runs directly. It returns the task's status
// (passed, failed, cancelled, or timeout) and exit code. On cancellation, or
// after timeout when it is positive, the process group is asked to stop and
// killed after a grace period. Invalid commands and launch failures are errors.
func Stream(ctx context.Context, command []string, cwd string, timeout time.Duration, stdin io.Reader, stdout, stderr io.Writer) (Result, error) {
	resolved, err := ResolveCommand(command)
	if err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{Status: "cancelled"}, nil
	}
	process := exec.Command(resolved[0], resolved[1:]...)
	process.Dir = cwd
	process.Stdin, process.Stdout, process.Stderr = stdin, stdout, stderr
	// Copying non-file streams must not outlive the task by long when a
	// descendant keeps them open.
	process.WaitDelay = pipeGrace
	if err := prepareProcess(process); err != nil {
		return Result{}, err
	}
	start := time.Now()
	if err := process.Start(); err != nil {
		return Result{}, fmt.Errorf("start %s: %w", resolved[0], err)
	}
	waited := make(chan error, 1)
	go func() { waited <- process.Wait() }()
	var deadline <-chan time.Time
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		deadline = timer.C
	}
	result := Result{Status: "passed"}
	var waitErr, stopErr error
	select {
	case waitErr = <-waited:
	case <-ctx.Done():
		result.Status = "cancelled"
		waitErr, stopErr = stopGracefully(process, waited)
	case <-deadline:
		result.Status = "timeout"
		waitErr, stopErr = stopGracefully(process, waited)
	}
	result.DurationSeconds = time.Since(start).Seconds()
	if process.ProcessState != nil {
		code := processExitCode(process.ProcessState)
		result.ExitCode = &code
		if result.Status == "passed" && code != 0 {
			result.Status = "failed"
		}
	}
	var exitErr *exec.ExitError
	if waitErr != nil && !errors.As(waitErr, &exitErr) && !errors.Is(waitErr, exec.ErrWaitDelay) {
		stopErr = errors.Join(stopErr, fmt.Errorf("wait for task: %w", waitErr))
	}
	return result, stopErr
}

// stopGracefully asks the process group to stop, then kills it if it has not
// exited within stopGrace.
func stopGracefully(process *exec.Cmd, waited <-chan error) (error, error) {
	stopErr := terminateProcess(process)
	grace := time.NewTimer(stopGrace)
	defer grace.Stop()
	select {
	case err := <-waited:
		return err, stopErr
	case <-grace.C:
		return <-waited, errors.Join(stopErr, stopProcess(process))
	}
}
