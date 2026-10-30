package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRunStreamsOneTaskAndExitsWithItsCode(t *testing.T) {
	literal := []string{"$(touch unsafe)", "", "space value"}
	root := checkFixture(t, map[string]any{
		"record": checkTool(t, "record", literal...),
		"fail":   checkTool(t, "exit", "7"),
		"slow":   map[string]any{"command": checkTool(t, "sleep", "unused"), "timeout_seconds": 1},
	})
	invoke := func(args ...string) (int, string, string) {
		var stdout, stderr bytes.Buffer
		code := Run(append([]string{"--root", root, "run"}, args...), &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}
	code, stdout, stderr := invoke("app", "record")
	if code != 0 || stdout != "SYNTHETIC-SECRET-CANARY" || stderr != "SYNTHETIC-SECRET-CANARY\n" {
		t.Fatalf("record: %d %q %q", code, stdout, stderr)
	}
	var recorded []string
	if body, err := os.ReadFile(filepath.Join(root, "app/args.json")); err != nil || json.Unmarshal(body, &recorded) != nil || !reflect.DeepEqual(recorded, literal) {
		t.Fatalf("arguments were not literal: %s %v", body, err)
	}
	if code, _, _ := invoke("app", "fail"); code != 7 {
		t.Fatalf("task exit code not returned: %d", code)
	}
	if code, _, stderr := invoke("app", "slow"); code != 124 || !strings.Contains(stderr, "timed out") {
		t.Fatalf("task timeout: %d %q", code, stderr)
	}
	if code, _, stderr := invoke("app", "slow", "--timeout", "0"); code != 2 || !strings.Contains(stderr, "--timeout") {
		t.Fatalf("invalid timeout: %d %q", code, stderr)
	}
	for _, args := range [][]string{{"app", "missing"}, {"missing", "record"}, {"app"}} {
		if code, stdout, _ := invoke(args...); code != 2 || stdout != "" {
			t.Fatalf("%v: %d %q", args, code, stdout)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "workbench")); !os.IsNotExist(err) {
		t.Fatal("run wrote a report")
	}
}
