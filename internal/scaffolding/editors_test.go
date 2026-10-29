package scaffolding

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brendenchu/wsup/internal/catalogue"
)

func TestEditorFileFollowsTheRegistry(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace")
	if _, err := InitialiseWith(root, "0.1.0", "workspace", catalogue.Selection{Editors: []string{"vscode"}}); err != nil {
		t.Fatal(err)
	}
	name := "my-workspace.code-workspace"
	loaded, err := LoadState(root)
	if _, managed := loaded.State.Files[name]; err != nil || !managed || !reflect.DeepEqual(loaded.State.Editors, []string{"vscode"}) {
		t.Fatalf("editor state: %+v %v", loaded.State, err)
	}
	if plan, err := Update(root, "0.1.0", nil, false); err != nil || len(plan.Actions) != 0 || len(plan.Conflicts) != 0 {
		t.Fatalf("fresh workspace does not settle: %+v %v", plan, err)
	}

	// Registering a project regenerates the file on update.
	writeFixture(t, root, "workspace.json", []byte(`{"version":1,"name":"My workspace","projects":[{"id":"api","path":"apps/api","kind":"app","repository":"workspace","purpose":"API","owner":"Team"}]}`))
	plan, err := Update(root, "0.1.0", nil, true)
	if err != nil || !plan.Applied || !reflect.DeepEqual(plan.Actions, []Action{{name, "write"}}) {
		t.Fatalf("project registration: %+v %v", plan, err)
	}
	body, err := os.ReadFile(filepath.Join(root, name))
	if err != nil || !strings.Contains(string(body), `"path": "apps/api"`) {
		t.Fatalf("regenerated file: %s %v", body, err)
	}

	// A hand-edited file is reported, never overwritten.
	writeFixture(t, root, name, append(body, ' '))
	writeFixture(t, root, "workspace.json", []byte(`{"version":1,"name":"My workspace","projects":[]}`))
	before := snapshot(t, root)
	if plan, err := Update(root, "0.1.0", nil, true); err == nil || !reflect.DeepEqual(plan.Conflicts, []string{name}) || !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatalf("hand-edited editor file: %+v %v", plan, err)
	}

	// An unreadable registry stops update rather than guessing the file.
	writeFixture(t, root, "workspace.json", []byte("not json"))
	if _, err := Update(root, "0.1.0", nil, false); err == nil {
		t.Fatal("update generated from an unreadable registry")
	}
}
