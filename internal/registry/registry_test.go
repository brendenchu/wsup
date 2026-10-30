package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) (string, map[string]any) {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "app"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app/README.md"), []byte("Synthetic\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return root, map[string]any{"version": 3, "name": "Synthetic", "projects": []any{
		map[string]any{"id": "app", "path": "app", "kind": "app", "repository": "independent",
			"purpose": "Fixture", "owner": "Test"}}}
}

func save(t *testing.T, root string, document any) {
	t.Helper()
	body, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, DocumentPath), body, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRejectsInvalidContracts(t *testing.T) {
	outside := t.TempDir()
	for _, tc := range []struct {
		name  string
		edit  func(root string, d, p map[string]any)
		error string
	}{
		{"boolean version", func(_ string, d, p map[string]any) { d["version"] = true }, "requires version 3"},
		{"float version", func(_ string, d, p map[string]any) { d["version"] = json.Number("3.0") }, "requires version 3"},
		{"unknown root", func(_ string, d, p map[string]any) { d["typo"] = 1 }, "unknown top-level field"},
		{"null projects", func(_ string, d, p map[string]any) { d["projects"] = nil }, "projects must be a list"},
		{"unknown project", func(_ string, d, p map[string]any) { p["typo"] = true }, "supported fields"},
		{"invalid id", func(_ string, d, p map[string]any) { p["id"] = "-app" }, "project ids use"},
		{"missing owner", func(_ string, d, p map[string]any) { delete(p, "owner") }, "missing owner"},
		{"bad kind", func(_ string, d, p map[string]any) { p["kind"] = "service" }, "kind must be one of"},
		{"null tasks", func(_ string, d, p map[string]any) { p["tasks"] = nil }, "named definitions"},
		{"null dependencies", func(_ string, d, p map[string]any) { p["dependencies"] = nil }, "project ids"},
		{"cycle", func(_ string, d, p map[string]any) { p["dependencies"] = []string{"app"} }, "Dependency cycle at app"},
		{"unknown dependency", func(_ string, d, p map[string]any) { p["dependencies"] = []string{"missing"} }, "Unknown dependency: missing"},
		{"path escape", func(_ string, d, p map[string]any) { p["path"] = "../outside" }, "relative forward-slash path"},
		{"missing path", func(_ string, d, p map[string]any) { p["path"] = "absent" }, "path must be unique"},
		{"duplicate id", func(_ string, d, p map[string]any) { d["projects"] = []any{p, p} }, "is not unique"},
		{"symlink escape", func(root string, d, p map[string]any) {
			if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
				t.Fatal(err)
			}
			p["path"] = "escape"
		}, "escapes the workspace"},
		{"duplicate resolved path", func(root string, d, p map[string]any) {
			if err := os.Symlink("app", filepath.Join(root, "alias")); err != nil {
				t.Fatal(err)
			}
			other := map[string]any{}
			for key, value := range p {
				other[key] = value
			}
			other["id"], other["path"] = "other", "alias"
			d["projects"] = []any{p, other}
		}, "path must be unique"},
		{"task cwd escape", func(root string, d, p map[string]any) {
			if err := os.Symlink(outside, filepath.Join(root, "app/out")); err != nil {
				t.Fatal(err)
			}
			p["tasks"] = map[string]any{"t": map[string]any{"command": []string{"x"}, "cwd": "out"}}
		}, "task t"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, document := fixture(t)
			project := document["projects"].([]any)[0].(map[string]any)
			tc.edit(root, document, project)
			save(t, root, document)
			if _, err := Load(root); err == nil || !strings.Contains(err.Error(), tc.error) {
				t.Fatalf("got %v, want %q", err, tc.error)
			}
		})
	}
}

func TestLoadRequiresAWorkspaceFile(t *testing.T) {
	root, _ := fixture(t)
	if _, err := Load(root); err == nil {
		t.Fatal("loaded without wsup.json")
	}
	if err := os.Mkdir(filepath.Join(root, DocumentPath), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("directory accepted: %v", err)
	}
}

func TestSelectRunsCheckTasksInOrder(t *testing.T) {
	root, document := fixture(t)
	if err := os.Mkdir(filepath.Join(root, "lib"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "lib/README.md"), []byte("Lib\n"), 0600); err != nil {
		t.Fatal(err)
	}
	document["projects"] = append(document["projects"].([]any), map[string]any{"id": "lib", "path": "lib", "kind": "package",
		"repository": "workspace", "purpose": "Lib", "owner": "Test",
		"tasks": map[string]any{"a": map[string]any{"command": []string{"lib"}, "check": true}}})
	save(t, root, document)
	body, err := os.ReadFile(filepath.Join(root, DocumentPath))
	if err != nil {
		t.Fatal(err)
	}
	body = []byte(strings.Replace(string(body), `"id":"app"`,
		`"tasks":{"z":{"command":["tool","", "$(touch unsafe)"],"check":true},"build":["make"],"a":{"command":["tool"],"check":true,"cwd":"src","timeout_seconds":null,"finding_exit_codes":[]}},"id":"app"`, 1))
	if err := os.WriteFile(filepath.Join(root, DocumentPath), body, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := loaded.Select([]string{"app", "app"}, nil, false)
	if err != nil || len(selected) != 2 || selected[0].Name != "z" || selected[1].Name != "a" ||
		selected[0].Check.Command[1] != "" || selected[1].Cwd != "src" || selected[1].Check.TimeoutSeconds != nil ||
		len(selected[1].Check.FindingExitCodes) != 0 {
		t.Fatalf("selection lost order, deduplication, or policy: %+v %v", selected, err)
	}
	selected, err = loaded.Select([]string{"app"}, []string{"a", "a"}, false)
	if err != nil || len(selected) != 1 || selected[0].Name != "a" {
		t.Fatalf("explicit selection: %v %v", selected, err)
	}
	selected, err = loaded.Select(nil, []string{"a"}, true)
	if err != nil || len(selected) != 2 || selected[0].Project.ID != "app" || selected[1].Project.ID != "lib" {
		t.Fatalf("--all selection: %+v %v", selected, err)
	}
	selected, err = loaded.Select(nil, []string{"z"}, true)
	if err != nil || len(selected) != 1 || selected[0].Project.ID != "app" {
		t.Fatalf("--all skips projects without the task: %+v %v", selected, err)
	}
	for _, tc := range []struct {
		projects, tasks []string
		all             bool
		error           string
	}{
		{[]string{"missing"}, nil, false, "Unknown project"},
		{[]string{"app"}, []string{"z", "missing"}, false, "unknown task missing"},
		{[]string{"app"}, []string{"build"}, false, "not a check task"},
		{nil, []string{"build"}, true, "no project has"},
	} {
		if selected, err := loaded.Select(tc.projects, tc.tasks, tc.all); err == nil || selected != nil || !strings.Contains(err.Error(), tc.error) {
			t.Fatalf("invalid selection %+v: %v %v", tc, selected, err)
		}
	}
}

func TestInvalidCheckDefinitions(t *testing.T) {
	for _, body := range []string{
		`null`, `[]`, `[""]`, `["tool",null]`, `["tool","\u0000"]`,
		`{"command":["tool"],"typo":true}`, `{"command":["tool"],"enabled":null}`,
		`{"command":["tool"],"mode":null}`, `{"command":["tool"],"reason":null}`,
		`{"command":["tool"],"enabled":false}`, `{"command":["tool"],"timeout_seconds":true}`,
		`{"command":["tool"],"enabled":false,"reason":""}`,
		`{"command":["tool"],"timeout_seconds":1.0}`, `{"command":["tool"],"timeout_seconds":86401}`,
		`{"command":["tool"],"finding_exit_codes":null}`, `{"command":["tool"],"finding_exit_codes":[true]}`,
		`{"command":["tool"],"finding_exit_codes":[0]}`, `{"command":["tool"],"finding_exit_codes":[256]}`,
	} {
		t.Run(body, func(t *testing.T) {
			decoder := json.NewDecoder(strings.NewReader(body))
			decoder.UseNumber()
			var raw any
			if err := decoder.Decode(&raw); err != nil {
				t.Fatal(err)
			}
			if _, err := ParseCheck(raw); err == nil {
				t.Fatal("invalid check accepted")
			}
		})
	}
}

func TestRelativeTargetRejectsTraversalAndSymlinkEscapes(t *testing.T) {
	root := t.TempDir()
	for _, value := range []any{"", nil, "/absolute", "../outside", "a/../b", `a\b`, "a:b"} {
		if _, err := RelativeTarget(root, value); err == nil {
			t.Fatalf("accepted %v", value)
		}
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	for _, value := range []string{"escape", "escape/missing/child"} {
		if _, err := RelativeTarget(root, value); err == nil {
			t.Fatalf("accepted symlink escape %s", value)
		}
	}
	if err := os.Symlink(filepath.Join(root, "not-yet-created"), filepath.Join(root, "local")); err != nil {
		t.Fatal(err)
	}
	if _, err := RelativeTarget(root, "local/child"); err != nil {
		t.Fatalf("safe dangling symlink rejected: %v", err)
	}
	if err := os.Mkdir(filepath.Join(outside, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "nested"), filepath.Join(root, "intermediate")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("intermediate/../secret", filepath.Join(root, "indirect")); err != nil {
		t.Fatal(err)
	}
	if _, err := RelativeTarget(root, "indirect"); err == nil {
		t.Fatal("symlink target traversal was cleaned before resolving its intermediate symlink")
	}
}

func TestJSONRequiresSingleObject(t *testing.T) {
	for _, body := range []string{"null", "[]", "{}", "{}{}", `{"version":`, `true`, "{\"name\":\"\xff\"}"} {
		root := t.TempDir()
		filename := filepath.Join(root, "input.json")
		if err := os.WriteFile(filename, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := ReadObject(filename)
		if (err == nil) != (body == "{}") {
			t.Fatalf("%s: %v", body, err)
		}
	}
}
