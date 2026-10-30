package scaffolding

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/brendenchu/wsup/internal/catalogue"
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
