package scaffolding

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brendenchu/wsup/internal/catalogue"
)

func TestMigrationPreservesSelectionsOmissionsKeptHashesAndOrphanBytes(t *testing.T) {
	root := pythonFixture(t)
	loaded, err := LoadState(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := catalogue.BundleSelected("workspace", []string{"archive", "journal"})
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if _, exists := loaded.State.Files[name]; exists {
			continue
		}
		writeFixture(t, root, name, body)
		loaded.State.Files[name] = contentDigest(body)
	}
	loaded.State.Collections = []string{"archive", "journal"}
	loaded.State.OmittedHomeFiles = []string{"apps/.gitkeep", "packages/README.md"}
	writeFixture(t, root, "learning/README.md", []byte("Former default starter"))
	loaded.State.Files["learning/README.md"] = contentDigest([]byte("Former default starter"))
	oldArchiveHash := loaded.State.Files["archive/README.md"]
	state, err := json.Marshal(loaded.State)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, StatePath, state)
	if err := os.Remove(filepath.Join(root, "apps/.gitkeep")); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "packages/README.md", []byte("Owner omitted home"))
	writeFixture(t, root, "archive/README.md", []byte("Owner archive"))
	writeFixture(t, root, "learning/README.md", []byte("Owner orphan"))
	result, err := ApplyMigration(root, "0.2.0", []string{"archive/README.md"})
	if err != nil || !result.Applied {
		t.Fatalf("metadata migration: %+v %v", result, err)
	}
	next, err := LoadState(root)
	if err != nil || !reflect.DeepEqual(next.State.Collections, loaded.State.Collections) ||
		!reflect.DeepEqual(next.State.OmittedHomeFiles, loaded.State.OmittedHomeFiles) ||
		next.State.Files["archive/README.md"] != oldArchiveHash {
		t.Fatalf("metadata changed: %+v %v", next.State, err)
	}
	for name, want := range map[string][]byte{
		"archive/README.md": []byte("Owner archive"), "packages/README.md": []byte("Owner omitted home"),
		"learning/README.md": []byte("Owner orphan"),
	} {
		current, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || !bytes.Equal(current, want) {
			t.Fatalf("owner bytes changed: %s", name)
		}
	}
	for _, name := range []string{"apps/.gitkeep", "packages/README.md", "learning/README.md"} {
		if _, exists := next.State.Files[name]; exists {
			t.Fatalf("omission/orphan still managed: %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "apps/.gitkeep")); !os.IsNotExist(err) {
		t.Fatal("omitted file recreated")
	}
}

func TestMigrationDoesNotSweepUnknownFileAddedDuringRetirement(t *testing.T) {
	root, prepared := migrationFixture(t)
	added := false
	result, err := prepared.applyWith(writeManaged, func(handle *os.Root, name string) error {
		if !added {
			added = true
			writeFixture(t, root, ".wsup/templates/concurrent-owner.txt", []byte("Unknown owner bytes"))
		}
		return handle.Remove(name)
	})
	if err != nil || !result.Applied {
		t.Fatalf("retirement: %+v %v", result, err)
	}
	body, err := os.ReadFile(filepath.Join(root, ".wsup/templates/concurrent-owner.txt"))
	if err != nil || string(body) != "Unknown owner bytes" {
		t.Fatal("unknown concurrent bytes swept up")
	}
}

func TestMigrationRefusesChangedExactSourceDuringRetirement(t *testing.T) {
	root, prepared := migrationFixture(t)
	once := false
	result, err := prepared.applyWith(writeManaged, func(handle *os.Root, name string) error {
		if !once {
			once = true
			writeFixture(t, root, ".wsup/wsup.py", []byte("Owner runtime changed during retirement"))
		}
		return handle.Remove(name)
	})
	if err == nil || result.Applied || !strings.Contains(err.Error(), "retirement incomplete") {
		t.Fatalf("changed source not blocked: %+v %v", result, err)
	}
	body, err := os.ReadFile(filepath.Join(root, ".wsup/wsup.py"))
	if err != nil || string(body) != "Owner runtime changed during retirement" {
		t.Fatal("changed source was retired")
	}
}
