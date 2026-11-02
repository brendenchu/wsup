package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wavykit/wsup/internal/registry"
)

func projectFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "app"), 0755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"wsup.json":     `{"version":3,"name":"Fixture","projects":[{"id":"app","path":"app","kind":"app","repository":"workspace","owner":"Team","purpose":"Example","tasks":{"test":{"command":["sh","-c","touch EXECUTED"],"check":true}}}]}`,
		"app/README.md": "App", "AGENTS.md": "Root instructions",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestProjectViewsShareMetadataAndPreserveListing(t *testing.T) {
	root := projectFixture(t)
	before := projectSnapshot(t, root)
	var stdout, stderr bytes.Buffer
	invoke := func(args ...string) string {
		t.Helper()
		stdout.Reset()
		stderr.Reset()
		if code := Run(append([]string{"--root", root}, args...), &stdout, &stderr); code != 0 || stderr.Len() != 0 {
			t.Fatalf("%v: %d %q %q", args, code, &stdout, &stderr)
		}
		return stdout.String()
	}
	data, err := registry.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	var expected bytes.Buffer
	if err := writeJSON(&expected, []registry.ProjectEntry{data.Projects[0].Entry}); err != nil {
		t.Fatal(err)
	}
	if got := invoke("projects"); got != expected.String() {
		t.Fatalf("no-ID listing changed:\n%s\nwant:\n%s", got, &expected)
	}
	encoded := invoke("projects", "app", "--json")
	var envelope projectEnvelope
	if err := json.Unmarshal([]byte(encoded), &envelope); err != nil || envelope.Version != 1 || envelope.Command != "projects" ||
		!envelope.OK || envelope.Result == nil || !reflect.DeepEqual(envelope.Errors, []string{}) {
		t.Fatalf("success envelope: %s %v", encoded, err)
	}
	if strings.Contains(encoded, root) || envelope.Result.Dependencies == nil || envelope.Result.Dependents == nil ||
		envelope.Result.Tasks == nil || envelope.Result.Instructions == nil || envelope.Result.Skills == nil {
		t.Fatalf("invalid result shape: %s", encoded)
	}
	var readable bytes.Buffer
	if err := writeProject(&readable, *envelope.Result); err != nil {
		t.Fatal(err)
	}
	if got := invoke("projects", "app"); got != readable.String() {
		t.Fatalf("readable and JSON result diverge: %s", got)
	}
	for _, want := range []string{"Location: app", "Owner: Team", "wsup run app test", "wsup check --project app --task test",
		"not executed", "prerequisites not checked", "AGENTS.md", "workspace policy", "Presence does not prove"} {
		if !strings.Contains(readable.String(), want) {
			t.Errorf("readable output lacks %q: %s", want, &readable)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "app/EXECUTED")); !os.IsNotExist(err) {
		t.Fatal("task executed")
	}
	if _, err := os.Stat(filepath.Join(root, "workbench")); !os.IsNotExist(err) {
		t.Fatal("inspection wrote a report")
	}
	if help := invoke("projects", "--help"); !strings.Contains(help, "projects [ID]") || !strings.Contains(help, "--json") {
		t.Fatalf("project help: %s", help)
	}
	if !reflect.DeepEqual(before, projectSnapshot(t, root)) {
		t.Fatal("CLI inspection changed fixture contents or inventory")
	}
}

func projectSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			result[name] = "directory"
			return nil
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		result[name] = string(body)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestProjectErrorsUseOneEnvelopeOnlyAfterParsing(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		edit func(*testing.T, string)
		json bool
		want string
	}{
		{"unknown readable", []string{"projects", "missing"}, nil, false, "Unknown project: missing"},
		{"unknown JSON", []string{"projects", "missing", "--json"}, nil, true, "Unknown project: missing"},
		{"missing ID", []string{"projects", "--json"}, nil, false, "requires a project ID"},
		{"extra ID", []string{"projects", "app", "extra", "--json"}, nil, false, "accepts at most 1 arg"},
		{"unknown flag", []string{"projects", "app", "--json", "--invalid"}, nil, false, "unknown flag"},
		{"malformed registry", []string{"projects", "app", "--json"}, func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, "wsup.json"), []byte("{"), 0600); err != nil {
				t.Fatal(err)
			}
		}, true, "invalid JSON"},
		{"bad settings", []string{"projects", "app", "--json"}, func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, "wsup.local.json"), []byte(`{"version":1,"settings":{"timeout_seconds":0}}`), 0600); err != nil {
				t.Fatal(err)
			}
		}, true, "timeout_seconds"},
		{"invalid path", []string{"projects", "app", "--json"}, func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, "app/README.md")); err != nil {
				t.Fatal(err)
			}
		}, true, "contain README.md"},
		{"guidance error", []string{"projects", "app", "--json"}, func(t *testing.T, root string) {
			if err := os.Mkdir(filepath.Join(root, "app/AGENTS.md"), 0755); err != nil {
				t.Fatal(err)
			}
		}, true, "regular file"},
		{"local config escape", []string{"projects", "app", "--json"}, func(t *testing.T, root string) {
			if err := os.Symlink(filepath.Join(t.TempDir(), "outside"), filepath.Join(root, "wsup.local.json")); err != nil {
				t.Fatal(err)
			}
		}, true, "escapes"},
		{"local config nonregular", []string{"projects", "app", "--json"}, func(t *testing.T, root string) {
			if err := os.Mkdir(filepath.Join(root, "wsup.local.json"), 0755); err != nil {
				t.Fatal(err)
			}
		}, true, "regular file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := projectFixture(t)
			if tc.edit != nil {
				tc.edit(t, root)
			}
			var stdout, stderr bytes.Buffer
			if code := Run(append([]string{"--root", root}, tc.args...), &stdout, &stderr); code != 2 {
				t.Fatalf("exit %d: %s %s", code, &stdout, &stderr)
			}
			if tc.json {
				var envelope projectEnvelope
				if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || envelope.OK || envelope.Result != nil ||
					envelope.Version != 1 || envelope.Command != "projects" || len(envelope.Errors) != 1 ||
					!strings.Contains(envelope.Errors[0], tc.want) || stderr.Len() != 0 {
					t.Fatalf("JSON error: %s stderr %s parse %v", &stdout, &stderr, err)
				}
			} else if stdout.Len() != 0 || !strings.Contains(stderr.String(), "Error: ") || !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("ordinary error: %q %q", &stdout, &stderr)
			}
		})
	}
}

type projectFailWriter struct{ calls int }

func (w *projectFailWriter) Write(body []byte) (int, error) {
	w.calls++
	return 0, errors.New("synthetic writer failure")
}

func TestProjectOutputFailuresDoNotAttemptSecondDocument(t *testing.T) {
	root := projectFixture(t)
	for _, args := range [][]string{{"projects", "app"}, {"projects", "app", "--json"}, {"projects", "missing", "--json"}} {
		var output projectFailWriter
		var stderr bytes.Buffer
		if code := Run(append([]string{"--root", root}, args...), &output, &stderr); code != 2 ||
			output.calls != 1 || !strings.Contains(stderr.String(), "synthetic writer failure") {
			t.Fatalf("%v: %d writes=%d %s", args, code, output.calls, &stderr)
		}
	}
}
