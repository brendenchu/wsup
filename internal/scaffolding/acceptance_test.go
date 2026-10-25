package scaffolding

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/brendenchu/wsup/internal/catalogue"
)

func TestCollectionAdditionPreservesUnrelatedDirectoryContents(t *testing.T) {
	root := collectionWorkspace(t)
	writeFixture(t, root, "archive/existing.txt", []byte("Owner evidence\x00\r\n"))
	before := snapshot(t, root)
	if result, err := AddCollections(root, []string{"archive"}, true); err != nil || !result.Applied {
		t.Fatalf("addition: %+v %v", result, err)
	}
	after := snapshot(t, root)
	for name, body := range before {
		if name != filepath.FromSlash(StatePath) && after[name] != body {
			t.Fatalf("unrelated content changed: %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "archive/README.md")); err != nil {
		t.Fatal(err)
	}
}

func TestSelectedCollectionScaffoldUpdatesWithNewBundle(t *testing.T) {
	root := collectionWorkspace(t, "archive")
	files, err := catalogue.BundleSelected("workspace", []string{"archive"})
	if err != nil {
		t.Fatal(err)
	}
	files["archive/README.md"] = []byte("Updated archive guidance\n")
	before := snapshot(t, root)
	preview, err := PlanUpdate(root, "0.2.0", files, nil)
	if err != nil || !reflect.DeepEqual(preview.Actions, []Action{{"archive/README.md", "write"}}) ||
		len(preview.Conflicts) != 0 || !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatalf("scaffold preview: %+v %v", preview, err)
	}
	if result, err := ApplyUpdate(root, "0.2.0", files, nil); err != nil || !result.Applied {
		t.Fatalf("scaffold apply: %+v %v", result, err)
	}
	body, err := os.ReadFile(filepath.Join(root, "archive/README.md"))
	if err != nil || !bytes.Equal(body, files["archive/README.md"]) {
		t.Fatal("updated scaffold not installed")
	}
	loaded, err := LoadState(root)
	if err != nil || !reflect.DeepEqual(loaded.State.Collections, []string{"archive"}) ||
		loaded.State.Files["archive/README.md"] != contentDigest(body) {
		t.Fatal("scaffold selection or next baseline lost")
	}
	settled, err := PlanUpdate(root, "0.2.0", files, nil)
	if err != nil || len(settled.Actions) != 0 || len(settled.Conflicts) != 0 {
		t.Fatalf("scaffold update did not settle: %+v %v", settled, err)
	}
}

func TestUpdateAddsUnmanagedDefaultMarkersAndKeepsOmissionsLocal(t *testing.T) {
	root := collectionWorkspace(t)
	loaded, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	markers := []string{"apps/.gitkeep", "infrastructure/.gitkeep", "packages/README.md", "tools/README.md", "workbench/README.md"}
	want := []Action{}
	for _, name := range markers {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Fatal(err)
		}
		delete(loaded.State.Files, name)
		want = append(want, Action{name, "write"})
	}
	saveState(t, root, loaded.State)
	before := snapshot(t, root)
	preview, err := Update(root, "0.1.0", nil, false)
	if err != nil || len(preview.Conflicts) != 0 || !reflect.DeepEqual(preview.Actions, want) ||
		!reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatalf("marker preview: %+v %v", preview, err)
	}
	if _, err := Update(root, "0.1.0", nil, true); err != nil {
		t.Fatal(err)
	}
	loaded, err = LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	loaded.State.OmittedHomeFiles = []string{"apps/.gitkeep", "packages/README.md"}
	saveState(t, root, loaded.State)
	if err := os.Remove(filepath.Join(root, "apps/.gitkeep")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "apps")); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "packages/README.md", []byte("Owner home policy"))
	if _, err := Update(root, "0.1.0", nil, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(root, "apps")); !os.IsNotExist(err) {
		t.Fatal("omitted home recreated")
	}
	body, err := os.ReadFile(filepath.Join(root, "packages/README.md"))
	if err != nil || string(body) != "Owner home policy" {
		t.Fatal("omitted home overwritten")
	}
	loaded, err = LoadState(root)
	if err != nil || !reflect.DeepEqual(loaded.State.OmittedHomeFiles, []string{"apps/.gitkeep", "packages/README.md"}) {
		t.Fatal("omissions lost")
	}
	child := collectionWorkspace(t)
	for _, name := range markers {
		if _, err := os.Stat(filepath.Join(child, filepath.FromSlash(name))); err != nil {
			t.Fatalf("child inherited omission: %s", name)
		}
	}
	settled, err := Update(root, "0.1.0", nil, false)
	if err != nil || len(settled.Actions) != 0 || len(settled.Conflicts) != 0 {
		t.Fatalf("omitted update did not settle: %+v %v", settled, err)
	}
}

func TestInvalidPersistedSelectionsAndPathsBlockEveryOperation(t *testing.T) {
	for field, values := range map[string][]any{
		"collections":        {nil, "archive", []any{"unknown"}, []any{1}},
		"omitted_home_files": {nil, "apps/.gitkeep", []any{"../escape"}, []any{".wsup/wsup.py"}, []any{1}},
		"files":              {map[string]string{"../escape": contentDigest(nil)}},
	} {
		for _, value := range values {
			for _, version := range []int{1, StateVersion} {
				root := collectionWorkspace(t)
				document := stateDocument(version)
				document[field] = value
				writeFixture(t, root, StatePath, encoded(t, document))
				before := snapshot(t, root)
				if _, err := Update(root, "0.2.0", nil, true); err == nil {
					t.Fatalf("update accepted invalid %s: %v", field, value)
				}
				if _, err := AddCollections(root, []string{"archive"}, true); err == nil {
					t.Fatalf("collections accepted invalid %s: %v", field, value)
				}
				if !reflect.DeepEqual(before, snapshot(t, root)) {
					t.Fatalf("invalid %s caused writes", field)
				}
			}
		}
	}
}

func TestCollectionFileSymlinkAndNonDirectoryParentsBlockAllWrites(t *testing.T) {
	for _, kind := range []string{"file-symlink", "file-parent", "unmanaged-root"} {
		t.Run(kind, func(t *testing.T) {
			root, external := collectionWorkspace(t), t.TempDir()
			switch kind {
			case "file-symlink":
				writeFixture(t, external, "source", []byte("Owner source"))
				if err := os.Mkdir(filepath.Join(root, "archive"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(external, "source"), filepath.Join(root, "archive/README.md")); err != nil {
					t.Fatal(err)
				}
			case "file-parent":
				writeFixture(t, root, "archive", []byte("Owner file"))
			case "unmanaged-root":
				if err := os.Remove(filepath.Join(root, filepath.FromSlash(StatePath))); err != nil {
					t.Fatal(err)
				}
			}
			before, outside := snapshot(t, root), snapshot(t, external)
			if _, err := AddCollections(root, []string{"archive", "artifacts"}, true); err == nil {
				t.Fatal("invalid destination accepted")
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) || !reflect.DeepEqual(outside, snapshot(t, external)) {
				t.Fatal("invalid destination changed owner bytes")
			}
		})
	}
}
