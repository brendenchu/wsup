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
	"github.com/brendenchu/wsup/internal/guidance"
)

func TestNativeProjectMatchesEmbeddedCatalogueAndRelocates(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "nested", "project")
	actual, err := InitialiseProject(destination, "0.1.0")
	if err != nil || actual != destination {
		t.Fatalf("init: %s %v", actual, err)
	}
	files, err := catalogue.Bundle("project")
	if err != nil {
		t.Fatal(err)
	}
	state, err := LoadState(destination)
	if err != nil || state.State.Version != StateVersion || state.State.Template != "project" {
		t.Fatalf("native state: %+v %v", state.State, err)
	}
	for relative, want := range files {
		got, err := os.ReadFile(filepath.Join(destination, filepath.FromSlash(relative)))
		if err != nil || !bytes.Equal(want, got) || state.State.Files[relative] != contentDigest(want) {
			t.Errorf("starter/hash differs: %s %v", relative, err)
		}
	}
	if len(files) != len(state.State.Files) {
		t.Fatalf("inventory mismatch: %d", len(files))
	}
	for _, name := range []string{"workspace.json", ".wsup/runtime", ".wsup/templates"} {
		if _, err := os.Lstat(filepath.Join(destination, name)); !os.IsNotExist(err) {
			t.Fatalf("unexpected registry/runtime/catalogue: %s", name)
		}
	}
	moved := filepath.Join(parent, "relocated")
	if err := os.Rename(destination, moved); err != nil {
		t.Fatal(err)
	}
	result, err := guidance.Inspect(moved)
	if err != nil || len(result.Errors) != 0 {
		t.Fatalf("relocated guidance: %+v %v", result, err)
	}
}

func TestProjectInitRefusesExistingDestinationsAndInvalidBundleBeforeWrites(t *testing.T) {
	parent := t.TempDir()
	existing := filepath.Join(parent, "existing")
	if err := os.Mkdir(existing, 0755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, parent, "file", []byte("owner"))
	paths := []string{existing, filepath.Join(parent, "file")}
	link := filepath.Join(parent, "dangling")
	if err := os.Symlink(filepath.Join(parent, "absent"), link); err == nil {
		paths = append(paths, link)
	}
	before := snapshot(t, parent)
	for _, destination := range paths {
		if _, err := InitialiseProject(destination, "0.1.0"); err == nil {
			t.Fatalf("existing destination accepted: %s", destination)
		}
	}
	for _, name := range []string{"../escape", StatePath, ".wsup/runtime", "workspace.json"} {
		if _, err := initialiseProject(filepath.Join(parent, "new"), "0.1.0", map[string][]byte{name: {}}, writeManaged); err == nil {
			t.Fatalf("invalid bundle accepted: %s", name)
		}
	}
	if !reflect.DeepEqual(before, snapshot(t, parent)) {
		t.Fatal("rejected initialization changed files")
	}
}

func TestProjectInitIOFailureIsExplicitAndStateIsLast(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "partial")
	injected := errors.New("synthetic state write denied")
	_, err := initialiseProject(destination, "0.1.0", map[string][]byte{"README.md": []byte("new")}, func(root *os.Root, name string, body []byte, exclusive bool) error {
		if name == StatePath {
			return injected
		}
		return writeManaged(root, name, body, exclusive)
	})
	if !errors.Is(err, injected) || !strings.Contains(err.Error(), "partial files/directories may remain") {
		t.Fatalf("partial failure hidden: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(destination, "README.md"))
	if err != nil || string(body) != "new" {
		t.Fatal("partial output not retained for inspection")
	}
	if _, err := os.Lstat(filepath.Join(destination, StatePath)); !os.IsNotExist(err) {
		t.Fatal("state committed before content completed")
	}
}
