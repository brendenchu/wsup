package execution

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

// Result contains transient tool output. Report callers must explicitly copy
// sanitized metrics, never serialize this value into persistent evidence.
type Result struct {
	Status          string
	ExitCode        *int
	Error           string
	DurationSeconds float64
	Stdout          string
	Stderr          string
	OutputBytes     int64
	OutputTruncated bool
}

type capture struct {
	data  []byte
	total int64
	limit int
}

func (buffer *capture) Write(body []byte) (int, error) {
	buffer.total += int64(len(body))
	remaining := buffer.limit - len(buffer.data)
	if remaining > len(body) {
		remaining = len(body)
	}
	buffer.data = append(buffer.data, body[:remaining]...)
	return len(body), nil
}

const pipeGrace = 200 * time.Millisecond

// Run executes literal arguments with null stdin and per-stream bounded capture.
// Context cancellation and timeout kill the owned process group on Unix.
// Invalid inputs/unsupported platforms and supervision failures return errors;
// ordinary launch failures are explicit error results with sanitized errno codes.
func Run(ctx context.Context, command []string, cwd string, timeout time.Duration, outputLimit int) (result Result, err error) {
	start := time.Now()
	defer func() {
		result.DurationSeconds = math.Round(time.Since(start).Seconds()*1e6) / 1e6
	}()
	if timeout <= 0 || outputLimit < 1 || outputLimit > 10485760 {
		return Result{}, fmt.Errorf("execution requires a positive timeout and output limit 1..10485760")
	}
	if err := ctx.Err(); err != nil {
		return Result{Status: "cancelled"}, nil
	}
	resolved, err := ResolveCommand(command)
	if err != nil {
		return launchFailure(err)
	}
	process := exec.Command(resolved[0], resolved[1:]...)
	process.Dir = cwd
	if err := prepareProcess(process); err != nil {
		return Result{}, err
	}
	stdout, outWriter, err := os.Pipe()
	if err != nil {
		return Result{}, fmt.Errorf("create stdout pipe: %w", err)
	}
	defer stdout.Close()
	defer outWriter.Close()
	stderr, errWriter, err := os.Pipe()
	if err != nil {
		return Result{}, fmt.Errorf("create stderr pipe: %w", err)
	}
	defer stderr.Close()
	defer errWriter.Close()
	process.Stdout, process.Stderr = outWriter, errWriter
	if err := process.Start(); err != nil {
		return launchFailure(err)
	}
	// Only the child owns write ends now, so EOF reflects all inherited writers.
	closeErr := errors.Join(outWriter.Close(), errWriter.Close())
	buffers := [2]capture{{limit: outputLimit}, {limit: outputLimit}}
	drained := make(chan error, 1)
	outDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(&buffers[0], stdout)
		outDone <- err
	}()
	go func() {
		_, err := io.Copy(&buffers[1], stderr)
		drained <- errors.Join(<-outDone, err)
	}()
	waited := make(chan error, 1)
	go func() { waited <- process.Wait() }()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	result.Status = "passed"
	var waitErr, stopErr error
	select {
	case waitErr = <-waited:
	case <-ctx.Done():
		result.Status = "cancelled"
		stopErr = stopProcess(process)
		waitErr = <-waited
	case <-timer.C:
		result.Status = "timeout"
		stopErr = stopProcess(process)
		waitErr = <-waited
	}
	if process.ProcessState != nil {
		code := processExitCode(process.ProcessState)
		result.ExitCode = &code
		if result.Status == "passed" && code != 0 {
			result.Status = "failed"
		}
	}
	var exitErr *exec.ExitError
	if waitErr != nil && !errors.As(waitErr, &exitErr) {
		stopErr = errors.Join(stopErr, fmt.Errorf("wait for check: %w", waitErr))
	}
	var drainErr error
	grace := time.NewTimer(pipeGrace)
	select {
	case drainErr = <-drained:
		grace.Stop()
	case <-grace.C:
		// The immediate process may be gone while descendants retain the pipes.
		stopErr = errors.Join(stopErr, stopProcess(process))
		grace.Reset(pipeGrace)
		select {
		case drainErr = <-drained:
			grace.Stop()
		case <-grace.C:
			stopErr = errors.Join(stopErr, fmt.Errorf("check descendants retained output pipes after group cleanup"))
			_ = stdout.Close()
			_ = stderr.Close()
			drainErr = <-drained
		}
	}
	result.Stdout, result.Stderr = decodeOutput(buffers[0].data), decodeOutput(buffers[1].data)
	result.OutputBytes = buffers[0].total + buffers[1].total
	result.OutputTruncated = buffers[0].total > int64(outputLimit) || buffers[1].total > int64(outputLimit)
	if err := errors.Join(closeErr, stopErr, drainErr); err != nil {
		result.Status, result.Error = "error", "pipe_cleanup_error"
		return result, fmt.Errorf("supervise check process: %w", err)
	}
	return result, nil
}

func launchFailure(err error) (Result, error) {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return Result{Status: "error", Error: fmt.Sprintf("launch_error:%d", errno)}, nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return Result{Status: "error", Error: fmt.Sprintf("launch_error:%d", syscall.ENOENT)}, nil
	}
	return Result{}, fmt.Errorf("prepare check command: %w", err)
}

// Python replaces an incomplete valid UTF-8 prefix once, but distinct invalid
// bytes separately. Retained byte limits may split a multibyte character.
func decodeOutput(body []byte) string {
	var text strings.Builder
	for len(body) > 0 {
		char, size := utf8.DecodeRune(body)
		if char == utf8.RuneError && size == 1 {
			size = invalidPrefix(body)
		}
		text.WriteRune(char)
		body = body[size:]
	}
	return text.String()
}

func invalidPrefix(body []byte) int {
	first := body[0]
	width := 0
	switch {
	case first >= 0xc2 && first <= 0xdf:
		width = 2
	case first >= 0xe0 && first <= 0xef:
		width = 3
	case first >= 0xf0 && first <= 0xf4:
		width = 4
	default:
		return 1
	}
	for index := 1; index < width; index++ {
		if index == len(body) {
			return index
		}
		next := body[index]
		if next < 0x80 || next > 0xbf ||
			(index == 1 && ((first == 0xe0 && next < 0xa0) || (first == 0xed && next > 0x9f) ||
				(first == 0xf0 && next < 0x90) || (first == 0xf4 && next > 0x8f))) {
			return index
		}
	}
	return 1
}
