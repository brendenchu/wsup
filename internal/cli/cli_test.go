package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionMatchesPythonBaseline(t *testing.T) {
	body, err := os.ReadFile("../../src/wsup_core/__init__.py")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(`VERSION = "`+Version+`"`)) {
		t.Fatal("Go version differs from the Python product version")
	}
}

func TestHelpVersionAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		args   []string
		code   int
		output string
		error  string
	}{
		{"help", []string{"--help"}, 0, "Go migration bootstrap", ""},
		{"empty", nil, 0, "Usage:", ""},
		{"version", []string{"--version"}, 0, Version + "\n", ""},
		{"unknown command", []string{"unknown"}, 2, "", "unknown command"},
		{"unknown flag", []string{"--invalid"}, 2, "", "unknown flag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run(tc.args, &stdout, &stderr); code != tc.code {
				t.Fatalf("exit %d, want %d; stderr: %s", code, tc.code, &stderr)
			}
			if !strings.Contains(stdout.String(), tc.output) ||
				!strings.Contains(stderr.String(), tc.error) {
				t.Fatalf("stdout=%q stderr=%q", &stdout, &stderr)
			}
			if tc.error == "" && stderr.Len() != 0 {
				t.Fatalf("unexpected stderr: %s", &stderr)
			}
		})
	}
}

func TestUnportedCommandsNeverSucceed(t *testing.T) {
	for _, name := range []string{
		"check", "init", "collections", "update",
	} {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run([]string{"--root", t.TempDir(), name, "--apply"}, &stdout, &stderr); code != 2 {
				t.Fatalf("exit %d, want 2", code)
			}

			if stdout.Len() != 0 || !strings.Contains(stderr.String(), "not yet implemented in Go") {
				t.Fatalf("stdout=%q stderr=%q", &stdout, &stderr)
			}
		})
	}
}

func TestEditorExclusiveCreationAndReadOnlyErrors(t *testing.T) {
	root := t.TempDir()
	document := `{"version":1,"name":"Synthetic","projects":[{"id":"app","path":".","kind":"tool","repository":"workspace","purpose":"Fixture","owner":"Test"}]}`
	for name, body := range map[string]string{"README.md": "Local\n", "workspace.json": document} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--root", root, "editor"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, &stderr)
	}
	name := filepath.Join(root, "workspace.code-workspace")
	body, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	var editor struct{ Folders []map[string]string }
	if err := json.Unmarshal(body, &editor); err != nil || len(editor.Folders) != 1 ||
		editor.Folders[0]["path"] != "." {
		t.Fatalf("invalid editor document: %s %v", body, err)
	}
	for _, args := range [][]string{{"editor"}, {"editor", "--output", "../escape"}, {"editor", "--output", "sub/file"}} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(append([]string{"--root", root}, args...), &stdout, &stderr); code != 2 {
			t.Fatalf("invalid editor output accepted: %v", args)
		}
		current, err := os.ReadFile(name)
		if err != nil || !bytes.Equal(body, current) || stdout.Len() != 0 {
			t.Fatal("existing editor file changed or failed operation printed success")
		}
	}
}

func TestGuidanceFindingsExitOneWithoutOperationalError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--root", t.TempDir(), "guidance"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit %d: %s", code, &stderr)
	}
	if stderr.Len() != 0 || !strings.Contains(stdout.String(), "Missing AI.md") {
		t.Fatalf("stdout=%q stderr=%q", &stdout, &stderr)
	}
}
