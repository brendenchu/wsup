package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/brendenchu/wsup/internal/catalogue"
	"github.com/brendenchu/wsup/internal/scaffolding"
)

// ask runs init interactively with scripted answers.
func ask(t *testing.T, input string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runSession(context.Background(), append([]string{"init"}, args...), &stdout, &stderr,
		session{in: strings.NewReader(input), interactive: true})
	return code, stdout.String(), stderr.String()
}

func selection(t *testing.T, root string) catalogue.Selection {
	t.Helper()
	loaded, err := scaffolding.LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	return lockSelection(t, loaded.Lock)
}

func TestInteractiveInitAsksOpenQuestions(t *testing.T) {
	parent := t.TempDir()
	development := []string{"apps", "infra", "packages", "tools"}
	for _, tc := range []struct {
		name, input string
		args        []string
		questions   []string
		collections []string
		agents      []string
		editors     []string
	}{
		{"defaults", "\n\n\n", nil, []string{"Preset", "Agents", "Editors"}, development, []string{"claude", "gemini"}, []string{"vscode"}},
		{"answers", "none\nclaude\nnone\n", nil, []string{"Preset", "Agents", "Editors"}, nil, []string{"claude"}, nil},
		{"comma list", "development\ngemini, claude\nvscode\n", nil, []string{"Preset", "Agents", "Editors"}, development, []string{"claude", "gemini"}, []string{"vscode"}},
		{"retry", "bogus\nnone\nnobody\nnone\n\n", nil, []string{"Preset", "Agents", "Editors"}, nil, nil, []string{"vscode"}},
		{"flags answer", "\n", []string{"--preset", "none", "--agent", "none"}, []string{"Editors"}, nil, nil, []string{"vscode"}},
		{"collection still asks preset", "none\n\n\n", []string{"--collection", "research"}, []string{"Preset", "Agents", "Editors"}, []string{"research"}, []string{"claude", "gemini"}, []string{"vscode"}},
		{"yes", "", []string{"--yes"}, nil, development, []string{"claude", "gemini"}, []string{"vscode"}},
	} {
		root := filepath.Join(parent, strings.ReplaceAll(tc.name, " ", "-"))
		code, stdout, stderr := ask(t, tc.input, append([]string{root}, tc.args...)...)
		if code != 0 {
			t.Fatalf("%s: %d %s %s", tc.name, code, stdout, stderr)
		}
		for _, question := range []string{"Preset", "Agents", "Editors"} {
			want := false
			for _, asked := range tc.questions {
				want = want || asked == question
			}
			if strings.Contains(stdout, question+" ") != want {
				t.Fatalf("%s: question %s asked=%v\n%s", tc.name, question, !want, stdout)
			}
		}
		state := selection(t, root)
		if !slices.Equal(state.Collections, tc.collections) || !slices.Equal(state.Agents, tc.agents) ||
			!slices.Equal(state.Editors, tc.editors) {
			t.Fatalf("%s: %v %v %v", tc.name, state.Collections, state.Agents, state.Editors)
		}
	}
	_, stdout, _ := ask(t, "bogus\nnone\n\n\n", filepath.Join(parent, "explained"))
	if !strings.Contains(stdout, `unknown preset "bogus"`) {
		t.Fatalf("invalid answer not explained: %s", stdout)
	}
}

func TestInteractiveProjectInitAsksOnlyForAgents(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	code, stdout, stderr := ask(t, "gemini\n", root, "--template", "project")
	if code != 0 || strings.Contains(stdout, "Preset") || strings.Contains(stdout, "Editors") || !strings.Contains(stdout, "Agents") {
		t.Fatalf("project questions: %d %s %s", code, stdout, stderr)
	}
	if state := selection(t, root); !reflect.DeepEqual(state.Agents, []string{"gemini"}) || len(state.Editors) != 0 {
		t.Fatalf("project selection: %+v", state)
	}
}

func TestInteractiveInitWritesNothingWhenCancelledOrRejected(t *testing.T) {
	parent := t.TempDir()
	for name, tc := range map[string]struct {
		input string
		args  []string
		want  string
	}{
		"end of input":       {"\n", nil, "init cancelled"},
		"no valid answer":    {"x\ny\nz\n", nil, "no valid answer after 3 attempts"},
		"bad flag first":     {"\n\n\n", []string{"--agent", "unknown"}, `unknown agent "unknown"`},
		"project preset":     {"\n", []string{"--template", "project", "--preset", "none"}, "presets require a workspace"},
		"none with an agent": {"none claude\nnone claude\nnone claude\n", []string{"--preset", "none"}, "no valid answer"},
	} {
		root := filepath.Join(parent, strings.ReplaceAll(name, " ", "-"))
		code, stdout, stderr := ask(t, tc.input, append([]string{root}, tc.args...)...)
		if code != 2 || !strings.Contains(stderr, tc.want) {
			t.Fatalf("%s: %d %q %q", name, code, stdout, stderr)
		}
		if strings.HasPrefix(name, "bad") || strings.HasPrefix(name, "project") {
			if strings.Contains(stdout, "[") {
				t.Fatalf("%s: asked before rejecting flags: %s", name, stdout)
			}
		}
		if _, err := os.Lstat(root); !os.IsNotExist(err) {
			t.Fatalf("%s: wrote output", name)
		}
	}
}

func TestNonTerminalInitNeverAsks(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"init", root}, &stdout, &stderr); code != 0 || strings.Contains(stdout.String(), "[") {
		t.Fatalf("non-terminal init: %d %s %s", code, &stdout, &stderr)
	}
	if session := terminalSession(&stdout); session.interactive {
		t.Fatal("a buffer was treated as a terminal")
	}
}
