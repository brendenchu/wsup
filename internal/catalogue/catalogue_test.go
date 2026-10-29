package catalogue

import (
	"bytes"
	"io/fs"
	"os"
	"testing"
)

func TestEmbeddedCatalogueMatchesSource(t *testing.T) {
	embedded := Open()
	source := os.DirFS("../../templates")
	inventory := func(tree fs.FS) map[string][]byte {
		t.Helper()
		files := map[string][]byte{}
		err := fs.WalkDir(tree, ".", func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type()&fs.ModeSymlink != 0 {
				t.Fatalf("template symlink is not allowed: %s", name)
			}
			if entry.IsDir() {
				return nil
			}
			if name == "embed.go" {
				return nil
			}
			if !entry.Type().IsRegular() {
				t.Fatalf("native catalogue requires regular assets: %s", name)
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
		t.Fatalf("embedded %d files, source %d", len(got), len(want))
	}
	for name, body := range want {
		if actual, exists := got[name]; !exists || !bytes.Equal(actual, body) {
			t.Errorf("embedded asset differs: %s", name)
		}
	}
	for _, name := range []string{
		"modules/workspace-base/module.json", "modules/workspace-base/templates/.gitignore", "modules/workspace-base/files/.editorconfig",
		"modules/decisions/files/decisions/TEMPLATE.md", "skills/check-workspace/SKILL.md",
		"modules/project-base/files/.gitignore", "modules/project-base/files/WSUP-LICENSE",
		"modules/journal/files/journal/TEMPLATE.md",
	} {
		if _, exists := got[name]; !exists {
			t.Errorf("missing required hidden/licensed asset: %s", name)
		}
	}
}
