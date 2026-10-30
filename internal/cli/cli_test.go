package cli

import (
	"bytes"
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

func TestVSCodeRefreshesOnlyTheManagedEditorFile(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "workspace")
	invoke := func(want int, args ...string) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := Run(append([]string{"--root", root}, args...), &stdout, &stderr); code != want {
			t.Fatalf("%v: %d %s %s", args, code, &stdout, &stderr)
		}
		return stdout.String()
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"init", root, "--yes"}, &stdout, &stderr); code != 0 {
		t.Fatalf("init: %s", &stderr)
	}
	if output := invoke(0, "vscode"); !strings.Contains(output, `"actions": []`) {
		t.Fatalf("fresh workspace not settled: %s", output)
	}
	if err := os.MkdirAll(filepath.Join(root, "apps/app"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "apps/app/README.md"), []byte("App\n"), 0600); err != nil {
		t.Fatal(err)
	}
	document, err := os.ReadFile(filepath.Join(root, "wsup.json"))
	if err != nil {
		t.Fatal(err)
	}
	document = bytes.Replace(document, []byte(`"projects": []`), []byte(`"projects": [{"id":"app","path":"apps/app","kind":"app","repository":"workspace","purpose":"P","owner":"O"}]`), 1)
	if err := os.WriteFile(filepath.Join(root, "wsup.json"), document, 0600); err != nil {
		t.Fatal(err)
	}
	// Another edited managed file is out of scope and left alone.
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("Owner readme\n"), 0600); err != nil {
		t.Fatal(err)
	}
	editor := filepath.Join(root, "my-workspace.code-workspace")
	before, err := os.ReadFile(editor)
	if err != nil {
		t.Fatal(err)
	}
	if output := invoke(0, "vscode"); !strings.Contains(output, `"path": "my-workspace.code-workspace"`) || strings.Contains(output, "README.md") {
		t.Fatalf("preview: %s", output)
	}
	if current, err := os.ReadFile(editor); err != nil || !bytes.Equal(before, current) {
		t.Fatal("preview wrote the editor file")
	}
	invoke(0, "vscode", "--apply")
	body, err := os.ReadFile(editor)
	if err != nil || !strings.Contains(string(body), `"path": "apps/app"`) {
		t.Fatalf("regenerated file: %s %v", body, err)
	}
	if output := invoke(1, "update"); !strings.Contains(output, `"README.md"`) {
		t.Fatalf("update no longer sees the edited README: %s", output)
	}
	// A hand-edited editor file is a conflict, never overwritten.
	if err := os.WriteFile(editor, append(body, ' '), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "apps/web"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "apps/web/README.md"), []byte("Web\n"), 0600); err != nil {
		t.Fatal(err)
	}
	document = bytes.Replace(document, []byte(`"owner":"O"}]`), []byte(`"owner":"O"},{"id":"web","path":"apps/web","kind":"app","repository":"workspace","purpose":"P","owner":"O"}]`), 1)
	if err := os.WriteFile(filepath.Join(root, "wsup.json"), document, 0600); err != nil {
		t.Fatal(err)
	}
	invoke(1, "vscode")
	invoke(2, "vscode", "--apply")
	if current, err := os.ReadFile(editor); err != nil || !bytes.Equal(current, append(body, ' ')) {
		t.Fatal("hand-edited editor file overwritten")
	}

	plain := filepath.Join(parent, "plain")
	if code := Run([]string{"init", plain, "--yes", "--editor", "none"}, &stdout, &stderr); code != 0 {
		t.Fatalf("init: %s", &stderr)
	}
	stderr.Reset()
	if code := Run([]string{"--root", plain, "vscode"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "wsup add vscode") {
		t.Fatalf("missing adapter: %d %s", code, &stderr)
	}
}

func TestGuidanceFindingsExitOneWithoutOperationalError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--root", t.TempDir(), "guidance"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit %d: %s", code, &stderr)
	}

	if stderr.Len() != 0 || !strings.Contains(stdout.String(), "Missing AGENTS.md") {
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

func TestMalformedRegistryNeverProducesSuccessOrEditorOutput(t *testing.T) {
	for _, body := range []string{
		`{"version":`, `{} {}`, `null`, `[]`, `{"version":true}`,
		`{"version":3,"name":"Synthetic","projects":null}`,
		"{\"version\":1,\"name\":\"\xff\",\"projects\":[]}",
	} {
		t.Run(body, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "wsup.json"), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}

			for _, command := range []string{"validate", "projects", "config", "vscode"} {
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

func TestLongOptionAbbreviations(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
		text string
	}{
		{[]string{"--ver"}, 0, Version},
		{[]string{"--h"}, 0, "Usage:"},
		{[]string{"guidance", "--h"}, 0, "Usage:"},
		{[]string{"--ro", t.TempDir(), "guidance"}, 1, "Missing AGENTS.md"},
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
