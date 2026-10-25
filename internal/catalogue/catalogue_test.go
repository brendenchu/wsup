package catalogue

import (
	"bytes"
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestEmbeddedCatalogueMatchesSource(t *testing.T) {
	embedded, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	source := os.DirFS("../../src/templates")
	inventory := func(tree fs.FS) map[string][]byte {
		t.Helper()
		files := map[string][]byte{}
		err := fs.WalkDir(tree, ".", func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "__pycache__" {
					return fs.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(name, ".pyc") {
				return nil
			}
			body, err := fs.ReadFile(tree, name)
			if err != nil {
				return err
			}
			files[name] = body
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return files
	}
	want, got := inventory(source), inventory(embedded)
	if len(want) != len(got) {
		t.Fatalf("embedded %d files, source %d; run go generate ./internal/catalogue", len(got), len(want))
	}
	for name, body := range want {
		if actual, exists := got[name]; !exists || !bytes.Equal(actual, body) {
			t.Errorf("embedded asset differs: %s; run go generate ./internal/catalogue", name)
		}
	}
	for _, name := range []string{
		"workspace/.gitignore", "workspace/apps/.gitkeep",
		"workspace/.agents/skills/workspace-check/SKILL.md",
		"project/.gitignore", "project/WSUP-LICENSE",
		"collections/journal/TEMPLATE.md",
	} {
		if _, exists := got[name]; !exists {
			t.Errorf("missing required hidden/licensed asset: %s", name)
		}
	}
}
