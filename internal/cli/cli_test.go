package cli

import (
	"bytes"
	"os"
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
		"validate", "projects", "config", "guidance", "editor",
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
