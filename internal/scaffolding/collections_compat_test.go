package scaffolding

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/brendenchu/wsup/internal/catalogue"
)

func TestFormerDefaultCollectionsRetainCustomizationsAndBaselines(t *testing.T) {
	root := collectionWorkspace(t)
	loaded, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := catalogue.BundleSelected("workspace", []string{"journal", "learning", "reference", "research"})
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range bundle {
		if _, exists := loaded.State.Files[name]; exists {
			continue
		}
		writeFixture(t, root, name, body)
		loaded.State.Files[name] = contentDigest(body)
	}
	loaded.State.OmittedHomeFiles = []string{"apps/.gitkeep"}
	saveState(t, root, loaded.State)
	writeFixture(t, root, "learning/README.md", []byte("Owner learning policy"))
	stateBefore, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := AddCollections(root, []string{"journal", "learning", "reference", "research"}, true)
	if err != nil || !result.Applied || len(result.Actions) != 0 || len(result.Conflicts) != 0 {
		t.Fatalf("former defaults: %+v %v", result, err)
	}
	stateAfter, err := LoadState(root)
	if err != nil || !reflect.DeepEqual(stateBefore.State.Files, stateAfter.State.Files) ||
		stateAfter.State.TemplateVersion != stateBefore.State.TemplateVersion ||
		!reflect.DeepEqual(stateAfter.State.OmittedHomeFiles, stateBefore.State.OmittedHomeFiles) {
		t.Fatal("selection changed prior baselines, version or omissions")
	}
	current, err := os.ReadFile(filepath.Join(root, "learning/README.md"))
	if err != nil || !bytes.Equal(current, []byte("Owner learning policy")) {
		t.Fatal("former default customization changed")
	}
	plan, err := Update(root, "0.1.0", nil, false)
	if err != nil || !reflect.DeepEqual(plan.Conflicts, []string{"learning/README.md"}) {
		t.Fatalf("customization no longer conflicts on update: %+v %v", plan, err)
	}
}

func TestUnselectedManagedNonDefaultCollectionCannotBeSilentlyEnabled(t *testing.T) {
	root := collectionWorkspace(t)
	loaded, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "archive/README.md", []byte("Managed but unselected"))
	loaded.State.Files["archive/README.md"] = contentDigest([]byte("Managed but unselected"))
	saveState(t, root, loaded.State)
	before := snapshot(t, root)
	if result, err := AddCollections(root, []string{"archive"}, true); err == nil || result.Applied {
		t.Fatal("unselected managed non-default was adopted")
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("conflicting selection wrote state")
	}
}

func TestCollectionAdditionsRejectUnsupportedStateWithoutWrites(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, StatePath, []byte(`{"version":1,"template_version":"0.1.0","template":"workspace","files":{}}`))
	before := snapshot(t, root)
	if _, err := AddCollections(root, []string{"archive"}, true); err == nil {
		t.Fatal("unsupported state accepted")
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("unsupported state modified")
	}
}
