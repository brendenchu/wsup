//go:build unix

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportFailureNeverPrintsSuccessfulReportPath(t *testing.T) {
	root := checkFixture(t, map[string]any{"break": checkTool(t, "break-report", "unused")})
	code, stdout, stderr := invokeCheck(root, "--project", "app")
	if code != 2 || !strings.Contains(stdout, "app:break: passed") ||
		strings.Contains(stdout, "Report: ") || !strings.Contains(stderr, "report input workspace.json must be a regular file") {
		t.Fatalf("report failure hidden: %d %q %q", code, stdout, stderr)
	}
}

func TestMissingPythonDoesNotPreventSubsequentExplicitTool(t *testing.T) {
	root := checkFixture(t, map[string]any{
		"a": []string{"{python}", "-c", "pass"},
		"b": checkTool(t, "marker", "executed"),
	})
	t.Setenv("PATH", t.TempDir())
	code, stdout, stderr := invokeCheck(root, "--project", "app")
	if code != 1 || stderr != "" || !strings.Contains(stdout, "app:a: error\nRunning app:b\napp:b: passed") {
		t.Fatalf("missing Python fallback or early exit: %d %q %q", code, stdout, stderr)
	}
	data, _ := readCheckReport(t, root)
	if data.Results[0].Error != "launch_error:2" || data.Results[1].Status != "passed" {
		t.Fatal("missing Python incorrectly classified")
	}
	if _, err := os.Stat(filepath.Join(root, "app/executed")); err != nil {
		t.Fatal("explicit tool did not execute")
	}
}

func TestCheckTimeoutBeatsWorkspaceDefault(t *testing.T) {
	root := checkFixture(t, map[string]any{
		"sleep": map[string]any{"command": checkTool(t, "sleep", "unused"), "timeout_seconds": 1},
	})
	t.Setenv("WSUP_TIMEOUT_SECONDS", "30")
	code, stdout, stderr := invokeCheck(root, "--project", "app")
	if code != 1 || stderr != "" || !strings.Contains(stdout, "app:sleep: timeout") {
		t.Fatalf("per-check timeout: %d %q %q", code, stdout, stderr)
	}
	data, _ := readCheckReport(t, root)
	if data.Sources["timeout_seconds"] != "WSUP_TIMEOUT_SECONDS" || data.Results[0].DurationSeconds >= 5 {
		t.Fatal("check timeout failed to override workspace setting")
	}
}

func TestOutputHiddenByDefaultAndDependenciesNotSelectedImplicitly(t *testing.T) {
	root := checkFixture(t, map[string]any{"record": checkTool(t, "record", "SYNTHETIC-SECRET-CANARY")})
	filename := filepath.Join(root, "workspace.json")
	body, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatal(err)
	}
	projects := document["projects"].([]any)
	projects[0].(map[string]any)["dependencies"] = []string{"dependency"}
	projects = append(projects, map[string]any{
		"id": "dependency", "path": ".", "kind": "tool", "repository": "workspace",
		"purpose": "Fixture", "owner": "Test", "checks": map[string]any{"write": checkTool(t, "marker", "unexpected")},
	})
	document["projects"] = projects
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("Synthetic\n"), 0600); err != nil {
		t.Fatal(err)
	}
	body, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, body, 0600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := invokeCheck(root, "--pro", "app", "--ch", "record")
	if code != 0 || stderr != "" || strings.Contains(stdout, "SYNTHETIC-SECRET-CANARY") ||
		strings.Contains(stdout, "dependency") {
		t.Fatalf("unexpected output/dependency selection: %d %q %q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "unexpected")); !os.IsNotExist(err) {
		t.Fatal("dependency ran implicitly")
	}
	data, _ := readCheckReport(t, root)
	if len(data.Results) != 1 || data.Results[0].Project != "app" {
		t.Fatal("report included unselected dependency")
	}
}
