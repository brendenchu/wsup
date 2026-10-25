package scaffolding

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brendenchu/wsup/internal/catalogue"
	"github.com/brendenchu/wsup/internal/guidance"
	"github.com/brendenchu/wsup/internal/registry"
)

func TestNativeWorkspaceInventoryStateAndGuidance(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "workspace")
	actual, err := Initialise(destination, "0.1.0", "workspace")
	if err != nil || actual != destination {
		t.Fatalf("workspace init: %s %v", actual, err)
	}
	files, err := catalogue.Bundle("workspace")
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadState(destination)
	if err != nil || loaded.State.Version != StateVersion || loaded.State.Template != "workspace" ||
		len(loaded.State.Files) != len(files) || len(loaded.State.Collections) != 0 {
		t.Fatalf("workspace state: %+v %v", loaded.State, err)
	}
	for name, body := range files {
		current, err := os.ReadFile(filepath.Join(destination, filepath.FromSlash(name)))
		if err != nil || !bytes.Equal(current, body) || loaded.State.Files[name] != contentDigest(body) {
			t.Fatalf("workspace asset or baseline mismatch: %s %v", name, err)
		}
	}
	for _, name := range defaultHomes {
		if _, err := os.Stat(filepath.Join(destination, filepath.FromSlash(name))); err != nil {
			t.Fatalf("missing default home %s: %v", name, err)
		}
	}
	for _, name := range collections {
		if _, err := os.Lstat(filepath.Join(destination, name)); !os.IsNotExist(err) {
			t.Fatalf("unselected collection created: %s", name)
		}
	}
	entries, err := os.ReadDir(filepath.Join(destination, ".wsup"))
	if err != nil || len(entries) != 1 || entries[0].Name() != "template-state.json" {
		t.Fatalf("unexpected runtime/catalogue/report files: %v %v", entries, err)
	}
	data, err := registry.Load(destination)
	if err != nil || len(data.Projects) != 0 || data.Name != "My workspace" {
		t.Fatalf("invalid generated registry: %+v %v", data, err)
	}
	result, err := guidance.Inspect(destination)
	if err != nil || len(result.Errors) != 0 || len(result.Skills) != 12 {
		t.Fatalf("workspace guidance: %+v %v", result, err)
	}
	before := snapshot(t, destination)
	if _, err := Initialise(destination, "0.1.0", "workspace"); err == nil {
		t.Fatal("workspace initialization overwrote existing destination")
	}
	if !reflect.DeepEqual(before, snapshot(t, destination)) {
		t.Fatal("collision changed workspace")
	}
	plan, err := PlanUpdate(destination, "0.1.0", files, nil)
	if err != nil || len(plan.Actions) != 0 || len(plan.Conflicts) != 0 {
		t.Fatalf("fresh workspace does not settle: %+v %v", plan, err)
	}
}

func TestWorkspaceInitRefusesInvalidKindsBeforeDestinationWrites(t *testing.T) {
	parent := t.TempDir()
	for _, kind := range []string{"", "unknown", "../workspace"} {
		if _, err := Initialise(filepath.Join(parent, "new"), "0.1.0", kind); err == nil {
			t.Fatalf("invalid template kind accepted: %q", kind)
		}
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid kind wrote destination")
	}
}

func TestWorkspaceInitFailurePreservesPartialOutputWithoutState(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "partial")
	_, err := initialise(destination, "0.1.0", "workspace", map[string][]byte{
		"README.md": []byte("new"), "workspace.json": []byte("{}"),
	}, func(root *os.Root, name string, body []byte, exclusive bool) error {
		if name == StatePath {
			return os.ErrPermission
		}
		return writeManaged(root, name, body, exclusive)
	})
	if err == nil || !strings.Contains(err.Error(), "workspace initialization incomplete") {
		t.Fatalf("failure not explicit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "README.md")); err != nil {
		t.Fatal("partial content removed")
	}
	if _, err := os.Lstat(filepath.Join(destination, StatePath)); !os.IsNotExist(err) {
		t.Fatal("failed workspace initialization committed state")
	}
}
