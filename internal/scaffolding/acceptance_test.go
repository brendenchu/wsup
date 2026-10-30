package scaffolding

import (
	"bytes"
	"encoding/json"
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
	if result, err := AddModules(root, []string{"archive"}, true); err != nil || !result.Applied {
		t.Fatalf("addition: %+v %v", result, err)
	}
	after := snapshot(t, root)
	for name, body := range before {
		// AGENTS.md lists the selected collections in its generated layout, and
		// wsup.json records them.
		if name != filepath.FromSlash(LockPath) && name != "AGENTS.md" && name != "wsup.json" && after[name] != body {
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
	loaded, err := LoadLock(root)
	if err != nil || loaded.Lock.Modules["archive"] == "" ||
		loaded.Lock.Files["archive/README.md"] != lockDigest(body) {
		t.Fatal("scaffold selection or next baseline lost")
	}
	settled, err := PlanUpdate(root, "0.2.0", files, nil)
	if err != nil || len(settled.Actions) != 0 || len(settled.Conflicts) != 0 {
		t.Fatalf("scaffold update did not settle: %+v %v", settled, err)
	}
}

func TestUpdateRestoresMissingUnmanagedFiles(t *testing.T) {
	development := []string{"apps", "infra", "packages", "tools"}
	root := collectionWorkspace(t, development...)
	loaded, err := LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	markers := []string{"apps/README.md", "infra/README.md", "packages/README.md", "tools/README.md", "workbench/README.md"}
	want := []Action{}
	for _, name := range markers {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Fatal(err)
		}
		delete(loaded.Lock.Files, name)
		want = append(want, Action{name, "write"})
	}
	saveLock(t, root, loaded.Lock)
	before := snapshot(t, root)
	preview, err := Update(root, "0.1.0", nil, false)
	if err != nil || len(preview.Conflicts) != 0 || !reflect.DeepEqual(preview.Actions, want) ||
		!reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatalf("marker preview: %+v %v", preview, err)
	}
	if _, err := Update(root, "0.1.0", nil, true); err != nil {
		t.Fatal(err)
	}
	// A managed file the owner deletes is a conflict, never silently restored.
	if err := os.Remove(filepath.Join(root, "tools/README.md")); err != nil {
		t.Fatal(err)
	}
	if plan, err := Update(root, "0.1.0", nil, false); err != nil || !reflect.DeepEqual(plan.Conflicts, []string{"tools/README.md"}) {
		t.Fatalf("deleted managed file: %+v %v", plan, err)
	}
}

func TestInvalidPersistedLocksBlockEveryOperation(t *testing.T) {
	digest := lockDigest(nil)
	for name, edit := range map[string]func(*Lock){
		"unknown module":      func(l *Lock) { l.Modules["unknown"] = "1.0.0" },
		"preset module":       func(l *Lock) { l.Modules["development"] = "1.0.0" },
		"project base":        func(l *Lock) { l.Modules["project-base"] = "1.0.0" },
		"missing base":        func(l *Lock) { delete(l.Modules, "workspace-base") },
		"escaping file":       func(l *Lock) { l.Files["../escape"] = digest },
		"managed wsup.json":   func(l *Lock) { l.Files["wsup.json"] = digest },
		"unsupported version": func(l *Lock) { l.Version = 2 },
	} {
		root := collectionWorkspace(t)
		loaded, err := LoadLock(root)
		if err != nil {
			t.Fatal(err)
		}
		edit(&loaded.Lock)
		body, err := json.MarshalIndent(loaded.Lock, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		writeFixture(t, root, LockPath, body)
		before := snapshot(t, root)
		if _, err := Update(root, "0.2.0", nil, true); err == nil {
			t.Fatalf("update accepted %s", name)
		}
		if _, err := AddModules(root, []string{"archive"}, true); err == nil {
			t.Fatalf("collections accepted %s", name)
		}
		if !reflect.DeepEqual(before, snapshot(t, root)) {
			t.Fatalf("%s caused writes", name)
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
				if err := os.Remove(filepath.Join(root, filepath.FromSlash(LockPath))); err != nil {
					t.Fatal(err)
				}
			}
			before, outside := snapshot(t, root), snapshot(t, external)
			if _, err := AddModules(root, []string{"archive", "releases"}, true); err == nil {
				t.Fatal("invalid destination accepted")
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) || !reflect.DeepEqual(outside, snapshot(t, external)) {
				t.Fatal("invalid destination changed owner bytes")
			}
		})
	}
}
