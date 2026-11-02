package scaffolding

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wavykit/wsup/internal/catalogue"
	"github.com/wavykit/wsup/internal/guidance"
	"github.com/wavykit/wsup/internal/registry"
)

// TestNestedStartersAreIndependent nests a project and two workspaces inside
// a workspace. Each child validates, inspects, and updates on its own, and
// the parent's commands never read, manage, or report the children's files.
func TestNestedStartersAreIndependent(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "parent")
	all := catalogue.Selection{Collections: contractPreset, Agents: []string{"claude", "gemini"}, Editors: []string{"vscode"}}
	if _, err := InitialiseWith(parent, "0.1.0", "workspace", all); err != nil {
		t.Fatal(err)
	}
	before, err := guidance.Inspect(parent)
	if err != nil {
		t.Fatal(err)
	}
	children := map[string]string{"apps/app": "project", "infra/ops": "workspace", "inner": "workspace"}
	for path, starter := range children {
		selection := all
		if starter == "project" {
			selection = catalogue.Selection{Agents: all.Agents}
		}
		if _, err := InitialiseWith(filepath.Join(parent, path), "0.1.0", starter, selection); err != nil {
			t.Fatal(err)
		}
		// A child's own guidance problem is the child's, not the parent's.
		writeFixture(t, filepath.Join(parent, path), "AGENTS.md", []byte("Child rules. [missing](missing.md)\n"))
	}
	register(t, parent, []registry.ProjectEntry{
		{ID: "app", Path: "apps/app", Kind: "app", Repository: "independent", Purpose: "App.", Owner: "Owner"},
		{ID: "ops", Path: "infra/ops", Kind: "infra", Repository: "independent", Purpose: "Operations.", Owner: "Owner"},
	})

	if data, err := registry.Load(parent); err != nil || len(data.Projects) != 2 {
		t.Fatalf("parent registry: %+v %v", data, err)
	}
	after, err := guidance.Inspect(parent)
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("parent guidance read a child:\nbefore %+v\n after %+v %v", before, after, err)
	}
	// Registering projects regenerates only the parent's editor file.
	plan, err := Update(parent, "0.1.0", nil, true)
	if err != nil || !reflect.DeepEqual(plan.Actions, []Action{{"my-workspace.code-workspace", "write"}}) {
		t.Fatalf("parent update: %+v %v", plan, err)
	}
	for name := range must(LoadLock(parent)).Lock.Files {
		for path := range children {
			if strings.HasPrefix(name, path+"/") {
				t.Errorf("parent lock manages a child file: %s", name)
			}
		}
	}

	for path, starter := range children {
		root := filepath.Join(parent, path)
		inspection, err := guidance.Inspect(root)
		if err != nil || !reflect.DeepEqual(inspection.Errors, []string{"AGENTS.md: unresolved/nonlocal reference"}) {
			t.Errorf("%s guidance: %v %v", path, inspection.Errors, err)
		}
		if starter == "workspace" {
			if data, err := registry.Load(root); err != nil || len(data.Projects) != 0 {
				t.Errorf("%s registry: %+v %v", path, data, err)
			}
		}
		// The edited AGENTS.md is the only difference from a fresh child.
		if plan, err := Update(root, "0.1.0", nil, false); err != nil || len(plan.Actions) != 0 ||
			!reflect.DeepEqual(plan.Conflicts, []string{"AGENTS.md"}) {
			t.Errorf("%s update: %+v %v", path, plan, err)
		}
	}
	if _, err := os.Stat(filepath.Join(parent, "apps/app/wsup.json")); !os.IsNotExist(err) {
		t.Error("a nested project gained a registry")
	}
}

// register replaces the workspace's projects in wsup.json.
func register(t *testing.T, root string, projects []registry.ProjectEntry) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, registry.DocumentPath))
	if err != nil {
		t.Fatal(err)
	}
	document, err := registry.Decode(body)
	if err != nil {
		t.Fatal(err)
	}
	document.Projects = projects
	if body, err = registry.Encode(document); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, registry.DocumentPath, body)
}
