//go:build unix

package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	"github.com/brendenchu/wsup/internal/reporting"
)

func checkTool(t *testing.T, mode string, args ...string) []string {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return append([]string{binary, "-test.run=^TestCheckTool$", "--", mode}, args...)
}

func TestCheckTool(t *testing.T) {
	if os.Getenv("WSUP_CHECK_HELPER") != "1" {
		return
	}
	index := 0
	for index < len(os.Args) && os.Args[index] != "--" {
		index++
	}
	if index+2 >= len(os.Args) {
		os.Exit(90)
	}
	args := os.Args[index+1:]
	switch args[0] {
	case "record":
		body, _ := json.Marshal(args[1:])
		if err := os.WriteFile("args.json", body, 0600); err != nil {
			os.Exit(91)
		}
		fmt.Print("SYNTHETIC-SECRET-CANARY")
		fmt.Fprint(os.Stderr, "SYNTHETIC-SECRET-CANARY\n")
	case "exit":
		code, _ := strconv.Atoi(args[1])
		os.Exit(code)
	case "marker":
		if err := os.WriteFile(args[1], []byte("Executed\n"), 0600); err != nil {
			os.Exit(92)
		}
	case "sleep":
		time.Sleep(30 * time.Second)
	case "break-report":
		if err := os.Remove("../workspace.json"); err != nil {
			os.Exit(98)
		}
		if err := os.Mkdir("../workspace.json", 0700); err != nil {
			os.Exit(99)
		}
	case "descendant":
		if err := os.WriteFile("descendant.pid", []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
			os.Exit(93)
		}
		for count := 0; ; count++ {
			if err := os.WriteFile("heartbeat", []byte(strconv.Itoa(count)), 0600); err != nil {
				os.Exit(94)
			}
			time.Sleep(20 * time.Millisecond)
		}
	case "parent", "exited-parent":
		command := exec.Command(os.Args[0], "-test.run=^TestCheckTool$", "--", "descendant", "ready")
		command.Stdout, command.Stderr = os.Stdout, os.Stderr
		if err := command.Start(); err != nil {
			os.Exit(95)
		}
		deadline := time.Now().Add(3 * time.Second)
		for {
			if _, err := os.Stat("heartbeat"); err == nil {
				break
			}
			if time.Now().After(deadline) {
				os.Exit(96)
			}
			time.Sleep(10 * time.Millisecond)
		}
		if args[0] == "parent" {
			time.Sleep(30 * time.Second)
		}
	default:
		os.Exit(97)
	}
	os.Exit(0)
}

func checkFixture(t *testing.T, checks map[string]any) string {
	t.Helper()
	t.Setenv("WSUP_CHECK_HELPER", "1")
	for _, name := range []string{"WSUP_TIMEOUT_SECONDS", "WSUP_OUTPUT_LIMIT_BYTES", "WSUP_REPORT_DIR"} {
		if _, exists := os.LookupEnv(name); exists {
			t.Setenv(name, os.Getenv(name))
			if err := os.Unsetenv(name); err != nil {
				t.Fatal(err)
			}
		}
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "app"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app/README.md"), []byte("Synthetic\n"), 0600); err != nil {
		t.Fatal(err)
	}
	document := map[string]any{"version": 1, "name": "Synthetic", "projects": []any{
		map[string]any{"id": "app", "path": "app", "kind": "app", "repository": "independent",
			"purpose": "Fixture", "owner": "Test", "checks": checks},
	}}
	body, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "workspace.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

type checkReport struct {
	Version int `json:"version"`
	Results []struct {
		Project string `json:"project"`
		Check   string `json:"check"`
		Mode    string `json:"mode"`
		Status  string `json:"status"`
		Reason  string `json:"reason"`
		*reporting.Evidence
	} `json:"results"`
	Selection []struct {
		Check         string `json:"check"`
		CommandSHA256 string `json:"command_sha256"`
	} `json:"selection"`
	Inputs  map[string]string `json:"inputs"`
	Sources map[string]string `json:"config_sources"`
}

func readCheckReport(t *testing.T, root string) (checkReport, []byte) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(root, ".wsup/reports/*.json"))
	if err != nil || len(files) != 1 {
		t.Fatalf("expected one report: %v %v", files, err)
	}
	body, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	var data checkReport
	if err := json.Unmarshal(body, &data); err != nil || data.Version != 2 {
		t.Fatalf("invalid native report: %s %v", body, err)
	}
	return data, body
}

func invokeCheck(root string, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := Run(append([]string{"--root", root, "check"}, args...), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestChecksContinueLiteralArgumentsAndSanitizedReports(t *testing.T) {
	literal := []string{"$(touch unsafe)", "", "space value", "{tool}", "prefix{tool}", "SYNTHETIC-SECRET-CANARY"}
	record := checkTool(t, "record", literal...)
	root := checkFixture(t, map[string]any{
		"fail":   checkTool(t, "exit", "7"),
		"record": record,
	})
	t.Setenv("SYNTHETIC_PRIVATE_TOKEN", "SYNTHETIC-SECRET-CANARY")
	code, stdout, stderr := invokeCheck(root, "--project", "app", "--show-output")
	if code != 1 || stderr != "" || !strings.Contains(stdout, "app:fail: failed\nRunning app:record\n") ||
		strings.Count(stdout, "SYNTHETIC-SECRET-CANARY") != 2 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	body, err := os.ReadFile(filepath.Join(root, "app/args.json"))
	if err != nil {
		t.Fatal(err)
	}
	var args []string
	if err := json.Unmarshal(body, &args); err != nil || !reflect.DeepEqual(args, literal) {
		t.Fatalf("literal arguments changed: %q %v", args, err)
	}
	if _, err := os.Stat(filepath.Join(root, "app/unsafe")); !os.IsNotExist(err) {
		t.Fatal("shell-looking argument executed")
	}
	data, body := readCheckReport(t, root)
	if len(data.Results) != 2 || data.Results[0].Status != "failed" ||
		data.Results[1].Status != "passed" || data.Results[1].Evidence == nil {
		t.Fatalf("incorrect results: %s", body)
	}
	digest, err := reporting.CommandDigest(record)
	if err != nil || data.Selection[1].CommandSHA256 != digest {
		t.Fatalf("command identity mismatch: %s %v", body, err)
	}
	registry, err := os.ReadFile(filepath.Join(root, "workspace.json"))
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(registry)
	if data.Inputs["workspace.json"] != hex.EncodeToString(hash[:]) {
		t.Fatal("input hash differs")
	}
	reports, _ := filepath.Glob(filepath.Join(root, ".wsup/reports/*"))
	for _, path := range reports {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"SYNTHETIC-SECRET-CANARY", "SYNTHETIC_PRIVATE_TOKEN", `"stdout"`, `"stderr"`} {
			if bytes.Contains(body, []byte(secret)) {
				t.Fatalf("report leaked %q", secret)
			}
		}
	}
}

func TestReportModesSkipsAndLaunchErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		spec   any
		code   int
		status string
	}{
		{"findings", map[string]any{"command": checkTool(t, "exit", "1"), "mode": "report"}, 0, "findings"},
		{"unexpected exit", map[string]any{"command": checkTool(t, "exit", "2"), "mode": "report"}, 1, "failed"},
		{"custom findings", map[string]any{"command": checkTool(t, "exit", "7"), "mode": "report", "finding_exit_codes": []int{7}}, 0, "findings"},
		{"empty findings", map[string]any{"command": checkTool(t, "exit", "1"), "mode": "report", "finding_exit_codes": []int{}}, 1, "failed"},
		{"required skip", map[string]any{"command": []string{"unused"}, "enabled": false, "reason": "Unavailable fixture"}, 1, "skipped"},
		{"report skip", map[string]any{"command": []string{"unused"}, "enabled": false, "reason": "Unavailable fixture", "mode": "report"}, 0, "skipped"},
		{"launch error", map[string]any{"command": []string{"/synthetic/not-installed-tool"}, "mode": "report"}, 1, "error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := checkFixture(t, map[string]any{"test": tc.spec})
			code, stdout, stderr := invokeCheck(root, "--project", "app")
			if code != tc.code || stderr != "" || !strings.Contains(stdout, "app:test: "+tc.status) {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			data, _ := readCheckReport(t, root)
			if len(data.Results) != 1 || data.Results[0].Status != tc.status {
				t.Fatalf("incorrect result: %+v", data.Results)
			}
			if tc.status == "skipped" && (data.Results[0].Evidence != nil || data.Results[0].Reason == "") {
				t.Fatal("skip lost reason or claimed execution")
			}
			if tc.status == "error" && data.Results[0].Error != "launch_error:2" {
				t.Fatal("launch failure lost sanitized errno")
			}
		})
	}
}

func TestCheckPreflightRejectsInvalidSelectionConfigAndReportParent(t *testing.T) {
	for _, tc := range []struct {
		name       string
		args       []string
		settings   string
		parentFile bool
	}{
		{"missing projects", nil, "", false},
		{"unknown check", []string{"--project", "app", "--check", "write", "--check", "missing"}, "", false},
		{"unknown project", []string{"--project", "app", "--project", "missing"}, "", false},
		{"bad boolean", []string{"--project", "app"}, `{"timeout_seconds":false}`, false},
		{"bad report", []string{"--project", "app"}, `{"report_dir":"../outside"}`, false},
		{"unknown config", []string{"--project", "app"}, `{"unknown":1}`, false},
		{"zero timeout", []string{"--project", "app", "--timeout", "0"}, "", false},
		{"bad parent", []string{"--project", "app"}, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := checkFixture(t, map[string]any{"write": checkTool(t, "marker", "unexpected")})
			if tc.settings != "" {
				if err := os.WriteFile(filepath.Join(root, "wsup.local.json"), []byte(`{"version":1,"settings":`+tc.settings+`}`), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.parentFile {
				if err := os.WriteFile(filepath.Join(root, ".wsup"), []byte("Keep\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			code, stdout, stderr := invokeCheck(root, tc.args...)
			if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "Error: ") {
				t.Fatalf("preflight code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if _, err := os.Stat(filepath.Join(root, "app/unexpected")); !os.IsNotExist(err) {
				t.Fatal("preflight ran a command")
			}
			if !tc.parentFile {
				if _, err := os.Stat(filepath.Join(root, ".wsup")); !os.IsNotExist(err) {
					t.Fatal("invalid selection/config created reports")
				}
			}
		})
	}
}

func TestSelectionDeduplicationOrderAndTimeoutPrecedence(t *testing.T) {
	one, two := 1, 2
	for _, tc := range []struct {
		check, override *int
		want            int
	}{{nil, nil, 3}, {&two, nil, 2}, {&two, &one, 1}, {nil, &one, 1}} {
		if got := effectiveTimeout(3, tc.check, tc.override); got != tc.want {
			t.Fatalf("timeout %d, want %d", got, tc.want)
		}
	}
	root := checkFixture(t, map[string]any{
		"a": checkTool(t, "exit", "0"),
		"b": checkTool(t, "exit", "0"),
	})
	code, stdout, stderr := invokeCheck(root, "--project", "app", "--project", "app",
		"--check", "b", "--check", "b", "--check", "a")
	if code != 0 || stderr != "" || strings.Count(stdout, "Running ") != 2 ||
		!strings.Contains(stdout, "app:b: passed\nRunning app:a\n") {
		t.Fatalf("selection order: %d %q %q", code, stdout, stderr)
	}
	data, _ := readCheckReport(t, root)
	if data.Selection[0].Check != "b" || data.Selection[1].Check != "a" {
		t.Fatal("report selection order changed")
	}
}

func TestCheckTimeoutOverridesAndPrecancelledRun(t *testing.T) {
	root := checkFixture(t, map[string]any{
		"sleep": map[string]any{"command": checkTool(t, "sleep", "unused"), "timeout_seconds": 20},
	})
	t.Setenv("WSUP_TIMEOUT_SECONDS", "30")
	code, stdout, stderr := invokeCheck(root, "--project", "app", "--timeout", "1")
	if code != 1 || stderr != "" || !strings.Contains(stdout, "app:sleep: timeout") {
		t.Fatalf("timeout override: %d %q %q", code, stdout, stderr)
	}
	data, _ := readCheckReport(t, root)
	if data.Sources["timeout_seconds"] != "CLI" || data.Results[0].DurationSeconds >= 5 {
		t.Fatal("CLI timeout did not override check/environment")
	}
	root = checkFixture(t, map[string]any{
		"a": checkTool(t, "marker", "unexpected"), "b": checkTool(t, "marker", "also-unexpected"),
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, diagnostic bytes.Buffer
	if code := RunContext(ctx, []string{"--root", root, "check", "--project", "app"}, &out, &diagnostic); code != 130 || diagnostic.Len() != 0 {
		t.Fatalf("cancelled invocation: %d %s", code, &diagnostic)
	}
	data, _ = readCheckReport(t, root)
	if data.Results[0].Status != "cancelled" || data.Results[1].Status != "skipped" || data.Results[1].Reason != "cancelled" {
		t.Fatal("cancellation did not skip remaining selection")
	}
	if _, err := os.Stat(filepath.Join(root, "app/unexpected")); !os.IsNotExist(err) {
		t.Fatal("pre-cancelled command executed")
	}
}

func TestExecutableInterruptAndExitedParent(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "wsup")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/wsup")
	if body, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build standalone CLI: %s %v", body, err)
	}
	for _, mode := range []string{"parent", "exited-parent"} {
		t.Run(mode, func(t *testing.T) {
			root := checkFixture(t, map[string]any{
				"a": checkTool(t, mode, "unused"),
				"b": checkTool(t, "marker", "second"),
			})
			command := exec.Command(binary, "--root", root, "check", "--project", "app")
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			finished := false
			defer func() {
				if !finished {
					_ = command.Process.Kill()
					_ = command.Wait()
				}
				body, _ := os.ReadFile(filepath.Join(root, "app/descendant.pid"))
				if pid, err := strconv.Atoi(string(body)); err == nil && pid > 0 {
					_ = syscall.Kill(pid, syscall.SIGKILL)
				}
			}()
			deadline := time.Now().Add(4 * time.Second)
			for {
				if _, err := os.Stat(filepath.Join(root, "app/heartbeat")); err == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("descendant not ready")
				}
				time.Sleep(10 * time.Millisecond)
			}
			if mode == "parent" {
				if err := command.Process.Signal(os.Interrupt); err != nil {
					t.Fatal(err)
				}
			}

			waited := make(chan error, 1)
			go func() { waited <- command.Wait() }()
			select {
			case <-waited:
				finished = true
			case <-time.After(5 * time.Second):
				_ = command.Process.Kill()
				<-waited
				finished = true
				t.Fatal("CLI hung on process cleanup")
			}
			want := 0
			if mode == "parent" {
				want = 130
			}
			if command.ProcessState.ExitCode() != want || stderr.Len() != 0 {
				t.Fatalf("CLI exit=%d stdout=%q stderr=%q", command.ProcessState.ExitCode(), &stdout, &stderr)
			}
			data, _ := readCheckReport(t, root)
			if mode == "parent" {
				if data.Results[0].Status != "cancelled" || data.Results[1].Status != "skipped" || data.Results[1].Reason != "cancelled" {
					t.Fatal("interrupt report incorrect")
				}
				if _, err := os.Stat(filepath.Join(root, "app/second")); !os.IsNotExist(err) {
					t.Fatal("interrupt ran remaining check")
				}
			} else if data.Results[0].Status != "passed" || data.Results[1].Status != "passed" {
				t.Fatal("retained pipe cleanup changed successful statuses")
			}
			first, err := os.ReadFile(filepath.Join(root, "app/heartbeat"))
			if err != nil {
				t.Fatal(err)
			}
			time.Sleep(100 * time.Millisecond)
			second, err := os.ReadFile(filepath.Join(root, "app/heartbeat"))
			if err != nil || !bytes.Equal(first, second) {
				t.Fatal("descendant heartbeat continued after CLI exit")
			}
		})
	}
}
