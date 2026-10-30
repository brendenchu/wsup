package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestInitAnswersFlagsAndYesAgree initializes each selection three ways:
// answering the questions, passing the equivalent flags without a terminal,
// and passing them with --yes in a terminal. All three must write the same
// bytes.
func TestInitAnswersFlagsAndYesAgree(t *testing.T) {
	for _, tc := range []struct {
		name, answers string
		flags         []string
	}{
		{"defaults", "\n\n\n", nil},
		{"base with one agent", "none\nclaude\nnone\n", []string{"--preset", "none", "--agent", "claude", "--editor", "none"}},
		{"preset and collection", "development\ngemini\nvscode\n", []string{"--collection", "journal", "--agent", "gemini", "--editor", "vscode"}},
		{"project", "gemini\n", []string{"--template", "project", "--agent", "gemini"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent := t.TempDir()
			answered, flagged, yes := filepath.Join(parent, "answered"), filepath.Join(parent, "flagged"), filepath.Join(parent, "yes")
			// Answers supply only what the flags would; keep the starter kind and
			// any collection flag, which have no question.
			fixed := []string{}
			for i := 0; i < len(tc.flags); i += 2 {
				if tc.flags[i] == "--template" || tc.flags[i] == "--collection" {
					fixed = append(fixed, tc.flags[i], tc.flags[i+1])
				}
			}
			if code, stdout, stderr := ask(t, tc.answers, append([]string{answered}, fixed...)...); code != 0 {
				t.Fatalf("answered: %d %s %s", code, stdout, stderr)
			}
			var stdout, stderr bytes.Buffer
			if code := Run(append([]string{"init", flagged}, tc.flags...), &stdout, &stderr); code != 0 {
				t.Fatalf("flagged: %d %s", code, &stderr)
			}
			if code, stdout, stderr := ask(t, "", append([]string{yes, "--yes"}, tc.flags...)...); code != 0 || strings.Contains(stdout, "[") {
				t.Fatalf("yes: %d %s %s", code, stdout, stderr)
			}
			want := relativeBytes(t, flagged)
			for name, root := range map[string]string{"answered": answered, "yes": yes} {
				if got := relativeBytes(t, root); !reflect.DeepEqual(got, want) {
					for file := range want {
						if got[file] != want[file] {
							t.Errorf("%s: %s differs from the flagged starter", name, file)
						}
					}
					for file := range got {
						if _, ok := want[file]; !ok {
							t.Errorf("%s: extra %s", name, file)
						}
					}
				}
			}
		})
	}
}

// relativeBytes is workspaceBytes keyed by path relative to root, so two
// starters can be compared.
func relativeBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	for name, body := range workspaceBytes(t, root) {
		relative, err := filepath.Rel(root, name)
		if err != nil {
			t.Fatal(err)
		}
		files[relative] = body
	}
	return files
}

// TestInvalidSelectionsWriteNothing runs every invalid selection through the
// CLI: init must fail with a usage error before creating its destination, and
// add must fail without changing the starter.
func TestInvalidSelectionsWriteNothing(t *testing.T) {
	parent := t.TempDir()
	for _, args := range [][]string{
		{"--collection", "unknown"},
		{"--preset", "unknown"},
		{"--agent", "unknown"},
		{"--editor", "unknown"},
		{"--agent", "none", "--agent", "claude"},
		{"--editor", "none", "--editor", "vscode"},
		{"--template", "unknown"},
		{"--template", "project", "--collection", "research"},
		{"--template", "project", "--preset", "none"},
		{"--template", "project", "--editor", "vscode"},
	} {
		destination := filepath.Join(parent, "starter")
		var stdout, stderr bytes.Buffer
		if code := Run(append([]string{"init", destination}, args...), &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Errorf("init %v: %d stdout=%s stderr=%s", args, code, &stdout, &stderr)
		}
		if _, err := os.Lstat(destination); !os.IsNotExist(err) {
			t.Fatalf("init %v created its destination", args)
		}
	}
	for starter, invalid := range map[string][][]string{
		"workspace": {{"unknown"}, {"development"}, {"workspace-base"}, {"project-base"}, {"none"}},
		"project":   {{"research"}, {"vscode"}, {"development"}, {"project-base"}},
	} {
		root := filepath.Join(parent, starter)
		var stdout, stderr bytes.Buffer
		if code := Run([]string{"init", root, "--template", starter, "--agent", "none"}, &stdout, &stderr); code != 0 {
			t.Fatalf("init %s: %d %s", starter, code, &stderr)
		}
		before := workspaceBytes(t, root)
		for _, ids := range invalid {
			var stdout, stderr bytes.Buffer
			args := append([]string{"--root", root, "add"}, append(ids, "--apply")...)
			if code := Run(args, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
				t.Errorf("%s add %v: %d stdout=%s stderr=%s", starter, ids, code, &stdout, &stderr)
			}
		}
		if !reflect.DeepEqual(before, workspaceBytes(t, root)) {
			t.Fatalf("invalid additions changed the %s", starter)
		}
	}
}
