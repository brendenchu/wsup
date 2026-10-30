package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureVersion1(t *testing.T) (string, map[string]any) {
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

func saveVersion1(t *testing.T, root string, document any) {
	t.Helper()
	body, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "workspace.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadVersion1RejectsInvalidContracts(t *testing.T) {
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
			root, document := fixtureVersion1(t)
			project := document["projects"].([]any)[0].(map[string]any)
			tc.edit(document, project)
			saveVersion1(t, root, document)
			if _, err := LoadVersion1(root); err == nil || !strings.Contains(err.Error(), tc.error) {
				t.Fatalf("got %v, want %q", err, tc.error)
			}
		})
	}
}

func TestVersion1ChecksPreserveLiteralArgsOrderAndNullTimeout(t *testing.T) {
	root, document := fixtureVersion1(t)
	saveVersion1(t, root, document)
	body, err := os.ReadFile(filepath.Join(root, "workspace.json"))
	if err != nil {
		t.Fatal(err)
	}
	body = []byte(strings.Replace(string(body), `"id":"app"`,
		`"checks":{"z":["tool","", "$(touch unsafe)"],"a":{"command":["tool"],"timeout_seconds":null,"finding_exit_codes":[]}},"id":"app"`, 1))
	if err := os.WriteFile(filepath.Join(root, "workspace.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadVersion1(root)
	if err != nil {
		t.Fatal(err)
	}
	project := loaded.Projects[0]
	if strings.Join(project.CheckNames, ",") != "z,a" || project.Checks["z"].Command[1] != "" ||
		project.Checks["a"].TimeoutSeconds != nil || len(project.Checks["a"].FindingExitCodes) != 0 {
		t.Fatalf("lost check contract: %+v", project)
	}
}
