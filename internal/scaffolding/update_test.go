package scaffolding

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/brendenchu/wsup/internal/catalogue"
)

func TestEmbeddedUpdatePreservesSelectedCollectionsAndOmissions(t *testing.T) {
	files, err := catalogue.BundleSelected("workspace", []string{"apps", "archive", "journal"})
	if err != nil {
		t.Fatal(err)
	}
	root, state := nativeFixture(t, files)
	state.Collections = []string{"apps", "archive", "journal"}
	state.OmittedHomeFiles = []string{"apps/.gitkeep"}
	saveState(t, root, state)
	if err := os.Remove(filepath.Join(root, "apps/.gitkeep")); err != nil {
		t.Fatal(err)
	}
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
	loaded, err := LoadState(root)
	if err != nil || !reflect.DeepEqual(loaded.State.Collections, state.Collections) ||
		!reflect.DeepEqual(loaded.State.OmittedHomeFiles, state.OmittedHomeFiles) ||
		loaded.State.Files["archive/README.md"] != state.Files["archive/README.md"] {
		t.Fatalf("selection or baseline changed: %+v %v", loaded.State, err)
	}
	if _, err := os.Stat(filepath.Join(root, "apps/.gitkeep")); !os.IsNotExist(err) {
		t.Fatal("omitted home recreated")
	}
	if _, exists := loaded.State.Files["journal/TEMPLATE.md"]; !exists {
		t.Fatal("selected journal template dropped")
	}
	for _, name := range []string{"learning", "private", "research"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("unselected collection created: %s", name)
		}
	}
}
