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
	loaded, err := LoadLock(root)
	if _, managed := loaded.Lock.Files[name]; err != nil || !managed || !reflect.DeepEqual(mustSelection(t, loaded.Lock).Editors, []string{"vscode"}) {
		t.Fatalf("editor state: %+v %v", loaded.Lock, err)
	}
	if plan, err := Update(root, "0.1.0", nil, false); err != nil || len(plan.Actions) != 0 || len(plan.Conflicts) != 0 {
		t.Fatalf("fresh workspace does not settle: %+v %v", plan, err)
	}

	// Registering a project regenerates the file on update.
	writeFixture(t, root, "wsup.json", []byte(`{"version":3,"name":"My workspace","projects":[{"id":"api","path":"apps/api","kind":"app","repository":"workspace","purpose":"API","owner":"Team"}]}`))
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
	writeFixture(t, root, "wsup.json", []byte(`{"version":3,"name":"My workspace","projects":[]}`))
	before := snapshot(t, root)
	if plan, err := Update(root, "0.1.0", nil, true); err == nil || !reflect.DeepEqual(plan.Conflicts, []string{name}) || !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatalf("hand-edited editor file: %+v %v", plan, err)
	}

	// Renaming the workspace replaces the file instead of orphaning it.
	writeFixture(t, root, name, body)
	writeFixture(t, root, "wsup.json", []byte(`{"version":3,"name":"Commerce","projects":[]}`))
	renamed := "commerce.code-workspace"
	plan, err = Update(root, "0.1.0", nil, true)
	if err != nil || !plan.Applied || !reflect.DeepEqual(plan.Actions, []Action{{renamed, "write"}, {name, "remove"}}) {
		t.Fatalf("rename: %+v %v", plan, err)
	}
	if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
		t.Fatalf("old editor file kept: %v", err)
	}
	loaded, err = LoadLock(root)
	if _, managed := loaded.Lock.Files[name]; err != nil || managed {
		t.Fatalf("old editor file still managed: %+v %v", loaded.Lock.Files, err)
	}
	if plan, err := Update(root, "0.1.0", nil, false); err != nil || len(plan.Actions) != 0 || len(plan.Conflicts) != 0 {
		t.Fatalf("renamed workspace does not settle: %+v %v", plan, err)
	}

	// An edited file under the old name is a conflict, and nothing is written.
	writeFixture(t, root, "wsup.json", []byte(`{"version":3,"name":"Shop","projects":[]}`))
	edited, err := os.ReadFile(filepath.Join(root, renamed))
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, renamed, append(edited, ' '))
	before = snapshot(t, root)
	if plan, err := Update(root, "0.1.0", nil, true); err == nil || !reflect.DeepEqual(plan.Conflicts, []string{renamed}) || !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatalf("edited old editor file: %+v %v", plan, err)
	}

	// An old file the owner already deleted needs no action.
	if err := os.Remove(filepath.Join(root, renamed)); err != nil {
		t.Fatal(err)
	}
	if plan, err := Update(root, "0.1.0", nil, true); err != nil || !plan.Applied || !reflect.DeepEqual(plan.Actions, []Action{{"shop.code-workspace", "write"}}) {
		t.Fatalf("deleted old editor file: %+v %v", plan, err)
	}

	// An unreadable registry stops update rather than guessing the file.
	writeFixture(t, root, "wsup.json", []byte("not json"))
	if _, err := Update(root, "0.1.0", nil, false); err == nil {
		t.Fatal("update generated from an unreadable registry")
	}
}
