package catalogue

import (
	"reflect"
	"strings"
	"testing"
)

func TestCodeWorkspaceUsesStableASCIIEncoding(t *testing.T) {
	body, err := CodeWorkspace(Workspace{Name: "x", Projects: []Project{
		{ID: "root", Path: "."}, {ID: "app", Path: "café/\U0001f680/\x7f/<>&/ "},
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
		"Café": "caf", "": "workspace", "---": "workspace",
	} {
		if got := Slug(name); got != want {
			t.Fatalf("Slug(%q) = %q, want %q", name, got, want)
		}
	}
	workspace, err := ParseWorkspace([]byte(`{"version":1,"name":"Acme","projects":[{"id":"app","path":"apps/app","kind":"app"}]}`))
	if err != nil || !reflect.DeepEqual(workspace, Workspace{Name: "Acme", Projects: []Project{{ID: "app", Path: "apps/app"}}}) {
		t.Fatalf("workspace input: %+v %v", workspace, err)
	}
	for _, body := range []string{`{`, `{"projects":[{"id":"app"}]}`, `[]`} {
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
		Registry: []byte(`{"name":"Client Work","projects":[{"id":"api","path":"apps/api"}]}`)})
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
