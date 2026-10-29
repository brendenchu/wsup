package scaffolding

import (
	"bytes"
	"maps"
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
	// Earlier versions installed only these files for the former defaults.
	former := []string{"journal/README.md", "journal/TEMPLATE.md", "learning/README.md", "reference/README.md", "research/README.md"}
	for _, name := range former {
		body := bundle[name]
		writeFixture(t, root, name, body)
		loaded.State.Files[name] = contentDigest(body)
	}
	loaded.State.OmittedHomeFiles = []string{"tools/README.md"}
	saveState(t, root, loaded.State)
	writeFixture(t, root, "learning/README.md", []byte("Owner learning policy"))
	stateBefore, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := AddCollections(root, []string{"journal", "learning", "reference", "research"}, true)
	// Only files the former defaults never had are written.
	want := []Action{{"AGENTS.md", "write"}, {"journal/AGENTS.md", "write"}, {"learning/AGENTS.md", "write"},
		{"reference/AGENTS.md", "write"}, {"research/AGENTS.md", "write"}}
	if err != nil || !result.Applied || !reflect.DeepEqual(result.Actions, want) || len(result.Conflicts) != 0 {
		t.Fatalf("former defaults: %+v %v", result, err)
	}
	stateAfter, err := LoadState(root)
	kept := true
	for _, name := range former {
		kept = kept && stateAfter.State.Files[name] == stateBefore.State.Files[name]
	}
	if err != nil || !kept ||
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

func TestFormerDefaultHomesStayManagedForEarlierWorkspaces(t *testing.T) {
	// v0.2.0 installed these homes in every workspace base without selecting them.
	files, err := catalogue.Bundle("workspace")
	if err != nil {
		t.Fatal(err)
	}
	homes, err := catalogue.BundleSelected("workspace", []string{"packages", "tools"})
	if err != nil {
		t.Fatal(err)
	}
	maps.Copy(files, homes)
	files["apps/.gitkeep"], files["infrastructure/.gitkeep"] = []byte{}, []byte{}
	root, _ := nativeFixture(t, files)
	plan, err := Update(root, "0.2.0", nil, false)
	want := []Action{{"AGENTS.md", "write"}, {"apps/.gitkeep", "retain-orphan"}, {"apps/AGENTS.md", "write"},
		{"apps/README.md", "write"}, {"infrastructure/.gitkeep", "retain-orphan"}}
	if err != nil || !reflect.DeepEqual(plan.Actions, want) || len(plan.Conflicts) != 0 {
		t.Fatalf("earlier workspace update: %+v %v", plan, err)
	}
	added, err := AddCollections(root, []string{"apps", "journal"}, true)
	if err != nil || len(added.Conflicts) != 0 || !added.Applied {
		t.Fatalf("earlier workspace addition: %+v %v", added, err)
	}
	loaded, err := LoadState(root)
	if err != nil || !reflect.DeepEqual(loaded.State.Collections, []string{"apps", "journal", "packages", "tools"}) {
		t.Fatalf("recorded collections: %+v %v", loaded.State, err)
	}
	if _, managed := loaded.State.Files["tools/README.md"]; !managed {
		t.Fatal("former default home stopped being managed")
	}
}
