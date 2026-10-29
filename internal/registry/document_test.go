package registry

import (
	"reflect"
	"strings"
	"testing"
)

// contractExample is the wsup.json example from the v0.3.0 contract.
const contractExample = `{
  "$schema": "https://raw.githubusercontent.com/brendenchu/wsup/main/schema/wsup-3.json",
  "version": 3,
  "name": "Projects",
  "collections": ["apps", "infra", "packages", "tools", "issues"],
  "agents": ["claude", "gemini"],
  "editors": ["vscode"],
  "settings": { "timeout_seconds": 300, "output_limit_bytes": 65536 },
  "projects": [
    {
      "id": "wsup",
      "path": "tools/wsup",
      "kind": "tool",
      "repository": "independent",
      "git": "https://github.com/brendenchu/wsup.git",
      "purpose": "Portable workspace starter and CLI",
      "owner": "Workspace maintainers",
      "dependencies": [],
      "tasks": {
        "test": { "command": ["go", "test", "-race", "./..."], "check": true },
        "vet":  { "command": ["go", "vet", "./..."], "check": true },
        "build": { "command": ["go", "build", "-o", "dist/wsup", "./cmd/wsup"] }
      }
    }
  ],
  "vscode": {
    "settings": { "files.exclude": { "**/node_modules": true } },
    "extensions": ["golang.go"],
    "tasks": [],
    "launch": []
  }
}`

func TestDecodeReadsTheContractExample(t *testing.T) {
	document, err := Decode([]byte(contractExample))
	if err != nil {
		t.Fatal(err)
	}
	if document.Name != "Projects" || !reflect.DeepEqual(document.Agents, []string{"claude", "gemini"}) ||
		document.VSCode == nil || !reflect.DeepEqual(document.VSCode.Extensions, []string{"golang.go"}) {
		t.Fatalf("document: %+v", document)
	}
	project := document.Projects[0]
	names := []string{}
	for _, task := range project.Tasks {
		names = append(names, task.Name)
	}
	if project.Git == "" || project.Kind != "tool" || !reflect.DeepEqual(names, []string{"test", "vet", "build"}) {
		t.Fatalf("project: %+v", project)
	}
	if !project.Tasks[0].IsCheck || project.Tasks[2].IsCheck || project.Tasks[0].Check.Mode != "required" {
		t.Fatalf("tasks: %+v", project.Tasks)
	}
}

func TestEncodeRoundTripsInContractOrder(t *testing.T) {
	document, err := Decode([]byte(contractExample))
	if err != nil {
		t.Fatal(err)
	}
	// Non-default policy fields, cwd, and an explicit empty finding list survive.
	document.Projects = append(document.Projects, ProjectEntry{
		ID: "audit", Path: "tools/audit", Kind: "infra", Repository: "workspace", Purpose: "Audits", Owner: "Ops",
		Dependencies: []string{"wsup"},
	})
	timeout := 30
	for _, task := range []Task{
		{Name: "scan", IsCheck: true, Cwd: "src", Check: Check{Command: []string{"scan"}, Mode: "report", Enabled: true, FindingExitCodes: []int{}, TimeoutSeconds: &timeout}},
		{Name: "lint", Check: Check{Command: []string{"lint"}, Mode: "required", Enabled: false, Reason: "Flaky", FindingExitCodes: []int{1}}},
	} {
		document.Projects[1].Tasks = append(document.Projects[1].Tasks, task)
	}
	encoded, err := Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("%v\n%s", err, encoded)
	}
	if !reflect.DeepEqual(decoded, document) {
		t.Fatalf("round trip:\n%+v\n%+v", decoded, document)
	}
	again, err := Encode(decoded)
	if err != nil || string(again) != string(encoded) {
		t.Fatalf("encoding is not stable: %v", err)
	}
	text := string(encoded)
	order := []string{`"$schema":`, `"version":`, `"name":`, `"collections":`, `"agents":`, `"editors":`, `"settings":`, `"projects":`, `"vscode":`}
	for i := 1; i < len(order); i++ {
		if strings.Index(text, order[i-1]) > strings.Index(text, order[i]) {
			t.Fatalf("%s written after %s:\n%s", order[i-1], order[i], text)
		}
	}
	if strings.Index(text, `"scan"`) > strings.Index(text, `"lint"`) || !strings.HasSuffix(text, "}\n") {
		t.Fatalf("task order or trailing newline:\n%s", text)
	}
	for _, absent := range []string{`"mode": "required"`, `"enabled": true`, `"finding_exit_codes": [1]`} {
		if strings.Contains(text, absent) {
			t.Fatalf("default %s written:\n%s", absent, text)
		}
	}
}

func TestEncodeWritesAnEmptyWorkspace(t *testing.T) {
	encoded, err := Encode(Document{Name: "My workspace"})
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"version\": 3,\n  \"name\": \"My workspace\",\n  \"collections\": [],\n  \"agents\": [],\n  \"editors\": [],\n  \"projects\": []\n}\n"
	if string(encoded) != want {
		t.Fatalf("got:\n%s", encoded)
	}
}

func TestDecodeRejectsInvalidDocuments(t *testing.T) {
	project := func(fields string) string {
		return `{"version":3,"name":"W","projects":[{"id":"a","path":"apps/a","kind":"app","repository":"workspace","purpose":"P","owner":"O"` + fields + `}]}`
	}
	// Each case below changes one thing in an otherwise valid document.
	if _, err := Decode([]byte(project(`,"dependencies":[],"tasks":{"t":{"command":["x"],"cwd":"src"}}`))); err != nil {
		t.Fatalf("base document rejected: %v", err)
	}
	for name, body := range map[string]string{
		"version 1":            `{"version":1,"name":"W","projects":[]}`,
		"unknown field":        `{"version":3,"name":"W","projects":[],"extra":1}`,
		"blank name":           `{"version":3,"name":" ","projects":[]}`,
		"blank schema":         `{"$schema":"","version":3,"name":"W","projects":[]}`,
		"missing projects":     `{"version":3,"name":"W"}`,
		"bad module id":        `{"version":3,"name":"W","collections":["Apps"],"projects":[]}`,
		"duplicate module":     `{"version":3,"name":"W","agents":["claude","claude"],"projects":[]}`,
		"unknown setting":      `{"version":3,"name":"W","settings":{"color":true},"projects":[]}`,
		"vscode field":         `{"version":3,"name":"W","projects":[],"vscode":{"keybindings":[]}}`,
		"vscode task shape":    `{"version":3,"name":"W","projects":[],"vscode":{"tasks":["build"]}}`,
		"old kind":             project(`,"kind":"infrastructure"`),
		"unknown project key":  project(`,"checks":{}`),
		"bad repository":       project(`,"repository":"monorepo"`),
		"absolute path":        project(`,"path":"/apps/a"`),
		"escaping path":        project(`,"path":"../a"`),
		"unclean path":         project(`,"path":"apps//a"`),
		"multi-line git":       project(`,"git":"a\nb"`),
		"unknown dependency":   project(`,"dependencies":["b"]`),
		"self dependency":      project(`,"dependencies":["a"]`),
		"task bad name":        project(`,"tasks":{"a b":["x"]}`),
		"task empty command":   project(`,"tasks":{"t":[]}`),
		"task check not bool":  project(`,"tasks":{"t":{"command":["x"],"check":"yes"}}`),
		"task escaping cwd":    project(`,"tasks":{"t":{"command":["x"],"cwd":"../x"}}`),
		"task unknown field":   project(`,"tasks":{"t":{"command":["x"],"shell":true}}`),
		"task disabled reason": project(`,"tasks":{"t":{"command":["x"],"enabled":false}}`),
		"duplicate project":    `{"version":3,"name":"W","projects":[{"id":"a","path":"a","kind":"app","repository":"workspace","purpose":"P","owner":"O"},{"id":"a","path":"b","kind":"app","repository":"workspace","purpose":"P","owner":"O"}]}`,
		"shared path":          `{"version":3,"name":"W","projects":[{"id":"a","path":"a","kind":"app","repository":"workspace","purpose":"P","owner":"O"},{"id":"b","path":"a","kind":"app","repository":"workspace","purpose":"P","owner":"O"}]}`,
		"cycle":                `{"version":3,"name":"W","projects":[{"id":"a","path":"a","kind":"app","repository":"workspace","purpose":"P","owner":"O","dependencies":["b"]},{"id":"b","path":"b","kind":"app","repository":"workspace","purpose":"P","owner":"O","dependencies":["a"]}]}`,
	} {
		if _, err := Decode([]byte(body)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestDecodeAcceptsEveryProjectKindAndTheShorthand(t *testing.T) {
	for _, kind := range projectKinds {
		body := `{"version":3,"name":"W","projects":[{"id":"a","path":".","kind":"` + kind + `","repository":"independent","purpose":"P","owner":"O","tasks":{"t":["go","test"]}}]}`
		document, err := Decode([]byte(body))
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if task := document.Projects[0].Tasks[0]; task.IsCheck || !reflect.DeepEqual(task.Check.Command, []string{"go", "test"}) {
			t.Fatalf("shorthand: %+v", task)
		}
	}
}
