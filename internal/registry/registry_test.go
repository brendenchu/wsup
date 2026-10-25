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
	return root, map[string]any{"version": 1, "name": "Synthetic", "projects": []any{
		map[string]any{"id": "app", "path": "app", "kind": "app", "repository": "independent",
			"purpose": "Fixture", "owner": "Test"}}}
}

func save(t *testing.T, root string, document any) {
	t.Helper()
	body, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "workspace.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRejectsInvalidContracts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		edit  func(map[string]any, map[string]any)
		error string
	}{
		{"boolean version", func(d, p map[string]any) { d["version"] = true }, "version 1"},
		{"float version", func(d, p map[string]any) { d["version"] = json.Number("1.0") }, "version 1"},
		{"unknown root", func(d, p map[string]any) { d["typo"] = 1 }, "version 1"},
		{"null projects", func(d, p map[string]any) { d["projects"] = nil }, "projects list"},
		{"unknown project", func(d, p map[string]any) { p["typo"] = true }, "Invalid project fields"},
		{"invalid id", func(d, p map[string]any) { p["id"] = "-app" }, "Project IDs"},
		{"missing owner", func(d, p map[string]any) { delete(p, "owner") }, "missing owner"},
		{"bad kind", func(d, p map[string]any) { p["kind"] = "service" }, "invalid kind/repository"},
		{"null checks", func(d, p map[string]any) { p["checks"] = nil }, "named definitions"},
		{"null dependencies", func(d, p map[string]any) { p["dependencies"] = nil }, "project IDs"},
		{"cycle", func(d, p map[string]any) { p["dependencies"] = []string{"app"} }, "Dependency cycle at app"},
		{"unknown dependency", func(d, p map[string]any) { p["dependencies"] = []string{"missing"} }, "Unknown dependency: missing"},
		{"path escape", func(d, p map[string]any) { p["path"] = "../outside" }, "Paths must stay"},
		{"duplicate id", func(d, p map[string]any) { d["projects"] = []any{p, p} }, "Project IDs"},
		{"duplicate resolved path", func(d, p map[string]any) {
			other := map[string]any{}
			for key, value := range p {
				other[key] = value
			}
			other["id"], other["path"] = "other", "./app"
			d["projects"] = []any{p, other}
		}, "path must be unique"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, document := fixture(t)
			project := document["projects"].([]any)[0].(map[string]any)
			tc.edit(document, project)
			save(t, root, document)
			if _, err := Load(root); err == nil || !strings.Contains(err.Error(), tc.error) {
				t.Fatalf("got %v, want %q", err, tc.error)
			}
		})
	}
}

func TestChecksPreserveLiteralArgsOrderAndNullTimeout(t *testing.T) {
	root, document := fixture(t)
	save(t, root, document)
	body, err := os.ReadFile(filepath.Join(root, "workspace.json"))
	if err != nil {
		t.Fatal(err)
	}
	body = []byte(strings.Replace(string(body), `"id":"app"`,
		`"checks":{"z":["{python}","", "$(touch unsafe)"],"a":{"command":["tool"],"timeout_seconds":null,"finding_exit_codes":[]}},"id":"app"`, 1))
	if err := os.WriteFile(filepath.Join(root, "workspace.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	project := loaded.Projects[0]
	if strings.Join(project.CheckNames, ",") != "z,a" || project.Checks["z"].Command[1] != "" ||
		project.Checks["a"].TimeoutSeconds != nil || len(project.Checks["a"].FindingExitCodes) != 0 {
		t.Fatalf("lost check contract: %+v", project)
	}
	selected, err := loaded.Select([]string{"app", "app"}, nil)
	if err != nil || len(selected) != 2 || selected[0].Name != "z" || selected[1].Name != "a" {
		t.Fatalf("selection lost order/deduplication: %v %v", selected, err)
	}
	selected, err = loaded.Select([]string{"app"}, []string{"a", "a"})
	if err != nil || len(selected) != 1 || selected[0].Name != "a" {
		t.Fatalf("explicit selection: %v %v", selected, err)
	}
	for _, tc := range []struct{ projects, checks []string }{
		{[]string{"missing"}, nil}, {[]string{"app"}, []string{"z", "missing"}},
	} {
		if selected, err := loaded.Select(tc.projects, tc.checks); err == nil || selected != nil {
			t.Fatalf("invalid selection returned partial results: %v %v", selected, err)
		}
	}
}

func TestInvalidCheckDefinitions(t *testing.T) {
	for _, body := range []string{
		`null`, `[]`, `[""]`, `["tool",null]`, `["tool","\u0000"]`,
		`{"command":["tool"],"typo":true}`, `{"command":["tool"],"enabled":null}`,
		`{"command":["tool"],"mode":null}`, `{"command":["tool"],"reason":null}`,
		`{"command":["tool"],"enabled":false}`, `{"command":["tool"],"timeout_seconds":true}`,
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
	for _, body := range []string{"null", "[]", "{}", "{}{}", `{"version":`, `true`} {
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
