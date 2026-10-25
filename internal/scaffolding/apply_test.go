package scaffolding

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNativeApplyPreservesOwnershipAndSettles(t *testing.T) {
	root, state := nativeFixture(t, map[string][]byte{
		"README.md": []byte("old"), "custom.md": []byte("starter"),
		"workspace.json": []byte("starter registry"), "orphan.md": []byte("orphan"),
		"packages/README.md": []byte("home starter"),
	})
	state.Collections = []string{"archive"}
	state.OmittedHomeFiles = []string{"apps/.gitkeep", "packages/README.md"}
	saveState(t, root, state)
	for name, body := range map[string]string{
		"custom.md": "owner customization", "workspace.json": "owner registry",
		"orphan.md": "owner orphan", "packages/README.md": "owner home",
		".wsup/reports/old.json": "historical evidence",
		"wsup.local.json":        "local settings", "tools/project/source": "project source",
	} {
		writeFixture(t, root, name, []byte(body))
	}
	before := snapshot(t, root)
	bundle := map[string][]byte{
		"README.md": []byte("new"), "custom.md": []byte("new starter"),
		"workspace.json": []byte("new registry"), "new/nested.md": []byte("new content"),
		"apps/.gitkeep": {}, "packages/README.md": []byte("new home"),
	}
	result, err := ApplyUpdate(root, "0.2.0", bundle, []string{"custom.md"})
	if err != nil || !result.Applied || len(result.Conflicts) != 0 {
		t.Fatalf("apply: %+v %v", result, err)
	}
	after := snapshot(t, root)
	for name, body := range before {
		if name != "README.md" && name != filepath.FromSlash(StatePath) && after[name] != body {
			t.Fatalf("owner content changed: %s", name)
		}
	}
	if after["README.md"] != "file:new" || after[filepath.FromSlash("new/nested.md")] != "file:new content" {
		t.Fatal("planned writes missing")
	}
	loaded, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.State.TemplateVersion != "0.2.0" ||
		loaded.State.Files["custom.md"] != state.Files["custom.md"] ||
		loaded.State.Files["workspace.json"] != state.Files["workspace.json"] ||
		loaded.State.Files["README.md"] != contentDigest([]byte("new")) ||
		!reflect.DeepEqual(loaded.State.Collections, state.Collections) ||
		!reflect.DeepEqual(loaded.State.OmittedHomeFiles, state.OmittedHomeFiles) {
		t.Fatalf("incorrect next state: %+v", loaded.State)
	}
	for _, name := range []string{"orphan.md", "packages/README.md", "apps/.gitkeep"} {
		if _, managed := loaded.State.Files[name]; managed {
			t.Fatalf("retained orphan/omission still managed: %s", name)
		}
	}
	settled, err := PlanUpdate(root, "0.2.0", bundle, []string{"custom.md"})
	if err != nil || !reflect.DeepEqual(settled.Actions, []Action{{"custom.md", "keep-customization"}}) || len(settled.Conflicts) != 0 {
		t.Fatalf("not settled: %+v %v", settled, err)
	}
	unkept, err := PlanUpdate(root, "0.2.0", bundle, nil)
	if err != nil || !reflect.DeepEqual(unkept.Conflicts, []string{"custom.md"}) {
		t.Fatalf("kept customization adopted as baseline: %+v %v", unkept, err)
	}
}

func TestApplyConflictsBlockEveryWrite(t *testing.T) {
	root, _ := nativeFixture(t, map[string][]byte{"custom.md": []byte("old")})
	writeFixture(t, root, "custom.md", []byte("owner"))
	before := snapshot(t, root)
	result, err := ApplyUpdate(root, "0.2.0", map[string][]byte{
		"a/new.md": []byte("new"), "custom.md": []byte("new"),
	}, nil)
	if err == nil || result.Applied || !strings.Contains(err.Error(), "no files updated") ||
		!reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatalf("conflict did not block everything: %+v %v", result, err)
	}
}

func prepareFixture(t *testing.T) (string, preparedUpdate) {
	t.Helper()
	root, _ := nativeFixture(t, map[string][]byte{"README.md": []byte("old")})
	var prepared preparedUpdate
	_, err := planUpdate(root, "0.2.0", map[string][]byte{
		"README.md": []byte("new"), "new/nested.md": []byte("new file"),
	}, nil, &prepared)
	if err != nil {
		t.Fatal(err)
	}
	return root, prepared
}

func TestApplyRechecksStateFilesAndNewDestinationsBeforeAnyWrite(t *testing.T) {
	for _, name := range []string{StatePath, "README.md", "new/nested.md"} {
		t.Run(name, func(t *testing.T) {
			root, prepared := prepareFixture(t)
			writeFixture(t, root, name, []byte("owner changed during planning"))
			before := snapshot(t, root)
			result, err := prepared.apply()
			if err == nil || result.Applied || !strings.Contains(err.Error(), "changed during update planning") ||
				!reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatalf("stale plan changed workspace: %+v %v", result, err)
			}
		})
	}
}

func TestApplyRechecksSymlinkParentsWithoutExternalWrites(t *testing.T) {
	root, prepared := prepareFixture(t)
	outside := t.TempDir()
	writeFixture(t, outside, "nested.md", []byte("external owner"))
	if err := os.Symlink(outside, filepath.Join(root, "new")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	before, external := snapshot(t, root), snapshot(t, outside)
	if _, err := prepared.apply(); err == nil {
		t.Fatal("redirected parent accepted")
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) || !reflect.DeepEqual(external, snapshot(t, outside)) {
		t.Fatal("stale symlink plan wrote content")
	}
}

func TestApplyIOFailureLeavesOldStateAndExplicitPartialError(t *testing.T) {
	for _, failing := range []string{"README.md", "new/nested.md", StatePath} {
		t.Run(failing, func(t *testing.T) {
			root, prepared := prepareFixture(t)
			before := snapshot(t, root)
			injected := errors.New("synthetic write denied")
			result, err := prepared.applyWith(func(handle *os.Root, name string, body []byte, exclusive bool) error {
				if name == failing {
					return injected
				}
				return writeManaged(handle, name, body, exclusive)
			})
			if !errors.Is(err, injected) || result.Applied ||
				!strings.Contains(err.Error(), "may already be written") {
				t.Fatalf("hidden I/O failure: %+v %v", result, err)
			}
			after := snapshot(t, root)
			if before[filepath.FromSlash(StatePath)] != after[filepath.FromSlash(StatePath)] {
				t.Fatal("failed update committed new state")
			}
			if failing == "README.md" && !reflect.DeepEqual(before, after) {
				t.Fatal("first write failure changed files")
			}
			for name := range after {
				if strings.HasPrefix(filepath.Base(name), ".wsup-") {
					t.Fatalf("temporary file leaked: %s", name)
				}
			}
		})
	}
}

func TestManagedWriterExclusiveCreationAndHardLinkIsolation(t *testing.T) {
	root := t.TempDir()
	handle, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	if err := writeManaged(handle, "file.md", []byte("old"), true); err != nil {
		t.Fatal(err)
	}
	if err := writeManaged(handle, "file.md", []byte("overwrite"), true); err == nil {
		t.Fatal("exclusive creation replaced an existing file")
	}
	if err := os.Link(filepath.Join(root, "file.md"), filepath.Join(root, "owner.md")); err != nil {
		t.Fatal(err)
	}
	if err := writeManaged(handle, "file.md", []byte("new"), false); err != nil {
		t.Fatal(err)
	}
	for name, expected := range map[string][]byte{"file.md": []byte("new"), "owner.md": []byte("old")} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || !bytes.Equal(body, expected) {
			t.Fatalf("hard link content corrupted: %s %q %v", name, body, err)
		}
	}
}
