package scaffolding

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brendenchu/wsup/internal/catalogue"
)

func collectionWorkspace(t *testing.T, selected ...string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workspace")
	if _, err := InitialiseSelected(root, "0.1.0", "workspace", selected); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCollectionsInitSelectionAndAllOptionalHomes(t *testing.T) {
	for _, selected := range [][]string{{"research", "archive", "archive"}, catalogue.CollectionNames()} {
		root := collectionWorkspace(t, selected...)
		state, err := LoadState(root)
		want, _ := catalogue.SelectCollections(selected)
		if err != nil || !reflect.DeepEqual(state.State.Collections, want) {
			t.Fatalf("init selections: %+v %v", state.State, err)
		}
		files, err := catalogue.BundleSelected("workspace", selected)
		if err != nil || len(files) != len(state.State.Files) {
			t.Fatal("selected file inventory mismatch")
		}
		for name, body := range files {
			current, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
			if err != nil || !bytes.Equal(current, body) || state.State.Files[name] != contentDigest(body) {
				t.Fatalf("selected asset/hash mismatch: %s %v", name, err)
			}
		}
		for _, name := range catalogue.CollectionNames() {
			exists := false
			for file := range state.State.Files {
				exists = exists || strings.HasPrefix(file, name+"/")
			}
			found := false
			for _, enabled := range want {
				if name == enabled {
					found = true
				}
			}
			if exists != found {
				t.Fatalf("unexpected selected home: %s", name)
			}
		}
		plan, err := Update(root, "0.1.0", nil, false)
		if err != nil || len(plan.Actions) != 0 || len(plan.Conflicts) != 0 {
			t.Fatalf("selected init not settled: %+v %v", plan, err)
		}
	}
	parent := t.TempDir()
	for _, selection := range [][]string{{"unknown"}, {"../escape"}, {""}} {
		if _, err := InitialiseSelected(filepath.Join(parent, "bad"), "0.1.0", "workspace", selection); err == nil {
			t.Fatal("invalid init selection accepted")
		}
	}
	if _, err := InitialiseSelected(filepath.Join(parent, "bad"), "0.1.0", "project", []string{"archive"}); err == nil {
		t.Fatal("project collection accepted")
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid selection wrote output")
	}
}

func TestCollectionListingPreviewApplyAndIdempotence(t *testing.T) {
	// Listing needs no workspace or state.
	listed, err := AddCollections(filepath.Join(t.TempDir(), "absent"), nil, false)
	if err != nil || !reflect.DeepEqual(listed.Available, catalogue.CollectionNames()) || listed.Selected != nil {
		t.Fatalf("list: %+v %v", listed, err)
	}
	root := collectionWorkspace(t)
	before := snapshot(t, root)
	preview, err := AddCollections(root, []string{"journal", "archive", "journal"}, false)
	want := []Action{{"archive/README.md", "write"}, {"journal/README.md", "write"}, {"journal/TEMPLATE.md", "write"}}
	if err != nil || !reflect.DeepEqual(preview.Actions, want) || !reflect.DeepEqual(*preview.Selected, []string{"archive", "journal"}) || preview.Applied {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("preview wrote files")
	}
	applied, err := AddCollections(root, []string{"journal", "archive"}, true)
	if err != nil || !applied.Applied || !reflect.DeepEqual(applied.Actions, want) {
		t.Fatalf("apply: %+v %v", applied, err)
	}
	before = snapshot(t, root)
	again, err := AddCollections(root, []string{"journal", "archive"}, true)
	if err != nil || !again.Applied || len(again.Actions) != 0 || !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatalf("not idempotent: %+v %v", again, err)
	}
	writeFixture(t, root, "archive/README.md", []byte("Owner customization"))
	stateBefore, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	before = snapshot(t, root)
	again, err = AddCollections(root, []string{"archive"}, true)
	if err != nil || len(again.Actions) != 0 || !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("selected customized scaffold rewritten")
	}
	stateAfter, err := LoadState(root)
	if err != nil || !bytes.Equal(stateBefore.Original, stateAfter.Original) {
		t.Fatal("selected customization adopted")
	}
}

func TestCollectionConflictsBlockAllAdditionsEvenIdenticalUnmanagedBytes(t *testing.T) {
	for _, managed := range []bool{false, true} {
		root := collectionWorkspace(t)
		bundle, err := catalogue.BundleSelected("workspace", []string{"archive"})
		if err != nil {
			t.Fatal(err)
		}
		if managed {
			if _, err := AddCollections(root, []string{"archive"}, true); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(root, "archive/README.md")); err != nil {
				t.Fatal(err)
			}
		} else {
			writeFixture(t, root, "archive/README.md", bundle["archive/README.md"])
		}
		before := snapshot(t, root)
		preview, err := AddCollections(root, []string{"archive", "releases"}, false)
		if err != nil || !reflect.DeepEqual(preview.Conflicts, []string{"archive/README.md"}) {
			t.Fatalf("conflicts: %+v %v", preview, err)
		}
		result, err := AddCollections(root, []string{"archive", "releases"}, true)
		if err == nil || result.Applied || !reflect.DeepEqual(before, snapshot(t, root)) {
			t.Fatal("conflict did not block all additions")
		}
	}
}

func TestCollectionStaleStateDestinationAndPartialIOFailure(t *testing.T) {
	for _, change := range []string{StatePath, "archive/README.md"} {
		root := collectionWorkspace(t)
		_, prepared, err := planCollections(root, []string{"archive"})
		if err != nil {
			t.Fatal(err)
		}
		writeFixture(t, root, change, []byte("owner concurrent bytes"))
		before := snapshot(t, root)
		if _, err := prepared.apply(); err == nil || !reflect.DeepEqual(before, snapshot(t, root)) {
			t.Fatal("stale collection plan wrote content")
		}
	}
	root := collectionWorkspace(t)
	_, prepared, err := planCollections(root, []string{"archive", "journal"})
	if err != nil {
		t.Fatal(err)
	}
	injected := errors.New("synthetic state write failure")
	_, err = prepared.applyWith(func(handle *os.Root, name string, body []byte, exclusive bool) error {
		if name == StatePath {
			return injected
		}
		return writeManaged(handle, name, body, exclusive)
	})
	if !errors.Is(err, injected) || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("partial failure not surfaced: %v", err)
	}
	current, err := LoadState(root)
	if err != nil || !bytes.Equal(current.Original, prepared.stateBefore) {
		t.Fatal("partial collection update committed state")
	}
	if _, err := os.Stat(filepath.Join(root, "journal/TEMPLATE.md")); err != nil {
		t.Fatal("partial files not observable")
	}
}

func TestCollectionsInvalidInputsAndSymlinkRefusal(t *testing.T) {
	root := collectionWorkspace(t)
	before := snapshot(t, root)
	for _, names := range [][]string{nil, {"unknown"}, {"../escape"}} {
		if _, err := AddCollections(root, names, true); err == nil {
			t.Fatalf("invalid addition accepted: %v", names)
		}
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("invalid addition changed bytes")
	}
	project := filepath.Join(t.TempDir(), "project")
	if _, err := InitialiseProject(project, "0.1.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := AddCollections(project, []string{"archive"}, true); err == nil {
		t.Fatal("project addition accepted")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "archive")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	before, external := snapshot(t, root), snapshot(t, outside)
	if _, err := AddCollections(root, []string{"archive"}, true); err == nil {
		t.Fatal("symlink addition accepted")
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) || !reflect.DeepEqual(external, snapshot(t, outside)) {
		t.Fatal("symlink target or owner modified")
	}
}
