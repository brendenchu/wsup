//go:build unix

package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func helperCommand(t *testing.T, mode string, args ...string) []string {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return append([]string{binary, "-test.run=^TestExecutionHelper$", "--", mode}, args...)
}

func TestExecutionHelper(t *testing.T) {
	if os.Getenv("WSUP_EXECUTION_HELPER") != "1" {
		return
	}
	index := 0
	for index < len(os.Args) && os.Args[index] != "--" {
		index++
	}
	if index+1 >= len(os.Args) {
		os.Exit(91)
	}
	args := os.Args[index+1:]
	switch args[0] {
	case "literal":
		body, _ := json.Marshal(args[1:])
		if err := os.WriteFile("args.json", body, 0600); err != nil {
			os.Exit(92)
		}
		body, _ = os.ReadFile("/dev/stdin")
		fmt.Printf("stdin=%d", len(body))
	case "overflow":
		fmt.Fprint(os.Stdout, strings.Repeat("x", 200000))
		fmt.Fprint(os.Stderr, strings.Repeat("y", 200000))
	case "bytes":
		_, _ = os.Stdout.Write([]byte{0xf0, 0x9f, 0x9a, 0x80, 0xff, 0xff})
	case "fail":
		os.Exit(7)
	case "signal":
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
		os.Exit(93)
	case "sleep":
		time.Sleep(30 * time.Second)
	case "descendant":
		if err := os.WriteFile("descendant.pid", []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
			os.Exit(94)
		}
		for counter := 0; ; counter++ {
			if err := os.WriteFile("heartbeat", []byte(strconv.Itoa(counter)), 0600); err != nil {
				os.Exit(95)
			}
			time.Sleep(20 * time.Millisecond)
		}
	case "parent", "exited-parent", "escaped-parent":
		command := exec.Command(os.Args[0], "-test.run=^TestExecutionHelper$", "--", "descendant")
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		if args[0] == "escaped-parent" {
			command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		}
		if err := command.Start(); err != nil {
			os.Exit(96)
		}
		ready := time.Now().Add(3 * time.Second)
		for {
			if _, err := os.Stat("heartbeat"); err == nil {
				break
			}
			if time.Now().After(ready) {
				os.Exit(97)
			}
			time.Sleep(10 * time.Millisecond)
		}
		if args[0] == "parent" {
			time.Sleep(30 * time.Second)
		}
	default:
		os.Exit(98)
	}
	os.Exit(0)
}

func TestRunLiteralArgumentsCwdAndNullStdin(t *testing.T) {
	t.Setenv("WSUP_EXECUTION_HELPER", "1")
	root := t.TempDir()
	args := []string{"$(touch unsafe)", "", "space value", "*", "caf\u00e9"}
	result, err := Run(context.Background(), helperCommand(t, "literal", args...), root, 5*time.Second, 128)
	if err != nil || result.Status != "passed" || result.ExitCode == nil || *result.ExitCode != 0 ||
		result.Stdout != "stdin=0" || result.Stderr != "" || result.OutputBytes != 7 || result.OutputTruncated {
		t.Fatalf("literal run: %+v %v", result, err)
	}
	body, err := os.ReadFile(filepath.Join(root, "args.json"))
	if err != nil {
		t.Fatal(err)
	}
	var recorded []string
	if err := json.Unmarshal(body, &recorded); err != nil || !reflect.DeepEqual(recorded, args) {
		t.Fatalf("arguments changed: %q %v", recorded, err)
	}
	if _, err := os.Stat(filepath.Join(root, "unsafe")); !os.IsNotExist(err) {
		t.Fatal("shell-looking argument was executed")
	}
}

func TestLargeOutputIsBounded(t *testing.T) {
	t.Setenv("WSUP_EXECUTION_HELPER", "1")
	result, err := Run(context.Background(), helperCommand(t, "overflow"), t.TempDir(), 5*time.Second, 128)
	if err != nil || result.Status != "passed" || result.OutputBytes != 400000 ||
		result.Stdout != strings.Repeat("x", 128) || result.Stderr != strings.Repeat("y", 128) || !result.OutputTruncated {
		t.Fatalf("bounded capture: %+v %v", result, err)
	}
}

func TestFailureTimeoutAndCancellationAreDistinct(t *testing.T) {
	t.Setenv("WSUP_EXECUTION_HELPER", "1")
	for _, tc := range []struct {
		mode, status string
		timeout      time.Duration
		code         int
	}{{"fail", "failed", 5 * time.Second, 7}, {"signal", "failed", 5 * time.Second, -15},
		{"sleep", "timeout", 100 * time.Millisecond, -9}} {
		result, err := Run(context.Background(), helperCommand(t, tc.mode), t.TempDir(), tc.timeout, 128)
		if err != nil || result.Status != tc.status || result.ExitCode == nil ||
			*result.ExitCode != tc.code || result.DurationSeconds >= 3 {
			t.Fatalf("%s: %+v %v", tc.mode, result, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	root := t.TempDir()
	result, err := Run(ctx, helperCommand(t, "literal"), root, time.Second, 128)
	if err != nil || result.Status != "cancelled" || result.ExitCode != nil {
		t.Fatalf("pre-cancelled run: %+v %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(root, "args.json")); !os.IsNotExist(err) {
		t.Fatal("pre-cancelled command executed")
	}
}

func TestLaunchErrorsAndInvalidInputs(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", root)
	for _, command := range [][]string{{filepath.Join(root, "missing")}, {"{python}"}} {
		result, err := Run(context.Background(), command, root, time.Second, 128)
		if err != nil || result.Status != "error" || result.Error != "launch_error:2" ||
			result.ExitCode != nil || result.OutputBytes != 0 || result.Stdout != "" || result.Stderr != "" {
			t.Fatalf("launch error: %+v %v", result, err)
		}
	}
	file := filepath.Join(root, "not-executable")
	if err := os.WriteFile(file, []byte("not executable"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Run(context.Background(), []string{file}, root, time.Second, 128)
	if err != nil || result.Error != "launch_error:13" {
		t.Fatalf("permission failure: %+v %v", result, err)
	}
	for _, tc := range []struct {
		command []string
		timeout time.Duration
		limit   int
	}{{nil, time.Second, 128}, {[]string{""}, time.Second, 128},
		{[]string{"tool", "\x00"}, time.Second, 128}, {[]string{"tool"}, 0, 128},
		{[]string{"tool"}, time.Second, 0}, {[]string{"tool"}, time.Second, 10485761}} {
		if _, err := Run(context.Background(), tc.command, root, tc.timeout, tc.limit); err == nil {
			t.Fatalf("invalid execution accepted: %+v", tc)
		}
	}
}

func TestRetainedPipesAndCancellationStopDescendantHeartbeats(t *testing.T) {
	t.Setenv("WSUP_EXECUTION_HELPER", "1")
	for _, mode := range []string{"parent", "exited-parent", "timeout-parent"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			type outcome struct {
				result Result
				err    error
			}
			done := make(chan outcome, 1)
			helperMode := mode
			timeout := 4 * time.Second
			if mode == "timeout-parent" {
				helperMode = "parent"
				timeout = 2 * time.Second
			}
			command := helperCommand(t, helperMode)
			go func() {
				result, err := Run(ctx, command, root, timeout, 128)
				done <- outcome{result, err}
			}()
			deadline := time.Now().Add(3 * time.Second)
			var pid int
			for time.Now().Before(deadline) {
				body, _ := os.ReadFile(filepath.Join(root, "descendant.pid"))
				pid, _ = strconv.Atoi(string(body))
				if pid > 0 {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if pid == 0 {
				cancel()
				<-done
				t.Fatal("descendant did not become ready")
			}
			t.Cleanup(func() {
				if process, err := os.FindProcess(pid); err == nil {
					_ = process.Kill()
				}
			})
			if mode == "parent" {
				cancel()
			}
			var actual outcome
			select {
			case actual = <-done:
			case <-time.After(5 * time.Second):
				cancel()
				t.Fatal("runner hung with inherited pipes")
			}
			want, code := "passed", 0
			if mode == "parent" {
				want, code = "cancelled", -9
			} else if mode == "timeout-parent" {
				want, code = "timeout", -9
			}
			if actual.err != nil || actual.result.Status != want || actual.result.ExitCode == nil ||
				*actual.result.ExitCode != code || actual.result.DurationSeconds >= 5 {
				t.Fatalf("descendant supervision: %+v %v", actual.result, actual.err)
			}
			first, err := os.ReadFile(filepath.Join(root, "heartbeat"))
			if err != nil {
				t.Fatal(err)
			}
			time.Sleep(100 * time.Millisecond)
			second, err := os.ReadFile(filepath.Join(root, "heartbeat"))
			if err != nil || !bytes.Equal(first, second) {
				t.Fatalf("descendant heartbeat continued after runner: %q -> %q %v", first, second, err)
			}
		})
	}
}

func TestEscapedDescendantPipeCleanupFailureIsBoundedAndExplicit(t *testing.T) {
	t.Setenv("WSUP_EXECUTION_HELPER", "1")
	root := t.TempDir()
	defer func() {
		body, _ := os.ReadFile(filepath.Join(root, "descendant.pid"))
		if pid, err := strconv.Atoi(string(body)); err == nil && pid > 0 {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}()
	result, err := Run(context.Background(), helperCommand(t, "escaped-parent"), root, 4*time.Second, 128)
	if err == nil || result.Status != "error" || result.Error != "pipe_cleanup_error" ||
		result.DurationSeconds >= 5 {
		t.Fatalf("escaped pipe retention was hidden or hung: %+v %v", result, err)
	}
}

func TestOutputDecodingMatchesPythonReplacement(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"\xff\xff", "\ufffd\ufffd"}, {"\xf0\x9f\x9a", "\ufffd"},
		{"\xe2\x82x", "\ufffdx"}, {"\xed\xa0\x80", "\ufffd\ufffd\ufffd"},
		{"\xf4\x90\x80\x80", "\ufffd\ufffd\ufffd\ufffd"}, {"\xe0\x80", "\ufffd\ufffd"},
		{"\xef\xbf\xbd", "\ufffd"},
	} {
		if got := decodeOutput([]byte(tc.input)); got != tc.want {
			t.Fatalf("%x: %q, want %q", tc.input, got, tc.want)
		}
	}
	t.Setenv("WSUP_EXECUTION_HELPER", "1")
	result, err := Run(context.Background(), helperCommand(t, "bytes"), t.TempDir(), 5*time.Second, 3)
	if err != nil || result.Stdout != "\ufffd" || result.OutputBytes != 6 || !result.OutputTruncated {
		t.Fatalf("split Unicode retention: %+v %v", result, err)
	}
}
