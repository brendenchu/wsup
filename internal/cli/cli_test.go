package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

func TestHelpVersionAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		args   []string
		code   int
		output string
		error  string
	}{
		{"help", []string{"--help"}, 0, "WSUP portable local workspace tools", ""},
		{"empty", nil, 2, "", "a command is required"},
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

func TestRootFlagMustPrecedeCommand(t *testing.T) {
	for _, args := range [][]string{
		{"guidance", "--root", t.TempDir()},
		{"guidance", "--root=" + t.TempDir()},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr); code != 2 || stdout.Len() != 0 ||
			!strings.Contains(stderr.String(), "unknown flag") {
			t.Fatalf("%v: stdout=%q stderr=%q", args, &stdout, &stderr)
		}
	}
}

func TestEditorJSONUsesStableASCIIEncoding(t *testing.T) {
	var actual bytes.Buffer
	if err := writeEditorJSON(&actual, map[string]string{
		"path": "caf\u00e9/\U0001f680/\x7f/<>&/\u2028",
	}); err != nil {
		t.Fatal(err)
	}
	expected := "{\n  \"path\": \"caf\\u00e9/\\ud83d\\ude80/\\u007f/<>&/\\u2028\"\n}\n"
	if actual.String() != expected {
		t.Fatalf("got %q, want %q", actual.String(), expected)
	}
}

func TestMalformedRegistryNeverProducesSuccessOrEditorOutput(t *testing.T) {
	for _, body := range []string{
		`{"version":`, `{} {}`, `null`, `[]`, `{"version":true}`,
		`{"version":1,"name":"Synthetic","projects":null}`,
		"{\"version\":1,\"name\":\"\xff\",\"projects\":[]}",
	} {
		t.Run(body, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "workspace.json"), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}

			for _, command := range []string{"validate", "projects", "config", "editor"} {
				var stdout, stderr bytes.Buffer
				if code := Run([]string{"--root", root, command}, &stdout, &stderr); code != 2 ||
					stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "Error: ") {
					t.Fatalf("%s: stdout=%q stderr=%q", command, &stdout, &stderr)
				}
			}
			if _, err := os.Lstat(filepath.Join(root, "workspace.code-workspace")); !os.IsNotExist(err) {
				t.Fatalf("malformed registry created editor output: %v", err)
			}
		})
	}
}

func TestEditorIsRelativeAndPreservesExistingOutput(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "app"), 0755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"app/README.md":  "Synthetic\n",
		"workspace.json": `{"version":1,"name":"Synthetic","projects":[{"id":"app","path":"app","kind":"app","repository":"independent","purpose":"Fixture","owner":"Test"}]}`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--root", root, "editor"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, &stderr)
	}
	filename := filepath.Join(root, "workspace.code-workspace")
	original, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var document struct{ Folders []map[string]string }
	if err := json.Unmarshal(original, &document); err != nil ||
		len(document.Folders) != 2 || document.Folders[1]["path"] != "app" {
		t.Fatalf("expected relative project folder: %s %v", original, err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"--root", root, "editor"}, &stdout, &stderr); code != 2 || stdout.Len() != 0 {
		t.Fatalf("overwrite exit %d: stdout=%q stderr=%q", code, &stdout, &stderr)
	}
	current, err := os.ReadFile(filename)
	if err != nil || !bytes.Equal(original, current) {
		t.Fatalf("existing output changed: %v", err)
	}
}

func TestLongOptionAbbreviations(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
		text string
	}{
		{[]string{"--ver"}, 0, Version},
		{[]string{"--h"}, 0, "Usage:"},
		{[]string{"guidance", "--h"}, 0, "Usage:"},
		{[]string{"--ro", t.TempDir(), "guidance"}, 1, "Missing AI.md"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(tc.args, &stdout, &stderr); code != tc.code ||
			!strings.Contains(stdout.String(), tc.text) || stderr.Len() != 0 {
			t.Fatalf("%v: stdout=%q stderr=%q", tc.args, &stdout, &stderr)
		}
	}
	flags := pflag.NewFlagSet("ambiguity", pflag.ContinueOnError)
	flags.String("project", "", "")
	flags.String("profile", "", "")
	if got := abbreviatedFlag(flags, "pro"); got != "pro" {
		t.Fatalf("ambiguous prefix resolved to %q", got)
	}
	if got := abbreviatedFlag(flags, "proj"); got != "project" {
		t.Fatalf("unique prefix did not resolve: %q", got)
	}
	if got := abbreviatedFlag(flags, "project"); got != "project" {
		t.Fatalf("exact name changed: %q", got)
	}
}
