package scaffolding

import (
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brendenchu/wsup/internal/catalogue"
	"github.com/brendenchu/wsup/internal/registry"
)

func TestEmbeddedUpdatePreservesInstalledModulesAndBaselines(t *testing.T) {
	selected := []string{"archive", "journal", "packages"}
	files, err := catalogue.BundleSelected("workspace", selected)
	if err != nil {
		t.Fatal(err)
	}
	root, state := nativeFixture(t, files)
	if state.Modules, err = catalogue.Installed("workspace", catalogue.Selection{Collections: selected}); err != nil {
		t.Fatal(err)
	}
	saveLock(t, root, state)
	writeFixture(t, root, "archive/README.md", []byte("Owner archive policy"))
	before := snapshot(t, root)
	plan, err := Update(root, "0.2.0", nil, false)
	if err != nil || !reflect.DeepEqual(plan.Conflicts, []string{"archive/README.md"}) {
		t.Fatalf("selected collection not updated: %+v %v", plan, err)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("embedded preview changed bytes")
	}
	result, err := Update(root, "0.2.0", []string{"archive/README.md"}, true)
	if err != nil || !result.Applied {
		t.Fatalf("selected update: %+v %v", result, err)
	}
	loaded, err := LoadLock(root)
	if err != nil || !reflect.DeepEqual(mustSelection(t, loaded.Lock).Collections, selected) ||
		!reflect.DeepEqual(loaded.Lock.Modules, state.Modules) ||
		loaded.Lock.Files["archive/README.md"] != state.Files["archive/README.md"] {
		t.Fatalf("modules or baseline changed: %+v %v", loaded.Lock, err)
	}
	if _, exists := loaded.Lock.Files["journal/TEMPLATE.md"]; !exists {
		t.Fatal("selected journal template dropped")
	}
	for _, name := range []string{"learning", "research", "sandbox"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("unselected collection created: %s", name)
		}
	}
}

func TestUpdateMergesGeneratedSectionsAroundOwnerEdits(t *testing.T) {
	root := collectionWorkspace(t)
	current, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	// Pretend an older wsup generated a different Layout section.
	old := []byte(strings.Replace(string(current), "- `docs/`:", "- `docs/`: (old)", 1))
	loaded, err := LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	loaded.Lock.Files["AGENTS.md"] = lockDigest(old)
	maps.Copy(loaded.Lock.Blocks, blockDigests("AGENTS.md", old))
	saveLock(t, root, loaded.Lock)
	owner := append([]byte("Owner preface.\n\n"), old...)
	writeFixture(t, root, "AGENTS.md", owner)

	plan, err := Update(root, "0.1.0", nil, false)
	if err != nil || !reflect.DeepEqual(plan.Actions, []Action{{"AGENTS.md", "merge"}}) || len(plan.Conflicts) != 0 {
		t.Fatalf("merge preview: %+v %v", plan, err)
	}
	if applied, err := Update(root, "0.1.0", nil, true); err != nil || !applied.Applied {
		t.Fatalf("merge apply: %+v %v", applied, err)
	}
	merged, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil || string(merged) != "Owner preface.\n\n"+string(current) {
		t.Fatalf("merged AGENTS.md: %q %v", merged, err)
	}
	after, err := LoadLock(root)
	if err != nil || after.Lock.Files["AGENTS.md"] != lockDigest(old) ||
		after.Lock.Blocks["AGENTS.md#layout"] != blockDigests("AGENTS.md", current)["AGENTS.md#layout"] {
		t.Fatalf("merge must keep the file baseline and adopt the new sections: %+v %v", after.Lock, err)
	}
	if settled, err := Update(root, "0.1.0", nil, false); err != nil || len(settled.Actions) != 0 || len(settled.Conflicts) != 0 {
		t.Fatalf("merged file did not settle: %+v %v", settled, err)
	}

	// An edit inside a generated section is a conflict, never merged.
	inside := strings.Replace(string(merged), "- `docs/`:", "- `docs/`: owner note,", 1)
	writeFixture(t, root, "AGENTS.md", []byte(inside))
	before := snapshot(t, root)
	if plan, err := Update(root, "0.1.0", nil, true); err == nil || !reflect.DeepEqual(plan.Conflicts, []string{"AGENTS.md"}) ||
		!reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatalf("section edit: %+v %v", plan, err)
	}
}

func TestUpdateReportsModuleVersionsOffersAndUnlisted(t *testing.T) {
	root := collectionWorkspace(t, "apps", "tools")
	loaded, err := LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	loaded.Lock.Modules["apps"] = "0.9.0"
	saveLock(t, root, loaded.Lock)
	document, err := registry.Decode(must(os.ReadFile(filepath.Join(root, "wsup.json"))))
	if err != nil {
		t.Fatal(err)
	}
	// journal is listed by hand; tools is no longer listed.
	document.Collections = []string{"apps", "journal"}
	writeFixture(t, root, "wsup.json", must(registry.Encode(document)))

	plan, err := Update(root, "0.1.0", nil, false)
	want := &ModuleReport{
		Updated:  []VersionChange{{ID: "apps", From: "0.9.0", To: "1.0.1"}},
		Offered:  []string{"journal"},
		Unlisted: []string{"tools"},
	}
	if err != nil || !reflect.DeepEqual(plan.Modules, want) || len(plan.Actions) != 0 {
		t.Fatalf("module report: %+v %+v %v", plan, plan.Modules, err)
	}
	if _, err := Update(root, "0.1.0", nil, true); err != nil {
		t.Fatal(err)
	}
	after, err := LoadLock(root)
	if err != nil || after.Lock.Modules["apps"] != "1.0.1" || after.Lock.Modules["tools"] == "" {
		t.Fatalf("apply must record the new version and keep unlisted modules: %+v %v", after.Lock.Modules, err)
	}
	if _, installed := after.Lock.Modules["journal"]; installed {
		t.Fatal("offered module installed silently")
	}
	if _, err := os.Stat(filepath.Join(root, "journal")); !os.IsNotExist(err) {
		t.Fatal("offered module wrote files")
	}
	settled, err := Update(root, "0.1.0", nil, false)
	if err != nil || len(settled.Modules.Updated) != 0 || !reflect.DeepEqual(settled.Modules.Offered, []string{"journal"}) {
		t.Fatalf("after apply: %+v %v", settled.Modules, err)
	}
}
