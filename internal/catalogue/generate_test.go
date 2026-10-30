package catalogue

import (
	"strings"
	"testing"

	"github.com/brendenchu/wsup/internal/registry"
)

func TestCodeWorkspaceUsesStableASCIIEncoding(t *testing.T) {
	body, err := CodeWorkspace(Workspace{Name: "x", Projects: []registry.ProjectEntry{
		{ID: "root", Path: "."}, {ID: "app", Path: "caf\u00e9/\U0001f680/\x7f/<>&/\u2028"},
	}})
	want := "{\n  \"folders\": [\n    {\n      \"name\": \"Workspace\",\n      \"path\": \".\"\n    },\n" +
		"    {\n      \"name\": \"app\",\n      \"path\": \"caf\\u00e9/\\ud83d\\ude80/\\u007f/<>&/\\u2028\"\n    }\n  ],\n" +
		"  \"settings\": {}\n}\n"
	if err != nil || string(body) != want {
		t.Fatalf("got %q, want %q (%v)", body, want, err)
	}
}

func TestSlugAndWorkspaceInput(t *testing.T) {
	for name, want := range map[string]string{
		"My workspace": "my-workspace", "  Acme  Client--Work 2 ": "acme-client-work-2",
		"Caf\u00e9": "caf", "": "workspace", "---": "workspace",
	} {
		if got := Slug(name); got != want {
			t.Fatalf("Slug(%q) = %q, want %q", name, got, want)
		}
	}
	workspace, err := ParseWorkspace([]byte(`{"version":3,"name":"Acme","projects":[` + project("app", "apps/app", "") + `]}`))
	if err != nil || workspace.Name != "Acme" || len(workspace.Projects) != 1 || workspace.Projects[0].Path != "apps/app" {
		t.Fatalf("workspace input: %+v %v", workspace, err)
	}
	for _, body := range []string{`{`, `{"version":1,"name":"Acme","projects":[]}`, `{"version":3,"name":"Acme","projects":[{"id":"app"}]}`, `[]`} {
		if _, err := ParseWorkspace([]byte(body)); err == nil {
			t.Fatalf("invalid registry accepted: %s", body)
		}
	}
}

func TestStarterGeneratesTheEditorFileFromTheRegistry(t *testing.T) {
	files, _, err := Starter("workspace", Selection{Editors: []string{"vscode"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := files["my-workspace.code-workspace"]; !ok {
		t.Fatalf("missing generated file: %v", len(files))
	}
	files, _, err = Starter("workspace", Selection{Editors: []string{"vscode"},
		Registry: []byte(`{"version":3,"name":"Client Work","projects":[` + project("api", "apps/api", "") + `]}`)})
	body := files["client-work.code-workspace"]
	if err != nil || !strings.Contains(string(body), `"name": "api",`+"\n"+`      "path": "apps/api"`) {
		t.Fatalf("registry-driven file: %s %v", body, err)
	}
	if _, _, err := Starter("project", Selection{Editors: []string{"vscode"}}); err == nil {
		t.Fatal("editor adapter accepted for a project")
	}
	if plain, _, err := Starter("workspace", Selection{Registry: []byte(`not json`)}); err != nil || len(plain) == 0 {
		t.Fatalf("registry read without a generator: %v", err)
	}
}

// project returns one wsup.json project entry with the given tasks object.
func project(id, path, tasks string) string {
	entry := `{"id":"` + id + `","path":"` + path + `","kind":"app","repository":"workspace","purpose":"P","owner":"O"`
	if tasks != "" {
		entry += `,"tasks":` + tasks
	}
	return entry + "}"
}

func TestCodeWorkspaceExportsTasksAndThePassthrough(t *testing.T) {
	workspace, err := ParseWorkspace([]byte(`{"version":3,"name":"W","projects":[` +
		project("api", "apps/api", `{"test":{"command":["go","test","./..."],"check":true},"serve":{"command":["air"],"cwd":"cmd"}}`) + `,` +
		project("root", ".", `{"lint":["make"]}`) +
		`],"vscode":{"settings":{"editor.tabSize":2},"extensions":["golang.go"],"tasks":[{"label":"owner"}],"launch":[{"name":"debug"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	body, err := CodeWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		`"editor.tabSize": 2`, `"recommendations": [` + "\n" + `      "golang.go"`,
		`"label": "api: test"`, `"group": "test"`, `"cwd": "${workspaceFolder:Workspace}/apps/api"`,
		`"label": "api: serve"`, `"cwd": "${workspaceFolder:Workspace}/apps/api/cmd"`,
		`"label": "root: lint"`, `"cwd": "${workspaceFolder:Workspace}"`,
		`"version": "2.0.0"`, `"label": "owner"`, `"version": "0.2.0"`, `"name": "debug"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s in:\n%s", want, text)
		}
	}
	// Project tasks come first, in order; owner tasks follow; only checks join the test group.
	if strings.Index(text, `"api: test"`) > strings.Index(text, `"api: serve"`) ||
		strings.Index(text, `"root: lint"`) > strings.Index(text, `"owner"`) || strings.Count(text, `"group": "test"`) != 1 {
		t.Fatalf("task order or groups:\n%s", text)
	}
	if strings.Contains(text, `"args": []`) {
		t.Fatalf("empty args written:\n%s", text)
	}
}
